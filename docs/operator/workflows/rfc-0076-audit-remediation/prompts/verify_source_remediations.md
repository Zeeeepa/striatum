# Verify Source Remediations

You are verifying RFC 0076 remediation items REM-001, REM-002, and REM-009.

Read:

- `docs/operator/artifacts/rfc-0076-code-doc-audit/REMEDIATION_PLAN.md`
- `docs/operator/artifacts/rfc-0076-code-doc-audit/SYNTHESIS.md`
- `go/pkg/mutations/claim.go`
- `go/pkg/mutations/claim_test.go`
- `go/pkg/mcp/tools.go`
- `go/pkg/mcp/http_test.go`
- `src/striatum/daemon_pg/handlers/context.py`
- `tests/daemon_pg/handlers/workflow_loop/test_claim_next.py`
- `src/striatum/day_zero.py`
- `tests/test_day_zero.py`

Produce `docs/operator/artifacts/rfc-0076-audit-remediation/source-verification/REPORT.md`.

The report must map REM-001, REM-002, and REM-009 to `closed`, `open`, or
`needs_followup`, cite file paths and tests, and list the exact validation
commands that should be run before closure. Do not edit source or docs in this
job.
