package routes

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type SubmissionsHandler struct {
	db  *gorm.DB
	aws *lib.AWSClients
}

func NewSubmissionsHandler(database *gorm.DB, aws *lib.AWSClients) *SubmissionsHandler {
	return &SubmissionsHandler{db: database, aws: aws}
}

type submissionResponse struct {
	db.Submission
	LatestJobID string `json:"latestJobId"`
	MfgScore    int    `json:"mfgScore"`
	MfgGrade    string `json:"mfgGrade"`
}

// ListSubmissions GET /submissions
func (h *SubmissionsHandler) ListSubmissions(c echo.Context) error {
	user := lib.GetUser(c)
	var submissions []db.Submission
	if err := h.db.Where("org_id = ?", user.OrgID).Order("created_at desc").Find(&submissions).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Fetch latest job ID for each submission in one query
	ids := make([]string, len(submissions))
	for i, s := range submissions {
		ids[i] = s.ID
	}
	type jobRow struct {
		ID           string
		SubmissionID string
		MfgScore     int
		MfgGrade     string
	}
	var jobs []jobRow
	if len(ids) > 0 {
		h.db.Raw("SELECT id, submission_id, mfg_score, mfg_grade FROM analysis_jobs WHERE submission_id IN ?", ids).Scan(&jobs)
	}
	type jobInfo struct {
		id       string
		mfgScore int
		mfgGrade string
	}
	jobMap := map[string]jobInfo{}
	for _, j := range jobs {
		if _, exists := jobMap[j.SubmissionID]; !exists {
			jobMap[j.SubmissionID] = jobInfo{id: j.ID, mfgScore: j.MfgScore, mfgGrade: j.MfgGrade}
		}
	}

	result := make([]submissionResponse, len(submissions))
	for i, s := range submissions {
		info := jobMap[s.ID]
		result[i] = submissionResponse{
			Submission:  s,
			LatestJobID: info.id,
			MfgScore:    info.mfgScore,
			MfgGrade:    info.mfgGrade,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// CreateSubmission POST /submissions
func (h *SubmissionsHandler) CreateSubmission(c echo.Context) error {
	user := lib.GetUser(c)

	var req struct {
		Filename  string  `json:"filename"`
		FileType  string  `json:"fileType"` // GERBER | ODB_PLUS_PLUS
		ProjectID *string `json:"projectId"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Filename == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "filename required")
	}
	if req.FileType != "GERBER" && req.FileType != "ODB_PLUS_PLUS" {
		return echo.NewHTTPError(http.StatusBadRequest, "fileType must be GERBER or ODB_PLUS_PLUS")
	}

	submissionID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(req.Filename))
	if ext == "" {
		ext = ".zip"
	}
	fileKey := fmt.Sprintf("submissions/%s/%s%s", user.OrgID, submissionID, ext)

	// Validate projectId if provided
	if req.ProjectID != nil && *req.ProjectID != "" {
		var project db.Project
		if err := h.db.Where("id = ? AND org_id = ?", *req.ProjectID, user.OrgID).First(&project).Error; err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "project not found")
		}
	}

	submission := db.Submission{
		ID:        submissionID,
		OrgID:     user.OrgID,
		UserID:    user.Sub,
		ProjectID: req.ProjectID,
		Filename:  req.Filename,
		FileType:  req.FileType,
		FileKey:   fileKey,
		Status:    "UPLOADED",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("Submission Created", user.OrgID, map[string]any{"orgId": user.OrgID, "fileType": req.FileType, "projectId": req.ProjectID})

	contentType := "application/zip"
	presignedURL, err := h.aws.PresignPutURL(c.Request().Context(), fileKey, contentType)
	if err != nil {
		// Return the submission even if presign fails (dev mode without real S3)
		return c.JSON(http.StatusCreated, map[string]string{
			"submissionId": submissionID,
			"presignedUrl": "",
			"fileKey":      fileKey,
		})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"submissionId": submissionID,
		"presignedUrl": presignedURL,
		"fileKey":      fileKey,
	})
}

// GetSubmission GET /submissions/:id
func (h *SubmissionsHandler) GetSubmission(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")
	var submission db.Submission
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}
	return c.JSON(http.StatusOK, submission)
}

// StartAnalysis POST /submissions/:id/analyze
func (h *SubmissionsHandler) StartAnalysis(c echo.Context) error {
	user := lib.GetUser(c)
	submissionID := c.Param("id")

	var req struct {
		ProfileID string `json:"profileId"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	var submission db.Submission
	if err := h.db.Where("id = ? AND org_id = ?", submissionID, user.OrgID).First(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}

	// Use default profile if none specified
	if req.ProfileID == "" {
		var profile db.CapabilityProfile
		if err := h.db.Where("org_id = ? AND is_default = ?", user.OrgID, true).First(&profile).Error; err != nil {
			// Create a default profile if none exists
			defaultRules := `{"minTraceWidthMM":0.15,"minClearanceMM":0.15,"minDrillDiamMM":0.3,"maxDrillDiamMM":6.3,"minAnnularRingMM":0.15,"maxAspectRatio":10,"minSolderMaskDamMM":0.1,"minEdgeClearanceMM":0.3}`
			profile = db.CapabilityProfile{
				ID:        uuid.New().String(),
				OrgID:     user.OrgID,
				Name:      "Default",
				IsDefault: true,
				Rules:     []byte(defaultRules),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			h.db.Create(&profile)
		}
		req.ProfileID = profile.ID
	}

	job := db.AnalysisJob{
		ID:           uuid.New().String(),
		OrgID:        user.OrgID,
		SubmissionID: submissionID,
		ProfileID:    req.ProfileID,
		Status:       "PENDING",
	}
	if err := h.db.Create(&job).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Update submission status
	h.db.Model(&submission).Update("status", "ANALYZING")

	// Enqueue SQS message
	if err := h.aws.EnqueueJob(c.Request().Context(), job.ID); err != nil {
		// Don't fail the request if SQS is unavailable in dev
		log.Printf("ERROR: SQS enqueue failed for job %s: %v", job.ID, err)
	} else {
		log.Printf("SQS enqueue succeeded for job %s", job.ID)
	}

	lib.Track("Analysis Requested", user.OrgID, map[string]any{"orgId": user.OrgID, "submissionId": submissionID, "profileId": req.ProfileID})

	return c.JSON(http.StatusCreated, job)
}
