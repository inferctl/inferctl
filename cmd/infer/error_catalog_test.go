package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownVerbJSON(t *testing.T) {
	stdout, _, err := executeForTest("doctr", "--json")
	if err == nil {
		t.Fatal("expected unknown verb error")
	}
	var env struct {
		Errors []struct {
			Code       string `json:"code"`
			DidYouMean string `json:"did_you_mean"`
			ExitCode   int    `json:"exit_code"`
			Retryable  bool   `json:"retryable"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(env.Errors) != 1 || env.Errors[0].Code != "E_UNKNOWN_VERB" || env.Errors[0].DidYouMean != "infer doctor" {
		t.Fatalf("unexpected envelope: %#v", env.Errors)
	}
}

func TestRenamedVerbJSON(t *testing.T) {
	stdout, _, err := executeForTest("explain", "code", "--json")
	if err == nil {
		t.Fatal("expected renamed explain error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "infer route code --json --explain") {
		t.Fatalf("unexpected explain redirect: %s", stdout)
	}

	stdout, _, err = executeForTest("capabilities", "qwen3:8b", "--json")
	if err == nil {
		t.Fatal("expected renamed capabilities error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "infer model qwen3:8b --json") {
		t.Fatalf("unexpected capabilities redirect: %s", stdout)
	}

	stdout, _, err = executeForTest("config", "valid", "--json")
	if err == nil {
		t.Fatal("expected renamed config valid error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "infer config validate --json") {
		t.Fatalf("unexpected config valid redirect: %s", stdout)
	}
}

func TestUnknownFlagJSON(t *testing.T) {
	stdout, _, err := executeForTest("doctor", "--bogus", "--json")
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	if !strings.Contains(stdout, "E_UNKNOWN_FLAG") || !strings.Contains(stdout, "infer doctor --help") {
		t.Fatalf("unexpected unknown flag envelope: %s", stdout)
	}
}

func TestHumanErrorRenderingIncludesCatalogMetadata(t *testing.T) {
	_, stderr, err := executeForTest("doctr")
	if err == nil {
		t.Fatal("expected human unknown verb error")
	}
	if !strings.Contains(stderr, "error: unknown verb 'doctr'") ||
		!strings.Contains(stderr, "try: infer doctor") ||
		!strings.Contains(stderr, "exit: 1 (user_input_error, retryable: false)") {
		t.Fatalf("stderr missing catalog metadata:\n%s", stderr)
	}
}

func TestInvalidEnumSuggestsNearestValue(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeTempConfig(t))
	stdout, _, err := executeForTest("route", "code", "--prefer", "defalt", "--json")
	if err == nil {
		t.Fatal("expected invalid enum error")
	}
	var env struct {
		Errors []struct {
			Code       string         `json:"code"`
			DidYouMean string         `json:"did_you_mean"`
			Details    map[string]any `json:"details"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(env.Errors) != 1 || env.Errors[0].Code != "E_INVALID_ARG" || env.Errors[0].DidYouMean != "--prefer=default" {
		t.Fatalf("unexpected invalid arg envelope: %#v", env.Errors)
	}
	if env.Errors[0].Details["nearest"] != "default" {
		t.Fatalf("nearest missing from details: %#v", env.Errors[0].Details)
	}
}

func TestValidationErrorSuggestsConfigExplainKey(t *testing.T) {
	cfg := stringsReplace(workedExampleConfig, "schema_version = \"0.1\"\n", "")
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(stdout, `"did_you_mean":"infer config explain --key meta.schema_version --json"`) {
		t.Fatalf("validation did_you_mean missing: %s", stdout)
	}
}

func TestExitCodeNamesCoverCapabilitiesCatalog(t *testing.T) {
	for code, name := range map[int]string{
		0: "success",
		1: "user_input_error",
		2: "safety_block",
		3: "tool_environment_error",
		4: "transient_failure",
		5: "conflict",
	} {
		if got := exitCodeName(code); got != name {
			t.Fatalf("exitCodeName(%d) = %q, want %q", code, got, name)
		}
	}
}

func assertJSONSubsetGolden(t *testing.T, name string, got any) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "contract", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	gotJSON = append(gotJSON, '\n')
	if string(gotJSON) != string(want) {
		t.Fatalf("%s mismatch\nwant:\n%s\ngot:\n%s", name, want, gotJSON)
	}
}
