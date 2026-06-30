package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigShowJSON(t *testing.T) {
	path := writeTempConfig(t)
	t.Setenv("INFERCTL_CONFIG", path)
	t.Setenv("INFERCTL_DEFAULT_BACKEND", "llamacpp_32b")

	stdout, stderr, err := executeForTest("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show error = %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			SourcePaths struct {
				Selected   string `json:"selected"`
				SelectedBy string `json:"selected_by"`
			} `json:"source_paths"`
			EffectiveConfig map[string]any    `json:"effective_config"`
			Provenance      map[string]string `json:"provenance"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json unmarshal: %v\n%s", err, stdout)
	}
	if !env.OK {
		t.Fatalf("ok=false: %s", stdout)
	}
	if env.Data.SourcePaths.Selected != path || env.Data.SourcePaths.SelectedBy != "env" {
		t.Fatalf("source_paths = %#v", env.Data.SourcePaths)
	}
	if env.Data.Provenance["backends.ollama.default"] != "env" ||
		env.Data.Provenance["backends.llamacpp_32b.default"] != "env" {
		t.Fatalf("default backend provenance = %#v", env.Data.Provenance)
	}
	if _, ok := env.Data.Provenance["backends.ollama.auth_header_value"]; ok {
		t.Fatalf("secret provenance should be omitted: %#v", env.Data.Provenance)
	}
	backends, ok := env.Data.EffectiveConfig["backends"].(map[string]any)
	if !ok {
		t.Fatalf("backends missing from effective_config: %#v", env.Data.EffectiveConfig)
	}
	ollama, ok := backends["ollama"].(map[string]any)
	if !ok {
		t.Fatalf("ollama backend missing from effective_config: %#v", backends)
	}
	if got := ollama["base_url"]; got != "http://127.0.0.1:11434" {
		t.Fatalf("ollama base_url = %#v", got)
	}
	if _, ok := ollama["auth_header_value"]; ok {
		t.Fatalf("auth_header_value should be omitted from config show output: %#v", ollama)
	}
	if strings.Contains(stdout, "fixture-auth-value") {
		t.Fatalf("config show leaked auth header value:\n%s", stdout)
	}
}

func TestConfigShowKeyAndSection(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeTempConfig(t))
	stdout, _, err := executeForTest("config", "show", "--key", "profile.mode", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var keyEnv struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &keyEnv); err != nil {
		t.Fatal(err)
	}
	if keyEnv.Data["key"] != "profile.mode" || keyEnv.Data["value"] != "warn" {
		t.Fatalf("key data = %#v", keyEnv.Data)
	}

	stdout, _, err = executeForTest("config", "show", "--section", "routing", "--no-provenance", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var sectionEnv struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &sectionEnv); err != nil {
		t.Fatal(err)
	}
	if _, ok := sectionEnv.Data["provenance"]; ok {
		t.Fatalf("provenance should be omitted: %#v", sectionEnv.Data)
	}

	stdout, _, err = executeForTest("config", "show", "--key", "backends.ollama.auth_header_value", "--json")
	if err == nil {
		t.Fatalf("expected redacted key lookup to fail, stdout=%s", stdout)
	}
	if strings.Contains(stdout, "fixture-auth-value") {
		t.Fatalf("redacted key lookup leaked auth header value:\n%s", stdout)
	}
}

func TestConfigShowMissingConfigError(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	stdout, _, err := executeForTest("config", "show", "--json")
	if err == nil {
		t.Fatal("expected error")
	}
	var env struct {
		OK     bool `json:"ok"`
		Errors []struct {
			Code       string `json:"code"`
			ExitCode   int    `json:"exit_code"`
			DidYouMean string `json:"did_you_mean"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.OK || env.Errors[0].Code != "E_CONFIG_MISSING" || env.Errors[0].ExitCode != 3 {
		t.Fatalf("envelope = %#v", env)
	}
}

func executeForTest(args ...string) (string, string, error) {
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func writeTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(workedExampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const workedExampleConfig = `[meta]
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
auth_header_name = "Authorization"
auth_header_value = "fixture-auth-value"

[backends.llamacpp_32b]
kind = "llama.cpp"
base_url = "http://127.0.0.1:8090"
default = false

[routing.code]
model = "qwen3-coder:30b-a3b-q4_K_M"
backend = "llamacpp_32b"
fallback = ["qwen3-coder:8b", "qwen3:8b"]
`
