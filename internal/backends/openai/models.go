package openai

import "github.com/Ozhiaki/inferctl/pkg/inferctl"

type ModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (r ModelsResponse) ToModelInfo(backend string) []inferctl.ModelInfo {
	models := make([]inferctl.ModelInfo, 0, len(r.Data))
	for _, model := range r.Data {
		models = append(models, inferctl.ModelInfo{Name: model.ID, Backend: backend})
	}
	return models
}
