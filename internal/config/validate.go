package config

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/inferctl/inferctl/pkg/inferctl"
)

type ValidationSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type ValidationResult struct {
	SourcePath *string            `json:"source_path"`
	Findings   []inferctl.Finding `json:"findings"`
	Summary    ValidationSummary  `json:"summary"`
	Passed     bool               `json:"passed"`
}

func Validate(result *Result, strict bool) ValidationResult {
	findings := []inferctl.Finding{}
	cfg := result.Config
	pos := result.Positions

	findings = append(findings, unknownKeyFindings(cfg, pos)...)

	requireKey(&findings, pos, "meta.schema_version", "missing required key")
	requireKey(&findings, pos, "profile.name", "missing required key")
	requireKey(&findings, pos, "profile.max_context_tokens", "missing required key")
	requireKey(&findings, pos, "profile.max_concurrent_models", "missing required key")
	requireKey(&findings, pos, "profile.allow_premium", "missing required key")
	requireKey(&findings, pos, "profile.mode", "missing required key")

	if cfg.Meta.SchemaVersion != "" && hasKey(pos, "meta.schema_version") && cfg.Meta.SchemaVersion != "0.1" {
		findings = append(findings, warning(pos, "meta.schema_version", "config schema_version does not match expected 0.1", map[string]any{
			"code":     "W_CONFIG_SCHEMA_VERSION_MISMATCH",
			"got":      cfg.Meta.SchemaVersion,
			"expected": "0.1",
		}))
	}
	if len(cfg.Backends) == 0 {
		findings = append(findings, derivedError("backends", "at least one backend must be configured", map[string]any{}))
	}

	defaults := 0
	for name, backend := range cfg.Backends {
		prefix := "backends." + name + "."
		requireKey(&findings, pos, prefix+"kind", "missing required key")
		requireKey(&findings, pos, prefix+"base_url", "missing required key")
		if backend.Default {
			defaults++
		}
		if backend.Kind != "" && !slices.Contains([]string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"}, backend.Kind) {
			findings = append(findings, errorFinding(pos, prefix+"kind", "backend kind is not recognized", map[string]any{"valid_set": []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"}}))
		}
		if backend.BaseURL != "" {
			parsed, err := url.Parse(backend.BaseURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				findings = append(findings, errorFinding(pos, prefix+"base_url", "value must be a valid URL with scheme and host", map[string]any{}))
			}
		}
	}
	if len(cfg.Backends) > 0 && defaults != 1 {
		findings = append(findings, derivedError("backends.*.default", "exactly one backend must set default=true", map[string]any{"default_count": defaults}))
	}

	if cfg.Profile.MaxContextTokens <= 0 && hasKey(pos, "profile.max_context_tokens") {
		findings = append(findings, errorFinding(pos, "profile.max_context_tokens", "value must be > 0", map[string]any{}))
	}
	if cfg.Profile.MaxConcurrentModels < 1 && hasKey(pos, "profile.max_concurrent_models") {
		findings = append(findings, errorFinding(pos, "profile.max_concurrent_models", "value must be >= 1", map[string]any{}))
	}
	if cfg.Profile.Mode != "" && !slices.Contains([]string{"strict", "warn", "advisory"}, cfg.Profile.Mode) {
		findings = append(findings, errorFinding(pos, "profile.mode", "profile.mode must be one of strict, warn, advisory", map[string]any{"valid_set": []string{"strict", "warn", "advisory"}}))
	}
	if cfg.Profile.Mode == "strict" || cfg.Profile.Mode == "advisory" {
		findings = append(findings, warning(pos, "profile.mode", "v0.1 enforces warn semantics regardless of profile.mode", map[string]any{
			"code": "W_PROFILE_MODE_NOT_ENFORCED",
			"mode": cfg.Profile.Mode,
		}))
	}

	for task, route := range cfg.Routing {
		prefix := "routing." + task + "."
		requireKey(&findings, pos, prefix+"model", "missing required key")
		requireKey(&findings, pos, prefix+"backend", "missing required key")
		if route.Backend != "" {
			if _, ok := cfg.Backends[route.Backend]; !ok {
				findings = append(findings, errorFinding(pos, prefix+"backend", "routing backend references a non-existent backend", map[string]any{"backend": route.Backend}))
			}
		}
		if route.NumCtx != nil && *route.NumCtx > cfg.Profile.MaxContextTokens {
			findings = append(findings, errorFinding(pos, prefix+"num_ctx", "routing num_ctx exceeds profile.max_context_tokens", map[string]any{
				"num_ctx":            *route.NumCtx,
				"max_context_tokens": cfg.Profile.MaxContextTokens,
			}))
		}
		for i, fallback := range route.Fallback {
			if fallback == route.Model {
				findings = append(findings, derivedError(fmt.Sprintf("%sfallback[%d]", prefix, i), "fallback chain contains the primary model", map[string]any{
					"related_keys": []string{prefix + "model", prefix + "fallback"},
				}))
			}
		}
	}

	summary := summarize(findings)
	return ValidationResult{
		SourcePath: result.SourcePaths.Selected,
		Findings:   findings,
		Summary:    summary,
		Passed:     summary.Errors == 0 && (!strict || summary.Warnings == 0),
	}
}

func requireKey(findings *[]inferctl.Finding, pos map[string]Position, key, message string) {
	if !hasKey(pos, key) {
		*findings = append(*findings, derivedError(key, message, map[string]any{"kind": "missing_required"}))
	}
}

func hasKey(pos map[string]Position, key string) bool {
	_, ok := pos[key]
	return ok
}

func errorFinding(pos map[string]Position, key, message string, details map[string]any) inferctl.Finding {
	return finding("error", pos, key, message, details)
}

func warning(pos map[string]Position, key, message string, details map[string]any) inferctl.Finding {
	return finding("warning", pos, key, message, details)
}

func derivedError(key, message string, details map[string]any) inferctl.Finding {
	return inferctl.Finding{
		Severity: "error",
		Key:      key,
		Message:  message,
		Details:  details,
	}
}

func finding(severity string, pos map[string]Position, key, message string, details map[string]any) inferctl.Finding {
	var line, column *int
	if p, ok := pos[key]; ok {
		line = &p.Line
		column = &p.Column
	}
	return inferctl.Finding{
		Severity: severity,
		Key:      key,
		Message:  message,
		Line:     line,
		Column:   column,
		Details:  details,
	}
}

func summarize(findings []inferctl.Finding) ValidationSummary {
	var summary ValidationSummary
	for _, finding := range findings {
		switch finding.Severity {
		case "error":
			summary.Errors++
		case "warning":
			summary.Warnings++
		case "info":
			summary.Info++
		}
	}
	return summary
}

func unknownKeyFindings(cfg Config, pos map[string]Position) []inferctl.Finding {
	candidates := knownConfigKeyCandidates(cfg)
	findings := []inferctl.Finding{}
	keys := make([]string, 0, len(pos))
	for key := range pos {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if knownConfigKey(cfg, key) {
			continue
		}
		nearest, distance := nearestConfigKey(key, candidates)
		details := map[string]any{"code": "E_CONFIG_KEY_UNKNOWN", "kind": "unknown_key"}
		if nearest != "" && distance <= 6 {
			details["did_you_mean"] = nearest
			details["remediation"] = "inferctl config explain --key " + nearest + " --json"
			details["distance"] = distance
		}
		findings = append(findings, errorFinding(pos, key, "config key is not recognized", details))
	}
	return findings
}

func knownConfigKey(cfg Config, key string) bool {
	switch key {
	case "meta", "meta.schema_version",
		"profile", "profile.name", "profile.max_context_tokens", "profile.max_concurrent_models", "profile.allow_premium", "profile.mode", "profile.vram_total_bytes_hint",
		"backends", "routing":
		return true
	}
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return false
	}
	switch parts[0] {
	case "backends":
		if len(parts) == 2 {
			_, ok := cfg.Backends[parts[1]]
			return ok
		}
		if len(parts) != 3 {
			return false
		}
		if _, ok := cfg.Backends[parts[1]]; !ok {
			return false
		}
		return slices.Contains(backendConfigFields(), parts[2])
	case "routing":
		if len(parts) == 2 {
			_, ok := cfg.Routing[parts[1]]
			return ok
		}
		if len(parts) != 3 {
			return false
		}
		if _, ok := cfg.Routing[parts[1]]; !ok {
			return false
		}
		return slices.Contains(routingConfigFields(), parts[2])
	default:
		return false
	}
}

func knownConfigKeyCandidates(cfg Config) []string {
	keys := []string{
		"meta", "meta.schema_version",
		"profile", "profile.name", "profile.max_context_tokens", "profile.max_concurrent_models", "profile.allow_premium", "profile.mode", "profile.vram_total_bytes_hint",
		"backends", "routing",
	}
	for name := range cfg.Backends {
		prefix := "backends." + name
		keys = append(keys, prefix)
		for _, field := range backendConfigFields() {
			keys = append(keys, prefix+"."+field)
		}
	}
	for task := range cfg.Routing {
		prefix := "routing." + task
		keys = append(keys, prefix)
		for _, field := range routingConfigFields() {
			keys = append(keys, prefix+"."+field)
		}
	}
	sort.Strings(keys)
	return keys
}

func backendConfigFields() []string {
	return []string{
		"auth_header_name",
		"auth_header_value",
		"base_url",
		"default",
		"fallback_chain_position",
		"kind",
		"remote_allowed",
		"timeout_ms",
	}
}

func routingConfigFields() []string {
	return []string{"backend", "fallback", "model", "num_ctx"}
}

func nearestConfigKey(given string, candidates []string) (string, int) {
	best := ""
	bestDistance := 1 << 30
	for _, candidate := range candidates {
		distance := levenshteinDistance(given, candidate)
		if distance < bestDistance || (distance == bestDistance && candidate < best) {
			best = candidate
			bestDistance = distance
		}
	}
	return best, bestDistance
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(
				current[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev = current
	}
	return prev[len(b)]
}
