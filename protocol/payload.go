package protocol

// InferRequest is sent to InferMux to perform LLM inference.
type InferRequest struct {
	Model    string            `json:"model"`              // model name or "auto" for routing
	Provider string            `json:"provider,omitempty"` // explicit provider or empty for auto
	Messages []ChatMessage     `json:"messages"`
	Params   map[string]any    `json:"params,omitempty"` // temperature, max_tokens, etc.
	Meta     map[string]string `json:"meta,omitempty"`   // trace context, request tags
}

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// InferResponse is returned by InferMux after inference completes.
type InferResponse struct {
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	Content      string  `json:"content"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMS    int64   `json:"latency_ms"`
	FinishReason string  `json:"finish_reason"`
}

// EvalRun starts an evaluation job in MatchSpec.
type EvalRun struct {
	Suite    string            `json:"suite"`               // benchmark suite name
	Tasks    []string          `json:"tasks,omitempty"`     // specific tasks, or empty for all
	Baseline bool              `json:"baseline"`            // run without tools as baseline
	InferURL string            `json:"infer_url,omitempty"` // InferMux endpoint
	Tags     map[string]string `json:"tags,omitempty"`      // metadata tags
}

// EvalResult is the outcome of a single evaluation task.
type EvalResult struct {
	Suite      string  `json:"suite"`
	Task       string  `json:"task"`
	Passed     bool    `json:"passed"`
	Score      float64 `json:"score"`
	Baseline   float64 `json:"baseline_score"`
	Delta      float64 `json:"delta"`
	DurationMS int64   `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
}

// TraceSpan is a single span in a trace, following the MIST Token Trace
// Protocol (MTTP). Spans capture token-level metrics for LLM operations.
type TraceSpan struct {
	TraceID   string         `json:"trace_id"`
	SpanID    string         `json:"span_id"`
	ParentID  string         `json:"parent_id,omitempty"`
	Operation string         `json:"operation"`
	StartNS   int64          `json:"start_ns"`
	EndNS     int64          `json:"end_ns"`
	Status    string         `json:"status"` // "ok", "error"
	Attrs     map[string]any `json:"attrs,omitempty"`
}

// TraceAlert is emitted by TokenTrace when a threshold is breached.
type TraceAlert struct {
	Level     string  `json:"level"`  // "warning", "critical"
	Metric    string  `json:"metric"` // "latency_p99", "cost_hourly", "error_rate"
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

// DataEntities is a batch of structured entities from SchemaFlux.
type DataEntities struct {
	Count    int    `json:"count"`
	Format   string `json:"format"`           // "json", "jsonl", "csv"
	Path     string `json:"path"`             // file path to entity data
	Schema   string `json:"schema,omitempty"` // schema identifier
	Checksum string `json:"checksum,omitempty"`
}

// DataSchema describes a schema definition from SchemaFlux.
type DataSchema struct {
	Name   string        `json:"name"`
	Fields []SchemaField `json:"fields"`
}

// SchemaField is a single field in a schema.
type SchemaField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// HealthPing is a liveness check.
type HealthPing struct {
	From string `json:"from"`
}

// HealthPong is a liveness response.
type HealthPong struct {
	From    string `json:"from"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime_s"`
}
