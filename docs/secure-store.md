---
title: Secure Store Credentials
description: Use a macOS Keychain reference for a local backend credential.
bucket: guides
order: 26
---

# Secure-store credentials

The `secure_store` credential source is supported on macOS. It reads one
generic password from the macOS Keychain during a bounded control-plane check.
inferctl does not show the resolved value.

```toml
[backends.local_api.credential]
version = "v1"
source = "secure_store"
service = "inferctl"
account = "local-api-token"
```

Create the item with Keychain Access or with your approved system process. To
remove it, delete the same service and account in Keychain Access. Do not put a
credential value in config, command arguments, or logs.

On other platforms, inferctl returns
`E_CREDENTIAL_REFERENCE_SECURE_STORE_UNSUPPORTED_PLATFORM`. A missing or
unavailable Keychain service returns a stable secure-store diagnostic without
including the credential value.
