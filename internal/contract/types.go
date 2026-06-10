package contract

const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	StatusInvalid  = "invalid"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusTimeout  = "timeout"
	StatusDegraded = "degraded"
)

type PluginRequest struct {
	RequestID string         `json:"request_id"`
	Data      map[string]any `json:"data"`
	Context   map[string]any `json:"context,omitempty"`
}

type PluginResponse struct {
	OK    bool           `json:"ok"`
	Data  map[string]any `json:"data,omitempty"`
	Error string         `json:"error,omitempty"`
}

type PluginResult struct {
	Plugin  string         `json:"plugin"`
	Version string         `json:"version"`
	Status  string         `json:"status"`
	Data    map[string]any `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type ExecutionSummary struct {
	RequestID string         `json:"request_id"`
	Results   []PluginResult `json:"results"`
}
