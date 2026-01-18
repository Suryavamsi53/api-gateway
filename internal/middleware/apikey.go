package middleware

import (
	"log"
	"net/http"
	"sync"
)

// APIKeyStore manages API keys and their permissions
type APIKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKey
}

// APIKey represents an API key with permissions
type APIKey struct {
	Key       string   // The actual key
	Name      string   // Human readable name
	Role      string   // Role assigned to this key
	Enabled   bool     // Whether the key is active
	Paths     []string // Allowed paths (if empty, all allowed for role)
	RateLimit int      // Requests per second (0 = unlimited)
}

// NewAPIKeyStore creates a new API key store
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]*APIKey),
	}
}

// AddKey adds a new API key
func (s *APIKeyStore) AddKey(key *APIKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[key.Key] = key
}

// RemoveKey removes an API key
func (s *APIKeyStore) RemoveKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, key)
}

// GetKey retrieves an API key
func (s *APIKeyStore) GetKey(key string) (*APIKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.keys[key]
	return val, ok
}

// ValidateKey checks if an API key is valid and allowed for the path
func (s *APIKeyStore) ValidateKey(key, path string) (*APIKey, error) {
	apiKey, exists := s.GetKey(key)
	if !exists {
		return nil, ErrInvalidAPIKey
	}

	if !apiKey.Enabled {
		return nil, ErrAPIKeyDisabled
	}

	// Check path access if specific paths are set
	if len(apiKey.Paths) > 0 {
		allowed := false
		for _, p := range apiKey.Paths {
			if matchPath(p, path) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrAPIKeyPathDenied
		}
	}

	return apiKey, nil
}

// ListKeys returns all API keys (without sensitive data)
func (s *APIKeyStore) ListKeys() []*APIKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]*APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, key)
	}
	return keys
}

// Custom errors
type APIKeyError struct {
	Code    string
	Message string
}

func (e APIKeyError) Error() string {
	return e.Message
}

var (
	ErrInvalidAPIKey    = APIKeyError{Code: "invalid_api_key", Message: "API key is invalid"}
	ErrAPIKeyDisabled   = APIKeyError{Code: "api_key_disabled", Message: "API key is disabled"}
	ErrAPIKeyPathDenied = APIKeyError{Code: "api_key_path_denied", Message: "API key not allowed for this path"}
)

// APIKeyMiddleware validates API keys from X-API-Key header
type APIKeyMiddleware struct {
	store *APIKeyStore
}

// NewAPIKeyMiddleware creates a new API key middleware
func NewAPIKeyMiddleware(store *APIKeyStore) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		store: store,
	}
}

// Handler returns the middleware handler
func (am *APIKeyMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key from header
			apiKey := r.Header.Get("X-API-Key")

			// If no API key provided, continue (other auth methods may handle it)
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Validate API key
			key, err := am.store.ValidateKey(apiKey, r.URL.Path)
			if err != nil {
				log.Printf("API key validation failed: %v", err)
				http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
				return
			}

			// Inject role and key info into headers for downstream
			r.Header.Set("X-User-Role", key.Role)
			r.Header.Set("X-API-Key-Name", key.Name)
			r.Header.Set("X-Auth-Method", "api-key")

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultAPIKeys returns some example API keys for testing
func DefaultAPIKeys() *APIKeyStore {
	store := NewAPIKeyStore()
	store.AddKey(&APIKey{
		Key:       "key_admin_prod_123",
		Name:      "Admin Production Key",
		Role:      "admin",
		Enabled:   true,
		Paths:     []string{"/admin/*", "/api/*", "/metrics"},
		RateLimit: 10000,
	})
	store.AddKey(&APIKey{
		Key:       "key_user_prod_456",
		Name:      "User Production Key",
		Role:      "user",
		Enabled:   true,
		Paths:     []string{"/api/*"},
		RateLimit: 1000,
	})
	store.AddKey(&APIKey{
		Key:       "key_viewer_prod_789",
		Name:      "Viewer Key",
		Role:      "viewer",
		Enabled:   true,
		Paths:     []string{"/metrics", "/health"},
		RateLimit: 100,
	})
	return store
}
