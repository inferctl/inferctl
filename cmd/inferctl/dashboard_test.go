package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDashboardStatusFeedArgsUsePublicStatusWatchFeed(t *testing.T) {
	got := dashboardStatusFeedArgs(1500 * time.Millisecond)
	want := []string{"status", "--json", "--watch", "--events", "--interval", "1.5s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dashboard feed args = %#v, want %#v", got, want)
	}
}

func TestDashboardParsesStatusSnapshotEnvelope(t *testing.T) {
	line := marshalDashboardEnvelope(t, statusSnapshot{
		StatusFrameSchemaVersion: statusFrameSchemaVersion,
		ContractVersion:          "0.1",
		CapturedAtISO:            "2026-06-30T15:00:00Z",
		Summary:                  statusSummary{BackendsTotal: 1, BackendsReachable: 1},
		Backends:                 []statusBackend{{Name: "ollama", Kind: "ollama", Reachable: true}},
		Routes: []statusRoute{{
			Task: "code",
		}},
	})

	msg := dashboardRecordFromEnvelope(line)
	if msg.err != nil {
		t.Fatalf("parse snapshot envelope: %v", msg.err)
	}
	if msg.snapshot == nil || msg.snapshot.StatusFrameSchemaVersion != statusFrameSchemaVersion || len(msg.snapshot.Backends) != 1 {
		t.Fatalf("snapshot msg = %#v", msg)
	}
}

func TestDashboardParsesStatusEventBatchEnvelope(t *testing.T) {
	line := marshalDashboardEnvelope(t, statusEventBatch{
		EventSchemaVersion: statusEventSchemaVersion,
		ContractVersion:    "0.1",
		CapturedAtISO:      "2026-06-30T15:00:01Z",
		SinceCapturedAtISO: "2026-06-30T15:00:00Z",
		Events: []statusEvent{{
			Sequence: 1,
			Kind:     "route_selection_changed",
			Subject:  "route:code",
			Severity: "high",
			Summary:  "route code changed from llamacpp/primary.gguf to ollama/fallback:8b",
		}},
	})

	msg := dashboardRecordFromEnvelope(line)
	if msg.err != nil {
		t.Fatalf("parse event envelope: %v", msg.err)
	}
	if msg.eventBatch == nil || len(msg.eventBatch.Events) != 1 || msg.eventBatch.Events[0].Kind != "route_selection_changed" {
		t.Fatalf("event msg = %#v", msg)
	}
}

func TestDashboardParsesErrorEnvelope(t *testing.T) {
	line := []byte(`{"ok":false,"data":null,"errors":[{"message":"status feed failed"}]}`)
	msg := dashboardRecordFromEnvelope(line)
	if msg.err == nil || !strings.Contains(msg.err.Error(), "status feed failed") {
		t.Fatalf("error msg = %#v", msg)
	}
}

func marshalDashboardEnvelope(t *testing.T, data any) []byte {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"ok":     true,
		"data":   data,
		"errors": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return line
}
