package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ozhiaki/inferctl/internal/config"
	"github.com/Ozhiaki/inferctl/internal/envelope"
	"github.com/Ozhiaki/inferctl/internal/render"
	"github.com/spf13/cobra"
)

func newConfigCommand(jsonFlag *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect inferctl configuration",
	}
	cmd.AddCommand(newConfigShowCommand(jsonFlag))
	cmd.AddCommand(newConfigValidateCommand(jsonFlag))
	cmd.AddCommand(newConfigExplainCommand(jsonFlag))
	cmd.AddCommand(newConfigInitCommand(jsonFlag))
	cmd.AddCommand(newConfigSetCommand(jsonFlag))
	cmd.AddCommand(newConfigPatchCommand(jsonFlag))
	return cmd
}

func newConfigShowCommand(jsonFlag *bool) *cobra.Command {
	var section string
	var key string
	var noProvenance bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective config with provenance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if section != "" && key != "" {
				return writeError(cmd, *jsonFlag, invalidArg("--section/--key", "both", "mutually exclusive flags", nil))
			}
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			data, err := configShowData(result, section, key, noProvenance)
			if err != nil {
				return writeError(cmd, *jsonFlag, invalidArg("selector", section+key, err.Error(), nil))
			}
			return writeData(cmd, *jsonFlag, data, func() error {
				if key != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s = %v\n", data["key"], data["value"])
					return nil
				}
				b, _ := json.MarshalIndent(data["effective_config"], "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&section, "section", "", "top-level section to show")
	cmd.Flags().StringVar(&key, "key", "", "dotted config key to show")
	cmd.Flags().BoolVar(&noProvenance, "no-provenance", false, "omit provenance map")
	return cmd
}

func newConfigValidateCommand(jsonFlag *bool) *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate config and report findings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := (config.Loader{}).Load(config.LoadOptions{})
			if err != nil {
				return writeError(cmd, *jsonFlag, configLoadError(err))
			}
			data := config.Validate(result, strict)
			if data.Passed {
				return writeData(cmd, *jsonFlag, data, func() error {
					fmt.Fprintf(cmd.OutOrStdout(), "config passed: %d error(s), %d warning(s)\n", data.Summary.Errors, data.Summary.Warnings)
					return nil
				})
			}
			errObj := validationFailedError(data)
			mode := render.SelectMode(render.Options{JSONFlag: *jsonFlag, Env: envMap()})
			if mode == render.ModeJSON {
				start := time.Now()
				env, err := envelope.New(toolVersion, data, envelope.Options{
					StartedAt:  start,
					FinishedAt: time.Now(),
					Env:        envMap(),
					Errors:     []envelope.Error{errObj},
				})
				if err != nil {
					return err
				}
				if err := render.WriteJSON(cmd.OutOrStdout(), env); err != nil {
					return err
				}
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), errObj.Message)
				for _, finding := range data.Findings {
					fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", finding.Severity, finding.Key, finding.Message)
				}
			}
			return exitError(1)
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as validation failure")
	return cmd
}

func configShowData(result *config.Result, section, key string, noProvenance bool) (map[string]any, error) {
	cfgMap, err := configAsMap(result.Config)
	if err != nil {
		return nil, err
	}
	if section != "" {
		value, ok := cfgMap[section]
		if !ok {
			return nil, fmt.Errorf("unknown section %q", section)
		}
		cfgMap = map[string]any{section: value}
	}
	if key != "" {
		value, ok := lookupDotted(cfgMap, key)
		if !ok {
			return nil, fmt.Errorf("unknown key %q", key)
		}
		prov := string(result.Provenance[key])
		return map[string]any{
			"source_paths": result.SourcePaths,
			"key":          key,
			"value":        value,
			"provenance":   prov,
			"type":         typeName(value),
		}, nil
	}
	data := map[string]any{
		"source_paths":     result.SourcePaths,
		"effective_config": cfgMap,
	}
	if !noProvenance {
		data["provenance"] = provenanceAsStrings(result.Provenance)
	}
	return data, nil
}

func configAsMap(cfg config.Config) (map[string]any, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func lookupDotted(m map[string]any, key string) (any, bool) {
	var current any = m
	for _, part := range strings.Split(key, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func provenanceAsStrings(in map[string]config.Provenance) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}

func typeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func configLoadError(err error) envelope.Error {
	loadErr, ok := err.(*config.LoadError)
	if !ok {
		return envelope.Error{
			Code:      "E_BINARY_INTERNAL",
			Message:   "internal error: " + err.Error(),
			ExitCode:  3,
			Retryable: false,
			Details:   map[string]any{"short_description": err.Error()},
		}
	}
	details := map[string]any{}
	message := loadErr.Reason
	did := (*string)(nil)
	exit := 3
	switch loadErr.Code {
	case "E_CONFIG_MISSING":
		message = "no config file found"
		did = stringPtr("infer config explain")
		details["searched_paths"] = loadErr.Searched
	case "E_CONFIG_INVALID":
		message = "config file at " + loadErr.Path + " failed to parse"
		did = stringPtr("infer config validate")
		details["path"] = loadErr.Path
		details["parse_error"] = loadErr.Reason
		details["line"] = loadErr.Line
		details["column"] = loadErr.Column
	case "E_CONFIG_UNREADABLE":
		message = "config file at " + loadErr.Path + " could not be read: " + loadErr.Reason
		details["path"] = loadErr.Path
		details["reason"] = loadErr.Reason
	case "E_INVALID_ARG":
		exit = 1
		message = loadErr.Reason
	}
	return envelope.Error{
		Code:       loadErr.Code,
		Message:    message,
		DidYouMean: did,
		ExitCode:   exit,
		Retryable:  false,
		Details:    details,
	}
}

func configWriteError(code, message string, details map[string]any) envelope.Error {
	if details == nil {
		details = map[string]any{}
	}
	return envelope.Error{
		Code:       code,
		Message:    message,
		DidYouMean: stringPtr("infer config validate --json"),
		ExitCode:   3,
		Retryable:  false,
		Details:    details,
	}
}

func invalidArg(arg, given, expected string, validSet []string) envelope.Error {
	details := map[string]any{
		"arg":       arg,
		"given":     given,
		"expected":  expected,
		"valid_set": validSet,
	}
	var did *string
	if len(validSet) > 0 {
		nearest, distance := nearestString(given, validSet)
		details["nearest"] = nearest
		details["distance"] = distance
		suggestion := arg + "=" + nearest
		did = &suggestion
	}
	return envelope.Error{
		Code:       "E_INVALID_ARG",
		Message:    fmt.Sprintf("invalid value for %s: %q (expected: %s)", arg, given, expected),
		DidYouMean: did,
		ExitCode:   1,
		Retryable:  false,
		Details:    details,
	}
}

func validationFailedError(data config.ValidationResult) envelope.Error {
	first := ""
	for _, finding := range data.Findings {
		if finding.Severity == "error" || first == "" {
			first = finding.Key
		}
		if finding.Severity == "error" {
			break
		}
	}
	did := "infer config validate --json"
	if first != "" {
		did = "infer config explain --key " + first + " --json"
	}
	return envelope.Error{
		Code:       "E_CONFIG_VALIDATION_FAILED",
		Message:    fmt.Sprintf("config validation found %d error(s) and %d warning(s)", data.Summary.Errors, data.Summary.Warnings),
		DidYouMean: &did,
		ExitCode:   1,
		Retryable:  false,
		Details: map[string]any{
			"errors":          data.Summary.Errors,
			"warnings":        data.Summary.Warnings,
			"first_error_key": first,
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
