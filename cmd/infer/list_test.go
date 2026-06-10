package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ozhiaki/inferctl/internal/testserver"
)

func TestBackendsModelsAndModelCommands(t *testing.T) {
	ollamaServer := testserver.New(testserver.Fixture{
		Kind:   testserver.KindOllama,
		Models: []testserver.Model{{Name: "qwen3:8b", SizeBytes: 123}},
		Loaded: []testserver.LoadedModel{{Name: "qwen3:8b", VRAMBytes: 456}},
	})
	defer ollamaServer.Close()
	llamaServer := testserver.New(testserver.Fixture{
		Kind:   testserver.KindLlamaCPP,
		Models: []testserver.Model{{Name: "coder.gguf"}},
	})
	defer llamaServer.Close()
	t.Setenv("INFERCTL_CONFIG", writeListConfig(t, ollamaServer.URL, llamaServer.URL))

	stdout, _, err := executeForTest("backends", "--json")
	if err != nil {
		t.Fatalf("backends error = %v stdout=%s", err, stdout)
	}
	var backendsEnv struct {
		Data struct {
			TotalCount     int `json:"total_count"`
			ReachableCount int `json:"reachable_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &backendsEnv); err != nil {
		t.Fatal(err)
	}
	if backendsEnv.Data.TotalCount != 2 || backendsEnv.Data.ReachableCount != 2 {
		t.Fatalf("backends data = %#v", backendsEnv.Data)
	}

	stdout, _, err = executeForTest("models", "--json")
	if err != nil {
		t.Fatalf("models error = %v stdout=%s", err, stdout)
	}
	var modelsEnv struct {
		Data struct {
			TotalCount  int `json:"total_count"`
			LoadedCount int `json:"loaded_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &modelsEnv); err != nil {
		t.Fatal(err)
	}
	if modelsEnv.Data.TotalCount != 2 || modelsEnv.Data.LoadedCount != 2 {
		t.Fatalf("models data = %#v", modelsEnv.Data)
	}

	stdout, _, err = executeForTest("model", "qwen3:8b", "--json")
	if err != nil {
		t.Fatalf("model error = %v stdout=%s", err, stdout)
	}
	var modelEnv struct {
		Data struct {
			Name         string `json:"name"`
			LatencyStats struct {
				Samples int `json:"samples"`
			} `json:"latency_stats"`
			Routing struct {
				FallbackChain []string `json:"fallback_chain"`
			} `json:"routing"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &modelEnv); err != nil {
		t.Fatal(err)
	}
	if modelEnv.Data.Name != "qwen3:8b" || modelEnv.Data.LatencyStats.Samples != 0 || len(modelEnv.Data.Routing.FallbackChain) != 1 {
		t.Fatalf("model data = %#v", modelEnv.Data)
	}
}

func TestUnknownBackendAndModelErrors(t *testing.T) {
	server := testserver.New(testserver.Fixture{Kind: testserver.KindOllama, Models: []testserver.Model{{Name: "qwen3:8b"}}})
	defer server.Close()
	t.Setenv("INFERCTL_CONFIG", writeListConfig(t, server.URL, server.URL))

	stdout, _, err := executeForTest("backends", "--filter", "missing", "--json")
	if err == nil {
		t.Fatal("expected unknown backend error")
	}
	if !strings.Contains(stdout, "E_UNKNOWN_BACKEND") || !strings.Contains(stdout, "infer backends") {
		t.Fatalf("unexpected backend error envelope: %s", stdout)
	}

	stdout, _, err = executeForTest("model", "missing-model", "--json")
	if err == nil {
		t.Fatal("expected unknown model error")
	}
	if !strings.Contains(stdout, "E_UNKNOWN_MODEL") || !strings.Contains(stdout, "infer models") {
		t.Fatalf("unexpected model error envelope: %s", stdout)
	}
}

func writeListConfig(t *testing.T, ollamaURL, llamaURL string) string {
	t.Helper()
	body := `[meta]
schema_version = "0.1"

[profile]
name = "default_local_workstation"
max_context_tokens = 8192
max_concurrent_models = 1
allow_premium = false
mode = "warn"

[backends.ollama]
kind = "ollama"
base_url = "` + ollamaURL + `"
default = true

[backends.llamacpp]
kind = "llama.cpp"
base_url = "` + llamaURL + `"
default = false

[routing.code]
model = "qwen3:8b"
backend = "ollama"
fallback = ["coder.gguf"]
`
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
