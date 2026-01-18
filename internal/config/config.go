package config

import (
	"os"
	"strconv"
	"sync"
)

// PolicyConfig specifies rate limit policy for an endpoint or key.
type PolicyConfig struct {
	Algorithm string
	Capacity  int64
	Rate      float64
	WindowMs  int64
	Limit     int64
}

// PolicyStore loads and retrieves policies (in production, backed by DB or config service).
type PolicyStore interface {
	GetPolicy(key string) PolicyConfig
	SetPolicy(key string, p PolicyConfig)
	ListPolicies() map[string]PolicyConfig
}

// staticPolicies is a simple in-memory policy store (in production use dynamic backend).
type dynamicPolicyStore struct {
	mu       sync.RWMutex
	policies map[string]PolicyConfig
}

func (d *dynamicPolicyStore) GetPolicy(key string) PolicyConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if p, ok := d.policies[key]; ok {
		return p
	}
	return PolicyConfig{Algorithm: "tokenbucket", Capacity: 100, Rate: 100, Limit: 100}
}

func (d *dynamicPolicyStore) SetPolicy(key string, p PolicyConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.policies == nil {
		d.policies = make(map[string]PolicyConfig)
	}
	d.policies[key] = p
}

func (d *dynamicPolicyStore) ListPolicies() map[string]PolicyConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]PolicyConfig, len(d.policies))
	for k, v := range d.policies {
		out[k] = v
	}
	return out
}

// NewPolicyStore returns a dynamic in-memory policy store pre-populated with defaults.
func NewPolicyStore() PolicyStore {
	d := &dynamicPolicyStore{policies: make(map[string]PolicyConfig)}
	d.policies["api-key:premium"] = PolicyConfig{Algorithm: "tokenbucket", Capacity: 1000, Rate: 1000}
	d.policies["api-key:standard"] = PolicyConfig{Algorithm: "tokenbucket", Capacity: 100, Rate: 100}
	d.policies["endpoint:/api/expensive"] = PolicyConfig{Algorithm: "slidingwindow", WindowMs: 1000, Limit: 10}
	return d
}

// Config holds configuration loaded from environment variables.
type Config struct {
	RedisAddr               string
	DownstreamURL           string
	ListenAddr              string
	GracefulShutdownTimeout int
}

// Load reads environment variables and returns a Config with sensible defaults.
func Load() Config {
	cfg := Config{
		RedisAddr:     os.Getenv("REDIS_ADDR"),
		DownstreamURL: os.Getenv("DOWNSTREAM_URL"),
		ListenAddr:    os.Getenv("LISTEN_ADDR"),
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.DownstreamURL == "" {
		cfg.DownstreamURL = "http://localhost:8081"
	}
	timeout := os.Getenv("GRACEFUL_SHUTDOWN_TIMEOUT")
	if timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil {
			cfg.GracefulShutdownTimeout = t
		}
	}
	if cfg.GracefulShutdownTimeout == 0 {
		cfg.GracefulShutdownTimeout = 15
	}
	return cfg
}
