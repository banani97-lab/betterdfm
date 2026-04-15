package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type JobsHandler struct {
	db  *gorm.DB
	aws *lib.AWSClients
}

func NewJobsHandler(database *gorm.DB, awsClients *lib.AWSClients) *JobsHandler {
	return &JobsHandler{db: database, aws: awsClients}
}

// jobResponse is AnalysisJob without BoardData — the board geometry is
// fetched separately via GET /jobs/:id/board and can be several MB on
// large boards. Including it inline made the job endpoint too slow.
//
// When the job is DONE and result blobs live in S3, we also inline
// presigned URLs for board + violations so the client skips a round-trip
// and can kick off both S3 fetches in parallel.
type jobResponse struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"orgId"`
	SubmissionID  string     `json:"submissionId"`
	ProfileID     string     `json:"profileId"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt"`
	CompletedAt   *time.Time `json:"completedAt"`
	ErrorMsg      string     `json:"errorMsg"`
	MfgScore      int        `json:"mfgScore"`
	MfgGrade      string     `json:"mfgGrade"`
	BoardURL      string     `json:"boardUrl,omitempty"`
	ViolationsURL string     `json:"violationsUrl,omitempty"`
}

// GetJob GET /jobs/:id
func (h *JobsHandler) GetJob(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")
	var job db.AnalysisJob
	if err := h.db.Omit("board_data").First(&job, "id = ? AND org_id = ?", id, user.OrgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "job not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	resp := jobResponse{
		ID:           job.ID,
		OrgID:        job.OrgID,
		SubmissionID: job.SubmissionID,
		ProfileID:    job.ProfileID,
		Status:       job.Status,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
		ErrorMsg:     job.ErrorMsg,
		MfgScore:     job.MfgScore,
		MfgGrade:     job.MfgGrade,
	}
	if job.Status == "DONE" && h.aws != nil && h.aws.S3Presign != nil {
		ctx := c.Request().Context()
		if job.BoardDataKey != "" {
			if url, err := h.aws.PresignGetURL(ctx, job.BoardDataKey, 15*time.Minute); err == nil {
				resp.BoardURL = url
			}
		}
		if job.ViolationsKey != "" {
			if url, err := h.aws.PresignGetURL(ctx, job.ViolationsKey, 15*time.Minute); err == nil {
				resp.ViolationsURL = url
			}
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// GetBoardData GET /jobs/:id/board
func (h *JobsHandler) GetBoardData(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")
	var job db.AnalysisJob
	if err := h.db.Select("id, org_id, board_data_key, board_data").
		First(&job, "id = ? AND org_id = ?", id, user.OrgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}
	// New path: serve from S3 via presigned URL
	if job.BoardDataKey != "" && h.aws != nil && h.aws.S3Presign != nil {
		url, err := h.aws.PresignGetURL(c.Request().Context(), job.BoardDataKey, 15*time.Minute)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate presigned URL")
		}
		return c.JSON(http.StatusOK, map[string]string{"url": url})
	}
	// Legacy fallback: inline JSONB
	if len(job.BoardData) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "board data not available")
	}
	return c.JSONBlob(http.StatusOK, job.BoardData)
}

// UpdateViolation PATCH /violations/:id
// Body: {"ignored": true|false}
// Returns the updated ignored state and the recomputed job score.
func (h *JobsHandler) UpdateViolation(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var body struct {
		Ignored bool `json:"ignored"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	var v db.Violation
	if err := h.db.First(&v, "id = ? AND org_id = ?", id, user.OrgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "violation not found")
	}

	if err := h.db.Model(&v).Update("ignored", body.Ignored).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Recompute score from remaining non-ignored violations for this job.
	var activeViolations []db.Violation
	h.db.Where("job_id = ? AND ignored = false", v.JobID).Find(&activeViolations)

	var job db.AnalysisJob
	h.db.Omit("board_data").First(&job, "id = ?", v.JobID)

	var board rptBoardData
	if len(job.BoardOutline) > 0 {
		_ = json.Unmarshal(job.BoardOutline, &board)
	} else {
		_ = json.Unmarshal(job.BoardData, &board)
	}

	sr := computeReportScore(activeViolations, board)
	h.db.Model(&job).Updates(map[string]interface{}{"mfg_score": sr.Score, "mfg_grade": sr.Grade})

	lib.Track("Violation Ignored", user.OrgID, map[string]any{"orgId": user.OrgID, "ruleId": v.RuleID, "severity": v.Severity, "ignored": body.Ignored})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":       v.ID,
		"ignored":  body.Ignored,
		"mfgScore": sr.Score,
		"mfgGrade": sr.Grade,
	})
}

// BulkIgnoreLayerViolations PATCH /jobs/:id/violations/by-layer
// Body: {"layer": "signal_1", "ignored": true, "severity": "ERROR"}
// Severity is optional — omit to affect all severities on the layer.
func (h *JobsHandler) BulkIgnoreLayerViolations(c echo.Context) error {
	user := lib.GetUser(c)
	jobID := c.Param("id")

	var body struct {
		Layer    string `json:"layer"`
		Ignored  bool   `json:"ignored"`
		Severity string `json:"severity"` // optional: "ERROR" | "WARNING" | "INFO" | ""
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if body.Layer == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "layer is required")
	}

	var job db.AnalysisJob
	if err := h.db.Omit("board_data").First(&job, "id = ? AND org_id = ?", jobID, user.OrgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	q := h.db.Model(&db.Violation{}).Where("job_id = ? AND layer = ?", jobID, body.Layer)
	if body.Severity != "" {
		q = q.Where("severity = ?", body.Severity)
	}
	if err := q.Update("ignored", body.Ignored).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var activeViolations []db.Violation
	h.db.Where("job_id = ? AND ignored = false", jobID).Find(&activeViolations)

	var board rptBoardData
	if len(job.BoardOutline) > 0 {
		_ = json.Unmarshal(job.BoardOutline, &board)
	} else {
		_ = json.Unmarshal(job.BoardData, &board)
	}

	sr := computeReportScore(activeViolations, board)
	h.db.Model(&job).Updates(map[string]interface{}{"mfg_score": sr.Score, "mfg_grade": sr.Grade})

	lib.Track("Violation Ignored", user.OrgID, map[string]any{"orgId": user.OrgID, "layer": body.Layer, "severity": body.Severity, "bulk": true})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"layer":    body.Layer,
		"ignored":  body.Ignored,
		"mfgScore": sr.Score,
		"mfgGrade": sr.Grade,
	})
}

// GetViolations GET /jobs/:id/violations
func (h *JobsHandler) GetViolations(c echo.Context) error {
	user := lib.GetUser(c)
	jobID := c.Param("id")

	// Verify job exists and belongs to user's org
	var job db.AnalysisJob
	if err := h.db.Omit("board_data").First(&job, "id = ? AND org_id = ?", jobID, user.OrgID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	// New path: serve from S3 via presigned URL
	if job.ViolationsKey != "" && h.aws != nil && h.aws.S3Presign != nil {
		url, err := h.aws.PresignGetURL(c.Request().Context(), job.ViolationsKey, 15*time.Minute)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate presigned URL")
		}
		return c.JSON(http.StatusOK, map[string]string{"url": url})
	}

	// Legacy fallback: query violations table
	var violations []db.Violation
	if err := h.db.Where("job_id = ?", jobID).Find(&violations).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, violations)
}
