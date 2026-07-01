# Agent Discovery Demo

These tiny examples show an agent using inferctl for control-plane routing and
then calling the selected backend directly.

```sh
examples/agent-discovery/demo.py --allow-fallback "what is 2+2?"
examples/agent-discovery/demo.py --allow-fallback --dry-run "what is 2+2?"
```

The scripts call:

```sh
inferctl preflight code --prompt-file <file> --allow-fallback --json
inferctl config show --json
```

`preflight` decides whether the task may run and returns the selected
backend/model. `config show` maps that backend to its `base_url`. The script then
prints the handoff before making any backend request:

```text
inferctl selected: ollama_small / qwen3:8b
reason: selected fallback because primary 'qwen-coder-32b.gguf' is unavailable
data plane: calling http://127.0.0.1:11434/v1 directly
---
```

Inferctl never sends the chat request. The example uses the OpenAI client
against the selected backend's OpenAI-compatible `/v1` endpoint. Use `--dry-run`
to verify the handoff without requiring the backend to implement chat
completions.
