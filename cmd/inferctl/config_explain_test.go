package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigExplainWorksWithoutConfig(t *testing.T) {
	t.Setenv("INFERCTL_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	stdout, _, err := executeForTest("config", "explain", "--json")
	if err != nil {
		t.Fatalf("config explain error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data configExplainData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.Format != "toml" || env.Data.SchemaVersion != "0.1" || len(env.Data.Keys) < 10 {
		t.Fatalf("unexpected explain data = %#v", env.Data)
	}
	if !strings.Contains(env.Data.AnnotatedSource, "[backends.<name>]") {
		t.Fatalf("annotated source missing backend section:\n%s", env.Data.AnnotatedSource)
	}
}

func TestConfigExplainKeyAndWildcard(t *testing.T) {
	stdout, _, err := executeForTest("config", "explain", "--key", "profile.mode", "--format", "md", "--json")
	if err != nil {
		t.Fatalf("config explain key error = %v stdout=%s", err, stdout)
	}
	var keyEnv struct {
		Data configExplainData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &keyEnv); err != nil {
		t.Fatal(err)
	}
	if keyEnv.Data.Format != "md" || len(keyEnv.Data.Keys) != 1 || keyEnv.Data.Keys[0].Key != "profile.mode" {
		t.Fatalf("key explain = %#v", keyEnv.Data)
	}
	if !strings.Contains(keyEnv.Data.AnnotatedSource, "Valid values") {
		t.Fatalf("markdown source missing valid values:\n%s", keyEnv.Data.AnnotatedSource)
	}

	stdout, _, err = executeForTest("config", "explain", "--key", "routing.<task>.*", "--json")
	if err != nil {
		t.Fatalf("config explain wildcard error = %v stdout=%s", err, stdout)
	}
	var wildcardEnv struct {
		Data configExplainData `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &wildcardEnv); err != nil {
		t.Fatal(err)
	}
	if len(wildcardEnv.Data.Keys) != 4 {
		t.Fatalf("wildcard keys = %#v", wildcardEnv.Data.Keys)
	}
}

func TestConfigExplainUnknownKey(t *testing.T) {
	stdout, _, err := executeForTest("config", "explain", "--key", "profile.nope", "--json")
	if err == nil {
		t.Fatal("expected unknown key error")
	}
	if !strings.Contains(stdout, "E_INVALID_ARG") {
		t.Fatalf("unexpected unknown key envelope: %s", stdout)
	}
}
