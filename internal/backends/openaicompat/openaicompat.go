package openaicompat

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/Ozhiaki/inferctl/internal/backends"
	openaimodels "github.com/Ozhiaki/inferctl/internal/backends/openai"
	"github.com/Ozhiaki/inferctl/pkg/inferctl"
)

var ErrNotSupported = errors.New("operation not supported by openai_compat in v0.1")
var ErrAuthFailed = errors.New("openai_compat authentication failed")
var ErrRemoteNotAllowed = errors.New("openai_compat remote endpoint requires remote_allowed=true")

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
	headers := map[string]string{}
	if options.AuthHeaderName != nil && options.AuthHeaderValue != nil {
		headers[*options.AuthHeaderName] = *options.AuthHeaderValue
	}
	return Backend{
		name:           name,
		baseURL:        baseURL,
		defaultBackend: defaultBackend,
		options:        options,
		http:           backends.NewHTTPClientWithHeaders(baseURL, timeout, headers),
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
	return nil
}

func (b Backend) Reachable(ctx context.Context) (inferctl.BackendInfo, error) {
	if !b.options.RemoteAllowed && RemoteURL(b.baseURL) {
		return b.Info(), ErrRemoteNotAllowed
	}
	var body openaimodels.ModelsResponse
	latency, err := b.http.GetJSON(ctx, "/v1/models", &body)
	info := b.Info()
	if err != nil {
		return info, classifyError(err)
	}
	ms := int(latency.Milliseconds())
	info.Reachable = true
	info.LatencyMS = &ms
	return info, nil
}

func (b Backend) ListInstalledModels(ctx context.Context) ([]inferctl.ModelInfo, error) {
	if !b.options.RemoteAllowed && RemoteURL(b.baseURL) {
		return nil, ErrRemoteNotAllowed
	}
	var body openaimodels.ModelsResponse
	if _, err := b.http.GetJSON(ctx, "/v1/models", &body); err != nil {
		return nil, classifyError(err)
	}
	return body.ToModelInfo(b.name), nil
}

func (b Backend) ListLoadedModels(context.Context) ([]inferctl.LoadedModelInfo, error) {
	return nil, ErrNotSupported
}

func classifyError(err error) error {
	var status backends.StatusError
	if errors.As(err, &status) && (status.StatusCode == 401 || status.StatusCode == 403) {
		return ErrAuthFailed
	}
	return err
}

func RemoteURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}
