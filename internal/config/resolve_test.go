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

func TestResolutionOrderFallbacks(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	xdg := filepath.Join(dir, "xdg")
	repoConfig := filepath.Join(dir, "inferctl.toml")
	xdgConfig := filepath.Join(xdg, "inferctl", "config.toml")
	homeConfig := filepath.Join(home, ".config", "inferctl", "config.toml")
	for _, path := range []string{repoConfig, xdgConfig, homeConfig} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(workedExampleTOML), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	assertSelectedPath(t, Loader{WorkingDir: dir, HomeDir: home, Env: map[string]string{"XDG_CONFIG_HOME": xdg}}, repoConfig, "repo_local")
	if err := os.Remove(repoConfig); err != nil {
		t.Fatal(err)
	}
	assertSelectedPath(t, Loader{WorkingDir: dir, HomeDir: home, Env: map[string]string{"XDG_CONFIG_HOME": xdg}}, xdgConfig, "xdg_explicit")
	if err := os.Remove(xdgConfig); err != nil {
		t.Fatal(err)
	}
	assertSelectedPath(t, Loader{WorkingDir: dir, HomeDir: home, Env: map[string]string{"XDG_CONFIG_HOME": xdg}}, homeConfig, "xdg_default")
}

func TestWindowsAppDataFallback(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	appData := filepath.Join(dir, "AppData", "Roaming")
	configPath := filepath.Join(appData, "inferctl", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(workedExampleTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	assertSelectedPath(t, Loader{
		WorkingDir:  dir,
		HomeDir:     home,
		RuntimeGOOS: "windows",
		Env:         map[string]string{"APPDATA": appData},
	}, configPath, "windows_appdata")
}

func assertSelectedPath(t *testing.T, loader Loader, wantPath, wantBy string) {
	t.Helper()
	got, err := loader.Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SourcePaths.Selected == nil || *got.SourcePaths.Selected != wantPath {
		t.Fatalf("selected = %#v, want %s", got.SourcePaths.Selected, wantPath)
	}
	if got.SourcePaths.SelectedBy == nil || *got.SourcePaths.SelectedBy != wantBy {
		t.Fatalf("selected_by = %#v, want %s", got.SourcePaths.SelectedBy, wantBy)
	}
}
