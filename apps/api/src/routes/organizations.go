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
