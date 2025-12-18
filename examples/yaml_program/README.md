# YAML Program Runner (internal DSGo feature)

This example demonstrates DSGo's **internal** YAML-to-DSGo program compiler (`internal/yamlprogram`).

It lets you define:
- signatures
- tool sources (builtin tools + MCP clients)
- modules (all DSGo modules)
- a top-level pipeline (Program)

…and run it without writing Go orchestration code.

## Run

```bash
go run ./examples/yaml_program --schema > yaml_program.schema.json

# Software development workflow (covers all modules across this directory)
go run ./examples/yaml_program ./examples/yaml_program/software_dev.yaml

# Web research workflow (requires API keys)
go run ./examples/yaml_program ./examples/yaml_program/deep_research.yaml

# Security scan workflow
go run ./examples/yaml_program ./examples/yaml_program/security_scan.yaml
```

## Files

- `main.go`: small runner that loads YAML, builds a `dsgo.Program`, and executes it
- `software_dev.yaml`: software dev loop (planning → synthesis → tool-using implementation → review → refine)
- `deep_research.yaml`: deep web research (parallel agents + synthesis)
- `security_scan.yaml`: parallel file scan + synthesized report

## Schema reference

The runner can emit a JSON Schema for editor support:

```bash
go run ./examples/yaml_program --schema
```

You can use this with the VS Code YAML extension to get:
- autocomplete
- documentation tooltips
- validation errors for unknown fields / invalid values

## Notes

- YAML decoding is **strict**: unknown fields are errors.
- Scalar options (e.g. `temperature`, `max_tokens`) use pointer types in Go so that
  explicit zero values like `temperature: 0` are representable.
- This is currently **internal-only** (`internal/yamlprogram`) and not part of the
  public `dsgo` API surface.
