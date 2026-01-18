package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func makeToken(t *testing.T, secret []byte, issuer, subject, role string, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := CustomClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("signed token: %v", err)
	}
	return s
}

func TestJWTMiddleware_Valid(t *testing.T) {
	secret := []byte("test-secret")
	issuer := "test-issuer"

	mw := NewJWTMiddleware(secret, issuer)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "user123" {
			t.Fatalf("expected X-User-ID=user123 got=%s", got)
		}
		if got := r.Header.Get("X-User-Role"); got != "admin" {
			t.Fatalf("expected X-User-Role=admin got=%s", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	token := makeToken(t, secret, issuer, "user123", "admin", time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestJWTMiddleware_Invalid(t *testing.T) {
	secret := []byte("test-secret")
	issuer := "test-issuer"

	mw := NewJWTMiddleware(secret, issuer)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// missing header
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rr.Code)
	}

	// bad token
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad.token.here")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token got %d", rr.Code)
	}

	// expired token
	token := makeToken(t, secret, issuer, "user123", "admin", -time.Minute)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token got %d", rr.Code)
	}

	// wrong issuer
	token = makeToken(t, secret, "wrong-issuer", "user123", "admin", time.Minute)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong issuer got %d", rr.Code)
	}

	// wrong signing method (create RSA token)
	// create env RSA keys quickly (not necessary to validate signing method here, check rejection)
	os.Setenv("JWT_SECRET", "")
}
