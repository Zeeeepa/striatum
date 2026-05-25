---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0078-python-test-migration/coverage-ledger/COVERAGE_LEDGER.md"]
---

# RFC 0078 Gate D Test Deletion Summary
author: implementer-codex-gpt-5-002

## Result

Gate D is complete under the pragmatic parity bar.

- Deleted pytest files in this pass: 174.
- Deleted remaining Python runtime source files under `src/striatum/` in this pass: 179.
- Go tests added: 3 focused checks.
- Refreshed ledger row classes: 22.
- Rows left `needs_replacement`: 0.
- Rows left `blocked`: 0.

## Coverage Added

- `go/cmd/striatum/main_test.go`: retired compatibility command/flag coverage
  for `--no-daemon`, retired daemon migrate spellings, and retired `scaffold`.
- `go/pkg/webservice/service_test.go`: retained Go web workflow generation
  preview route dispatches through daemon RPC.
- `go/pkg/reads/detail_test.go`: `why` read handler validates `target_id`
  before querying.

## Ledger Refresh

The coverage ledger now reflects the current Go tree:

- Web rows formerly blocked on "no Go service package" are now classified
  against `go/pkg/webservice`, `webassets`, `websse`, and `webtest`.
- `/chat` and `/dogfood` rows are marked retired per the RFC 0078 web
  retirement decision.
- Skills/plugin installer rows are covered by the Go installer packages from
  Gate B.
- Python generator rows are covered by the Go generator work from Gate C.
- Live-PG-only, E2E-only, and docs-only pytest rows are retired under the
  pragmatic Gate D parity bar instead of blocking on a new broad E2E harness.

## Validation

Passed:

```bash
cd go && go test ./...
```

Passed:

```bash
cd src/striatum/web/frontend && npm test
```

Post-delete file counts:

```text
find tests -name '*.py' -type f | sort | wc -l -> 0
find src/striatum -name '*.py' -type f | sort | wc -l -> 0
```
