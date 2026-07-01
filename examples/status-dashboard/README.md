# Status Dashboard Scenario

This scenario demonstrates inferctl's read-only status feed and dashboard
boundary with deterministic fixture backends.

```sh
examples/status-dashboard/test.sh
```

The harness:

- builds a local inferctl binary;
- starts a reachable fallback backend;
- runs `inferctl status --json`;
- runs `inferctl status --json --watch --events --interval 200ms`;
- starts the primary backend after the watch stream begins, producing public
  status event batches;
- verifies `dashboard --json` refuses and points automation back to
  `status --json --watch`.

The dashboard is a human renderer over the public status feed. Automation should
consume `status --json --watch --events`, not scrape the dashboard.

See [demo-transcript.txt](demo-transcript.txt) for the terminal sequence this
scenario is meant to capture.
