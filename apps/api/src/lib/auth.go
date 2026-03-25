package lib

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// UserClaims extracted from Cognito JWT
type UserClaims struct {
	Sub   string
	Email string
	OrgID string
	Role  string
}

// AdminClaims extracted from admin Cognito JWT (no org scope)
type AdminClaims struct {
	Sub   string
	Email string
}

type jwksKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type JWTMiddleware struct {
	issuer   string
	audience string // Cognito app client ID for audience validation
	mu       sync.RWMutex
	keys     map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewJWTMiddleware(issuer, audience string) *JWTMiddleware {
	return &JWTMiddleware{
		issuer:   issuer,
		audience: audience,
		keys:     make(map[string]*rsa.PublicKey),
	}
}

func (m *JWTMiddleware) fetchJWKS() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.fetchedAt) < time.Hour && len(m.keys) > 0 {
		return nil
	}
	url := m.issuer + "/.well-known/jwks.json"
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		n := new(big.Int).SetBytes(nBytes)
		eInt := new(big.Int).SetBytes(eBytes)
		m.keys[k.Kid] = &rsa.PublicKey{N: n, E: int(eInt.Int64())}
	}
	m.fetchedAt = time.Now()
	return nil
}

func (m *JWTMiddleware) getKey(kid string) (*rsa.PublicKey, error) {
	if err := m.fetchJWKS(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.keys[kid]
	if !ok {
		return nil, fmt.Errorf("unknown key id: %s", kid)
	}
	return k, nil
}

// parseAndValidateJWT validates signature, issuer, and expiration. Returns claims.
func (m *JWTMiddleware) parseAndValidateJWT(tokenStr string) (jwt.MapClaims, error) {
	// Parse unverified to get kid
	unverified, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}
	kid, ok := unverified.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing kid")
	}
	pubKey, err := m.getKey(kid)
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}

	// Validate audience (Cognito uses "client_id" for access tokens, "aud" for ID tokens)
	if m.audience != "" {
		clientID, _ := claims["client_id"].(string)
		aud, _ := claims["aud"].(string)
		if clientID != m.audience && aud != m.audience {
			return nil, fmt.Errorf("audience mismatch")
		}
	}

	return claims, nil
}

// extractBearerToken pulls the token string from the Authorization header.
func extractBearerToken(c echo.Context) (string, error) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", fmt.Errorf("invalid authorization header")
	}
	return parts[1], nil
}

// Middleware returns an Echo middleware that validates Cognito JWTs for app users.
func (m *JWTMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Dev bypass: if no issuer configured, skip auth
			if m.issuer == "" {
				c.Set("user", &UserClaims{Sub: "dev-user", Email: "dev@localhost", OrgID: "default-org", Role: "ADMIN"})
				return next(c)
			}

			tokenStr, err := extractBearerToken(c)
			if err != nil {
				return echo.ErrUnauthorized
			}

			claims, err := m.parseAndValidateJWT(tokenStr)
			if err != nil {
				return echo.ErrUnauthorized
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
			c.Set("user", &UserClaims{Sub: sub, Email: email, OrgID: orgID, Role: role})
			return next(c)
		}
	}
}

// AdminMiddleware returns an Echo middleware that validates Cognito JWTs for BetterDFM admins.
// Admin tokens use a different Cognito app client (different audience) than app tokens.
func (m *JWTMiddleware) AdminMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Dev bypass: if no issuer configured, skip auth
			if m.issuer == "" {
				c.Set("admin", &AdminClaims{Sub: "dev-admin", Email: "admin@localhost"})
				return next(c)
			}

			tokenStr, err := extractBearerToken(c)
			if err != nil {
				return echo.ErrUnauthorized
			}

			claims, err := m.parseAndValidateJWT(tokenStr)
			if err != nil {
				return echo.ErrUnauthorized
			}

			sub, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			c.Set("admin", &AdminClaims{Sub: sub, Email: email})
			return next(c)
		}
	}
}

// GetUser retrieves the current user claims from the echo context.
func GetUser(c echo.Context) *UserClaims {
	u, _ := c.Get("user").(*UserClaims)
	if u == nil {
		return &UserClaims{OrgID: "default-org", Role: "ANALYST"}
	}
	return u
}

// RequireRole returns middleware that checks the user has one of the allowed roles.
// Must be chained after Middleware() which populates user claims.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUser(c)
			if !allowed[user.Role] {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}
			return next(c)
		}
	}
}

// GetAdmin retrieves the current admin claims from the echo context.
func GetAdmin(c echo.Context) *AdminClaims {
	a, _ := c.Get("admin").(*AdminClaims)
	if a == nil {
		return &AdminClaims{}
	}
	return a
}
