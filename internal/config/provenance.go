package config

type Provenance string

const (
	ProvenanceDefault Provenance = "default"
	ProvenanceFile    Provenance = "file"
	ProvenanceEnv     Provenance = "env"
	ProvenanceFlag    Provenance = "flag"
)

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func markDefaultProvenance(cfg Config, out map[string]Provenance) {
	markDefault(out, "meta.schema_version")
	markDefault(out, "profile.mode")
	markDefault(out, "profile.allow_premium")
	markDefault(out, "profile.vram_total_bytes_hint")
	for name, b := range cfg.Backends {
		prefix := "backends." + name + "."
		markDefault(out, prefix+"default")
		markDefault(out, prefix+"timeout_ms")
		markDefault(out, prefix+"fallback_chain_position")
		if b.Kind == "openai_compat" {
			markDefault(out, prefix+"auth_header_name")
			markDefault(out, prefix+"auth_header_value")
			markDefault(out, prefix+"remote_allowed")
		}
	}
	for task := range cfg.Routing {
		prefix := "routing." + task + "."
		markDefault(out, prefix+"fallback")
		markDefault(out, prefix+"num_ctx")
	}
}

func markDefault(out map[string]Provenance, key string) {
	if _, ok := out[key]; !ok {
		out[key] = ProvenanceDefault
	}
}
