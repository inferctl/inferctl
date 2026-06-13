package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigValidateClean(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", writeTempConfig(t))
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
