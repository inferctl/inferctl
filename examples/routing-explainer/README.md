# Routing Explainer

Run the terminal-native route explanation:

```sh
inferctl route code --prompt-file sample-task.txt --explain
```

From this directory:

```sh
./run.sh
```

Primary route selected:

```text
selected: qwen3:8b on ollama
reason: primary model is available

candidates:
  role      backend          model                  status
  primary  ollama           qwen3:8b               selected_ready

prompt:
  15 estimated tokens / 8192 max context tokens

next: inferctl model qwen3:8b --json
```

Fallback selected:

```text
selected: qwen3:8b on ollama_small (fallback)
reason: selected fallback because primary 'qwen-coder-32b.gguf' is unavailable

candidates:
  role      backend          model                  status
  primary  llamacpp_large   qwen-coder-32b.gguf    backend_unreachable
  fallback ollama_small     qwen3:8b               selected_ready

prompt:
  15 estimated tokens / 8192 max context tokens

warnings:
- W_BACKEND_UNREACHABLE: backend 'llamacpp_large' is unreachable
- W_FALLBACK_USED: routed to fallback 'qwen3:8b' because primary 'qwen-coder-32b.gguf' is unavailable

next: inferctl backends --filter llamacpp_large --json
```

`route --explain` is the human explanation of which configured local route
inferctl would choose and why. `route --json` is the structured form for tools
and tests. Use `preflight` when automation needs pass/fail readiness behavior
before attempting work.

