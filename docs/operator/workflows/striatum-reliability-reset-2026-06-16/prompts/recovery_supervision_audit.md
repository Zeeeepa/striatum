# Recovery And Supervision Audit

Audit recovery, supervision, lane delivery, liveness, leases, auto-finalization,
and `run drive`.

Look for places where Striatum appears correct in the database but remains
operationally wedged, where recovery depends on lease timing rather than true
liveness, where supervised lanes silently degrade, or where operator recovery
ceremony is excessive.

Write `docs/operator/artifacts/striatum-reliability-reset-2026-06-16/recovery_supervision/REVIEW.md`.
Include the exact author line from your work packet.

Required sections:

- Most painful recovery/supervision failure modes.
- Timing/liveness/lease hazards.
- Silent-degrade hazards.
- Places where recovery should become automatic, louder, or simpler.
- P0/P1 fixes with exact tests.
- What not to build until this is stable.

