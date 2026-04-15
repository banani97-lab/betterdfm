package routes

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
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

type ShareHandler struct {
	db    *gorm.DB
	aws   *lib.AWSClients
	quota *lib.QuotaService
}

func NewShareHandler(database *gorm.DB, aws *lib.AWSClients, quota *lib.QuotaService) *ShareHandler {
	return &ShareHandler{db: database, aws: aws, quota: quota}
}

// ── Authenticated routes (CM-side) ─────────────────────────────────────────

// CreateShareLink POST /share-links
func (h *ShareHandler) CreateShareLink(c echo.Context) error {
	user := lib.GetUser(c)

	// Check share links feature enabled for tier
	if !h.quota.CheckFeatureEnabled(user.OrgID, "share_links") {
		return echo.NewHTTPError(http.StatusForbidden, map[string]string{
			"message": "Upgrade required: share links are not available on your current plan.",
		})
	}

	// Check resource limit for active share links
	check := h.quota.CheckResourceLimitWithTier(user.OrgID, "share_links")
	if !check.Allowed {
		return echo.NewHTTPError(http.StatusForbidden, map[string]string{"message": check.Message})
	}

	var req struct {
		ProjectID   *string    `json:"projectId"`
		JobID       *string    `json:"jobId"`
		Label       string     `json:"label"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		AllowUpload bool       `json:"allowUpload"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Label == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "label is required")
	}

	// Generate 32-byte crypto-random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	link := db.ShareLink{
		ID:          uuid.New().String(),
		OrgID:       user.OrgID,
		Token:       token,
		ProjectID:   req.ProjectID,
		JobID:       req.JobID,
		CreatedBy:   user.Sub,
		ExpiresAt:   req.ExpiresAt,
		AllowUpload: req.AllowUpload,
		Active:      true,
		Label:       req.Label,
		CreatedAt:   time.Now(),
	}
	if err := h.db.Create(&link).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	shareType := "job"
	if req.ProjectID != nil {
		shareType = "project"
	}
	lib.Track("ShareLink Created", user.OrgID, map[string]any{"orgId": user.OrgID, "shareType": shareType, "allowUpload": req.AllowUpload, "hasExpiry": req.ExpiresAt != nil})

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"id":          link.ID,
		"token":       link.Token,
		"label":       link.Label,
		"projectId":   link.ProjectID,
		"jobId":       link.JobID,
		"expiresAt":   link.ExpiresAt,
		"allowUpload": link.AllowUpload,
		"active":      link.Active,
		"createdAt":   link.CreatedAt,
		"shareUrl":    fmt.Sprintf("/share/%s", link.Token),
	})
}

// ListShareLinks GET /share-links
func (h *ShareHandler) ListShareLinks(c echo.Context) error {
	user := lib.GetUser(c)
	var links []db.ShareLink
	if err := h.db.Where("org_id = ?", user.OrgID).Order("created_at desc").Find(&links).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, links)
}

// DeactivateShareLink DELETE /share-links/:id
func (h *ShareHandler) DeactivateShareLink(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var link db.ShareLink
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "share link not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if err := h.db.Model(&link).Update("active", false).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("ShareLink Deleted", user.OrgID, map[string]any{"orgId": user.OrgID})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":     link.ID,
		"active": false,
	})
}

// ListShareUploads GET /share-links/:id/uploads
func (h *ShareHandler) ListShareUploads(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	// Verify share link belongs to user's org
	var link db.ShareLink
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&link).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "share link not found")
	}

	var uploads []db.ShareUpload
	if err := h.db.Where("share_link_id = ?", id).Order("created_at desc").Find(&uploads).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, uploads)
}

// ── Token middleware ────────────────────────────────────────────────────────

// TokenMiddleware validates :token param and puts resolved ShareLink into context.
func (h *ShareHandler) TokenMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Param("token")
			if token == "" {
				return echo.NewHTTPError(http.StatusNotFound, "not found")
			}

			var link db.ShareLink
			if err := h.db.Where("token = ? AND active = ?", token, true).First(&link).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return echo.NewHTTPError(http.StatusNotFound, "share link not found")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}

			// Check expiration
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return echo.NewHTTPError(http.StatusGone, "share link has expired")
			}

			c.Set("shareLink", &link)
			return next(c)
		}
	}
}

// getShareLink retrieves the resolved share link from the echo context.
func getShareLink(c echo.Context) *db.ShareLink {
	sl, _ := c.Get("shareLink").(*db.ShareLink)
	return sl
}

// ── Public routes (customer-side) ──────────────────────────────────────────

// GetShareInfo GET /shared/:token
func (h *ShareHandler) GetShareInfo(c echo.Context) error {
	link := getShareLink(c)

	// Get org branding
	var org db.Organization
	h.db.First(&org, "id = ?", link.OrgID)

	result := map[string]interface{}{
		"id":          link.ID,
		"label":       link.Label,
		"allowUpload": link.AllowUpload,
		"expiresAt":   link.ExpiresAt,
		"orgName":     org.Name,
		"orgLogoUrl":  org.LogoURL,
	}

	if link.ProjectID != nil {
		result["shareType"] = "project"
		result["projectId"] = *link.ProjectID
	} else if link.JobID != nil {
		result["shareType"] = "job"
		result["jobId"] = *link.JobID

		// Include job info directly
		var job db.AnalysisJob
		if err := h.db.First(&job, "id = ? AND org_id = ?", *link.JobID, link.OrgID).Error; err == nil {
			result["job"] = map[string]interface{}{
				"id":          job.ID,
				"status":      job.Status,
				"mfgScore":    job.MfgScore,
				"mfgGrade":    job.MfgGrade,
				"completedAt": job.CompletedAt,
			}
		}
	}

	infoShareType := "job"
	if link.ProjectID != nil {
		infoShareType = "project"
	}
	lib.Track("SharePage Viewed", link.OrgID, map[string]any{"orgId": link.OrgID, "shareType": infoShareType})

	return c.JSON(http.StatusOK, result)
}

// GetSharedSubmissions GET /shared/:token/submissions
func (h *ShareHandler) GetSharedSubmissions(c echo.Context) error {
	link := getShareLink(c)
	if link.ProjectID == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "this share link does not include a project")
	}

	// For project shares, list only submissions belonging to this project
	var submissions []db.Submission
	if err := h.db.Where("org_id = ? AND project_id = ?", link.OrgID, *link.ProjectID).Order("created_at desc").Find(&submissions).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Fetch latest job info for each submission
	ids := make([]string, len(submissions))
	for i, s := range submissions {
		ids[i] = s.ID
	}

	type jobRow struct {
		ID           string
		SubmissionID string
		MfgScore     int
		MfgGrade     string
		Status       string
	}
	var jobs []jobRow
	if len(ids) > 0 {
		h.db.Raw("SELECT id, submission_id, mfg_score, mfg_grade, status FROM analysis_jobs WHERE submission_id IN ?", ids).Scan(&jobs)
	}

	type jobInfo struct {
		id       string
		mfgScore int
		mfgGrade string
		status   string
	}
	jobMap := map[string]jobInfo{}
	for _, j := range jobs {
		if _, exists := jobMap[j.SubmissionID]; !exists {
			jobMap[j.SubmissionID] = jobInfo{id: j.ID, mfgScore: j.MfgScore, mfgGrade: j.MfgGrade, status: j.Status}
		}
	}

	type subResponse struct {
		ID          string    `json:"id"`
		Filename    string    `json:"filename"`
		FileType    string    `json:"fileType"`
		Status      string    `json:"status"`
		CreatedAt   time.Time `json:"createdAt"`
		LatestJobID string    `json:"latestJobId"`
		MfgScore    int       `json:"mfgScore"`
		MfgGrade    string    `json:"mfgGrade"`
	}

	result := make([]subResponse, len(submissions))
	for i, s := range submissions {
		info := jobMap[s.ID]
		result[i] = subResponse{
			ID:          s.ID,
			Filename:    s.Filename,
			FileType:    s.FileType,
			Status:      s.Status,
			CreatedAt:   s.CreatedAt,
			LatestJobID: info.id,
			MfgScore:    info.mfgScore,
			MfgGrade:    info.mfgGrade,
		}
	}

	return c.JSON(http.StatusOK, result)
}

// verifyJobAccess checks that the requested jobId is allowed by the share link scope.
func (h *ShareHandler) verifyJobAccess(link *db.ShareLink, jobID string) (*db.AnalysisJob, error) {
	var job db.AnalysisJob
	if err := h.db.First(&job, "id = ? AND org_id = ?", jobID, link.OrgID).Error; err != nil {
		return nil, err
	}

	if link.JobID != nil {
		// Job share: must match exactly
		if *link.JobID != jobID {
			return nil, gorm.ErrRecordNotFound
		}
	} else if link.ProjectID != nil {
		// Project share: verify the job's submission belongs to the org
		var sub db.Submission
		if err := h.db.First(&sub, "id = ? AND org_id = ?", job.SubmissionID, link.OrgID).Error; err != nil {
			return nil, gorm.ErrRecordNotFound
		}
	}

	return &job, nil
}

// GetSharedJob GET /shared/:token/jobs/:jobId
func (h *ShareHandler) GetSharedJob(c echo.Context) error {
	link := getShareLink(c)
	jobID := c.Param("jobId")

	job, err := h.verifyJobAccess(link, jobID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	resp := map[string]interface{}{
		"id":           job.ID,
		"submissionId": job.SubmissionID,
		"status":       job.Status,
		"mfgScore":     job.MfgScore,
		"mfgGrade":     job.MfgGrade,
		"startedAt":    job.StartedAt,
		"completedAt":  job.CompletedAt,
	}
	if job.Status == "DONE" && h.aws != nil && h.aws.S3Presign != nil {
		ctx := c.Request().Context()
		if job.BoardDataKey != "" {
			if url, err := h.aws.PresignGetURL(ctx, job.BoardDataKey, 60*time.Minute); err == nil {
				resp["boardUrl"] = url
			}
		}
		if job.ViolationsKey != "" {
			if url, err := h.aws.PresignGetURL(ctx, job.ViolationsKey, 60*time.Minute); err == nil {
				resp["violationsUrl"] = url
			}
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// GetSharedViolations GET /shared/:token/jobs/:jobId/violations
func (h *ShareHandler) GetSharedViolations(c echo.Context) error {
	link := getShareLink(c)
	jobID := c.Param("jobId")

	job, err := h.verifyJobAccess(link, jobID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	if job.ViolationsKey != "" && h.aws != nil && h.aws.S3Presign != nil {
		url, err := h.aws.PresignGetURL(c.Request().Context(), job.ViolationsKey, 60*time.Minute)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate presigned URL")
		}
		return c.JSON(http.StatusOK, map[string]string{"url": url})
	}

	var violations []db.Violation
	if err := h.db.Where("job_id = ?", jobID).Find(&violations).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, violations)
}

// GetSharedBoardData GET /shared/:token/jobs/:jobId/board
func (h *ShareHandler) GetSharedBoardData(c echo.Context) error {
	link := getShareLink(c)
	jobID := c.Param("jobId")

	job, err := h.verifyJobAccess(link, jobID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "job not found")
	}

	if job.BoardDataKey != "" && h.aws != nil && h.aws.S3Presign != nil {
		url, err := h.aws.PresignGetURL(c.Request().Context(), job.BoardDataKey, 60*time.Minute)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate presigned URL")
		}
		return c.JSON(http.StatusOK, map[string]string{"url": url})
	}

	if len(job.BoardData) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "board data not available")
	}
	return c.JSONBlob(http.StatusOK, job.BoardData)
}

// SharedUpload POST /shared/:token/upload
func (h *ShareHandler) SharedUpload(c echo.Context) error {
	link := getShareLink(c)
	if !link.AllowUpload {
		return echo.NewHTTPError(http.StatusForbidden, "uploads are not allowed on this share link")
	}

	var req struct {
		Filename      string `json:"filename"`
		FileType      string `json:"fileType"`
		UploaderName  string `json:"uploaderName"`
		UploaderEmail string `json:"uploaderEmail"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Filename == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "filename is required")
	}
	if req.FileType != "ODB_PLUS_PLUS" {
		return echo.NewHTTPError(http.StatusBadRequest, "fileType must be ODB_PLUS_PLUS")
	}

	submissionID := uuid.New().String()
	ext := strings.ToLower(filepath.Ext(req.Filename))
	if ext == "" {
		ext = ".zip"
	}
	fileKey := fmt.Sprintf("submissions/%s/%s%s", link.OrgID, submissionID, ext)

	submission := db.Submission{
		ID:        submissionID,
		OrgID:     link.OrgID,
		UserID:    "shared:" + link.ID,
		ProjectID: link.ProjectID,
		Filename:  req.Filename,
		FileType:  req.FileType,
		FileKey:   fileKey,
		Status:    "UPLOADED",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Create share upload record
	shareUpload := db.ShareUpload{
		ID:            uuid.New().String(),
		ShareLinkID:   link.ID,
		SubmissionID:  submissionID,
		UploaderName:  req.UploaderName,
		UploaderEmail: req.UploaderEmail,
		CreatedAt:     time.Now(),
	}
	if err := h.db.Create(&shareUpload).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("SharePage Uploaded", link.OrgID, map[string]any{"orgId": link.OrgID, "fileType": req.FileType})

	contentType := "application/octet-stream"
	presignedURL, err := h.aws.PresignPutURL(c.Request().Context(), fileKey, contentType)
	if err != nil {
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

// SharedAnalyze POST /shared/:token/analyze/:submissionId
// Triggers analysis for a submission uploaded through a share link.
// Called by the frontend AFTER the S3 upload completes.
func (h *ShareHandler) SharedAnalyze(c echo.Context) error {
	link := getShareLink(c)
	if !link.AllowUpload {
		return echo.NewHTTPError(http.StatusForbidden, "uploads are not allowed on this share link")
	}

	submissionID := c.Param("submissionId")
	var submission db.Submission
	if err := h.db.Where("id = ? AND org_id = ?", submissionID, link.OrgID).First(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}
	if submission.Status != "UPLOADED" {
		return echo.NewHTTPError(http.StatusBadRequest, "submission already analyzed")
	}

	// Use org's default profile
	var profile db.CapabilityProfile
	if err := h.db.Where("org_id = ? AND is_default = ?", link.OrgID, true).First(&profile).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "no default capability profile configured")
	}

	job := db.AnalysisJob{
		ID:           uuid.New().String(),
		OrgID:        link.OrgID,
		SubmissionID: submissionID,
		ProfileID:    profile.ID,
		Status:       "PENDING",
	}
	if err := h.db.Create(&job).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	h.db.Model(&submission).Update("status", "ANALYZING")

	if err := h.aws.EnqueueJob(c.Request().Context(), job.ID); err != nil {
		log.Printf("WARNING: SQS enqueue failed for shared analyze job %s: %v", job.ID, err)
	}

	lib.Track("SharePage Requested", link.OrgID, map[string]any{"orgId": link.OrgID, "submissionId": submissionID})

	return c.JSON(http.StatusOK, map[string]string{
		"jobId": job.ID,
	})
}

// ── Rate limiting middleware for public routes ──────────────────────────────

// Note: For production, consider adding Echo's built-in rate limiter middleware
// to the shared routes group. Example:
//   import "github.com/labstack/echo/v4/middleware"
//   sharedGroup.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))

