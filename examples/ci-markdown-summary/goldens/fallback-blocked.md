## inferctl preflight: code_review

- Runnability: `policy_blocked`
- Selected route: `ollama_small/qwen3:8b`
- Readiness: configured=`true` reachable=`true` ready=`true`
- Fallback: `selected`
- Warnings:
  - `W_BACKEND_UNREACHABLE`: backend 'llamacpp_large' is unreachable
  - `W_FALLBACK_USED`: routed to fallback 'qwen3:8b' because primary 'qwen-coder-32b.gguf' is unavailable
- Recommended action: `inferctl route code_review --json`
