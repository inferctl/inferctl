package llamacpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLlamaCppModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"id":"Qwen3-8B-Instruct.Q4_K_M.gguf"}]}`))
	}))
	defer server.Close()

	backend := New("llamacpp_8b", server.URL, false, time.Second)
	info, err := backend.Reachable(context.Background())
	if err != nil {
		t.Fatalf("Reachable() error = %v", err)
	}
	if !info.Reachable {
		t.Fatalf("info = %#v", info)
	}
	installed, err := backend.ListInstalledModels(context.Background())
	if err != nil {
		t.Fatalf("ListInstalledModels() error = %v", err)
	}
	if len(installed) != 1 || installed[0].Name != "Qwen3-8B-Instruct.Q4_K_M.gguf" {
		t.Fatalf("installed = %#v", installed)
	}
	loaded, err := backend.ListLoadedModels(context.Background())
	if err != nil {
		t.Fatalf("ListLoadedModels() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Name != installed[0].Name {
		t.Fatalf("loaded = %#v", loaded)
	}
}
