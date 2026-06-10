package config

import (
	"errors"
	"testing"
)

func TestLoadWorkedExample(t *testing.T) {
	source := testSource("/tmp/config.toml")
	got, err := LoadBytes([]byte(workedExampleTOML), source, nil, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadBytes() error = %v", err)
	}
	if got.Config.Meta.SchemaVersion != "0.1" {
		t.Fatalf("schema version = %q", got.Config.Meta.SchemaVersion)
	}
	if got.Config.Profile.Name != "default_local_workstation" {
		t.Fatalf("profile name = %q", got.Config.Profile.Name)
	}
	if got.Config.Backends["ollama"].TimeoutMS != 2000 {
		t.Fatalf("ollama timeout default = %d", got.Config.Backends["ollama"].TimeoutMS)
	}
	if got.Config.Routing["chat"].Fallback == nil {
		t.Fatalf("routing.chat.fallback should default to empty slice, got nil")
	}
	if got.Provenance["profile.mode"] != ProvenanceFile {
		t.Fatalf("profile.mode provenance = %q", got.Provenance["profile.mode"])
	}
	if got.Provenance["backends.ollama.timeout_ms"] != ProvenanceDefault {
		t.Fatalf("backends.ollama.timeout_ms provenance = %q", got.Provenance["backends.ollama.timeout_ms"])
	}
}

func TestMalformedTOMLReturnsLineColumn(t *testing.T) {
	_, err := LoadBytes([]byte("[meta]\nschema_version = \"0.1\"\n[profile]\nmode = [\n"), testSource("/tmp/bad.toml"), nil, LoadOptions{})
	if err == nil {
		t.Fatal("LoadBytes() error = nil")
	}
	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error type = %T", err)
	}
	if loadErr.Code != "E_CONFIG_INVALID" {
		t.Fatalf("code = %s", loadErr.Code)
	}
	if loadErr.Line == nil || loadErr.Column == nil {
		t.Fatalf("line/column not populated: %#v", loadErr)
	}
}

func TestDefaultBackendEnvMutationProvenance(t *testing.T) {
	got, err := LoadBytes([]byte(workedExampleTOML), testSource("/tmp/config.toml"), map[string]string{
		"INFERCTL_DEFAULT_BACKEND": "llamacpp_32b",
	}, LoadOptions{})
	if err != nil {
		t.Fatalf("LoadBytes() error = %v", err)
	}
	if got.Config.Backends["ollama"].Default {
		t.Fatalf("ollama default should be false after env mutation")
	}
	if !got.Config.Backends["llamacpp_32b"].Default {
		t.Fatalf("llamacpp_32b default should be true after env mutation")
	}
	if got.Provenance["backends.ollama.default"] != ProvenanceEnv {
		t.Fatalf("ollama default provenance = %q", got.Provenance["backends.ollama.default"])
	}
	if got.Provenance["backends.llamacpp_32b.default"] != ProvenanceEnv {
		t.Fatalf("llamacpp default provenance = %q", got.Provenance["backends.llamacpp_32b.default"])
	}
}

func TestPositionMapSmoke(t *testing.T) {
	positions, err := KeyPositions([]byte("[profile]\nname = \"x\"\nmax_context_tokens = 8192\n"))
	if err != nil {
		t.Fatalf("KeyPositions() error = %v", err)
	}
	pos, ok := positions["profile.max_context_tokens"]
	if !ok {
		t.Fatalf("missing position for profile.max_context_tokens: %#v", positions)
	}
	if pos.Line != 3 || pos.Column != 1 {
		t.Fatalf("position = line %d column %d, want line 3 column 1", pos.Line, pos.Column)
	}
}

func TestConfigExplainBypassesLoader(t *testing.T) {
	if !ConfigExplanationAvailableWithoutFile() {
		t.Fatal("config explain should remain available without loading a config file")
	}
}

func testSource(path string) SourcePaths {
	by := "env"
	return SourcePaths{Selected: &path, Searched: []string{path}, SelectedBy: &by}
}

const workedExampleTOML = `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:11434"
default = true

[backends.llamacpp_32b]
kind = "llama.cpp"
base_url = "http://127.0.0.1:8090"
default = false

[backends.lmstudio_local]
kind = "openai_compat"
base_url = "http://127.0.0.1:1234"
default = false

[routing.code]
model = "qwen3-coder:30b-a3b-q4_K_M"
backend = "llamacpp_32b"
fallback = ["qwen3-coder:8b", "qwen3:8b"]

[routing.chat]
model = "qwen3:8b"
backend = "ollama"
fallback = []

[routing.summarize]
model = "qwen3:8b"
backend = "ollama"
fallback = []
num_ctx = 4096
`
