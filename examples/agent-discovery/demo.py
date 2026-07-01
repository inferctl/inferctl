#!/usr/bin/env python3
"""Minimal agent handoff demo for inferctl."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any
from urllib.parse import urlparse, urlunparse


def main() -> int:
    parser = argparse.ArgumentParser(description="Ask inferctl where to run a coding-agent prompt.")
    parser.add_argument("prompt", nargs="?", help="prompt text for the one-shot agent call")
    parser.add_argument("--prompt-file", type=Path)
    parser.add_argument("--task", default="code")
    parser.add_argument("--allow-fallback", action="store_true")
    parser.add_argument("--dry-run", action="store_true", help="print handoff and skip backend call")
    parser.add_argument("--inferctl", default=os.environ.get("INFERCTL_BIN", "inferctl"))
    args = parser.parse_args()

    try:
        prompt_file, cleanup = prompt_source(args.prompt, args.prompt_file)
        try:
            preflight = inferctl_json(
                args.inferctl,
                "preflight",
                args.task,
                "--prompt-file",
                str(prompt_file),
                *(["--allow-fallback"] if args.allow_fallback else []),
            )
            config = inferctl_json(args.inferctl, "config", "show")
            decision = preflight["data"]["route_decision"]
            backend_name = required_string(decision, "selected_backend", "preflight route_decision")
            model = required_string(decision, "selected_model", "preflight route_decision")
            reason = required_string(decision, "reason", "preflight route_decision")
            api_base = openai_compatible_base_url(selected_base_url(config, backend_name))
            print_handoff(backend_name, model, reason, api_base)
            if args.dry_run:
                print("[dry run: backend call skipped]")
                return 0
            response = call_backend(api_base, model, prompt_file.read_text(encoding="utf-8"))
            print(response)
            return 0
        finally:
            cleanup()
    except UserFacingError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return exc.exit_code


class UserFacingError(Exception):
    def __init__(self, message: str, exit_code: int = 1) -> None:
        super().__init__(message)
        self.exit_code = exit_code


def prompt_source(prompt: str | None, prompt_file: Path | None) -> tuple[Path, callable]:
    if prompt and prompt_file:
        raise UserFacingError("use either prompt text or --prompt-file, not both")
    if prompt_file:
        if not prompt_file.exists():
            raise UserFacingError(f"prompt file does not exist: {prompt_file}")
        return prompt_file, lambda: None
    if not prompt:
        raise UserFacingError("provide prompt text or --prompt-file")
    temp = tempfile.NamedTemporaryFile("w", encoding="utf-8", suffix=".txt", delete=False)
    try:
        temp.write(prompt)
        temp.close()
        path = Path(temp.name)
        return path, lambda: path.unlink(missing_ok=True)
    except Exception:
        Path(temp.name).unlink(missing_ok=True)
        raise


def inferctl_json(binary: str, *args: str) -> dict[str, Any]:
    proc = subprocess.run(
        [binary, *args, "--json"],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    try:
        envelope = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        detail = proc.stderr.strip() or proc.stdout.strip()
        if "preflight" in args:
            detail = detail or "inferctl did not recognize preflight"
            raise UserFacingError(
                "this demo requires an inferctl build with `preflight`; " + detail,
                proc.returncode or 1,
            ) from exc
        raise UserFacingError(f"inferctl {' '.join(args)} did not emit JSON: {detail}") from exc
    if proc.returncode != 0 or not envelope.get("ok", False):
        errors = envelope.get("errors") or []
        if errors:
            first = errors[0]
            message = first.get("message") or first.get("code") or "inferctl command failed"
        else:
            message = proc.stderr.strip() or "inferctl command failed"
        raise UserFacingError(f"inferctl {' '.join(args)} failed: {message}", proc.returncode or 1)
    return envelope


def selected_base_url(config_env: dict[str, Any], backend_name: str) -> str:
    backends = config_env.get("data", {}).get("effective_config", {}).get("backends", {})
    backend = backends.get(backend_name)
    if not isinstance(backend, dict):
        raise UserFacingError(f"selected backend {backend_name!r} is missing from config show output")
    return required_string(backend, "base_url", f"backend {backend_name}")


def required_string(value: dict[str, Any], key: str, context: str) -> str:
    raw = value.get(key)
    if not isinstance(raw, str) or raw == "":
        raise UserFacingError(f"{context} missing string field {key!r}")
    return raw


def openai_compatible_base_url(base_url: str) -> str:
    parsed = urlparse(base_url)
    path = parsed.path.rstrip("/")
    if path.endswith("/v1") or path == "/v1":
        return urlunparse(parsed._replace(path=path))
    path = (path + "/v1") if path else "/v1"
    return urlunparse(parsed._replace(path=path))


def print_handoff(backend: str, model: str, reason: str, api_base: str) -> None:
    print(f"inferctl selected: {backend} / {model}")
    print(f"reason: {reason}")
    print(f"data plane: calling {api_base} directly")
    print("---")


def call_backend(api_base: str, model: str, prompt: str) -> str:
    try:
        from openai import OpenAI
    except ImportError as exc:
        raise UserFacingError(
            "install the OpenAI Python client or rerun with --dry-run: python -m pip install openai"
        ) from exc
    client = OpenAI(base_url=api_base, api_key=os.environ.get("OPENAI_API_KEY", "placeholder"))
    response = client.chat.completions.create(
        model=model,
        messages=[{"role": "user", "content": prompt}],
    )
    content = response.choices[0].message.content
    return content or ""


if __name__ == "__main__":
    raise SystemExit(main())
