# Gate D — Pytest deletion by refreshed ledger row

You are the implementer for RFC 0078 Gate D. This job runs AFTER Gates A, B,
and C. Read first: `docs/operator/plans/rfc-0078-remaining-work.md` (Gate D),
`docs/operator/artifacts/rfc-0078-python-test-migration/coverage-ledger/COVERAGE_LEDGER.md`,
and the current Go test tree under `go/`.

## Parity bar (operator decision: PRAGMATIC)

- Row `covered` / `retire` / `historical_exception` → delete the Python file.
- Row `needs_replacement` protecting **core** behavior → add the named Go (or
  shell/browser) test FIRST, then delete the Python file.
- Row `needs_replacement` that is **E2E-only, live-PG-only, or docs-only** →
  RETIRE it with a one-line recorded reason in the ledger (do not block on a
  full Go E2E harness). Pragmatic closure is the goal.

## Steps

1. **Refresh the ledger.** It is stale: it marks web rows `blocked` for "no Go
   service package," but `go/pkg/webservice` / `webassets` / `websse` /
   `webtest` now exist and `/chat` + `/dogfood` are deliberately retired
   (`scripts/guard_rfc0078_web_retirement.sh`). Re-classify those rows to
   `covered` (Go web service) or `retire` (chat/dogfood) as appropriate.
2. Walk every `tests/**/*.py` row. Apply the parity bar. For core
   `needs_replacement`, add the focused Go test in the package named by the
   row's gate command, then delete the Python file.
3. Delete `tests/conftest.py`, package `__init__.py` markers, and
   `tests/_harness/**` LAST (after their dependents are gone).
4. Delete any remaining `src/striatum/**/*.py` runtime modules not already
   removed by Gates A/B — the CLI, web, service, daemon_pg handlers, api,
   bootstrap, etc. — once nothing tracked imports them. The Go daemon/CLI/web
   are the live runtime; the Python tree is superseded.

## Constraints

- Preserve behavioral coverage, not file shape. Every deletion is paired with a
  Go test, a `retire`/`historical_exception` ledger reason, or an existing
  `covered` row.
- Stay within `write_scope.allowed_paths`. Do not touch `pyproject.toml`,
  `Makefile`, `scripts/`, or current-guidance docs — those are Gates E/F.

## Validate

```bash
cd go && go test ./...
(cd src/striatum/web/frontend && npm test) || true
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/tests/SUMMARY.md`
(`artifact_kind: synthesis`) with: counts deleted vs Go-test-added vs retired,
the refreshed ledger summary, any rows left `needs_replacement` with reason,
and validation output. Update the COVERAGE_LEDGER.md in place. Use your byline.
