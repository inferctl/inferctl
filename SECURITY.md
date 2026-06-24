# Security Policy

## Reporting a Vulnerability

Report security issues privately via GitHub's [private vulnerability
reporting](https://github.com/inferctl/inferctl/security/advisories/new). Do
not open public issues for suspected vulnerabilities.

If GitHub reporting is unavailable to you, email `security@inferctl.dev` with
the details below. PGP not currently required.

Include:

- inferctl version (`inferctl version --json`) and OS/arch
- Reproduction steps
- Observed vs expected behavior
- Impact assessment (what an attacker gains)

## Response

We aim to acknowledge reports within 5 business days and provide a remediation
timeline within 10. No bug bounty at this stage.

## Scope

In scope:

- The `inferctl` CLI and its packaging (`go install`, future Homebrew/Scoop
  formulas).
- Configuration parsing and route-selection logic.
- Backend adapter code paths.

Out of scope:

- Vulnerabilities in upstream backends (Ollama, llama.cpp, LM Studio, MLX);
  report those to their respective projects.
- Issues that require local code execution as the user already running
  `inferctl`. `inferctl` runs with the user's own privileges and does not
  elevate; compromise of the user's shell is not an inferctl issue.
- DoS via misconfiguration of local backends.

## Supported Versions

Pre-1.0: only the latest tagged release is supported. Older minor versions
will not receive backports.
