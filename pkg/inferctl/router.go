package inferctl

import "context"

type Router interface {
	Backends(ctx context.Context) ([]BackendStatus, error)
	Models(ctx context.Context) ([]ModelInfo, error)
	Model(ctx context.Context, name string) (*ModelDetail, error)
	Route(ctx context.Context, task string, input RouteInput) (*RouteExplanation, error)
}

type ModelDetail struct {
	Name         string         `json:"name"`
	Backends     []ModelBackend `json:"backends"`
	Capabilities Capabilities   `json:"capabilities"`
	LatencyStats LatencyStats   `json:"latency_stats"`
	Routing      ModelRouting   `json:"routing"`
}

type ModelBackend struct {
	Backend   string  `json:"backend"`
	Installed bool    `json:"installed"`
	Loaded    bool    `json:"loaded"`
	SizeBytes *int64  `json:"size_bytes"`
	Digest    *string `json:"digest"`
}

type ModelRouting struct {
	PrimaryForTasks  []string `json:"primary_for_tasks"`
	FallbackForTasks []string `json:"fallback_for_tasks"`
	FallbackChain    []string `json:"fallback_chain"`
}

type RouteInput struct {
	PromptChars     int    `json:"prompt_chars"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Source          string `json:"source"`
}

type RouteExplanation struct {
	Task        string           `json:"task"`
	Input       RouteInput       `json:"input"`
	Decision    RouteDecision    `json:"decision"`
	Candidates  []RouteCandidate `json:"candidates"`
	Constraints RouteConstraints `json:"constraints"`
}
