package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolutionOrderEnvThenRepoLocal(t *testing.T) {
	dir := t.TempDir()
	envConfig := filepath.Join(dir, "env.toml")
	repoConfig := filepath.Join(dir, "inferctl.toml")
	if err := os.WriteFile(envConfig, []byte(workedExampleTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoConfig, []byte(workedExampleTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	loader := Loader{
		WorkingDir: dir,
		HomeDir:    filepath.Join(dir, "home"),
		Env:        map[string]string{"INFERCTL_CONFIG": envConfig},
	}
	got, err := loader.Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SourcePaths.Selected == nil || *got.SourcePaths.Selected != envConfig {
		t.Fatalf("selected = %#v, want %s", got.SourcePaths.Selected, envConfig)
	}
	if got.SourcePaths.SelectedBy == nil || *got.SourcePaths.SelectedBy != "env" {
		t.Fatalf("selected_by = %#v", got.SourcePaths.SelectedBy)
	}
}
