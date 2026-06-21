# RFC 0142 P0 — verifier-workflow result (Stage 3)

The RFC 0141 `verification_gate` workflow (`run_d8108681…`, `vg_rfc0142_p0`) ran
against the **merged P0 work** (PR #553, `1ae38f14`): build shipped a claim ledger,
the verify lane ran `striatum verifier run` and minted sealed `receipt.v1`s, and the
adjudicator gated claims-vs-receipts.

## Minted receipts (sealed, this directory — verbatim from the run)

| check | exit | passed | classification | seal_digest |
| --- | --- | --- | --- | --- |
| `builtin:go-vet` | 0 | **GREEN** | asserted | `21d19c0c…` |
| `builtin:go-build` | 1 | RED | asserted | `8a35ce1a…` |
| `builtin:go-test` (`RECEIPTS.md`) | 1 | RED | asserted | `05f65a4c…` |

## The two REDs are environmental, NOT P0-code defects (both filed)

The verify lane diagnosed each RED honestly (see `VERIFY_SUMMARY.md`):

- **go-build / go-test RED → [#554](https://github.com/halbritt/striatum/issues/554):**
  the builtin checks don't pass `-buildvcs=false`, so go's VCS-status probe returns
  `exit 128` inside the strict bubblewrap sandbox when the per-job worktree's `.git`
  is a pointer file — the build aborts before compiling. A verifier limitation that
  blocks `go-build`/`go-test` for ANY repo under per-job worktree isolation; `go-vet`
  (no binary stamp) is unaffected.
- **go-test (direct) RED → [#555](https://github.com/halbritt/striatum/issues/555):**
  a pre-existing non-hermetic test (`TestSpawnRunAsSpecResolvesLaneUser` in
  `go/pkg/mutations`) fails only because this lane runs as OS user `striatum-lane`.
  Orthogonal to P0 (which never touched `pkg/mutations`).

The adjudicator correctly returned **needs_revision** on the RED receipts (the gate
must not clear over a RED), and the run was **banked** — the verifier workflow ran
and minted receipts, but honestly did not clear because of the verifier-in-worktree
limitation, not because P0 is unsound.

## P0 IS verified GREEN (independently)

- `go-build` / `go-vet` / `go-test` mint **passing ASSERTED** receipts against a
  standalone clone (real `.git` dir) — RFC 0142 Stage 3 direct verifier run,
  `~/striatum-rfc0142-verification-receipts/` (the #554 worktree limitation does not
  apply to a clone).
- CI on PR #553 is fully green (vet+lint + full pgtest suite + lane-isolation).
- The PG-backed two-role suite passes **live 8/8** against a real cluster under a
  non-superuser owner DSN — the #442/D248 `42501` oracle reproduces.

So P0 is sound and verified; the `verification_gate`'s own builtin go-build/go-test
just can't run honestly inside a per-job worktree until #554 lands.
