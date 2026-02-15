package metrics

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCounterIncrement(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("requests_total", "method", "GET")

	c.Inc()
	c.Inc()
	c.Inc()

	if c.Value() != 3 {
		t.Errorf("value = %d, want 3", c.Value())
	}
}

func TestCounterAdd(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("bytes_total")

	c.Add(100)
	c.Add(200)

	if c.Value() != 300 {
		t.Errorf("value = %d, want 300", c.Value())
	}
}

func TestCounterLabels(t *testing.T) {
	r := NewRegistry()
	c1 := r.Counter("http_requests", "method", "GET")
	c2 := r.Counter("http_requests", "method", "POST")

	c1.Inc()
	c1.Inc()
	c2.Inc()

	if c1.Value() != 2 {
		t.Errorf("GET = %d, want 2", c1.Value())
	}
	if c2.Value() != 1 {
		t.Errorf("POST = %d, want 1", c2.Value())
	}
}

func TestCounterSameLabelsReturnsSame(t *testing.T) {
	r := NewRegistry()
	c1 := r.Counter("test", "a", "1")
	c2 := r.Counter("test", "a", "1")

	c1.Inc()
	if c2.Value() != 1 {
		t.Error("same labels should return same counter")
	}
}

func TestGaugeSet(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("temperature")

	g.Set(98.6)
	if g.Value() != 98.6 {
		t.Errorf("value = %f, want 98.6", g.Value())
	}

	g.Set(100.0)
	if g.Value() != 100.0 {
		t.Errorf("value = %f, want 100.0", g.Value())
	}
}

func TestGaugeIncDec(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("active_connections")

	g.Inc()
	g.Inc()
	g.Inc()
	g.Dec()

	if g.Value() != 2.0 {
		t.Errorf("value = %f, want 2.0", g.Value())
	}
}

func TestGaugeAdd(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("queue_depth")

	g.Add(10)
	g.Add(-3)

	if g.Value() != 7.0 {
		t.Errorf("value = %f, want 7.0", g.Value())
	}
}

func TestHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("request_duration_ms", DefaultBuckets)

	h.Observe(5)
	h.Observe(15)
	h.Observe(150)
	h.Observe(500)
	h.Observe(1500)

	snap := h.Snapshot()
	if snap.Count != 5 {
		t.Errorf("count = %d, want 5", snap.Count)
	}
	if snap.Sum != 2170 {
		t.Errorf("sum = %f, want 2170", snap.Sum)
	}
	if snap.Min != 5 {
		t.Errorf("min = %f, want 5", snap.Min)
	}
	if snap.Max != 1500 {
		t.Errorf("max = %f, want 1500", snap.Max)
	}
}

func TestHistogramBuckets(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("latency", []float64{10, 50, 100, 500})

	h.Observe(5)   // bucket ≤10
	h.Observe(25)  // bucket ≤50
	h.Observe(75)  // bucket ≤100
	h.Observe(200) // bucket ≤500
	h.Observe(800) // over all buckets

	snap := h.Snapshot()
	if snap.Buckets[10] != 1 {
		t.Errorf("bucket[10] = %d, want 1", snap.Buckets[10])
	}
	if snap.Buckets[50] != 2 {
		t.Errorf("bucket[50] = %d, want 2 (cumulative)", snap.Buckets[50])
	}
	if snap.Buckets[100] != 3 {
		t.Errorf("bucket[100] = %d, want 3", snap.Buckets[100])
	}
	if snap.Buckets[500] != 4 {
		t.Errorf("bucket[500] = %d, want 4", snap.Buckets[500])
	}
}

func TestHistogramAvg(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("test", DefaultBuckets)

	h.Observe(10)
	h.Observe(20)
	h.Observe(30)

	snap := h.Snapshot()
	if snap.Avg() != 20 {
		t.Errorf("avg = %f, want 20", snap.Avg())
	}
}

func TestHistogramAvgEmpty(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("test", DefaultBuckets)

	snap := h.Snapshot()
	if snap.Avg() != 0 {
		t.Errorf("avg = %f, want 0 for empty", snap.Avg())
	}
}

func TestRegistrySnapshot(t *testing.T) {
	r := NewRegistry()
	r.Counter("req_total").Inc()
	r.Gauge("connections").Set(42)
	r.Histogram("latency", DefaultBuckets).Observe(10)

	snap := r.Snapshot()
	if len(snap.Counters) != 1 {
		t.Errorf("counters = %d, want 1", len(snap.Counters))
	}
	if len(snap.Gauges) != 1 {
		t.Errorf("gauges = %d, want 1", len(snap.Gauges))
	}
	if len(snap.Histograms) != 1 {
		t.Errorf("histograms = %d, want 1", len(snap.Histograms))
	}
}

func TestHandlerReturnsJSON(t *testing.T) {
	r := NewRegistry()
	r.Counter("test_counter").Add(5)
	r.Gauge("test_gauge").Set(3.14)
	r.Histogram("test_hist", DefaultBuckets).Observe(42)

	handler := r.Handler()
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest("GET", "/metricsz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestHistogramP50P99(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("test", []float64{10, 20, 50, 100, 200, 500, 1000})

	// Add observations to create a distribution.
	for i := 0; i < 50; i++ {
		h.Observe(5) // bucket ≤10
	}
	for i := 0; i < 30; i++ {
		h.Observe(15) // bucket ≤20
	}
	for i := 0; i < 20; i++ {
		h.Observe(150) // bucket ≤200
	}

	snap := h.Snapshot()
	p50 := snap.Percentile(50)
	p99 := snap.Percentile(99)

	// p50 should be in the ≤10 bucket range (50th percentile of 100 items, 50 are ≤10).
	if p50 > 20 {
		t.Errorf("p50 = %f, expected ≤20", p50)
	}
	// p99 should be in the ≤200 bucket range.
	if p99 > 500 {
		t.Errorf("p99 = %f, expected ≤500", p99)
	}
}

func TestDefaultBuckets(t *testing.T) {
	if len(DefaultBuckets) == 0 {
		t.Error("DefaultBuckets should not be empty")
	}
	// Should be sorted.
	for i := 1; i < len(DefaultBuckets); i++ {
		if DefaultBuckets[i] <= DefaultBuckets[i-1] {
			t.Errorf("DefaultBuckets not sorted at index %d", i)
		}
	}
}

func TestHistogramLabels(t *testing.T) {
	r := NewRegistry()
	h1 := r.Histogram("http_duration", DefaultBuckets, "path", "/api")
	h2 := r.Histogram("http_duration", DefaultBuckets, "path", "/health")

	h1.Observe(10)
	h2.Observe(20)

	s1 := h1.Snapshot()
	s2 := h2.Snapshot()

	if s1.Count != 1 || s2.Count != 1 {
		t.Error("labeled histograms should be independent")
	}
}

func TestGaugeNegative(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("temperature")

	g.Set(-40)
	if g.Value() != -40 {
		t.Errorf("value = %f, want -40", g.Value())
	}
}

func TestHistogramInfinity(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("test", []float64{10, 100})

	h.Observe(math.Inf(1))
	snap := h.Snapshot()
	if snap.Count != 1 {
		t.Errorf("count = %d, want 1", snap.Count)
	}
}
