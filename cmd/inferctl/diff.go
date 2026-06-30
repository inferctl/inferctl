package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/spf13/cobra"
)

type diffReport struct {
	Before  diffSnapshotSummary  `json:"before"`
	After   diffSnapshotSummary  `json:"after"`
	Summary diffSummary          `json:"summary"`
	Changes []controlPlaneChange `json:"changes"`
}

type diffSnapshotSummary struct {
	Task                  string `json:"task"`
	SnapshotSchemaVersion string `json:"snapshot_schema_version"`
	InferctlVersion       string `json:"inferctl_version"`
	CapturedAtISO         string `json:"captured_at_iso"`
}

type diffSummary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

func newDiffCommand(jsonFlag *bool) *cobra.Command {
	var beforePath string
	var afterPath string
	var since string
	var task string
	var format string
	cmd := &cobra.Command{
		Use:   "diff (--before <snapshot.json> --after <snapshot.json> | --task <task> --since <relative>)",
		Short: "Compare two inferctl control-plane snapshots",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "human" {
				return writeError(cmd, *jsonFlag, invalidArg("--format", format, "human", []string{"human"}))
			}
			var report diffReport
			var errObj *envelope.Error
			if since != "" {
				if beforePath != "" || afterPath != "" {
					return writeError(cmd, *jsonFlag, invalidArg("--since", since, "cannot be combined with --before or --after", nil))
				}
				if task == "" {
					return writeError(cmd, *jsonFlag, invalidArg("--task", "", "configured task name when --since is used", nil))
				}
				report, errObj = buildSinceDiffReport(cmd.Context(), cmd, task, since)
			} else {
				if beforePath == "" || afterPath == "" {
					return writeError(cmd, *jsonFlag, invalidArg("--before/--after", "missing", "both --before and --after snapshot paths are required, or use --task with --since", []string{"--before", "--after"}))
				}
				report, errObj = buildDiffReport(beforePath, afterPath)
			}
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			return writeData(cmd, *jsonFlag, report, func() error {
				return writeDiffHuman(cmd, report)
			})
		},
	}
	cmd.Flags().StringVar(&beforePath, "before", "", "path to the baseline snapshot JSON artifact")
	cmd.Flags().StringVar(&afterPath, "after", "", "path to the comparison snapshot JSON artifact")
	cmd.Flags().StringVar(&since, "since", "", "compare current task state against the retained baseline at or before this relative time")
	cmd.Flags().StringVar(&task, "task", "", "configured task for --since lookup")
	cmd.Flags().StringVar(&format, "format", "human", "human output format: human")
	return cmd
}

func buildDiffReport(beforePath, afterPath string) (diffReport, *envelope.Error) {
	before, errObj := readControlPlaneSnapshotFile(beforePath, "--before")
	if errObj != nil {
		return diffReport{}, errObj
	}
	after, errObj := readControlPlaneSnapshotFile(afterPath, "--after")
	if errObj != nil {
		return diffReport{}, errObj
	}
	if before.SnapshotSchemaVersion != after.SnapshotSchemaVersion {
		err := invalidArg("snapshot_schema_version", before.SnapshotSchemaVersion+" != "+after.SnapshotSchemaVersion, "matching snapshot schema versions", nil)
		return diffReport{}, &err
	}
	if before.SnapshotSchemaVersion != snapshotSchemaVersion {
		err := invalidArg("snapshot_schema_version", before.SnapshotSchemaVersion, snapshotSchemaVersion, []string{snapshotSchemaVersion})
		return diffReport{}, &err
	}
	changes := classifyControlPlaneChanges(before, after)
	report := diffReport{
		Before:  summarizeDiffSnapshot(before),
		After:   summarizeDiffSnapshot(after),
		Changes: changes,
		Summary: summarizeDiffChanges(changes),
	}
	return report, nil
}

func buildSinceDiffReport(ctx context.Context, cmd *cobra.Command, task, since string) (diffReport, *envelope.Error) {
	baseline, errObj := selectStoredSnapshot(task, since, envMap(), timeNow())
	if errObj != nil {
		return diffReport{}, errObj
	}
	current, _, _, errObj := buildSnapshot(ctx, cmd, snapshotOptions{task: task})
	if errObj != nil {
		return diffReport{}, errObj
	}
	if baseline.Snapshot.SnapshotSchemaVersion != current.SnapshotSchemaVersion {
		err := invalidArg("snapshot_schema_version", baseline.Snapshot.SnapshotSchemaVersion+" != "+current.SnapshotSchemaVersion, "matching snapshot schema versions", nil)
		return diffReport{}, &err
	}
	changes := classifyControlPlaneChanges(baseline.Snapshot, current)
	return diffReport{
		Before:  summarizeDiffSnapshot(baseline.Snapshot),
		After:   summarizeDiffSnapshot(current),
		Changes: changes,
		Summary: summarizeDiffChanges(changes),
	}, nil
}

func readControlPlaneSnapshotFile(path, argName string) (controlPlaneSnapshot, *envelope.Error) {
	data, err := os.ReadFile(path)
	if err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_UNREADABLE",
			Message:   "snapshot file at " + path + " could not be read: " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"arg": argName, "path": path, "reason": err.Error()},
		}
		return controlPlaneSnapshot{}, &errObj
	}
	snapshot, err := decodeControlPlaneSnapshot(data)
	if err != nil {
		errObj := envelope.Error{
			Code:      "E_CONFIG_INVALID",
			Message:   "snapshot file at " + path + " is not a compatible inferctl snapshot: " + err.Error(),
			ExitCode:  exitEnvironment,
			Retryable: false,
			Details:   map[string]any{"arg": argName, "path": path, "reason": err.Error()},
		}
		return controlPlaneSnapshot{}, &errObj
	}
	return snapshot, nil
}

func decodeControlPlaneSnapshot(data []byte) (controlPlaneSnapshot, error) {
	var snapshot controlPlaneSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return controlPlaneSnapshot{}, err
	}
	if snapshot.SnapshotSchemaVersion == "" {
		var wrapped struct {
			Data controlPlaneSnapshot `json:"data"`
		}
		if err := json.Unmarshal(data, &wrapped); err != nil {
			return controlPlaneSnapshot{}, err
		}
		snapshot = wrapped.Data
	}
	if snapshot.SnapshotSchemaVersion == "" {
		return controlPlaneSnapshot{}, fmt.Errorf("missing snapshot_schema_version")
	}
	if snapshot.Task == "" {
		return controlPlaneSnapshot{}, fmt.Errorf("missing task")
	}
	return snapshot, nil
}

func summarizeDiffSnapshot(snapshot controlPlaneSnapshot) diffSnapshotSummary {
	return diffSnapshotSummary{
		Task:                  snapshot.Task,
		SnapshotSchemaVersion: snapshot.SnapshotSchemaVersion,
		InferctlVersion:       snapshot.InferctlVersion,
		CapturedAtISO:         snapshot.CapturedAtISO,
	}
}

func summarizeDiffChanges(changes []controlPlaneChange) diffSummary {
	var summary diffSummary
	summary.Total = len(changes)
	for _, change := range changes {
		switch change.Severity {
		case "high":
			summary.High++
		case "medium":
			summary.Medium++
		case "low":
			summary.Low++
		}
	}
	return summary
}

func writeDiffHuman(cmd *cobra.Command, report diffReport) error {
	fmt.Fprintf(cmd.OutOrStdout(), "diff: %d change(s), %d high\n", report.Summary.Total, report.Summary.High)
	if len(report.Changes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no domain-significant control-plane changes")
		return nil
	}
	limit := min(5, len(report.Changes))
	top := append([]controlPlaneChange{}, report.Changes[:limit]...)
	slices.SortFunc(top, func(a, b controlPlaneChange) int { return a.Rank - b.Rank })
	for _, change := range top {
		fmt.Fprintf(cmd.OutOrStdout(), "- [%s] %s %s: %v -> %v (%s)\n", change.Severity, change.Type, change.Subject, change.Before, change.After, change.Explanation)
	}
	return nil
}
