package config

import (
	"strings"
	"testing"
)

func TestConfigurationFingerprintIsPublicAndStable(t *testing.T) {
	first, err := LoadBytes([]byte(workedExampleTOML), testSource("/tmp/a.toml"), nil, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadBytes([]byte(stringReplace(workedExampleTOML, "fixture-auth-value", "other-secret")), testSource("/tmp/b.toml"), nil, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := ConfigurationFingerprint(first.Config)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ConfigurationFingerprint(second.Config)
	if err != nil {
		t.Fatal(err)
	}
	if a.Value != b.Value {
		t.Fatalf("secret changed fingerprint: %s %s", a.Value, b.Value)
	}
	changed, err := LoadBytes([]byte(stringReplace(workedExampleTOML, "http://127.0.0.1:11434", "http://127.0.0.1:11435")), testSource("/tmp/c.toml"), nil, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := ConfigurationFingerprint(changed.Config)
	if a.Value == c.Value {
		t.Fatal("route-affecting URL did not change fingerprint")
	}
	if CheckConfigurationDrift(a, a.Value).Status != "match" || CheckConfigurationDrift(a, "v2:sha256:x").Status != "unsupported_format" {
		t.Fatal("unexpected drift result")
	}
}

func stringReplace(value, old, replacement string) string {
	return strings.Replace(value, old, replacement, 1)
}
