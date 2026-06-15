package main

import (
	"encoding/json"
	"strings"
	"testing"

	internalversion "github.com/Ozhiaki/inferctl/internal/version"
)

func TestVersionJSONNoCheck(t *testing.T) {
	stdout, _, err := executeForTest("version", "--json")
	if err != nil {
		t.Fatalf("version error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.ToolVersion != internalversion.Tool() || env.Data.ContractVersion != "0.1" || env.Data.SchemaVersion != "0.1" {
		t.Fatalf("version data = %#v", env.Data)
	}
	if env.Data.Build.GoVersion == "" || env.Data.Build.OS == "" || env.Data.Build.Arch == "" {
		t.Fatalf("build metadata incomplete = %#v", env.Data.Build)
	}
	if env.Data.Dependencies["cobra"] == "" || env.Data.Dependencies["go-toml/v2"] == "" {
		t.Fatalf("dependencies missing = %#v", env.Data.Dependencies)
	}
	if env.Data.Update.Checked || env.Data.Update.LatestKnown != nil || env.Data.Update.UpdateAvailable != nil || env.Data.Update.CheckedAtISO != nil {
		t.Fatalf("update should be unchecked without --check: %#v", env.Data.Update)
	}
}

func TestVersionCheckFailureWarnsAndExitsZero(t *testing.T) {
	t.Setenv("INFERCTL_UPDATE_CHECK_URL", "http://127.0.0.1:1/nope")

	stdout, _, err := executeForTest("version", "--check", "--json")
	if err != nil {
		t.Fatalf("version --check should exit 0 on check failure: %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "W_UPDATE_CHECK_FAILED") {
		t.Fatalf("missing update warning: %s", stdout)
	}
	var env struct {
		Data versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Update.Checked {
		t.Fatalf("failed check should leave checked=false: %#v", env.Data.Update)
	}
}
