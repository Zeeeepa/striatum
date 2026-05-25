---
schema_version: striatum.decision.v1
decision_id: "rfc-0078-version-module"
run_id: "rfc-0078-go-only-packaging-release"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "RFC 0078 VERSION and Go module packaging decision"
created_at: "2026-05-25T00:00:00Z"
---

# RFC 0078 VERSION And Go Module Decision
author: operator [self-declared: release-architect-codex-gpt-5-001]

## Decision

- Root `VERSION` is the single version source for Go build and release
  archive metadata.
- The first Go-only release line is `v2.0.0`, represented in `VERSION` as
  `2.0.0`.
- `go/go.mod` stays nested for this packaging gate. Moving the module to the
  repository root is deferred until Python source deletion and root module
  ownership can happen in one follow-up.
- `striatum`, `striatumd`, and `striatum-supervisor-helper` remain separate
  release binaries for the first Go-only archive release.

## Rationale

The packaging gate can stop using `pyproject.toml` as version authority
without forcing the larger root module migration while Python files and
frontend source still live outside `go/`. Separate binaries match the current
operator model: CLI compatibility client, resident daemon, and supervised PTY
helper are distinct processes.

## Follow-Up

The Python deletion gate should decide whether to move `go/go.mod` to the
repository root after `pyproject.toml`, Python package data, and Python source
are gone. Until then, downstream packaging jobs should read only `VERSION`.
