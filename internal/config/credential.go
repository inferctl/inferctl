package config

import (
	"os/exec"
	"runtime"
	"strings"
)

type SecureStoreCredentialResolver struct {
	GOOS   string
	Lookup func(service, account string) (string, error)
}

func (r SecureStoreCredentialResolver) Source() string { return CredentialSourceSecureStore }
func (r SecureStoreCredentialResolver) Resolve(reference CredentialReference) (*string, *CredentialResolutionError) {
	if reference.Service == nil || reference.Account == nil || strings.TrimSpace(*reference.Service) == "" || strings.TrimSpace(*reference.Account) == "" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_SECURE_STORE_IDENTITY_REQUIRED"}
	}
	goos := r.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_SECURE_STORE_UNSUPPORTED_PLATFORM"}
	}
	lookup := r.Lookup
	if lookup == nil {
		lookup = macOSKeychainLookup
	}
	value, err := lookup(*reference.Service, *reference.Account)
	if err != nil {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_SECURE_STORE_UNAVAILABLE"}
	}
	if strings.TrimSpace(value) == "" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_SECURE_STORE_MISSING"}
	}
	return &value, credentialValueError(value)
}
func macOSKeychainLookup(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	return strings.TrimSpace(string(out)), err
}

// CredentialResolutionError is safe for public diagnostics. It identifies a
// reference failure without including a resolved credential value.
type CredentialResolutionError struct {
	Code string
}

// CredentialResolver resolves one typed credential source.
type CredentialResolver interface {
	Source() string
	Resolve(CredentialReference) (*string, *CredentialResolutionError)
}

// LiteralCredentialResolver resolves a literal held in the local config file.
type LiteralCredentialResolver struct{}

func (LiteralCredentialResolver) Source() string { return CredentialSourceLiteral }

func (LiteralCredentialResolver) Resolve(reference CredentialReference) (*string, *CredentialResolutionError) {
	if reference.LiteralValue == nil || strings.TrimSpace(*reference.LiteralValue) == "" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_LITERAL_REQUIRED"}
	}
	return reference.LiteralValue, credentialValueError(*reference.LiteralValue)
}

// EnvironmentCredentialResolver resolves an environment variable only when a
// control-plane operation needs it.
type EnvironmentCredentialResolver struct {
	Values map[string]string
}

func (EnvironmentCredentialResolver) Source() string { return CredentialSourceEnvironment }

func (r EnvironmentCredentialResolver) Resolve(reference CredentialReference) (*string, *CredentialResolutionError) {
	if reference.EnvironmentVariable == nil || strings.TrimSpace(*reference.EnvironmentVariable) == "" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_ENVIRONMENT_VARIABLE_REQUIRED"}
	}
	value, ok := r.Values[*reference.EnvironmentVariable]
	if !ok {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_ENVIRONMENT_MISSING"}
	}
	if strings.TrimSpace(value) == "" {
		return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_ENVIRONMENT_EMPTY"}
	}
	return &value, credentialValueError(value)
}

// ResolveCredential resolves a typed reference through the supplied source
// resolvers. A nil reference preserves the legacy literal configuration.
func ResolveCredential(backend BackendConfig, resolvers ...CredentialResolver) (*string, *CredentialResolutionError) {
	if backend.Credential == nil {
		return backend.AuthHeaderValue, nil
	}
	for _, resolver := range resolvers {
		if resolver.Source() == backend.Credential.Source {
			return resolver.Resolve(*backend.Credential)
		}
	}
	return nil, &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_SOURCE_UNSUPPORTED"}
}

func credentialValueError(value string) *CredentialResolutionError {
	if strings.ContainsAny(value, "\r\n") {
		return &CredentialResolutionError{Code: "E_CREDENTIAL_REFERENCE_VALUE_UNUSABLE"}
	}
	return nil
}
