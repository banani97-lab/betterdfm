package routes

import (
	"net/http"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ContactHandler struct {
	db *gorm.DB
}

func NewContactHandler(database *gorm.DB) *ContactHandler {
	return &ContactHandler{db: database}
}

// SubmitContact POST /contact — public, no auth
func (h *ContactHandler) SubmitContact(c echo.Context) error {
	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Company string `json:"company"`
		Message string `json:"message"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" || req.Email == "" || req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name, email, and message are required")
	}

	sub := db.ContactSubmission{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Company:   req.Company,
		Message:   req.Message,
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&sub).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusCreated, map[string]any{"ok": true, "id": sub.ID})
}

// ListContacts GET /admin/contacts — admin only
func (h *ContactHandler) ListContacts(c echo.Context) error {
	var subs []db.ContactSubmission
	if err := h.db.Order("created_at desc").Limit(100).Find(&subs).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, subs)
}
