package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/inferctl/inferctl/internal/config"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/internal/render"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type preflightReport struct {
	Task              string                      `json:"task"`
	Prompt            promptMetadata              `json:"prompt"`
	RouteDecision     inferctl.RouteDecision      `json:"route_decision"`
	RouteCandidates   []inferctl.RouteCandidate   `json:"route_candidates"`
	Constraints       inferctl.RouteConstraints   `json:"constraints"`
	Runnability       preflightRunnability        `json:"runnability"`
	Policy            preflightPolicy             `json:"policy"`
	Warnings          []envelope.Warning          `json:"warnings"`
	RecommendedAction *inferctl.RecommendedAction `json:"recommended_action"`
}

type preflightRunnability struct {
	Status   string `json:"status"`
	Runnable bool   `json:"runnable"`
	ExitCode int    `json:"exit_code"`
	Reason   string `json:"reason"`
}

type preflightPolicy struct {
	AllowFallback bool `json:"allow_fallback"`
	RequireReady  bool `json:"require_ready"`
}

type preflightOptions struct {
	prompt        routePromptOptions
	allowFallback bool
	requireReady  bool
	format        string
}

func newPreflightCommand(jsonFlag *bool) *cobra.Command {
	opts := preflightOptions{format: "human"}
	cmd := &cobra.Command{
		Use:   "preflight <task>",
		Short: "Decide whether automation may attempt a configured task",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return writeError(cmd, *jsonFlag, envelope.Error{
					Code:       "E_MISSING_ARG",
					Message:    "verb 'preflight' requires task",
					DidYouMean: stringPtr("inferctl preflight <task>"),
					ExitCode:   exitUserInput,
					Retryable:  false,
					Details:    map[string]any{"verb": "preflight", "missing": "task"},
				})
			}
			if len(args) > 2 || (len(args) == 2 && args[1] != "-") {
				return writeError(cmd, *jsonFlag, invalidArg("args", strings.Join(args, " "), "one task and optional '-' stdin marker", nil))
			}
			if len(args) == 2 && args[1] == "-" {
				opts.prompt.fromStdin = true
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains([]string{"human", "markdown"}, opts.format) {
				return writeError(cmd, *jsonFlag, invalidArg("--format", opts.format, "one of human, markdown", []string{"human", "markdown"}))
			}
			report, warnings, commands, errObj := runPreflight(cmd.Context(), cmd, args[0], opts)
			if errObj != nil {
				return writePreflightResult(cmd, *jsonFlag, opts.format, report, warnings, commands, *errObj)
			}
			return writeDataWithDiagnostics(cmd, *jsonFlag, report, warnings, commands, func() error {
				return writePreflightHuman(cmd, report, opts.format)
			})
		},
	}
	cmd.Flags().StringVar(&opts.prompt.file, "prompt-file", "", "read prompt from file")
	cmd.Flags().StringVar(&opts.prompt.inline, "prompt", "", "inline prompt text")
	cmd.Flags().BoolVar(&opts.prompt.fromStdin, "from-stdin", false, "read prompt from stdin")
	cmd.Flags().BoolVar(&opts.allowFallback, "allow-fallback", false, "allow automation to proceed when the selected route is a fallback")
	cmd.Flags().BoolVar(&opts.requireReady, "require-ready", false, "require the selected model to already be loaded")
	cmd.Flags().StringVar(&opts.format, "format", "human", "human output format: human or markdown")
	return cmd
}

func runPreflight(ctx context.Context, cmd *cobra.Command, task string, opts preflightOptions) (preflightReport, []envelope.Warning, []envelope.Command, *envelope.Error) {
	meta, errObj := readPromptMetadata(cmd, promptReadOptions{
		inline:      opts.prompt.inline,
		file:        opts.prompt.file,
		fromStdin:   opts.prompt.fromStdin,
		includeHash: true,
	})
	if errObj != nil {
		return preflightReport{}, nil, nil, errObj
	}
	result, err := (config.Loader{}).Load(config.LoadOptions{})
	if err != nil {
		errObj := configLoadError(err)
		return preflightReport{}, nil, nil, &errObj
	}
	if len(result.Config.Backends) == 0 {
		errObj := noBackendsError(result)
		return preflightReport{}, nil, nil, &errObj
	}
	if validation := config.Validate(result, false); validation.Summary.Errors > 0 {
		errObj := validationFailedError(validation)
		errObj.ExitCode = exitEnvironment
		return preflightReport{}, nil, nil, &errObj
	}
	routeCfg, ok := result.Config.Routing[task]
	if !ok {
		errObj := unknownTaskError(task, result.Config)
		return preflightReport{}, nil, nil, &errObj
	}
	entries, backendsErr := configuredBackends(result, "", "")
	if backendsErr != nil {
		return preflightReport{}, nil, nil, backendsErr
	}
	if errObj := firstFatalBackendReadError(ctx, entries); errObj != nil {
		return preflightReport{}, nil, nil, errObj
	}
	routeInput := routeInput{PromptChars: meta.PromptChars, EstimatedTokens: meta.EstimatedTokens, Source: meta.Source}
	route, warnings, commands, noRoute := buildRouteReport(ctx, result.Config, task, routeCfg, entries, routeInput)
	if noRoute != nil {
		return preflightReport{}, warnings, commands, noRoute
	}
	report := preflightReport{
		Task:            task,
		Prompt:          meta,
		RouteDecision:   route.Decision,
		RouteCandidates: route.Candidates,
		Constraints:     route.Constraints,
		Policy: preflightPolicy{
			AllowFallback: opts.allowFallback,
			RequireReady:  opts.requireReady,
		},
		Warnings:          nonNilPreflightWarnings(warnings),
		RecommendedAction: recommendedAction(commands),
	}
	report.Runnability = preflightRunnability{Status: runnabilityRunnable, Runnable: true, ExitCode: exitSuccess, Reason: "route satisfies preflight policy"}
	if route.Decision.IsFallback && !opts.allowFallback {
		errObj := preflightPolicyBlockedError(task, "fallback selected but --allow-fallback was not set")
		report.Runnability = preflightRunnability{Status: runnabilityPolicyBlock, Runnable: false, ExitCode: errObj.ExitCode, Reason: errObj.Message}
		return report, warnings, commands, &errObj
	}
	if !route.Decision.Ready && opts.requireReady {
		errObj := preflightPolicyBlockedError(task, "selected model is not loaded and --require-ready was set")
		report.Runnability = preflightRunnability{Status: runnabilityPolicyBlock, Runnable: false, ExitCode: errObj.ExitCode, Reason: errObj.Message}
		return report, warnings, commands, &errObj
	}
	return report, warnings, commands, nil
}

func nonNilPreflightWarnings(warnings []envelope.Warning) []envelope.Warning {
	if warnings == nil {
		return []envelope.Warning{}
	}
	return warnings
}

func preflightPolicyBlockedError(task, reason string) envelope.Error {
	return envelope.Error{
		Code:       "E_PREFLIGHT_POLICY_BLOCKED",
		Message:    reason,
		DidYouMean: stringPtr("inferctl preflight " + task + " --allow-fallback"),
		ExitCode:   exitUserInput,
		Retryable:  false,
		Details:    map[string]any{"task": task, "reason": reason},
	}
}

func writePreflightResult(cmd *cobra.Command, jsonFlag bool, format string, report preflightReport, warnings []envelope.Warning, commands []envelope.Command, errObj envelope.Error) error {
	mode := render.SelectMode(render.Options{JSONFlag: jsonFlag, Env: envMap()})
	if mode == render.ModeJSON {
		start := time.Now()
		env, err := envelope.New(resolvedToolVersion(), report, envelope.Options{
			StartedAt:  start,
			FinishedAt: time.Now(),
			Env:        envMap(),
			Warnings:   warnings,
			Commands:   commands,
			Errors:     []envelope.Error{errObj},
		})
		if err != nil {
			return err
		}
		if err := render.WriteJSON(cmd.OutOrStdout(), env); err != nil {
			return err
		}
	} else if report.Task != "" {
		if err := writePreflightHuman(cmd, report, format); err != nil {
			return err
		}
	}
	writeErrorDiagnostic(cmd, errObj)
	return exitError(errObj.ExitCode)
}

func writePreflightHuman(cmd *cobra.Command, report preflightReport, format string) error {
	if format == "markdown" {
		fmt.Fprintf(cmd.OutOrStdout(), "## inferctl preflight: %s\n\n", report.Task)
		fmt.Fprintf(cmd.OutOrStdout(), "- Runnability: `%s`\n", report.Runnability.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "- Selected route: `%s/%s`\n", report.RouteDecision.SelectedBackend, report.RouteDecision.SelectedModel)
		fmt.Fprintf(cmd.OutOrStdout(), "- Ready: `%v`\n", report.RouteDecision.Ready)
		fmt.Fprintf(cmd.OutOrStdout(), "- Fallback: `%v`\n", report.RouteDecision.IsFallback)
		if len(report.Warnings) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "- Warnings:")
			for _, warning := range report.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "  - `%s`: %s\n", warning.Code, warning.Message)
			}
		}
		if report.RecommendedAction != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "- Recommended action: `%s`\n", report.RecommendedAction.Command)
		}
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "preflight: %s %s\n", report.Task, report.Runnability.Status)
	fmt.Fprintf(cmd.OutOrStdout(), "route: %s/%s ready=%v fallback=%v\n", report.RouteDecision.SelectedBackend, report.RouteDecision.SelectedModel, report.RouteDecision.Ready, report.RouteDecision.IsFallback)
	if report.RecommendedAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", report.RecommendedAction.Command)
	}
	return nil
}
