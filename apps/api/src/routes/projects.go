package routes

import (
	"net/http"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ProjectsHandler struct {
	db  *gorm.DB
	aws *lib.AWSClients
}

func NewProjectsHandler(database *gorm.DB, aws *lib.AWSClients) *ProjectsHandler {
	return &ProjectsHandler{db: database, aws: aws}
}

type projectResponse struct {
	db.Project
	SubmissionCount int     `json:"submissionCount"`
	AvgScore        float64 `json:"avgScore"`
	LatestGrade     string  `json:"latestGrade"`
	LastActivityAt  *string `json:"lastActivityAt"`
}

// ListProjects GET /projects
func (h *ProjectsHandler) ListProjects(c echo.Context) error {
	user := lib.GetUser(c)
	q := c.QueryParam("q")
	archived := c.QueryParam("archived")

	query := h.db.Where("org_id = ?", user.OrgID)
	if archived == "" || archived == "false" {
		query = query.Where("archived = ?", false)
	} else if archived == "true" {
		query = query.Where("archived = ?", true)
	}
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("(name ILIKE ? OR description ILIKE ? OR customer_ref ILIKE ?)", like, like, like)
	}

	var projects []db.Project
	if err := query.Order("created_at desc").Find(&projects).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if len(projects) == 0 {
		return c.JSON(http.StatusOK, []projectResponse{})
	}

	// Gather project IDs
	ids := make([]string, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}

	// Aggregated stats per project via subquery
	type statsRow struct {
		ProjectID       string
		SubmissionCount int
		AvgScore        float64
		LatestGrade     string
		LastActivityAt  *time.Time
	}
	var stats []statsRow
	h.db.Raw(`
		SELECT
			s.project_id,
			COUNT(s.id) AS submission_count,
			COALESCE(AVG(NULLIF(j.mfg_score, 0)), 0) AS avg_score,
			COALESCE(
				(SELECT j2.mfg_grade FROM analysis_jobs j2
				 WHERE j2.submission_id = (
					SELECT s2.id FROM submissions s2 WHERE s2.project_id = s.project_id
					ORDER BY s2.created_at DESC LIMIT 1
				 )
				 ORDER BY j2.created_at DESC LIMIT 1
				), '') AS latest_grade,
			MAX(s.created_at) AS last_activity_at
		FROM submissions s
		LEFT JOIN analysis_jobs j ON j.submission_id = s.id
		WHERE s.project_id IN ?
		GROUP BY s.project_id
	`, ids).Scan(&stats)

	statsMap := map[string]statsRow{}
	for _, s := range stats {
		statsMap[s.ProjectID] = s
	}

	result := make([]projectResponse, len(projects))
	for i, p := range projects {
		s := statsMap[p.ID]
		var lastActivity *string
		if s.LastActivityAt != nil {
			t := s.LastActivityAt.Format(time.RFC3339)
			lastActivity = &t
		}
		result[i] = projectResponse{
			Project:         p,
			SubmissionCount: s.SubmissionCount,
			AvgScore:        s.AvgScore,
			LatestGrade:     s.LatestGrade,
			LastActivityAt:  lastActivity,
		}
	}
	return c.JSON(http.StatusOK, result)
}

// GetProject GET /projects/:id
func (h *ProjectsHandler) GetProject(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var project db.Project
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	// Stats
	type statsRow struct {
		SubmissionCount int
		AvgScore        float64
		LatestGrade     string
		LastActivityAt  *time.Time
	}
	var s statsRow
	h.db.Raw(`
		SELECT
			COUNT(s.id) AS submission_count,
			COALESCE(AVG(NULLIF(j.mfg_score, 0)), 0) AS avg_score,
			COALESCE(
				(SELECT j2.mfg_grade FROM analysis_jobs j2
				 WHERE j2.submission_id = (
					SELECT s2.id FROM submissions s2 WHERE s2.project_id = ?
					ORDER BY s2.created_at DESC LIMIT 1
				 )
				 ORDER BY j2.created_at DESC LIMIT 1
				), '') AS latest_grade,
			MAX(s.created_at) AS last_activity_at
		FROM submissions s
		LEFT JOIN analysis_jobs j ON j.submission_id = s.id
		WHERE s.project_id = ?
	`, id, id).Scan(&s)

	var lastActivity *string
	if s.LastActivityAt != nil {
		t := s.LastActivityAt.Format(time.RFC3339)
		lastActivity = &t
	}

	return c.JSON(http.StatusOK, projectResponse{
		Project:         project,
		SubmissionCount: s.SubmissionCount,
		AvgScore:        s.AvgScore,
		LatestGrade:     s.LatestGrade,
		LastActivityAt:  lastActivity,
	})
}

// CreateProject POST /projects
func (h *ProjectsHandler) CreateProject(c echo.Context) error {
	user := lib.GetUser(c)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		CustomerRef string `json:"customerRef"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}

	now := time.Now()
	project := db.Project{
		ID:          uuid.New().String(),
		OrgID:       user.OrgID,
		Name:        req.Name,
		Description: req.Description,
		CustomerRef: req.CustomerRef,
		CreatedBy:   user.Sub,
		Archived:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.db.Create(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, project)
}

// UpdateProject PUT /projects/:id
func (h *ProjectsHandler) UpdateProject(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var project db.Project
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		CustomerRef *string `json:"customerRef"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Name != nil {
		if *req.Name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "name cannot be empty")
		}
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.CustomerRef != nil {
		project.CustomerRef = *req.CustomerRef
	}
	project.UpdatedAt = time.Now()

	if err := h.db.Save(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, project)
}

// ArchiveProject DELETE /projects/:id
func (h *ProjectsHandler) ArchiveProject(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var project db.Project
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	project.Archived = true
	project.UpdatedAt = time.Now()
	if err := h.db.Save(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, project)
}

// ListProjectSubmissions GET /projects/:id/submissions
func (h *ProjectsHandler) ListProjectSubmissions(c echo.Context) error {
	user := lib.GetUser(c)
	projectID := c.Param("id")

	// Verify project belongs to org
	var project db.Project
	if err := h.db.Where("id = ? AND org_id = ?", projectID, user.OrgID).First(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	var submissions []db.Submission
	if err := h.db.Where("project_id = ? AND org_id = ?", projectID, user.OrgID).Order("created_at desc").Find(&submissions).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Fetch latest job info (same pattern as ListSubmissions)
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

// MoveSubmissionToProject POST /projects/:id/submissions
func (h *ProjectsHandler) MoveSubmissionToProject(c echo.Context) error {
	user := lib.GetUser(c)
	projectID := c.Param("id")

	// Verify project belongs to org
	var project db.Project
	if err := h.db.Where("id = ? AND org_id = ?", projectID, user.OrgID).First(&project).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}

	var req struct {
		SubmissionID string `json:"submissionId"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.SubmissionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "submissionId required")
	}

	var submission db.Submission
	if err := h.db.Where("id = ? AND org_id = ?", req.SubmissionID, user.OrgID).First(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "submission not found")
	}

	submission.ProjectID = &projectID
	if err := h.db.Save(&submission).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, submission)
}
