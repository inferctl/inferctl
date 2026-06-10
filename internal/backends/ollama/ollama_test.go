package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOllamaHappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			w.Write([]byte(`{"version":"0.5.1"}`))
		case "/api/tags":
			w.Write([]byte(`{"models":[{"name":"qwen3:8b","size":123,"digest":"sha256:abc","modified_at":"2026-06-10T12:00:00Z"}]}`))
		case "/api/ps":
			w.Write([]byte(`{"models":[{"name":"qwen3:8b","size_vram":456}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	backend := New("ollama", server.URL, true, time.Second)
	info, err := backend.Reachable(context.Background())
	if err != nil {
		t.Fatalf("Reachable() error = %v", err)
	}
	if !info.Reachable || info.Version == nil || *info.Version != "0.5.1" {
		t.Fatalf("info = %#v", info)
	}
	installed, err := backend.ListInstalledModels(context.Background())
	if err != nil {
		t.Fatalf("ListInstalledModels() error = %v", err)
	}
	if len(installed) != 1 || installed[0].Name != "qwen3:8b" || installed[0].SizeBytes == nil {
		t.Fatalf("installed = %#v", installed)
	}
	loaded, err := backend.ListLoadedModels(context.Background())
	if err != nil {
		t.Fatalf("ListLoadedModels() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].VRAMBytes == nil || *loaded[0].VRAMBytes != 456 {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestOllamaMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	backend := New("ollama", server.URL, false, time.Second)
	if _, err := backend.ListInstalledModels(context.Background()); err == nil {
		t.Fatal("ListInstalledModels() error = nil")
	}
}

func TestOllamaTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte(`{"version":"late"}`))
	}))
	defer server.Close()

	backend := New("ollama", server.URL, false, 5*time.Millisecond)
	if _, err := backend.Reachable(context.Background()); err == nil {
		t.Fatal("Reachable() error = nil")
	}
}
