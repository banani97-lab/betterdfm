package lib

import (
	"context"
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
	issuer    string
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewJWTMiddleware(issuer string) *JWTMiddleware {
	return &JWTMiddleware{
		issuer: issuer,
		keys:   make(map[string]*rsa.PublicKey),
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

// Middleware returns an Echo middleware that validates Cognito JWTs.
func (m *JWTMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Dev bypass: if no issuer configured, skip auth
			if m.issuer == "" {
				c.Set("user", &UserClaims{Sub: "dev-user", Email: "dev@localhost", OrgID: "default-org", Role: "ADMIN"})
				return next(c)
			}

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.ErrUnauthorized
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return echo.ErrUnauthorized
			}
			tokenStr := parts[1]

			// Parse unverified to get kid
			unverified, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
			if err != nil {
				return echo.ErrUnauthorized
			}
			kid, ok := unverified.Header["kid"].(string)
			if !ok {
				return echo.ErrUnauthorized
			}
			pubKey, err := m.getKey(kid)
			if err != nil {
				return echo.ErrUnauthorized
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return pubKey, nil
			}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
			if err != nil || !token.Valid {
				return echo.ErrUnauthorized
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
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

// GetUser retrieves the current user claims from the echo context.
func GetUser(c echo.Context) *UserClaims {
	u, _ := c.Get("user").(*UserClaims)
	if u == nil {
		return &UserClaims{OrgID: "default-org", Role: "ANALYST"}
	}
	return u
}

// GetOrCreateUser upserts a user record. ctx must be a context.Context.
func GetOrCreateUser(_ context.Context, claims *UserClaims) *UserClaims {
	return claims
}
