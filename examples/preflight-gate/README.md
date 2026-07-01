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

