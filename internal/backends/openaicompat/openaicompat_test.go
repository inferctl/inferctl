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

func TestOpenAICompatUnsupportedOptions(t *testing.T) {
	headerName := "Authorization"
	headerValue := "Bearer token"
	backend := New("remote", "http://127.0.0.1:1234", false, time.Second, Options{
		AuthHeaderName:  &headerName,
		AuthHeaderValue: &headerValue,
		RemoteAllowed:   true,
	})
	got := backend.UnsupportedOptions()
	want := map[string]bool{"auth_header_name": true, "auth_header_value": true, "remote_allowed": true}
	if len(got) != len(want) {
		t.Fatalf("unsupported = %#v", got)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("unexpected unsupported option %q in %#v", name, got)
		}
	}
}
