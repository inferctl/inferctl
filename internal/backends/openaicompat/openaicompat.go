package openaicompat

import (
	"context"
	"errors"
	"time"

	"github.com/Ozhiaki/inferctl/internal/backends"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
)

var ErrNotSupported = errors.New("operation not supported by openai_compat in v0.1")

type Options struct {
	AuthHeaderName  *string
	AuthHeaderValue *string
	RemoteAllowed   bool
}

type Backend struct {
	name           string
	baseURL        string
	defaultBackend bool
	options        Options
	http           backends.HTTPClient
}

func New(name, baseURL string, defaultBackend bool, timeout time.Duration, options Options) Backend {
	return Backend{
		name:           name,
		baseURL:        baseURL,
		defaultBackend: defaultBackend,
		options:        options,
		http:           backends.NewHTTPClient(baseURL, timeout),
	}
}

func (b Backend) Info() inferctl.BackendInfo {
	return inferctl.BackendInfo{
		Name:    b.name,
		Kind:    "openai_compat",
		BaseURL: b.baseURL,
		Default: b.defaultBackend,
	}
}

func (b Backend) UnsupportedOptions() []string {
	var unsupported []string
	if b.options.AuthHeaderName != nil {
		unsupported = append(unsupported, "auth_header_name")
	}
	if b.options.AuthHeaderValue != nil {
		unsupported = append(unsupported, "auth_header_value")
	}
	if b.options.RemoteAllowed {
		unsupported = append(unsupported, "remote_allowed")
	}
	return unsupported
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

func (b Backend) ListLoadedModels(context.Context) ([]inferctl.LoadedModelInfo, error) {
	return nil, ErrNotSupported
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
