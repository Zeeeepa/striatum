---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Retirement Guardrails Handoff
author: operator [self-declared: retirement-guard-codex-gpt-5-002]

## Changed Files

- `go/pkg/webguardrails/guardrails_test.go`
- `scripts/guard_rfc0078_web_retirement.sh`
- `go/pkg/webservice/service.go`
- `go/pkg/webservice/service_test.go`

## Guardrails Added

The Go guard test scans the scoped Go web/service packages and `go/cmd/striatumd`
web files for retired Python imports and non-authoritative state markers:
`striatum.service`, `striatum.web`, `src/striatum/service.py`, `sqlite3`,
`retired-local-state`, terminal-output state, and transcripts.

The shell guard is runnable from normal repository validation and names the
retired `/dogfood` and `/chat` route families. The Go service returns `410
Gone` for those route families, and route tests assert that refusal.

## Validation

- `go test ./pkg/webguardrails` passed as part of focused validation.
- `scripts/guard_rfc0078_web_retirement.sh` passed.

## Remaining Risk

The guardrail is scoped to this gate's intended Go web/service paths. The
final RFC 0078 Python-removal gate still needs the broader repository-wide
Python trace guardrail once other workers finish CLI, packaging, docs, and test
migration slices.
