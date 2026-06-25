package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalversion "github.com/inferctl/inferctl/internal/version"
)

func TestVersionJSONNoCheck(t *testing.T) {
	stdout, _, err := executeForTest("version", "--json")
	if err != nil {
		t.Fatalf("version error = %v stdout=%s", err, stdout)
	}
	var env struct {
		ToolVersion string      `json:"tool_version"`
		Data        versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.ToolVersion != internalversion.Tool() || env.Data.ContractVersion != "0.1" || env.Data.SchemaVersion != "0.1" {
		t.Fatalf("version data = %#v", env.Data)
	}
	if env.ToolVersion != env.Data.ToolVersion {
		t.Fatalf("top-level tool_version = %q, data.tool_version = %q", env.ToolVersion, env.Data.ToolVersion)
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
		ToolVersion string      `json:"tool_version"`
		Data        versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Update.Checked {
		t.Fatalf("failed check should leave checked=false: %#v", env.Data.Update)
	}
	if env.ToolVersion != env.Data.ToolVersion {
		t.Fatalf("top-level tool_version = %q, data.tool_version = %q", env.ToolVersion, env.Data.ToolVersion)
	}
}

func TestVersionCheckSuccessHonorsDeterministicTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer server.Close()
	t.Setenv("INFERCTL_UPDATE_CHECK_URL", server.URL)
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	t.Setenv("SOURCE_DATE_EPOCH", "946684800")

	first, _, err := executeForTest("version", "--check", "--json")
	if err != nil {
		t.Fatalf("first version --check error = %v stdout=%s", err, first)
	}
	second, _, err := executeForTest("version", "--check", "--json")
	if err != nil {
		t.Fatalf("second version --check error = %v stdout=%s", err, second)
	}
	if first != second {
		t.Fatalf("deterministic version --check output drifted\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	var env struct {
		Data versionData `json:"data"`
	}
	if err := json.Unmarshal([]byte(first), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Update.CheckedAtISO == nil || *env.Data.Update.CheckedAtISO != "2000-01-01T00:00:00Z" {
		t.Fatalf("checked_at_iso did not honor SOURCE_DATE_EPOCH: %#v", env.Data.Update)
	}
}
