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

type BatchesHandler struct {
	db  *gorm.DB
	aws *lib.AWSClients
}

func NewBatchesHandler(database *gorm.DB, aws *lib.AWSClients) *BatchesHandler {
	return &BatchesHandler{db: database, aws: aws}
}

const maxBatchFiles = 50

// CreateBatch POST /batches
func (h *BatchesHandler) CreateBatch(c echo.Context) error {
	user := lib.GetUser(c)

	var req struct {
		ProjectID *string `json:"projectId"`
		ProfileID *string `json:"profileId"`
		Files     []struct {
			Filename string `json:"filename"`
			FileType string `json:"fileType"`
		} `json:"files"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if len(req.Files) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one file required")
	}
	if len(req.Files) > maxBatchFiles {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("maximum %d files per batch", maxBatchFiles))
	}

	// Validate all files
	for i, f := range req.Files {
		if f.Filename == "" {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("files[%d]: filename required", i))
		}
		if f.FileType != "GERBER" && f.FileType != "ODB_PLUS_PLUS" {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("files[%d]: fileType must be GERBER or ODB_PLUS_PLUS", i))
		}
	}

	batchID := uuid.New().String()
	now := time.Now()

	batch := db.Batch{
		ID:        batchID,
		OrgID:     user.OrgID,
		ProjectID: req.ProjectID,
		UserID:    user.Sub,
		ProfileID: req.ProfileID,
		Status:    "PENDING",
		Total:     len(req.Files),
		Completed: 0,
		Failed:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	type submissionOut struct {
		SubmissionID string `json:"submissionId"`
		Filename     string `json:"filename"`
		PresignedURL string `json:"presignedUrl"`
	}

	var submissions []db.Submission
	var outputs []submissionOut

	for _, f := range req.Files {
		subID := uuid.New().String()
		ext := strings.ToLower(filepath.Ext(f.Filename))
		if ext == "" {
			ext = ".zip"
		}
		fileKey := fmt.Sprintf("submissions/%s/%s%s", user.OrgID, subID, ext)

		submissions = append(submissions, db.Submission{
			ID:        subID,
			OrgID:     user.OrgID,
			UserID:    user.Sub,
			BatchID:   &batchID,
			Filename:  f.Filename,
			FileType:  f.FileType,
			FileKey:   fileKey,
			Status:    "UPLOADED",
			CreatedAt: now,
		})

		contentType := "application/zip"
		presignedURL, err := h.aws.PresignPutURL(c.Request().Context(), fileKey, contentType)
		if err != nil {
			presignedURL = ""
		}

		outputs = append(outputs, submissionOut{
			SubmissionID: subID,
			Filename:     f.Filename,
			PresignedURL: presignedURL,
		})
	}

	// Create batch + submissions in a transaction
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		if err := tx.Create(&submissions).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("Batch Created", user.OrgID, map[string]any{"orgId": user.OrgID, "fileCount": len(req.Files), "projectId": req.ProjectID})

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"batchId":     batchID,
		"submissions": outputs,
	})
}

// GetBatch GET /batches/:id
func (h *BatchesHandler) GetBatch(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var batch db.Batch
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&batch).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "batch not found")
	}

	// Fetch submissions in this batch
	var submissions []db.Submission
	h.db.Where("batch_id = ?", id).Order("created_at asc").Find(&submissions)

	// Fetch latest job for each submission
	subIDs := make([]string, len(submissions))
	for i, s := range submissions {
		subIDs[i] = s.ID
	}

	type jobRow struct {
		ID           string
		SubmissionID string
		Status       string
		MfgScore     int
		MfgGrade     string
	}
	var jobs []jobRow
	if len(subIDs) > 0 {
		h.db.Raw(`SELECT DISTINCT ON (submission_id) id, submission_id, status, mfg_score, mfg_grade
			FROM analysis_jobs WHERE submission_id IN ? ORDER BY submission_id, created_at DESC`, subIDs).Scan(&jobs)
	}
	jobMap := map[string]jobRow{}
	for _, j := range jobs {
		jobMap[j.SubmissionID] = j
	}

	type submissionWithJob struct {
		db.Submission
		LatestJobID string `json:"latestJobId"`
		JobStatus   string `json:"jobStatus"`
		MfgScore    int    `json:"mfgScore"`
		MfgGrade    string `json:"mfgGrade"`
	}

	var subsOut []submissionWithJob
	var totalScore int
	var scoredCount int
	for _, s := range submissions {
		j := jobMap[s.ID]
		subsOut = append(subsOut, submissionWithJob{
			Submission:  s,
			LatestJobID: j.ID,
			JobStatus:   j.Status,
			MfgScore:    j.MfgScore,
			MfgGrade:    j.MfgGrade,
		})
		if j.Status == "DONE" && j.MfgScore > 0 {
			totalScore += j.MfgScore
			scoredCount++
		}
	}

	var avgScore *float64
	if scoredCount > 0 {
		avg := float64(totalScore) / float64(scoredCount)
		avgScore = &avg
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"batch":       batch,
		"submissions": subsOut,
		"avgScore":    avgScore,
	})
}

// AnalyzeBatch POST /batches/:id/analyze
func (h *BatchesHandler) AnalyzeBatch(c echo.Context) error {
	user := lib.GetUser(c)
	batchID := c.Param("id")

	var req struct {
		ProfileID string `json:"profileId"`
	}
	_ = c.Bind(&req)

	var batch db.Batch
	if err := h.db.Where("id = ? AND org_id = ?", batchID, user.OrgID).First(&batch).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "batch not found")
	}

	// Use profile from request, then batch, then default
	profileID := req.ProfileID
	if profileID == "" && batch.ProfileID != nil {
		profileID = *batch.ProfileID
	}
	if profileID == "" {
		var profile db.CapabilityProfile
		if err := h.db.Where("org_id = ? AND is_default = ?", user.OrgID, true).First(&profile).Error; err != nil {
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
		profileID = profile.ID
	}

	// Find all UPLOADED submissions in this batch
	var submissions []db.Submission
	h.db.Where("batch_id = ? AND status = ?", batchID, "UPLOADED").Find(&submissions)

	if len(submissions) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no submissions ready for analysis")
	}

	// Update batch status to PROCESSING
	h.db.Model(&batch).Update("status", "PROCESSING")

	var jobIDs []string
	for _, sub := range submissions {
		job := db.AnalysisJob{
			ID:           uuid.New().String(),
			OrgID:        user.OrgID,
			SubmissionID: sub.ID,
			ProfileID:    profileID,
			Status:       "PENDING",
		}
		if err := h.db.Create(&job).Error; err != nil {
			log.Printf("ERROR: failed to create job for submission %s: %v", sub.ID, err)
			continue
		}

		// Update submission status
		h.db.Model(&sub).Update("status", "ANALYZING")

		// Enqueue SQS message
		if err := h.aws.EnqueueJob(c.Request().Context(), job.ID); err != nil {
			log.Printf("ERROR: SQS enqueue failed for job %s: %v", job.ID, err)
		} else {
			log.Printf("SQS enqueue succeeded for job %s (batch %s)", job.ID, batchID)
		}
		jobIDs = append(jobIDs, job.ID)
	}

	lib.Track("Batch Requested", user.OrgID, map[string]any{"orgId": user.OrgID, "batchId": batchID})

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"batchId": batchID,
		"jobIds":  jobIDs,
	})
}

// RetryBatch POST /batches/:id/retry
func (h *BatchesHandler) RetryBatch(c echo.Context) error {
	user := lib.GetUser(c)
	batchID := c.Param("id")

	var batch db.Batch
	if err := h.db.Where("id = ? AND org_id = ?", batchID, user.OrgID).First(&batch).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "batch not found")
	}

	// Find FAILED submissions in this batch
	var submissions []db.Submission
	h.db.Where("batch_id = ? AND status = ?", batchID, "FAILED").Find(&submissions)

	if len(submissions) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "no failed submissions to retry")
	}

	// Use batch profile or default
	profileID := ""
	if batch.ProfileID != nil {
		profileID = *batch.ProfileID
	}
	if profileID == "" {
		var profile db.CapabilityProfile
		if err := h.db.Where("org_id = ? AND is_default = ?", user.OrgID, true).First(&profile).Error; err == nil {
			profileID = profile.ID
		}
	}

	// Reset batch counters for retried submissions
	h.db.Model(&batch).Updates(map[string]interface{}{
		"status":  "PROCESSING",
		"failed":  gorm.Expr("failed - ?", len(submissions)),
	})

	var jobIDs []string
	for _, sub := range submissions {
		job := db.AnalysisJob{
			ID:           uuid.New().String(),
			OrgID:        user.OrgID,
			SubmissionID: sub.ID,
			ProfileID:    profileID,
			Status:       "PENDING",
		}
		if err := h.db.Create(&job).Error; err != nil {
			log.Printf("ERROR: failed to create retry job for submission %s: %v", sub.ID, err)
			continue
		}

		h.db.Model(&sub).Update("status", "ANALYZING")

		if err := h.aws.EnqueueJob(c.Request().Context(), job.ID); err != nil {
			log.Printf("ERROR: SQS enqueue failed for retry job %s: %v", job.ID, err)
		}
		jobIDs = append(jobIDs, job.ID)
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"batchId": batchID,
		"jobIds":  jobIDs,
	})
}
