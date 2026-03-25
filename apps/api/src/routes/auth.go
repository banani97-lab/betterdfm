package routes

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

// Me GET /auth/me — returns current user info and upserts the user record
func (h *AuthHandler) Me(c echo.Context) error {
	user := lib.GetUser(c)

	// Upsert user record in DB
	h.upsertUserFromClaims(user)

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
		h.upsertUserFromIDToken(idToken)
	}

	return c.JSON(http.StatusOK, tokenResp)
}

// upsertUserFromClaims creates or updates a User row from parsed JWT claims.
func (h *AuthHandler) upsertUserFromClaims(claims *lib.UserClaims) {
	if claims.Sub == "" || claims.Sub == "dev-user" {
		return
	}
	user := db.User{
		ID:         uuid.New().String(),
		OrgID:      claims.OrgID,
		CognitoSub: claims.Sub,
		Email:      claims.Email,
		Role:       claims.Role,
		CreatedAt:  time.Now(),
	}
	result := h.db.Where("cognito_sub = ?", claims.Sub).First(&db.User{})
	if result.Error != nil {
		// User doesn't exist — create
		if err := h.db.Create(&user).Error; err != nil {
			log.Printf("upsert user create: %v", err)
		}
	} else {
		// User exists — update
		h.db.Model(&db.User{}).Where("cognito_sub = ?", claims.Sub).Updates(map[string]interface{}{
			"email":  claims.Email,
			"org_id": claims.OrgID,
			"role":   claims.Role,
		})
	}
}

// upsertUserFromIDToken parses a raw ID token (without signature verification — it was
// already verified by Cognito token endpoint) and upserts the user record.
func (h *AuthHandler) upsertUserFromIDToken(idToken string) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	orgID, _ := claims["custom:orgId"].(string)
	role, _ := claims["custom:role"].(string)
	if orgID == "" {
		orgID = "default-org"
	}
	if role == "" {
		role = "ANALYST"
	}

	h.upsertUserFromClaims(&lib.UserClaims{Sub: sub, Email: email, OrgID: orgID, Role: role})
}
