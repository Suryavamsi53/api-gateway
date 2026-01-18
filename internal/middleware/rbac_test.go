package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRBACMiddleware_AdminAccess(t *testing.T) {
	rbac := NewRBACMiddleware(map[string][]string{
		"admin": {"/admin/*"},
	})

	handler := rbac.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin/policies", nil)
	req.Header.Set("X-User-Role", "admin")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRBACMiddleware_DeniedAccess(t *testing.T) {
	rbac := NewRBACMiddleware(map[string][]string{
		"viewer": {"/health"},
	})

	handler := rbac.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin/policies", nil)
	req.Header.Set("X-User-Role", "viewer")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRBACMiddleware_NoRole(t *testing.T) {
	rbac := NewRBACMiddleware(map[string][]string{
		"admin": {"/admin/*"},
	})

	handler := rbac.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/admin/policies", nil)
	// No X-User-Role header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRBACMiddleware_WildcardMatch(t *testing.T) {
	rbac := NewRBACMiddleware(map[string][]string{
		"user": {"/api/*"},
	})

	handler := rbac.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	paths := []string{"/api/users", "/api/products", "/api/orders"}
	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-User-Role", "user")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("path %s: expected 200, got %d", path, w.Code)
		}
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"/admin", "/admin", true},
		{"/admin/*", "/admin/policies", true},
		{"/admin/*", "/admin/users", true},
		{"/admin/*", "/admin", false},
		{"/api/*", "/api/v1/users", true}, // Matches any path under /api/
		{"/api/*", "/api/users", true},
	}

	for _, tt := range tests {
		got := matchPath(tt.pattern, tt.path)
		if got != tt.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}
