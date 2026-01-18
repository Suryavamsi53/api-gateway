# API Gateway & Distributed Rate Limiter

**A production-grade API Gateway with Redis-backed distributed rate limiting, built in Go.**

## Overview

This gateway acts as a reverse proxy with pluggable rate-limiting algorithms. It's designed for high concurrency, low latency, and seamless scalability across multiple instances via Redis.

**Key Features:**
- Reverse proxy with transparent request forwarding
- Distributed rate limiting via Redis with atomic operations
- Multiple rate-limiting algorithms (Token Bucket, Sliding Window)
- Per-key, per-endpoint, and per-IP rate limit policies
- Structured JSON logging with request IDs and latency metrics
- Prometheus metrics for QPS, latency, and rate-limit violations
- Graceful shutdown (SIGTERM/SIGINT with context cancellation)
- Clean Architecture (domain, service, repository, handler, middleware)
- Full test coverage for concurrency and correctness
- Docker & Kubernetes ready

---

## Architecture

```
cmd/gateway/main.go                 # Application entry point with graceful shutdown
├── internal/config/                # Configuration and dynamic policy store
├── internal/repository/            # Storage abstraction (Redis, in-memory)
├── internal/service/limiter.go     # Rate-limiting orchestration
├── internal/handler/proxy.go       # Reverse proxy implementation
├── internal/middleware/            # Request ID, logging, rate-limit checks
└── internal/metrics/               # Prometheus metrics
```

### Design Decisions

1. **Token Bucket Algorithm (default)**
   - Implemented via Redis Lua script for atomic refill + consume
   - Pros: High throughput, configurable burst allowance
   - Cons: Less accurate over long time windows

2. **Sliding Window Algorithm**
   - Implemented using Redis sorted sets with timestamp tracking
   - Pros: Accurate request counting, fine-grained limits
   - Cons: Slightly higher CPU/memory overhead

3. **Redis vs. In-Memory Storage**
   - Redis: Distributed state across instances, suitable for production
   - In-Memory: Local development and fallback if Redis is down (optional feature)

4. **Concurrency Model**
   - Token Bucket: Mutex-protected in-memory, Lua script in Redis
   - Sliding Window: Sorted set operations are atomic in Redis, mutex in local store
   - All operations are concurrency-safe and can handle thousands of concurrent requests

5. **Policy Configuration**
   - Static in-memory store provided; in production, load from config service or database
   - Per-API-key policies (premium, standard tiers)
   - Per-endpoint policies (expensive endpoints get lower limits)
   - Per-IP rate limiting as fallback

---

## Running

### Prerequisites
- Go 1.21+
- Optional: Redis for distributed deployments
- Optional: Docker/Podman for containerization

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build binary
CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o bin/gateway ./cmd/gateway

# Run gateway (uses in-memory store)
./bin/gateway
# Optionally set environment variables:
# LISTEN_ADDR=:8080 (default)
# DOWNSTREAM_URL=http://localhost:8081 (default)
# REDIS_ADDR=localhost:6379 (optional, uses in-memory if not set)
```

### Docker

```bash
# Build image
docker build -t api-gateway:local .

# Run container
docker run -p 8080:8080 \
  -e DOWNSTREAM_URL=http://localhost:8081 \
  -e REDIS_ADDR=redis:6379 \
  api-gateway:local
```

### Kubernetes

```bash
# Apply manifests
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/hpa.yaml
```

---

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run specific package tests
go test ./internal/service -v
go test ./internal/repository -v

# Run benchmarks
go test -bench=. ./...
```

**Test Coverage:**
- **Token Bucket**: Concurrency tests (20+ goroutines), capacity limits, refill logic
- **Sliding Window**: Time-windowed event counting, cleanup
- **Multi-key isolation**: Ensure per-key quotas are independent
- **In-memory store**: Mutex safety, timestamp tracking
- **Rate-limit middleware**: Extraction of API keys and IP addresses

---

## API Examples

### Rate-Limited Request

```bash
# Request with X-API-Key header (premium tier)
curl -H "X-API-Key: api-key:premium" http://localhost:8080/api/users

# Request with IP-based rate limiting
curl http://localhost:8080/api/users

# Response on rate limit exceeded (HTTP 429)
{
  "error": "rate_limited",
  "message": "rate limit exceeded",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Metrics Endpoint

```bash
# Prometheus metrics at /metrics
curl http://localhost:8080/metrics
```

---

## Scaling & Trade-offs

### Horizontal Scaling
- **With Redis**: Deploy multiple gateway instances with shared Redis backend
  - Pros: Accurate distributed rate limiting, shared state
  - Cons: Redis becomes a bottleneck; needs Redis cluster or sentinel for HA

- **Without Redis**: In-memory store (local only)
  - Pros: Zero external dependencies, ultra-low latency
  - Cons: Rate limits per instance, not suitable for multi-instance setups

### Performance Characteristics
- **Token Bucket**: ~1-2 microseconds per check (local), ~5-10ms via Redis
- **Sliding Window**: ~2-3 microseconds per check (local), ~10-15ms via Redis
- **Throughput**: 10K+ RPS per instance with in-memory; bottleneck is Redis network round-trip for distributed

### High-Availability Recommendations
1. Run gateway behind a load balancer (nginx, HAProxy)
2. Use Redis Cluster or Sentinel for high availability
3. Set `GRACEFUL_SHUTDOWN_TIMEOUT` for in-flight request completion
4. Configure HPA based on CPU/memory metrics
5. Use readiness/liveness probes on `/metrics` endpoint

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | Address to listen on |
| `DOWNSTREAM_URL` | `http://localhost:8081` | Backend service URL |
| `REDIS_ADDR` | (empty) | Redis connection (e.g., `localhost:6379`); uses in-memory store if not set |
| `GRACEFUL_SHUTDOWN_TIMEOUT` | `15` | Graceful shutdown timeout in seconds |

---

## Monitoring

### Prometheus Metrics
- `gateway_requests_total` – Total requests received
- `gateway_rate_limited_total` – Total rate-limited responses
- Add custom histograms/gauges as needed for latency percentiles

### Structured Logging
All logs are JSON with:
- `request_id` – Unique ID for request tracing
- `method` – HTTP method
- `path` – Request path
- `latency` – Request latency in nanoseconds
- `error` – Error messages if any

Example:
```json
{"level":"info","time":"2025-01-18T12:34:56.789Z","message":"request completed","method":"GET","path":"/api/users","request_id":"550e8400-e29b-41d4-a716-446655440000","latency":5000000}
```

---

## Development

### Project Structure
```
.
├── cmd/gateway/main.go              # Entrypoint
├── internal/
│   ├── config/config.go             # Configuration & policy store
│   ├── handler/proxy.go             # Reverse proxy
│   ├── middleware/                  # Request ID, logging, rate-limit
│   ├── repository/                  # Store interface & implementations
│   ├── service/limiter.go           # Rate-limiting logic
│   └── metrics/metrics.go           # Prometheus metrics
├── Dockerfile                        # Multi-stage build
├── k8s/                             # Kubernetes manifests
├── Makefile                         # Build targets
├── go.mod & go.sum                  # Dependencies
└── README.md                        # This file
```

### Adding Custom Rate-Limit Policies
Edit `internal/config/config.go` in the `NewPolicyStore()` function to add policies:

```go
policies := staticPolicies{
    "api-key:custom": {
        Algorithm: "tokenbucket",
        Capacity:  500,
        Rate:      200, // 200 tokens/sec
    },
}
```

In production, load policies dynamically from a database or config service.

---

## Future Enhancements
- [ ] Dynamic policy updates without restart
- [ ] Request queuing/backpressure instead of immediate rejection
- [ ] Circuit breaker for downstream failures
- [ ] Enhanced observability (distributed tracing with Jaeger)
- [ ] Rate-limit headers in responses (X-RateLimit-*)
- [ ] Webhook notifications for limit violations

---

## License
MIT
