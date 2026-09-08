package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateClean(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, typedCredentialConfig))
	stdout, stderr, err := executeForTest("config", "validate", "--json")
	if err != nil {
		t.Fatalf("config validate error = %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Passed   bool  `json:"passed"`
			Findings []any `json:"findings"`
			Summary  struct {
				Errors   int `json:"errors"`
				Warnings int `json:"warnings"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK || !env.Data.Passed || env.Data.Summary.Errors != 0 || env.Data.Summary.Warnings != 0 {
		t.Fatalf("unexpected envelope = %#v", env)
	}
	assertJSONSubsetGolden(t, "config_validate.clean.golden.json", map[string]any{"findings": env.Data.Findings})
}

func TestConfigValidateLegacyCredentialWarns(t *testing.T) {
	legacyConfig := stringsReplace(typedCredentialConfig, `[backends.remote.credential]
version = "v1"
source = "literal"
literal_value = "Bearer typed-fixture"
`, `auth_header_value = "Bearer legacy-fixture"
`)
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, legacyConfig))
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err != nil {
		t.Fatalf("legacy config validate error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Findings []struct {
				Key      string         `json:"key"`
				Severity string         `json:"severity"`
				Details  map[string]any `json:"details"`
			} `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	for _, finding := range env.Data.Findings {
		if finding.Key == "backends.remote.auth_header_value" && finding.Severity == "warning" && finding.Details["code"] == "W_CONFIG_KEY_DEPRECATED" {
			return
		}
	}
	t.Fatalf("legacy credential warning missing: %s", stdout)
}

func TestConfigValidateTypedCredentialFailuresAreStable(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		change   string
		wantKey  string
		wantCode string
	}{
		{
			name:     "source",
			old:      `source = "literal"`,
			change:   `source = "environment"`,
			wantKey:  "backends.remote.credential.source",
			wantCode: "E_CREDENTIAL_REFERENCE_SOURCE_UNSUPPORTED",
		},
		{
			name:     "version",
			old:      `version = "v1"`,
			change:   `version = "v2"`,
			wantKey:  "backends.remote.credential.version",
			wantCode: "E_CREDENTIAL_REFERENCE_VERSION_UNSUPPORTED",
		},
		{
			name:     "literal value",
			old:      `literal_value = "Bearer typed-fixture"`,
			change:   `literal_value = ""`,
			wantKey:  "backends.remote.credential.literal_value",
			wantCode: "E_CREDENTIAL_REFERENCE_LITERAL_REQUIRED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := stringsReplace(typedCredentialConfig, tt.old, tt.change)
			t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
			stdout, _, err := executeForTest("config", "validate", "--json")
			if err == nil {
				t.Fatalf("expected validation error: %s", stdout)
			}
			assertCredentialFinding(t, stdout, tt.wantKey, tt.wantCode)
		})
	}
}

func TestConfigValidateTypedCredentialMissingSourceIsStable(t *testing.T) {
	cfg := stringsReplace(typedCredentialConfig, "source = \"literal\"\n", "")
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err == nil {
		t.Fatalf("expected validation error: %s", stdout)
	}
	assertCredentialFinding(t, stdout, "backends.remote.credential.source", "E_CREDENTIAL_REFERENCE_SOURCE_REQUIRED")
}

func assertCredentialFinding(t *testing.T, stdout, wantKey, wantCode string) {
	t.Helper()
	var env struct {
		Data struct {
			Findings []struct {
				Key     string         `json:"key"`
				Details map[string]any `json:"details"`
			} `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	for _, finding := range env.Data.Findings {
		if finding.Key == wantKey && finding.Details["code"] == wantCode {
			return
		}
	}
	t.Fatalf("missing %s at %s: %s", wantCode, wantKey, stdout)
}

func TestConfigValidateWarningOnlyAndStrict(t *testing.T) {
	path := writeConfig(t, stringsReplace(workedExampleConfig, `mode = "warn"`, `mode = "strict"`))
	t.Setenv("INFERCTL_CONFIG", path)
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err != nil {
		t.Fatalf("warning-only validate should exit 0: %v", err)
	}
	var okEnv struct {
		OK   bool `json:"ok"`
		Data struct {
			Passed  bool `json:"passed"`
			Summary struct {
				Warnings int `json:"warnings"`
			} `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &okEnv); err != nil {
		t.Fatal(err)
	}
	if !okEnv.OK || !okEnv.Data.Passed || okEnv.Data.Summary.Warnings == 0 {
		t.Fatalf("warning-only envelope = %#v", okEnv)
	}

	stdout, _, err = executeForTest("config", "validate", "--strict", "--json")
	if err == nil {
		t.Fatal("strict warning validate should fail")
	}
	var strictEnv struct {
		OK   bool `json:"ok"`
		Data struct {
			Findings []any `json:"findings"`
		} `json:"data"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &strictEnv); err != nil {
		t.Fatal(err)
	}
	if strictEnv.OK || strictEnv.Errors[0].Code != "E_CONFIG_VALIDATION_FAILED" || len(strictEnv.Data.Findings) == 0 {
		t.Fatalf("strict envelope = %#v", strictEnv)
	}
}

func TestConfigValidateMissingSchemaVersion(t *testing.T) {
	cfg := stringsReplace(workedExampleConfig, "schema_version = \"0.1\"\n", "")
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err == nil {
		t.Fatal("missing schema_version should fail")
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Findings []struct {
				Severity string `json:"severity"`
				Key      string `json:"key"`
			} `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if env.OK {
		t.Fatal("ok=true")
	}
	found := false
	for _, finding := range env.Data.Findings {
		if finding.Key == "meta.schema_version" && finding.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing schema_version finding absent: %#v", env.Data.Findings)
	}
}

func TestConfigValidateDirectKeyLineColumn(t *testing.T) {
	cfg := stringsReplace(workedExampleConfig, `base_url = "http://127.0.0.1:11434"`, `base_url = "not a url"`)
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
	stdout, _, err := executeForTest("config", "validate", "--json")
	if err == nil {
		t.Fatal("invalid base_url should fail")
	}
	var env struct {
		Data struct {
			Findings []struct {
				Key    string `json:"key"`
				Line   *int   `json:"line"`
				Column *int   `json:"column"`
			} `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	for _, finding := range env.Data.Findings {
		if finding.Key == "backends.ollama.base_url" {
			if finding.Line == nil || finding.Column == nil {
				t.Fatalf("line/column not populated: %#v", finding)
			}
			return
		}
	}
	t.Fatalf("base_url finding missing: %#v", env.Data.Findings)
}

func TestConfigValidateUnknownKeySuggestsNearestKnownKey(t *testing.T) {
	cfg := stringsReplace(workedExampleConfig, `max_concurrent_models = 1`, "max_concurrent_models = 1\nmax_concurent_models = 2")
	t.Setenv("INFERCTL_CONFIG", writeConfig(t, cfg))
	stdout, stderr, err := executeForTest("config", "validate", "--json")
	if err == nil {
		t.Fatal("unknown key should fail validation")
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Findings []struct {
				Severity string         `json:"severity"`
				Key      string         `json:"key"`
				Line     *int           `json:"line"`
				Details  map[string]any `json:"details"`
			} `json:"findings"`
		} `json:"data"`
		Errors []struct {
			Code       string `json:"code"`
			DidYouMean string `json:"did_you_mean"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "E_CONFIG_VALIDATION_FAILED" {
		t.Fatalf("unexpected envelope = %#v", env)
	}
	if env.Errors[0].DidYouMean != "inferctl config explain --key profile.max_concurrent_models --json" {
		t.Fatalf("unexpected top-level remediation: %#v", env.Errors[0])
	}
	for _, finding := range env.Data.Findings {
		if finding.Key != "profile.max_concurent_models" {
			continue
		}
		if finding.Severity != "error" || finding.Line == nil {
			t.Fatalf("unknown-key finding missing severity/line: %#v", finding)
		}
		if finding.Details["did_you_mean"] != "profile.max_concurrent_models" {
			t.Fatalf("unknown-key suggestion missing: %#v", finding.Details)
		}
		if !strings.Contains(stderr, "error: config validation found") ||
			!strings.Contains(stderr, "try: inferctl config explain --key profile.max_concurrent_models --json") {
			t.Fatalf("stderr missing validation diagnostic:\n%s", stderr)
		}
		return
	}
	t.Fatalf("unknown-key finding absent: %#v", env.Data.Findings)
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func stringsReplace(s, old, new string) string {
	return strings.Replace(s, old, new, 1)
}
