package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const defaultConfigScaffold = `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.ollama]
kind = "ollama"
base_url = "http://127.0.0.1:11434"
default = true

[routing.code]
model = "qwen3:8b"
backend = "ollama"
fallback = []
`

type configMutationResult struct {
	Path        *string  `json:"path"`
	Written     bool     `json:"written"`
	DryRun      bool     `json:"dry_run"`
	ChangedKeys []string `json:"changed_keys"`
	Preview     string   `json:"preview,omitempty"`
	Message     string   `json:"message"`
}

func newConfigInitCommand(jsonFlag *bool) *cobra.Command {
	var path string
	var force bool
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a starter inferctl config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.TrimSpace(path)
			if target == "" && !printOnly {
				target = filepath.Join(".", "inferctl.toml")
			}
			var targetPtr *string
			if target != "" {
				targetPtr = &target
			}
			data := configMutationResult{
				Path:        targetPtr,
				Written:     false,
				DryRun:      printOnly,
				ChangedKeys: []string{"meta.schema_version", "profile", "backends.ollama", "routing.code"},
				Preview:     defaultConfigScaffold,
				Message:     "config scaffold generated",
			}
			if !printOnly {
				if _, err := os.Stat(target); err == nil && !force {
					return writeError(cmd, *jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "config file already exists at "+target, map[string]any{"path": target, "reason": "exists"}))
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil && filepath.Dir(target) != "." {
					return writeError(cmd, *jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "could not create config directory for "+target, config.FileErrorDetails(err)))
				}
				if err := config.AtomicWriteFile(target, []byte(defaultConfigScaffold), 0o600); err != nil {
					return writeError(cmd, *jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "could not write config to "+target, config.FileErrorDetails(err)))
				}
				data.Written = true
				data.Message = "config file written"
			}
			return writeData(cmd, *jsonFlag, data, func() error {
				if printOnly {
					fmt.Fprint(cmd.OutOrStdout(), defaultConfigScaffold)
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", target)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "config path to write")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	cmd.Flags().BoolVar(&printOnly, "print", false, "print scaffold instead of writing")
	return cmd
}

func newConfigSetCommand(jsonFlag *bool) *cobra.Command {
	var path string
	var valueType string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one config key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			encoded, errObj := encodeConfigValue(args[0], args[1], valueType)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			return runConfigMutation(cmd, *jsonFlag, path, dryRun, []configEdit{{Key: args[0], EncodedValue: encoded}})
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "config path to edit")
	cmd.Flags().StringVar(&valueType, "type", "", "value type: string, int, bool, null, or array")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the edit without writing")
	return cmd
}

func newConfigPatchCommand(jsonFlag *bool) *cobra.Command {
	var path string
	var fromStdin bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "patch <toml-fragment>",
		Short: "Merge a TOML fragment into config",
		Args: func(cmd *cobra.Command, args []string) error {
			if fromStdin && len(args) > 0 {
				return writeError(cmd, *jsonFlag, invalidArg("patch_source", "multiple", "use either --from-stdin or an inline fragment", []string{"--from-stdin"}))
			}
			if !fromStdin && len(args) != 1 {
				return writeError(cmd, *jsonFlag, envelope.Error{
					Code:       "E_MISSING_ARG",
					Message:    "verb 'config patch' requires a TOML fragment or --from-stdin",
					DidYouMean: stringPtr("infer config patch --from-stdin"),
					ExitCode:   1,
					Retryable:  false,
					Details:    map[string]any{"verb": "config patch", "missing": "toml-fragment"},
				})
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var fragment string
			if fromStdin {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return writeError(cmd, *jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "could not read patch from stdin", map[string]any{"reason": err.Error()}))
				}
				fragment = string(data)
			} else {
				fragment = args[0]
			}
			edits, errObj := editsFromPatch(fragment)
			if errObj != nil {
				return writeError(cmd, *jsonFlag, *errObj)
			}
			return runConfigMutation(cmd, *jsonFlag, path, dryRun, edits)
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "config path to edit")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "read TOML fragment from stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the edit without writing")
	return cmd
}

type configEdit struct {
	Key          string
	EncodedValue string
}

func runConfigMutation(cmd *cobra.Command, jsonFlag bool, path string, dryRun bool, edits []configEdit) error {
	target, errObj := resolveMutationPath(path)
	if errObj != nil {
		return writeError(cmd, jsonFlag, *errObj)
	}
	original, err := os.ReadFile(target)
	if err != nil {
		return writeError(cmd, jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "could not read config at "+target, config.FileErrorDetails(err)))
	}
	updated := string(original)
	var changed []string
	for _, edit := range edits {
		var err error
		updated, err = applyConfigEdit(updated, edit)
		if err != nil {
			return writeError(cmd, jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", err.Error(), map[string]any{"key": edit.Key}))
		}
		changed = append(changed, edit.Key)
	}
	if err := validateConfigBytes([]byte(updated), target); err != nil {
		return writeError(cmd, jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "edited config failed validation", map[string]any{"path": target, "reason": err.Error()}))
	}
	redactedPreview := redactConfigText(updated)
	data := configMutationResult{
		Path:        &target,
		Written:     false,
		DryRun:      dryRun,
		ChangedKeys: changed,
		Preview:     redactedPreview,
		Message:     "config edit previewed",
	}
	if !dryRun {
		info, statErr := os.Stat(target)
		perm := os.FileMode(0o600)
		if statErr == nil {
			perm = info.Mode().Perm()
		}
		if err := config.AtomicWriteFile(target, []byte(updated), perm); err != nil {
			return writeError(cmd, jsonFlag, configWriteError("E_CONFIG_WRITE_FAILED", "could not write config to "+target, config.FileErrorDetails(err)))
		}
		data.Written = true
		data.Message = "config file updated"
	}
	return writeData(cmd, jsonFlag, data, func() error {
		if dryRun {
			fmt.Fprint(cmd.OutOrStdout(), redactedPreview)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", target)
		return nil
	})
}

func resolveMutationPath(path string) (string, *envelope.Error) {
	if strings.TrimSpace(path) != "" {
		return path, nil
	}
	result, err := (config.Loader{}).Load(config.LoadOptions{})
	if err != nil {
		errObj := configLoadError(err)
		return "", &errObj
	}
	if result.SourcePaths.Selected == nil {
		errObj := configWriteError("E_CONFIG_WRITE_FAILED", "no selected config path", map[string]any{})
		return "", &errObj
	}
	return *result.SourcePaths.Selected, nil
}

func encodeConfigValue(key, raw, valueType string) (string, *envelope.Error) {
	typ := strings.TrimSpace(valueType)
	if typ == "" {
		typ = inferValueType(raw)
	}
	switch typ {
	case "string":
		return strconv.Quote(raw), nil
	case "int":
		if _, err := strconv.Atoi(raw); err != nil {
			errObj := invalidArg("--type int", raw, "integer", nil)
			return "", &errObj
		}
		return raw, nil
	case "bool":
		if raw != "true" && raw != "false" {
			errObj := invalidArg("--type bool", raw, "true or false", []string{"true", "false"})
			return "", &errObj
		}
		return raw, nil
	case "null":
		if raw != "null" && raw != "" {
			errObj := invalidArg("--type null", raw, "null", []string{"null"})
			return "", &errObj
		}
		return "null", nil
	case "array":
		var v any
		if err := toml.Unmarshal([]byte("value = "+raw+"\n"), &v); err != nil || !strings.HasPrefix(strings.TrimSpace(raw), "[") {
			errObj := invalidArg("--type array", raw, "TOML array", nil)
			return "", &errObj
		}
		return raw, nil
	default:
		errObj := invalidArg("--type", typ, "one of string, int, bool, null, array", []string{"string", "int", "bool", "null", "array"})
		return "", &errObj
	}
}

func inferValueType(raw string) string {
	if raw == "true" || raw == "false" {
		return "bool"
	}
	if raw == "null" {
		return "null"
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return "int"
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return "array"
	}
	return "string"
}

func editsFromPatch(fragment string) ([]configEdit, *envelope.Error) {
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(fragment), &parsed); err != nil {
		errObj := configWriteError("E_CONFIG_WRITE_FAILED", "config patch is not valid TOML", map[string]any{"reason": err.Error()})
		errObj.ExitCode = 1
		return nil, &errObj
	}
	var edits []configEdit
	collectPatchEdits("", []byte(fragment), &edits)
	if len(edits) == 0 {
		errObj := configWriteError("E_CONFIG_PATCH_DELETE_UNSUPPORTED", "config patch contains no scalar assignments", map[string]any{"reason": "empty patch"})
		errObj.ExitCode = 1
		return nil, &errObj
	}
	return edits, nil
}

func collectPatchEdits(prefix string, fragment []byte, edits *[]configEdit) {
	table := prefix
	for _, line := range strings.Split(string(fragment), "\n") {
		trimmed := strings.TrimSpace(stripInlineComment(line))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			table = strings.Trim(trimmed, "[]")
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		fullKey := strings.TrimSpace(key)
		if table != "" {
			fullKey = table + "." + fullKey
		}
		*edits = append(*edits, configEdit{Key: fullKey, EncodedValue: strings.TrimSpace(value)})
	}
}

func applyConfigEdit(body string, edit configEdit) (string, error) {
	table, key := splitConfigEditKey(edit.Key)
	lines := strings.SplitAfter(body, "\n")
	start, end := tableRange(lines, table)
	if start == -1 {
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return body + "\n[" + table + "]\n" + key + " = " + edit.EncodedValue + "\n", nil
	}
	keyRE := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*)(.*?)(\s*(#.*)?\n?)$`)
	for i := start; i < end; i++ {
		if keyRE.MatchString(lines[i]) {
			lines[i] = keyRE.ReplaceAllString(lines[i], "${1}"+edit.EncodedValue+"${3}")
			return strings.Join(lines, ""), nil
		}
	}
	insert := key + " = " + edit.EncodedValue + "\n"
	lines = append(lines[:end], append([]string{insert}, lines[end:]...)...)
	return strings.Join(lines, ""), nil
}

func splitConfigEditKey(full string) (string, string) {
	parts := strings.Split(full, ".")
	if len(parts) < 2 {
		return "meta", full
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func tableRange(lines []string, table string) (int, int) {
	header := "[" + table + "]"
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(stripInlineComment(line))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if start != -1 {
				return start, i
			}
			if trimmed == header {
				start = i + 1
			}
		}
	}
	if start == -1 {
		return -1, -1
	}
	return start, len(lines)
}

func stripInlineComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if r == '\\' && inString {
			escaped = !escaped
			continue
		}
		if r == '"' && !escaped {
			inString = !inString
		}
		if r == '#' && !inString {
			return line[:i]
		}
		escaped = false
	}
	return line
}

func validateConfigBytes(data []byte, path string) error {
	source := config.SourcePaths{Selected: &path}
	result, err := config.LoadBytes(data, source, map[string]string{}, config.LoadOptions{})
	if err != nil {
		return err
	}
	validation := config.Validate(result, false)
	if validation.Summary.Errors > 0 {
		return fmt.Errorf("config validation found %d error(s)", validation.Summary.Errors)
	}
	return nil
}

func redactConfigText(body string) string {
	var out bytes.Buffer
	for _, line := range strings.SplitAfter(body, "\n") {
		keyPart, _, ok := strings.Cut(line, "=")
		if ok && secretKey(strings.TrimSpace(keyPart)) {
			prefix := keyPart + "= "
			if strings.Contains(keyPart, " =") {
				prefix = strings.Split(line, "=")[0] + "= "
			}
			comment := ""
			if idx := strings.Index(line, "#"); idx >= 0 {
				comment = " " + strings.TrimRight(line[idx:], "\n")
			}
			newline := ""
			if strings.HasSuffix(line, "\n") {
				newline = "\n"
			}
			out.WriteString(prefix + strconv.Quote("<redacted>") + comment + newline)
			continue
		}
		out.WriteString(line)
	}
	return out.String()
}

func secretKey(key string) bool {
	key = strings.ToLower(key)
	return key == "auth_header_value" ||
		strings.HasSuffix(key, "_token") ||
		strings.HasSuffix(key, "_secret") ||
		strings.HasSuffix(key, "_password") ||
		strings.HasSuffix(key, "_key")
}
