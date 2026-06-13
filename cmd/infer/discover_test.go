package main

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func TestDiscoverEmptyGolden(t *testing.T) {
	t.Setenv("INFERCTL_TEST_DISCOVERY_PORTS", strconv.Itoa(unusedLocalPort(t)))
	stdout, _, err := executeForTest("discover", "--json")
	if err != nil {
		t.Fatalf("discover empty error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data discoveryReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	assertJSONSubsetGolden(t, "discover.empty.golden.json", map[string]any{
		"summary": env.Data.Summary,
		"mode":    env.Data.Scan.Mode,
	})
}

func TestDiscoverOllamaGolden(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Version: "0.20.5"})
	defer server.Close()
	t.Setenv("INFERCTL_TEST_DISCOVERY_PORTS", strconv.Itoa(serverPort(t, server.URL)))

	stdout, _, err := executeForTest("discover", "--kind", "ollama", "--json")
	if err != nil {
		t.Fatalf("discover ollama error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data discoveryReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(env.Data.Candidates) != 1 || env.Data.Candidates[0].ConfigPatch == nil {
		t.Fatalf("candidate = %#v", env.Data.Candidates)
	}
	assertJSONSubsetGolden(t, "discover.ollama.golden.json", map[string]any{
		"summary":  env.Data.Summary,
		"kind":     env.Data.Candidates[0].Kind,
		"verified": env.Data.Candidates[0].Verified,
	})
}

func TestDiscoverProvesThreeBackendKinds(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		serverKind testserver.Kind
	}{
		{"ollama", testserver.KindOllama},
		{"lmstudio", testserver.KindLMStudio},
		{"mlx", testserver.KindMLX},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			server := testserver.New(testserver.Fixture{Kind: tc.serverKind, Models: []testserver.Model{{Name: "model-a"}}})
			defer server.Close()
			t.Setenv("INFERCTL_TEST_DISCOVERY_PORTS", strconv.Itoa(serverPort(t, server.URL)))
			stdout, _, err := executeForTest("discover", "--kind", tc.kind, "--json")
			if err != nil {
				t.Fatalf("discover %s error = %v stdout=%s", tc.kind, err, stdout)
			}
			if !strings.Contains(stdout, `"kind":"`+tc.kind+`"`) || !strings.Contains(stdout, `"verified":true`) {
				t.Fatalf("discover %s not verified: %s", tc.kind, stdout)
			}
		})
	}
}

func TestDiscoverAmbiguousOpenAIStylePortHasNoPatch(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindLMStudio, Models: []testserver.Model{{Name: "model-a"}}})
	defer server.Close()
	t.Setenv("INFERCTL_TEST_DISCOVERY_PORTS", strconv.Itoa(serverPort(t, server.URL)))

	stdout, _, err := executeForTest("discover", "--json")
	if err != nil {
		t.Fatalf("discover ambiguous error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data discoveryReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if len(env.Data.Candidates) < 2 {
		t.Fatalf("expected ambiguous candidates: %#v", env.Data.Candidates)
	}
	for _, candidate := range env.Data.Candidates {
		if candidate.ConfigPatch != nil {
			t.Fatalf("ambiguous candidate produced patch: %#v", candidate)
		}
	}
}

func TestDiscoverOpenAICompatAuthFailureIsTyped(t *testing.T) {
	server := testserver.New(testserver.Fixture{
		Kind:            testserver.KindOpenAICompat,
		Models:          []testserver.Model{{Name: "model-a"}},
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: "fixture-" + "auth-" + "value",
	})
	defer server.Close()
	t.Setenv("INFERCTL_TEST_DISCOVERY_PORTS", strconv.Itoa(serverPort(t, server.URL)))

	stdout, _, err := executeForTest("discover", "--kind", "openai_compat", "--json")
	if err == nil {
		t.Fatal("expected auth failure")
	}
	if !strings.Contains(stdout, "E_BACKEND_AUTH_FAILED") {
		t.Fatalf("unexpected auth failure: %s", stdout)
	}
}

func TestDiscoverGrammarErrors(t *testing.T) {
	for _, args := range [][]string{
		{"discover", "--format", "yaml", "--json"},
		{"discover", "--kind", "bogus", "--json"},
		{"discover", "--timeout-ms", "10", "--json"},
	} {
		stdout, _, err := executeForTest(args...)
		if err == nil || !strings.Contains(stdout, "E_INVALID_ARG") {
			t.Fatalf("%v expected invalid arg, stdout=%s err=%v", args, stdout, err)
		}
	}
}

func serverPort(t *testing.T, raw string) int {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func unusedLocalPort(t *testing.T) int {
	t.Helper()
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama})
	port := serverPort(t, server.URL)
	server.Close()
	return port
}
