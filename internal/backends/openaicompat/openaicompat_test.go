package openaicompat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatModelsAndLoadedUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[{"id":"qwen3:8b"}]}`))
	}))
	defer server.Close()

	backend := New("lmstudio_local", server.URL, false, time.Second, Options{})
	info, err := backend.Reachable(context.Background())
	if err != nil {
		t.Fatalf("Reachable() error = %v", err)
	}
	if !info.Reachable {
		t.Fatalf("info = %#v", info)
	}
	models, err := backend.ListInstalledModels(context.Background())
	if err != nil {
		t.Fatalf("ListInstalledModels() error = %v", err)
	}
	if len(models) != 1 || models[0].Name != "qwen3:8b" {
		t.Fatalf("models = %#v", models)
	}
	if _, err := backend.ListLoadedModels(context.Background()); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("ListLoadedModels() error = %v", err)
	}
}

func TestOpenAICompatAuthHeaderAndRemoteOptions(t *testing.T) {
	headerName := "Authorization"
	headerValue := "fixture-" + "auth-" + "value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerName) != headerValue {
			http.Error(w, "auth required", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":[{"id":"qwen3:8b"}]}`))
	}))
	defer server.Close()

	backend := New("remote", server.URL, false, time.Second, Options{
		AuthHeaderName:  &headerName,
		AuthHeaderValue: &headerValue,
		RemoteAllowed:   true,
	})
	if unsupported := backend.UnsupportedOptions(); len(unsupported) != 0 {
		t.Fatalf("unsupported = %#v", unsupported)
	}
	models, err := backend.ListInstalledModels(context.Background())
	if err != nil {
		t.Fatalf("ListInstalledModels() error = %v", err)
	}
	if len(models) != 1 || models[0].Name != "qwen3:8b" {
		t.Fatalf("models = %#v", models)
	}

	noAuth := New("remote", server.URL, false, time.Second, Options{})
	if _, err := noAuth.ListInstalledModels(context.Background()); !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("no-auth error = %v", err)
	}

	remote := New("remote", "https://example.com", false, time.Second, Options{})
	if _, err := remote.ListInstalledModels(context.Background()); !errors.Is(err, ErrRemoteNotAllowed) {
		t.Fatalf("remote-not-allowed error = %v", err)
	}
}
