# Editor Config Generator

Generate editor local-model settings from inferctl's route and config state:

```sh
examples/editor-configs/generate.py --target aider
examples/editor-configs/generate.py --target cline
examples/editor-configs/generate.py --target aider --output .aider.conf.yml --overwrite
```

The generator calls:

```sh
inferctl route code --json
inferctl config show --json
```

`route --json` selects the backend/model for the task. `config show --json`
maps the selected backend to its `base_url`. The generated artifact keeps
inferctl in the control plane; Aider or Cline sends inference traffic directly
to the backend.

If inferctl selects a fallback route, the generator prints a warning on stderr
while keeping stdout parseable. If the selected route is not ready, generation
fails unless `--allow-not-ready` is passed.

For local OpenAI-compatible editor settings, the generator appends `/v1` to the
selected backend URL when it is not already present.
