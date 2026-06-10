package envelope

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEnvelopeConformanceDeterministic(t *testing.T) {
	env, err := New("0.1.0", map[string]any{"b": 2, "a": 1}, Options{
		Env: map[string]string{"INFERCTL_TEST_DETERMINISTIC": "1"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !env.OK {
		t.Fatal("OK = false")
	}
	if env.Meta.RequestID != "req_01TEST00000000000000000000" {
		t.Fatalf("request_id = %q", env.Meta.RequestID)
	}
	if env.Meta.TSISO != "1970-01-01T00:00:00.000Z" {
		t.Fatalf("ts_iso = %q", env.Meta.TSISO)
	}
	if env.Meta.ElapsedMS != 0 {
		t.Fatalf("elapsed_ms = %d", env.Meta.ElapsedMS)
	}
	if env.Meta.DataHash == nil || !strings.HasPrefix(*env.Meta.DataHash, "sha256:") {
		t.Fatalf("data_hash = %#v", env.Meta.DataHash)
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, key := range []string{"ok", "tool_version", "data", "meta", "warnings", "commands", "errors"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing top-level key %q in %s", key, b)
		}
	}
}

func TestDataHashReflectsRealDataInDeterministicMode(t *testing.T) {
	a, err := New("0.1.0", map[string]any{"value": "a"}, Options{Env: map[string]string{"INFERCTL_TEST_DETERMINISTIC": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := New("0.1.0", map[string]any{"value": "b"}, Options{Env: map[string]string{"INFERCTL_TEST_DETERMINISTIC": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if *a.Meta.DataHash == *b.Meta.DataHash {
		t.Fatalf("data hashes should differ for different data: %s", *a.Meta.DataHash)
	}
}

func TestElapsedMSNonDeterministic(t *testing.T) {
	start := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	finish := start.Add(47 * time.Millisecond)
	env, err := New("0.1.0", map[string]string{"ok": "yes"}, Options{
		StartedAt:  start,
		FinishedAt: finish,
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.Meta.ElapsedMS != 47 {
		t.Fatalf("elapsed_ms = %d", env.Meta.ElapsedMS)
	}
	if !strings.HasPrefix(env.Meta.RequestID, "req_") {
		t.Fatalf("request_id = %q", env.Meta.RequestID)
	}
}
