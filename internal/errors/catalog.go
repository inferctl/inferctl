package errors

const (
	EUnknownVerb                  = "E_UNKNOWN_VERB"
	EUnknownFlag                  = "E_UNKNOWN_FLAG"
	EMissingArg                   = "E_MISSING_ARG"
	EInvalidArg                   = "E_INVALID_ARG"
	EIncompatibleFlags            = "E_INCOMPATIBLE_FLAGS"
	EUnknownTask                  = "E_UNKNOWN_TASK"
	EUnknownModel                 = "E_UNKNOWN_MODEL"
	EUnknownBackend               = "E_UNKNOWN_BACKEND"
	EConfigMissing                = "E_CONFIG_MISSING"
	EConfigInvalid                = "E_CONFIG_INVALID"
	EConfigUnreadable             = "E_CONFIG_UNREADABLE"
	ENoBackendsConfigured         = "E_NO_BACKENDS_CONFIGURED"
	EConfigWriteFailed            = "E_CONFIG_WRITE_FAILED"
	EConfigPatchDeleteUnsupported = "E_CONFIG_PATCH_DELETE_UNSUPPORTED"
	EConfigKeyUnknown             = "E_CONFIG_KEY_UNKNOWN"
	EBackendAuthFailed            = "E_BACKEND_AUTH_FAILED"
	EBackendRemoteNotAllowed      = "E_BACKEND_REMOTE_NOT_ALLOWED"
	EBackendTimeout               = "E_BACKEND_TIMEOUT"
	ENoRouteAvailable             = "E_NO_ROUTE_AVAILABLE"
	EBinaryInternal               = "E_BINARY_INTERNAL"
	EVerbRenamed                  = "E_VERB_RENAMED"
	EConfigValidationFailed       = "E_CONFIG_VALIDATION_FAILED"
	WBackendUnreachable           = "W_BACKEND_UNREACHABLE"
	WBackendBackoff               = "W_BACKEND_BACKOFF"
	WBackendDegraded              = "W_BACKEND_DEGRADED"
	WProbeTimeout                 = "W_PROBE_TIMEOUT"
	WModelNotInstalled            = "W_MODEL_NOT_INSTALLED"
	WModelNotLoaded               = "W_MODEL_NOT_LOADED"
	WFallbackUsed                 = "W_FALLBACK_USED"
	WContextNearLimit             = "W_CONTEXT_NEAR_LIMIT"
	WConfigKeyDeprecated          = "W_CONFIG_KEY_DEPRECATED"
	WConfigKeyUnknown             = "W_CONFIG_KEY_UNKNOWN"
	WConfigSchemaMismatch         = "W_CONFIG_SCHEMA_VERSION_MISMATCH"
	WUpdateCheckFailed            = "W_UPDATE_CHECK_FAILED"
	WBackendKindUnsupported       = "W_BACKEND_KIND_UNSUPPORTED"
	WProfileModeNotEnforced       = "W_PROFILE_MODE_NOT_ENFORCED"
)

var ActiveErrorCodes = []string{
	EUnknownVerb,
	EUnknownFlag,
	EMissingArg,
	EInvalidArg,
	EIncompatibleFlags,
	EUnknownTask,
	EUnknownModel,
	EUnknownBackend,
	EConfigMissing,
	EConfigInvalid,
	EConfigUnreadable,
	ENoBackendsConfigured,
	EConfigWriteFailed,
	EConfigPatchDeleteUnsupported,
	EConfigKeyUnknown,
	EBackendAuthFailed,
	EBackendRemoteNotAllowed,
	EBackendTimeout,
	ENoRouteAvailable,
	EBinaryInternal,
	EVerbRenamed,
	EConfigValidationFailed,
}

var ReservedErrorCodes = []string{}

var ActiveWarningCodes = []string{
	WBackendUnreachable,
	WBackendBackoff,
	WBackendDegraded,
	WProbeTimeout,
	WModelNotInstalled,
	WModelNotLoaded,
	WFallbackUsed,
	WContextNearLimit,
	WConfigKeyDeprecated,
	WConfigKeyUnknown,
	WConfigSchemaMismatch,
	WUpdateCheckFailed,
	WBackendKindUnsupported,
	WProfileModeNotEnforced,
}
