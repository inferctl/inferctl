package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/testserver"
)

func TestPreflightPrimaryReady(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("preflight", "code", "--json")
	if err != nil {
		t.Fatalf("preflight error = %v stdout=%s", err, stdout)
	}
	env := decodePreflightEnvelope(t, stdout)
	if !env.OK || !env.Data.Runnability.Runnable || env.Data.Runnability.ExitCode != 0 {
		t.Fatalf("runnability = %#v ok=%v", env.Data.Runnability, env.OK)
	}
	if env.Data.RouteDecision.SelectedModel != "qwen3:8b" || !env.Data.RouteDecision.Ready {
		t.Fatalf("route decision = %#v", env.Data.RouteDecision)
	}
	assertJSONSubsetGolden(t, "preflight.golden.json", map[string]any{
		"prompt":         env.Data.Prompt,
		"route_decision": env.Data.RouteDecision,
		"runnability":    env.Data.Runnability,
		"policy":         env.Data.Policy,
		"warnings":       env.Data.Warnings,
	})
}

func TestPreflightFallbackPolicy(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "fallback:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "fallback:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "primary:70b",
		Fallback:  "fallback:8b",
	}))

	stdout, _, err := executeForTest("preflight", "code", "--json")
	if err == nil {
		t.Fatal("fallback should be blocked without --allow-fallback")
	}
	blocked := decodePreflightEnvelope(t, stdout)
	if blocked.OK || blocked.Data.Runnability.Runnable || blocked.Data.Runnability.ExitCode != exitUserInput {
		t.Fatalf("blocked envelope = %#v", blocked)
	}
	if !strings.Contains(stdout, "E_PREFLIGHT_POLICY_BLOCKED") || !blocked.Data.RouteDecision.IsFallback {
		t.Fatalf("blocked stdout = %s", stdout)
	}

	stdout, _, err = executeForTest("preflight", "code", "--allow-fallback", "--json")
	if err != nil {
		t.Fatalf("fallback should be accepted with --allow-fallback: %v stdout=%s", err, stdout)
	}
	accepted := decodePreflightEnvelope(t, stdout)
	if !accepted.OK || !accepted.Data.Runnability.Runnable || !accepted.Data.Policy.AllowFallback {
		t.Fatalf("accepted envelope = %#v", accepted)
	}
}

func TestPreflightRequireReady(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("preflight", "code", "--require-ready", "--json")
	if err == nil {
		t.Fatal("expected require-ready policy block")
	}
	env := decodePreflightEnvelope(t, stdout)
	if env.OK || env.Data.Runnability.Runnable || !env.Data.Policy.RequireReady {
		t.Fatalf("require-ready envelope = %#v", env)
	}
}

func TestPreflightPromptSourcesAndMarkdown(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b"}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b"}},
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))
	promptPath := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("abcdabcd"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeForTest("preflight", "code", "--prompt-file", promptPath, "--json")
	if err != nil {
		t.Fatalf("prompt-file preflight error = %v stdout=%s", err, stdout)
	}
	fileEnv := decodePreflightEnvelope(t, stdout)
	if fileEnv.Data.Prompt.Source != "file" || fileEnv.Data.Prompt.Filename == nil || *fileEnv.Data.Prompt.Filename != "prompt.txt" {
		t.Fatalf("file prompt metadata = %#v", fileEnv.Data.Prompt)
	}
	if strings.Contains(stdout, promptPath) || strings.Contains(stdout, "abcdabcd") {
		t.Fatalf("preflight leaked prompt path or content: %s", stdout)
	}

	stdout, _, err = executeForTestWithInput("abcd", "preflight", "code", "-", "--format", "markdown")
	if err != nil {
		t.Fatalf("stdin marker markdown error = %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "## inferctl preflight: code") || strings.Contains(stdout, "abcd") {
		t.Fatalf("markdown output = %s", stdout)
	}
}

func TestPreflightInvocationAndEnvironmentErrors(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{OllamaURL: server.URL}))

	stdout, _, err := executeForTest("preflight", "--json")
	if err == nil || !strings.Contains(stdout, "E_MISSING_ARG") {
		t.Fatalf("missing task stdout=%s err=%v", stdout, err)
	}
	stdout, _, err = executeForTest("preflight", "unknown", "--json")
	if err == nil || !strings.Contains(stdout, "E_UNKNOWN_TASK") {
		t.Fatalf("unknown task stdout=%s err=%v", stdout, err)
	}
	stdout, _, err = executeForTest("preflight", "code", "--prompt", "a", "--from-stdin", "--json")
	if err == nil || !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("multiple prompt sources stdout=%s err=%v", stdout, err)
	}
	stdout, _, err = executeForTest("preflight", "code", "--prompt-file", filepath.Join(t.TempDir(), "missing.txt"), "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_UNREADABLE") {
		t.Fatalf("unreadable prompt stdout=%s err=%v", stdout, err)
	}
}

func TestPreflightInvalidConfigAndNoRouteExitClasses(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "inferctl.toml")
	if err := os.WriteFile(invalidPath, []byte("[meta]\nschema_version = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INFERCTL_CONFIG", invalidPath)
	stdout, _, err := executeForTest("preflight", "code", "--json")
	if err == nil || !strings.Contains(stdout, "E_CONFIG_INVALID") {
		t.Fatalf("invalid config stdout=%s err=%v", stdout, err)
	}
	assertEnvelopeExitCode(t, stdout, exitEnvironment)

	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "other:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeDoctorConfig(t, doctorConfigOptions{
		OllamaURL: server.URL,
		Primary:   "primary:70b",
		Fallback:  "fallback:8b",
	}))
	stdout, _, err = executeForTest("preflight", "code", "--json")
	if err == nil || !strings.Contains(stdout, "E_NO_ROUTE_AVAILABLE") {
		t.Fatalf("no route stdout=%s err=%v", stdout, err)
	}
	assertEnvelopeExitCode(t, stdout, exitTransient)
}

type preflightEnvelopeForTest struct {
	OK   bool            `json:"ok"`
	Data preflightReport `json:"data"`
}

func decodePreflightEnvelope(t *testing.T, stdout string) preflightEnvelopeForTest {
	t.Helper()
	var env preflightEnvelopeForTest
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal preflight envelope: %v\n%s", err, stdout)
	}
	return env
}

func assertEnvelopeExitCode(t *testing.T, stdout string, want int) {
	t.Helper()
	var env struct {
		Errors []struct {
			ExitCode int `json:"exit_code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal error envelope: %v\n%s", err, stdout)
	}
	if len(env.Errors) == 0 || env.Errors[0].ExitCode != want {
		t.Fatalf("exit code = %#v want %d", env.Errors, want)
	}
}
