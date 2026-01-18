package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/metrics"
	"api-gateway/internal/service"

	"github.com/rs/zerolog/log"
)

// RateLimit builds a middleware using the given limiter service and policy store.
func RateLimit(l *service.Limiter, m *metrics.Registry, ps config.PolicyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = clientIP(r)
			}
			lookup := strings.Join([]string{key, r.URL.Path}, ":")

			// lookup policy
			pc := ps.GetPolicy(lookup)
			// map to service.Policy
			p := service.Policy{
				Algorithm: service.AlgorithmType(pc.Algorithm),
				Capacity:  pc.Capacity,
				Rate:      pc.Rate,
				WindowMs:  pc.WindowMs,
				Limit:     pc.Limit,
			}

			ctx, cancel := context.WithTimeout(r.Context(), 50*time.Millisecond)
			defer cancel()
			allowed, remaining, err := l.Allow(ctx, lookup, p)
			if err != nil {
				log.Error().Err(err).Msg("rate limit evaluation error")
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}

			// attach rate-limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(p.Capacity, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(1*time.Second).Unix(), 10))

			m.Requests.Inc()
			if !allowed {
				m.RateLimited.Inc()
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":      "rate_limited",
					"message":    "rate limit exceeded",
					"request_id": r.Header.Get("X-Request-ID"),
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP attempts to extract the remote IP address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
