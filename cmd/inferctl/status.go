package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/inferctl/inferctl/internal/config"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

const statusSchemaVersion = "0.1"

type statusSnapshot struct {
	StatusSchemaVersion string                      `json:"status_schema_version"`
	ContractVersion     string                      `json:"contract_version"`
	CapturedAtISO       string                      `json:"captured_at_iso"`
	Summary             statusSummary               `json:"summary"`
	Backends            []statusBackend             `json:"backends"`
	Models              statusModels                `json:"models"`
	Routes              []statusRoute               `json:"routes"`
	Warnings            []envelope.Warning          `json:"warnings"`
	RecommendedAction   *inferctl.RecommendedAction `json:"recommended_action"`
}

type statusSummary struct {
	BackendsTotal      int `json:"backends_total"`
	BackendsReachable  int `json:"backends_reachable"`
	ModelsExposedTotal int `json:"models_exposed_total"`
	ModelsLoadedTotal  int `json:"models_loaded_total"`
	RoutesTotal        int `json:"routes_total"`
	RoutesReady        int `json:"routes_ready"`
	WarningsTotal      int `json:"warnings_total"`
}

type statusBackend struct {
	Name                 string  `json:"name"`
	Kind                 string  `json:"kind"`
	BaseURL              string  `json:"base_url"`
	Reachable            bool    `json:"reachable"`
	Default              bool    `json:"default"`
	ModelsInstalledCount *int    `json:"models_installed_count"`
	ModelsLoadedCount    *int    `json:"models_loaded_count"`
	Error                *string `json:"error"`
}

type statusModels struct {
	Exposed []modelListRow             `json:"exposed"`
	Loaded  []inferctl.LoadedModelInfo `json:"loaded"`
}

type statusRoute struct {
	Task       string                    `json:"task"`
	Decision   inferctl.RouteDecision    `json:"decision"`
	Candidates []inferctl.RouteCandidate `json:"candidates"`
	Warnings   []envelope.Warning        `json:"warnings"`
}

func newStatusCommand(jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Emit an aggregate live-state status snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot, warnings, commands, errObj := buildStatusSnapshot(cmd.Context())
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			return writeDataWithDiagnostics(cmd, *jsonFlag, snapshot, warnings, commands, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "status: %d/%d backends reachable, %d loaded models, %d route(s)\n",
					snapshot.Summary.BackendsReachable,
					snapshot.Summary.BackendsTotal,
					snapshot.Summary.ModelsLoadedTotal,
					snapshot.Summary.RoutesTotal,
				)
				return nil
			})
		},
	}
	return cmd
}

func buildStatusSnapshot(ctx context.Context) (statusSnapshot, []envelope.Warning, []envelope.Command, *envelope.Error) {
	result, err := (config.Loader{}).Load(config.LoadOptions{})
	if err != nil {
		errObj := configLoadError(err)
		return statusSnapshot{}, nil, nil, &errObj
	}
	if len(result.Config.Backends) == 0 {
		errObj := noBackendsError(result)
		return statusSnapshot{}, nil, nil, &errObj
	}
	if validation := config.Validate(result, false); validation.Summary.Errors > 0 {
		errObj := validationFailedError(validation)
		errObj.ExitCode = exitEnvironment
		return statusSnapshot{}, nil, nil, &errObj
	}
	entries, errObj := configuredBackends(result, "", "")
	if errObj != nil {
		return statusSnapshot{}, nil, nil, errObj
	}
	if errObj := firstFatalBackendReadError(ctx, entries); errObj != nil {
		return statusSnapshot{}, nil, nil, errObj
	}

	doctor, warnings, commands := buildDoctorReport(ctx, result.Config, entries, false)
	doctor.Warnings = warnings
	doctor.RecommendedAction = recommendedAction(commands)

	routes, routeWarnings, routeCommands, routeErr := buildStatusRoutes(ctx, result.Config, entries)
	if routeErr != nil {
		return statusSnapshot{}, append(warnings, routeWarnings...), append(commands, routeCommands...), routeErr
	}
	warnings = append(warnings, routeWarnings...)
	warnings = dedupeWarnings(warnings)
	commands = append(commands, routeCommands...)
	commands = dedupeCommands(commands)

	exposed := statusExposedModels(ctx, entries)
	loaded := nonNilLoadedModels(doctor.LoadedModels)
	status := statusSnapshot{
		StatusSchemaVersion: statusSchemaVersion,
		ContractVersion:     "0.1",
		CapturedAtISO:       deterministicSnapshotTime().Format("2006-01-02T15:04:05Z"),
		Backends:            statusBackendsFromDoctor(doctor.Backends),
		Models: statusModels{
			Exposed: exposed,
			Loaded:  loaded,
		},
		Routes:            routes,
		Warnings:          nonNilSnapshotWarnings(warnings),
		RecommendedAction: recommendedAction(commands),
	}
	status.Summary = statusSummary{
		BackendsTotal:      len(status.Backends),
		BackendsReachable:  countReachableStatusBackends(status.Backends),
		ModelsExposedTotal: len(status.Models.Exposed),
		ModelsLoadedTotal:  len(status.Models.Loaded),
		RoutesTotal:        len(status.Routes),
		RoutesReady:        countReadyStatusRoutes(status.Routes),
		WarningsTotal:      len(status.Warnings),
	}
	return status, warnings, commands, nil
}

func dedupeWarnings(warnings []envelope.Warning) []envelope.Warning {
	seen := map[string]bool{}
	out := make([]envelope.Warning, 0, len(warnings))
	for _, warning := range warnings {
		key := warning.Code + "\x00" + warning.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, warning)
	}
	return out
}

func buildStatusRoutes(ctx context.Context, cfg config.Config, entries []backendEntry) ([]statusRoute, []envelope.Warning, []envelope.Command, *envelope.Error) {
	tasks := make([]string, 0, len(cfg.Routing))
	for task := range cfg.Routing {
		tasks = append(tasks, task)
	}
	slices.Sort(tasks)
	routes := make([]statusRoute, 0, len(tasks))
	var warnings []envelope.Warning
	var commands []envelope.Command
	for _, task := range tasks {
		route, routeWarnings, routeCommands, errObj := buildRouteReport(ctx, cfg, task, cfg.Routing[task], entries, routeInput{Source: "none"})
		warnings = append(warnings, routeWarnings...)
		commands = append(commands, routeCommands...)
		if errObj != nil {
			return routes, warnings, commands, errObj
		}
		routes = append(routes, statusRoute{
			Task:       task,
			Decision:   route.Decision,
			Candidates: nonNilRouteCandidates(route.Candidates),
			Warnings:   nonNilSnapshotWarnings(routeWarnings),
		})
	}
	return routes, warnings, commands, nil
}

func statusBackendsFromDoctor(backends []inferctl.BackendStatus) []statusBackend {
	out := make([]statusBackend, 0, len(backends))
	for _, backend := range backends {
		var reason *string
		if !backend.Reachable {
			value := "backend_unreachable"
			reason = &value
		}
		out = append(out, statusBackend{
			Name:                 backend.Name,
			Kind:                 backend.Kind,
			BaseURL:              backend.BaseURL,
			Reachable:            backend.Reachable,
			Default:              backend.Default,
			ModelsInstalledCount: backend.ModelsInstalledCount,
			ModelsLoadedCount:    backend.ModelsLoadedCount,
			Error:                reason,
		})
	}
	return out
}

func statusExposedModels(ctx context.Context, entries []backendEntry) []modelListRow {
	var rows []modelListRow
	for _, entry := range entries {
		if _, err := entry.backend.Reachable(ctx); err != nil {
			continue
		}
		installed, err := entry.backend.ListInstalledModels(ctx)
		if err != nil {
			continue
		}
		loadedByModel := map[string]bool{}
		if loaded, err := entry.backend.ListLoadedModels(ctx); err == nil {
			for _, model := range loaded {
				loadedByModel[model.Name] = true
			}
		}
		for _, model := range installed {
			rows = append(rows, modelListRow{
				Name:           model.Name,
				Backend:        model.Backend,
				SizeBytes:      model.SizeBytes,
				Digest:         model.Digest,
				InstalledAtISO: model.InstalledAtISO,
				Loaded:         loadedByModel[model.Name],
				Available:      true,
			})
		}
	}
	slices.SortFunc(rows, func(a, b modelListRow) int {
		if a.Backend == b.Backend {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.Backend, b.Backend)
	})
	if rows == nil {
		return []modelListRow{}
	}
	return rows
}

func nonNilLoadedModels(models []inferctl.LoadedModelInfo) []inferctl.LoadedModelInfo {
	if models == nil {
		return []inferctl.LoadedModelInfo{}
	}
	return models
}

func countReachableStatusBackends(backends []statusBackend) int {
	total := 0
	for _, backend := range backends {
		if backend.Reachable {
			total++
		}
	}
	return total
}

func countReadyStatusRoutes(routes []statusRoute) int {
	total := 0
	for _, route := range routes {
		if route.Decision.Ready {
			total++
		}
	}
	return total
}
