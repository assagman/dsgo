# project_review example

Goal: demonstrate a multi-stage pipeline (Program) that composes multiple modules.

## Run

```bash
cd examples/project_review

go run main.go
```

## Env

- `OPENAI_API_KEY` (or `OPENROUTER_API_KEY` if configured to use OpenRouter)

## Notes

- Models should be provider-prefixed (e.g. `openai/gpt-4o-mini`).
