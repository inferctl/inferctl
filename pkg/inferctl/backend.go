package inferctl

import "context"

type Backend interface {
	Info() BackendInfo
	Reachable(ctx context.Context) (BackendInfo, error)
	ListInstalledModels(ctx context.Context) ([]ModelInfo, error)
	ListLoadedModels(ctx context.Context) ([]LoadedModelInfo, error)
}
