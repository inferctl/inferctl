package inferctl

type BackendInfo struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	BaseURL   string  `json:"base_url"`
	Reachable bool    `json:"reachable"`
	LatencyMS *int    `json:"latency_ms"`
	Version   *string `json:"version"`
	Default   bool    `json:"default"`
}

type BackoffState struct {
	Active              bool    `json:"active"`
	SecondsRemaining    int     `json:"seconds_remaining"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	SinceISO            *string `json:"since_iso"`
	LastError           *string `json:"last_error"`
}

type BackendStatus struct {
	BackendInfo
	ModelsInstalledCount *int          `json:"models_installed_count"`
	ModelsLoadedCount    *int          `json:"models_loaded_count"`
	Backoff              *BackoffState `json:"backoff"`
}

type ModelInfo struct {
	Name           string  `json:"name"`
	Backend        string  `json:"backend"`
	SizeBytes      *int64  `json:"size_bytes"`
	Digest         *string `json:"digest"`
	InstalledAtISO *string `json:"installed_at_iso"`
}

type LoadedModelInfo struct {
	Name               string   `json:"name"`
	Backend            string   `json:"backend"`
	VRAMBytes          *int64   `json:"vram_bytes"`
	KVCachePressurePct *float64 `json:"kv_cache_pressure_pct"`
	LoadedAtISO        *string  `json:"loaded_at_iso"`
	IdleSeconds        *int     `json:"idle_seconds"`
}

type Capabilities struct {
	SupportsTools       bool   `json:"supports_tools"`
	SupportsVision      bool   `json:"supports_vision"`
	SupportsJSONMode    bool   `json:"supports_json_mode"`
	ContextWindow       *int   `json:"context_window"`
	EmbeddingDimensions *int   `json:"embedding_dimensions"`
	Source              string `json:"source"`
}

type LatencyStats struct {
	Samples            int      `json:"samples"`
	FirstTokenMSP50    *int     `json:"first_token_ms_p50"`
	FirstTokenMSP95    *int     `json:"first_token_ms_p95"`
	TokensPerSecondP50 *float64 `json:"tokens_per_second_p50"`
	LastObservedISO    *string  `json:"last_observed_iso"`
}

type RouteCandidate struct {
	Model                 string  `json:"model"`
	Backend               *string `json:"backend"`
	Role                  string  `json:"role"`
	FallbackIndex         *int    `json:"fallback_index"`
	Available             bool    `json:"available"`
	UnavailabilityReason  *string `json:"unavailability_reason"`
	Loaded                bool    `json:"loaded"`
	EstimatedFirstTokenMS *int    `json:"estimated_first_token_ms"`
}

type RouteDecision struct {
	SelectedModel         string `json:"selected_model"`
	SelectedBackend       string `json:"selected_backend"`
	IsFallback            bool   `json:"is_fallback"`
	FallbackIndex         *int   `json:"fallback_index"`
	Ready                 bool   `json:"ready"`
	EstimatedFirstTokenMS *int   `json:"estimated_first_token_ms"`
	EstimatedTotalMS      *int   `json:"estimated_total_ms"`
	Reason                string `json:"reason"`
}

type RouteConstraints struct {
	Profile             string  `json:"profile"`
	MaxContextTokens    int     `json:"max_context_tokens"`
	ContextUsedTokens   int     `json:"context_used_tokens"`
	ContextPct          float64 `json:"context_pct"`
	MaxConcurrentModels int     `json:"max_concurrent_models"`
	CurrentLoadedCount  int     `json:"current_loaded_count"`
	AllowPremium        bool    `json:"allow_premium"`
	SelectedIsPremium   *bool   `json:"selected_is_premium"`
}

type Finding struct {
	Severity string         `json:"severity"`
	Key      string         `json:"key"`
	Message  string         `json:"message"`
	Line     *int           `json:"line"`
	Column   *int           `json:"column"`
	Details  map[string]any `json:"details"`
}

type ConfigKeyDef struct {
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default"`
	Description string   `json:"description"`
	ValidSet    []string `json:"valid_set"`
	Example     any      `json:"example"`
}

type RecommendedAction struct {
	Command      string              `json:"command"`
	Rationale    string              `json:"rationale"`
	Alternatives []RecommendedOption `json:"alternatives"`
}

type RecommendedOption struct {
	Command   string `json:"command"`
	Rationale string `json:"rationale"`
}
