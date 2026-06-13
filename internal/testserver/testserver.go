package testserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"
)

type Kind string

const (
	KindOllama       Kind = "ollama"
	KindLlamaCPP     Kind = "llama.cpp"
	KindOpenAICompat Kind = "openai_compat"
	KindLMStudio     Kind = "lmstudio"
	KindMLX          Kind = "mlx"
)

type Fixture struct {
	Kind        Kind
	Version     string
	Models      []Model
	Loaded      []LoadedModel
	Latency     time.Duration
	Unreachable bool
	Malformed   map[string]bool
	Backoff     Backoff
}

type Model struct {
	Name           string
	SizeBytes      int64
	Digest         string
	InstalledAtISO string
}

type LoadedModel struct {
	Name      string
	VRAMBytes int64
}

type Backoff struct {
	Active              bool
	SecondsRemaining    int
	ConsecutiveFailures int
	LastError           string
}

type Server struct {
	*httptest.Server
	Fixture Fixture
}

func New(f Fixture) *Server {
	s := &Server{Fixture: f}
	s.Server = httptest.NewServer(Handler(f))
	return s
}

func (s *Server) Close() {
	s.Server.Close()
}

func Handler(f Fixture) http.Handler {
	s := &Server{Fixture: f}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	return mux
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if s.Fixture.Latency > 0 {
		time.Sleep(s.Fixture.Latency)
	}
	if s.Fixture.Unreachable {
		http.Error(w, "fixture unreachable", http.StatusServiceUnavailable)
		return
	}
	if s.Fixture.Malformed != nil && s.Fixture.Malformed[r.URL.Path] {
		w.Write([]byte(`not-json`))
		return
	}

	switch s.Fixture.Kind {
	case KindOllama:
		s.handleOllama(w, r)
	case KindLlamaCPP, KindOpenAICompat, KindLMStudio, KindMLX:
		s.handleOpenAIModels(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleOllama(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/version":
		version := s.Fixture.Version
		if version == "" {
			version = "fixture"
		}
		writeJSON(w, map[string]any{"version": version})
	case "/api/tags":
		models := make([]map[string]any, 0, len(s.Fixture.Models))
		for _, model := range s.Fixture.Models {
			models = append(models, map[string]any{
				"name":        model.Name,
				"size":        model.SizeBytes,
				"digest":      model.Digest,
				"modified_at": model.InstalledAtISO,
			})
		}
		writeJSON(w, map[string]any{"models": models})
	case "/api/ps":
		models := make([]map[string]any, 0, len(s.Fixture.Loaded))
		for _, model := range s.Fixture.Loaded {
			models = append(models, map[string]any{
				"name":      model.Name,
				"size_vram": model.VRAMBytes,
			})
		}
		writeJSON(w, map[string]any{"models": models})
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/models" {
		http.NotFound(w, r)
		return
	}
	models := make([]map[string]any, 0, len(s.Fixture.Models))
	for _, model := range s.Fixture.Models {
		models = append(models, map[string]any{
			"id":     model.Name,
			"object": "model",
		})
	}
	writeJSON(w, map[string]any{"object": "list", "data": models})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(err)
	}
}
