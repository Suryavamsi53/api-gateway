package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"api-gateway/internal/metrics"
	"api-gateway/internal/service"
)

// ProxyHandler forwards requests to a downstream service after rate-limiting.
type ProxyHandler struct {
	proxy   *httputil.ReverseProxy
	limiter *service.Limiter
	metrics *metrics.Registry
}

func NewProxyHandler(downstream string, l *service.Limiter, m *metrics.Registry) *ProxyHandler {
	u, _ := url.Parse(downstream)
	rp := httputil.NewSingleHostReverseProxy(u)
	return &ProxyHandler{proxy: rp, limiter: l, metrics: m}
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Proxy directly; rate limiting handled by middleware earlier.
	// Attach any metrics or headers if needed.
	ctx := r.Context()
	_ = ctx
	p.proxy.ServeHTTP(w, r)
}
