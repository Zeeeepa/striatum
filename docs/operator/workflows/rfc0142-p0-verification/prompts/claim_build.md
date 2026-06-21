Build the slice and publish a claim_ledger. Give every capability claim a stable id, a status (VERIFIED|ASSERTED|DESIGNED), and (above DESIGNED) the id of the sanctioned check that substantiates it. Do not state a claim above the status its check can earn; deferral is a DESIGNED row, never hidden prose. You cannot author the sanctioned check set.

## For this run (RFC 0142 P0)

The P0 work is **already built and merged to `main`** (PR #553) — it is present in
your worktree (read `SEED.md` for the file list). You are NOT rebuilding it; you
are publishing the **claim ledger** the verify job will substantiate.

Claim the P0 capabilities, each at the status its sanctioned check earns:
- **ASSERTED** (backed by `builtin:go-build`, `builtin:go-vet`, `builtin:go-test`):
  the two-role fixture and the migration/test packages **build, vet, and pass
  their non-PG tests** (the static guard in `migrations_test.go` now runs on the
  shared `db.RuntimeOwnedTablesAlterable()` source). Give each a stable id and name
  the builtin check id that backs it.
- **DESIGNED** (no in-sandbox check earns more): the live `42501` red/green
  behavior of the PG-backed two-role suite. The no-network verifier sandbox cannot
  run a PG cluster, so this is a DESIGNED row with an evidence pointer to the
  out-of-band operator validation (live 8/8 under a non-superuser owner DSN, per
  `SEED.md`). Do NOT claim it ASSERTED/VERIFIED.

No claim may exceed what its receipt earns. Builtins cap at ASSERTED; VERIFIED is
out of scope for P0.
