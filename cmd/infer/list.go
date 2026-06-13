package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Ozhiaki/inferctl/internal/backends/llamacpp"
	"github.com/Ozhiaki/inferctl/internal/backends/lmstudio"
	"github.com/Ozhiaki/inferctl/internal/backends/mlx"
	"github.com/Ozhiaki/inferctl/internal/backends/ollama"
	"github.com/Ozhiaki/inferctl/internal/backends/openaicompat"
	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type backendEntry struct {
	name    string
	kind    string
	backend inferctl.Backend
}

func newBackendsCommand(jsonFlag *bool) *cobra.Command {
	var filter string
	var kind string
	var fast bool
	cmd := &cobra.Command{
		Use:   "backends",
		Short: "List configured backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			entries, errObj := configuredBackends(result, filter, kind)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			var statuses []inferctl.BackendStatus
			reachable := 0
			for _, entry := range entries {
				status := inferctl.BackendStatus{BackendInfo: entry.backend.Info()}
				info, err := entry.backend.Reachable(context.Background())
				if err == nil {
					status.BackendInfo = info
					status = normalizeBackendStatusForOutput(status)
					reachable++
					if !fast {
						if installed, err := entry.backend.ListInstalledModels(context.Background()); err == nil {
							n := len(installed)
							status.ModelsInstalledCount = &n
						}
						if loaded, err := entry.backend.ListLoadedModels(context.Background()); err == nil {
							n := len(loaded)
							status.ModelsLoadedCount = &n
						}
					}
				} else if errObj := backendReadError(entry, err); errObj != nil {
					return writeError(cmd, *jsonFlag, *errObj)
				}
				statuses = append(statuses, status)
			}
			data := map[string]any{"backends": statuses, "total_count": len(statuses), "reachable_count": reachable}
			return writeData(cmd, *jsonFlag, data, func() error {
				for _, status := range statuses {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\treachable=%v\n", status.Name, status.Kind, status.Reachable)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "backend name to show")
	cmd.Flags().StringVar(&kind, "kind", "", "backend kind to show")
	cmd.Flags().BoolVar(&fast, "fast", false, "skip model count probes")
	return cmd
}

func newModelsCommand(jsonFlag *bool) *cobra.Command {
	var backendName string
	var loadedOnly bool
	var installed bool
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List models across backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			entries, errObj := configuredBackends(result, backendName, "")
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			var models []inferctl.ModelInfo
			loadedCount := 0
			for _, entry := range entries {
				if loadedOnly {
					loadedModels, err := entry.backend.ListLoadedModels(context.Background())
					if err != nil {
						if errObj := backendReadError(entry, err); errObj != nil {
							return writeError(cmd, *jsonFlag, *errObj)
						}
						continue
					}
					loadedCount += len(loadedModels)
					for _, model := range loadedModels {
						models = append(models, inferctl.ModelInfo{Name: model.Name, Backend: model.Backend})
					}
					continue
				}
				if installed {
					installedModels, err := entry.backend.ListInstalledModels(context.Background())
					if err != nil {
						if errObj := backendReadError(entry, err); errObj != nil {
							return writeError(cmd, *jsonFlag, *errObj)
						}
						continue
					}
					models = append(models, installedModels...)
					loadedModels, err := entry.backend.ListLoadedModels(context.Background())
					if err == nil {
						loadedCount += len(loadedModels)
					}
				}
			}
			data := map[string]any{"models": models, "total_count": len(models), "loaded_count": loadedCount}
			return writeData(cmd, *jsonFlag, data, func() error {
				for _, model := range models {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", model.Backend, model.Name)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&backendName, "backend", "", "backend name to query")
	cmd.Flags().BoolVar(&loadedOnly, "loaded", false, "show only loaded models")
	cmd.Flags().BoolVar(&installed, "installed", true, "show installed models")
	return cmd
}

func newModelCommand(jsonFlag *bool) *cobra.Command {
	var noProbe bool
	cmd := &cobra.Command{
		Use:   "model <name>",
		Short: "Inspect one model",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return writeError(cmd, *jsonFlag, envelope.Error{
					Code:       "E_MISSING_ARG",
					Message:    "verb 'model' requires model_name",
					DidYouMean: stringPtr("infer model <model_name>"),
					ExitCode:   1,
					Retryable:  false,
					Details:    map[string]any{"verb": "model", "missing": "model_name"},
				})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			entries, errObj := configuredBackends(result, "", "")
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			if errObj := firstFatalBackendReadError(context.Background(), entries); errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			detail, found := inspectModel(context.Background(), result.Config, entries, name, noProbe)
			if !found {
				return writeError(cmd, *jsonFlag, envelope.Error{
					Code:       "E_UNKNOWN_MODEL",
					Message:    "model '" + name + "' not found on any reachable backend",
					DidYouMean: stringPtr("infer models"),
					ExitCode:   1,
					Retryable:  false,
					Details:    map[string]any{"given": name, "nearest": nil, "searched_backends": backendNames(entries)},
				})
			}
			return writeData(cmd, *jsonFlag, detail, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", detail.Name)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "skip live capability probing")
	return cmd
}

func configuredBackends(result *config.Result, filter, kind string) ([]backendEntry, *envelope.Error) {
	if len(result.Config.Backends) == 0 {
		return nil, &envelope.Error{
			Code:       "E_NO_BACKENDS_CONFIGURED",
			Message:    "config defines no backends",
			DidYouMean: stringPtr("infer config explain"),
			ExitCode:   3,
			Retryable:  false,
			Details:    map[string]any{"path": result.SourcePaths.Selected},
		}
	}
	var entries []backendEntry
	var configured []string
	for name := range result.Config.Backends {
		configured = append(configured, name)
	}
	slices.Sort(configured)
	for _, name := range configured {
		cfg := result.Config.Backends[name]
		if filter != "" && name != filter {
			continue
		}
		if kind != "" && cfg.Kind != kind {
			continue
		}
		if cfg.Kind == "openai_compat" && !cfg.RemoteAllowed && openaicompatRemoteURL(cfg.BaseURL) {
			return nil, backendConfigError(name, "E_BACKEND_REMOTE_NOT_ALLOWED", "backend '"+name+"' uses a remote openai_compat URL without remote_allowed=true")
		}
		entries = append(entries, backendEntry{name: name, kind: cfg.Kind, backend: instantiateBackend(name, cfg)})
	}
	if filter != "" && len(entries) == 0 {
		return nil, &envelope.Error{
			Code:       "E_UNKNOWN_BACKEND",
			Message:    "no backend named '" + filter + "' in config",
			DidYouMean: stringPtr("infer backends"),
			ExitCode:   1,
			Retryable:  false,
			Details:    map[string]any{"given": filter, "configured": configured, "nearest": nil},
		}
	}
	return entries, nil
}

func firstFatalBackendReadError(ctx context.Context, entries []backendEntry) *envelope.Error {
	for _, entry := range entries {
		if _, err := entry.backend.Reachable(ctx); err != nil {
			if errObj := backendReadError(entry, err); errObj != nil {
				return errObj
			}
		}
	}
	return nil
}

func backendReadError(entry backendEntry, err error) *envelope.Error {
	switch {
	case errors.Is(err, openaicompat.ErrAuthFailed):
		return backendConfigError(entry.name, "E_BACKEND_AUTH_FAILED", "backend '"+entry.name+"' authentication failed")
	case errors.Is(err, openaicompat.ErrRemoteNotAllowed):
		return backendConfigError(entry.name, "E_BACKEND_REMOTE_NOT_ALLOWED", "backend '"+entry.name+"' remote endpoint requires remote_allowed=true")
	default:
		return nil
	}
}

func backendConfigError(backend, code, message string) *envelope.Error {
	return &envelope.Error{
		Code:       code,
		Message:    message,
		DidYouMean: stringPtr("infer config show --key backends." + backend + " --json"),
		ExitCode:   3,
		Retryable:  false,
		Details:    map[string]any{"backend": backend},
	}
}

func openaicompatRemoteURL(raw string) bool {
	return openaicompat.RemoteURL(raw)
}

func instantiateBackend(name string, cfg config.BackendConfig) inferctl.Backend {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	switch cfg.Kind {
	case "llama.cpp":
		return llamacpp.New(name, cfg.BaseURL, cfg.Default, timeout)
	case "lmstudio":
		return lmstudio.New(name, cfg.BaseURL, cfg.Default, timeout)
	case "mlx":
		return mlx.New(name, cfg.BaseURL, cfg.Default, timeout)
	case "openai_compat":
		return openaicompat.New(name, cfg.BaseURL, cfg.Default, timeout, openaicompat.Options{
			AuthHeaderName:  cfg.AuthHeaderName,
			AuthHeaderValue: cfg.AuthHeaderValue,
			RemoteAllowed:   cfg.RemoteAllowed,
		})
	default:
		return ollama.New(name, cfg.BaseURL, cfg.Default, timeout)
	}
}

func inspectModel(ctx context.Context, cfg config.Config, entries []backendEntry, name string, noProbe bool) (inferctl.ModelDetail, bool) {
	loaded := map[string]bool{}
	var backends []inferctl.ModelBackend
	for _, entry := range entries {
		for _, loadedModel := range mustLoaded(ctx, entry.backend) {
			if loadedModel.Name == name {
				loaded[entry.name] = true
			}
		}
		for _, model := range mustInstalled(ctx, entry.backend) {
			if model.Name != name {
				continue
			}
			backends = append(backends, inferctl.ModelBackend{
				Backend:   entry.name,
				Installed: true,
				Loaded:    loaded[entry.name],
				SizeBytes: model.SizeBytes,
				Digest:    model.Digest,
			})
		}
	}
	if len(backends) == 0 {
		return inferctl.ModelDetail{}, false
	}
	capSource := "heuristic"
	if noProbe {
		capSource = "manifest"
	}
	detail := inferctl.ModelDetail{
		Name:         name,
		Backends:     backends,
		Capabilities: inferctl.Capabilities{Source: capSource},
		LatencyStats: inferctl.LatencyStats{Samples: 0},
		Routing:      routingForModel(cfg, name),
	}
	return detail, true
}

func mustInstalled(ctx context.Context, backend inferctl.Backend) []inferctl.ModelInfo {
	models, err := backend.ListInstalledModels(ctx)
	if err != nil {
		return nil
	}
	return models
}

func mustLoaded(ctx context.Context, backend inferctl.Backend) []inferctl.LoadedModelInfo {
	models, err := backend.ListLoadedModels(ctx)
	if err != nil {
		return nil
	}
	return models
}

func routingForModel(cfg config.Config, model string) inferctl.ModelRouting {
	var routing inferctl.ModelRouting
	for task, route := range cfg.Routing {
		if route.Model == model {
			routing.PrimaryForTasks = append(routing.PrimaryForTasks, task)
			routing.FallbackChain = append([]string{}, route.Fallback...)
		}
		if slices.Contains(route.Fallback, model) {
			routing.FallbackForTasks = append(routing.FallbackForTasks, task)
		}
	}
	slices.Sort(routing.PrimaryForTasks)
	slices.Sort(routing.FallbackForTasks)
	return routing
}

func backendNames(entries []backendEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.name)
	}
	slices.Sort(names)
	return names
}
