package contract

import (
	"embed"
	"encoding/json"
)

//go:embed capabilities.golden.json
var fs embed.FS

func CapabilitiesData() (map[string]any, error) {
	raw, err := fs.ReadFile("capabilities.golden.json")
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func CapabilitiesRaw() ([]byte, error) {
	return fs.ReadFile("capabilities.golden.json")
}
