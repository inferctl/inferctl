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

Run the deterministic harness from a source checkout:

```sh
./examples/preflight-gate/test.sh
```

The harness uses repository fixture servers and temporary configs. It does not
require Ollama, llama.cpp, LM Studio, MLX, or remote endpoints.
