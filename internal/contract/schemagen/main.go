package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type capabilities struct {
	Verbs        []verb              `json:"verbs"`
	ErrorCodes   map[string]codeInfo `json:"error_codes"`
	WarningCodes map[string]codeInfo `json:"warning_codes"`
}

type verb struct {
	Name               string           `json:"name"`
	NamespaceOnly      bool             `json:"namespace_only"`
	Summary            string           `json:"summary"`
	MegaCommand        string           `json:"mega_command"`
	Flags              []map[string]any `json:"flags"`
	Args               []map[string]any `json:"args"`
	ExitCodes          []int            `json:"exit_codes"`
	OutputSchemaRef    string           `json:"output_schema_ref"`
	EmitsDataOnFailure bool             `json:"emits_data_on_failure"`
}

type codeInfo struct {
	Status           string `json:"status"`
	ExitCode         *int   `json:"exit_code"`
	Retryable        *bool  `json:"retryable"`
	MessageTemplate  string `json:"message_template"`
	DetailsSchemaRef string `json:"details_schema_ref"`
}

func main() {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile("capabilities.golden.json")
	if err != nil {
		fatal(err)
	}
	var caps capabilities
	if err := json.Unmarshal(raw, &caps); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "errors.md"), []byte(renderErrors(caps)), 0o644); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "verbs.md"), []byte(renderVerbs(caps)), 0o644); err != nil {
		fatal(err)
	}
}

func renderErrors(caps capabilities) string {
	var b strings.Builder
	b.WriteString("# inferctl error catalog\n\n")
	b.WriteString("Generated from `internal/contract/capabilities.golden.json`. Regenerate with `go generate ./internal/contract`.\n\n")
	b.WriteString("## Errors\n\n")
	b.WriteString("| Code | Status | Exit | Retryable | Message | Details |\n")
	b.WriteString("|---|---:|---:|---:|---|---|\n")
	for _, code := range sortedCodes(caps.ErrorCodes) {
		info := caps.ErrorCodes[code]
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | `%s` |\n",
			code, info.Status, intPtr(info.ExitCode), boolPtr(info.Retryable), escapeTable(info.MessageTemplate), info.DetailsSchemaRef))
	}
	b.WriteString("\n## Warnings\n\n")
	b.WriteString("| Code | Status | Message | Details |\n")
	b.WriteString("|---|---:|---|---|\n")
	for _, code := range sortedCodes(caps.WarningCodes) {
		info := caps.WarningCodes[code]
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | `%s` |\n",
			code, info.Status, escapeTable(info.MessageTemplate), info.DetailsSchemaRef))
	}
	b.WriteString("\n## Examples\n\n")
	b.WriteString("- `infer doctr --json` emits `E_UNKNOWN_VERB` with `did_you_mean: \"infer doctor\"`.\n")
	b.WriteString("- `infer explain code --json` emits `E_VERB_RENAMED` with `did_you_mean: \"infer route code --json --explain\"`.\n")
	b.WriteString("- `infer version --check --json` emits `W_UPDATE_CHECK_FAILED` if the update endpoint cannot be reached and still exits 0.\n")
	return b.String()
}

func renderVerbs(caps capabilities) string {
	var b strings.Builder
	b.WriteString("# inferctl v0.1 verbs\n\n")
	b.WriteString("Generated from `internal/contract/capabilities.golden.json`. Regenerate with `go generate ./internal/contract`.\n\n")
	for _, verb := range caps.Verbs {
		b.WriteString("## `infer " + verb.Name + "`\n\n")
		b.WriteString(verb.Summary + "\n\n")
		if verb.NamespaceOnly {
			b.WriteString("Namespace only; use one of its subcommands.\n\n")
			continue
		}
		if verb.MegaCommand != "" {
			b.WriteString("- Mega-command: `" + verb.MegaCommand + "`\n")
		}
		if verb.OutputSchemaRef != "" {
			b.WriteString("- JSON data schema: `" + verb.OutputSchemaRef + "`\n")
		}
		b.WriteString("- Exit codes: `" + joinInts(verb.ExitCodes) + "`\n")
		b.WriteString(fmt.Sprintf("- Emits data on failure: `%v`\n\n", verb.EmitsDataOnFailure))
		if len(verb.Args) > 0 {
			b.WriteString("### Args\n\n")
			for _, arg := range verb.Args {
				b.WriteString(fmt.Sprintf("- `%v` required=%v\n", arg["name"], arg["required"]))
			}
			b.WriteString("\n")
		}
		if len(verb.Flags) > 0 {
			b.WriteString("### Flags\n\n")
			for _, flag := range verb.Flags {
				b.WriteString(fmt.Sprintf("- `%v` type=`%v` default=`%v`\n", flag["name"], flag["type"], flag["default"]))
			}
			b.WriteString("\n")
		}
		b.WriteString("### Example\n\n")
		b.WriteString("```sh\n")
		b.WriteString(exampleForVerb(verb.Name))
		b.WriteString("\n```\n\n")
	}
	return b.String()
}

func sortedCodes(codes map[string]codeInfo) []string {
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func intPtr(v *int) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(*v)
}

func boolPtr(v *bool) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(*v)
}

func escapeTable(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, "`, `")
}

func exampleForVerb(name string) string {
	switch name {
	case "doctor":
		return "infer doctor --json"
	case "backends":
		return "infer backends --json"
	case "models":
		return "infer models --json"
	case "model":
		return "infer model qwen3:8b --json"
	case "route":
		return "infer route code --prompt \"summarize this\" --json"
	case "config show":
		return "infer config show --json"
	case "config validate":
		return "infer config validate --json"
	case "config explain":
		return "infer config explain --key profile.mode --json"
	case "capabilities":
		return "infer capabilities --json"
	case "version":
		return "infer version --json"
	default:
		return "infer " + name + " --json"
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
