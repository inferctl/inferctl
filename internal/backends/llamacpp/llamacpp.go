package llamacpp

import (
	"context"
	"time"

	"github.com/inferctl/inferctl/internal/backends"
	"github.com/inferctl/inferctl/pkg/inferctl"
)

type Backend struct {
	name           string
	baseURL        string
	defaultBackend bool
	http           backends.HTTPClient
}

func New(name, baseURL string, defaultBackend bool, timeout time.Duration) Backend {
	return Backend{
		name:           name,
		baseURL:        baseURL,
		defaultBackend: defaultBackend,
		http:           backends.NewHTTPClient(baseURL, timeout),
	}
}

func (b Backend) Info() inferctl.BackendInfo {
	return inferctl.BackendInfo{
		Name:    b.name,
		Kind:    "llama.cpp",
		BaseURL: b.baseURL,
		Default: b.defaultBackend,
	}
}

func (b Backend) Reachable(ctx context.Context) (inferctl.BackendInfo, error) {
	var body modelsResponse
	latency, err := b.http.GetJSON(ctx, "/v1/models", &body)
	info := b.Info()
	if err != nil {
		return info, err
	}
	ms := int(latency.Milliseconds())
	info.Reachable = true
	info.LatencyMS = &ms
	return info, nil
}

func (b Backend) ListInstalledModels(ctx context.Context) ([]inferctl.ModelInfo, error) {
	var body modelsResponse
	if _, err := b.http.GetJSON(ctx, "/v1/models", &body); err != nil {
		return nil, err
	}
	return body.toModelInfo(b.name), nil
}

func (b Backend) ListLoadedModels(ctx context.Context) ([]inferctl.LoadedModelInfo, error) {
	models, err := b.ListInstalledModels(ctx)
	if err != nil {
		return nil, err
	}
	loaded := make([]inferctl.LoadedModelInfo, 0, len(models))
	for _, model := range models {
		loaded = append(loaded, inferctl.LoadedModelInfo{
			Name:    model.Name,
			Backend: b.name,
		})
	}
	return loaded, nil
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (r modelsResponse) toModelInfo(backend string) []inferctl.ModelInfo {
	models := make([]inferctl.ModelInfo, 0, len(r.Data))
	for _, model := range r.Data {
		models = append(models, inferctl.ModelInfo{Name: model.ID, Backend: backend})
	}
	return models
}
