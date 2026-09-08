package inferctl

import "testing"

func TestSelectRouteCandidateUsesEvidence(t *testing.T) {
	candidates := []RouteCandidate{{Model: "first", Role: "primary", Available: true, Capabilities: map[string]CapabilityEvidence{"tools": {Status: "unknown", Source: "declared"}}}, {Model: "second", Role: "fallback", Available: true, Loaded: true, Capabilities: map[string]CapabilityEvidence{"tools": {Status: "supported", Source: "observed"}}}}
	selected, refusal := SelectRouteCandidate(candidates, RouteRequirements{Version: RouteRequirementsVersionV1, RequiredCapabilities: []string{"tools"}, AllowFallback: true})
	if refusal != nil || selected == nil || selected.Model != "second" {
		t.Fatalf("selected=%#v refusal=%#v", selected, refusal)
	}
	selected, refusal = SelectRouteCandidate(candidates[:1], RouteRequirements{Version: RouteRequirementsVersionV1, RequiredCapabilities: []string{"tools"}, AllowFallback: true})
	if selected != nil || refusal == nil || refusal.Code != "E_ROUTE_REQUIREMENTS_UNSATISFIED" {
		t.Fatalf("selected=%#v refusal=%#v", selected, refusal)
	}
}
