# Testing Guide - API Gateway

## Quick Reference

```bash
# Run all tests
make test

# With coverage
make coverage

# Benchmarks
make bench

# Specific package
go test -v ./internal/middleware

# With race detector
go test -race ./...

# Specific test
go test -run TestJWTMiddleware -v ./internal/middleware
```

## Test Suite Overview

### Middleware Tests (`internal/middleware`)

#### JWT Authentication (HMAC)
- ✅ Valid token → headers injected correctly
- ✅ Missing authorization header → 401 response
- ✅ Invalid token signature → 401 response
- ✅ Expired token → 401 response
- ✅ Wrong issuer → 401 response

**File:** [internal/middleware/jwt_auth_test.go](internal/middleware/jwt_auth_test.go)

#### JWKS Authentication (RS256)
- ✅ Valid RS256 token from JWKS endpoint
- ✅ Key caching and TTL expiry
- ✅ Automatic cache refresh on expiry
- ✅ Missing kid header handling
- ✅ Audience claim validation

**File:** [internal/middleware/jwks_auth_test.go](internal/middleware/jwks_auth_test.go)

### Repository Tests (`internal/repository`)

#### Redis Token Bucket
- ✅ Capacity enforcement
- ✅ Token expiry and cleanup
- ✅ Concurrent operations (20+ goroutines)
- ✅ Lua script atomicity

**File:** [internal/repository/redis_test.go](internal/repository/redis_test.go)

#### Redis Sliding Window
- ✅ Window counting accuracy
- ✅ Timestamp-based expiry
- ✅ Concurrent window updates
- ✅ Large key stress test (10K+ entries)

**File:** [internal/repository/redis_test.go](internal/repository/redis_test.go)

#### In-Memory Token Bucket
- ✅ Basic capacity management
- ✅ Concurrent bucket operations
- ✅ Goroutine safety with 500+ parallel requests

**File:** [internal/repository/memory_test.go](internal/repository/memory_test.go)

#### In-Memory Sliding Window
- ✅ Window sliding and cleanup
- ✅ Concurrent access patterns
- ✅ Memory efficiency

**File:** [internal/repository/memory_test.go](internal/repository/memory_test.go)

### Service Tests (`internal/service`)

#### Limiter Service
- ✅ Token bucket algorithm correctness
- ✅ Sliding window algorithm correctness
- ✅ Concurrent rate limiting under load (20+ goroutines)
- ✅ Memory efficiency over time

**File:** [internal/service/limiter_test.go](internal/service/limiter_test.go)

#### Extended Tests
- ✅ Large burst handling
- ✅ Concurrent concurrency safety
- ✅ Edge cases (zero capacity, negative rates)

**File:** [internal/service/limiter_extended_test.go](internal/service/limiter_extended_test.go)

### Benchmarks

#### Performance Metrics

```
Token Bucket (in-memory):
  - BenchmarkTokenBucket
    ~646 ns/op
    0 B/op
    0 allocs/op

Sliding Window (in-memory):
  - BenchmarkSlidingWindow
    ~887 ns/op
    0 B/op (typically)
    0 allocs/op

Concurrent Operations (20 goroutines):
  - BenchmarkLimiterConcurrent
    ~1.5 µs/op total
    Scales linearly with request count
```

**File:** [internal/service/limiter_bench_test.go](internal/service/limiter_bench_test.go)

## Running Tests Locally

### Prerequisites

```bash
# Install Go 1.21+
go version

# Clone repository
git clone https://github.com/Suryavamsi53/api-gateway.git
cd api-gateway

# Download dependencies
go mod download
```

### Full Test Suite

```bash
# Run all tests with verbose output
make test

# Output:
# ok      internal/middleware    0.523s
# ok      internal/repository   0.814s
# ok      internal/service      0.421s
# PASS    ./... (total ~2.2s)
```

### Coverage Report

```bash
# Generate coverage
make coverage

# View in browser
open coverage.html  # macOS
xdg-open coverage.html  # Linux
start coverage.html  # Windows

# Or check coverage percentage
go test -v -cover ./...
```

### Specific Tests

```bash
# JWT middleware only
go test -v -run JWT ./internal/middleware

# Rate limiting only
go test -v -run Limiter ./internal/service

# Concurrent tests only
go test -v -run Concurrent ./...

# Exclude slow tests
go test -short ./...

# With race detector (slower, finds data races)
go test -race ./...
```

## Integration Testing

### Manual Testing with Docker

```bash
# Start full stack
make dev

# Test health check
curl http://localhost:8080/health
# Response: {"status":"ok"}

# Test rate limiting (default: 100 req/sec for IP)
for i in {1..150}; do
  curl -s http://localhost:8080/health -o /dev/null
done | wc -l
# Should see 429 responses after rate limit

# Test proxy (downstream at :8081)
curl http://localhost:8080/api/users
# Response from downstream service

# Test metrics
curl http://localhost:8080/metrics | grep gateway_requests_total
# Response: gateway_requests_total{...} 42

# Stop stack
docker-compose -f docker-compose.yml down
```

### JWT Token Testing

#### Generate HMAC Token

```bash
# Using Python
python3 << 'EOF'
import jwt
import json

secret = "my-secret-key"
payload = {
    "sub": "user123",
    "iss": "api-gateway",
    "role": "admin",
    "exp": 9999999999
}

token = jwt.encode(payload, secret, algorithm="HS256")
print(f"Token: {token}")
EOF

# Using Go
go run << 'EOF'
package main

import (
    "fmt"
    "time"
    jwt "github.com/golang-jwt/jwt/v5"
)

func main() {
    secret := "my-secret-key"
    claims := jwt.MapClaims{
        "sub": "user123",
        "iss": "api-gateway",
        "role": "admin",
        "exp": time.Now().Add(time.Hour).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    ss, _ := token.SignedString([]byte(secret))
    fmt.Println(ss)
}
EOF
```

#### Test JWT Protection

```bash
# Export token
export TOKEN="eyJhbGciOiJIUzI1NiIs..." # from above

# Test without token (if JWT enabled)
curl http://localhost:8080/admin/policies
# Response: 401 Unauthorized

# Test with token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/admin/policies
# Response: 200 OK with list of policies

# Check injected headers (in gateway logs)
# X-User-ID: user123
# X-User-Role: admin
```

### Load Testing

```bash
# Simple load test with curl (1000 requests, 10 concurrency)
seq 1 1000 | xargs -P 10 -I {} curl -s http://localhost:8080/health -o /dev/null
echo "Done"

# Using Apache Bench (if installed)
ab -n 1000 -c 10 http://localhost:8080/health

# Using wrk (if installed)
wrk -t 4 -c 100 -d 10s http://localhost:8080/health

# Monitor metrics during load
watch -n 1 'curl -s http://localhost:8080/metrics | grep gateway_'
```

## Performance Benchmarks

### Interpreting Results

```
BenchmarkTokenBucket-4         	 1000000	   646 ns/op	       0 B/op	       0 allocs/op
^                              ^          ^       ^         ^       ^         ^
Test name                       Runs       Duration/op   Memory  Allocations
                                                         per op  per op
```

### Running Benchmarks

```bash
# All benchmarks
make bench

# Specific benchmark
go test -bench=TokenBucket -benchmem -run=^$ ./internal/service

# With CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./internal/service
go tool pprof cpu.prof

# With memory profiling
go test -bench=. -memprofile=mem.prof ./internal/service
go tool pprof mem.prof
```

### Expected Results

| Operation | Duration | Memory | Notes |
|-----------|----------|--------|-------|
| Token Bucket Allow() | 600-700 ns | 0 B | Minimal allocation |
| Sliding Window Check | 800-1000 ns | 0-16 B | Possible slice append |
| Concurrent (20 req) | 1-2 µs | 0 B | Lock contention |
| Redis Operation | 5-10 ms | Varies | Network RTT |

## Troubleshooting

### Test Failures

#### Redis Connection Errors
```
error connecting to Redis: connection refused
```
**Solution:** Redis not running. Use `make dev` or `docker-compose up redis -d`

#### Port Already in Use
```
listen tcp :8080: bind: address already in use
```
**Solution:** Kill existing process: `lsof -i :8080 | grep LISTEN | awk '{print $2}' | xargs kill -9`

#### JWT Token Invalid
```
Token signature is invalid
```
**Solution:** Ensure JWT_SECRET matches when generating and validating token

#### Benchmark Inconsistency
```
Results vary by 30%+ between runs
```
**Solution:** 
- Close other applications
- Use `-benchtime=5s` for longer runs
- Run on idle system
- Check CPU governor: `cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor`

### Code Coverage Issues

#### Coverage report not generated
```bash
# Force regeneration
rm coverage.* 2>/dev/null
make coverage
```

#### Low coverage in certain files
```bash
# Check which lines aren't covered
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
# Review lines with red background (not covered)
```

## Continuous Integration

### GitHub Actions

Tests automatically run on:
- Push to `main` or `master`
- Pull requests

**Workflow:** [.github/workflows/ci.yml](.github/workflows/ci.yml)

**Steps:**
1. Checkout code
2. Setup Go 1.21
3. Download dependencies
4. Run tests
5. Build binaries

### Running CI Locally

```bash
# Simulate CI environment
docker run -v $(pwd):/app -w /app golang:1.21-alpine sh -c '
  apk add --no-cache git
  go test ./... -v
  go build ./cmd/gateway
  go build ./cmd/downstream
'
```

## Best Practices

### Writing Tests

```go
// Good: Clear name, single assertion
func TestRateLimitExceeded(t *testing.T) {
    limiter := NewLimiter(10) // 10 req/sec
    for i := 0; i < 15; i++ {
        limiter.Allow("user1")
    }
    result := limiter.Allow("user1")
    if result {
        t.Error("expected rate limit exceeded")
    }
}

// Avoid: Vague name, multiple assertions
func TestLimiter(t *testing.T) {
    // ... many assertions mixed together
}
```

### Performance Testing

```go
// Good: Baseline before optimization
// BenchmarkTokenBucket-4    1000000    646 ns/op
// After optimization: measure regression

// Avoid: Only benchmarking in CI without tracking history
```

## Resources

- [Go Testing Package](https://golang.org/pkg/testing/)
- [Go Benchmarking](https://golang.org/pkg/testing/#hdr-Benchmarks)
- [Coverage Best Practices](https://golang.org/doc/tutorial/add-a-test)

---

**Last Updated:** January 18, 2026  
**Test Framework:** Go testing (standard library)  
**Coverage Target:** 85%+
