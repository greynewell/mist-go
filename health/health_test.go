package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveness(t *testing.T) {
	h := New("matchspec", "1.0.0")
	handler := h.Liveness()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Tool != "matchspec" {
		t.Errorf("tool = %q", resp.Tool)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("version = %q", resp.Version)
	}
	if resp.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestReadinessAllOK(t *testing.T) {
	h := New("infermux", "0.2.0")
	h.AddCheck("database", func() error { return nil })
	h.AddCheck("cache", func() error { return nil })

	handler := h.Readiness()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Checks["database"] != "ok" {
		t.Errorf("database check = %q", resp.Checks["database"])
	}
	if resp.Checks["cache"] != "ok" {
		t.Errorf("cache check = %q", resp.Checks["cache"])
	}
}

func TestReadinessDegraded(t *testing.T) {
	h := New("matchspec", "1.0.0")
	h.AddCheck("database", func() error { return nil })
	h.AddCheck("cache", func() error { return fmt.Errorf("connection refused") })

	handler := h.Readiness()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "degraded" {
		t.Errorf("status = %q, want degraded", resp.Status)
	}
	if resp.Checks["cache"] != "connection refused" {
		t.Errorf("cache check = %q", resp.Checks["cache"])
	}
}

func TestReadinessNotReady(t *testing.T) {
	h := New("matchspec", "1.0.0")
	h.SetReady(false)

	handler := h.Readiness()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "not_ready" {
		t.Errorf("status = %q, want not_ready", resp.Status)
	}
}

func TestSetReady(t *testing.T) {
	h := New("test", "1.0.0")

	handler := h.Readiness()

	// Default: ready.
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Error("should be ready by default")
	}

	// Mark not ready.
	h.SetReady(false)
	w = httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Error("should be not ready")
	}

	// Mark ready again.
	h.SetReady(true)
	w = httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != http.StatusOK {
		t.Error("should be ready again")
	}
}

func TestNoChecks(t *testing.T) {
	h := New("test", "1.0.0")
	handler := h.Readiness()

	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/readyz", nil))

	if w.Code != http.StatusOK {
		t.Error("no checks = healthy")
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Checks) != 0 {
		t.Error("checks should be empty")
	}
}

func TestContentType(t *testing.T) {
	h := New("test", "1.0.0")

	for _, handler := range []http.HandlerFunc{h.Liveness(), h.Readiness()} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/", nil))
		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	}
}
