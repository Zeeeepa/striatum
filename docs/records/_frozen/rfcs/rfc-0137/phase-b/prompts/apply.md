# Apply — finalize RFC 0137 Phase B

Review verdict was **accept** (or an accepted revision). Finalize Phase B.

## Do

1. Re-confirm green: `make -C go build`, `go test ./pkg/metrics/...`,
   `go test ./pkg/mutations/...`, `go build ./...`. Address any in-scope
   reviewer nits (you have repo-write to `go/pkg/metrics/`,
   `go/pkg/mutations/`, `go/cmd/striatumd/`); do not expand into Phase C/D.
2. Confirm the necrosis-domain guardrail test and
   `TestLivenessMissCanRecoverWithoutNecrosis` pass, the redaction golden was
   updated for the new families and its forbidden-content regex still passes,
   and `go/pkg/mutations/` edits stayed surgical (no recovery-logic refactor).

## Deliverable artifact: SUMMARY.md

Write `striatum/rfc-0137/phase-b/artifacts/SUMMARY.md`: final file list, the
enum→source-constant anchoring, the tx-safe counter mechanism, how F-A6 is
enforced, the acceptance-criteria → test mapping with verification commands +
pass results, and the follow-ups left for Phase C (Classification/Register
refusal, series budget + cardinality_clipped, allowlist hash + boot abort,
doctor_problems collector) so the next run's scope is clear.

Publish SUMMARY.md and complete the job.
