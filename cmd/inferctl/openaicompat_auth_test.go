package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func TestOpenAICompatAuthFailuresAcrossBackendReadingVerbs(t *testing.T) {
	headerValue := "fixture-" + "auth-" + "value"
	server := testserver.New(testserver.Fixture{
		Kind:            testserver.KindOpenAICompat,
		Models:          []testserver.Model{{Name: "remote-model"}},
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: headerValue,
	})
	defer server.Close()
	wrongValue := "wrong-" + "value"
	t.Setenv("INFERCTL_CONFIG", writeOpenAICompatConfig(t, server.URL, false, "Authorization", wrongValue))

	for _, args := range [][]string{
		{"doctor", "--json"},
		{"backends", "--json"},
		{"models", "--json"},
		{"model", "remote-model", "--json"},
		{"route", "code", "--json"},
	} {
		stdout, _, err := executeForTest(args...)
		if err == nil {
			t.Fatalf("%v expected auth failure", args)
		}
		if !strings.Contains(stdout, "E_BACKEND_AUTH_FAILED") {
			t.Fatalf("%v missing auth failure envelope: %s", args, stdout)
		}
		if strings.Contains(stdout, headerValue) || strings.Contains(stdout, wrongValue) {
			t.Fatalf("%v leaked auth value: %s", args, stdout)
		}
	}
}

func TestOpenAICompatRemoteOptInAcrossBackendReadingVerbs(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeOpenAICompatConfig(t, "https://example.com/v1", false, "", ""))

	for _, args := range [][]string{
		{"doctor", "--json"},
		{"backends", "--json"},
		{"models", "--json"},
		{"model", "remote-model", "--json"},
		{"route", "code", "--json"},
	} {
		stdout, _, err := executeForTest(args...)
		if err == nil {
			t.Fatalf("%v expected remote-not-allowed failure", args)
		}
		if !strings.Contains(stdout, "E_BACKEND_REMOTE_NOT_ALLOWED") {
			t.Fatalf("%v missing remote-not-allowed envelope: %s", args, stdout)
		}
	}
}

func TestOpenAICompatAuthHeaderSucceeds(t *testing.T) {
	headerValue := "fixture-" + "auth-" + "value"
	server := testserver.New(testserver.Fixture{
		Kind:            testserver.KindOpenAICompat,
		Models:          []testserver.Model{{Name: "remote-model"}},
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: headerValue,
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeOpenAICompatConfig(t, server.URL, false, "Authorization", headerValue))

	stdout, _, err := executeForTest("models", "--json")
	if err != nil {
		t.Fatalf("models with auth error = %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "remote-model") || strings.Contains(stdout, headerValue) {
		t.Fatalf("unexpected models output: %s", stdout)
	}
}

func writeOpenAICompatConfig(t *testing.T, baseURL string, remoteAllowed bool, headerName, headerValue string) string {
	t.Helper()
	auth := ""
	if headerName != "" {
		auth += `auth_header_name = "` + headerName + `"` + "\n"
	}
	if headerValue != "" {
		auth += `auth_header_value = "` + headerValue + `"` + "\n"
	}
	body := `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.openai]
kind = "openai_compat"
base_url = "` + baseURL + `"
default = true
remote_allowed = ` + boolString(remoteAllowed) + `
` + auth + `
[routing.code]
model = "remote-model"
backend = "openai"
fallback = []
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
