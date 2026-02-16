// Package metrics provides lightweight, zero-dependency counters, gauges,
// and histograms for MIST tools. All types are concurrent-safe and designed
// for high-throughput recording with minimal overhead.
//
// Usage:
//
//	reg := metrics.NewRegistry()
//	reqCounter := reg.Counter("http_requests_total", "method", "GET")
//	latency := reg.Histogram("request_duration_ms", metrics.DefaultBuckets, "path", "/api")
//
//	reqCounter.Inc()
//	latency.Observe(42.5)
//
//	// Expose via HTTP:
//	http.HandleFunc("/metricsz", reg.Handler())
package metrics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// DefaultBuckets are the default histogram boundaries for latency (milliseconds).
var DefaultBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000}

// Registry holds all metrics for a MIST tool.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry creates an empty metric registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// metricKey builds a deduplication key from name + label pairs.
func metricKey(name string, labels []string) string {
	if len(labels) == 0 {
		return name
	}
	return name + "{" + strings.Join(labels, ",") + "}"
}

// Counter returns a counter with the given name and optional label key-value pairs.
// Calling Counter with the same name and labels returns the same counter.
func (r *Registry) Counter(name string, labels ...string) *Counter {
	key := metricKey(name, labels)

	r.mu.RLock()
	if c, ok := r.counters[key]; ok {
		r.mu.RUnlock()
		return c
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[key]; ok {
		return c
	}
	c := &Counter{name: name, labels: labels}
	r.counters[key] = c
	return c
}

// Gauge returns a gauge with the given name and optional label key-value pairs.
func (r *Registry) Gauge(name string, labels ...string) *Gauge {
	key := metricKey(name, labels)

	r.mu.RLock()
	if g, ok := r.gauges[key]; ok {
		r.mu.RUnlock()
		return g
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[key]; ok {
		return g
	}
	g := &Gauge{name: name, labels: labels}
	r.gauges[key] = g
	return g
}

// Histogram returns a histogram with the given name, bucket boundaries,
// and optional label key-value pairs.
func (r *Registry) Histogram(name string, buckets []float64, labels ...string) *Histogram {
	key := metricKey(name, labels)

	r.mu.RLock()
	if h, ok := r.histograms[key]; ok {
		r.mu.RUnlock()
		return h
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[key]; ok {
		return h
	}
	sorted := make([]float64, len(buckets))
	copy(sorted, buckets)
	sort.Float64s(sorted)
	h := &Histogram{
		name:    name,
		labels:  labels,
		bounds:  sorted,
		buckets: make([]atomic.Int64, len(sorted)),
	}
	h.minBits.Store(math.Float64bits(math.Inf(1)))
	h.maxBits.Store(math.Float64bits(math.Inf(-1)))
	r.histograms[key] = h
	return h
}

// RegistrySnapshot is a point-in-time view of all metrics.
type RegistrySnapshot struct {
	Counters   map[string]CounterSnapshot   `json:"counters,omitempty"`
	Gauges     map[string]GaugeSnapshot     `json:"gauges,omitempty"`
	Histograms map[string]HistogramSnapshot `json:"histograms,omitempty"`
}

// Snapshot returns a point-in-time copy of all registered metrics.
func (r *Registry) Snapshot() RegistrySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := RegistrySnapshot{
		Counters:   make(map[string]CounterSnapshot, len(r.counters)),
		Gauges:     make(map[string]GaugeSnapshot, len(r.gauges)),
		Histograms: make(map[string]HistogramSnapshot, len(r.histograms)),
	}

	for key, c := range r.counters {
		snap.Counters[key] = CounterSnapshot{
			Name:   c.name,
			Labels: c.labels,
			Value:  c.Value(),
		}
	}
	for key, g := range r.gauges {
		snap.Gauges[key] = GaugeSnapshot{
			Name:   g.name,
			Labels: g.labels,
			Value:  g.Value(),
		}
	}
	for key, h := range r.histograms {
		snap.Histograms[key] = h.Snapshot()
	}

	return snap
}

// Handler returns an HTTP handler that serves the current metrics as JSON.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(r.Snapshot())
		if err != nil {
			http.Error(w, "metrics marshal error", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}

// ---- Counter ----

// Counter is a monotonically increasing integer metric.
type Counter struct {
	name   string
	labels []string
	value  atomic.Int64
}

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.value.Add(1) }

// Add increments the counter by n.
func (c *Counter) Add(n int64) { c.value.Add(n) }

// Value returns the current counter value.
func (c *Counter) Value() int64 { return c.value.Load() }

// CounterSnapshot is a point-in-time counter value.
type CounterSnapshot struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
	Value  int64    `json:"value"`
}

// ---- Gauge ----

// Gauge is a metric that can go up and down.
type Gauge struct {
	name   string
	labels []string
	bits   atomic.Uint64 // stored as float64 bits for atomic ops
}

// Set sets the gauge to the given value.
func (g *Gauge) Set(v float64) {
	g.bits.Store(math.Float64bits(v))
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc() { g.Add(1) }

// Dec decrements the gauge by 1.
func (g *Gauge) Dec() { g.Add(-1) }

// Add adds the given value to the gauge (can be negative).
func (g *Gauge) Add(delta float64) {
	for {
		old := g.bits.Load()
		new := math.Float64bits(math.Float64frombits(old) + delta)
		if g.bits.CompareAndSwap(old, new) {
			return
		}
	}
}

// Value returns the current gauge value.
func (g *Gauge) Value() float64 {
	return math.Float64frombits(g.bits.Load())
}

// GaugeSnapshot is a point-in-time gauge value.
type GaugeSnapshot struct {
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
	Value  float64  `json:"value"`
}

// ---- Histogram ----

// Histogram tracks the distribution of observed values using cumulative buckets.
type Histogram struct {
	name    string
	labels  []string
	bounds  []float64      // sorted bucket boundaries
	buckets []atomic.Int64 // raw counts per bucket
	count   atomic.Int64
	sum     atomic.Uint64 // stored as float64 bits
	minBits atomic.Uint64 // stored as float64 bits
	maxBits atomic.Uint64 // stored as float64 bits
}

// Observe records a value.
func (h *Histogram) Observe(v float64) {
	h.count.Add(1)

	// Atomically add to sum.
	for {
		old := h.sum.Load()
		new := math.Float64bits(math.Float64frombits(old) + v)
		if h.sum.CompareAndSwap(old, new) {
			break
		}
	}

	// Lock-free min update.
	for {
		old := h.minBits.Load()
		if v >= math.Float64frombits(old) {
			break
		}
		if h.minBits.CompareAndSwap(old, math.Float64bits(v)) {
			break
		}
	}
	// Lock-free max update.
	for {
		old := h.maxBits.Load()
		if v <= math.Float64frombits(old) {
			break
		}
		if h.maxBits.CompareAndSwap(old, math.Float64bits(v)) {
			break
		}
	}

	// Find the bucket this value falls into and increment its raw count.
	// We store raw (non-cumulative) counts; cumulative is computed at snapshot time.
	for i, bound := range h.bounds {
		if v <= bound {
			h.buckets[i].Add(1)
			return
		}
	}
	// Value exceeds all buckets — no bucket incremented.
}

// HistogramSnapshot is a point-in-time histogram state.
type HistogramSnapshot struct {
	Name    string            `json:"name"`
	Labels  []string          `json:"labels,omitempty"`
	Count   int64             `json:"count"`
	Sum     float64           `json:"sum"`
	Min     float64           `json:"min"`
	Max     float64           `json:"max"`
	Buckets map[float64]int64 `json:"-"` // use custom marshal
	bounds  []float64
}

// MarshalJSON implements custom JSON marshaling to handle float64 map keys.
func (s HistogramSnapshot) MarshalJSON() ([]byte, error) {
	type alias struct {
		Name    string           `json:"name"`
		Labels  []string         `json:"labels,omitempty"`
		Count   int64            `json:"count"`
		Sum     float64          `json:"sum"`
		Min     float64          `json:"min"`
		Max     float64          `json:"max"`
		Buckets map[string]int64 `json:"buckets"`
	}
	a := alias{
		Name: s.Name, Labels: s.Labels,
		Count: s.Count, Sum: s.Sum, Min: s.Min, Max: s.Max,
		Buckets: make(map[string]int64, len(s.Buckets)),
	}
	for k, v := range s.Buckets {
		a.Buckets[fmt.Sprintf("%g", k)] = v
	}
	return json.Marshal(a)
}

// Snapshot returns a point-in-time copy of the histogram state.
func (h *Histogram) Snapshot() HistogramSnapshot {
	min := math.Float64frombits(h.minBits.Load())
	max := math.Float64frombits(h.maxBits.Load())

	snap := HistogramSnapshot{
		Name:    h.name,
		Labels:  h.labels,
		Count:   h.count.Load(),
		Sum:     math.Float64frombits(h.sum.Load()),
		Min:     min,
		Max:     max,
		Buckets: make(map[float64]int64, len(h.bounds)),
		bounds:  h.bounds,
	}

	if snap.Count == 0 {
		snap.Min = 0
		snap.Max = 0
	}

	// Compute cumulative counts from raw per-bucket counts.
	var cumulative int64
	for i, bound := range h.bounds {
		cumulative += h.buckets[i].Load()
		snap.Buckets[bound] = cumulative
	}

	return snap
}

// Avg returns the mean of all observed values.
func (s HistogramSnapshot) Avg() float64 {
	if s.Count == 0 {
		return 0
	}
	return s.Sum / float64(s.Count)
}

// Percentile estimates the given percentile (0-100) from bucket data.
func (s HistogramSnapshot) Percentile(p float64) float64 {
	if s.Count == 0 || len(s.bounds) == 0 {
		return 0
	}

	target := float64(s.Count) * p / 100.0

	prevBound := 0.0
	prevCount := int64(0)

	for _, bound := range s.bounds {
		count := s.Buckets[bound]
		if float64(count) >= target {
			// Linear interpolation within this bucket.
			bucketCount := count - prevCount
			if bucketCount == 0 {
				return bound
			}
			fraction := (target - float64(prevCount)) / float64(bucketCount)
			return prevBound + fraction*(bound-prevBound)
		}
		prevBound = bound
		prevCount = count
	}

	// Beyond all buckets.
	return s.Max
}
