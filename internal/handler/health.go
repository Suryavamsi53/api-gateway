package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler handles health check requests.
type HealthHandler struct{}

// LivenessResponse represents liveness probe response.
type LivenessResponse struct {
	Status string `json:"status"`
	Time   int64  `json:"timestamp"`
}

// ReadinessResponse represents readiness probe response.
type ReadinessResponse struct {
	Status string `json:"status"`
	Redis  string `json:"redis"`
}

// Liveness returns 200 if the service is running.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LivenessResponse{
		Status: "alive",
		Time:   time.Now().Unix(),
	})
}

// Readiness returns 200 if the service is ready to serve traffic.
// In production, check Redis connectivity, database, etc.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	redisStatus := "ok"
	// In production, perform actual health check on Redis
	json.NewEncoder(w).Encode(ReadinessResponse{
		Status: "ready",
		Redis:  redisStatus,
	})
}

// Status returns detailed status information.
func (h *HealthHandler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := map[string]interface{}{
		"service":   "api-gateway",
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(startTime).Seconds(),
	}
	json.NewEncoder(w).Encode(status)
}

var startTime = time.Now()
