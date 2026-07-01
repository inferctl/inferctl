package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/inferctl/inferctl/internal/envelope"
	internalversion "github.com/inferctl/inferctl/internal/version"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

const snapshotSchemaVersion = "0.1"

const (
	runnabilityRunnable        = "runnable"
	runnabilityInvocationBlock = "invocation_blocked"
	runnabilityPolicyBlock     = "policy_blocked"
	runnabilityReadinessBlock  = "readiness_blocked"
	runnabilityConfigError     = "config_error"
	runnabilityTransientError  = "transient_error"
)

type promptMetadata struct {
	SourceKind      string  `json:"source_kind"`
	Source          string  `json:"source"`
	PromptChars     int     `json:"prompt_chars"`
	EstimatedTokens int     `json:"estimated_tokens"`
	ContentSHA256   *string `json:"content_sha256,omitempty"`
	Filename        *string `json:"filename,omitempty"`
	Basename        *string `json:"basename,omitempty"`
}

type promptReadOptions struct {
	inline      string
	file        string
	fromStdin   bool
	includeHash bool
}

type controlPlaneSnapshot struct {
	SnapshotSchemaVersion string                      `json:"snapshot_schema_version"`
	ContractVersion       string                      `json:"contract_version"`
	InferctlVersion       string                      `json:"inferctl_version"`
	CapturedAtISO         string                      `json:"captured_at_iso"`
	Task                  string                      `json:"task"`
	Prompt                promptMetadata              `json:"prompt"`
	RouteDecision         inferctl.RouteDecision      `json:"route_decision"`
	RouteCandidates       []inferctl.RouteCandidate   `json:"route_candidates"`
	BackendReachability   []backendReachability       `json:"backend_reachability"`
	LoadedModels          []inferctl.LoadedModelInfo  `json:"loaded_models"`
	InstalledModels       []inferctl.ModelInfo        `json:"installed_models"`
	Warnings              []envelope.Warning          `json:"warnings"`
	Errors                []envelope.Error            `json:"errors"`
	RecommendedAction     *inferctl.RecommendedAction `json:"recommended_action,omitempty"`
}

type backendReachability struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Reachable bool    `json:"reachable"`
	BaseURL   string  `json:"base_url"`
	Error     *string `json:"error,omitempty"`
}

type controlPlaneChange struct {
	Rank        int    `json:"rank"`
	Type        string `json:"type"`
	Subject     string `json:"subject"`
	Severity    string `json:"severity"`
	Before      any    `json:"before"`
	After       any    `json:"after"`
	Explanation string `json:"explanation"`
}

func readPromptMetadata(cmd *cobra.Command, opts promptReadOptions) (promptMetadata, *envelope.Error) {
	sources := 0
	if opts.inline != "" {
		sources++
	}
	if opts.file != "" {
		sources++
	}
	if opts.fromStdin {
		sources++
	}
	if sources > 1 {
		did := cmd.CommandPath() + " <task> --prompt-file <path> --json"
		err := envelope.Error{
			Code:       "E_INVALID_ARG",
			Message:    "choose only one prompt source: --prompt, --prompt-file, or --from-stdin",
			DidYouMean: &did,
			ExitCode:   exitUserInput,
			Retryable:  false,
			Details: map[string]any{
				"arg":        "prompt_source",
				"given":      "multiple",
				"expected":   "exactly zero or one prompt source",
				"valid_set":  []string{"--prompt", "--prompt-file", "--from-stdin"},
				"correction": "remove all but one prompt source flag",
			},
		}
		return promptMetadata{}, &err
	}

	var text string
	meta := promptMetadata{SourceKind: "none", Source: "none"}
	switch {
	case opts.inline != "":
		text = opts.inline
		meta.SourceKind = "inline"
		meta.Source = "inline"
	case opts.file != "":
		data, err := os.ReadFile(opts.file)
		if err != nil {
			errObj := envelope.Error{
				Code:      "E_CONFIG_UNREADABLE",
				Message:   "prompt file at " + opts.file + " could not be read: " + err.Error(),
				ExitCode:  3,
				Retryable: false,
				Details:   map[string]any{"path": opts.file, "reason": err.Error()},
			}
			return promptMetadata{}, &errObj
		}
		text = string(data)
		base := filepath.Base(opts.file)
		meta.SourceKind = "file"
		meta.Source = "file"
		meta.Filename = &base
		meta.Basename = &base
	case opts.fromStdin:
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			errObj := envelope.Error{
				Code:      "E_CONFIG_UNREADABLE",
				Message:   "stdin could not be read: " + err.Error(),
				ExitCode:  3,
				Retryable: false,
				Details:   map[string]any{"path": "stdin", "reason": err.Error()},
			}
			return promptMetadata{}, &errObj
		}
		text = string(data)
		meta.SourceKind = "stdin"
		meta.Source = "stdin"
	}

	chars := len([]rune(text))
	meta.PromptChars = chars
	meta.EstimatedTokens = int(math.Ceil(float64(chars) / 4.0))
	if opts.includeHash && sources > 0 {
		sum := sha256.Sum256([]byte(text))
		hash := hex.EncodeToString(sum[:])
		meta.ContentSHA256 = &hash
	}
	return meta, nil
}

func routeInputFromPromptMetadata(meta promptMetadata) routeInput {
	source := meta.SourceKind
	if meta.SourceKind == "file" && meta.Filename != nil {
		source = "file:" + *meta.Filename
	}
	return routeInput{PromptChars: meta.PromptChars, EstimatedTokens: meta.EstimatedTokens, Source: source}
}

func classifyControlPlaneChanges(before, after controlPlaneSnapshot) []controlPlaneChange {
	var changes []controlPlaneChange
	changes = appendStringChange(changes, "selected_route", before.Task, "high", before.RouteDecision.SelectedBackend+"/"+before.RouteDecision.SelectedModel, after.RouteDecision.SelectedBackend+"/"+after.RouteDecision.SelectedModel, "selected route changed")
	changes = appendBoolChange(changes, "fallback_status", before.Task, "high", before.RouteDecision.IsFallback, after.RouteDecision.IsFallback, "fallback status changed")
	changes = appendBoolChange(changes, "selected_model_readiness", after.RouteDecision.SelectedModel, "medium", before.RouteDecision.Ready, after.RouteDecision.Ready, "selected model readiness changed")
	changes = appendStringSetChanges(changes, "backend_reachability", "high", reachabilitySet(before.BackendReachability), reachabilitySet(after.BackendReachability), "backend reachability changed")
	changes = appendStringSetChanges(changes, "warning_codes", "medium", warningCodeSet(before.Warnings), warningCodeSet(after.Warnings), "warning code set changed")
	changes = appendStringSetChanges(changes, "error_codes", "high", errorCodeSet(before.Errors), errorCodeSet(after.Errors), "error code set changed")
	changes = appendStringChange(changes, "recommended_action", before.Task, "medium", recommendedActionCommand(before.RecommendedAction), recommendedActionCommand(after.RecommendedAction), "recommended action changed")
	changes = appendIntChange(changes, "loaded_model_count", "loaded_models", "low", len(before.LoadedModels), len(after.LoadedModels), "loaded model count changed")
	rankChanges(changes)
	return changes
}

func appendStringChange(changes []controlPlaneChange, typ, subject, severity, before, after, explanation string) []controlPlaneChange {
	if before == after {
		return changes
	}
	return append(changes, controlPlaneChange{Type: typ, Subject: subject, Severity: severity, Before: before, After: after, Explanation: explanation})
}

func appendBoolChange(changes []controlPlaneChange, typ, subject, severity string, before, after bool, explanation string) []controlPlaneChange {
	if before == after {
		return changes
	}
	return append(changes, controlPlaneChange{Type: typ, Subject: subject, Severity: severity, Before: before, After: after, Explanation: explanation})
}

func appendIntChange(changes []controlPlaneChange, typ, subject, severity string, before, after int, explanation string) []controlPlaneChange {
	if before == after {
		return changes
	}
	return append(changes, controlPlaneChange{Type: typ, Subject: subject, Severity: severity, Before: before, After: after, Explanation: explanation})
}

func appendStringSetChanges(changes []controlPlaneChange, typ, severity string, before, after map[string]string, explanation string) []controlPlaneChange {
	keys := make([]string, 0, len(before)+len(after))
	seen := map[string]bool{}
	for key := range before {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range after {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		if before[key] == after[key] {
			continue
		}
		changes = append(changes, controlPlaneChange{Type: typ, Subject: key, Severity: severity, Before: before[key], After: after[key], Explanation: explanation})
	}
	return changes
}

func reachabilitySet(items []backendReachability) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		status := "unreachable"
		if item.Reachable {
			status = "reachable"
		}
		out[item.Name] = status
	}
	return out
}

func warningCodeSet(warnings []envelope.Warning) map[string]string {
	out := map[string]string{}
	for _, warning := range warnings {
		out[warning.Code] = "present"
	}
	return out
}

func errorCodeSet(errors []envelope.Error) map[string]string {
	out := map[string]string{}
	for _, errObj := range errors {
		out[errObj.Code] = "present"
	}
	return out
}

func recommendedActionCommand(action *inferctl.RecommendedAction) string {
	if action == nil {
		return ""
	}
	return action.Command
}

func rankChanges(changes []controlPlaneChange) {
	priority := map[string]int{"high": 0, "medium": 1, "low": 2}
	slices.SortFunc(changes, func(a, b controlPlaneChange) int {
		if priority[a.Severity] != priority[b.Severity] {
			return priority[a.Severity] - priority[b.Severity]
		}
		if a.Type != b.Type {
			return strings.Compare(a.Type, b.Type)
		}
		return strings.Compare(a.Subject, b.Subject)
	})
	for i := range changes {
		changes[i].Rank = i + 1
	}
}

func newControlPlaneSnapshot(task string, prompt promptMetadata) controlPlaneSnapshot {
	return controlPlaneSnapshot{
		SnapshotSchemaVersion: snapshotSchemaVersion,
		ContractVersion:       "0.1",
		InferctlVersion:       internalversion.Tool(),
		CapturedAtISO:         deterministicSnapshotTime().Format(time.RFC3339Nano),
		Task:                  task,
		Prompt:                prompt,
		Warnings:              []envelope.Warning{},
		Errors:                []envelope.Error{},
	}
}
