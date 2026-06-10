package main

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type doctorReport struct {
	Summary           doctorSummary               `json:"summary"`
	Backends          []inferctl.BackendStatus    `json:"backends"`
	LoadedModels      []inferctl.LoadedModelInfo  `json:"loaded_models"`
	Routes            []doctorRouteSummary        `json:"routes"`
	System            doctorSystem                `json:"system"`
	Warnings          []envelope.Warning          `json:"warnings"`
	RecommendedAction *inferctl.RecommendedAction `json:"recommended_action"`
}

type doctorSummary struct {
	BackendsTotal        int `json:"backends_total"`
	BackendsReachable    int `json:"backends_reachable"`
	ModelsInstalledTotal int `json:"models_installed_total"`
	ModelsLoadedTotal    int `json:"models_loaded_total"`
	WarningsTotal        int `json:"warnings_total"`
	ErrorsTotal          int `json:"errors_total"`
}

type doctorRouteSummary struct {
	Task              string            `json:"task"`
	ConfiguredPrimary doctorRouteModel  `json:"configured_primary"`
	Selected          *doctorRouteModel `json:"selected"`
	IsFallback        bool              `json:"is_fallback"`
	Ready             bool              `json:"ready"`
	CandidatesCount   int               `json:"candidates_count"`
	Reason            string            `json:"reason"`
}

type doctorRouteModel struct {
	Model   string `json:"model"`
	Backend string `json:"backend"`
}

type doctorSystem struct {
	Profile             string   `json:"profile"`
	ProfileMode         string   `json:"profile_mode"`
	VRAMTotalBytes      *int64   `json:"vram_total_bytes"`
	VRAMUsedBytes       *int64   `json:"vram_used_bytes"`
	VRAMUsedPct         *float64 `json:"vram_used_pct"`
	VRAMSource          *string  `json:"vram_source"`
	MaxConcurrentModels int      `json:"max_concurrent_models"`
	AllowPremium        bool     `json:"allow_premium"`
	GOOS                string   `json:"goos"`
	GOARCH              string   `json:"goarch"`
}

type doctorProbe struct {
	status         inferctl.BackendStatus
	installed      []inferctl.ModelInfo
	loaded         []inferctl.LoadedModelInfo
	reachableError error
	installedError error
	loadedError    error
}

func newDoctorCommand(jsonFlag *bool) *cobra.Command {
	var fast bool
	var backendName string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose configured local inference backends",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
			entries, errObj := configuredBackends(result, backendName, "")
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			report, warnings, commands := buildDoctorReport(context.Background(), result.Config, entries, fast)
			report.Warnings = warnings
			report.Summary.WarningsTotal = len(warnings)
			report.RecommendedAction = recommendedAction(commands)
			return writeDataWithDiagnostics(cmd, *jsonFlag, report, warnings, commands, func() error {
				return writeDoctorHuman(cmd, report, commands)
			})
		},
	}
	cmd.Flags().BoolVar(&fast, "fast", false, "skip slower probes")
	cmd.Flags().StringVar(&backendName, "backend", "", "backend name to diagnose")
	return cmd
}

func buildDoctorReport(ctx context.Context, cfg config.Config, entries []backendEntry, fast bool) (doctorReport, []envelope.Warning, []envelope.Command) {
	var probes []doctorProbe
	var warnings []envelope.Warning
	installedByModel := map[string][]inferctl.ModelInfo{}
	loadedByBackendModel := map[string]bool{}
	for _, entry := range entries {
		probe := doctorProbe{status: inferctl.BackendStatus{BackendInfo: entry.backend.Info()}}
		info, err := entry.backend.Reachable(ctx)
		if err != nil {
			probe.reachableError = err
			warnings = append(warnings, backendWarning("W_BACKEND_UNREACHABLE", entry.name, "backend '"+entry.name+"' is unreachable", err))
			probes = append(probes, probe)
			continue
		}
		probe.status.BackendInfo = info
		if !fast {
			probe.installed, probe.installedError = entry.backend.ListInstalledModels(ctx)
			if probe.installedError != nil {
				warnings = append(warnings, backendWarning("W_BACKEND_DEGRADED", entry.name, "backend '"+entry.name+"' model inventory probe failed", probe.installedError))
			}
			probe.loaded, probe.loadedError = entry.backend.ListLoadedModels(ctx)
			if probe.loadedError != nil {
				warnings = append(warnings, backendWarning("W_BACKEND_DEGRADED", entry.name, "backend '"+entry.name+"' loaded-model probe failed", probe.loadedError))
			}
		}
		installedCount := len(probe.installed)
		loadedCount := len(probe.loaded)
		probe.status.ModelsInstalledCount = &installedCount
		probe.status.ModelsLoadedCount = &loadedCount
		for _, model := range probe.installed {
			installedByModel[model.Name] = append(installedByModel[model.Name], model)
		}
		for _, model := range probe.loaded {
			loadedByBackendModel[entry.name+"\x00"+model.Name] = true
		}
		probes = append(probes, probe)
	}
	statuses, loadedModels, summary := summarizeDoctorProbes(probes)
	routes, routeWarnings := summarizeDoctorRoutes(cfg, entries, installedByModel, loadedByBackendModel)
	warnings = append(warnings, routeWarnings...)
	report := doctorReport{
		Summary:      summary,
		Backends:     statuses,
		LoadedModels: loadedModels,
		Routes:       routes,
		System:       doctorSystemForConfig(cfg),
	}
	report.Summary.WarningsTotal = len(warnings)
	commands := doctorCommands(report)
	return report, warnings, commands
}

func summarizeDoctorProbes(probes []doctorProbe) ([]inferctl.BackendStatus, []inferctl.LoadedModelInfo, doctorSummary) {
	var statuses []inferctl.BackendStatus
	var loaded []inferctl.LoadedModelInfo
	var summary doctorSummary
	summary.BackendsTotal = len(probes)
	for _, probe := range probes {
		statuses = append(statuses, probe.status)
		if probe.status.Reachable {
			summary.BackendsReachable++
		}
		if probe.status.ModelsInstalledCount != nil {
			summary.ModelsInstalledTotal += *probe.status.ModelsInstalledCount
		}
		if probe.status.ModelsLoadedCount != nil {
			summary.ModelsLoadedTotal += *probe.status.ModelsLoadedCount
		}
		loaded = append(loaded, probe.loaded...)
	}
	slices.SortFunc(statuses, func(a, b inferctl.BackendStatus) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(loaded, func(a, b inferctl.LoadedModelInfo) int {
		if a.Backend == b.Backend {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.Backend, b.Backend)
	})
	return statuses, loaded, summary
}

func summarizeDoctorRoutes(cfg config.Config, entries []backendEntry, installedByModel map[string][]inferctl.ModelInfo, loadedByBackendModel map[string]bool) ([]doctorRouteSummary, []envelope.Warning) {
	tasks := make([]string, 0, len(cfg.Routing))
	for task := range cfg.Routing {
		tasks = append(tasks, task)
	}
	slices.Sort(tasks)
	var routes []doctorRouteSummary
	var warnings []envelope.Warning
	for _, task := range tasks {
		route := cfg.Routing[task]
		candidates := append([]string{route.Model}, route.Fallback...)
		primary := doctorRouteModel{Model: route.Model, Backend: route.Backend}
		summary := doctorRouteSummary{
			Task:              task,
			ConfiguredPrimary: primary,
			CandidatesCount:   len(candidates),
			Reason:            "no configured candidate is installed on a reachable backend",
		}
		for i, model := range candidates {
			placement, ok := firstPlacement(model, route.Backend, i == 0, entries, installedByModel)
			if !ok {
				if i == 0 {
					warnings = append(warnings, envelope.Warning{
						Code:    "W_MODEL_NOT_INSTALLED",
						Message: "primary model '" + model + "' is not installed on a reachable backend",
						Details: map[string]any{"task": task, "model": model, "backend": route.Backend},
					})
				}
				continue
			}
			ready := loadedByBackendModel[placement.Backend+"\x00"+placement.Model]
			summary.Selected = &placement
			summary.Ready = ready
			summary.IsFallback = i > 0
			if i == 0 {
				summary.Reason = "primary model is available"
			} else {
				summary.Reason = "primary unavailable; selected fallback"
				warnings = append(warnings, envelope.Warning{
					Code:    "W_FALLBACK_USED",
					Message: "routed to fallback '" + model + "' because primary '" + route.Model + "' is unavailable",
					Details: map[string]any{"task": task, "primary": route.Model, "selected": model},
				})
			}
			break
		}
		routes = append(routes, summary)
	}
	return routes, warnings
}

func firstPlacement(model, primaryBackend string, primary bool, entries []backendEntry, installedByModel map[string][]inferctl.ModelInfo) (doctorRouteModel, bool) {
	installed := installedByModel[model]
	for _, entry := range entries {
		if primary && primaryBackend != "" && entry.name != primaryBackend {
			continue
		}
		for _, candidate := range installed {
			if candidate.Backend == entry.name {
				return doctorRouteModel{Model: model, Backend: entry.name}, true
			}
		}
	}
	return doctorRouteModel{}, false
}

func doctorSystemForConfig(cfg config.Config) doctorSystem {
	system := doctorSystem{
		Profile:             cfg.Profile.Name,
		ProfileMode:         cfg.Profile.Mode,
		MaxConcurrentModels: cfg.Profile.MaxConcurrentModels,
		AllowPremium:        cfg.Profile.AllowPremium,
		GOOS:                runtime.GOOS,
		GOARCH:              runtime.GOARCH,
	}
	if cfg.Profile.VRAMTotalBytesHint != nil {
		source := "config_hint"
		system.VRAMTotalBytes = cfg.Profile.VRAMTotalBytesHint
		system.VRAMSource = &source
	}
	return system
}

func doctorCommands(report doctorReport) []envelope.Command {
	var commands []envelope.Command
	for _, backend := range report.Backends {
		if !backend.Reachable {
			commands = append(commands, envelope.Command{
				Label:     "Inspect backend '" + backend.Name + "'",
				Command:   "infer backends --filter " + backend.Name + " --json",
				Rationale: "Surfaces backend reachability and model counts",
			})
		}
	}
	for _, route := range report.Routes {
		if route.IsFallback {
			commands = append(commands, envelope.Command{
				Label:     "Full route evaluation for '" + route.Task + "'",
				Command:   "infer route " + route.Task + " --json",
				Rationale: "Shows all candidates considered and constraint checks",
			})
		}
	}
	for _, route := range report.Routes {
		if !route.Ready && route.Selected != nil {
			commands = append(commands, envelope.Command{
				Label:              "Warm model '" + route.Selected.Model + "'",
				Command:            "infer warmup " + route.Selected.Model,
				Rationale:          "Loads the selected model before inference",
				AvailableInVersion: stringPtr("0.5"),
			})
		}
	}
	if report.System.VRAMUsedPct != nil && *report.System.VRAMUsedPct > 80 {
		commands = append(commands, envelope.Command{
			Label:              "Release idle models",
			Command:            "infer release-idle",
			Rationale:          "Frees VRAM used by idle models",
			AvailableInVersion: stringPtr("0.5"),
		})
	}
	if len(commands) > 6 {
		commands = commands[:6]
	}
	return commands
}

func recommendedAction(commands []envelope.Command) *inferctl.RecommendedAction {
	if len(commands) == 0 {
		return nil
	}
	action := inferctl.RecommendedAction{
		Command:   commands[0].Command,
		Rationale: commands[0].Rationale,
	}
	for _, cmd := range commands[1:] {
		action.Alternatives = append(action.Alternatives, inferctl.RecommendedOption{
			Command:   cmd.Command,
			Rationale: cmd.Rationale,
		})
	}
	return &action
}

func backendWarning(code, backend, message string, err error) envelope.Warning {
	details := map[string]any{"backend": backend}
	if err != nil {
		details["error"] = err.Error()
	}
	return envelope.Warning{Code: code, Message: message, Details: details}
}

func noBackendsError(result *config.Result) envelope.Error {
	return envelope.Error{
		Code:       "E_NO_BACKENDS_CONFIGURED",
		Message:    "config defines no backends",
		DidYouMean: stringPtr("infer config explain"),
		ExitCode:   3,
		Retryable:  false,
		Details:    map[string]any{"path": result.SourcePaths.Selected},
	}
}

func writeDoctorHuman(cmd *cobra.Command, report doctorReport, commands []envelope.Command) error {
	fmt.Fprintf(cmd.OutOrStdout(), "doctor: %d/%d backends reachable, %d loaded models\n",
		report.Summary.BackendsReachable, report.Summary.BackendsTotal, report.Summary.ModelsLoadedTotal)
	if report.RecommendedAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", report.RecommendedAction.Command)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nbackends")
	for _, backend := range report.Backends {
		fmt.Fprintf(cmd.OutOrStdout(), "- %s\t%s\treachable=%v\n", backend.Name, backend.Kind, backend.Reachable)
	}
	if len(report.Routes) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nroutes")
		for _, route := range report.Routes {
			selected := "<none>"
			if route.Selected != nil {
				selected = route.Selected.Backend + "/" + route.Selected.Model
			}
			fmt.Fprintf(cmd.OutOrStdout(), "- %s\t%s\t%s\n", route.Task, selected, route.Reason)
		}
	}
	if len(commands) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\ncommands")
		for _, command := range commands {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", command.Command)
		}
	}
	return nil
}
