package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/testserver"
	"github.com/inferctl/inferctl/pkg/inferctl"
)

func TestRoutePrimarySuccess(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("route", "code", "--json")
	if err != nil {
		t.Fatalf("route error = %v stdout=%s", err, stdout)
	}
	env := decodeRouteEnvelope(t, stdout)
	if env.Data.Decision.SelectedModel != "qwen3:8b" || env.Data.Decision.IsFallback || !env.Data.Decision.Ready {
		t.Fatalf("decision = %#v", env.Data.Decision)
	}
	if len(env.Data.Candidates) != 1 || env.Data.Input.Source != "none" {
		t.Fatalf("route data = %#v", env.Data)
	}
}

func TestRouteFallbackSuccess(t *testing.T) {
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

	stdout, _, err := executeForTest("route", "code", "--prompt", "hello world", "--json")
	if err != nil {
		t.Fatalf("route error = %v stdout=%s", err, stdout)
	}
	env := decodeRouteEnvelope(t, stdout)
	if !env.Data.Decision.IsFallback || env.Data.Decision.SelectedModel != "fallback:8b" {
		t.Fatalf("decision = %#v", env.Data.Decision)
	}
	assertRouteWarningCode(t, env.Warnings, "W_FALLBACK_USED")
	assertRouteWarningCode(t, env.Warnings, "W_MODEL_NOT_LOADED")
	if env.Data.Input.Source != "inline" || env.Data.Input.EstimatedTokens != 3 {
		t.Fatalf("input = %#v", env.Data.Input)
	}
}

func TestRouteHumanExplanationFormat(t *testing.T) {
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

	stdout, _, err := executeForTest("route", "code", "--prompt", "secret prompt text")
	if err != nil {
		t.Fatalf("route human error = %v stdout=%s", err, stdout)
	}
	assertOrderedSubstrings(t, stdout, []string{
		"selected: fallback:8b on ollama (fallback)",
		"reason: selected fallback because primary 'primary:70b' is unavailable",
		"candidates:",
		"role      backend",
		"primary  ollama",
		"fallback ollama",
		"prompt:",
		"5 estimated tokens / 8192 max context tokens",
		"warnings:",
		"W_FALLBACK_USED",
		"W_MODEL_NOT_LOADED",
		"next: inferctl model fallback:8b --json",
	})
	if strings.Contains(stdout, "secret prompt text") || strings.Contains(stdout, "inferctl warmup") {
		t.Fatalf("human output leaked prompt or future command:\n%s", stdout)
	}
}

func TestRouteHumanOutputMatchesJSONDecision(t *testing.T) {
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

	jsonOut, _, err := executeForTest("route", "code", "--prompt", "hello world", "--json")
	if err != nil {
		t.Fatalf("route json error = %v stdout=%s", err, jsonOut)
	}
	env := decodeRouteEnvelope(t, jsonOut)
	humanOut, _, err := executeForTest("route", "code", "--prompt", "hello world")
	if err != nil {
		t.Fatalf("route human error = %v stdout=%s", err, humanOut)
	}

	selectedLine := "selected: " + env.Data.Decision.SelectedModel + " on " + env.Data.Decision.SelectedBackend
	if env.Data.Decision.IsFallback {
		selectedLine += " (fallback)"
	}
	assertOrderedSubstrings(t, humanOut, []string{
		selectedLine,
		"reason: " + env.Data.Decision.Reason,
		"candidates:",
	})
	for _, candidate := range env.Data.Candidates {
		for _, fragment := range routeCandidateFragments(candidate) {
			if !strings.Contains(humanOut, fragment) {
				t.Fatalf("human output missing candidate fragment %q from JSON %#v:\n%s", fragment, candidate, humanOut)
			}
		}
		if !strings.Contains(humanOut, routeCandidateStatus(candidate)) {
			t.Fatalf("human output missing candidate from JSON %#v:\n%s", candidate, humanOut)
		}
	}
	for _, warning := range env.Warnings {
		if !strings.Contains(humanOut, warning.Code) {
			t.Fatalf("human output missing warning %s from JSON:\n%s", warning.Code, humanOut)
		}
	}
	next := firstCurrentCommand(env.Commands)
	if next == "" || !strings.Contains(humanOut, "next: "+next) {
		t.Fatalf("human output next command does not match JSON commands %q:\n%s", next, humanOut)
	}
}

func TestRouteNoRouteAvailable(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "other:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "primary:70b",
		Fallback:  "fallback:8b",
	}))

	stdout, _, err := executeForTest("route", "code", "--json")
	if err == nil {
		t.Fatal("expected no-route error")
	}
	if !strings.Contains(stdout, "E_NO_ROUTE_AVAILABLE") || !strings.Contains(stdout, "inferctl doctor") {
		t.Fatalf("unexpected no-route envelope: %s", stdout)
	}

	stdout, stderr, err := executeForTest("route", "code")
	if err == nil {
		t.Fatal("expected human no-route error")
	}
	assertOrderedSubstrings(t, stdout, []string{
		"selected: none",
		"reason: no candidate model for task 'code' is reachable",
		"candidates:",
		"primary  ollama",
		"fallback                  fallback:8b",
		"next: inferctl doctor --json",
	})
	if !strings.Contains(stderr, "no candidate model for task 'code' is reachable") || !strings.Contains(stderr, "exit: 4") {
		t.Fatalf("stderr missing no-route diagnostic:\n%s", stderr)
	}
}

func TestRoutePromptFileAndNearLimit(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}, Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeRouteConfig(t, server.URL, 10))
	promptPath := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptPath, []byte(strings.Repeat("a", 40)), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeForTest("route", "code", "--prompt-file", promptPath, "--quiet", "--json")
	if err != nil {
		t.Fatalf("route error = %v stdout=%s", err, stdout)
	}
	env := decodeRouteEnvelope(t, stdout)
	if env.Data.Input.Source != "file:"+promptPath || env.Data.Constraints.ContextPct != 100 {
		t.Fatalf("input/constraints = %#v %#v", env.Data.Input, env.Data.Constraints)
	}
	assertRouteWarningCode(t, env.Warnings, "W_CONTEXT_NEAR_LIMIT")

	stdout, _, err = executeForTest("route", "code", "--prompt-file", promptPath)
	if err != nil {
		t.Fatalf("route human prompt-file error = %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "10 estimated tokens / 10 max context tokens") {
		t.Fatalf("human prompt metadata missing:\n%s", stdout)
	}
	if strings.Contains(stdout, promptPath) || strings.Contains(stdout, strings.Repeat("a", 40)) {
		t.Fatalf("human output leaked prompt path or content:\n%s", stdout)
	}
}

func TestRouteStdin(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}, Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTestWithInput("abcdabcd", "route", "code", "--from-stdin", "--json")
	if err != nil {
		t.Fatalf("route error = %v stdout=%s", err, stdout)
	}
	env := decodeRouteEnvelope(t, stdout)
	if env.Data.Input.Source != "stdin" || env.Data.Input.EstimatedTokens != 2 {
		t.Fatalf("stdin input = %#v", env.Data.Input)
	}
}

func TestRouteInvocationErrors(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("route", "--json")
	if err == nil || !strings.Contains(stdout, "E_MISSING_ARG") {
		t.Fatalf("missing task stdout=%s err=%v", stdout, err)
	}
	stdout, _, err = executeForTest("route", "unknown", "--json")
	if err == nil || !strings.Contains(stdout, "E_UNKNOWN_TASK") {
		t.Fatalf("unknown task stdout=%s err=%v", stdout, err)
	}
}

type routeEnvelopeForTest struct {
	Data     routeReport `json:"data"`
	Warnings []struct {
		Code string `json:"code"`
	} `json:"warnings"`
	Commands []struct {
		Command            string  `json:"command"`
		AvailableInVersion *string `json:"available_in_version"`
	} `json:"commands"`
}

func decodeRouteEnvelope(t *testing.T, stdout string) routeEnvelopeForTest {
	t.Helper()
	var env routeEnvelopeForTest
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal route envelope: %v\n%s", err, stdout)
	}
	return env
}

func assertRouteWarningCode(t *testing.T, warnings []struct {
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

func assertOrderedSubstrings(t *testing.T, text string, wants []string) {
	t.Helper()
	offset := 0
	for _, want := range wants {
		idx := strings.Index(text[offset:], want)
		if idx < 0 {
			t.Fatalf("missing %q after offset %d in:\n%s", want, offset, text)
		}
		offset += idx + len(want)
	}
}

func routeCandidateFragments(candidate inferctl.RouteCandidate) []string {
	fragments := []string{candidate.Role, candidate.Model}
	if candidate.Backend != nil {
		fragments = append(fragments, *candidate.Backend)
	}
	return fragments
}

func routeCandidateStatus(candidate inferctl.RouteCandidate) string {
	status := "available"
	if !candidate.Available && candidate.UnavailabilityReason != nil {
		status = *candidate.UnavailabilityReason
	} else if candidate.Available && !candidate.Loaded {
		status = "available_not_ready"
	} else if candidate.Loaded {
		status = "selected_ready"
	}
	return status
}

func firstCurrentCommand(commands []struct {
	Command            string  `json:"command"`
	AvailableInVersion *string `json:"available_in_version"`
}) string {
	for _, command := range commands {
		if command.AvailableInVersion == nil {
			return command.Command
		}
	}
	return ""
}

func executeForTestWithInput(input string, args ...string) (string, string, error) {
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func writeRouteConfig(t *testing.T, ollamaURL string, maxContext int) string {
	t.Helper()
	body := `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = ` + fmtInt(maxContext) + `
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.ollama]
kind = "ollama"
base_url = "` + ollamaURL + `"
default = true

[routing.code]
model = "qwen3:8b"
backend = "ollama"
fallback = []
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fmtInt(v int) string {
	return strconv.Itoa(v)
}
