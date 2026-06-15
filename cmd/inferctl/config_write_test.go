package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitPrintGolden(t *testing.T) {
	stdout, _, err := executeForTest("config", "init", "--print", "--json")
	if err != nil {
		t.Fatalf("config init --print error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data configMutationResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	assertJSONSubsetGolden(t, "config_init.print.golden.json", map[string]any{
		"written":      env.Data.Written,
		"dry_run":      env.Data.DryRun,
		"changed_keys": env.Data.ChangedKeys,
	})
	if !strings.Contains(env.Data.Preview, `[backends.ollama]`) {
		t.Fatalf("scaffold missing backend: %s", env.Data.Preview)
	}
}

func TestConfigSetPreservesCommentsOrderingAndDryRun(t *testing.T) {
	path := writeConfig(t, commentedConfig)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("config", "set", "profile.mode", "strict", "--path", path, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("config set dry-run error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data configMutationResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	assertJSONSubsetGolden(t, "config_set.change.golden.json", map[string]any{
		"written":      env.Data.Written,
		"dry_run":      env.Data.DryRun,
		"changed_keys": env.Data.ChangedKeys,
	})
	afterDryRun, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterDryRun) != string(before) {
		t.Fatal("dry-run changed config file")
	}
	if !strings.Contains(env.Data.Preview, `mode = "strict" # keep this comment`) {
		t.Fatalf("dry-run preview did not preserve inline comment:\n%s", env.Data.Preview)
	}

	stdout, _, err = executeForTest("config", "set", "profile.mode", "strict", "--path", path, "--json")
	if err != nil {
		t.Fatalf("config set error = %v stdout=%s", err, stdout)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(written)
	if !strings.Contains(got, "# profile comment\n[profile]\n") ||
		!strings.Contains(got, `mode = "strict" # keep this comment`) ||
		strings.Index(got, "[profile]") > strings.Index(got, "[backends.ollama]") {
		t.Fatalf("comments/order not preserved:\n%s", got)
	}
}

func TestConfigPatchFromStdinRedactsSecrets(t *testing.T) {
	path := writeConfig(t, commentedConfig)
	sensitiveValue := "fixture-" + "redaction-" + "value"
	patch := "[backends.ollama]\nauth_header_name = \"Authorization\"\nauth_header_value = \"" + sensitiveValue + "\"\n"
	stdout, _, err := executeForTestWithInput(patch, "config", "patch", "--from-stdin", "--path", path, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("config patch error = %v stdout=%s", err, stdout)
	}
	if strings.Contains(stdout, sensitiveValue) {
		t.Fatalf("sensitive value leaked in dry-run JSON: %s", stdout)
	}
	var env struct {
		Data configMutationResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	assertJSONSubsetGolden(t, "config_patch.stdin.golden.json", map[string]any{
		"written":      env.Data.Written,
		"dry_run":      env.Data.DryRun,
		"changed_keys": env.Data.ChangedKeys,
	})
	if !strings.Contains(env.Data.Preview, `auth_header_value = "<redacted>"`) {
		t.Fatalf("preview missing redaction:\n%s", env.Data.Preview)
	}
}

func TestConfigPatchRejectsNoopWithoutPartialWrite(t *testing.T) {
	path := writeConfig(t, commentedConfig)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("config", "patch", "[profile]", "--path", path, "--json")
	if err == nil {
		t.Fatal("expected empty table patch error")
	}
	if !strings.Contains(stdout, "E_CONFIG_PATCH_DELETE_UNSUPPORTED") {
		t.Fatalf("unexpected patch error: %s", stdout)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid patch changed config file")
	}
}

func TestConfigSetRejectsInvalidTypeWithoutPartialWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(commentedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("config", "set", "profile.max_context_tokens", "not-an-int", "--type", "int", "--path", path, "--json")
	if err == nil {
		t.Fatal("expected invalid type error")
	}
	if !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("unexpected set error: %s", stdout)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("invalid set changed config file")
	}
}

const commentedConfig = `[meta]
schema_version = "0.1"

# profile comment
[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn" # keep this comment

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:11434"
default = true

[routing.code]
model = "qwen3:8b"
backend = "ollama"
fallback = []
`
