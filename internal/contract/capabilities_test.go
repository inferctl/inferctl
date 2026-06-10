package contract

import "testing"

func TestCapabilitiesDataLoadsGolden(t *testing.T) {
	data, err := CapabilitiesData()
	if err != nil {
		t.Fatalf("CapabilitiesData() error = %v", err)
	}
	if data["tool"] != "inferctl" {
		t.Fatalf("tool = %v", data["tool"])
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
	if invokable != 10 {
		t.Fatalf("invokable count = %d", invokable)
	}
	if !configNamespace {
		t.Fatal("config namespace entry missing")
	}
	if len(emitsDataOnFailure) != 1 || emitsDataOnFailure[0] != "config validate" {
		t.Fatalf("emits_data_on_failure = %#v", emitsDataOnFailure)
	}
}
