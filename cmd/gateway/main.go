package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/handler"
	"api-gateway/internal/metrics"
	"api-gateway/internal/middleware"
	"api-gateway/internal/repository"
	"api-gateway/internal/service"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()

	zerolog.TimeFieldFormat = time.RFC3339Nano

	// storage
	var store repository.Store
	if cfg.RedisAddr != "" {
		r, err := repository.NewRedisStore(cfg.RedisAddr)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect redis")
		}
		store = r
	} else {
		store = repository.NewMemoryStore()
	}

	// services
	limSvc := service.NewLimiter(store)

	// metrics
	metricsRegistry := metrics.NewRegistry()

	// policy store
	policyStore := config.NewPolicyStore()

	// handler
	proxy := handler.NewProxyHandler(cfg.DownstreamURL, limSvc, metricsRegistry)
	health := &handler.HealthHandler{}
	admin := handler.NewAdminHandler(policyStore)

	// JWT auth (optional: only if JWT_SECRET is set)
	var jwtMiddleware func(http.Handler) http.Handler
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		issuer := os.Getenv("JWT_ISS")
		jwtMiddleware = middleware.NewJWTMiddleware([]byte(secret), issuer)
		log.Info().Msg("JWT authentication enabled")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsRegistry.Handler())
	// Protect admin endpoints with JWT if enabled
	if jwtMiddleware != nil {
		mux.Handle("/admin/policies", jwtMiddleware(admin))
	} else {
		mux.Handle("/admin/policies", admin)
	}
	mux.HandleFunc("/health", health.Liveness)
	mux.HandleFunc("/ready", health.Readiness)
	mux.HandleFunc("/status", health.Status)
	mux.Handle("/", proxy)

	// middleware chain
	h := middleware.RequestID(mux)
	h = middleware.Logging(h)
	h = middleware.RateLimit(limSvc, metricsRegistry, policyStore)(h)
	h = middleware.RequestSizeLimit(middleware.MaxRequestSize)(h)

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: h}

	go func() {
		log.Info().Msgf("listening %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.GracefulShutdownTimeout)*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server shutdown failed")
	}
	log.Info().Msg("server exited")
}
