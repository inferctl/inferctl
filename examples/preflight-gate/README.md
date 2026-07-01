# Preflight Gate

Use `preflight` before an agent or CI job spends time on local inference work:

```sh
set -e
inferctl preflight code --prompt-file sample-task.txt --allow-fallback
python agent.py
```

Or run the wrapper with your own agent command:

```sh
./run-agent.sh -- python agent.py
```

`preflight` is a control-plane check. It inspects configuration, route
selection, backend reachability, model inventory, prompt metadata, warnings, and
policy flags. It performs no inference and only authorizes the caller to attempt
its own inference step.

The wrapper relies on the command exit code. It does not parse human output and
does not call `doctor`, `route`, or `triage` separately.

Common local modes:

```sh
# Accept any runnable configured route.
inferctl preflight code --prompt-file sample-task.txt --allow-fallback

# Require the selected route to be ready, not merely configured.
inferctl preflight code --prompt-file sample-task.txt --allow-fallback --require-ready

# Block fallback so automation only runs on the primary route.
inferctl preflight code --prompt-file sample-task.txt
```

CI shell:

```sh
if inferctl preflight code --prompt-file sample-task.txt --json >preflight.json; then
  python agent.py
else
  jq '.data.runnability, .errors' preflight.json
  exit 1
fi
```

Structured and summary output:

```sh
inferctl preflight code --prompt-file sample-task.txt --allow-fallback --json
inferctl preflight code --prompt-file sample-task.txt --allow-fallback --format markdown
```

The Markdown form can be redirected into CI summary or annotation surfaces. See
the CI Markdown Summary example for provider-specific packaging.

Readiness terms:

- `configured`: the task has a route in inferctl config.
- `reachable`: the selected backend responds to the control-plane probe.
- `ready`: the selected model is already resident on the selected backend.

`--require-ready` applies to the final term. Without it, a configured and
reachable route whose selected model is not resident can still be runnable, with
an explicit warning.

Default policy matrix:

| Flags | Fallback selected | Selected model not ready | Expected result |
|-------|-------------------|--------------------------|-----------------|
| none | block with exit `5` | allow with warning | strict primary route, readiness advisory |
| `--allow-fallback` | allow with warning | allow with warning | permissive route, readiness advisory |
| `--require-ready` | block with exit `5` | block with exit `5` | strict primary route and resident model |
| `--allow-fallback --require-ready` | allow with warning | block with exit `5` | fallback allowed, resident model required |

Cells combine when multiple conditions are true. The most restrictive outcome
wins: for example, a fallback route whose selected model is not ready under
`--allow-fallback --require-ready` still blocks with exit `5` because readiness
is required.

Run the deterministic harness from a source checkout:

```sh
./examples/preflight-gate/test.sh
```

The harness uses repository fixture servers and temporary configs. It does not
require Ollama, llama.cpp, LM Studio, MLX, or remote endpoints.
