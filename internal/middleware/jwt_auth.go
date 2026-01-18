package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims extends RegisteredClaims with application-specific fields.
type CustomClaims struct {
	Role string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// NewJWTMiddleware returns a middleware that validates JWT tokens signed with HMAC.
// It checks the signing method, the token expiration and issuer (`iss`).
// On success it injects `X-User-ID` (from `sub`) and `X-User-Role` into request headers.
func NewJWTMiddleware(secret []byte, expectedIssuer string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				writeUnauthorized(w, "missing Authorization header")
				return
			}
			parts := strings.Fields(auth)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeUnauthorized(w, "invalid Authorization header format")
				return
			}
			tokenStr := parts[1]

			var claims CustomClaims
			token, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
				// enforce HMAC
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return secret, nil
			})
			if err != nil {
				writeUnauthorized(w, "invalid token: "+err.Error())
				return
			}
			if !token.Valid {
				writeUnauthorized(w, "invalid token")
				return
			}

			// Validate registered claims: exp and iss
			if claims.ExpiresAt == nil {
				writeUnauthorized(w, "token missing exp claim")
				return
			}
			if time.Now().After(claims.ExpiresAt.Time) {
				writeUnauthorized(w, "token is expired")
				return
			}
			if expectedIssuer != "" {
				if claims.Issuer != expectedIssuer {
					writeUnauthorized(w, "invalid token issuer")
					return
				}
			}

			// Inject headers and continue
			r2 := r.Clone(r.Context())
			if claims.Subject != "" {
				r2.Header.Set("X-User-ID", claims.Subject)
			}
			if claims.Role != "" {
				r2.Header.Set("X-User-Role", claims.Role)
			}
			next.ServeHTTP(w, r2)
		})
	}
}

// NewJWTMiddlewareFromEnv reads `JWT_SECRET` and `JWT_ISS` from environment and
// returns the middleware. If `JWT_SECRET` is missing it returns an error.
func NewJWTMiddlewareFromEnv() (func(http.Handler) http.Handler, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is not set")
	}
	issuer := os.Getenv("JWT_ISS")
	return NewJWTMiddleware([]byte(secret), issuer), nil
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": msg})
}
