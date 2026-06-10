package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
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
	if !strings.Contains(stdout, "E_NO_ROUTE_AVAILABLE") || !strings.Contains(stdout, "infer doctor") {
		t.Fatalf("unexpected no-route envelope: %s", stdout)
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
