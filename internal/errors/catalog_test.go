package errors

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCatalogMatchesCapabilitiesGolden(t *testing.T) {
	data, err := os.ReadFile("../contract/capabilities.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var caps struct {
		ErrorCodes map[string]struct {
			Status string `json:"status"`
		} `json:"error_codes"`
		WarningCodes map[string]struct {
			Status string `json:"status"`
		} `json:"warning_codes"`
	}
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatal(err)
	}
	for _, code := range ActiveErrorCodes {
		status := caps.ErrorCodes[code].Status
		if status != "v0.1" && status != "v0.2" {
			t.Fatalf("active error code %s missing or non-active in capabilities", code)
		}
	}
	for _, code := range ReservedErrorCodes {
		if caps.ErrorCodes[code].Status != "reserved" {
			t.Fatalf("reserved error code %s missing or non-reserved in capabilities", code)
		}
	}
	for _, code := range ActiveWarningCodes {
		if caps.WarningCodes[code].Status != "v0.1" {
			t.Fatalf("active warning code %s missing or non-active in capabilities", code)
		}
	}
}

func index(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
