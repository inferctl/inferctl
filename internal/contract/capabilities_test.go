package contract

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestCapabilitiesDataLoadsGolden(t *testing.T) {
	data, err := CapabilitiesData()
	if err != nil {
		t.Fatalf("CapabilitiesData() error = %v", err)
	}
	if data["tool"] != "inferctl" {
		t.Fatalf("tool = %v", data["tool"])
	}
	backendKinds, ok := data["backend_kinds"].([]any)
	if !ok {
		t.Fatalf("backend_kinds type = %T", data["backend_kinds"])
	}
	docs, ok := data["docs"].([]any)
	if !ok {
		t.Fatalf("docs type = %T", data["docs"])
	}
	if !mapListContainsPath(docs, "docs/agent-guide.md") {
		t.Fatal("capabilities docs missing docs/agent-guide.md")
	}
	if !mapListContainsPath(docs, "docs/install.md") {
		t.Fatal("capabilities docs missing docs/install.md")
	}
	examples, ok := data["examples"].(map[string]any)
	if !ok || examples["packaging"] != "source_only" {
		t.Fatalf("examples packaging = %#v", data["examples"])
	}
	for _, kind := range []string{"ollama", "llama.cpp", "openai_compat", "lmstudio", "mlx"} {
		if !mapListContainsName(backendKinds, kind) {
			t.Fatalf("backend_kinds missing %s", kind)
		}
	}
	verbs, ok := data["verbs"].([]any)
	if !ok {
		t.Fatalf("verbs type = %T", data["verbs"])
	}
	var invokable int
	var configNamespace bool
	var emitsDataOnFailure []string
	for _, raw := range verbs {
		verb := raw.(map[string]any)
		if verb["namespace_only"] == true {
			if verb["name"] == "config" {
				configNamespace = true
			}
			continue
		}
		invokable++
		if verb["emits_data_on_failure"] == true {
			emitsDataOnFailure = append(emitsDataOnFailure, verb["name"].(string))
		}
	}
	if invokable != 23 {
		t.Fatalf("invokable count = %d", invokable)
	}
	if !configNamespace {
		t.Fatal("config namespace entry missing")
	}
	if len(emitsDataOnFailure) != 2 || !slices.Contains(emitsDataOnFailure, "config validate") || !slices.Contains(emitsDataOnFailure, "preflight") {
		t.Fatalf("emits_data_on_failure = %#v", emitsDataOnFailure)
	}
}

func mapListContainsName(values []any, name string) bool {
	for _, raw := range values {
		value, ok := raw.(map[string]any)
		if ok && value["name"] == name {
			return true
		}
	}
	return false
}

func mapListContainsPath(values []any, path string) bool {
	for _, raw := range values {
		value, ok := raw.(map[string]any)
		if ok && value["path"] == path {
			return true
		}
	}
	return false
}

func TestAgentGuideCoversRequiredWorkflows(t *testing.T) {
	guide := readString(t, "../../docs/agent-guide.md")
	required := []string{
		"triage` does not run discovery inline",
		"openai_compat",
		"remote_allowed = true",
		"E_BACKEND_AUTH_FAILED",
		"E_BACKEND_REMOTE_NOT_ALLOWED",
		"source-only checkout artifacts",
		"Delivery metadata belongs in `data.delivery`",
		"LM Studio",
		"MLX",
	}
	for _, text := range required {
		if !strings.Contains(guide, text) {
			t.Fatalf("agent guide missing %q", text)
		}
	}
	readme := readString(t, "../../README.md")
	if !strings.Contains(readme, "(docs/agent-guide.md)") {
		t.Fatal("README does not link docs/agent-guide.md")
	}
}

func TestPackagingDocsAndScriptsMatchExamplesDecision(t *testing.T) {
	install := readString(t, "../../docs/install.md")
	required := []string{
		"go install github.com/inferctl/inferctl/cmd/inferctl@latest",
		"does not currently publish release binaries",
		"release binaries",
		"archives, installers, Homebrew formulae, or Scoop manifests",
		"No Windows installer, Scoop manifest, zip archive, or PATH mutation workflow",
		"examples/` scripts remain source-only checkout artifacts",
		"not packaged",
		"tool_version: \"0.2.2\"",
	}
	for _, text := range required {
		if !strings.Contains(install, text) {
			t.Fatalf("install docs missing %q", text)
		}
	}
	caps := readString(t, "capabilities.golden.json")
	if !strings.Contains(caps, "Go toolchain installation guidance") || !strings.Contains(caps, "Go-install-only launch posture") {
		t.Fatal("capabilities metadata does not describe Go-install-only packaging decision")
	}
}

func TestCapabilitiesDocsCoverCodesAndVerbs(t *testing.T) {
	raw, err := CapabilitiesRaw()
	if err != nil {
		t.Fatal(err)
	}
	var caps struct {
		Verbs []struct {
			Name string `json:"name"`
		} `json:"verbs"`
		ErrorCodes    map[string]any `json:"error_codes"`
		WarningCodes  map[string]any `json:"warning_codes"`
		Features      []string       `json:"features"`
		GlobalFlags   map[string]any `json:"global_flags"`
		Config        map[string]any `json:"config"`
		SchemasURI    string         `json:"schemas_uri"`
		RobotDocsURI  string         `json:"robot_docs_uri"`
		KnownNonGoals []string       `json:"known_non_goals"`
	}
	if err := json.Unmarshal(raw, &caps); err != nil {
		t.Fatal(err)
	}
	errorsDoc := readString(t, "../../docs/errors.md")
	verbsDoc := readString(t, "../../docs/verbs.md")
	for code := range caps.ErrorCodes {
		if !strings.Contains(errorsDoc, "`"+code+"`") {
			t.Fatalf("docs/errors.md missing error code %s", code)
		}
	}
	for code := range caps.WarningCodes {
		if !strings.Contains(errorsDoc, "`"+code+"`") {
			t.Fatalf("docs/errors.md missing warning code %s", code)
		}
	}
	for _, verb := range caps.Verbs {
		if !strings.Contains(verbsDoc, "## `inferctl "+verb.Name+"`") {
			t.Fatalf("docs/verbs.md missing verb %s", verb.Name)
		}
	}
	for _, feature := range []string{"json_envelope", "stderr_error_mirror", "did_you_mean", "config_provenance", "schema_export", "robot_docs_command"} {
		if !slices.Contains(caps.Features, feature) {
			t.Fatalf("capabilities missing feature %s", feature)
		}
	}
	if _, ok := caps.GlobalFlags["--json"]; !ok {
		t.Fatal("capabilities missing --json global flag")
	}
	if caps.Config["schema_uri"] != "inferctl config schema --json" || caps.Config["show_uri"] != "inferctl config show --json" {
		t.Fatalf("capabilities config metadata incomplete: %#v", caps.Config)
	}
	if caps.SchemasURI != "inferctl schema --json" || caps.RobotDocsURI != "inferctl robot-docs guide" || len(caps.KnownNonGoals) == 0 {
		t.Fatalf("capabilities missing schema/docs/non-goal metadata: %#v", caps)
	}

	docCodeRE := regexp.MustCompile("`([EW]_[A-Z0-9_]+)`")
	for _, match := range docCodeRE.FindAllStringSubmatch(errorsDoc, -1) {
		code := match[1]
		if _, ok := caps.ErrorCodes[code]; !ok {
			if _, ok := caps.WarningCodes[code]; !ok {
				t.Fatalf("docs/errors.md contains code absent from capabilities: %s", code)
			}
		}
	}
}

func TestContractGoldensUsePortablePathPlaceholders(t *testing.T) {
	for _, path := range []string{
		"../../testdata/contract/config-show.golden.json",
		"../../testdata/contract/config-validate.golden.json",
	} {
		body := readString(t, path)
		for _, concrete := range []string{"/tmp/", "/Users/", "/home/", `C:\`, `\\Users\\`} {
			if strings.Contains(body, concrete) {
				t.Fatalf("%s contains concrete path fragment %q", path, concrete)
			}
		}
		if !strings.Contains(body, "<config_path>") {
			t.Fatalf("%s should use <config_path> placeholder", path)
		}
	}
}

func TestConfigShowGoldenRedactsSecrets(t *testing.T) {
	body := readString(t, "../../testdata/contract/config-show.golden.json")
	if strings.Contains(body, "auth_header_value\":") {
		t.Fatalf("config-show golden should not expose auth_header_value in effective config")
	}
	if strings.Contains(body, "fixture-auth-value") {
		t.Fatalf("config-show golden leaked fixture auth value")
	}
	var golden struct {
		EffectiveConfig struct {
			Backends map[string]struct {
				BaseURL string `json:"base_url"`
			} `json:"backends"`
		} `json:"effective_config"`
	}
	if err := json.Unmarshal([]byte(body), &golden); err != nil {
		t.Fatal(err)
	}
	if golden.EffectiveConfig.Backends["ollama"].BaseURL != "http://127.0.0.1:11434" {
		t.Fatalf("config-show golden should retain backend base_url: %#v", golden.EffectiveConfig.Backends)
	}
}

func TestModelsGoldenIncludesLoadedState(t *testing.T) {
	body := readString(t, "../../testdata/contract/models.golden.json")
	var golden struct {
		Models []struct {
			Name      string `json:"name"`
			Backend   string `json:"backend"`
			Loaded    bool   `json:"loaded"`
			Available bool   `json:"available"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(body), &golden); err != nil {
		t.Fatal(err)
	}
	for _, row := range golden.Models {
		if row.Name == "qwen3:8b" && row.Backend == "ollama" && row.Loaded && row.Available {
			return
		}
	}
	t.Fatalf("models golden should include representative loaded/available row: %#v", golden.Models)
}

func TestRouteGoldenCoversFallbackCandidateFields(t *testing.T) {
	body := readString(t, "../../testdata/contract/route.golden.json")
	var golden struct {
		Decision struct {
			IsFallback bool   `json:"is_fallback"`
			Reason     string `json:"reason"`
		} `json:"decision"`
		Candidates []struct {
			Role                 string  `json:"role"`
			Available            bool    `json:"available"`
			UnavailabilityReason *string `json:"unavailability_reason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(body), &golden); err != nil {
		t.Fatal(err)
	}
	if !golden.Decision.IsFallback || golden.Decision.Reason == "" {
		t.Fatalf("route golden should exercise fallback decision reason: %#v", golden.Decision)
	}
	for _, candidate := range golden.Candidates {
		if candidate.Role == "primary" && !candidate.Available && candidate.UnavailabilityReason != nil && *candidate.UnavailabilityReason != "" {
			return
		}
	}
	t.Fatalf("route golden should include an unavailable candidate reason: %#v", golden.Candidates)
}

func TestDoctorGoldenIncludesRecommendedAction(t *testing.T) {
	body := readString(t, "../../testdata/contract/doctor.golden.json")
	var golden struct {
		RecommendedAction *struct {
			Command   string `json:"command"`
			Rationale string `json:"rationale"`
		} `json:"recommended_action"`
	}
	if err := json.Unmarshal([]byte(body), &golden); err != nil {
		t.Fatal(err)
	}
	if golden.RecommendedAction == nil || golden.RecommendedAction.Command == "" || golden.RecommendedAction.Rationale == "" {
		t.Fatalf("doctor golden should include recommended_action: %#v", golden.RecommendedAction)
	}
}

func TestStatusGoldenCoversAggregateSchema(t *testing.T) {
	body := readString(t, "../../testdata/contract/status.golden.json")
	var golden struct {
		StatusFrameSchemaVersion string `json:"status_frame_schema_version"`
		Summary                  struct {
			BackendsTotal      int `json:"backends_total"`
			ModelsExposedTotal int `json:"models_exposed_total"`
			ModelsLoadedTotal  int `json:"models_loaded_total"`
			RoutesTotal        int `json:"routes_total"`
			WarningsTotal      int `json:"warnings_total"`
		} `json:"summary"`
		Backends []struct {
			Name      string `json:"name"`
			Reachable bool   `json:"reachable"`
			BaseURL   string `json:"base_url"`
		} `json:"backends"`
		Models struct {
			Exposed []struct {
				Name      string `json:"name"`
				Backend   string `json:"backend"`
				Loaded    bool   `json:"loaded"`
				Available bool   `json:"available"`
			} `json:"exposed"`
			Loaded []struct {
				Name    string `json:"name"`
				Backend string `json:"backend"`
			} `json:"loaded"`
		} `json:"models"`
		Routes []struct {
			Task     string `json:"task"`
			Decision struct {
				SelectedModel string `json:"selected_model"`
				IsFallback    bool   `json:"is_fallback"`
				Reason        string `json:"reason"`
			} `json:"decision"`
		} `json:"routes"`
		Warnings          []map[string]any `json:"warnings"`
		RecommendedAction *struct {
			Command string `json:"command"`
		} `json:"recommended_action"`
	}
	if err := json.Unmarshal([]byte(body), &golden); err != nil {
		t.Fatal(err)
	}
	if golden.StatusFrameSchemaVersion == "" || golden.Summary.BackendsTotal == 0 || golden.Summary.ModelsExposedTotal == 0 ||
		golden.Summary.ModelsLoadedTotal == 0 || golden.Summary.RoutesTotal == 0 || golden.Summary.WarningsTotal == 0 {
		t.Fatalf("status golden summary should cover aggregate counts: %#v", golden.Summary)
	}
	if len(golden.Backends) == 0 || len(golden.Models.Exposed) == 0 || len(golden.Models.Loaded) == 0 ||
		len(golden.Routes) == 0 || len(golden.Warnings) == 0 || golden.RecommendedAction == nil || golden.RecommendedAction.Command == "" {
		t.Fatalf("status golden missing required aggregate sections")
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
