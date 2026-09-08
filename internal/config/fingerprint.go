package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const ConfigurationFingerprintVersion = "v1"

type Fingerprint struct {
	Version string `json:"version"`
	Value   string `json:"value"`
}

type Drift struct {
	Status   string `json:"status"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual"`
}

func ConfigurationFingerprint(cfg Config) (Fingerprint, error) {
	canonical, err := json.Marshal(cfg)
	if err != nil {
		return Fingerprint{}, err
	}
	sum := sha256.Sum256(canonical)
	return Fingerprint{Version: ConfigurationFingerprintVersion, Value: ConfigurationFingerprintVersion + ":sha256:" + hex.EncodeToString(sum[:])}, nil
}

func CheckConfigurationDrift(actual Fingerprint, expected string) Drift {
	if expected == "" {
		return Drift{Status: "not_checked", Actual: actual.Value}
	}
	if !strings.HasPrefix(expected, ConfigurationFingerprintVersion+":sha256:") {
		return Drift{Status: "unsupported_format", Expected: expected, Actual: actual.Value}
	}
	if expected == actual.Value {
		return Drift{Status: "match", Expected: expected, Actual: actual.Value}
	}
	return Drift{Status: "mismatch", Expected: expected, Actual: actual.Value}
}
