# Changelog

## Unreleased

- `striatum branch confirm` now honors the previously inert `--create` and
  `--use-current` flags and adds a new `--strict` flag. `--create` runs
  `git checkout -b <branch>` (with idempotent fallback to `git checkout`),
  `--use-current` records the actual current git branch, and `--strict`
  refuses to record unless the working tree already matches. Default behavior
  remains records-only, and the JSON response now includes `mode` and
  `created` fields.
- Replaced the evidence-export key-name blocklist with a default-deny policy
  registry. Any field not explicitly classified as `safe` in
  `EVIDENCE_POLICY` is redacted from exported Markdown, so future schema
  additions cannot silently leak agent or user prose.

## 0.1.0 - 2026-05-07

- Split Striatum from Engram with history preserved from the former
  `agent-runner/` incubation directory.
- Renamed the package, CLI, workflow schema, and repo-local state directory to
  `striatum`.
- Replaced the initial all-rights-reserved status with Apache-2.0 licensing.
- Added standalone project metadata, CI, and a fresh-clone smoke script.
- Added workflow planning, run-summary export, stale-lease recovery
  introspection, local API wrapper, and minimal process-adapter launch support.
- Added workflow graph export, bounded stale-work requeue, decision-artifact
  recording, a local MCP-like stdio wrapper, and explicit adapter enforcement
  validation.
- Added stricter release checks with `ruff`, `mypy`, wheel/sdist smoke, and
  installed package metadata validation.
