package main

import (
	"fmt"
	"strings"

	"github.com/Ozhiaki/inferctl/pkg/inferctl"
	"github.com/spf13/cobra"
)

type configExplainData struct {
	Format          string                  `json:"format"`
	AnnotatedSource string                  `json:"annotated_source"`
	Keys            []inferctl.ConfigKeyDef `json:"keys"`
	SchemaVersion   string                  `json:"schema_version"`
}

func newConfigExplainCommand(jsonFlag *bool) *cobra.Command {
	var key string
	var format string
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain config keys and print an annotated template",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "toml" && format != "md" {
				return writeError(cmd, *jsonFlag, invalidArg("--format", format, "one of toml, md", []string{"toml", "md"}))
			}
			keys, err := filterConfigKeys(configKeyCatalog(), key)
			if err != nil {
				return writeError(cmd, *jsonFlag, invalidArg("--key", key, err.Error(), nil))
			}
			data := configExplainData{
				Format:          format,
				AnnotatedSource: annotatedConfigSource(format, keys),
				Keys:            keys,
				SchemaVersion:   "0.1",
			}
			return writeData(cmd, *jsonFlag, data, func() error {
				fmt.Fprint(cmd.OutOrStdout(), data.AnnotatedSource)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "dotted config key or wildcard section to explain")
	cmd.Flags().StringVar(&format, "format", "toml", "annotated source format: toml or md")
	return cmd
}

func configKeyCatalog() []inferctl.ConfigKeyDef {
	return []inferctl.ConfigKeyDef{
		keyDef("meta.schema_version", "string", true, nil, "Config schema version. v0.1 requires \"0.1\" when present.", nil, "0.1"),
		keyDef("profile.name", "string", true, nil, "Free-form profile identifier shown in doctor output.", nil, "default_local_workstation"),
		keyDef("profile.max_context_tokens", "int", true, nil, "Maximum prompt context budget used by route warnings.", nil, 8192),
		keyDef("profile.max_concurrent_models", "int", true, nil, "Maximum loaded models the profile expects at once.", nil, 1),
		keyDef("profile.allow_premium", "bool", true, nil, "Whether future routing may select premium-tagged models.", nil, false),
		keyDef("profile.mode", "enum string", true, nil, "Profile enforcement mode. v0.1 recognizes all values but enforces warn semantics.", []string{"strict", "warn", "advisory"}, "warn"),
		keyDef("profile.vram_total_bytes_hint", "int|null", false, nil, "Optional VRAM capacity hint used by doctor when platform probing is unavailable.", nil, 25769803776),
		keyDef("backends.<name>.kind", "enum string", true, nil, "Backend adapter kind.", []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"}, "ollama"),
		keyDef("backends.<name>.base_url", "string", true, nil, "URL the backend is reachable at. Must include scheme and port.", nil, "http://127.0.0.1:11434"),
		keyDef("backends.<name>.default", "bool", false, false, "True for the default backend. Exactly one backend should set true.", nil, true),
		keyDef("backends.<name>.timeout_ms", "int", false, 2000, "HTTP probe timeout for this backend in milliseconds.", nil, 2000),
		keyDef("backends.<name>.fallback_chain_position", "int|null", false, nil, "Optional ordering hint for future backend fallback behavior.", nil, 1),
		keyDef("backends.<name>.auth_header_name", "string|null", false, nil, "Reserved for authenticated openai_compat backends; v0.1 warns when set.", nil, "Authorization"),
		keyDef("backends.<name>.auth_header_value", "string|null", false, nil, "Reserved for authenticated openai_compat backends; v0.1 warns when set.", nil, "Bearer token"),
		keyDef("backends.<name>.remote_allowed", "bool", false, false, "Whether a remote backend is allowed. v0.1 expects false.", nil, false),
		keyDef("routing.<task>.model", "string", true, nil, "Primary model name for this task.", nil, "qwen3:8b"),
		keyDef("routing.<task>.backend", "string", true, nil, "Configured backend name for the primary model.", nil, "ollama"),
		keyDef("routing.<task>.fallback", "[]string", false, []string{}, "Fallback model names tried in order.", nil, []string{"qwen3:4b"}),
		keyDef("routing.<task>.num_ctx", "int|null", false, nil, "Optional task-specific context window override.", nil, 4096),
	}
}

func keyDef(key, typ string, required bool, defaultValue any, description string, validSet []string, example any) inferctl.ConfigKeyDef {
	return inferctl.ConfigKeyDef{
		Key:         key,
		Type:        typ,
		Required:    required,
		Default:     defaultValue,
		Description: description,
		ValidSet:    validSet,
		Example:     example,
	}
}

func filterConfigKeys(keys []inferctl.ConfigKeyDef, selector string) ([]inferctl.ConfigKeyDef, error) {
	if selector == "" {
		return keys, nil
	}
	if strings.HasSuffix(selector, ".*") {
		prefix := strings.TrimSuffix(selector, "*")
		var out []inferctl.ConfigKeyDef
		for _, key := range keys {
			if strings.HasPrefix(key.Key, prefix) {
				out = append(out, key)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("unknown config key wildcard")
		}
		return out, nil
	}
	for _, key := range keys {
		if key.Key == selector {
			return []inferctl.ConfigKeyDef{key}, nil
		}
	}
	return nil, fmt.Errorf("unknown config key")
}

func annotatedConfigSource(format string, keys []inferctl.ConfigKeyDef) string {
	if format == "md" {
		return annotatedConfigMarkdown(keys)
	}
	return annotatedConfigTOML(keys)
}

func annotatedConfigTOML(keys []inferctl.ConfigKeyDef) string {
	var b strings.Builder
	b.WriteString("# inferctl config\n")
	b.WriteString("# Save as ~/.config/inferctl/config.toml or set INFERCTL_CONFIG.\n\n")
	sections := map[string][]inferctl.ConfigKeyDef{}
	var order []string
	for _, key := range keys {
		section, _ := splitConfigKey(key.Key)
		if _, ok := sections[section]; !ok {
			order = append(order, section)
		}
		sections[section] = append(sections[section], key)
	}
	for _, section := range order {
		b.WriteString("[" + section + "]\n")
		for _, key := range sections[section] {
			_, name := splitConfigKey(key.Key)
			b.WriteString("# " + name + ": " + key.Description + "\n")
			b.WriteString(name + " = " + tomlExampleValue(key) + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func annotatedConfigMarkdown(keys []inferctl.ConfigKeyDef) string {
	var b strings.Builder
	b.WriteString("# inferctl config\n\n")
	for _, key := range keys {
		required := "optional"
		if key.Required {
			required = "required"
		}
		b.WriteString("## `" + key.Key + "`\n\n")
		b.WriteString("- Type: `" + key.Type + "`\n")
		b.WriteString("- Required: " + required + "\n")
		if len(key.ValidSet) > 0 {
			b.WriteString("- Valid values: `" + strings.Join(key.ValidSet, "`, `") + "`\n")
		}
		b.WriteString("- Description: " + key.Description + "\n")
		b.WriteString("- Example: `" + fmt.Sprint(key.Example) + "`\n\n")
	}
	return b.String()
}

func splitConfigKey(key string) (string, string) {
	index := strings.LastIndex(key, ".")
	if index < 0 {
		return key, key
	}
	return key[:index], key[index+1:]
}

func tomlExampleValue(key inferctl.ConfigKeyDef) string {
	value := key.Example
	if value == nil {
		value = key.Default
	}
	switch v := value.(type) {
	case string:
		return `"` + v + `"`
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprint(v)
	case int64:
		return fmt.Sprint(v)
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		quoted := make([]string, 0, len(v))
		for _, item := range v {
			quoted = append(quoted, `"`+item+`"`)
		}
		return "[" + strings.Join(quoted, ", ") + "]"
	default:
		return "null"
	}
}
