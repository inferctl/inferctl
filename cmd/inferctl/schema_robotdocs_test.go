package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaCommandExportsEnvelopeAndVerbSchemas(t *testing.T) {
	stdout, _, err := executeForTest("schema", "--json")
	if err != nil {
		t.Fatalf("schema error = %v stdout=%s", err, stdout)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Envelope    map[string]any `json:"envelope"`
			Schemas     map[string]any `json:"schemas"`
			Definitions map[string]any `json:"definitions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if !env.OK || env.Data.Envelope["type"] != "object" || env.Data.Schemas["doctor_report"] == nil || env.Data.Definitions["error"] == nil {
		t.Fatalf("schema export incomplete: %#v", env.Data)
	}
}

func TestSchemaCommandCanReturnOneCommandSchema(t *testing.T) {
	stdout, _, err := executeForTest("schema", "--command", "config show", "--json")
	if err != nil {
		t.Fatalf("schema --command error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data struct {
			Command string         `json:"command"`
			Schema  map[string]any `json:"schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.Command != "config show" || env.Data.Schema["type"] != "object" {
		t.Fatalf("unexpected command schema: %#v", env.Data)
	}
}

func TestConfigSchemaCommandExportsConfigSchema(t *testing.T) {
	stdout, _, err := executeForTest("config", "schema", "--json")
	if err != nil {
		t.Fatalf("config schema error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data struct {
			Type       string         `json:"type"`
			Required   []string       `json:"required"`
			Properties map[string]any `json:"properties"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.Type != "object" || len(env.Data.Required) == 0 || env.Data.Properties["backends"] == nil || env.Data.Properties["routing"] == nil {
		t.Fatalf("config schema incomplete: %#v", env.Data)
	}
}

func TestRobotDocsGuideCommand(t *testing.T) {
	stdout, _, err := executeForTest("robot-docs", "guide", "--json")
	if err != nil {
		t.Fatalf("robot-docs guide error = %v stdout=%s", err, stdout)
	}
	var env struct {
		Data struct {
			Name    string `json:"name"`
			Format  string `json:"format"`
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if env.Data.Name != "inferctl Agent Guide" || env.Data.Format != "markdown" || !strings.Contains(env.Data.Content, "inferctl capabilities --json") {
		t.Fatalf("robot guide incomplete: %#v", env.Data)
	}
}
