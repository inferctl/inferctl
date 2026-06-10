package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func TestDoctorCleanReport(t *testing.T) {
	ollamaServer := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b", SizeBytes: 123}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b", VRAMBytes: 456}},
	})
	defer ollamaServer.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: ollamaServer.URL}))

	stdout, _, err := executeForTest("doctor", "--json")
	if err != nil {
		t.Fatalf("doctor error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK       bool         `json:"ok"`
		Data     doctorReport `json:"data"`
		Warnings []struct {
			Code string `json:"code"`
		} `json:"warnings"`
		Commands []struct {
			Command string `json:"command"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || env.Data.Summary.BackendsReachable != 1 || env.Data.Summary.ModelsLoadedTotal != 1 {
		t.Fatalf("doctor summary = %#v ok=%v", env.Data.Summary, env.OK)
	}
	if env.Data.RecommendedAction != nil || len(env.Warnings) != 0 || len(env.Commands) != 0 {
		t.Fatalf("unexpected diagnostics: action=%#v warnings=%#v commands=%#v", env.Data.RecommendedAction, env.Warnings, env.Commands)
	}
}

func TestDoctorDegradedBackendStillExitsZero(t *testing.T) {
	up := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b"}},
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

	stdout, _, err := executeForTest("doctor", "--json")
	if err != nil {
		t.Fatalf("doctor should exit 0 for backend failures: %v stdout=%s", err, stdout)
	}
	var env struct {
		Data     doctorReport `json:"data"`
		Warnings []struct {
			Code string `json:"code"`
		} `json:"warnings"`
		Commands []struct {
			Command            string  `json:"command"`
			AvailableInVersion *string `json:"available_in_version"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Summary.BackendsReachable != 1 || env.Data.RecommendedAction == nil {
		t.Fatalf("doctor degraded data = %#v action=%#v", env.Data.Summary, env.Data.RecommendedAction)
	}
	if len(env.Commands) < 2 || !strings.HasPrefix(env.Commands[0].Command, "infer backends --filter llamacpp") {
		t.Fatalf("commands not ranked as expected: %#v", env.Commands)
	}
	assertWarningCode(t, env.Warnings, "W_BACKEND_UNREACHABLE")
	assertWarningCode(t, env.Warnings, "W_FALLBACK_USED")
}

func TestDoctorFallbackAndFutureWarmupCommand(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "primary:70b",
		Fallback:  "fallback:8b",
	}))

	stdout, _, err := executeForTest("doctor", "--json")
	if err != nil {
		t.Fatalf("doctor error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data     doctorReport `json:"data"`
		Commands []struct {
			Command            string  `json:"command"`
			AvailableInVersion *string `json:"available_in_version"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data.Routes) != 1 || !env.Data.Routes[0].IsFallback || env.Data.Routes[0].Ready {
		t.Fatalf("route summary = %#v", env.Data.Routes)
	}
	foundFuture := false
	for _, command := range env.Commands {
		if strings.HasPrefix(command.Command, "infer warmup ") && command.AvailableInVersion != nil && *command.AvailableInVersion == "0.5" {
			foundFuture = true
		}
	}
	if !foundFuture {
		t.Fatalf("missing future warmup command: %#v", env.Commands)
	}
}

func TestDoctorNoBackendsErrors(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{NoBackends: true}))

	stdout, _, err := executeForTest("doctor", "--json")
	if err == nil {
		t.Fatal("expected no-backends error")
	}
	if !strings.Contains(stdout, "E_NO_BACKENDS_CONFIGURED") || !strings.Contains(stdout, "infer config explain") {
		t.Fatalf("unexpected no-backends envelope: %s", stdout)
	}
}

func assertWarningCode(t *testing.T, warnings []struct {
	Code string `json:"code"`
}, code string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == code {
			return
		}
	}
	t.Fatalf("missing warning %s in %#v", code, warnings)
}

type doctorConfigOptions struct {
	OllamaURL  string
	LlamaURL   string
	Primary    string
	Fallback   string
	NoBackends bool
}

func writeDoctorConfig(t *testing.T, opts doctorConfigOptions) string {
	t.Helper()
	primary := opts.Primary
	if primary == "" {
		primary = "qwen3:8b"
	}
	fallback := opts.Fallback
	fallbackLine := "fallback = []"
	if fallback != "" {
		fallbackLine = `fallback = ["` + fallback + `"]`
	}
	backends := ""
	routeBackend := "ollama"
	if !opts.NoBackends {
		backends = `
[backends.ollama]
kind = "ollama"
base_url = "` + opts.OllamaURL + `"
default = true
`
		if opts.LlamaURL != "" {
			backends += `
[backends.llamacpp]
kind = "llama.cpp"
base_url = "` + opts.LlamaURL + `"
default = false
`
			routeBackend = "llamacpp"
		}
	}
	body := `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"
vram_total_bytes_hint = 123456
` + backends + `
[routing.code]
model = "` + primary + `"
backend = "` + routeBackend + `"
` + fallbackLine + `
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
