package routes

import (
	"net/http"

	"github.com/betterdfm/api/src/lib"
	"github.com/labstack/echo/v4"
)

type UsageHandler struct {
	quota *lib.QuotaService
}

func NewUsageHandler(quota *lib.QuotaService) *UsageHandler {
	return &UsageHandler{quota: quota}
}

// GetUsage GET /usage — returns usage summary for the authenticated user's org.
func (h *UsageHandler) GetUsage(c echo.Context) error {
	user := lib.GetUser(c)
	summary := h.quota.GetUsageSummary(user.OrgID)
	return c.JSON(http.StatusOK, summary)
}
