// Package health provides a standard HTTP health check handler for MIST tools.
// Mount it on your server to expose /healthz for liveness probes and
// /readyz for readiness probes.
//
//	h := health.New("matchspec", "1.0.0")
//	srv.Handle("GET /healthz", h.Liveness())
//	srv.Handle("GET /readyz", h.Readiness())
package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Handler provides HTTP health check endpoints.
type Handler struct {
	tool    string
	version string
	started time.Time

	mu     sync.RWMutex
	checks map[string]CheckFunc
	ready  atomic.Bool
}

// CheckFunc is a function that reports whether a dependency is healthy.
// Return nil for healthy, an error describing the problem otherwise.
type CheckFunc func() error

// Response is the JSON body returned by health endpoints.
type Response struct {
	Status  string            `json:"status"`
	Tool    string            `json:"tool"`
	Version string            `json:"version"`
	Uptime  string            `json:"uptime"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// New creates a health handler for the given tool.
func New(tool, version string) *Handler {
	h := &Handler{
		tool:    tool,
		version: version,
		started: time.Now(),
		checks:  make(map[string]CheckFunc),
	}
	h.ready.Store(true)
	return h
}

// AddCheck registers a named dependency check for the readiness probe.
func (h *Handler) AddCheck(name string, fn CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = fn
}

// SetReady marks the service as ready or not ready.
func (h *Handler) SetReady(ready bool) {
	h.ready.Store(ready)
}

// Liveness returns an HTTP handler for /healthz. It always returns 200
// if the process is running — this is for Kubernetes liveness probes
// and load balancer health checks.
func (h *Handler) Liveness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := Response{
			Status:  "ok",
			Tool:    h.tool,
			Version: h.version,
			Uptime:  time.Since(h.started).Round(time.Second).String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// Readiness returns an HTTP handler for /readyz. It runs all registered
// checks and returns 200 only if all pass. Use this for Kubernetes
// readiness probes — it tells the load balancer when the tool is ready
// to accept traffic.
func (h *Handler) Readiness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.ready.Load() {
			resp := Response{
				Status:  "not_ready",
				Tool:    h.tool,
				Version: h.version,
				Uptime:  time.Since(h.started).Round(time.Second).String(),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(resp)
			return
		}

		h.mu.RLock()
		checks := make(map[string]CheckFunc, len(h.checks))
		for k, v := range h.checks {
			checks[k] = v
		}
		h.mu.RUnlock()

		results := make(map[string]string, len(checks))
		allOK := true
		for name, fn := range checks {
			if err := fn(); err != nil {
				results[name] = err.Error()
				allOK = false
			} else {
				results[name] = "ok"
			}
		}

		status := "ok"
		code := http.StatusOK
		if !allOK {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}

		resp := Response{
			Status:  status,
			Tool:    h.tool,
			Version: h.version,
			Uptime:  time.Since(h.started).Round(time.Second).String(),
			Checks:  results,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp)
	}
}
