package handler

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/config"
)

// AdminHandler provides simple endpoints to manage rate-limit policies at runtime.
type AdminHandler struct {
	store config.PolicyStore
}

func NewAdminHandler(s config.PolicyStore) *AdminHandler {
	return &AdminHandler{store: s}
}

// ServeHTTP dispatches on method: GET lists policies, POST upserts a policy.
func (a *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		policies := a.store.ListPolicies()
		json.NewEncoder(w).Encode(policies)
	case http.MethodPost:
		var payload struct {
			Key    string              `json:"key"`
			Policy config.PolicyConfig `json:"policy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		a.store.SetPolicy(payload.Key, payload.Policy)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
