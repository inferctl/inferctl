package testserver

import (
	"net/http"
	"testing"
	"time"
)

func TestOllamaFixtureEndpoints(t *testing.T) {
	server := New(Fixture{
		Kind:    KindOllama,
		Version: "0.5.1",
		Models:  []Model{{Name: "qwen3:8b", SizeBytes: 123, Digest: "sha256:abc", InstalledAtISO: "2026-06-10T12:00:00Z"}},
		Loaded:  []LoadedModel{{Name: "qwen3:8b", VRAMBytes: 456}},
	})
	defer server.Close()

	assertStatus(t, server.URL+"/api/version", http.StatusOK)
	assertStatus(t, server.URL+"/api/tags", http.StatusOK)
	assertStatus(t, server.URL+"/api/ps", http.StatusOK)
}

func TestOpenAICompatFixtureEndpoint(t *testing.T) {
	server := New(Fixture{
		Kind:   KindOpenAICompat,
		Models: []Model{{Name: "qwen3:8b"}},
	})
	defer server.Close()

	assertStatus(t, server.URL+"/v1/models", http.StatusOK)
	assertStatus(t, server.URL+"/api/tags", http.StatusNotFound)
}

func TestFixtureCanSimulateMalformedUnreachableAndLatency(t *testing.T) {
	server := New(Fixture{
		Kind:    KindLlamaCPP,
		Latency: 10 * time.Millisecond,
		Malformed: map[string]bool{
			"/v1/models": true,
		},
		Backoff: Backoff{
			Active:              true,
			SecondsRemaining:    90,
			ConsecutiveFailures: 3,
			LastError:           "connection refused",
		},
	})
	defer server.Close()

	start := time.Now()
	resp, err := http.Get(server.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("latency not applied: %s", elapsed)
	}
	if server.Fixture.Backoff.SecondsRemaining != 90 {
		t.Fatalf("backoff fixture not retained: %#v", server.Fixture.Backoff)
	}

	unreachable := New(Fixture{Kind: KindOllama, Unreachable: true})
	defer unreachable.Close()
	assertStatus(t, unreachable.URL+"/api/version", http.StatusServiceUnavailable)
}

func assertStatus(t *testing.T, url string, want int) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("GET %s status = %d, want %d", url, resp.StatusCode, want)
	}
}
