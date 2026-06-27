# Apply — finalize RFC 0170 P0

You are the **author**, finalizing. Apply the accepted review findings and any required
revision. Ensure `cd go && go build ./... && go vet ./...` pass and **every** deliverable is in
place:

- `0045_cullable_entity.sql` (table + both CHECKs + grant; no owner DDL / FK / DELETE)
- the `RESERVATIONS.toml` row (`ordinal=45`)
- **both** authority-inventory rows; **no `SELECT *`** on `cullable_entity`
- the Tier-1 predicate (clauses 1–5)
- `DecayTickSweep.SweepOnce` **off the wait-gating path** with the latency machinery + the
  detached-goroutine `recover()` seam, wired into `cmd/striatumd/main.go`
- all tests: the **BC-618** known-set corpus (`D267` nominated / `D081` documented-withheld) +
  the protected-pathspec fixture + the bare-`superseded` negative control; the **B2** panic
  test; the **B5** HANG A/B + refresh-not-deferred test + its on-wait-path negative control; the
  **BC-619** late-return-zero-write guard; the **B3** cost test; **A5**; **A6**; the
  correctly-written two-role pgtest.

Publish **SUMMARY.md** enumerating: every file changed grouped by gate (G1–G4); each
gate-critical assertion (A1–A8, B1–B5, C1–C3, D1) and the named test that discharges it; the
BC-618 / BC-619 dispositions; the verified free `0045` slot; and any residual the verifier stage
must prove (the PG-backed pgtests, which the operator runs live). Source changes are captured via
`publish_source_changes` — this is the implementation the verifier stage seals and the operator
integrates to `main`.
