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

func TestConfigShowTypedCredentialRedactsLiteralValue(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, typedCredentialConfig))
	stdout, _, err := executeForTest("config", "show", "--json")
	if err != nil {
		t.Fatalf("config show error = %v stdout=%s", err, stdout)
	}
	if strings.Contains(stdout, "Bearer typed-fixture") {
		t.Fatalf("config show leaked typed credential: %s", stdout)
	}
	var env struct {
		Data struct {
			EffectiveConfig map[string]any    `json:"effective_config"`
			Provenance      map[string]string `json:"provenance"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	backends := env.Data.EffectiveConfig["backends"].(map[string]any)
	remote := backends["remote"].(map[string]any)
	credential := remote["credential"].(map[string]any)
	if credential["version"] != "v1" || credential["source"] != "literal" {
		t.Fatalf("public credential reference = %#v", credential)
	}
	if _, ok := credential["literal_value"]; ok {
		t.Fatalf("public credential reference leaked literal value: %#v", credential)
	}
	if _, ok := env.Data.Provenance["backends.remote.credential.literal_value"]; ok {
		t.Fatalf("credential literal provenance leaked: %#v", env.Data.Provenance)
	}
	assertJSONSubsetGolden(t, "config_typed_credential.golden.json", map[string]any{
		"credential": credential,
		"provenance": map[string]string{
			"backends.remote.credential.source":  env.Data.Provenance["backends.remote.credential.source"],
			"backends.remote.credential.version": env.Data.Provenance["backends.remote.credential.version"],
		},
	})

	stdout, _, err = executeForTest("config", "show", "--key", "backends.remote.credential.literal_value", "--json")
	if err == nil || strings.Contains(stdout, "Bearer typed-fixture") {
		t.Fatalf("typed credential key lookup should be redacted: err=%v stdout=%s", err, stdout)
	}
}

func TestConfigFingerprintDriftStates(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, typedCredentialConfig))
	stdout, _, err := executeForTest("config", "fingerprint", "--json")
	if err != nil {
		t.Fatalf("fingerprint error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data struct {
			Fingerprint struct {
				Value string `json:"value"`
			} `json:"fingerprint"`
			Drift struct {
				Status string `json:"status"`
			} `json:"drift"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(env.Data.Fingerprint.Value, "v1:sha256:") || env.Data.Drift.Status != "not_checked" {
		t.Fatalf("fingerprint result = %s", stdout)
	}
	stdout, _, err = executeForTest("config", "fingerprint", "--expect", env.Data.Fingerprint.Value, "--json")
	if err != nil || !strings.Contains(stdout, `"status":"match"`) {
		t.Fatalf("match result err=%v stdout=%s", err, stdout)
	}
	stdout, _, err = executeForTest("config", "fingerprint", "--expect", "v2:sha256:x", "--json")
	if err != nil || !strings.Contains(stdout, `"status":"unsupported_format"`) {
		t.Fatalf("format result err=%v stdout=%s", err, stdout)
	}
}

func TestConfigShowMissingConfigError(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
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

const typedCredentialConfig = `[meta]
schema_version = "0.1"

[profile]
name = "typed_credential"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.remote]
kind = "openai_compat"
base_url = "http://127.0.0.1:8080"
default = true
auth_header_name = "Authorization"

[backends.remote.credential]
version = "v1"
source = "literal"
literal_value = "Bearer typed-fixture"

[routing.code]
model = "typed-model"
backend = "remote"
fallback = []
`
