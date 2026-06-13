package mlx

import (
	"context"
	"time"

	"github.com/Ozhiaki/inferctl/internal/backends"
	openaimodels "github.com/Ozhiaki/inferctl/internal/backends/openai"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
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
		Kind:    "mlx",
		BaseURL: b.baseURL,
		Default: b.defaultBackend,
	}
}

func (b Backend) Reachable(ctx context.Context) (inferctl.BackendInfo, error) {
	var body openaimodels.ModelsResponse
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
	var body openaimodels.ModelsResponse
	if _, err := b.http.GetJSON(ctx, "/v1/models", &body); err != nil {
		return nil, err
	}
	return body.ToModelInfo(b.name), nil
}

func (b Backend) ListLoadedModels(ctx context.Context) ([]inferctl.LoadedModelInfo, error) {
	models, err := b.ListInstalledModels(ctx)
	if err != nil {
		return nil, err
	}
	loaded := make([]inferctl.LoadedModelInfo, 0, len(models))
	for _, model := range models {
		loaded = append(loaded, inferctl.LoadedModelInfo{Name: model.Name, Backend: b.name})
	}
	return loaded, nil
}

func (b Backend) ProbeIdentity(ctx context.Context) error {
	_, err := b.ListInstalledModels(ctx)
	return err
}
