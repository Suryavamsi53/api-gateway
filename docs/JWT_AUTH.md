# JWT Authentication Guide

This API Gateway supports two JWT authentication methods:

1. **HMAC (Symmetric)** - `NewJWTMiddleware()` - Uses a shared secret
2. **JWKS (Asymmetric)** - `NewJWKSMiddleware()` - Uses remote public keys (RS256)

## Quick Start

### HMAC-Based Authentication

**Setup:**
```bash
export JWT_SECRET="your-super-secret-key"
export JWT_ISS="https://your-issuer.com"  # optional
```

**Create a token:**
```go
import "github.com/golang-jwt/jwt/v5"

secret := []byte(os.Getenv("JWT_SECRET"))
claims := jwt.RegisteredClaims{
    Issuer:    "https://your-issuer.com",
    Subject:   "user123",
    ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, _ := token.SignedString(secret)
```

**Use in request:**
```bash
curl -H "Authorization: Bearer $tokenString" http://localhost:8080/admin/policies
```

### JWKS-Based Authentication (OpenID Connect Compatible)

**Setup:**
```go
import "api-gateway/internal/middleware"

// Point to your JWKS endpoint (e.g., https://auth.example.com/.well-known/jwks.json)
jwksClient := middleware.NewJWKSClient("https://your-issuer.com/.well-known/jwks.json", 5*time.Minute)
jwtMiddleware := middleware.NewJWKSMiddleware(
    jwksClient,
    "https://your-issuer.com",  // expected issuer
    "api-gateway",              // expected audience (optional)
)

// Apply to routes
mux.Handle("/admin", jwtMiddleware(yourHandler))
```

## Token Format

Both methods use JWT tokens with the following claims:

```json
{
  "iss": "https://your-issuer.com",      // issuer
  "sub": "user123",                      // subject (user ID)
  "aud": ["api-gateway"],                // audience
  "exp": 1234567890,                     // expiration time
  "iat": 1234567890,                     // issued at
  "role": "admin"                        // custom: user role (optional)
}
```

### Custom Claims

The gateway uses `CustomClaims` which extends `RegisteredClaims` with a `role` field:

```go
type CustomClaims struct {
    Role string `json:"role,omitempty"`
    jwt.RegisteredClaims
}
```

## Request Headers Injected

On successful authentication, the gateway injects:

- **X-User-ID** - Value from `sub` claim
- **X-User-Role** - Value from `role` claim (if present)

These headers are available to downstream services.

## Error Handling

Invalid tokens return **HTTP 401 Unauthorized** with JSON error details:

```json
{
  "error": "unauthorized",
  "message": "token is expired"
}
```

### Common Error Messages

| Message | Cause |
|---------|-------|
| `missing Authorization header` | No Bearer token in request |
| `invalid Authorization header format` | Not in `Bearer <token>` format |
| `token is expired` | `exp` claim is in the past |
| `invalid token issuer` | `iss` claim doesn't match expected issuer |
| `invalid token audience` | `aud` claim doesn't include expected audience |
| `invalid token: [reason]` | Token signature, parsing, or validation error |

## Gateway Integration

### Protecting Admin Endpoints

In `cmd/gateway/main.go`:

```go
// JWT auth is optional and only enabled if JWT_SECRET is set
var jwtMiddleware func(http.Handler) http.Handler
if secret := os.Getenv("JWT_SECRET"); secret != "" {
    issuer := os.Getenv("JWT_ISS")
    jwtMiddleware = middleware.NewJWTMiddleware([]byte(secret), issuer)
    log.Info().Msg("JWT authentication enabled")
}

// Protect /admin/policies with JWT
if jwtMiddleware != nil {
    mux.Handle("/admin/policies", jwtMiddleware(admin))
} else {
    mux.Handle("/admin/policies", admin)  // No auth if not configured
}
```

### Public Endpoints

Routes like `/health`, `/ready`, `/metrics` are **not** protected by JWT by default. Add them selectively:

```go
mux.Handle("/private", jwtMiddleware(privateHandler))
```

## Middleware Chain Order

The middleware stack applies in this order:

1. **RequestID** - Generates unique request ID
2. **Logging** - Logs all requests
3. **RateLimit** - Applies rate-limiting policies
4. **RequestSizeLimit** - Enforces max body size (10MB)
5. **JWT** (per-route) - Validates authentication
6. **Handler** - Processes request

**Note:** JWT is applied per-route, not globally, so it only protects endpoints explicitly wrapped with the JWT middleware.

## Examples

### Generate HMAC Token (Python)
```python
import jwt
import json
from datetime import datetime, timedelta

secret = "your-super-secret-key"
payload = {
    "iss": "https://your-issuer.com",
    "sub": "user123",
    "role": "admin",
    "exp": datetime.utcnow() + timedelta(hours=1)
}
token = jwt.encode(payload, secret, algorithm="HS256")
print(f"Authorization: Bearer {token}")
```

### Validate with curl
```bash
export TOKEN="eyJ..."
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/admin/policies
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/users
```

### Test Missing/Expired Tokens
```bash
# No token
curl http://localhost:8080/admin/policies
# Response: 401 Unauthorized

# Malformed header
curl -H "Authorization: InvalidFormat token" http://localhost:8080/admin/policies
# Response: 401 Unauthorized

# Bad token
curl -H "Authorization: Bearer not.a.token" http://localhost:8080/admin/policies
# Response: 401 Unauthorized
```

## JWKS (RS256) Details

### How It Works

1. Client sends JWT signed with private key
2. Gateway extracts `kid` (key ID) from token header
3. Gateway fetches public key from JWKS endpoint using `kid`
4. Gateway verifies token signature with public key
5. Gateway validates claims (`exp`, `iss`, `aud`)
6. If all valid, request proceeds with user headers injected

### JWKS Endpoint Format

Your JWKS endpoint should return:

```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "rsa-key-1",
      "n": "modulus (base64url)",
      "e": "AQAB"
    }
  ]
}
```

### Key Rotation

The `JWKSClient` automatically refreshes keys every 5 minutes (configurable) or on cache miss, so key rotations are picked up transparently.

## Performance Notes

- **HMAC**: ~1-2 microseconds per validation (no network)
- **JWKS**: ~5-10ms per validation (network roundtrip) + caching
- Cache TTL: Configurable, default 5 minutes

## Security Best Practices

1. **Use HTTPS** - Always transmit JWTs over encrypted connections
2. **Short Expiry** - Set `exp` to 15 minutes or less
3. **Rotate Keys** - If using HMAC, rotate the secret regularly
4. **JWKS Endpoint** - Keep JWKS endpoint on same domain as issuer
5. **Audience** - Use specific audience claims to prevent token misuse
6. **Signing Method** - Enforce HMAC (HS256) or RS256, reject others

## Testing

Run the test suite to verify JWT functionality:

```bash
go test ./internal/middleware -v
```

Tests cover:
- Valid token authentication
- Missing headers
- Expired tokens
- Wrong issuer
- JWKS cache behavior and refresh

## Troubleshooting

### "token is expired"
Check that your system clock is synchronized (NTP). Token `exp` claim must be in the future.

### "invalid token issuer"
Verify the `iss` claim in your token matches the expected issuer. Set `JWT_ISS` environment variable or pass to middleware.

### "missing Authorization header"
Client must include `Authorization: Bearer <token>` header in the request.

### JWKS fetch fails
- Verify JWKS endpoint URL is correct
- Endpoint should return HTTP 200 with valid JSON
- Check network connectivity to JWKS server
- Verify token `kid` header matches a key in JWKS response

## API Reference

### HMAC Middleware

```go
// Create from secret
mw := middleware.NewJWTMiddleware([]byte("secret"), "issuer")

// Create from environment
mw, err := middleware.NewJWTMiddlewareFromEnv()  // reads JWT_SECRET and JWT_ISS
```

### JWKS Middleware

```go
// Create client
client := middleware.NewJWKSClient("https://issuer/.well-known/jwks.json", 5*time.Minute)

// Create middleware
mw := middleware.NewJWKSMiddleware(client, "issuer", "audience")
```

### Custom Claims

```go
type CustomClaims struct {
    Role string `json:"role,omitempty"`
    jwt.RegisteredClaims
}
```
