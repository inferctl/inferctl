package ollama

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
		Kind:    "ollama",
		BaseURL: b.baseURL,
		Default: b.defaultBackend,
	}
}

func (b Backend) Reachable(ctx context.Context) (inferctl.BackendInfo, error) {
	var body struct {
		Version string `json:"version"`
	}
	latency, err := b.http.GetJSON(ctx, "/api/version", &body)
	info := b.Info()
	if err != nil {
		return info, err
	}
	ms := int(latency.Milliseconds())
	info.Reachable = true
	info.LatencyMS = &ms
	if body.Version != "" {
		info.Version = &body.Version
	}
	return info, nil
}

func (b Backend) ListInstalledModels(ctx context.Context) ([]inferctl.ModelInfo, error) {
	var body struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			Digest     string `json:"digest"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if _, err := b.http.GetJSON(ctx, "/api/tags", &body); err != nil {
		return nil, err
	}
	models := make([]inferctl.ModelInfo, 0, len(body.Models))
	for _, model := range body.Models {
		size := model.Size
		digest := model.Digest
		installedAt := model.ModifiedAt
		models = append(models, inferctl.ModelInfo{
			Name:           model.Name,
			Backend:        b.name,
			SizeBytes:      optionalInt64(size),
			Digest:         optionalString(digest),
			InstalledAtISO: optionalString(installedAt),
		})
	}
	return models, nil
}

func (b Backend) ListLoadedModels(ctx context.Context) ([]inferctl.LoadedModelInfo, error) {
	var body struct {
		Models []struct {
			Name      string `json:"name"`
			SizeVRAM  int64  `json:"size_vram"`
			ExpiresAt string `json:"expires_at"`
		} `json:"models"`
	}
	if _, err := b.http.GetJSON(ctx, "/api/ps", &body); err != nil {
		return nil, err
	}
	models := make([]inferctl.LoadedModelInfo, 0, len(body.Models))
	for _, model := range body.Models {
		vram := model.SizeVRAM
		models = append(models, inferctl.LoadedModelInfo{
			Name:      model.Name,
			Backend:   b.name,
			VRAMBytes: optionalInt64(vram),
		})
	}
	return models, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
