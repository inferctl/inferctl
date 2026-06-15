package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func TestTriageCleanMachineReturnsZeroItems(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("triage", "--json")
	if err != nil {
		t.Fatalf("triage error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK   bool         `json:"ok"`
		Data triageReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || env.Data.Summary.Total != 0 || len(env.Data.Items) != 0 {
		t.Fatalf("clean triage = %#v", env.Data)
	}
	assertJSONSubsetGolden(t, "triage.clean.golden.json", map[string]any{"items": env.Data.Items})
}

func TestTriageRanksConfigErrorsAboveBackendWarnings(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Unreachable: true})
	defer server.Close()
	cfg := stringsReplace(writeDoctorConfigBody(doctorConfigOptions{OllamaURL: server.URL}), "schema_version = \"0.1\"\n", "")
	path := writeConfig(t, cfg)
	t.Setenv("INFERCTL_CONFIG", path)
	t.Setenv("INFERCTL_TEST_DETERMINISTIC", "1")

	first, _, err := executeForTest("triage", "--json")
	if err != nil {
		t.Fatalf("triage should exit zero: %v stdout=%s", err, first)
	}
	second, _, err := executeForTest("triage", "--json")
	if err != nil {
		t.Fatalf("second triage should exit zero: %v stdout=%s", err, second)
	}
	if first != second {
		t.Fatalf("triage output is not deterministic\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	var env struct {
		Data triageReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(first), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data.Items) < 2 {
		t.Fatalf("expected config error and backend warning: %#v", env.Data.Items)
	}
	if env.Data.Items[0].Severity != "error" || env.Data.Items[0].Subject != "meta.schema_version" {
		t.Fatalf("first item should be config error: %#v", env.Data.Items)
	}
	if !hasTriageCode(env.Data.Items, "W_BACKEND_UNREACHABLE") {
		t.Fatalf("missing backend warning: %#v", env.Data.Items)
	}
	assertJSONSubsetGolden(t, "triage.errors.golden.json", map[string]any{"items": env.Data.Items})
}

func TestTriageFiltersByBackendSeverityAndLimit(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Unreachable: true})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("triage", "--backend", "ollama", "--severity", "warning", "--limit", "1", "--json")
	if err != nil {
		t.Fatalf("triage error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data triageReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Data.Items) != 1 || env.Data.Items[0].Severity != "warning" || env.Data.Items[0].Backend == nil || *env.Data.Items[0].Backend != "ollama" {
		t.Fatalf("filtered items = %#v", env.Data.Items)
	}
}

func TestTriageAcceptsPriorEnvelopeInputFile(t *testing.T) {
	input := `{
  "ok": false,
  "data": {"findings": [{"severity": "error", "key": "profile.mode", "message": "bad mode", "details": {}}]},
  "warnings": [{"code": "W_MODEL_NOT_INSTALLED", "message": "model missing", "details": {"task": "code", "model": "qwen3:8b", "backend": "ollama"}}],
  "commands": [{"label": "Inspect model", "command": "inferctl model qwen3:8b --json", "rationale": "from input"}],
  "errors": []
}`
	path := filepath.Join(t.TempDir(), "envelope.json")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("triage", "--input-file", path, "--json")
	if err != nil {
		t.Fatalf("triage input-file error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data     triageReport `json:"data"`
		Commands []struct {
			Command string `json:"command"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Summary.Total != 2 || env.Data.Items[0].Subject != "profile.mode" {
		t.Fatalf("input-file items = %#v", env.Data.Items)
	}
	if len(env.Commands) != 2 {
		t.Fatalf("commands should be deduplicated and retained: %#v", env.Commands)
	}
}

func writeDoctorConfigBody(opts doctorConfigOptions) string {
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
	return `[meta]
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
}

func hasTriageCode(items []triageItem, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
