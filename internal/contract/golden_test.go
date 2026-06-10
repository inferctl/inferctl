package contract

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInferCapabilitiesMatchesDataGolden(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/infer", "capabilities", "--json")
	cmd.Dir = "../.."
	cmd.Env = append(cmd.Environ(), "INFERCTL_TEST_DETERMINISTIC=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("infer capabilities --json failed: %v", err)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", err, out)
	}
	goldenRaw, err := CapabilitiesRaw()
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]any
	if err := json.Unmarshal(goldenRaw, &golden); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(golden, envelope.Data); diff != "" {
		t.Fatalf("capabilities data mismatch (-want +got):\n%s", diff)
	}
	if !bytes.Contains(out, []byte(`"request_id":"req_01TEST00000000000000000000"`)) {
		t.Fatalf("deterministic envelope not used: %s", out)
	}
}
