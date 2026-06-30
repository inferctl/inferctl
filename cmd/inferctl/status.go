package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/inferctl/inferctl/internal/config"
	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

const statusFrameSchemaVersion = "0.1"
const statusSchemaVersion = statusFrameSchemaVersion

const defaultStatusWatchInterval = 2 * time.Second

const statusEventSchemaVersion = "0.1"

type statusSnapshot struct {
	StatusFrameSchemaVersion string                      `json:"status_frame_schema_version"`
	ContractVersion          string                      `json:"contract_version"`
	CapturedAtISO            string                      `json:"-"`
	Summary                  statusSummary               `json:"summary"`
	Backends                 []statusBackend             `json:"backends"`
	Models                   statusModels                `json:"models"`
	Routes                   []statusRoute               `json:"routes"`
	Warnings                 []envelope.Warning          `json:"warnings"`
	RecommendedAction        *inferctl.RecommendedAction `json:"recommended_action"`
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

type statusEventBatch struct {
	EventSchemaVersion string        `json:"event_schema_version"`
	ContractVersion    string        `json:"contract_version"`
	CapturedAtISO      string        `json:"captured_at_iso"`
	SinceCapturedAtISO string        `json:"since_captured_at_iso"`
	Events             []statusEvent `json:"events"`
}

type statusEvent struct {
	Sequence int    `json:"sequence"`
	Kind     string `json:"kind"`
	Subject  string `json:"subject"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Before   any    `json:"before"`
	After    any    `json:"after"`
}

type statusOptions struct {
	watch    bool
	interval time.Duration
	events   bool
}

func newStatusCommand(jsonFlag *bool) *cobra.Command {
	opts := statusOptions{interval: defaultStatusWatchInterval}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Emit an aggregate live-state status snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.interval <= 0 {
				return writeError(cmd, *jsonFlag, invalidArg("--interval", opts.interval.String(), "positive duration such as 2s", nil))
			}
			if opts.events && !opts.watch {
				return writeError(cmd, *jsonFlag, invalidArg("--events", "true", "only valid with --watch", []string{"--watch"}))
			}
			if opts.watch {
				return runStatusWatch(cmd, *jsonFlag, opts)
			}
			_, err := writeStatusSnapshot(cmd.Context(), cmd, *jsonFlag)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "emit status snapshots continuously")
	cmd.Flags().DurationVar(&opts.interval, "interval", opts.interval, "watch polling interval")
	cmd.Flags().BoolVar(&opts.events, "events", false, "emit event batches for status changes during watch")
	return cmd
}

func runStatusWatch(cmd *cobra.Command, jsonFlag bool, opts statusOptions) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	previous, err := writeStatusSnapshot(ctx, cmd, jsonFlag)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(opts.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := writeStatusSnapshot(ctx, cmd, jsonFlag)
			if err != nil {
				return err
			}
			if opts.events {
				events := diffStatusSnapshots(previous, current)
				if len(events) > 0 {
					if err := writeStatusEventBatch(cmd, jsonFlag, previous, current, events); err != nil {
						return err
					}
				}
			}
			previous = current
		}
	}
}

func writeStatusSnapshot(ctx context.Context, cmd *cobra.Command, jsonFlag bool) (statusSnapshot, error) {
	snapshot, warnings, commands, errObj := buildStatusSnapshot(ctx)
	if errObj != nil {
		return statusSnapshot{}, writeError(cmd, jsonFlag, *errObj)
	}
	err := writeDataWithDiagnostics(cmd, jsonFlag, snapshot, warnings, commands, func() error {
		fmt.Fprintf(cmd.OutOrStdout(), "status: %d/%d backends reachable, %d loaded models, %d route(s)\n",
			snapshot.Summary.BackendsReachable,
			snapshot.Summary.BackendsTotal,
			snapshot.Summary.ModelsLoadedTotal,
			snapshot.Summary.RoutesTotal,
		)
		return nil
	})
	return snapshot, err
}

func writeStatusEventBatch(cmd *cobra.Command, jsonFlag bool, previous, current statusSnapshot, events []statusEvent) error {
	batch := statusEventBatch{
		EventSchemaVersion: statusEventSchemaVersion,
		ContractVersion:    current.ContractVersion,
		CapturedAtISO:      current.CapturedAtISO,
		SinceCapturedAtISO: previous.CapturedAtISO,
		Events:             events,
	}
	return writeDataWithDiagnostics(cmd, jsonFlag, batch, nil, nil, func() error {
		for _, event := range events {
			fmt.Fprintf(cmd.OutOrStdout(), "event: %s %s\n", event.Kind, event.Summary)
		}
		return nil
	})
}

func diffStatusSnapshots(before, after statusSnapshot) []statusEvent {
	var changes []controlPlaneChange
	changes = append(changes, classifyControlPlaneChanges(statusGlobalSnapshot(before), statusGlobalSnapshot(after))...)
	for _, task := range commonStatusRouteTasks(before, after) {
		changes = append(changes, classifyControlPlaneChanges(statusRouteSnapshot(before, task), statusRouteSnapshot(after, task))...)
	}
	rankChanges(changes)
	events := make([]statusEvent, 0, len(changes))
	for _, change := range changes {
		events = append(events, statusEventFromChange(change))
	}
	return events
}

func statusEventFromChange(change controlPlaneChange) statusEvent {
	return statusEvent{
		Sequence: change.Rank,
		Kind:     change.Type + "_changed",
		Subject:  change.Subject,
		Severity: change.Severity,
		Summary:  change.Explanation,
		Before:   change.Before,
		After:    change.After,
	}
}

func statusGlobalSnapshot(status statusSnapshot) controlPlaneSnapshot {
	return controlPlaneSnapshot{
		Task:                "status",
		BackendReachability: statusBackendReachabilityAsControlPlane(status.Backends),
		LoadedModels:        status.Models.Loaded,
		Warnings:            status.Warnings,
		RecommendedAction:   status.RecommendedAction,
	}
}

func statusRouteSnapshot(status statusSnapshot, task string) controlPlaneSnapshot {
	route, ok := statusRouteByTask(status, task)
	if !ok {
		return controlPlaneSnapshot{Task: task}
	}
	return controlPlaneSnapshot{
		Task:          task,
		RouteDecision: route.Decision,
	}
}

func statusBackendReachabilityAsControlPlane(backends []statusBackend) []backendReachability {
	out := make([]backendReachability, 0, len(backends))
	for _, backend := range backends {
		out = append(out, backendReachability{
			Name:      backend.Name,
			Kind:      backend.Kind,
			Reachable: backend.Reachable,
			Error:     backend.Error,
		})
	}
	return out
}

func commonStatusRouteTasks(before, after statusSnapshot) []string {
	beforeByTask := map[string]bool{}
	for _, route := range before.Routes {
		beforeByTask[route.Task] = true
	}
	var tasks []string
	for _, route := range after.Routes {
		if beforeByTask[route.Task] {
			tasks = append(tasks, route.Task)
		}
	}
	slices.Sort(tasks)
	return tasks
}

func statusRouteByTask(status statusSnapshot, task string) (statusRoute, bool) {
	for _, route := range status.Routes {
		if route.Task == task {
			return route, true
		}
	}
	return statusRoute{}, false
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
		StatusFrameSchemaVersion: statusFrameSchemaVersion,
		ContractVersion:          "0.1",
		CapturedAtISO:            deterministicSnapshotTime().Format("2006-01-02T15:04:05Z"),
		Backends:                 statusBackendsFromDoctor(doctor.Backends),
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
