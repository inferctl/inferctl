package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

func TestDiffFileToFileClassifiesDomainChanges(t *testing.T) {
	before := testSnapshot("code")
	before.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "primary:70b", Ready: true}
	before.BackendReachability = []backendReachability{{Name: "ollama", Kind: "ollama", Reachable: true}}
	before.LoadedModels = []inferctl.LoadedModelInfo{{Name: "primary:70b", Backend: "ollama"}}
	before.RecommendedAction = &inferctl.RecommendedAction{Command: "inferctl model primary:70b --json"}

	after := testSnapshot("code")
	after.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "fallback:8b", IsFallback: true, Ready: false}
	after.BackendReachability = []backendReachability{{Name: "ollama", Kind: "ollama", Reachable: false}}
	after.Warnings = []envelope.Warning{{Code: "W_FALLBACK_USED", Message: "fallback used"}, {Code: "W_MODEL_NOT_LOADED", Message: "not loaded"}}
	after.Errors = []envelope.Error{{Code: "E_NO_ROUTE_AVAILABLE", Message: "no route", ExitCode: exitTransient, Retryable: true}}
	after.RecommendedAction = &inferctl.RecommendedAction{Command: "inferctl doctor --json"}

	stdout, _, err := executeForTest("diff", "--before", writeSnapshotFile(t, before), "--after", writeSnapshotFile(t, after), "--json")
	if err != nil {
		t.Fatalf("diff error = %v stdout=%s", err, stdout)
	}
	env := decodeDiffEnvelope(t, stdout)
	if !env.OK || env.Data.Summary.Total < 7 || env.Data.Summary.High < 3 {
		t.Fatalf("diff summary = %#v ok=%v", env.Data.Summary, env.OK)
	}
	seen := map[string]bool{}
	for i, change := range env.Data.Changes {
		if change.Rank != i+1 {
			t.Fatalf("rank mismatch at %d: %#v", i, change)
		}
		seen[change.Type] = true
	}
	for _, typ := range []string{"selected_route", "fallback_status", "backend_reachability", "selected_model_readiness", "warning_codes", "error_codes", "recommended_action", "loaded_model_count"} {
		if !seen[typ] {
			t.Fatalf("missing change type %s in %#v", typ, env.Data.Changes)
		}
	}
	assertJSONSubsetGolden(t, "diff.golden.json", map[string]any{
		"summary": env.Data.Summary,
		"changes": env.Data.Changes[:3],
	})
}

func TestDiffHumanOutputUsesSameChanges(t *testing.T) {
	before := testSnapshot("code")
	before.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "primary:70b", Ready: true}
	after := testSnapshot("code")
	after.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "fallback:8b", IsFallback: true, Ready: true}

	stdout, _, err := executeForTest("diff", "--before", writeSnapshotFile(t, before), "--after", writeSnapshotFile(t, after), "--format", "human")
	if err != nil {
		t.Fatalf("human diff error = %v stdout=%s", err, stdout)
	}
	assertOrderedSubstrings(t, stdout, []string{
		"Local inference drift detected",
		"Route changed:",
		"- before: primary:70b on ollama",
		"- after:  fallback:8b on ollama",
		"- reason: selected route changed from primary:70b on ollama to fallback:8b on ollama",
		"- fallback: fallback introduced",
	})
	if strings.Contains(stdout, "next:") {
		t.Fatalf("human diff should not invent a next diagnostic:\n%s", stdout)
	}
}

func TestDiffHumanOutputSortsByRankBeforeGrouping(t *testing.T) {
	report := diffReport{
		Summary: diffSummary{Total: 3, High: 3},
		Changes: []controlPlaneChange{
			{Rank: 3, Type: "backend_reachability", Subject: "llamacpp", Severity: "high", Before: "reachable", After: "unreachable:backend_unreachable", Explanation: "backend reachability changed (backend_unreachable)"},
			{Rank: 1, Type: "selected_route", Subject: "code", Severity: "high", Before: "llamacpp/primary.gguf", After: "ollama/fallback:8b", Explanation: "selected route changed from primary.gguf on llamacpp to fallback:8b on ollama"},
			{Rank: 2, Type: "fallback_status", Subject: "code", Severity: "high", Before: false, After: true, Explanation: "fallback introduced"},
		},
	}
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	if err := writeDiffHuman(cmd, report); err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	assertOrderedSubstrings(t, got, []string{
		"Route changed:",
		"- before: primary.gguf on llamacpp",
		"- fallback: fallback introduced",
		"Backend reachability changed:",
		"- llamacpp: reachable -> unreachable:backend_unreachable",
	})
	if strings.Contains(got, "next:") {
		t.Fatalf("human output = %s", got)
	}
}

func TestDiffIgnoresTimestampOnlyChanges(t *testing.T) {
	before := testSnapshot("code")
	before.CapturedAtISO = "2026-06-30T00:00:00Z"
	before.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "qwen3:8b", Ready: true}
	after := before
	after.CapturedAtISO = "2026-06-30T00:01:00Z"

	stdout, _, err := executeForTest("diff", "--before", writeSnapshotFile(t, before), "--after", writeSnapshotFile(t, after), "--json")
	if err != nil {
		t.Fatalf("timestamp-only diff error = %v stdout=%s", err, stdout)
	}
	env := decodeDiffEnvelope(t, stdout)
	if env.Data.Summary.Total != 0 {
		t.Fatalf("timestamp-only changes should be ignored: %#v", env.Data)
	}
}

func TestDiffInputValidation(t *testing.T) {
	valid := writeSnapshotFile(t, testSnapshot("code"))
	stdout, _, err := executeForTest("diff", "--before", filepath.Join(t.TempDir(), "missing.json"), "--after", valid, "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_UNREADABLE") {
		t.Fatalf("missing input stdout=%s err=%v", stdout, err)
	}

	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = executeForTest("diff", "--before", bad, "--after", valid, "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_INVALID") {
		t.Fatalf("malformed input stdout=%s err=%v", stdout, err)
	}

	incompatible := testSnapshot("code")
	incompatible.SnapshotSchemaVersion = "9.9"
	stdout, _, err = executeForTest("diff", "--before", valid, "--after", writeSnapshotFile(t, incompatible), "--json")
	if err == nil || !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("incompatible input stdout=%s err=%v", stdout, err)
	}
}

func TestDiffAcceptsEnvelopeWrappedSnapshot(t *testing.T) {
	before := testSnapshot("code")
	before.RouteDecision = inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "qwen3:8b", Ready: true}
	after := before
	after.RouteDecision.Ready = false
	wrappedPath := filepath.Join(t.TempDir(), "wrapped.json")
	wrapped := map[string]any{"ok": true, "data": after}
	raw, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrappedPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeForTest("diff", "--before", writeSnapshotFile(t, before), "--after", wrappedPath, "--json")
	if err != nil {
		t.Fatalf("wrapped snapshot diff error = %v stdout=%s", err, stdout)
	}
	env := decodeDiffEnvelope(t, stdout)
	if env.Data.Summary.Total != 1 || env.Data.Changes[0].Type != "selected_model_readiness" {
		t.Fatalf("wrapped diff = %#v", env.Data)
	}
}

func testSnapshot(task string) controlPlaneSnapshot {
	snapshot := newControlPlaneSnapshot(task, promptMetadata{SourceKind: "none", Source: "none"})
	snapshot.CapturedAtISO = "2026-06-30T00:00:00Z"
	snapshot.InferctlVersion = "dev"
	snapshot.RouteCandidates = []inferctl.RouteCandidate{}
	snapshot.BackendReachability = []backendReachability{}
	snapshot.LoadedModels = []inferctl.LoadedModelInfo{}
	snapshot.InstalledModels = []inferctl.ModelInfo{}
	snapshot.Warnings = []envelope.Warning{}
	snapshot.Errors = []envelope.Error{}
	return snapshot
}

func writeSnapshotFile(t *testing.T, snapshot controlPlaneSnapshot) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "snapshot.json")
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type diffEnvelopeForTest struct {
	OK   bool       `json:"ok"`
	Data diffReport `json:"data"`
}

func decodeDiffEnvelope(t *testing.T, stdout string) diffEnvelopeForTest {
	t.Helper()
	var env diffEnvelopeForTest
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal diff envelope: %v\n%s", err, stdout)
	}
	return env
}
