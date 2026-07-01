#!/usr/bin/env python3
"""Read-only verifier for repo-local inferctl requirements."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tomllib
from pathlib import Path
from typing import Any


def main() -> int:
    parser = argparse.ArgumentParser(description="Verify local inference state against repo requirements.")
    parser.add_argument("--requirements", type=Path, default=Path(__file__).with_name("inferctl.requirements.toml"))
    parser.add_argument("--inferctl", default=os.environ.get("INFERCTL_BIN", "inferctl"))
    args = parser.parse_args()

    try:
        requirements = load_requirements(args.requirements)
        failures = verify(args.inferctl, args.requirements, requirements)
        if failures:
            print("FAIL local inference preflight")
            for failure in failures:
                print(f"- {failure}")
            return 1
        print("PASS local inference preflight")
        return 0
    except UserFacingError as exc:
        print(f"FAIL local inference preflight\n- {exc}")
        return exc.exit_code


class UserFacingError(Exception):
    def __init__(self, message: str, exit_code: int = 1) -> None:
        super().__init__(message)
        self.exit_code = exit_code


def load_requirements(path: Path) -> dict[str, Any]:
    try:
        return tomllib.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise UserFacingError(f"requirements file not found: {path}") from exc
    except tomllib.TOMLDecodeError as exc:
        raise UserFacingError(f"requirements file is invalid TOML: {exc}") from exc


def verify(binary: str, requirements_path: Path, requirements: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    validation = inferctl_json(binary, ["config", "validate"])
    if not validation.get("ok", False):
        failures.append("inferctl config validation failed")
        failures.extend(error_messages(validation))

    status = inferctl_json(binary, ["status"])
    if not status.get("ok", False):
        failures.append("inferctl status failed")
        failures.extend(error_messages(status))
        return failures

    failures.extend(check_backends(requirements.get("backends", {}), status["data"]))
    failures.extend(check_tasks(binary, requirements_path.parent, requirements.get("tasks", {}), status["data"]))
    return failures


def check_backends(required: dict[str, Any], status_data: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    actual = {backend["name"]: backend for backend in status_data.get("backends", [])}
    for name, spec in required.items():
        if not spec.get("required", False) and not spec.get("require_reachable", False):
            continue
        backend = actual.get(name)
        if backend is None:
            failures.append(f"missing backend: {name}")
            continue
        if spec.get("require_reachable", False) and not backend.get("reachable", False):
            failures.append(f"backend unreachable: {name}")
    return failures


def check_tasks(
    binary: str,
    root: Path,
    tasks: dict[str, Any],
    status_data: dict[str, Any],
) -> list[str]:
    failures: list[str] = []
    exposed_models = {model.get("name") for model in status_data.get("models", {}).get("exposed", [])}
    for task, spec in tasks.items():
        prompt_file = root / required_string(spec, "prompt_file", f"task {task}")
        allowed_models = list_of_strings(spec, "selected_model_any_of", f"task {task}")
        if allowed_models and not any(model in exposed_models for model in allowed_models):
            failures.append(f"none of the allowed models are exposed for task {task}: {', '.join(allowed_models)}")
        preflight_args = ["preflight", task, "--prompt-file", str(prompt_file)]
        if spec.get("allow_fallback", False):
            preflight_args.append("--allow-fallback")
        if spec.get("require_ready", False):
            preflight_args.append("--require-ready")
        preflight = inferctl_json(binary, preflight_args, require_preflight=True)
        if not preflight.get("ok", False):
            failures.append(f"task {task} is not runnable")
            failures.extend(error_messages(preflight))
            continue
        decision = preflight.get("data", {}).get("route_decision", {})
        selected = decision.get("selected_model")
        if selected not in allowed_models:
            failures.append(f"task {task} selected unexpected model: {selected}")
        if decision.get("is_fallback", False) and not spec.get("allow_fallback", False):
            failures.append(f"task {task} selected fallback but allow_fallback is false")
        if spec.get("require_ready", False) and not decision.get("ready", False):
            failures.append(f"task {task} selected model is not ready")
    return failures


def inferctl_json(binary: str, args: list[str], require_preflight: bool = False) -> dict[str, Any]:
    proc = subprocess.run(
        [binary, *args, "--json"],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        detail = proc.stderr.strip() or proc.stdout.strip()
        if require_preflight:
            raise UserFacingError(
                "inferctl binary does not provide the required preflight JSON contract: " + detail,
                proc.returncode or 1,
            ) from exc
        raise UserFacingError(f"inferctl {' '.join(args)} did not emit JSON: {detail}") from exc


def error_messages(envelope: dict[str, Any]) -> list[str]:
    out: list[str] = []
    for error in envelope.get("errors") or []:
        message = error.get("message") or error.get("code")
        if message:
            out.append(message)
    return out


def required_string(value: dict[str, Any], key: str, context: str) -> str:
    raw = value.get(key)
    if not isinstance(raw, str) or raw == "":
        raise UserFacingError(f"{context} missing string field {key!r}")
    return raw


def list_of_strings(value: dict[str, Any], key: str, context: str) -> list[str]:
    raw = value.get(key)
    if not isinstance(raw, list) or not raw or not all(isinstance(item, str) for item in raw):
        raise UserFacingError(f"{context} missing non-empty string list {key!r}")
    return raw


if __name__ == "__main__":
    raise SystemExit(main())
