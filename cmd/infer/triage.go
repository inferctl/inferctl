package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type triageReport struct {
	Summary           triageSummary               `json:"summary"`
	Inputs            []triageInput               `json:"inputs"`
	Items             []triageItem                `json:"items"`
	RecommendedAction *inferctl.RecommendedAction `json:"recommended_action"`
}

type triageSummary struct {
	Total    int `json:"total"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type triageInput struct {
	Source string `json:"source"`
	OK     bool   `json:"ok"`
}

type triageItem struct {
	Rank     int               `json:"rank"`
	Severity string            `json:"severity"`
	Code     string            `json:"code"`
	Subject  string            `json:"subject"`
	Message  string            `json:"message"`
	Source   string            `json:"source"`
	Backend  *string           `json:"backend"`
	Command  *envelope.Command `json:"command"`
	Details  map[string]any    `json:"details"`
}

type triageOptions struct {
	inputFile string
	backend   string
	severity  string
	limit     int
}

type priorEnvelope struct {
	OK       bool               `json:"ok"`
	Data     json.RawMessage    `json:"data"`
	Warnings []envelope.Warning `json:"warnings"`
	Commands []envelope.Command `json:"commands"`
	Errors   []envelope.Error   `json:"errors"`
}

func newTriageCommand(jsonFlag *bool) *cobra.Command {
	opts := triageOptions{}
	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Rank deterministic diagnostic next actions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.severity != "" && !slices.Contains([]string{"error", "warning", "info"}, opts.severity) {
				return writeError(cmd, *jsonFlag, invalidArg("--severity", opts.severity, "one of error, warning, info", []string{"error", "warning", "info"}))
			}
			if opts.limit < 0 {
				return writeError(cmd, *jsonFlag, invalidArg("--limit", fmt.Sprint(opts.limit), "integer >= 0", nil))
			}
			report, commands, errObj := buildTriageReport(cmd.Context(), opts)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			report.RecommendedAction = recommendedAction(commands)
			return writeDataWithDiagnostics(cmd, *jsonFlag, report, nil, commands, func() error {
				return writeTriageHuman(cmd, report)
			})
		},
	}
	cmd.Flags().StringVar(&opts.inputFile, "input-file", "", "read a prior JSON envelope instead of probing live inputs")
	cmd.Flags().StringVar(&opts.backend, "backend", "", "only include items for a backend")
	cmd.Flags().StringVar(&opts.severity, "severity", "", "only include items with severity error, warning, or info")
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "maximum number of items to emit; 0 means no limit")
	return cmd
}

func buildTriageReport(ctx context.Context, opts triageOptions) (triageReport, []envelope.Command, *envelope.Error) {
	var report triageReport
	var items []triageItem
	var commands []envelope.Command
	if opts.inputFile != "" {
		inputItems, inputCommands, input, errObj := triageFromInputFile(opts.inputFile)
		if errObj != nil {
			return report, nil, errObj
		}
		report.Inputs = append(report.Inputs, input)
		items = append(items, inputItems...)
		commands = append(commands, inputCommands...)
	} else {
		liveItems, liveCommands, inputs := triageFromLiveInputs(ctx)
		report.Inputs = append(report.Inputs, inputs...)
		items = append(items, liveItems...)
		commands = append(commands, liveCommands...)
	}
	items = filterTriageItems(items, opts)
	sortTriageItems(items)
	if opts.limit > 0 && len(items) > opts.limit {
		items = items[:opts.limit]
	}
	for i := range items {
		items[i].Rank = i + 1
	}
	commands = commandsFromTriageItems(items, commands)
	report.Items = items
	report.Summary = summarizeTriageItems(items)
	return report, commands, nil
}

func triageFromLiveInputs(ctx context.Context) ([]triageItem, []envelope.Command, []triageInput) {
	inputs := []triageInput{}
	var items []triageItem
	var commands []envelope.Command
	result, err := (config.Loader{}).Load(config.LoadOptions{})
	if err != nil {
		errObj := configLoadError(err)
		items = append(items, triageItemFromError("config.load", errObj))
		inputs = append(inputs, triageInput{Source: "config.load", OK: false})
		return items, commands, inputs
	}
	inputs = append(inputs, triageInput{Source: "config.validate", OK: true})
	validation := config.Validate(result, false)
	for _, finding := range validation.Findings {
		items = append(items, triageItemFromFinding(finding))
	}
	entries, errObj := configuredBackends(result, "", "")
	if errObj != nil {
		items = append(items, triageItemFromError("backends.config", *errObj))
		inputs = append(inputs, triageInput{Source: "doctor", OK: false})
		return items, commands, inputs
	}
	for _, entry := range entries {
		if _, err := entry.backend.Reachable(ctx); err != nil {
			if errObj := backendReadError(entry, err); errObj != nil {
				items = append(items, triageItemFromError("doctor", *errObj))
			}
		}
	}
	_, warnings, doctorCommands := buildDoctorReport(ctx, result.Config, entries, false)
	for _, warning := range warnings {
		items = append(items, triageItemFromWarning("doctor", warning))
	}
	commands = append(commands, doctorCommands...)
	inputs = append(inputs, triageInput{Source: "doctor", OK: true})
	return items, commands, inputs
}

func triageFromInputFile(path string) ([]triageItem, []envelope.Command, triageInput, *envelope.Error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, triageInput{}, &envelope.Error{
			Code:      "E_CONFIG_UNREADABLE",
			Message:   "input file at " + path + " could not be read: " + err.Error(),
			ExitCode:  3,
			Retryable: false,
			Details:   map[string]any{"path": path, "reason": err.Error()},
		}
	}
	var prior priorEnvelope
	if err := json.Unmarshal(data, &prior); err != nil {
		return nil, nil, triageInput{}, &envelope.Error{
			Code:      "E_CONFIG_INVALID",
			Message:   "input file at " + path + " is not a JSON envelope",
			ExitCode:  3,
			Retryable: false,
			Details:   map[string]any{"path": path, "parse_error": err.Error()},
		}
	}
	var items []triageItem
	for _, errObj := range prior.Errors {
		items = append(items, triageItemFromError("input_file", errObj))
	}
	for _, warning := range prior.Warnings {
		items = append(items, triageItemFromWarning("input_file", warning))
	}
	items = append(items, triageFindingsFromRawData(prior.Data)...)
	input := triageInput{Source: "input_file:" + path, OK: prior.OK}
	return items, prior.Commands, input, nil
}

func triageFindingsFromRawData(raw json.RawMessage) []triageItem {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var validation struct {
		Findings []inferctl.Finding `json:"findings"`
	}
	if err := json.Unmarshal(raw, &validation); err != nil || len(validation.Findings) == 0 {
		return nil
	}
	items := make([]triageItem, 0, len(validation.Findings))
	for _, finding := range validation.Findings {
		items = append(items, triageItemFromFinding(finding))
	}
	return items
}

func triageItemFromFinding(finding inferctl.Finding) triageItem {
	code := "E_CONFIG_VALIDATION_FAILED"
	if finding.Severity == "warning" {
		code = "W_CONFIG_VALIDATION"
	}
	if raw, ok := finding.Details["code"].(string); ok && raw != "" {
		code = raw
	}
	command := envelope.Command{
		Label:     "Explain config key '" + finding.Key + "'",
		Command:   "infer config explain --key " + finding.Key + " --json",
		Rationale: "Shows the expected type, allowed values, and examples for this config key",
	}
	return triageItem{
		Severity: finding.Severity,
		Code:     code,
		Subject:  finding.Key,
		Message:  finding.Message,
		Source:   "config.validate",
		Backend:  backendFromSubject(finding.Key),
		Command:  &command,
		Details:  finding.Details,
	}
}

func triageItemFromWarning(source string, warning envelope.Warning) triageItem {
	command := commandForWarning(warning)
	return triageItem{
		Severity: "warning",
		Code:     warning.Code,
		Subject:  subjectFromDetails(warning.Details, warning.Code),
		Message:  warning.Message,
		Source:   source,
		Backend:  stringDetail(warning.Details, "backend"),
		Command:  command,
		Details:  warning.Details,
	}
}

func triageItemFromError(source string, errObj envelope.Error) triageItem {
	command := (*envelope.Command)(nil)
	if errObj.DidYouMean != nil {
		command = &envelope.Command{
			Label:     "Inspect " + errObj.Code,
			Command:   *errObj.DidYouMean,
			Rationale: "Recommended by the originating diagnostic",
		}
	}
	return triageItem{
		Severity: "error",
		Code:     errObj.Code,
		Subject:  subjectFromDetails(errObj.Details, errObj.Code),
		Message:  errObj.Message,
		Source:   source,
		Backend:  stringDetail(errObj.Details, "backend"),
		Command:  command,
		Details:  errObj.Details,
	}
}

func commandForWarning(warning envelope.Warning) *envelope.Command {
	switch warning.Code {
	case "W_BACKEND_UNREACHABLE", "W_BACKEND_DEGRADED":
		if backend := stringDetail(warning.Details, "backend"); backend != nil {
			return &envelope.Command{
				Label:     "Inspect backend '" + *backend + "'",
				Command:   "infer backends --filter " + *backend + " --json",
				Rationale: "Surfaces backend reachability and model counts",
			}
		}
	case "W_MODEL_NOT_INSTALLED", "W_MODEL_NOT_LOADED":
		if model := stringDetail(warning.Details, "model"); model != nil {
			return &envelope.Command{
				Label:     "Inspect model '" + *model + "'",
				Command:   "infer model " + *model + " --json",
				Rationale: "Shows placements, capabilities, loading state, and routing usage",
			}
		}
	case "W_FALLBACK_USED", "W_CONTEXT_NEAR_LIMIT":
		if task := stringDetail(warning.Details, "task"); task != nil {
			return &envelope.Command{
				Label:     "Explain route '" + *task + "'",
				Command:   "infer route " + *task + " --json",
				Rationale: "Shows route candidates and constraints",
			}
		}
	}
	return nil
}

func filterTriageItems(items []triageItem, opts triageOptions) []triageItem {
	out := make([]triageItem, 0, len(items))
	for _, item := range items {
		if opts.severity != "" && item.Severity != opts.severity {
			continue
		}
		if opts.backend != "" && (item.Backend == nil || *item.Backend != opts.backend) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sortTriageItems(items []triageItem) {
	slices.SortFunc(items, func(a, b triageItem) int {
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) - severityRank(b.Severity)
		}
		if a.Code != b.Code {
			return strings.Compare(a.Code, b.Code)
		}
		if a.Subject != b.Subject {
			return strings.Compare(a.Subject, b.Subject)
		}
		if a.Source != b.Source {
			return strings.Compare(a.Source, b.Source)
		}
		return strings.Compare(a.Message, b.Message)
	})
}

func severityRank(severity string) int {
	switch severity {
	case "error":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

func summarizeTriageItems(items []triageItem) triageSummary {
	summary := triageSummary{Total: len(items)}
	for _, item := range items {
		switch item.Severity {
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

func commandsFromTriageItems(items []triageItem, extra []envelope.Command) []envelope.Command {
	commands := make([]envelope.Command, 0, len(items)+len(extra))
	for _, item := range items {
		if item.Command != nil {
			commands = append(commands, *item.Command)
		}
	}
	commands = append(commands, extra...)
	return dedupeCommands(commands)
}

func subjectFromDetails(details map[string]any, fallback string) string {
	for _, key := range []string{"backend", "task", "model", "path"} {
		if value := stringDetail(details, key); value != nil && *value != "" {
			return *value
		}
	}
	return fallback
}

func backendFromSubject(subject string) *string {
	if !strings.HasPrefix(subject, "backends.") {
		return nil
	}
	parts := strings.Split(subject, ".")
	if len(parts) < 2 || parts[1] == "*" {
		return nil
	}
	return &parts[1]
}

func stringDetail(details map[string]any, key string) *string {
	if details == nil {
		return nil
	}
	value, ok := details[key].(string)
	if !ok || value == "" {
		return nil
	}
	return &value
}

func writeTriageHuman(cmd *cobra.Command, report triageReport) error {
	fmt.Fprintf(cmd.OutOrStdout(), "triage: %d item(s), %d error(s), %d warning(s)\n",
		report.Summary.Total, report.Summary.Errors, report.Summary.Warnings)
	for _, item := range report.Items {
		fmt.Fprintf(cmd.OutOrStdout(), "%d. %s %s %s: %s\n", item.Rank, item.Severity, item.Code, item.Subject, item.Message)
	}
	if report.RecommendedAction != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", report.RecommendedAction.Command)
	}
	return nil
}
