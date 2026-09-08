package config

import "github.com/inferctl/inferctl/pkg/inferctl"

// ResolveModel returns the concrete model configuration for an alias. A name
// without a catalog entry is a legacy direct model name.
func (cfg Config) ResolveModel(name string) (ModelConfig, bool) {
	model, ok := cfg.Models[name]
	return model, ok
}

func (cfg Config) CapabilityEvidenceFor(alias string) map[string]inferctl.CapabilityEvidence {
	evidence := unknownCapabilityEvidence()
	model, ok := cfg.Models[alias]
	if !ok {
		return evidence
	}
	for capability, item := range model.Capabilities {
		evidence[capability] = inferctl.CapabilityEvidence{Status: item.Status, Source: item.Source}
	}
	return evidence
}

func unknownCapabilityEvidence() map[string]inferctl.CapabilityEvidence {
	return map[string]inferctl.CapabilityEvidence{
		"tools":      {Status: CapabilityStatusUnknown, Source: "none"},
		"vision":     {Status: CapabilityStatusUnknown, Source: "none"},
		"json_mode":  {Status: CapabilityStatusUnknown, Source: "none"},
		"embeddings": {Status: CapabilityStatusUnknown, Source: "none"},
	}
}
