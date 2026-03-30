package routes

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ProfilesHandler struct {
	db *gorm.DB
}

func NewProfilesHandler(database *gorm.DB) *ProfilesHandler {
	return &ProfilesHandler{db: database}
}

// ListProfiles GET /profiles
func (h *ProfilesHandler) ListProfiles(c echo.Context) error {
	user := lib.GetUser(c)
	var profiles []db.CapabilityProfile
	if err := h.db.Where("org_id = ?", user.OrgID).Order("created_at asc").Find(&profiles).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, profiles)
}

// CreateProfile POST /profiles
func (h *ProfilesHandler) CreateProfile(c echo.Context) error {
	user := lib.GetUser(c)
	var req struct {
		Name      string          `json:"name"`
		IsDefault bool            `json:"isDefault"`
		Rules     db.ProfileRules `json:"rules"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name required")
	}

	rulesJSON, err := json.Marshal(req.Rules)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// If this profile is default, unset all others
	if req.IsDefault {
		h.db.Model(&db.CapabilityProfile{}).Where("org_id = ?", user.OrgID).Update("is_default", false)
	}

	now := time.Now()
	profile := db.CapabilityProfile{
		ID:        uuid.New().String(),
		OrgID:     user.OrgID,
		Name:      req.Name,
		IsDefault: req.IsDefault,
		Rules:     rulesJSON,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.db.Create(&profile).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("Profile Created", user.OrgID, map[string]any{"orgId": user.OrgID, "profileName": req.Name})

	return c.JSON(http.StatusCreated, profile)
}

// GetProfile GET /profiles/:id
func (h *ProfilesHandler) GetProfile(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")
	var profile db.CapabilityProfile
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&profile).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}
	return c.JSON(http.StatusOK, profile)
}

// UpdateProfile PUT /profiles/:id
func (h *ProfilesHandler) UpdateProfile(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")

	var profile db.CapabilityProfile
	if err := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).First(&profile).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}

	var req struct {
		Name      *string          `json:"name"`
		IsDefault *bool            `json:"isDefault"`
		Rules     *db.ProfileRules `json:"rules"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if req.Name != nil {
		profile.Name = *req.Name
	}
	if req.IsDefault != nil {
		if *req.IsDefault {
			h.db.Model(&db.CapabilityProfile{}).Where("org_id = ? AND id != ?", user.OrgID, id).Update("is_default", false)
		}
		profile.IsDefault = *req.IsDefault
	}
	if req.Rules != nil {
		rulesJSON, err := json.Marshal(req.Rules)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		profile.Rules = rulesJSON
	}
	profile.UpdatedAt = time.Now()

	if err := h.db.Save(&profile).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	lib.Track("Profile Updated", user.OrgID, map[string]any{"orgId": user.OrgID, "profileName": profile.Name})

	return c.JSON(http.StatusOK, profile)
}

// DeleteProfile DELETE /profiles/:id
func (h *ProfilesHandler) DeleteProfile(c echo.Context) error {
	user := lib.GetUser(c)
	id := c.Param("id")
	result := h.db.Where("id = ? AND org_id = ?", id, user.OrgID).Delete(&db.CapabilityProfile{})
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}

	lib.Track("Profile Deleted", user.OrgID, map[string]any{"orgId": user.OrgID})

	return c.NoContent(http.StatusNoContent)
}
