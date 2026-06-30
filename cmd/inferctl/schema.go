package main

import (
	"fmt"
	"strings"

	"github.com/inferctl/inferctl/internal/contract"
	"github.com/spf13/cobra"
)

type schemaExport struct {
	Tool            string         `json:"tool"`
	Binary          string         `json:"binary"`
	ContractVersion string         `json:"contract_version"`
	SchemaVersion   string         `json:"schema_version"`
	Envelope        map[string]any `json:"envelope"`
	Schemas         map[string]any `json:"schemas"`
	Definitions     map[string]any `json:"definitions"`
}

func newSchemaCommand(jsonFlag *bool) *cobra.Command {
	var commandName string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Export JSON schemas for envelopes and verb data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			export, err := buildSchemaExport()
			if err != nil {
				return err
			}
			data := any(export)
			if commandName != "" {
				schema, ok := schemaForCommand(commandName, export)
				if !ok {
					return writeError(cmd, *jsonFlag, invalidArg("--command", commandName, "known command from capabilities", commandNamesFromSchemas(export)))
				}
				data = map[string]any{
					"tool":             export.Tool,
					"binary":           export.Binary,
					"contract_version": export.ContractVersion,
					"schema_version":   export.SchemaVersion,
					"command":          commandName,
					"schema":           schema,
				}
			}
			return writeData(cmd, *jsonFlag, data, func() error {
				fmt.Fprintln(cmd.OutOrStdout(), "inferctl schema")
				fmt.Fprintf(cmd.OutOrStdout(), "contract: %s schema: %s\n", export.ContractVersion, export.SchemaVersion)
				fmt.Fprintln(cmd.OutOrStdout(), "json: inferctl schema --json")
				if commandName != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "command: %s\n", commandName)
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&commandName, "command", "", "return one command data schema by command name")
	return cmd
}

func buildSchemaExport() (schemaExport, error) {
	data, err := contract.CapabilitiesData()
	if err != nil {
		return schemaExport{}, err
	}
	return schemaExport{
		Tool:            stringFromCapability(data, "tool"),
		Binary:          stringFromCapability(data, "binary"),
		ContractVersion: stringFromCapability(data, "contract_version"),
		SchemaVersion:   "0.1",
		Envelope:        envelopeJSONSchema(),
		Schemas:         commandSchemas(),
		Definitions:     schemaDefinitions(),
	}, nil
}

func stringFromCapability(data map[string]any, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func schemaForCommand(commandName string, export schemaExport) (map[string]any, bool) {
	key := schemaKeyForCommand(commandName)
	schema, ok := export.Schemas[key].(map[string]any)
	return schema, ok
}

func commandNamesFromSchemas(export schemaExport) []string {
	out := make([]string, 0, len(schemaCommandMap()))
	for command := range schemaCommandMap() {
		if _, ok := export.Schemas[schemaKeyForCommand(command)]; ok {
			out = append(out, command)
		}
	}
	return out
}

func schemaKeyForCommand(commandName string) string {
	if key, ok := schemaCommandMap()[commandName]; ok {
		return key
	}
	return strings.ReplaceAll(commandName, " ", "_")
}

func schemaCommandMap() map[string]string {
	return map[string]string{
		"doctor":           "doctor_report",
		"backends":         "backend_list",
		"models":           "model_list",
		"model":            "model_detail",
		"route":            "route_explanation",
		"preflight":        "preflight_report",
		"diff":             "diff_report",
		"snapshot":         "control_plane_snapshot",
		"status":           "status_frame",
		"config show":      "config_view",
		"config validate":  "config_validation",
		"config explain":   "config_explanation",
		"config init":      "config_mutation",
		"config set":       "config_mutation",
		"config patch":     "config_mutation",
		"config schema":    "config_file",
		"discover":         "discovery_report",
		"triage":           "triage_report",
		"capabilities":     "capabilities_manifest",
		"version":          "version_info",
		"schema":           "schema_export",
		"robot-docs guide": "robot_docs_guide",
	}
}

func envelopeJSONSchema() map[string]any {
	return objectSchema(
		[]string{"ok", "tool_version", "data", "meta", "warnings", "commands", "errors"},
		map[string]any{
			"ok":           map[string]any{"type": "boolean"},
			"tool_version": map[string]any{"type": "string"},
			"data":         map[string]any{"description": "Verb-specific payload, or null on error."},
			"meta":         map[string]any{"$ref": "#/definitions/meta"},
			"warnings":     arrayOf(map[string]any{"$ref": "#/definitions/warning"}),
			"commands":     arrayOf(map[string]any{"$ref": "#/definitions/command"}),
			"errors":       arrayOf(map[string]any{"$ref": "#/definitions/error"}),
		},
	)
}

func commandSchemas() map[string]any {
	return map[string]any{
		"doctor_report":     doctorReportSchema(),
		"backend_list":      backendListSchema(),
		"model_list":        modelListSchema(),
		"model_detail":      modelDetailSchema(),
		"route_explanation": routeExplanationSchema(),
		"preflight_report":  preflightReportSchema(),
		"diff_report":       diffReportSchema(),
		"control_plane_snapshot": map[string]any{
			"$ref": "#/definitions/control_plane_snapshot",
		},
		"status_frame": map[string]any{
			"$ref": "#/definitions/status_frame",
		},
		"status_event_batch": map[string]any{
			"$ref": "#/definitions/status_event_batch",
		},
		"config_view":        configViewSchema(),
		"config_validation":  configValidationSchema(),
		"config_explanation": configExplanationSchema(),
		"config_mutation":    configMutationSchema(),
		"config_file":        configFileJSONSchema(),
		"discovery_report":   discoveryReportSchema(),
		"triage_report":      triageReportSchema(),
		"version_info":       versionInfoSchema(),
		"capabilities_manifest": objectSchema([]string{"tool", "binary", "contract_version", "verbs", "exit_codes", "error_codes"}, map[string]any{
			"tool":             map[string]any{"type": "string"},
			"binary":           map[string]any{"type": "string"},
			"contract_version": map[string]any{"type": "string"},
			"features":         arrayOf(map[string]any{"type": "string"}),
			"verbs":            arrayOf(map[string]any{"type": "object"}),
			"exit_codes":       map[string]any{"type": "object"},
			"error_codes":      map[string]any{"type": "object"},
			"warning_codes":    map[string]any{"type": "object"},
		}),
		"schema_export": schemaExportSchema(),
		"robot_docs_guide": objectSchema([]string{"name", "format", "source", "content"}, map[string]any{
			"name":    map[string]any{"type": "string"},
			"format":  map[string]any{"const": "markdown"},
			"source":  map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}),
	}
}

func configFileJSONSchema() map[string]any {
	return objectSchemaStrict(
		[]string{"meta", "profile", "backends", "routing"},
		map[string]any{
			"meta": objectSchemaStrict([]string{"schema_version"}, map[string]any{
				"schema_version": map[string]any{"type": "string", "const": "0.1", "description": "Config schema version."},
			}),
			"profile": objectSchemaStrict(
				[]string{"name", "max_context_tokens", "max_concurrent_models", "allow_premium", "mode"},
				map[string]any{
					"name":                  map[string]any{"type": "string"},
					"max_context_tokens":    map[string]any{"type": "integer", "minimum": 1},
					"max_concurrent_models": map[string]any{"type": "integer", "minimum": 1},
					"allow_premium":         map[string]any{"type": "boolean"},
					"mode":                  map[string]any{"enum": []string{"strict", "warn", "advisory"}},
					"vram_total_bytes_hint": nullable("integer"),
				},
			),
			"backends": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"patternProperties": map[string]any{
					"^[A-Za-z0-9_.-]+$": objectSchemaStrict([]string{"kind", "base_url"}, map[string]any{
						"kind":                    map[string]any{"enum": []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"}},
						"base_url":                map[string]any{"type": "string", "format": "uri"},
						"default":                 map[string]any{"type": "boolean", "default": false},
						"timeout_ms":              map[string]any{"type": "integer", "minimum": 1, "default": 2000},
						"fallback_chain_position": nullable("integer"),
						"auth_header_name":        nullable("string"),
						"auth_header_value":       nullable("string"),
						"remote_allowed":          map[string]any{"type": "boolean", "default": false},
					}),
				},
			},
			"routing": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"patternProperties": map[string]any{
					"^[A-Za-z0-9_.-]+$": objectSchemaStrict([]string{"model", "backend"}, map[string]any{
						"model":    map[string]any{"type": "string"},
						"backend":  map[string]any{"type": "string"},
						"fallback": arrayOf(map[string]any{"type": "string"}),
						"num_ctx":  nullable("integer"),
					}),
				},
			},
		},
	)
}

func schemaDefinitions() map[string]any {
	return map[string]any{
		"meta": objectSchema([]string{"request_id", "ts_iso", "contract_version", "elapsed_ms", "data_hash"}, map[string]any{
			"request_id":       map[string]any{"type": "string"},
			"ts_iso":           map[string]any{"type": "string", "format": "date-time"},
			"contract_version": map[string]any{"type": "string"},
			"elapsed_ms":       map[string]any{"type": "integer", "minimum": 0},
			"data_hash":        nullable("string"),
			"search_mode":      nullable("string"),
			"fallback_tier":    nullable("string"),
			"fallback_reason":  nullable("string"),
		}),
		"warning": objectSchema([]string{"code", "message", "details"}, map[string]any{
			"code":    map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
			"details": map[string]any{"type": "object"},
		}),
		"command": objectSchema([]string{"label", "command", "rationale", "available_in_version"}, map[string]any{
			"label":                map[string]any{"type": "string"},
			"command":              map[string]any{"type": "string"},
			"rationale":            map[string]any{"type": "string"},
			"available_in_version": nullable("string"),
		}),
		"error": objectSchema([]string{"code", "message", "did_you_mean", "exit_code", "retryable", "details"}, map[string]any{
			"code":         map[string]any{"type": "string"},
			"message":      map[string]any{"type": "string"},
			"did_you_mean": nullable("string"),
			"exit_code":    map[string]any{"type": "integer"},
			"retryable":    map[string]any{"type": "boolean"},
			"details":      map[string]any{"type": "object"},
		}),
		"backend_status": objectSchema([]string{"name", "kind", "base_url", "reachable", "default"}, map[string]any{
			"name":                   map[string]any{"type": "string"},
			"kind":                   map[string]any{"type": "string"},
			"base_url":               map[string]any{"type": "string"},
			"reachable":              map[string]any{"type": "boolean"},
			"latency_ms":             nullable("integer"),
			"version":                nullable("string"),
			"default":                map[string]any{"type": "boolean"},
			"models_installed_count": nullable("integer"),
			"models_loaded_count":    nullable("integer"),
			"backoff":                nullable("object"),
		}),
		"model_info": objectSchema([]string{"name", "backend"}, map[string]any{
			"name":             map[string]any{"type": "string"},
			"backend":          map[string]any{"type": "string"},
			"size_bytes":       nullable("integer"),
			"digest":           nullable("string"),
			"installed_at_iso": nullable("string"),
			"loaded":           map[string]any{"type": "boolean"},
			"available":        map[string]any{"type": "boolean"},
		}),
		"loaded_model_info": objectSchema([]string{"name", "backend"}, map[string]any{
			"name":                  map[string]any{"type": "string"},
			"backend":               map[string]any{"type": "string"},
			"vram_bytes":            nullable("integer"),
			"kv_cache_pressure_pct": nullable("number"),
			"loaded_at_iso":         nullable("string"),
			"idle_seconds":          nullable("integer"),
		}),
		"prompt_metadata": objectSchema([]string{"source_kind", "source", "prompt_chars", "estimated_tokens"}, map[string]any{
			"source_kind":      map[string]any{"enum": []string{"none", "inline", "file", "stdin"}},
			"source":           map[string]any{"type": "string", "description": "Redacted prompt source label; file sources do not include local paths."},
			"prompt_chars":     map[string]any{"type": "integer", "minimum": 0},
			"estimated_tokens": map[string]any{"type": "integer", "minimum": 0},
			"content_sha256":   nullable("string"),
			"filename":         nullable("string"),
			"basename":         nullable("string"),
		}),
		"backend_reachability": objectSchema([]string{"name", "kind", "reachable", "base_url"}, map[string]any{
			"name":      map[string]any{"type": "string"},
			"kind":      map[string]any{"type": "string"},
			"reachable": map[string]any{"type": "boolean"},
			"base_url":  map[string]any{"type": "string"},
			"error":     nullable("string"),
		}),
		"control_plane_snapshot": objectSchema([]string{
			"snapshot_schema_version",
			"contract_version",
			"inferctl_version",
			"captured_at_iso",
			"task",
			"prompt",
			"route_decision",
			"route_candidates",
			"backend_reachability",
			"loaded_models",
			"installed_models",
			"warnings",
			"errors",
		}, map[string]any{
			"snapshot_schema_version": map[string]any{"type": "string"},
			"contract_version":        map[string]any{"type": "string"},
			"inferctl_version":        map[string]any{"type": "string"},
			"captured_at_iso":         map[string]any{"type": "string", "format": "date-time"},
			"task":                    map[string]any{"type": "string"},
			"prompt":                  map[string]any{"$ref": "#/definitions/prompt_metadata"},
			"route_decision":          map[string]any{"type": "object"},
			"route_candidates":        arrayOf(map[string]any{"type": "object"}),
			"backend_reachability":    arrayOf(map[string]any{"$ref": "#/definitions/backend_reachability"}),
			"loaded_models":           arrayOf(map[string]any{"$ref": "#/definitions/loaded_model_info"}),
			"installed_models":        arrayOf(map[string]any{"$ref": "#/definitions/model_info"}),
			"warnings":                arrayOf(map[string]any{"$ref": "#/definitions/warning"}),
			"errors":                  arrayOf(map[string]any{"$ref": "#/definitions/error"}),
			"recommended_action":      nullable("object"),
		}),
		"status_backend": objectSchema([]string{"name", "kind", "base_url", "reachable", "default", "error"}, map[string]any{
			"name":                   map[string]any{"type": "string"},
			"kind":                   map[string]any{"type": "string"},
			"base_url":               map[string]any{"type": "string"},
			"reachable":              map[string]any{"type": "boolean"},
			"default":                map[string]any{"type": "boolean"},
			"models_installed_count": nullable("integer"),
			"models_loaded_count":    nullable("integer"),
			"error":                  nullable("string"),
		}),
		"status_frame": objectSchema([]string{
			"status_frame_schema_version",
			"contract_version",
			"summary",
			"backends",
			"models",
			"routes",
			"warnings",
			"recommended_action",
		}, map[string]any{
			"status_frame_schema_version": map[string]any{"type": "string"},
			"contract_version":            map[string]any{"type": "string"},
			"summary":                     map[string]any{"type": "object"},
			"backends":                    arrayOf(map[string]any{"$ref": "#/definitions/status_backend"}),
			"models": objectSchema([]string{"exposed", "loaded"}, map[string]any{
				"exposed": arrayOf(map[string]any{"$ref": "#/definitions/model_info"}),
				"loaded":  arrayOf(map[string]any{"$ref": "#/definitions/loaded_model_info"}),
			}),
			"routes":             arrayOf(map[string]any{"type": "object"}),
			"warnings":           arrayOf(map[string]any{"$ref": "#/definitions/warning"}),
			"recommended_action": nullable("object"),
		}),
		"status_backend_reachability": objectSchema([]string{"name", "kind", "reachable", "error"}, map[string]any{
			"name":      map[string]any{"type": "string"},
			"kind":      map[string]any{"type": "string"},
			"reachable": map[string]any{"type": "boolean"},
			"error":     nullable("string"),
		}),
		"status_route_selection": objectSchema([]string{"task", "selected_backend", "selected_model", "is_fallback", "fallback_index", "ready"}, map[string]any{
			"task":             map[string]any{"type": "string"},
			"selected_backend": map[string]any{"type": "string"},
			"selected_model":   map[string]any{"type": "string"},
			"is_fallback":      map[string]any{"type": "boolean"},
			"fallback_index":   nullable("integer"),
			"ready":            map[string]any{"type": "boolean"},
		}),
		"status_event": objectSchema([]string{"sequence", "kind", "subject", "severity", "summary", "before", "after"}, map[string]any{
			"sequence": map[string]any{"type": "integer", "minimum": 1},
			"kind": map[string]any{"enum": []string{
				"backend_reachability_changed",
				"error_codes_changed",
				"fallback_status_changed",
				"loaded_model_count_changed",
				"recommended_action_changed",
				"selected_model_readiness_changed",
				"selected_route_changed",
				"warning_codes_changed",
			}},
			"subject":  map[string]any{"type": "string"},
			"severity": map[string]any{"enum": []string{"high", "medium", "low"}},
			"summary":  map[string]any{"type": "string"},
			"before": map[string]any{"oneOf": []map[string]any{
				{"$ref": "#/definitions/status_backend_reachability"},
				{"$ref": "#/definitions/status_route_selection"},
			}},
			"after": map[string]any{"oneOf": []map[string]any{
				{"$ref": "#/definitions/status_backend_reachability"},
				{"$ref": "#/definitions/status_route_selection"},
			}},
		}),
		"status_event_batch": objectSchema([]string{
			"event_schema_version",
			"contract_version",
			"captured_at_iso",
			"since_captured_at_iso",
			"events",
		}, map[string]any{
			"event_schema_version": map[string]any{"type": "string"},
			"contract_version":     map[string]any{"type": "string"},
			"captured_at_iso":      map[string]any{"type": "string", "format": "date-time"},
			"since_captured_at_iso": map[string]any{
				"type":   "string",
				"format": "date-time",
			},
			"events": arrayOf(map[string]any{"$ref": "#/definitions/status_event"}),
		}),
		"control_plane_change": objectSchema([]string{"rank", "type", "subject", "severity", "before", "after", "explanation"}, map[string]any{
			"rank":        map[string]any{"type": "integer", "minimum": 1},
			"type":        map[string]any{"type": "string"},
			"subject":     map[string]any{"type": "string"},
			"severity":    map[string]any{"enum": []string{"high", "medium", "low"}},
			"before":      true,
			"after":       true,
			"explanation": map[string]any{"type": "string"},
		}),
		"finding": objectSchema([]string{"severity", "key", "message", "line", "column", "details"}, map[string]any{
			"severity": map[string]any{"enum": []string{"error", "warning", "info"}},
			"key":      map[string]any{"type": "string"},
			"message":  map[string]any{"type": "string"},
			"line":     nullable("integer"),
			"column":   nullable("integer"),
			"details":  map[string]any{"type": "object"},
		}),
	}
}

func doctorReportSchema() map[string]any {
	return objectSchema([]string{"summary", "backends", "loaded_models", "routes", "system", "warnings", "recommended_action"}, map[string]any{
		"summary":            map[string]any{"type": "object"},
		"backends":           arrayOf(map[string]any{"$ref": "#/definitions/backend_status"}),
		"loaded_models":      arrayOf(map[string]any{"$ref": "#/definitions/loaded_model_info"}),
		"routes":             arrayOf(map[string]any{"type": "object"}),
		"system":             map[string]any{"type": "object"},
		"warnings":           arrayOf(map[string]any{"$ref": "#/definitions/warning"}),
		"recommended_action": nullable("object"),
	})
}

func backendListSchema() map[string]any {
	return objectSchema([]string{"backends", "total_count", "reachable_count"}, map[string]any{
		"backends":        arrayOf(map[string]any{"$ref": "#/definitions/backend_status"}),
		"total_count":     map[string]any{"type": "integer", "minimum": 0},
		"reachable_count": map[string]any{"type": "integer", "minimum": 0},
	})
}

func modelListSchema() map[string]any {
	return objectSchema([]string{"models", "total_count", "loaded_count"}, map[string]any{
		"models":       arrayOf(map[string]any{"$ref": "#/definitions/model_info"}),
		"total_count":  map[string]any{"type": "integer", "minimum": 0},
		"loaded_count": map[string]any{"type": "integer", "minimum": 0},
	})
}

func modelDetailSchema() map[string]any {
	return objectSchema([]string{"name", "backends", "capabilities", "latency_stats", "routing"}, map[string]any{
		"name":          map[string]any{"type": "string"},
		"backends":      arrayOf(map[string]any{"type": "object"}),
		"capabilities":  map[string]any{"type": "object"},
		"latency_stats": map[string]any{"type": "object"},
		"routing":       map[string]any{"type": "object"},
	})
}

func routeExplanationSchema() map[string]any {
	return objectSchema([]string{"task", "input", "decision", "candidates", "constraints"}, map[string]any{
		"task":        map[string]any{"type": "string"},
		"input":       map[string]any{"type": "object"},
		"decision":    map[string]any{"type": "object"},
		"candidates":  arrayOf(map[string]any{"type": "object"}),
		"constraints": map[string]any{"type": "object"},
	})
}

func preflightReportSchema() map[string]any {
	return objectSchema([]string{"preflight_schema_version", "task", "runnable", "runnability_status", "prompt", "route", "route_decision", "route_candidates", "constraints", "runnability", "policy", "summary", "warnings", "recommended_action"}, map[string]any{
		"preflight_schema_version": map[string]any{"type": "string"},
		"task":                     map[string]any{"type": "string"},
		"runnable":                 map[string]any{"type": "boolean"},
		"runnability_status":       map[string]any{"type": "string"},
		"prompt":                   map[string]any{"$ref": "#/definitions/prompt_metadata"},
		"route":                    map[string]any{"$ref": "#/schemas/route_explanation"},
		"route_decision":           map[string]any{"type": "object"},
		"route_candidates":         arrayOf(map[string]any{"type": "object"}),
		"constraints":              map[string]any{"type": "object"},
		"runnability":              map[string]any{"type": "object"},
		"policy":                   map[string]any{"type": "object"},
		"summary":                  map[string]any{"type": "object"},
		"warnings":                 arrayOf(map[string]any{"$ref": "#/definitions/warning"}),
		"recommended_action":       nullable("object"),
	})
}

func diffReportSchema() map[string]any {
	return objectSchema([]string{"before", "after", "summary", "changes"}, map[string]any{
		"before":  map[string]any{"type": "object"},
		"after":   map[string]any{"type": "object"},
		"summary": map[string]any{"type": "object"},
		"changes": arrayOf(map[string]any{"$ref": "#/definitions/control_plane_change"}),
	})
}

func configViewSchema() map[string]any {
	return objectSchema([]string{"source_paths"}, map[string]any{
		"source_paths":     map[string]any{"type": "object"},
		"effective_config": map[string]any{"type": "object"},
		"provenance":       map[string]any{"type": "object"},
		"key":              map[string]any{"type": "string"},
		"value":            true,
		"type":             map[string]any{"type": "string"},
	})
}

func configValidationSchema() map[string]any {
	return objectSchema([]string{"source_path", "findings", "summary", "passed"}, map[string]any{
		"source_path": nullable("string"),
		"findings":    arrayOf(map[string]any{"$ref": "#/definitions/finding"}),
		"summary":     map[string]any{"type": "object"},
		"passed":      map[string]any{"type": "boolean"},
	})
}

func configExplanationSchema() map[string]any {
	return objectSchema([]string{"format", "annotated_source", "keys", "schema_version"}, map[string]any{
		"format":           map[string]any{"enum": []string{"toml", "md"}},
		"annotated_source": map[string]any{"type": "string"},
		"keys":             arrayOf(map[string]any{"type": "object"}),
		"schema_version":   map[string]any{"type": "string"},
	})
}

func configMutationSchema() map[string]any {
	return objectSchema([]string{"path", "written", "dry_run", "changed_keys", "preview", "message"}, map[string]any{
		"path":         nullable("string"),
		"written":      map[string]any{"type": "boolean"},
		"dry_run":      map[string]any{"type": "boolean"},
		"changed_keys": arrayOf(map[string]any{"type": "string"}),
		"preview":      map[string]any{"type": "string"},
		"message":      map[string]any{"type": "string"},
	})
}

func discoveryReportSchema() map[string]any {
	return objectSchema([]string{"summary", "scan", "candidates", "delivery"}, map[string]any{
		"summary":    map[string]any{"type": "object"},
		"scan":       map[string]any{"type": "object"},
		"candidates": arrayOf(map[string]any{"type": "object"}),
		"delivery":   nullable("object"),
	})
}

func triageReportSchema() map[string]any {
	return objectSchema([]string{"summary", "inputs", "items", "recommended_action"}, map[string]any{
		"summary":            map[string]any{"type": "object"},
		"inputs":             arrayOf(map[string]any{"type": "object"}),
		"items":              arrayOf(map[string]any{"type": "object"}),
		"recommended_action": nullable("object"),
	})
}

func versionInfoSchema() map[string]any {
	return objectSchema([]string{"tool_version", "build", "dependencies", "contract_version", "schema_version", "update"}, map[string]any{
		"tool_version":     map[string]any{"type": "string"},
		"build":            map[string]any{"type": "object"},
		"dependencies":     map[string]any{"type": "object"},
		"contract_version": map[string]any{"type": "string"},
		"schema_version":   map[string]any{"type": "string"},
		"update":           map[string]any{"type": "object"},
	})
}

func schemaExportSchema() map[string]any {
	return objectSchema([]string{"tool", "binary", "contract_version", "schema_version", "envelope", "schemas", "definitions"}, map[string]any{
		"tool":             map[string]any{"type": "string"},
		"binary":           map[string]any{"type": "string"},
		"contract_version": map[string]any{"type": "string"},
		"schema_version":   map[string]any{"type": "string"},
		"envelope":         map[string]any{"type": "object"},
		"schemas":          map[string]any{"type": "object"},
		"definitions":      map[string]any{"type": "object"},
	})
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"required":             required,
		"properties":           properties,
		"additionalProperties": true,
	}
}

func objectSchemaStrict(required []string, properties map[string]any) map[string]any {
	schema := objectSchema(required, properties)
	schema["additionalProperties"] = false
	return schema
}

func arrayOf(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func nullable(typ string) map[string]any {
	return map[string]any{"type": []string{typ, "null"}}
}
