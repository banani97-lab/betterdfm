package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(database *gorm.DB) *AuthHandler {
	return &AuthHandler{db: database}
}

// Me GET /auth/me
func (h *AuthHandler) Me(c echo.Context) error {
	user := lib.GetUser(c)
	return c.JSON(http.StatusOK, map[string]string{
		"sub":   user.Sub,
		"email": user.Email,
		"orgId": user.OrgID,
		"role":  user.Role,
	})
}

// Callback POST /auth/callback — exchange Cognito auth code for tokens
func (h *AuthHandler) Callback(c echo.Context) error {
	var req struct {
		Code        string `json:"code"`
		RedirectURI string `json:"redirectUri"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "code required")
	}

	cognitoDomain := os.Getenv("COGNITO_DOMAIN")
	clientID := os.Getenv("COGNITO_CLIENT_ID")
	if cognitoDomain == "" || clientID == "" {
		// Dev mode: return a mock token response
		return c.JSON(http.StatusOK, map[string]string{
			"accessToken": "dev-token",
			"idToken":     "dev-id-token",
		})
	}

	tokenURL := cognitoDomain + "/oauth2/token"
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {req.Code},
		"redirect_uri": {req.RedirectURI},
		"client_id":    {clientID},
	}
	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", //nolint:noctx
		strings.NewReader(form.Encode()))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("token exchange failed: %v", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp map[string]interface{}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid token response")
	}

	idToken, _ := tokenResp["id_token"].(string)
	if idToken != "" {
		// Upsert user in DB — in production, parse JWT claims here
		// For MVP we rely on the client sending the token in subsequent requests
		_ = h.upsertUser(idToken)
	}

	return c.JSON(http.StatusOK, tokenResp)
}

func (h *AuthHandler) upsertUser(idToken string) error {
	// Simplified: in production parse the JWT to get sub/email
	// For now just ensure a default org exists
	var org db.Organization
	if err := h.db.Where("slug = ?", "default").First(&org).Error; err != nil {
		org = db.Organization{
			ID:        uuid.New().String(),
			Slug:      "default",
			Name:      "Default Organization",
			CreatedAt: time.Now(),
		}
		h.db.Create(&org)
	}
	_ = idToken
	return nil
}
