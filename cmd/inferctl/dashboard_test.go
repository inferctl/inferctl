package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/inferctl/inferctl/pkg/inferctl"
)

func TestDashboardStatusFeedArgsUsePublicStatusWatchFeed(t *testing.T) {
	got := dashboardStatusFeedArgs(1500 * time.Millisecond)
	want := []string{"status", "--json", "--watch", "--events", "--interval", "1.5s"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dashboard feed args = %#v, want %#v", got, want)
	}
}

func TestDashboardJSONRefusesMachineContract(t *testing.T) {
	stdout, _, err := executeForTest("dashboard", "--json")
	if err == nil {
		t.Fatalf("dashboard --json unexpectedly succeeded: %s", stdout)
	}
	var env struct {
		OK     bool `json:"ok"`
		Errors []struct {
			Code       string `json:"code"`
			DidYouMean string `json:"did_you_mean"`
			ExitCode   int    `json:"exit_code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal dashboard --json error: %v\n%s", err, stdout)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "E_INVALID_ARG" || env.Errors[0].ExitCode != exitUserInput {
		t.Fatalf("dashboard --json envelope = %#v", env)
	}
	if !strings.Contains(env.Errors[0].DidYouMean, "status --json --watch") || !strings.Contains(stdout, "interactive dashboard") {
		t.Fatalf("dashboard --json did_you_mean/message = %s", stdout)
	}
}

func TestDashboardParsesStatusSnapshotEnvelope(t *testing.T) {
	var snapshot statusSnapshot
	readContractGolden(t, "status.golden.json", &snapshot)
	line := marshalDashboardEnvelope(t, snapshot)

	msg := dashboardRecordFromEnvelope(line)
	if msg.err != nil {
		t.Fatalf("parse snapshot envelope: %v", msg.err)
	}
	if msg.snapshot == nil || msg.snapshot.StatusFrameSchemaVersion != statusFrameSchemaVersion || len(msg.snapshot.Backends) != 2 {
		t.Fatalf("snapshot msg = %#v", msg)
	}
}

func TestDashboardParsesStatusEventBatchEnvelope(t *testing.T) {
	var batch statusEventBatch
	readContractGolden(t, "status_event_batch.golden.json", &batch)
	line := marshalDashboardEnvelope(t, batch)

	msg := dashboardRecordFromEnvelope(line)
	if msg.err != nil {
		t.Fatalf("parse event envelope: %v", msg.err)
	}
	if msg.eventBatch == nil || len(msg.eventBatch.Events) != 1 || msg.eventBatch.Events[0].Kind != "backend_reachability_changed" {
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

func TestDashboardViewShowsFallbackAndNewestEventsFirst(t *testing.T) {
	model := dashboardModel{
		source: statusFeedSource{Binary: "inferctl", Interval: 2 * time.Second},
		snapshot: &statusSnapshot{
			Summary: statusSummary{BackendsTotal: 2, BackendsReachable: 1, ModelsExposedTotal: 1, ModelsLoadedTotal: 1, RoutesTotal: 1, RoutesReady: 1, WarningsTotal: 2},
			Backends: []statusBackend{
				{Name: "llamacpp", Kind: "llama.cpp", BaseURL: "http://127.0.0.1:8080", Reachable: false},
				{Name: "ollama", Kind: "ollama", BaseURL: "http://127.0.0.1:11434", Reachable: true},
			},
			Routes: []statusRoute{{
				Task: "code",
				Decision: inferctl.RouteDecision{
					SelectedBackend: "ollama",
					SelectedModel:   "fallback:8b",
					Ready:           true,
					IsFallback:      true,
					Reason:          "selected fallback because primary is unavailable",
				},
			}},
		},
		events: []statusEvent{
			{Severity: "medium", Summary: "older event"},
			{Severity: "high", Summary: "newest event"},
		},
	}
	view := model.View()
	for _, want := range []string{
		"Backends 1/2 reachable",
		"down llamacpp",
		"code             ollama/fallback:8b",
		"yes    yes",
		"dashboard is read-only",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard view missing %q:\n%s", want, view)
		}
	}
	if strings.Index(view, "newest event") > strings.Index(view, "older event") {
		t.Fatalf("events not newest-first:\n%s", view)
	}
}

func TestDashboardDoesNotCallPrivateAggregationPaths(t *testing.T) {
	src, err := os.ReadFile("dashboard.go")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{
		"buildDoctorReport(",
		"buildStatusSnapshot(",
		"buildStatusRoutes(",
		"statusExposedModels(",
		"buildRouteExplanation(",
	}
	for _, needle := range forbidden {
		if strings.Contains(string(src), needle) {
			t.Fatalf("dashboard calls private aggregation path %s", needle)
		}
	}
}

func readContractGolden(t *testing.T, name string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "contract", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
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
