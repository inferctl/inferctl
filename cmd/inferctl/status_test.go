package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/inferctl/inferctl/internal/envelope"
	"github.com/inferctl/inferctl/internal/testserver"
	"github.com/inferctl/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

func TestStatusJSONEnvelopeMatchesGolden(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	up := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b", SizeBytes: 1}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b", VRAMBytes: 1}},
	})
	defer up.Close()
	down := testserver.New(testserver.Fixture{Kind: testserver.KindLlamaCPP, Unreachable: true})
	defer down.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: up.URL,
		LlamaURL:  down.URL,
		Primary:   "primary.gguf",
		Fallback:  "fallback:8b",
	}))

	stdout, _, err := executeForTest("status", "--json")
	if err != nil {
		t.Fatalf("status error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK   bool           `json:"ok"`
		Data statusSnapshot `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal status envelope: %v\n%s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("status envelope not ok: %s", stdout)
	}
	if env.Data.StatusFrameSchemaVersion != statusFrameSchemaVersion || strings.Contains(stdout, "captured_at_iso") {
		t.Fatalf("status frame schema/time contract violated: %#v stdout=%s", env.Data, stdout)
	}
	normalizeStatusForGolden(&env.Data)
	assertJSONSubsetGolden(t, "status.golden.json", env.Data)
}

func TestStatusDataHashStableAcrossUnchangedState(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b", SizeBytes: 1}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b", VRAMBytes: 1}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "fallback:8b",
	}))

	first, _, err := executeForTest("status", "--json")
	if err != nil {
		t.Fatalf("first status error = %v stdout=%s", err, first)
	}
	time.Sleep(5 * time.Millisecond)
	second, _, err := executeForTest("status", "--json")
	if err != nil {
		t.Fatalf("second status error = %v stdout=%s", err, second)
	}
	firstHash := statusDataHash(t, first)
	secondHash := statusDataHash(t, second)
	if firstHash == "" || firstHash != secondHash {
		t.Fatalf("data hash changed for unchanged state: first=%q second=%q", firstHash, secondHash)
	}
}

func TestStatusFramePrivacyExclusions(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b", SizeBytes: 1}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b", VRAMBytes: 1}},
	})
	defer server.Close()
	configPath := filepath.Join(t.TempDir(), "private-config.toml")
	promptPath := filepath.Join(t.TempDir(), "prompt-with-sensitive-name.txt")
	if err := os.WriteFile(promptPath, []byte("do not expose this prompt text"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := writeDoctorConfigBody(doctorConfigOptions{OllamaURL: server.URL, Primary: "fallback:8b"})
	cfg = strings.Replace(cfg, "default = true\n", "default = true\nauth_header_name = \"Authorization\"\nauth_header_value = \"secret-token-value\"\n", 1)
	if err := os.WriteFile(configPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INFERCTL_CONFIG", configPath)

	stdout, _, err := executeForTest("status", "--json")
	if err != nil {
		t.Fatalf("status error = %v stdout=%s", err, stdout)
	}
	for _, forbidden := range []string{
		"secret-token-value",
		"Authorization",
		configPath,
		promptPath,
		"prompt-with-sensitive-name.txt",
		"do not expose this prompt text",
		"[meta]",
		"auth_header_value",
	} {
		if strings.Contains(stdout, forbidden) {
			t.Fatalf("status leaked %q in %s", forbidden, stdout)
		}
	}
}

func TestStatusCoversSupportedBackendKinds(t *testing.T) {
	ollamaServer := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "ollama-model"}},
		Loaded: []testserver.LoadedModel{{Name: "ollama-model"}},
	})
	defer ollamaServer.Close()
	llamaServer := testserver.New(testserver.Fixture{Kind: testserver.KindLlamaCPP, Models: []testserver.Model{{Name: "llama-model"}}})
	defer llamaServer.Close()
	openaiCompatServer := testserver.New(testserver.Fixture{Kind: testserver.KindOpenAICompat, Models: []testserver.Model{{Name: "compat-model"}}})
	defer openaiCompatServer.Close()
	lmStudioServer := testserver.New(testserver.Fixture{Kind: testserver.KindLMStudio, Models: []testserver.Model{{Name: "lmstudio-model"}}})
	defer lmStudioServer.Close()
	mlxServer := testserver.New(testserver.Fixture{Kind: testserver.KindMLX, Models: []testserver.Model{{Name: "mlx-model"}}})
	defer mlxServer.Close()
	t.Setenv("INFERCTL_CONFIG", writeStatusAllKindsConfig(t, ollamaServer.URL, llamaServer.URL, openaiCompatServer.URL, lmStudioServer.URL, mlxServer.URL))

	stdout, _, err := executeForTest("status", "--json")
	if err != nil {
		t.Fatalf("status all-kinds error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data statusSnapshot `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal status envelope: %v\n%s", err, stdout)
	}
	for _, kind := range []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"} {
		if !hasStatusBackendKind(env.Data.Backends, kind) {
			t.Fatalf("status missing backend kind %s in %#v", kind, env.Data.Backends)
		}
	}
	if env.Data.Summary.BackendsTotal != 5 || env.Data.Summary.BackendsReachable != 5 || len(env.Data.Models.Exposed) < 5 {
		t.Fatalf("status summary = %#v exposed=%#v", env.Data.Summary, env.Data.Models.Exposed)
	}
}

func TestStatusWatchJSONEmitsRepeatedSnapshotsAndStopsOnContextCancel(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b", SizeBytes: 1}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b", VRAMBytes: 1}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "fallback:8b",
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelAfterLinesWriter{after: 2, cancel: cancel}
	var stderr bytes.Buffer
	cmd := newRootCommand()
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"status", "--json", "--watch", "--interval", "1ms"})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("status watch error = %v stderr=%s stdout=%s", err, stderr.String(), out.String())
		}
	case <-time.After(time.Second):
		cancel()
		t.Fatal("status watch did not stop after context cancellation")
	}

	lines := assertCompleteJSONLines(t, out.String(), 2)
	for _, line := range lines {
		var env struct {
			OK   bool           `json:"ok"`
			Data statusSnapshot `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("unmarshal watch envelope: %v\n%s", err, line)
		}
		if !env.OK || env.Data.StatusFrameSchemaVersion != statusFrameSchemaVersion {
			t.Fatalf("unexpected watch envelope: %#v", env)
		}
	}
}

func TestStatusWatchEventsEmitsNewlineDelimitedEventBatch(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	previousWriter := writeStatusSnapshotWatch
	t.Cleanup(func() { writeStatusSnapshotWatch = previousWriter })

	snapshots := []statusSnapshot{
		{
			StatusFrameSchemaVersion: statusFrameSchemaVersion,
			ContractVersion:          "0.1",
			CapturedAtISO:            "2026-06-30T15:00:00Z",
			Backends:                 []statusBackend{{Name: "ollama", Kind: "ollama", Reachable: true}},
		},
		{
			StatusFrameSchemaVersion: statusFrameSchemaVersion,
			ContractVersion:          "0.1",
			CapturedAtISO:            "2026-06-30T15:00:01Z",
			Backends:                 []statusBackend{{Name: "ollama", Kind: "ollama", Reachable: false}},
		},
	}
	var writes int
	writeStatusSnapshotWatch = func(ctx context.Context, cmd *cobra.Command, jsonFlag bool) (statusSnapshot, error) {
		index := writes
		if index >= len(snapshots) {
			index = len(snapshots) - 1
		}
		writes++
		snapshot := snapshots[index]
		return snapshot, writeData(cmd, jsonFlag, snapshot, func() error { return nil })
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &cancelAfterLinesWriter{after: 3, cancel: cancel}
	var stderr bytes.Buffer
	cmd := newRootCommand()
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"status", "--json", "--watch", "--events", "--interval", "1ms"})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("status watch events error = %v stderr=%s stdout=%s", err, stderr.String(), out.String())
		}
	case <-time.After(time.Second):
		cancel()
		t.Fatal("status watch events did not stop after context cancellation")
	}

	lines := assertCompleteJSONLines(t, out.String(), 3)
	var batchEnv struct {
		OK   bool             `json:"ok"`
		Data statusEventBatch `json:"data"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &batchEnv); err != nil {
		t.Fatalf("unmarshal status event batch: %v\n%s", err, lines[2])
	}
	if !batchEnv.OK || batchEnv.Data.EventSchemaVersion != statusEventSchemaVersion || len(batchEnv.Data.Events) != 1 {
		t.Fatalf("status event batch envelope = %#v", batchEnv)
	}
	if got := batchEnv.Data.Events[0]; got.Kind != "backend_reachability_changed" || got.Subject != "ollama" || got.After != "unreachable" {
		t.Fatalf("status event = %#v", got)
	}
}

func TestStatusWatchExitsCleanlyOnSignalCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		signal os.Signal
	}{
		{name: "sigint", signal: os.Interrupt},
		{name: "sigterm", signal: syscall.SIGTERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousNotify := statusSignalNotifyContext
			previousWriter := writeStatusSnapshotWatch
			t.Cleanup(func() {
				statusSignalNotifyContext = previousNotify
				writeStatusSnapshotWatch = previousWriter
			})

			signalCh := make(chan os.Signal, 1)
			var registered bool
			statusSignalNotifyContext = func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
				if signalListContains(signals, os.Interrupt) && signalListContains(signals, syscall.SIGTERM) {
					registered = true
				}
				ctx, cancel := context.WithCancel(parent)
				go func() {
					select {
					case sig := <-signalCh:
						if sig == tc.signal {
							cancel()
						}
					case <-parent.Done():
						cancel()
					}
				}()
				return ctx, cancel
			}
			writeStatusSnapshotWatch = func(ctx context.Context, cmd *cobra.Command, jsonFlag bool) (statusSnapshot, error) {
				snapshot := statusSnapshot{
					StatusFrameSchemaVersion: statusFrameSchemaVersion,
					ContractVersion:          "0.1",
					CapturedAtISO:            "2026-06-30T15:00:00Z",
				}
				return snapshot, writeData(cmd, jsonFlag, snapshot, func() error { return nil })
			}

			var triggerOnce sync.Once
			out := &cancelAfterLinesWriter{after: 1, cancel: func() {
				triggerOnce.Do(func() { signalCh <- tc.signal })
			}}
			var stderr bytes.Buffer
			cmd := newRootCommand()
			cmd.SetOut(out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"status", "--json", "--watch", "--interval", "1ms"})

			done := make(chan error, 1)
			go func() {
				done <- cmd.Execute()
			}()

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("status watch signal error = %v stderr=%s stdout=%s", err, stderr.String(), out.String())
				}
			case <-time.After(time.Second):
				t.Fatal("status watch did not stop after signal cancellation")
			}
			if !registered {
				t.Fatal("status watch did not register SIGINT and SIGTERM cancellation")
			}
			assertCompleteJSONLines(t, out.String(), 1)
		})
	}
}

func TestStatusWatchRejectsNonPositiveInterval(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeTempConfig(t))
	stdout, _, err := executeForTest("status", "--json", "--watch", "--interval", "0s")
	if err == nil {
		t.Fatalf("status watch accepted zero interval: %s", stdout)
	}
	if !strings.Contains(stdout, `"code":"E_INVALID_ARG"`) {
		t.Fatalf("status watch interval error = %s", stdout)
	}
}

func TestStatusEventsRepresentBackendReachabilityAndRouteSelection(t *testing.T) {
	fallbackIndex := 0
	before := statusSnapshot{
		CapturedAtISO: "2026-06-30T15:00:00Z",
		Backends: []statusBackend{
			{Name: "llamacpp", Kind: "llama.cpp", Reachable: true},
			{Name: "ollama", Kind: "ollama", Reachable: true},
		},
		Routes: []statusRoute{{
			Task:     "code",
			Decision: inferctl.RouteDecision{SelectedBackend: "llamacpp", SelectedModel: "primary.gguf", Ready: true},
		}},
	}
	errText := "backend_unreachable"
	after := statusSnapshot{
		CapturedAtISO: "2026-06-30T15:00:01Z",
		Backends: []statusBackend{
			{Name: "llamacpp", Kind: "llama.cpp", Reachable: false, Error: &errText},
			{Name: "ollama", Kind: "ollama", Reachable: true},
		},
		Routes: []statusRoute{{
			Task: "code",
			Decision: inferctl.RouteDecision{
				SelectedBackend: "ollama",
				SelectedModel:   "fallback:8b",
				IsFallback:      true,
				FallbackIndex:   &fallbackIndex,
				Ready:           true,
			},
		}},
	}

	events := diffStatusSnapshots(before, after)
	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3: %#v", len(events), events)
	}
	wantKinds := []string{"selected_route_changed", "fallback_status_changed", "backend_reachability_changed"}
	for i, want := range wantKinds {
		if events[i].Sequence != i+1 || events[i].Kind != want || events[i].Severity != "high" {
			t.Fatalf("event[%d] = %#v want kind=%s", i, events[i], want)
		}
	}
	if events[0].Subject != "code" || events[0].Before != "llamacpp/primary.gguf" || events[0].After != "ollama/fallback:8b" {
		t.Fatalf("route event = %#v", events[0])
	}
	if events[1].Subject != "code" || events[1].Before != false || events[1].After != true {
		t.Fatalf("fallback event = %#v", events[1])
	}
	if events[2].Subject != "llamacpp" || events[2].Before != "reachable" || events[2].After != "unreachable:backend_unreachable" {
		t.Fatalf("backend event = %#v", events[2])
	}
}

func TestStatusEventsUseSharedClassifierSemantics(t *testing.T) {
	beforeStatus := statusSnapshot{
		Backends: []statusBackend{{Name: "ollama", Kind: "ollama", Reachable: true}},
		Routes: []statusRoute{{
			Task:     "code",
			Decision: inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "primary:70b", Ready: true},
		}},
		Warnings:          []envelope.Warning{{Code: "W_OLD", Message: "old warning"}},
		RecommendedAction: &inferctl.RecommendedAction{Command: "inferctl model primary:70b --json"},
	}
	afterStatus := statusSnapshot{
		Backends: []statusBackend{{Name: "ollama", Kind: "ollama", Reachable: false}},
		Routes: []statusRoute{{
			Task:     "code",
			Decision: inferctl.RouteDecision{SelectedBackend: "ollama", SelectedModel: "fallback:8b", IsFallback: true, Ready: false},
		}},
		Warnings:          []envelope.Warning{{Code: "W_NEW", Message: "new warning"}},
		RecommendedAction: &inferctl.RecommendedAction{Command: "inferctl doctor --json"},
	}

	events := diffStatusSnapshots(beforeStatus, afterStatus)
	changes := append(classifyControlPlaneChanges(statusGlobalSnapshot(beforeStatus), statusGlobalSnapshot(afterStatus)), classifyControlPlaneChanges(statusRouteSnapshot(beforeStatus, "code"), statusRouteSnapshot(afterStatus, "code"))...)
	rankChanges(changes)
	if len(events) != len(changes) {
		t.Fatalf("event/change length mismatch events=%#v changes=%#v", events, changes)
	}
	for i := range changes {
		if events[i].Kind != changes[i].Type+"_changed" || events[i].Severity != changes[i].Severity || events[i].Summary != changes[i].Explanation {
			t.Fatalf("event[%d]=%#v change=%#v", i, events[i], changes[i])
		}
	}
}

func TestStatusEventBatchEnvelope(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")
	before := statusSnapshot{ContractVersion: "0.1", CapturedAtISO: "2026-06-30T15:00:00Z"}
	after := statusSnapshot{ContractVersion: "0.1", CapturedAtISO: "2026-06-30T15:00:01Z"}
	events := []statusEvent{{
		Sequence: 1,
		Kind:     "backend_reachability_changed",
		Subject:  "ollama",
		Severity: "medium",
		Summary:  "backend ollama became reachable",
		Before:   "unreachable",
		After:    "reachable",
	}}
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := writeStatusEventBatch(cmd.Command, true, before, after, events); err != nil {
		t.Fatalf("write event batch error = %v stderr=%s", err, stderr.String())
	}
	var env struct {
		OK   bool             `json:"ok"`
		Data statusEventBatch `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal event batch: %v\n%s", err, stdout.String())
	}
	if !env.OK || env.Data.EventSchemaVersion != statusEventSchemaVersion || len(env.Data.Events) != 1 {
		t.Fatalf("event batch envelope = %#v", env)
	}
	assertJSONSubsetGolden(t, "status_event_batch.golden.json", env.Data)
}

func TestStatusEventsRequireWatch(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeTempConfig(t))
	stdout, _, err := executeForTest("status", "--json", "--events")
	if err == nil {
		t.Fatalf("status accepted --events without --watch: %s", stdout)
	}
	if !strings.Contains(stdout, `"code":"E_INVALID_ARG"`) || !strings.Contains(stdout, `--watch`) {
		t.Fatalf("status --events error = %s", stdout)
	}
}

type cancelAfterLinesWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	lines  int
	after  int
	cancel context.CancelFunc
}

func (w *cancelAfterLinesWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	w.lines += strings.Count(string(p), "\n")
	if w.lines >= w.after {
		w.cancel()
	}
	return n, err
}

func (w *cancelAfterLinesWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func assertCompleteJSONLines(t *testing.T, output string, minLines int) []string {
	t.Helper()
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("status watch output ended with a partial JSON record: %q", output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < minLines {
		t.Fatalf("status watch emitted %d envelopes, want at least %d: %q", len(lines), minLines, output)
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("status watch emitted invalid JSON record: %q", line)
		}
	}
	return lines
}

func signalListContains(signals []os.Signal, want os.Signal) bool {
	for _, signal := range signals {
		if signal == want {
			return true
		}
	}
	return false
}

func hasStatusBackendKind(backends []statusBackend, kind string) bool {
	for _, backend := range backends {
		if backend.Kind == kind {
			return true
		}
	}
	return false
}

func normalizeStatusForGolden(status *statusSnapshot) {
	for i := range status.Backends {
		switch status.Backends[i].Name {
		case "llamacpp":
			status.Backends[i].BaseURL = "http://127.0.0.1:8090"
		case "ollama":
			status.Backends[i].BaseURL = "http://127.0.0.1:11434"
		}
	}
}

func statusDataHash(t *testing.T, stdout string) string {
	t.Helper()
	var env struct {
		Meta struct {
			DataHash string `json:"data_hash"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal status envelope: %v\n%s", err, stdout)
	}
	return env.Meta.DataHash
}

func writeStatusAllKindsConfig(t *testing.T, ollamaURL, llamaURL, compatURL, lmstudioURL, mlxURL string) string {
	t.Helper()
	body := `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.ollama]
kind = "ollama"
base_url = "` + ollamaURL + `"
default = true

[backends.llamacpp]
kind = "llama.cpp"
base_url = "` + llamaURL + `"
default = false

[backends.openai_compat]
kind = "openai_compat"
base_url = "` + compatURL + `"
default = false

[backends.lmstudio]
kind = "lmstudio"
base_url = "` + lmstudioURL + `"
default = false

[backends.mlx]
kind = "mlx"
base_url = "` + mlxURL + `"
default = false

[routing.code]
model = "ollama-model"
backend = "ollama"
fallback = []
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
