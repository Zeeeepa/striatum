# SEED — RFC 0142 P0 verification gate

This run **verifies the RFC 0142 P0 work** that was merged to `main` in **PR #553**
(`feat(pgtest): RFC 0142 P0 — two-role pgtest fixture + 42501 regression oracle`,
commit `1ae38f14`). It is the verifier-workflow (RFC 0141 `verification_gate`) pass
over the completed build: the builder ships a claim ledger, the verify job runs
`striatum verifier run` against sanctioned builtin checks and mints sealed
receipts, and the adjudicator gates the cleared release on those receipts.

The run forks from `main`, so the P0 code is present in every job's worktree.

## The work under verification (already on `main`)

- `go/pkg/pgtest/two_role.go` — `pgtest.TwoRole(t)`: the two-role fixture
  (dedicated non-superuser, non-owner LOGIN SUT role; prod-faithful ownership
  bootstrap; isolation self-check).
- `go/pkg/db/two_role_pg_test.go` — the `42501` red oracle + green control +
  ownership-fidelity differential + dynamic-touch form.
- `go/pkg/db/migrations_two_role_pg_test.go` — full-migration-as-runtime-role +
  forbidden-owner-FK red.
- `go/pkg/db/owner_runtime_ownership.go` — `db.RuntimeOwnedTablesAlterable()`
  (the shared owner-held source: "one source, no drift").
- `go/pkg/db/migrations_test.go` — static guard refactored onto that shared source.

It discharges the Stage 1 design committee's binding constraints C1–C5 (see
`docs/operator/workflows/rfc0142-design-falsification/committee-output/`). It is
test-harness + test code only — no migration, no owner bundle, no daemon change.

## Sanctioned checks (builtins — self-pinning, cap at ASSERTED)

- `builtin:go-build` — the module builds.
- `builtin:go-vet` — the module vets clean.
- `builtin:go-test` — the module's tests pass (the PG-backed two-role suites skip
  without a cluster bound; the non-PG tests, incl. the refactored static guard
  in `migrations_test.go`, run).

No `allowlist.intent.json` entry is needed — builtins self-pin and need NEITHER
`--allowlist` NOR `--intent`. `VERIFIED` is reserved for an external,
operator-pinned-and-attested check (RFC 0141); P0's builtins cap at **ASSERTED**.

## Out-of-band evidence (DESIGNED-level here, not re-runnable in the sandbox)

The PG-backed two-role suite was validated **live, 8/8, against a real cluster
under a non-superuser owner DSN** (the `42501` oracle reproduces) — operator
validation outside the no-network verifier sandbox. State this as a DESIGNED-level
claim with that evidence pointer; do NOT claim it VERIFIED (the sandbox cannot run
it).

## Honesty rule

Receipts come from the engine's exit codes, not prose. No claim above the status
its receipt earns. A check whose negative control passes voids the receipt → RED.
