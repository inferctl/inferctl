package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/testserver"
)

func TestSnapshotRawJSONShapeAndPromptRedaction(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	promptPath := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("sensitive prompt text"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeForTest("snapshot", "--task", "code", "--prompt-file", promptPath)
	if err != nil {
		t.Fatalf("snapshot error = %v stdout=%s", err, stdout)
	}
	if strings.Contains(stdout, promptPath) || strings.Contains(stdout, "sensitive prompt text") {
		t.Fatalf("snapshot leaked prompt path or content: %s", stdout)
	}
	snapshot := decodeSnapshotArtifact(t, stdout)
	if snapshot.SnapshotSchemaVersion != snapshotSchemaVersion || snapshot.Task != "code" || snapshot.CapturedAtISO != "1970-01-01T00:00:00Z" {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.Prompt.Source != "file" || snapshot.Prompt.Basename == nil || *snapshot.Prompt.Basename != "prompt.txt" || snapshot.Prompt.ContentSHA256 == nil {
		t.Fatalf("prompt metadata = %#v", snapshot.Prompt)
	}
	if len(snapshot.BackendReachability) != 1 || len(snapshot.InstalledModels) != 1 || len(snapshot.LoadedModels) != 1 {
		t.Fatalf("snapshot state = %#v", snapshot)
	}
	assertJSONSubsetGolden(t, "snapshot.golden.json", map[string]any{
		"snapshot_schema_version": snapshot.SnapshotSchemaVersion,
		"contract_version":        snapshot.ContractVersion,
		"captured_at_iso":         snapshot.CapturedAtISO,
		"task":                    snapshot.Task,
		"prompt":                  snapshot.Prompt,
		"route_decision":          snapshot.RouteDecision,
	})
}

func TestSnapshotJSONEnvelopeAndOutputFile(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	outPath := filepath.Join(t.TempDir(), "snapshot.json")

	stdout, _, err := executeForTest("snapshot", "--task", "code", "--output", outPath, "--json")
	if err != nil {
		t.Fatalf("snapshot --json --output error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK   bool                 `json:"ok"`
		Data controlPlaneSnapshot `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal snapshot envelope: %v\n%s", err, stdout)
	}
	if !env.OK || env.Data.Task != "code" {
		t.Fatalf("snapshot envelope = %#v", env)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	written := decodeSnapshotArtifact(t, string(raw))
	if written.Task != "code" || strings.Contains(string(raw), `"ok"`) {
		t.Fatalf("output artifact should be raw snapshot: %s", raw)
	}
}

func TestSnapshotInputAndConfigErrors(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	stdout, _, err := executeForTest("snapshot", "--task", "code", "--prompt-file", filepath.Join(t.TempDir(), "missing.txt"), "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_UNREADABLE") {
		t.Fatalf("unreadable prompt stdout=%s err=%v", stdout, err)
	}

	invalidPath := filepath.Join(t.TempDir(), "inferctl.toml")
	if err := os.WriteFile(invalidPath, []byte("[meta]\nschema_version = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INFERCTL_CONFIG", invalidPath)
	stdout, _, err = executeForTest("snapshot", "--task", "code", "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_INVALID") {
		t.Fatalf("invalid config stdout=%s err=%v", stdout, err)
	}
	assertEnvelopeExitCode(t, stdout, exitEnvironment)
}

func TestSnapshotArtifactsWorkWithDiffParser(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	beforePath := filepath.Join(t.TempDir(), "before.json")
	afterPath := filepath.Join(t.TempDir(), "after.json")
	if stdout, _, err := executeForTest("snapshot", "--task", "code", "--output", beforePath); err != nil {
		t.Fatalf("before snapshot error = %v stdout=%s", err, stdout)
	}
	if stdout, _, err := executeForTest("snapshot", "--task", "code", "--output", afterPath); err != nil {
		t.Fatalf("after snapshot error = %v stdout=%s", err, stdout)
	}
	stdout, _, err := executeForTest("diff", "--before", beforePath, "--after", afterPath, "--json")
	if err != nil {
		t.Fatalf("diff snapshot artifacts error = %v stdout=%s", err, stdout)
	}
	env := decodeDiffEnvelope(t, stdout)
	if env.Data.Summary.Total != 0 {
		t.Fatalf("identical snapshot diff = %#v", env.Data)
	}
}

func decodeSnapshotArtifact(t *testing.T, stdout string) controlPlaneSnapshot {
	t.Helper()
	var snapshot controlPlaneSnapshot
	if err := json.Unmarshal([]byte(stdout), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot artifact: %v\n%s", err, stdout)
	}
	return snapshot
}
