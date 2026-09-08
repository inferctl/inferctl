package inferctl

import "context"

const RouteRequirementsVersionV1 = "v1"

// RouteRequirements is the versioned, caller-owned selection request. Each
// named capability must have supported evidence on the selected candidate.
type RouteRequirements struct {
	Version              string   `json:"version"`
	RequiredCapabilities []string `json:"required_capabilities"`
	AllowFallback        bool     `json:"allow_fallback"`
	RequireReady         bool     `json:"require_ready"`
}

type RouteSelectionRefusal struct {
	Code              string `json:"code"`
	Capability        string `json:"capability,omitempty"`
	RecommendedAction string `json:"recommended_action"`
}

// SelectRouteCandidate applies the same deterministic requirements to every
// caller. It has no backend operations.
func SelectRouteCandidate(candidates []RouteCandidate, requirements RouteRequirements) (*RouteCandidate, *RouteSelectionRefusal) {
	for index := range candidates {
		candidate := &candidates[index]
		if !candidate.Available || (!requirements.AllowFallback && candidate.Role == "fallback") || (requirements.RequireReady && !candidate.Loaded) {
			continue
		}
		for _, capability := range requirements.RequiredCapabilities {
			evidence, ok := candidate.Capabilities[capability]
			if !ok || evidence.Status != "supported" {
				goto next
			}
		}
		return candidate, nil
	next:
	}
	for _, capability := range requirements.RequiredCapabilities {
		return nil, &RouteSelectionRefusal{Code: "E_ROUTE_REQUIREMENTS_UNSATISFIED", Capability: capability, RecommendedAction: "inferctl config explain --key models.<alias>.capabilities.<capability>.status --json"}
	}
	return nil, &RouteSelectionRefusal{Code: "E_NO_ROUTE_AVAILABLE", RecommendedAction: "inferctl doctor --json"}
}

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
