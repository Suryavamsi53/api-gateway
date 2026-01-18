package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:     "test-key-123",
		Name:    "Test Key",
		Role:    "user",
		Enabled: true,
		Paths:   []string{"/api/*"},
	})

	am := NewAPIKeyMiddleware(store)
	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		role := r.Header.Get("X-User-Role")
		if role != "user" {
			t.Errorf("expected role 'user', got %s", role)
		}
	}))

	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_InvalidKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:     "test-key-123",
		Name:    "Test Key",
		Role:    "user",
		Enabled: true,
		Paths:   []string{"/api/*"},
	})

	am := NewAPIKeyMiddleware(store)
	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_DisabledKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:     "disabled-key",
		Name:    "Disabled Key",
		Role:    "user",
		Enabled: false,
		Paths:   []string{"/api/*"},
	})

	am := NewAPIKeyMiddleware(store)
	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("X-API-Key", "disabled-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for disabled key, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_PathDenied(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:     "test-key",
		Name:    "Test Key",
		Role:    "user",
		Enabled: true,
		Paths:   []string{"/api/*"}, // Only /api/* allowed
	})

	am := NewAPIKeyMiddleware(store)
	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin/policies", nil)
	req.Header.Set("X-API-Key", "test-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for denied path, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_NoKeyProvided(t *testing.T) {
	store := NewAPIKeyStore()
	am := NewAPIKeyMiddleware(store)

	handler := am.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/users", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no key provided, got %d", w.Code)
	}
}

func TestAPIKeyStore_ValidateKey(t *testing.T) {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:     "test-key",
		Name:    "Test",
		Role:    "admin",
		Enabled: true,
		Paths:   []string{"/admin/*", "/api/*"},
	})

	tests := []struct {
		key       string
		path      string
		shouldErr bool
	}{
		{"test-key", "/admin/policies", false},
		{"test-key", "/api/users", false},
		{"test-key", "/metrics", true}, // Not allowed
		{"invalid", "/admin/policies", true},
	}

	for _, tt := range tests {
		_, err := store.ValidateKey(tt.key, tt.path)
		hasErr := err != nil
		if hasErr != tt.shouldErr {
			t.Errorf("ValidateKey(%q, %q): shouldErr=%v, got %v", tt.key, tt.path, tt.shouldErr, hasErr)
		}
	}
}
