package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/inferctl/inferctl/internal/config"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/internal/render"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type snapshotOptions struct {
	task       string
	prompt     routePromptOptions
	outputPath string
	store      bool
	retention  int
}

func newSnapshotCommand(jsonFlag *bool) *cobra.Command {
	opts := snapshotOptions{}
	cmd := &cobra.Command{
		Use:   "snapshot --task <task>",
		Short: "Capture a comparable inferctl control-plane snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.task == "" {
				return writeError(cmd, *jsonFlag, invalidArg("--task", "", "configured task name", nil))
			}
			snapshot, warnings, commands, errObj := buildSnapshot(cmd.Context(), cmd, opts)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			if opts.outputPath != "" {
				if errObj := writeSnapshotArtifact(opts.outputPath, snapshot); errObj != nil {
					return writeError(cmd, *jsonFlag, *errObj)
				}
			}
			var stored *snapshotStoreResult
			if opts.store {
				result, errObj := storeSnapshot(snapshot, opts.retention, envMap())
				if errObj != nil {
					return writeError(cmd, *jsonFlag, *errObj)
				}
				stored = &result
			}
			mode := render.SelectMode(render.Options{JSONFlag: *jsonFlag, Env: envMap()})
			if mode == render.ModeJSON {
				return writeDataWithDiagnostics(cmd, true, snapshot, warnings, commands, func() error { return nil })
			}
			if stored != nil && opts.outputPath == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "snapshot stored: %s\n", stored.Path)
				return nil
			}
			if opts.outputPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "snapshot written: %s\n", opts.outputPath)
				if stored != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "snapshot stored: %s\n", stored.Path)
				}
				return nil
			}
			return render.WriteJSON(cmd.OutOrStdout(), snapshot)
		},
	}
	cmd.Flags().StringVar(&opts.task, "task", "", "configured task to snapshot")
	cmd.Flags().StringVar(&opts.prompt.file, "prompt-file", "", "read prompt from file")
	cmd.Flags().StringVar(&opts.prompt.inline, "prompt", "", "inline prompt text")
	cmd.Flags().BoolVar(&opts.prompt.fromStdin, "from-stdin", false, "read prompt from stdin")
	cmd.Flags().StringVar(&opts.outputPath, "output", "", "write raw snapshot artifact to this path")
	cmd.Flags().BoolVar(&opts.store, "store", false, "write raw snapshot artifact to the configured snapshot store")
	cmd.Flags().IntVar(&opts.retention, "retention-limit", defaultSnapshotRetentionLimit, "stored snapshots to retain per task when --store is used")
	return cmd
}

func buildSnapshot(ctx context.Context, cmd *cobra.Command, opts snapshotOptions) (controlPlaneSnapshot, []envelope.Warning, []envelope.Command, *envelope.Error) {
	meta, errObj := readPromptMetadata(cmd, promptReadOptions{
		inline:      opts.prompt.inline,
		file:        opts.prompt.file,
		fromStdin:   opts.prompt.fromStdin,
		includeHash: true,
	})
	if errObj != nil {
		return controlPlaneSnapshot{}, nil, nil, errObj
	}
	result, err := (config.Loader{}).Load(config.LoadOptions{})
	if err != nil {
		errObj := configLoadError(err)
		return controlPlaneSnapshot{}, nil, nil, &errObj
	}
	if len(result.Config.Backends) == 0 {
		errObj := noBackendsError(result)
		return controlPlaneSnapshot{}, nil, nil, &errObj
	}
	if validation := config.Validate(result, false); validation.Summary.Errors > 0 {
		errObj := validationFailedError(validation)
		errObj.ExitCode = exitEnvironment
		return controlPlaneSnapshot{}, nil, nil, &errObj
	}
	routeCfg, ok := result.Config.Routing[opts.task]
	if !ok {
		errObj := unknownTaskError(opts.task, result.Config)
		return controlPlaneSnapshot{}, nil, nil, &errObj
	}
	entries, backendsErr := configuredBackends(result, "", "")
	if backendsErr != nil {
		return controlPlaneSnapshot{}, nil, nil, backendsErr
	}
	if errObj := firstFatalBackendReadError(ctx, entries); errObj != nil {
		return controlPlaneSnapshot{}, nil, nil, errObj
	}
	state, stateWarnings := collectSnapshotState(ctx, entries)
	routeInput := routeInput{PromptChars: meta.PromptChars, EstimatedTokens: meta.EstimatedTokens, Source: meta.Source}
	route, routeWarnings, routeCommands, noRoute := buildRouteReport(ctx, result.Config, opts.task, routeCfg, entries, routeInput)
	if noRoute != nil {
		return controlPlaneSnapshot{}, append(stateWarnings, routeWarnings...), routeCommands, noRoute
	}
	warnings := append(stateWarnings, routeWarnings...)
	commands := routeCommands
	snapshot := newControlPlaneSnapshot(opts.task, meta)
	snapshot.RouteDecision = route.Decision
	snapshot.RouteCandidates = nonNilRouteCandidates(route.Candidates)
	snapshot.BackendReachability = state.BackendReachability
	snapshot.LoadedModels = state.LoadedModels
	snapshot.InstalledModels = state.InstalledModels
	snapshot.Warnings = nonNilSnapshotWarnings(warnings)
	snapshot.Errors = []envelope.Error{}
	snapshot.RecommendedAction = recommendedAction(commands)
	return snapshot, warnings, commands, nil
}

type snapshotState struct {
	BackendReachability []backendReachability
	InstalledModels     []inferctl.ModelInfo
	LoadedModels        []inferctl.LoadedModelInfo
}

func collectSnapshotState(ctx context.Context, entries []backendEntry) (snapshotState, []envelope.Warning) {
	var state snapshotState
	var warnings []envelope.Warning
	for _, entry := range entries {
		info, err := entry.backend.Reachable(ctx)
		if err != nil {
			msg := err.Error()
			base := entry.backend.Info()
			state.BackendReachability = append(state.BackendReachability, backendReachability{
				Name:      entry.name,
				Kind:      base.Kind,
				BaseURL:   base.BaseURL,
				Reachable: false,
				Error:     &msg,
			})
			warnings = append(warnings, backendWarning("W_BACKEND_UNREACHABLE", entry.name, "backend '"+entry.name+"' is unreachable", err))
			continue
		}
		info = normalizeBackendInfoForOutput(info)
		state.BackendReachability = append(state.BackendReachability, backendReachability{
			Name:      entry.name,
			Kind:      info.Kind,
			BaseURL:   info.BaseURL,
			Reachable: true,
		})
		installed, err := entry.backend.ListInstalledModels(ctx)
		if err != nil {
			warnings = append(warnings, backendWarning("W_BACKEND_DEGRADED", entry.name, "backend '"+entry.name+"' model inventory probe failed", err))
		}
		loaded, err := entry.backend.ListLoadedModels(ctx)
		if err != nil {
			warnings = append(warnings, backendWarning("W_BACKEND_DEGRADED", entry.name, "backend '"+entry.name+"' loaded-model probe failed", err))
		}
		state.InstalledModels = append(state.InstalledModels, installed...)
		state.LoadedModels = append(state.LoadedModels, loaded...)
	}
	slices.SortFunc(state.BackendReachability, func(a, b backendReachability) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(state.InstalledModels, func(a, b inferctl.ModelInfo) int {
		if a.Backend == b.Backend {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.Backend, b.Backend)
	})
	slices.SortFunc(state.LoadedModels, func(a, b inferctl.LoadedModelInfo) int {
		if a.Backend == b.Backend {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.Backend, b.Backend)
	})
	if state.BackendReachability == nil {
		state.BackendReachability = []backendReachability{}
	}
	if state.InstalledModels == nil {
		state.InstalledModels = []inferctl.ModelInfo{}
	}
	if state.LoadedModels == nil {
		state.LoadedModels = []inferctl.LoadedModelInfo{}
	}
	return state, warnings
}

func writeSnapshotArtifact(path string, snapshot controlPlaneSnapshot) *envelope.Error {
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		errObj := envelope.Error{
			Code:      "E_BINARY_INTERNAL",
			Message:   "could not encode snapshot: " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"reason": err.Error()},
		}
		return &errObj
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_WRITE_FAILED",
			Message:   "could not write snapshot to " + path + ": " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"path": path, "reason": err.Error()},
		}
		return &errObj
	}
	return nil
}

func nonNilRouteCandidates(candidates []inferctl.RouteCandidate) []inferctl.RouteCandidate {
	if candidates == nil {
		return []inferctl.RouteCandidate{}
	}
	return candidates
}

func nonNilSnapshotWarnings(warnings []envelope.Warning) []envelope.Warning {
	if warnings == nil {
		return []envelope.Warning{}
	}
	return warnings
}

func deterministicSnapshotTime() time.Time {
	if deterministicOutputMode() {
		return time.Unix(0, 0).UTC()
	}
	return time.Now().UTC()
}

func timeNow() time.Time {
	return time.Now().UTC()
}
