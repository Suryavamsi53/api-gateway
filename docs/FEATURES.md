# Advanced Features Guide

This document describes the newly added features to the API Gateway: RBAC, API Keys, Caching, and Circuit Breakers.

## Table of Contents
1. [RBAC (Role-Based Access Control)](#rbac)
2. [API Keys](#api-keys)
3. [Response Caching](#response-caching)
4. [Circuit Breakers](#circuit-breakers)

---

## RBAC

### Overview
RBAC middleware provides fine-grained access control based on user roles. It works seamlessly with JWT authentication by checking the `X-User-Role` header.

### Roles & Permissions

#### Default Roles

```go
admin:     ["/admin/*", "/api/*", "/metrics", "/health"]
operator:  ["/admin/policies", "/api/*", "/health"]
viewer:    ["/metrics", "/health", "/status"]
user:      ["/api/*", "/health"]
```

### Usage

#### Enable RBAC in Gateway

```go
import "api-gateway/internal/middleware"

// In main.go
rbacMiddleware := middleware.NewRBACMiddleware(
    middleware.DefaultRolePermissions(),
)

// Wrap specific routes
mux.Handle("/admin/", rbacMiddleware.Handler()(adminHandler))
```

#### Custom Permissions

```go
customPermissions := map[string][]string{
    "premium_user": {"/api/*", "/reports/*"},
    "free_user":    {"/api/public/*"},
}

rbac := middleware.NewRBACMiddleware(customPermissions)
```

#### Wildcard Matching

```
Pattern         Matches
/admin/*        /admin/policies
                /admin/users
                /admin/settings

/api/*          /api/users
                /api/products

/api/users      /api/users (exact match only)
```

### Error Responses

| Status | Scenario |
|--------|----------|
| 401 | No `X-User-Role` header provided |
| 403 | Role exists but lacks permission for path |

### Testing

```bash
# Test RBAC with curl
curl -H "X-User-Role: admin" http://localhost:8080/admin/policies
# Response: 200 OK

curl -H "X-User-Role: viewer" http://localhost:8080/admin/policies
# Response: 403 Forbidden

curl http://localhost:8080/admin/policies
# Response: 401 Unauthorized
```

---

## API Keys

### Overview
API Keys provide an alternative authentication mechanism. They support:
- Path-based access control
- Per-key rate limiting
- Enable/disable toggle
- Human-readable names

### API Key Structure

```go
type APIKey struct {
    Key       string   // "key_prod_abc123"
    Name      string   // "Production API Key"
    Role      string   // "admin", "user", "viewer"
    Enabled   bool     // true/false
    Paths     []string // []string{"/api/*", "/admin/*"}
    RateLimit int      // Requests per second
}
```

### Default Keys

Three default keys are provided:

```go
key_admin_prod_123      → Admin role, 10K RPS, all paths
key_user_prod_456       → User role, 1K RPS, /api/* only
key_viewer_prod_789     → Viewer role, 100 RPS, /metrics, /health
```

### Usage

#### Enable API Key Middleware

```go
import "api-gateway/internal/middleware"

store := middleware.DefaultAPIKeys()
keyMiddleware := middleware.NewAPIKeyMiddleware(store)

// Apply to all routes
mux.Use(keyMiddleware.Handler())
```

#### Create Custom Keys

```go
store := middleware.NewAPIKeyStore()
store.AddKey(&middleware.APIKey{
    Key:       "key_webhook_prod_xyz",
    Name:      "Webhook Integration",
    Role:      "user",
    Enabled:   true,
    Paths:     []string{"/api/webhooks/*"},
    RateLimit: 100,
})
```

#### Managing Keys at Runtime

```go
// Add key
store.AddKey(newKey)

// Remove key
store.RemoveKey("key_to_remove")

// Get key details
key, exists := store.GetKey("key_admin_prod_123")
if exists {
    fmt.Printf("Key %s: %s\n", key.Key, key.Name)
}

// List all keys
keys := store.ListKeys()
for _, k := range keys {
    fmt.Printf("%s (%s)\n", k.Name, k.Role)
}
```

### API Key Usage

#### In Request Headers

```bash
curl -H "X-API-Key: key_user_prod_456" http://localhost:8080/api/users
```

#### Response Headers

```
X-User-Role: user
X-API-Key-Name: User Production Key
X-Auth-Method: api-key
```

### Error Responses

| Status | Scenario |
|--------|----------|
| 401 | Invalid, disabled, or path-denied API key |

### Testing

```bash
# Valid API key
curl -H "X-API-Key: key_user_prod_456" http://localhost:8080/api/users
# Response: 200 OK

# Disabled API key
curl -H "X-API-Key: key_disabled_789" http://localhost:8080/api/users
# Response: 401 Unauthorized

# Key lacks permission for path
curl -H "X-API-Key: key_viewer_prod_789" http://localhost:8080/api/users
# Response: 401 Unauthorized
```

---

## Response Caching

### Overview
Response caching improves performance by storing HTTP responses and serving them from cache for subsequent identical requests. Features:

- Automatic cache key generation (MD5 of method + path + query)
- TTL-based expiration (respects Cache-Control headers)
- LRU eviction when cache is full
- Per-request cache bypass options
- Configurable max entry size

### Configuration

```go
import "api-gateway/internal/service"

// Create cache: 1000 entries max, 10MB per entry max
cache := service.NewResponseCache(1000, 10*1024*1024)
```

### Cache Strategy

#### Cacheable Responses

Only responses are cached if:
- Status is 200 OK or 404 Not Found
- Cache-Control header allows caching (no "no-cache" / "no-store")

#### Cache Keys

Cache keys are generated from:
1. HTTP method (GET, POST, etc.)
2. Request path (/api/users)
3. Query string (id=123&sort=name)

```go
key := GenerateCacheKey("GET", "/api/users", "page=1")
// Returns: "3f7a8b9c2d1e5f4a6b8c9d0e1f2a3b4c"
```

#### TTL Extraction

TTL is determined by:
1. Cache-Control max-age header (preferred)
2. Default: 5 minutes

```
Response Header              TTL
Cache-Control: max-age=600   10 minutes
(no header)                  5 minutes (default)
Cache-Control: no-cache      Not cached
```

### Usage

#### With Reverse Proxy

```go
import "api-gateway/internal/service"

cache := service.NewResponseCache(1000, 10*1024*1024)
cachedTransport := service.NewCachedRoundTripper(cache)

// Use in http.Client
client := &http.Client{
    Transport: cachedTransport,
}

resp, _ := client.Do(request)
// If previously cached, X-Cache header will be "HIT"
```

#### Manual Cache Control

```go
// Clear entire cache
cache.Clear()

// Get cache size
size := cache.GetSize()
fmt.Printf("Cache has %d entries\n", size)

// Direct cache operations
entry := &service.CacheEntry{
    Status:     200,
    Body:       []byte("data"),
    ExpiresAt:  time.Now().Add(5 * time.Minute),
}
cache.Set("custom-key", entry)

entry, exists := cache.Get("custom-key")
if exists && !entry.IsExpired() {
    fmt.Printf("Got cached response: %s\n", entry.Body)
}
```

### Monitoring

```bash
# Check cache metrics
# (Implement GET /admin/cache endpoint to expose this)
cache.GetSize()          // Number of entries
cache.GetMetrics()       // Per-entry hit counts
```

### Testing

```bash
# First request (cache miss)
curl http://localhost:8080/api/users
# Response headers include: X-Cache: MISS

# Second request (cache hit)
curl http://localhost:8080/api/users
# Response headers include: X-Cache: HIT

# Bypass cache with POST
curl -X POST http://localhost:8080/api/users
# Response: Always fresh (POST not cached)

# With custom TTL
curl -H "Cache-Control: max-age=60" http://localhost:8080/api/custom
# Cached for 60 seconds
```

---

## Circuit Breakers

### Overview
Circuit breakers prevent cascading failures by monitoring health of downstream services. States:

- **Closed** (Normal): All requests pass through
- **Open** (Failing): Requests rejected immediately
- **Half-Open** (Recovery): Limited requests allowed to test recovery

### Circuit States

```
                 Failure Threshold Reached
                           ↓
    Closed → Open → [Wait Timeout] → Half-Open
                ↑                         ↓
                └─────────── Success ────┘
                  (all successes = Closed)
```

### Configuration

```go
import "api-gateway/internal/service"

// failureThreshold=5, successThreshold=3, timeout=30s
breaker := service.NewCircuitBreaker(5, 3, 30*time.Second)
```

| Parameter | Default | Meaning |
|-----------|---------|---------|
| failureThreshold | 5 | Failures before opening |
| successThreshold | 3 | Successes to close from half-open |
| timeout | 30s | Wait before trying recovery |

### Pool Usage

Manage circuit breakers for multiple services:

```go
import "api-gateway/internal/service"

pool := service.NewCircuitBreakerPool(5, 3, 30*time.Second)

// Get or create breaker for service
breaker := pool.Get("user-service")
breaker := pool.Get("order-service")

// Both services now have separate circuit breakers
```

### Making Calls

```go
breaker := pool.Get("api-service")

err := breaker.Call(func() error {
    // Execute call to service
    resp, err := http.Get("http://api-service/users")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
})

if err == service.ErrCircuitBreakerOpen {
    // Circuit is open, service unhealthy
    http.Error(w, "Service temporarily unavailable", 503)
    return
}
```

### Monitoring

```go
// Get state of single breaker
state := breaker.GetState()  // "closed", "open", "half-open"

// Get detailed metrics
metrics := breaker.GetMetrics()
fmt.Printf("State: %s, Failures: %d, Successes: %d\n",
    metrics.State, metrics.FailureCount, metrics.SuccessCount)

// Get all breakers' metrics
allMetrics := pool.GetMetrics()
for service, m := range allMetrics {
    fmt.Printf("%s: %s\n", service, m.State)
}
```

### Management

```go
// Reset single breaker
pool.Reset("user-service")

// Reset all breakers
pool.ResetAll()
```

### Example: Reverse Proxy with Circuit Breaker

```go
import (
    "api-gateway/internal/service"
    "net/http/httputil"
)

pool := service.NewCircuitBreakerPool(5, 3, 30*time.Second)
proxy := httputil.NewSingleHostReverseProxy(targetURL)

http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
    breaker := pool.Get("downstream-api")
    
    err := breaker.Call(func() error {
        proxy.ServeHTTP(w, r)
        return nil
    })
    
    if err == service.ErrCircuitBreakerOpen {
        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
    }
})
```

### Testing

```bash
# Monitor circuit breaker state
# Implement GET /admin/circuit-breakers endpoint

curl http://localhost:8080/admin/circuit-breakers
# Response:
# {
#   "downstream-api": {
#     "state": "closed",
#     "failures": 0,
#     "successes": 15,
#     "current_requests": 2
#   }
# }

# After multiple failures:
# {
#   "downstream-api": {
#     "state": "open",
#     "failures": 5,
#     "successes": 0
#   }
# }
```

---

## Integration Examples

### Combining RBAC + API Keys + Caching

```go
package main

import (
    "api-gateway/internal/middleware"
    "api-gateway/internal/service"
    "net/http"
)

func setupGateway() {
    // Setup API keys
    keyStore := middleware.DefaultAPIKeys()
    keyMiddleware := middleware.NewAPIKeyMiddleware(keyStore)
    
    // Setup RBAC
    rbac := middleware.NewRBACMiddleware(middleware.DefaultRolePermissions())
    
    // Setup caching
    cache := service.NewResponseCache(1000, 10*1024*1024)
    cachedTransport := service.NewCachedRoundTripper(cache)
    
    // Setup circuit breakers
    circuitPool := service.NewCircuitBreakerPool(5, 3, 30*time.Second)
    
    mux := http.NewServeMux()
    
    // Apply middleware stack: API Key → RBAC → Handler
    apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Use cached transport for downstream calls
        client := &http.Client{Transport: cachedTransport}
        req, _ := http.NewRequest("GET", "http://backend:8081"+r.URL.Path, nil)
        
        // Use circuit breaker
        cb := circuitPool.Get("backend-api")
        err := cb.Call(func() error {
            resp, _ := client.Do(req)
            resp.Write(w)
            return nil
        })
        
        if err != nil {
            http.Error(w, "Service Unavailable", 503)
        }
    })
    
    protectedHandler := rbac.Handler()(apiHandler)
    protectedHandler = keyMiddleware.Handler()(protectedHandler)
    
    mux.Handle("/api/", protectedHandler)
    http.ListenAndServe(":8080", mux)
}
```

---

## Performance Considerations

### RBAC
- Wildcard matching: O(n) where n = number of permissions
- Recommended: Keep permissions list < 100 entries

### API Keys
- Key lookup: O(1) hash map access
- Path validation: O(m) where m = number of allowed paths per key

### Response Caching
- Cache operations: O(1) average case
- Cache eviction: O(n) where n = cache size (amortized)
- Recommended: Keep cache size < 10,000 entries

### Circuit Breakers
- State transitions: O(1)
- Metrics collection: O(1)
- Recommended: Monitor state changes in production

---

## Security Best Practices

1. **RBAC**: Always validate role from authenticated source (JWT, API key)
2. **API Keys**: Rotate keys regularly, disable unused keys
3. **Caching**: Be cautious with sensitive data (PII, secrets)
4. **Circuit Breakers**: Pair with health checks for faster recovery

---

## Troubleshooting

### RBAC Issues
```
Problem:  403 Forbidden on allowed path
Solution: Check X-User-Role header is set correctly by auth middleware

Problem:  Permission pattern not matching
Solution: Use wildcard (*) for path prefixes
```

### API Key Issues
```
Problem:  401 Unauthorized with valid key
Solution: Verify key is enabled and path is in allowed list

Problem:  Rate limit applies per-key
Solution: Configure RateLimit field per APIKey instance
```

### Caching Issues
```
Problem:  Stale data being served
Solution: Reduce TTL, use Cache-Control: no-cache for fresh data

Problem:  Cache memory growing
Solution: Reduce maxSize or maxEntry parameters
```

### Circuit Breaker Issues
```
Problem:  Circuit stuck open
Solution: Call pool.Reset(service) or wait for timeout + successThreshold hits

Problem:  Too many state transitions
Solution: Increase failureThreshold to require more failures
```

---

## Testing

Run all feature tests:

```bash
# RBAC tests
go test -v ./internal/middleware -run RBAC

# API Key tests
go test -v ./internal/middleware -run APIKey

# Cache tests
go test -v ./internal/service -run Cache

# Circuit Breaker tests
go test -v ./internal/service -run CircuitBreaker
```

---

## Future Enhancements

- [ ] Implement metrics export for all features
- [ ] Add admin API endpoints for feature management
- [ ] Support conditional caching based on query parameters
- [ ] Implement adaptive circuit breaker thresholds
- [ ] Add distributed RBAC via external policy engine
- [ ] Support API key revocation lists (CRL)
- [ ] Implement cache warming
- [ ] Add circuit breaker webhooks for alerts

