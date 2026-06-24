package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/inferctl/inferctl/internal/config"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type routeReport struct {
	Task        string                    `json:"task"`
	Input       routeInput                `json:"input"`
	Decision    inferctl.RouteDecision    `json:"decision"`
	Candidates  []inferctl.RouteCandidate `json:"candidates"`
	Constraints inferctl.RouteConstraints `json:"constraints"`
}

type routeInput struct {
	PromptChars     int    `json:"prompt_chars"`
	EstimatedTokens int    `json:"estimated_tokens"`
	Source          string `json:"source"`
}

type routePromptOptions struct {
	inline    string
	file      string
	fromStdin bool
}

func newRouteCommand(jsonFlag *bool) *cobra.Command {
	var prompt routePromptOptions
	var prefer string
	var explain bool
	var quiet bool
	cmd := &cobra.Command{
		Use:   "route <task>",
		Short: "Select and explain a configured model route",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return writeError(cmd, *jsonFlag, envelope.Error{
					Code:       "E_MISSING_ARG",
					Message:    "verb 'route' requires task",
					DidYouMean: stringPtr("inferctl route <task>"),
					ExitCode:   1,
					Retryable:  false,
					Details:    map[string]any{"verb": "route", "missing": "task"},
				})
			}
			if len(args) > 2 || (len(args) == 2 && args[1] != "-") {
				return writeError(cmd, *jsonFlag, invalidArg("args", strings.Join(args, " "), "one task and optional '-' stdin marker", nil))
			}
			if len(args) == 2 && args[1] == "-" {
				prompt.fromStdin = true
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prefer != "default" && prefer != "speed" && prefer != "quality" {
				return writeError(cmd, *jsonFlag, invalidArg("--prefer", prefer, "one of default, speed, quality", []string{"default", "speed", "quality"}))
			}
			input, errObj := readRouteInput(cmd, prompt)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			if len(result.Config.Backends) == 0 {
				return writeError(cmd, *jsonFlag, noBackendsError(result))
			}
			if validation := config.Validate(result, false); validation.Summary.Errors > 0 {
				return writeError(cmd, *jsonFlag, validationFailedError(validation))
			}
			routeCfg, ok := result.Config.Routing[args[0]]
			if !ok {
				return writeError(cmd, *jsonFlag, unknownTaskError(args[0], result.Config))
			}
			entries, backendsErr := configuredBackends(result, "", "")
			if backendsErr != nil {
				return writeError(cmd, *jsonFlag, *backendsErr)
			}
			if errObj := firstFatalBackendReadError(context.Background(), entries); errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			report, warnings, commands, noRoute := buildRouteReport(context.Background(), result.Config, args[0], routeCfg, entries, input)
			if noRoute != nil {
				return writeError(cmd, *jsonFlag, *noRoute)
			}
			return writeDataWithDiagnostics(cmd, *jsonFlag, report, warnings, commands, func() error {
				return writeRouteHuman(cmd, report, explain, quiet)
			})
		},
	}
	cmd.Flags().StringVar(&prompt.file, "prompt-file", "", "read prompt from file")
	cmd.Flags().StringVar(&prompt.inline, "prompt", "", "inline prompt text")
	cmd.Flags().BoolVar(&prompt.fromStdin, "from-stdin", false, "read prompt from stdin")
	cmd.Flags().StringVar(&prefer, "prefer", "default", "routing preference: default, speed, or quality")
	cmd.Flags().BoolVar(&explain, "explain", true, "include route explanation in human output")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "only print the selected route in human output")
	return cmd
}

func readRouteInput(cmd *cobra.Command, opts routePromptOptions) (routeInput, *envelope.Error) {
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
		err := invalidArg("prompt_source", "multiple", "choose only one of --prompt, --prompt-file, or --from-stdin", []string{"--prompt", "--prompt-file", "--from-stdin"})
		return routeInput{}, &err
	}
	var text string
	source := "none"
	switch {
	case opts.inline != "":
		text = opts.inline
		source = "inline"
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
			return routeInput{}, &errObj
		}
		text = string(data)
		source = "file:" + opts.file
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
			return routeInput{}, &errObj
		}
		text = string(data)
		source = "stdin"
	}
	chars := len([]rune(text))
	return routeInput{PromptChars: chars, EstimatedTokens: int(math.Ceil(float64(chars) / 4.0)), Source: source}, nil
}

func buildRouteReport(ctx context.Context, cfg config.Config, task string, routeCfg config.RoutingConfig, entries []backendEntry, input routeInput) (routeReport, []envelope.Warning, []envelope.Command, *envelope.Error) {
	state := probeRouteBackends(ctx, entries)
	candidates := routeCandidates(routeCfg)
	var warnings []envelope.Warning
	var selected *inferctl.RouteCandidate
	for i := range candidates {
		candidate := evaluateRouteCandidate(candidates[i], routeCfg, entries, state)
		candidates[i] = candidate
		if candidate.Available && selected == nil {
			selected = &candidates[i]
		}
	}
	for _, entry := range entries {
		if err, ok := state.reachableErrors[entry.name]; ok {
			warnings = append(warnings, backendWarning("W_BACKEND_UNREACHABLE", entry.name, "backend '"+entry.name+"' is unreachable", err))
		}
	}
	if selected == nil {
		return routeReport{}, warnings, nil, noRouteAvailableError(task, candidates)
	}
	if selected.Role == "fallback" {
		warnings = append(warnings, envelope.Warning{
			Code:    "W_FALLBACK_USED",
			Message: "routed to fallback '" + selected.Model + "' because primary '" + routeCfg.Model + "' is unavailable",
			Details: map[string]any{"task": task, "primary": routeCfg.Model, "selected": selected.Model, "fallback_index": selected.FallbackIndex},
		})
	}
	if !selected.Loaded {
		warnings = append(warnings, envelope.Warning{
			Code:    "W_MODEL_NOT_LOADED",
			Message: "selected model '" + selected.Model + "' is not loaded",
			Details: map[string]any{"task": task, "model": selected.Model, "backend": selected.Backend, "estimated_warmup_ms": nil},
		})
	}
	constraints := routeConstraints(cfg, input, state.loadedCount)
	if constraints.ContextPct >= 90 {
		warnings = append(warnings, envelope.Warning{
			Code:    "W_CONTEXT_NEAR_LIMIT",
			Message: "prompt is near the configured context limit",
			Details: map[string]any{"task": task, "context_pct": constraints.ContextPct, "estimated_tokens": input.EstimatedTokens, "max_context_tokens": constraints.MaxContextTokens},
		})
	}
	decision := inferctl.RouteDecision{
		SelectedModel:         selected.Model,
		SelectedBackend:       derefString(selected.Backend),
		IsFallback:            selected.Role == "fallback",
		FallbackIndex:         selected.FallbackIndex,
		Ready:                 selected.Loaded,
		EstimatedFirstTokenMS: nil,
		EstimatedTotalMS:      nil,
		Reason:                routeDecisionReason(selected, routeCfg),
	}
	report := routeReport{
		Task:        task,
		Input:       input,
		Decision:    decision,
		Candidates:  candidates,
		Constraints: constraints,
	}
	return report, warnings, routeCommands(report), nil
}

type routeBackendState struct {
	reachable       map[string]bool
	reachableErrors map[string]error
	installed       map[string]map[string]bool
	loaded          map[string]map[string]bool
	loadedCount     int
}

func probeRouteBackends(ctx context.Context, entries []backendEntry) routeBackendState {
	state := routeBackendState{
		reachable:       map[string]bool{},
		reachableErrors: map[string]error{},
		installed:       map[string]map[string]bool{},
		loaded:          map[string]map[string]bool{},
	}
	for _, entry := range entries {
		if _, err := entry.backend.Reachable(ctx); err != nil {
			state.reachableErrors[entry.name] = err
			continue
		}
		state.reachable[entry.name] = true
		state.installed[entry.name] = map[string]bool{}
		state.loaded[entry.name] = map[string]bool{}
		for _, model := range mustInstalled(ctx, entry.backend) {
			state.installed[entry.name][model.Name] = true
		}
		loaded := mustLoaded(ctx, entry.backend)
		state.loadedCount += len(loaded)
		for _, model := range loaded {
			state.loaded[entry.name][model.Name] = true
		}
	}
	return state
}

func routeCandidates(routeCfg config.RoutingConfig) []inferctl.RouteCandidate {
	candidates := []inferctl.RouteCandidate{{
		Model: routeCfg.Model,
		Role:  "primary",
	}}
	if routeCfg.Backend != "" {
		candidates[0].Backend = &routeCfg.Backend
	}
	for i, model := range routeCfg.Fallback {
		idx := i
		candidates = append(candidates, inferctl.RouteCandidate{
			Model:         model,
			Role:          "fallback",
			FallbackIndex: &idx,
		})
	}
	return candidates
}

func evaluateRouteCandidate(candidate inferctl.RouteCandidate, routeCfg config.RoutingConfig, entries []backendEntry, state routeBackendState) inferctl.RouteCandidate {
	var names []string
	if candidate.Backend != nil {
		names = []string{*candidate.Backend}
	} else if candidate.Role == "primary" && routeCfg.Backend != "" {
		names = []string{routeCfg.Backend}
	} else {
		names = backendNames(entries)
	}
	seenReachable := false
	seenInstalled := false
	for _, name := range names {
		if !state.reachable[name] {
			continue
		}
		seenReachable = true
		if !state.installed[name][candidate.Model] {
			continue
		}
		seenInstalled = true
		candidate.Backend = &name
		candidate.Available = true
		candidate.Loaded = state.loaded[name][candidate.Model]
		return candidate
	}
	reason := "not_installed"
	if !seenReachable {
		reason = "backend_unreachable"
	} else if !seenInstalled {
		reason = "not_installed"
	}
	candidate.UnavailabilityReason = &reason
	return candidate
}

func routeConstraints(cfg config.Config, input routeInput, loadedCount int) inferctl.RouteConstraints {
	maxContext := cfg.Profile.MaxContextTokens
	pct := 0.0
	if maxContext > 0 {
		pct = float64(input.EstimatedTokens) / float64(maxContext) * 100
	}
	return inferctl.RouteConstraints{
		Profile:             cfg.Profile.Name,
		MaxContextTokens:    maxContext,
		ContextUsedTokens:   input.EstimatedTokens,
		ContextPct:          pct,
		MaxConcurrentModels: cfg.Profile.MaxConcurrentModels,
		CurrentLoadedCount:  loadedCount,
		AllowPremium:        cfg.Profile.AllowPremium,
		SelectedIsPremium:   nil,
	}
}

func routeCommands(report routeReport) []envelope.Command {
	var commands []envelope.Command
	if !report.Decision.Ready {
		commands = append(commands, envelope.Command{
			Label:              "Warm the selected model",
			Command:            "inferctl warmup " + report.Decision.SelectedModel,
			Rationale:          "Eliminate first-token latency before issuing the request",
			AvailableInVersion: stringPtr("0.5"),
		})
	}
	if report.Decision.IsFallback {
		for _, candidate := range report.Candidates {
			if candidate.Role == "primary" && candidate.Backend != nil && candidate.UnavailabilityReason != nil && *candidate.UnavailabilityReason == "backend_unreachable" {
				commands = append(commands, envelope.Command{
					Label:     "Inspect the primary backend",
					Command:   "inferctl backends --filter " + *candidate.Backend + " --json",
					Rationale: "See when the primary becomes available again",
				})
				break
			}
		}
	}
	if report.Constraints.ContextPct >= 90 {
		commands = append(commands, envelope.Command{
			Label:     "Inspect context limit",
			Command:   "inferctl config show --key profile.max_context_tokens --json",
			Rationale: "Compare the prompt estimate with the configured context budget",
		})
	}
	commands = append(commands, envelope.Command{
		Label:     "Inspect selected model",
		Command:   "inferctl model " + report.Decision.SelectedModel + " --json",
		Rationale: "Show placements, capabilities, and routing usage for the selected model",
	})
	if len(commands) > 6 {
		commands = commands[:6]
	}
	return commands
}

func routeDecisionReason(selected *inferctl.RouteCandidate, routeCfg config.RoutingConfig) string {
	if selected.Role == "primary" {
		return "primary model is available"
	}
	return "selected fallback because primary '" + routeCfg.Model + "' is unavailable"
}

func unknownTaskError(task string, cfg config.Config) envelope.Error {
	tasks := make([]string, 0, len(cfg.Routing))
	for name := range cfg.Routing {
		tasks = append(tasks, name)
	}
	slices.Sort(tasks)
	did := "inferctl config show --section routing"
	return envelope.Error{
		Code:       "E_UNKNOWN_TASK",
		Message:    "no routing rule for task '" + task + "'",
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details:    map[string]any{"given": task, "configured_tasks": tasks, "nearest": nil},
	}
}

func noRouteAvailableError(task string, candidates []inferctl.RouteCandidate) *envelope.Error {
	considered := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		reason := "unavailable"
		if candidate.UnavailabilityReason != nil {
			reason = *candidate.UnavailabilityReason
		}
		considered = append(considered, map[string]any{"model": candidate.Model, "reason_unavailable": reason})
	}
	return &envelope.Error{
		Code:       "E_NO_ROUTE_AVAILABLE",
		Message:    "no candidate model for task '" + task + "' is reachable",
		DidYouMean: stringPtr("inferctl doctor"),
		ExitCode:   4,
		Retryable:  true,
		Details:    map[string]any{"task": task, "candidates_considered": considered},
	}
}

func writeRouteHuman(cmd *cobra.Command, report routeReport, explain, quiet bool) error {
	if quiet {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", report.Task, report.Decision.SelectedBackend, report.Decision.SelectedModel)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s/%s\n", report.Task, report.Decision.SelectedBackend, report.Decision.SelectedModel)
	fmt.Fprintf(cmd.OutOrStdout(), "reason: %s\n", report.Decision.Reason)
	if !explain {
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "candidates")
	for _, candidate := range report.Candidates {
		status := "available"
		if !candidate.Available && candidate.UnavailabilityReason != nil {
			status = *candidate.UnavailabilityReason
		}
		backend := ""
		if candidate.Backend != nil {
			backend = *candidate.Backend
		}
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\t%s\t%s\tloaded=%v\n", candidate.Model, backend, status, candidate.Loaded)
	}
	return nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
