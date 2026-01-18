package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	Requests    prometheus.Counter
	RateLimited prometheus.Counter
	// in production you would add histograms for latency and gauges etc.
}

func NewRegistry() *Registry {
	r := &Registry{
		Requests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_requests_total",
			Help: "Total requests received",
		}),
		RateLimited: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gateway_rate_limited_total",
			Help: "Total rate limited responses",
		}),
	}
	prometheus.MustRegister(r.Requests, r.RateLimited)
	return r
}

func (r *Registry) Handler() http.Handler {
	return promhttp.Handler()
}
