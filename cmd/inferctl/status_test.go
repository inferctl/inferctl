package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/inferctl/inferctl/internal/testserver"
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
	normalizeStatusForGolden(&env.Data)
	assertJSONSubsetGolden(t, "status.golden.json", env.Data)
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

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("status watch emitted %d envelopes, want at least 2: %q", len(lines), out.String())
	}
	for _, line := range lines {
		var env struct {
			OK   bool           `json:"ok"`
			Data statusSnapshot `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("unmarshal watch envelope: %v\n%s", err, line)
		}
		if !env.OK || env.Data.StatusSchemaVersion != statusSchemaVersion {
			t.Fatalf("unexpected watch envelope: %#v", env)
		}
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
