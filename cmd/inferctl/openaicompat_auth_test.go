package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferctl/inferctl/internal/testserver"
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

func TestOpenAICompatTypedLiteralCredentialSucceeds(t *testing.T) {
	headerValue := "fixture-typed-auth-value"
	server := testserver.New(testserver.Fixture{
		Kind:            testserver.KindOpenAICompat,
		Models:          []testserver.Model{{Name: "remote-model"}},
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: headerValue,
	})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeOpenAICompatTypedCredentialConfig(t, server.URL, headerValue))

	stdout, _, err := executeForTest("models", "--json")
	if err != nil {
		t.Fatalf("models with typed credential error = %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "remote-model") || strings.Contains(stdout, headerValue) {
		t.Fatalf("unexpected models output: %s", stdout)
	}
}

func TestOpenAICompatEnvironmentCredentialSucceeds(t *testing.T) {
	headerValue := "fixture-environment-auth-value"
	server := testserver.New(testserver.Fixture{
		Kind:            testserver.KindOpenAICompat,
		Models:          []testserver.Model{{Name: "remote-model"}},
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: headerValue,
	})
	defer server.Close()
	t.Setenv("INFERCTL_TEST_REMOTE_TOKEN", headerValue)
	t.Setenv("INFERCTL_CONFIG", writeOpenAICompatEnvironmentCredentialConfig(t, server.URL, "INFERCTL_TEST_REMOTE_TOKEN"))

	stdout, _, err := executeForTest("models", "--json")
	if err != nil {
		t.Fatalf("models with environment credential error = %v stdout=%s", err, stdout)
	}
	if !strings.Contains(stdout, "remote-model") || strings.Contains(stdout, headerValue) {
		t.Fatalf("unexpected models output: %s", stdout)
	}
}

func TestOpenAICompatEnvironmentCredentialResolutionFailuresAreRedacted(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    *string
		wantCode string
	}{
		{name: "missing", variable: "INFERCTL_TEST_MISSING_TOKEN", wantCode: "E_CREDENTIAL_REFERENCE_ENVIRONMENT_MISSING"},
		{name: "empty", variable: "INFERCTL_TEST_EMPTY_TOKEN", value: stringPtr(""), wantCode: "E_CREDENTIAL_REFERENCE_ENVIRONMENT_EMPTY"},
		{name: "unusable", variable: "INFERCTL_TEST_UNUSABLE_TOKEN", value: stringPtr("Bearer bad\nvalue"), wantCode: "E_CREDENTIAL_REFERENCE_VALUE_UNUSABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == nil {
				original, existed := os.LookupEnv(tt.variable)
				if err := os.Unsetenv(tt.variable); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if existed {
						_ = os.Setenv(tt.variable, original)
					} else {
						_ = os.Unsetenv(tt.variable)
					}
				})
			} else {
				t.Setenv(tt.variable, *tt.value)
			}
			t.Setenv("INFERCTL_CONFIG", writeOpenAICompatEnvironmentCredentialConfig(t, "http://127.0.0.1:8080", tt.variable))
			stdout, _, err := executeForTest("models", "--json")
			if err == nil || !strings.Contains(stdout, tt.wantCode) {
				t.Fatalf("wanted %s: err=%v stdout=%s", tt.wantCode, err, stdout)
			}
			if tt.value != nil && *tt.value != "" && strings.Contains(stdout, *tt.value) {
				t.Fatalf("credential value leaked: %s", stdout)
			}
		})
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

func writeOpenAICompatTypedCredentialConfig(t *testing.T, baseURL, headerValue string) string {
	t.Helper()
	body := `[meta]
schema_version = "0.1"

[profile]
name = "typed_credential"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.openai]
kind = "openai_compat"
base_url = "` + baseURL + `"
default = true
auth_header_name = "Authorization"

[backends.openai.credential]
version = "v1"
source = "literal"
literal_value = "` + headerValue + `"

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

func writeOpenAICompatEnvironmentCredentialConfig(t *testing.T, baseURL, variable string) string {
	t.Helper()
	body := `[meta]
schema_version = "0.1"

[profile]
name = "environment_credential"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.openai]
kind = "openai_compat"
base_url = "` + baseURL + `"
default = true
auth_header_name = "Authorization"

[backends.openai.credential]
version = "v1"
source = "environment"
environment_variable = "` + variable + `"

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
