package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inferctl/inferctl/internal/testserver"
	"github.com/inferctl/inferctl/pkg/inferctl"
)

func TestSnapshotStoreRetention(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	storeDir := t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	for i := 0; i < 3; i++ {
		stdout, _, err := executeForTest("snapshot", "--task", "code", "--store", "--retention-limit", "2")
		if err != nil {
			t.Fatalf("snapshot --store %d error = %v stdout=%s", i, err, stdout)
		}
	}
	entries, err := os.ReadDir(filepath.Join(storeDir, "code"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshots int
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			snapshots++
		}
	}
	if snapshots != 2 {
		t.Fatalf("stored snapshots = %d want 2", snapshots)
	}
}

func TestDiffSinceUsesStoredBaseline(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	storeDir := t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	if stdout, _, err := executeForTest("snapshot", "--task", "code", "--store"); err != nil {
		t.Fatalf("snapshot --store error = %v stdout=%s", err, stdout)
	}

	stdout, _, err := executeForTest("diff", "--task", "code", "--since", "1h", "--json")
	if err != nil {
		t.Fatalf("diff --since error = %v stdout=%s", err, stdout)
	}
	env := decodeDiffEnvelope(t, stdout)
	if env.Data.Summary.Total != 0 {
		t.Fatalf("diff --since should use identical deterministic snapshot: %#v", env.Data)
	}
}

func TestDiffSinceMissingBaseline(t *testing.T) {
	t.Setenv("INFERCTL_SNAPSHOT_DIR", t.TempDir())
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("diff", "--task", "code", "--since", "1h", "--json")
	if err == nil || !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("missing baseline stdout=%s err=%v", stdout, err)
	}
}

func TestDiffSinceMultipleCandidates(t *testing.T) {
	storeDir := t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	taskDir := filepath.Join(storeDir, "code")
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot("code")
	snapshot.CapturedAtISO = time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	writeStoreFixture(t, filepath.Join(taskDir, "a.json"), snapshot)
	writeStoreFixture(t, filepath.Join(taskDir, "b.json"), snapshot)

	stdout, _, err := executeForTest("diff", "--task", "code", "--since", "1h", "--json")
	if err == nil || !strings.Contains(stdout, "multiple snapshots share") {
		t.Fatalf("multiple candidates stdout=%s err=%v", stdout, err)
	}
}

func TestDiffSinceMalformedAndIncompatibleStoredSnapshots(t *testing.T) {
	storeDir := t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	taskDir := filepath.Join(storeDir, "code")
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "bad.json"), []byte("{bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("diff", "--task", "code", "--since", "1h", "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_INVALID") {
		t.Fatalf("malformed stored snapshot stdout=%s err=%v", stdout, err)
	}

	storeDir = t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	taskDir = filepath.Join(storeDir, "code")
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	incompatible := testSnapshot("code")
	incompatible.SnapshotSchemaVersion = "9.9"
	incompatible.CapturedAtISO = time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	writeStoreFixture(t, filepath.Join(taskDir, "old.json"), incompatible)
	stdout, _, err = executeForTest("diff", "--task", "code", "--since", "1h", "--json")
	if err == nil || !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("incompatible stored snapshot stdout=%s err=%v", stdout, err)
	}
}

func TestSnapshotStoreRedactsPromptContent(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	storeDir := t.TempDir()
	t.Setenv("INFERCTL_SNAPSHOT_DIR", storeDir)
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	stdout, _, err := executeForTest("snapshot", "--task", "code", "--prompt", "do not store this", "--store")
	if err != nil {
		t.Fatalf("snapshot --store prompt error = %v stdout=%s", err, stdout)
	}
	rawFiles, err := filepath.Glob(filepath.Join(storeDir, "code", "*.json"))
	if err != nil || len(rawFiles) != 1 {
		t.Fatalf("stored files = %#v err=%v", rawFiles, err)
	}
	raw, err := os.ReadFile(rawFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "do not store this") {
		t.Fatalf("stored snapshot leaked prompt content: %s", raw)
	}
}

func writeStoreFixture(t *testing.T, path string, snapshot controlPlaneSnapshot) {
	t.Helper()
	if snapshot.RouteDecision == (inferctl.RouteDecision{}) {
		snapshot.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "qwen3:8b", Ready: true}
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
