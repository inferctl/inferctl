package contract

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestVerbGoldens(t *testing.T) {
	expected := []string{
		"backends.golden.json",
		"capabilities.golden.json",
		"config-explain.golden.json",
		"config-show.golden.json",
		"config-validate.golden.json",
		"config_init.print.golden.json",
		"config_patch.stdin.golden.json",
		"config_set.change.golden.json",
		"config_validate.clean.golden.json",
		"discover.empty.golden.json",
		"discover.ollama.golden.json",
		"doctor.golden.json",
		"doctor.recommended_action.no_future_verbs.golden.json",
		"model.golden.json",
		"models.golden.json",
		"route.golden.json",
		"triage.clean.golden.json",
		"triage.errors.golden.json",
		"version.golden.json",
	}
	dir := filepath.Join("..", "..", "testdata", "contract")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".golden.json") {
			actual = append(actual, entry.Name())
		}
	}
	slices.Sort(actual)
	if !slices.Equal(expected, actual) {
		t.Fatalf("contract golden files mismatch\nexpected: %#v\nactual:   %#v", expected, actual)
	}
}
