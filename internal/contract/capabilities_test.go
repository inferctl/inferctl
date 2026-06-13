package contract

import (
	"encoding/json"
	"os"
	"regexp"
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
	if invokable != 15 {
		t.Fatalf("invokable count = %d", invokable)
	}
	if !configNamespace {
		t.Fatal("config namespace entry missing")
	}
	if len(emitsDataOnFailure) != 1 || emitsDataOnFailure[0] != "config validate" {
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

func TestCapabilitiesDocsCoverCodesAndVerbs(t *testing.T) {
	raw, err := CapabilitiesRaw()
	if err != nil {
		t.Fatal(err)
	}
	var caps struct {
		Verbs []struct {
			Name string `json:"name"`
		} `json:"verbs"`
		ErrorCodes   map[string]any `json:"error_codes"`
		WarningCodes map[string]any `json:"warning_codes"`
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
		if !strings.Contains(verbsDoc, "## `infer "+verb.Name+"`") {
			t.Fatalf("docs/verbs.md missing verb %s", verb.Name)
		}
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

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
