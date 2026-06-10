package main

import (
	"encoding/json"
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
