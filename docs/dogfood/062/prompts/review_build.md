# Build review (3-way)

Read `docs/dogfood/062/build/HANDOFF.md` and the implementation.
Produce `docs/dogfood/062/review/build/<lane>/REVIEW.md` per your
posture.

**codex `threat_model`:** gate on BOTH publish paths (PG + SQLite
mirror); SQL parameterized; operator-override audit row carries the
reason; the forgery-refusal test actually exercises the
no-process_executions case (not a different denial reason).

**claude `ergonomics_dx`:** refusal message + operator hint cite both
`striatum supervise start` and the `--allow-no-process-execution`
override; tests fail with operator-readable diffs; dashboard surfaces
missing-attestation on affected jobs.

**gemini adversarial `threat_model`:** attempt (a) leftover
process_executions row from prior session, (b) supervise.start that
wrote the row but subprocess crashed before running, (c) operator-
override used without justification, (d) race where supervise.start
records the row but subprocess crashes mid-run — does the gate
still pass? Document gaps in the finding.

**Write scope:** `docs/dogfood/062/review/build/<lane>/REVIEW.md`.
