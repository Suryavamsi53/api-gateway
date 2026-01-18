package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Logging is a middleware that logs requests as structured JSON including request id and latency.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		dur := time.Since(start)
		log.Info().Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("request_id", r.Header.Get("X-Request-ID")).
			Dur("latency", dur).
			Msg("request completed")
	})
}
