package routes

import (
	"errors"
	"net/http"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OrgStats is the response for GET /admin/organizations/:id/stats
type OrgStats struct {
	TotalJobs        int64              `json:"totalJobs"`
	JobsByStatus     map[string]int64   `json:"jobsByStatus"`
	TotalSubmissions int64              `json:"totalSubmissions"`
	TotalUsers       int64              `json:"totalUsers"`
	TotalViolations  int64              `json:"totalViolations"`
	ViolationsBySev  map[string]int64   `json:"violationsBySeverity"`
	TopRules         []RuleCount        `json:"topRules"`
	AvgScore         float64            `json:"avgScore"`
	GradeDistribution map[string]int64  `json:"gradeDistribution"`
}

// PlatformStats is the response for GET /admin/stats
type PlatformStats struct {
	TotalOrgs        int64              `json:"totalOrgs"`
	TotalJobs        int64              `json:"totalJobs"`
	JobsByStatus     map[string]int64   `json:"jobsByStatus"`
	TotalSubmissions int64              `json:"totalSubmissions"`
	TotalUsers       int64              `json:"totalUsers"`
	TotalViolations  int64              `json:"totalViolations"`
	ViolationsBySev  map[string]int64   `json:"violationsBySeverity"`
	TopRules         []RuleCount        `json:"topRules"`
	AvgScore         float64            `json:"avgScore"`
	GradeDistribution map[string]int64  `json:"gradeDistribution"`
}

type RuleCount struct {
	RuleID string `json:"ruleId"`
	Count  int64  `json:"count"`
}

type AdminOrgHandler struct {
	db *gorm.DB
}

func NewAdminOrgHandler(database *gorm.DB) *AdminOrgHandler {
	return &AdminOrgHandler{db: database}
}

// ListOrganizations GET /admin/organizations
func (h *AdminOrgHandler) ListOrganizations(c echo.Context) error {
	var orgs []db.Organization
	if err := h.db.Order("created_at desc").Find(&orgs).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, orgs)
}

// CreateOrganization POST /admin/organizations
func (h *AdminOrgHandler) CreateOrganization(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" || req.Slug == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and slug are required")
	}

	// Check slug uniqueness
	var existing db.Organization
	if err := h.db.Where("slug = ?", req.Slug).First(&existing).Error; err == nil {
		return echo.NewHTTPError(http.StatusConflict, "slug already in use")
	}

	org := db.Organization{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Slug:      req.Slug,
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&org).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, org)
}

// GetOrganization GET /admin/organizations/:id
func (h *AdminOrgHandler) GetOrganization(c echo.Context) error {
	id := c.Param("id")
	var org db.Organization
	if err := h.db.First(&org, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "organization not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, org)
}

// UpdateOrganization PUT /admin/organizations/:id
func (h *AdminOrgHandler) UpdateOrganization(c echo.Context) error {
	id := c.Param("id")
	var org db.Organization
	if err := h.db.First(&org, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "organization not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	var req struct {
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		LogoURL string `json:"logoUrl"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Slug != "" {
		// Check slug uniqueness (excluding self)
		var existing db.Organization
		if err := h.db.Where("slug = ? AND id != ?", req.Slug, id).First(&existing).Error; err == nil {
			return echo.NewHTTPError(http.StatusConflict, "slug already in use")
		}
		updates["slug"] = req.Slug
	}
	if req.LogoURL != "" {
		updates["logo_url"] = req.LogoURL
	}

	if len(updates) > 0 {
		if err := h.db.Model(&org).Updates(updates).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	h.db.First(&org, "id = ?", id)
	return c.JSON(http.StatusOK, org)
}

// GetOrganizationStats GET /admin/organizations/:id/stats
func (h *AdminOrgHandler) GetOrganizationStats(c echo.Context) error {
	orgID := c.Param("id")

	// Verify org exists
	var org db.Organization
	if err := h.db.First(&org, "id = ?", orgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "organization not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	stats := OrgStats{
		JobsByStatus:      make(map[string]int64),
		ViolationsBySev:   make(map[string]int64),
		GradeDistribution: make(map[string]int64),
	}

	h.db.Model(&db.AnalysisJob{}).Where("org_id = ?", orgID).Count(&stats.TotalJobs)
	h.db.Model(&db.Submission{}).Where("org_id = ?", orgID).Count(&stats.TotalSubmissions)
	h.db.Model(&db.User{}).Where("org_id = ?", orgID).Count(&stats.TotalUsers)
	h.db.Model(&db.Violation{}).Where("org_id = ?", orgID).Count(&stats.TotalViolations)

	// Jobs by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	h.db.Model(&db.AnalysisJob{}).Select("status, count(*) as count").Where("org_id = ?", orgID).Group("status").Scan(&statusCounts)
	for _, sc := range statusCounts {
		stats.JobsByStatus[sc.Status] = sc.Count
	}

	// Violations by severity
	var sevCounts []struct {
		Severity string
		Count    int64
	}
	h.db.Model(&db.Violation{}).Select("severity, count(*) as count").Where("org_id = ?", orgID).Group("severity").Scan(&sevCounts)
	for _, sc := range sevCounts {
		stats.ViolationsBySev[sc.Severity] = sc.Count
	}

	// Top violated rules
	h.db.Model(&db.Violation{}).
		Select("rule_id, count(*) as count").
		Where("org_id = ?", orgID).
		Group("rule_id").
		Order("count desc").
		Limit(10).
		Scan(&stats.TopRules)

	// Average score (completed jobs only)
	h.db.Model(&db.AnalysisJob{}).
		Select("coalesce(avg(mfg_score), 0)").
		Where("org_id = ? AND status = ?", orgID, "DONE").
		Scan(&stats.AvgScore)

	// Grade distribution
	var gradeCounts []struct {
		MfgGrade string
		Count    int64
	}
	h.db.Model(&db.AnalysisJob{}).Select("mfg_grade, count(*) as count").Where("org_id = ? AND status = ? AND mfg_grade != ''", orgID, "DONE").Group("mfg_grade").Scan(&gradeCounts)
	for _, gc := range gradeCounts {
		stats.GradeDistribution[gc.MfgGrade] = gc.Count
	}

	return c.JSON(http.StatusOK, stats)
}

// GetPlatformStats GET /admin/stats
func (h *AdminOrgHandler) GetPlatformStats(c echo.Context) error {
	stats := PlatformStats{
		JobsByStatus:      make(map[string]int64),
		ViolationsBySev:   make(map[string]int64),
		GradeDistribution: make(map[string]int64),
	}

	h.db.Model(&db.Organization{}).Count(&stats.TotalOrgs)
	h.db.Model(&db.AnalysisJob{}).Count(&stats.TotalJobs)
	h.db.Model(&db.Submission{}).Count(&stats.TotalSubmissions)
	h.db.Model(&db.User{}).Count(&stats.TotalUsers)
	h.db.Model(&db.Violation{}).Count(&stats.TotalViolations)

	// Jobs by status
	var statusCounts []struct {
		Status string
		Count  int64
	}
	h.db.Model(&db.AnalysisJob{}).Select("status, count(*) as count").Group("status").Scan(&statusCounts)
	for _, sc := range statusCounts {
		stats.JobsByStatus[sc.Status] = sc.Count
	}

	// Violations by severity
	var sevCounts []struct {
		Severity string
		Count    int64
	}
	h.db.Model(&db.Violation{}).Select("severity, count(*) as count").Group("severity").Scan(&sevCounts)
	for _, sc := range sevCounts {
		stats.ViolationsBySev[sc.Severity] = sc.Count
	}

	// Top violated rules
	h.db.Model(&db.Violation{}).
		Select("rule_id, count(*) as count").
		Group("rule_id").
		Order("count desc").
		Limit(10).
		Scan(&stats.TopRules)

	// Average score
	h.db.Model(&db.AnalysisJob{}).
		Select("coalesce(avg(mfg_score), 0)").
		Where("status = ?", "DONE").
		Scan(&stats.AvgScore)

	// Grade distribution
	var gradeCounts []struct {
		MfgGrade string
		Count    int64
	}
	h.db.Model(&db.AnalysisJob{}).Select("mfg_grade, count(*) as count").Where("status = ? AND mfg_grade != ''", "DONE").Group("mfg_grade").Scan(&gradeCounts)
	for _, gc := range gradeCounts {
		stats.GradeDistribution[gc.MfgGrade] = gc.Count
	}

	return c.JSON(http.StatusOK, stats)
}
