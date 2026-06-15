package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/Ozhiaki/inferctl/internal/envelope"
	internalversion "github.com/Ozhiaki/inferctl/internal/version"
	"github.com/spf13/cobra"
)

type versionData struct {
	ToolVersion     string            `json:"tool_version"`
	Build           versionBuild      `json:"build"`
	Dependencies    map[string]string `json:"dependencies"`
	ContractVersion string            `json:"contract_version"`
	SchemaVersion   string            `json:"schema_version"`
	Update          versionUpdate     `json:"update"`
}

type versionBuild struct {
	Commit    string `json:"commit"`
	DateISO   string `json:"date_iso"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

type versionUpdate struct {
	Checked         bool    `json:"checked"`
	LatestKnown     *string `json:"latest_known"`
	UpdateAvailable *bool   `json:"update_available"`
	CheckedAtISO    *string `json:"checked_at_iso"`
}

func newVersionCommand(jsonFlag *bool) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, build, and contract metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data := buildVersionData()
			var warnings []envelope.Warning
			if check {
				update, warning := checkForUpdates()
				data.Update = update
				if warning != nil {
					warnings = append(warnings, *warning)
				}
			}
			return writeDataWithDiagnostics(cmd, *jsonFlag, data, warnings, nil, func() error {
				fmt.Fprintf(cmd.OutOrStdout(), "inferctl %s\n", data.ToolVersion)
				fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", data.Build.Commit)
				fmt.Fprintf(cmd.OutOrStdout(), "date: %s\n", data.Build.DateISO)
				fmt.Fprintf(cmd.OutOrStdout(), "go: %s %s/%s\n", data.Build.GoVersion, data.Build.OS, data.Build.Arch)
				fmt.Fprintf(cmd.OutOrStdout(), "contract: %s schema: %s\n", data.ContractVersion, data.SchemaVersion)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "check for newer releases")
	return cmd
}

func buildVersionData() versionData {
	commit, date, deps := buildMetadata()
	return versionData{
		ToolVersion: internalversion.Tool(),
		Build: versionBuild{
			Commit:    commit,
			DateISO:   date,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		},
		Dependencies:    deps,
		ContractVersion: "0.1",
		SchemaVersion:   "0.1",
		Update:          versionUpdate{Checked: false},
	}
}

func buildMetadata() (string, string, map[string]string) {
	commit := internalversion.Commit()
	date := internalversion.Date()
	deps := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			switch dep.Path {
			case "github.com/spf13/cobra":
				deps["cobra"] = dep.Version
			case "github.com/pelletier/go-toml/v2":
				deps["go-toml/v2"] = dep.Version
			case "github.com/oklog/ulid/v2":
				deps["ulid/v2"] = dep.Version
			}
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == "" {
					commit = setting.Value
				}
			case "vcs.time":
				if date == "" {
					date = setting.Value
				}
			}
		}
	}
	return commit, date, deps
}

func checkForUpdates() (versionUpdate, *envelope.Warning) {
	endpoint := os.Getenv("INFERCTL_UPDATE_CHECK_URL")
	if endpoint == "" {
		endpoint = "https://api.github.com/repos/Ozhiaki/inferctl/releases/latest"
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return versionUpdate{Checked: false}, updateCheckWarning(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return versionUpdate{Checked: false}, updateCheckWarning(fmt.Errorf("status %d", resp.StatusCode))
	}
	var body struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return versionUpdate{Checked: false}, updateCheckWarning(err)
	}
	latest := body.TagName
	if latest == "" {
		latest = body.Name
	}
	if latest == "" {
		return versionUpdate{Checked: false}, updateCheckWarning(fmt.Errorf("release response did not include tag_name"))
	}
	current := internalversion.Tool()
	available := latest != current && latest != "v"+current
	checkedAt := time.Now().UTC().Format(time.RFC3339Nano)
	return versionUpdate{
		Checked:         true,
		LatestKnown:     &latest,
		UpdateAvailable: &available,
		CheckedAtISO:    &checkedAt,
	}, nil
}

func updateCheckWarning(err error) *envelope.Warning {
	return &envelope.Warning{
		Code:    "W_UPDATE_CHECK_FAILED",
		Message: "failed to check for updates: " + err.Error(),
		Details: map[string]any{
			"reason": err.Error(),
		},
	}
}
