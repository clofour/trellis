# Trellis job schemas

These files are generated. Do not edit them by hand.

- `trellis-job-api.schema.json` is the canonical JSON `JobSpec` contract used by the Trellis API.
- `trellis-job.schema.json` is the first-party YAML authoring schema. It is derived from the canonical schema and adds only representation conveniences such as human-readable byte sizes and durations.

The generator reflects the Go model in `orchestrator/internal/spec`, then applies the semantic constraints that are useful to JSON Schema editors. Trellis server validation remains authoritative for cross-field and runtime semantics.

Regenerate from the `orchestrator/` directory:

```sh
go run ./cmd/generate-schemas
```

CI verifies that the checked-in files match generated output:

```sh
go run ./cmd/generate-schemas --check
```
