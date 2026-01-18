package middleware

import (
	"log"
	"net/http"
	"strings"
)

// RBACMiddleware enforces role-based access control
type RBACMiddleware struct {
	rolePermissions map[string][]string // role -> list of allowed paths
}

// NewRBACMiddleware creates a new RBAC middleware
func NewRBACMiddleware(rolePermissions map[string][]string) *RBACMiddleware {
	if rolePermissions == nil {
		rolePermissions = make(map[string][]string)
	}
	return &RBACMiddleware{
		rolePermissions: rolePermissions,
	}
}

// Handler returns the middleware handler
func (rm *RBACMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get role from request context (injected by JWT middleware)
			role := r.Header.Get("X-User-Role")

			// If no role, deny access
			if role == "" {
				http.Error(w, "Unauthorized: no role specified", http.StatusUnauthorized)
				return
			}

			// Check if role has access to this path
			if !rm.hasAccessToPath(role, r.URL.Path) {
				log.Printf("RBAC denied: role=%s path=%s", role, r.URL.Path)
				http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasAccessToPath checks if a role has access to a path
func (rm *RBACMiddleware) hasAccessToPath(role, path string) bool {
	permissions, exists := rm.rolePermissions[role]
	if !exists {
		// If role not in permissions map, deny
		return false
	}

	// Check if any permission matches the path
	for _, perm := range permissions {
		if matchPath(perm, path) {
			return true
		}
	}

	return false
}

// matchPath checks if a permission pattern matches a path
// Supports wildcards: /admin/* matches /admin/policies
func matchPath(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Wildcard match
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/")
	}

	return false
}

// SetRolePermissions updates role permissions
func (rm *RBACMiddleware) SetRolePermissions(rolePermissions map[string][]string) {
	rm.rolePermissions = rolePermissions
}

// GetRolePermissions returns current role permissions
func (rm *RBACMiddleware) GetRolePermissions() map[string][]string {
	return rm.rolePermissions
}

// DefaultRolePermissions returns sensible defaults
func DefaultRolePermissions() map[string][]string {
	return map[string][]string{
		"admin": {
			"/admin/*",
			"/api/*",
			"/metrics",
			"/health",
		},
		"operator": {
			"/admin/policies",
			"/api/*",
			"/health",
		},
		"viewer": {
			"/metrics",
			"/health",
			"/status",
		},
		"user": {
			"/api/*",
			"/health",
		},
	}
}
