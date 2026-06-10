package config

type Config struct {
	Meta     MetaConfig               `toml:"meta" json:"meta"`
	Profile  ProfileConfig            `toml:"profile" json:"profile"`
	Backends map[string]BackendConfig `toml:"backends" json:"backends"`
	Routing  map[string]RoutingConfig `toml:"routing" json:"routing"`
}

type MetaConfig struct {
	SchemaVersion string `toml:"schema_version" json:"schema_version"`
}

type ProfileConfig struct {
	Name                string `toml:"name" json:"name"`
	MaxContextTokens    int    `toml:"max_context_tokens" json:"max_context_tokens"`
	MaxConcurrentModels int    `toml:"max_concurrent_models" json:"max_concurrent_models"`
	AllowPremium        bool   `toml:"allow_premium" json:"allow_premium"`
	Mode                string `toml:"mode" json:"mode"`
	VRAMTotalBytesHint  *int64 `toml:"vram_total_bytes_hint" json:"vram_total_bytes_hint"`
}

type BackendConfig struct {
	Kind                  string  `toml:"kind" json:"kind"`
	BaseURL               string  `toml:"base_url" json:"base_url"`
	Default               bool    `toml:"default" json:"default"`
	TimeoutMS             int     `toml:"timeout_ms" json:"timeout_ms"`
	FallbackChainPosition *int    `toml:"fallback_chain_position" json:"fallback_chain_position"`
	AuthHeaderName        *string `toml:"auth_header_name" json:"auth_header_name"`
	AuthHeaderValue       *string `toml:"auth_header_value" json:"auth_header_value"`
	RemoteAllowed         bool    `toml:"remote_allowed" json:"remote_allowed"`
}

type RoutingConfig struct {
	Model    string   `toml:"model" json:"model"`
	Backend  string   `toml:"backend" json:"backend"`
	Fallback []string `toml:"fallback" json:"fallback"`
	NumCtx   *int     `toml:"num_ctx" json:"num_ctx"`
}

func Defaults() Config {
	return Config{
		Meta: MetaConfig{SchemaVersion: "0.1"},
		Profile: ProfileConfig{
			Mode:         "warn",
			AllowPremium: false,
		},
		Backends: map[string]BackendConfig{},
		Routing:  map[string]RoutingConfig{},
	}
}

func applyBackendDefaults(b BackendConfig) BackendConfig {
	if b.TimeoutMS == 0 {
		b.TimeoutMS = 2000
	}
	return b
}

func applyRoutingDefaults(r RoutingConfig) RoutingConfig {
	if r.Fallback == nil {
		r.Fallback = []string{}
	}
	return r
}
