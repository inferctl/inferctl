package config

import (
	"strings"
	"testing"
)

func TestModelCatalogAcceptsAliasAndEvidence(t *testing.T) {
	result, err := LoadBytes([]byte(modelCatalogConfig(`
[models.code_small]
backend = "ollama"
model = "qwen3:8b"

[models.code_small.capabilities.tools]
status = "supported"
source = "declared"
`)), SourcePaths{}, nil, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if validation := Validate(result, false); !validation.Passed {
		t.Fatalf("validation = %#v", validation.Findings)
	}
	model, ok := result.Config.ResolveModel("code_small")
	if !ok || model.Model != "qwen3:8b" || model.Backend != "ollama" {
		t.Fatalf("model = %#v ok=%v", model, ok)
	}
	tools := result.Config.CapabilityEvidenceFor("code_small")["tools"]
	if tools.Status != CapabilityStatusSupported || tools.Source != CapabilitySourceDeclared {
		t.Fatalf("tools evidence = %#v", tools)
	}
}

func TestModelCatalogRejectsConflictingEvidence(t *testing.T) {
	result, err := LoadBytes([]byte(modelCatalogConfig(`
[models.first]
backend = "ollama"
model = "qwen3:8b"
[models.first.capabilities.tools]
status = "supported"
source = "declared"

[models.second]
backend = "ollama"
model = "qwen3:8b"
[models.second.capabilities.tools]
status = "unsupported"
source = "observed"
`)), SourcePaths{}, nil, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	validation := Validate(result, false)
	for _, finding := range validation.Findings {
		if finding.Details["code"] == "E_MODEL_CAPABILITY_EVIDENCE_CONFLICT" {
			return
		}
	}
	t.Fatalf("findings did not include conflict: %#v", validation.Findings)
}

func modelCatalogConfig(models string) string {
	return strings.TrimSpace(`
[meta]
schema_version = "0.1"
[profile]
name = "test"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"
[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:11434"
default = true
[routing.code]
model = "code_small"
backend = "ollama"
`+models) + "\n"
}
