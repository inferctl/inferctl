# composability.md — Pipelining, `--from-stdin`, `--deliver`, `feedback`

A CLI is composable when its outputs can flow into another invocation's inputs without shell glue, and when its artifacts can be routed to arbitrary sinks. Three surfaces accomplish this: `--from-stdin` for input, `--deliver` for output routing, and `feedback` for the loopback channel.

This file covers all three. They are listed under one principle in the spine because they complete a single pipe: input in, output out, friction back.

## `--from-stdin`: structured input

Most CLIs accept positional arguments. Agents constructing a pipeline want to feed previous output in directly, without re-quoting and shell-escaping.

```bash
# Without --from-stdin
$ <tool> list --json | jq -r '.data.items[].id' | while read id; do <tool> show "$id"; done

# With --from-stdin
$ <tool> list --json | <tool> show --from-stdin --json
```

The contract: when `--from-stdin` is passed, the tool reads structured input from stdin and interprets it according to a documented schema. The schema is published in `capabilities`:

```json
"commands": {
  "show": {
    "flags": [{"name": "--from-stdin", "type": "bool"}],
    "stdin": {
      "accepts":      "json",
      "schema_ref":   "#/definitions/show_stdin_input",
      "alternatives": ["plain_ids_newline_separated"]
    }
  }
}
```

Two stdin shapes are common:

**Shape A — IDs only, one per line (or whitespace-separated):**

```bash
$ echo "X-001 X-007" | <tool> show --from-stdin --json
$ printf "X-001\nX-007\n" | <tool> show --from-stdin --json
```

**Shape B — Pipe a prior envelope; tool extracts what it needs:**

```bash
$ <tool> list --json | <tool> show --from-stdin --json
# <tool> reads stdin, parses the envelope, extracts data.items[].id
```

Shape B is the higher-leverage move: it lets two verbs chain with no `jq` between them. Document the extraction path in `capabilities.commands.<verb>.stdin.envelope_path` (e.g. `"$.data.items[].id"`).

### The `-` argument convention

For tools whose mutate verbs already take a positional arg, the conventional shortcut is `-`:

```bash
$ echo "the post body" | <tool> post create -
# `-` means: read the positional from stdin
```

Both forms are acceptable; pick one and document it. `--from-stdin` is more explicit (better for new tools); `-` matches Unix priors (better when matching an existing surface).

### Encoding

stdin is bytes; tools must declare encoding. JSON input MUST be UTF-8. Plain-text input SHOULD be UTF-8; document fallback if any. Don't auto-detect — declare in `capabilities`.

## `--deliver`: routing output artifacts

Standard output works for small payloads inline. For artifacts (files, blobs, multi-megabyte results), agents want them written to disk or POSTed to a webhook without a shell `>` redirect or a curl pipe.

```bash
$ <tool> export --deliver=stdout              # default; full payload to stdout
$ <tool> export --deliver=file:/tmp/out.tar   # atomic write to file
$ <tool> export --deliver=webhook:https://...  # POST to URL, return status
$ <tool> export --deliver=null                # discard payload; metadata only
```

The `--deliver` flag accepts a URL-like scheme. Tool dispatches on scheme.

### File delivery

```bash
$ <tool> export --deliver=file:/tmp/out.tar --json
```

```json
{
  "ok": true,
  "data": {"delivered_to": "/tmp/out.tar", "bytes": 4823014, "sha256": "..."},
  "meta": {...},
  "errors": []
}
```

Rules:

- **Atomic.** Write to `<target>.tmp`, fsync, rename. Partial writes never visible.
- **Hash returned.** Agents verify what they got.
- **Path must be writable.** Pre-flight check; fail fast with `INVALID_INPUT` if not.
- **No clobber by default.** `--force` required to overwrite an existing file.
- **Document permissions.** Default 0644; documented in `capabilities`.

### Webhook delivery

```bash
$ <tool> export --deliver=webhook:https://example.com/ingest --json
```

```json
{
  "ok": true,
  "data": {
    "delivered_to":      "https://example.com/ingest",
    "http_status":       200,
    "response_headers":  {"x-request-id": "..."},
    "bytes_sent":        4823014
  },
  "meta": {...}
}
```

Rules:

- **Return HTTP status.** Don't collapse 2xx/4xx/5xx into pass/fail.
- **Retry policy declared.** Tool either retries (with exponential backoff) or doesn't; document in `capabilities`.
- **Timeout configurable.** `--deliver-timeout=Ns`.
- **Auth via env vars or headers, never inline.** `TOOL_WEBHOOK_TOKEN` env. Never accept `--deliver=webhook:https://user:pass@host/` — credentials in args leak to ps and logs.

### Unknown schemes

If the agent passes a scheme the tool doesn't support:

```bash
$ <tool> export --deliver=s3://my-bucket/out.tar
```

```json
{
  "ok": false,
  "errors": [{
    "code":         "UNKNOWN_DELIVERY_SCHEME",
    "message":      "delivery scheme 's3' not supported",
    "remediation":  "use one of: stdout, file, webhook, null",
    "did_you_mean": null,
    "exit_code":    1
  }]
}
```

Structured refusal, not a half-implementation. Agents learn the surface.

### Capabilities advertise schemes

```json
"global_flags": {
  "--deliver": {
    "type":         "string",
    "schemes":      ["stdout", "file", "webhook", "null"],
    "default":      "stdout",
    "description":  "Where to route artifact output"
  }
}
```

## `feedback`: the loopback channel

Agents discover friction. They notice when an error message is unclear, when a flag's behavior surprises, when a workflow needs three round-trips it should've needed one. Capturing that signal closes the loop.

```bash
$ <tool> feedback "The 'doctor' command returned exit 3 but no recommended_action."
```

Default: writes locally to `$XDG_DATA_HOME/<tool>/feedback.jsonl` (one line per submission).

```json
{
  "ts_iso":        "2026-05-08T15:01:22Z",
  "tool_version":  "0.4.1",
  "contract_version": "1",
  "message":       "The 'doctor' command returned exit 3 but no recommended_action.",
  "context": {
    "cwd":         "$HOME/p/foo",
    "last_argv":   ["<tool>", "doctor", "--json"],
    "last_exit":   3
  }
}
```

Local-first matters: no network requirement, no auth surface to fail. Agents and contractors in air-gapped environments can still submit.

### Optional upstream POST

If the tool maintainer configures an ingestion endpoint:

```toml
# config.toml
[feedback]
upstream_url = "https://example.com/feedback-ingest"
upstream_token = "..."
```

Then `<tool> feedback` writes locally AND POSTs upstream:

```bash
$ <tool> feedback "..." --json
```

```json
{
  "ok": true,
  "data": {
    "local_path":     "$XDG_DATA_HOME/<tool>/feedback.jsonl",
    "upstream":       {"posted": true, "http_status": 202}
  }
}
```

If the upstream POST fails, the local write still succeeds; surface in `warnings`:

```json
"warnings": [{
  "code":    "UPSTREAM_FAILURE",
  "message": "feedback recorded locally; upstream POST failed (timeout)"
}]
```

### Reading feedback back

Maintainers want to inspect what users have reported:

```bash
$ <tool> feedback list --json
$ <tool> feedback list --since=7d --json
$ <tool> feedback clear --yes      # local clear; doesn't touch upstream
```

This closes the loop without making the agent depend on infrastructure that may not exist.

## Why these three together

They compose into a complete pipe:

```bash
$ <tool> plan --json \
  | <tool> execute --from-stdin --deliver=file:/tmp/run.log --json
$ <tool> feedback "execute took 4 min; should this be async by default?"
```

Each verb does its own thing; the pipe carries structured data; the loopback captures the friction. No shell glue, no `jq` between stages, no curl for the artifact, no email for the bug report.

Individually any one of these surfaces helps; together they cover the input, the output, and the feedback edge of every workflow.

## Anti-patterns

- **`--from-stdin` without a documented stdin schema.** Agent has to reverse-engineer what bytes the tool wants.
- **stdin shape changes by flag combination.** Same `--from-stdin` reads JSON sometimes, plain text other times, depending on `--format`. Pick one or split into separate flags.
- **File delivery without atomic write.** Half-written files leak into downstream consumers when the tool crashes mid-write.
- **Webhook delivery without status return.** Agent can't tell if the POST landed.
- **Credentials in `--deliver` URL.** Leaks to process listings and shell history.
- **`feedback` requires network.** Local-first means usable offline. Network as enrichment, not gate.
- **`feedback` posts upstream silently.** User should know where their words go. Surface upstream URL in `--help` or first-run notice.
- **No `feedback list`.** Maintainer (or the user themselves) can't inspect what's been collected.
- **Pipelining works only with `jq` between stages.** If `<tool> A --json | <tool> B --from-stdin` requires `jq -r '.data.items[].id'` in the middle, the stdin schema is too narrow. Accept the envelope too.

## Cross-references

- `envelope.md` — the JSON shape that flows through pipelines.
- `mega-commands.md` — mega-commands often deliver large payloads via `--deliver=file:...`.
- `errors.md` — `UNKNOWN_DELIVERY_SCHEME`, `UPSTREAM_FAILURE` error codes.
- `safety.md` — `--force` for clobber; `--yes` for `feedback clear`.
- `config.md` — `[feedback]` config section; upstream URL provenance.
