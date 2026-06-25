package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownVerbJSON(t *testing.T) {
	stdout, stderr, err := executeForTest("doctr", "--json")
	if err == nil {
		t.Fatal("expected unknown verb error")
	}
	var env struct {
		ToolVersion string `json:"tool_version"`
		Errors      []struct {
			Code       string `json:"code"`
			DidYouMean string `json:"did_you_mean"`
			ExitCode   int    `json:"exit_code"`
			Retryable  bool   `json:"retryable"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(env.Errors) != 1 || env.Errors[0].Code != "E_UNKNOWN_VERB" || env.Errors[0].DidYouMean != "inferctl doctor" {
		t.Fatalf("unexpected envelope: %#v", env.Errors)
	}
	if env.ToolVersion != resolvedToolVersion() {
		t.Fatalf("tool_version = %q, want %q", env.ToolVersion, resolvedToolVersion())
	}
	if !strings.Contains(stderr, "error: unknown verb 'doctr'") ||
		!strings.Contains(stderr, "try: inferctl doctor") ||
		!strings.Contains(stderr, "exit: 1 (user_input_error, retryable: false)") {
		t.Fatalf("stderr missing mirrored JSON diagnostic:\n%s", stderr)
	}
}

func TestBareInvocationPrintsHelp(t *testing.T) {
	stdout, stderr, err := executeForTest()
	if err != nil {
		t.Fatalf("bare invocation should print help successfully: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") ||
		!strings.Contains(stdout, "inferctl [command]") ||
		!strings.Contains(stdout, "capabilities") ||
		!strings.Contains(stdout, "Exit Codes:") ||
		!strings.Contains(stdout, "Agent/Automation:") ||
		!strings.Contains(stdout, "Machine contract: inferctl capabilities --json") {
		t.Fatalf("bare invocation did not print root help:\n%s", stdout)
	}
	if stderr != "" {
		t.Fatalf("bare invocation should not write stderr, got:\n%s", stderr)
	}
}

func TestHelpIncludesAgentFooter(t *testing.T) {
	for _, args := range [][]string{
		{"--help"},
		{"doctor", "--help"},
		{"config", "validate", "--help"},
	} {
		stdout, stderr, err := executeForTest(args...)
		if err != nil {
			t.Fatalf("%v help failed: %v stderr=%s", args, err, stderr)
		}
		for _, want := range []string{
			"Exit Codes:",
			"See: inferctl capabilities --json",
			"Agent/Automation:",
			"Machine contract: inferctl capabilities --json",
			"JSON envelope: add --json or set INFERCTL_FORMAT=json",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%v help missing %q:\n%s", args, want, stdout)
			}
		}
	}
}

func TestNoVerbJSONReturnsStructuredError(t *testing.T) {
	stdout, stderr, err := executeForTest("--json")
	if err == nil {
		t.Fatal("expected no-verb JSON error")
	}
	var env struct {
		OK     bool `json:"ok"`
		Errors []struct {
			Code       string `json:"code"`
			DidYouMean string `json:"did_you_mean"`
			ExitCode   int    `json:"exit_code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "E_MISSING_ARG" || env.Errors[0].DidYouMean != "inferctl --help" || env.Errors[0].ExitCode != 1 {
		t.Fatalf("unexpected no-verb envelope: %#v", env)
	}
	if !strings.Contains(stderr, "error: no verb specified") ||
		!strings.Contains(stderr, "try: inferctl --help") {
		t.Fatalf("stderr missing no-verb diagnostic:\n%s", stderr)
	}
}

func TestCapabilitiesJSONUsesResolvedToolVersion(t *testing.T) {
	stdout, _, err := executeForTest("capabilities", "--json")
	if err != nil {
		t.Fatalf("capabilities error = %v stdout=%s", err, stdout)
	}
	var env struct {
		ToolVersion string `json:"tool_version"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.ToolVersion != resolvedToolVersion() {
		t.Fatalf("tool_version = %q, want %q", env.ToolVersion, resolvedToolVersion())
	}
}

func TestRenamedVerbJSON(t *testing.T) {
	stdout, _, err := executeForTest("explain", "code", "--json")
	if err == nil {
		t.Fatal("expected renamed explain error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "inferctl route code --json --explain") {
		t.Fatalf("unexpected explain redirect: %s", stdout)
	}

	stdout, _, err = executeForTest("capabilities", "qwen3:8b", "--json")
	if err == nil {
		t.Fatal("expected renamed capabilities error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "inferctl model qwen3:8b --json") {
		t.Fatalf("unexpected capabilities redirect: %s", stdout)
	}

	stdout, _, err = executeForTest("config", "valid", "--json")
	if err == nil {
		t.Fatal("expected renamed config valid error")
	}
	if !strings.Contains(stdout, "E_VERB_RENAMED") || !strings.Contains(stdout, "inferctl config validate --json") {
		t.Fatalf("unexpected config valid redirect: %s", stdout)
	}
}

func TestUnknownFlagJSON(t *testing.T) {
	stdout, _, err := executeForTest("doctor", "--bogus", "--json")
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	if !strings.Contains(stdout, "E_UNKNOWN_FLAG") || !strings.Contains(stdout, "inferctl doctor --help") {
		t.Fatalf("unexpected unknown flag envelope: %s", stdout)
	}

	stdout, _, err = executeForTest("doctor", "--json", "--jsno")
	if err == nil {
		t.Fatal("expected typo'd flag error")
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
	if len(env.Errors) != 1 || env.Errors[0].Code != "E_UNKNOWN_FLAG" || env.Errors[0].DidYouMean != "inferctl doctor --json" {
		t.Fatalf("unexpected typo'd flag envelope: %#v", env.Errors)
	}
	if env.Errors[0].Details["nearest"] != "--json" {
		t.Fatalf("nearest flag missing: %#v", env.Errors[0].Details)
	}
}

func TestHumanErrorRenderingIncludesCatalogMetadata(t *testing.T) {
	_, stderr, err := executeForTest("doctr")
	if err == nil {
		t.Fatal("expected human unknown verb error")
	}
	if !strings.Contains(stderr, "error: unknown verb 'doctr'") ||
		!strings.Contains(stderr, "try: inferctl doctor") ||
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
	if !strings.Contains(stdout, `"did_you_mean":"inferctl config explain --key meta.schema_version --json"`) {
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
