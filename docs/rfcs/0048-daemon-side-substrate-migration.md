# RFC 0048 — Daemon-side substrate migration (V2.0 phase)

**Status:** proposed
**Scope:** V2.0 (multi-week phase, NOT V1.7)
**Closes:** gemini A1 finding from dogfood-050 (substrate mismatch)

## Background

RFC 0043 V1 made Postgres the sole substrate at the schema level and
made the daemon required at the CLI level (`STRIATUM_DAEMON_REQUIRED`
enforcement). RFC 0043 V1.5 (dogfood-050) and V1.6 (dogfood-052)
closed the migration ergonomics + escape-path + locking gaps.

The gemini adversarial review of dogfood-050 identified a deeper gap:
the daemon's RPC server still delegates single-repo business logic
back to the SQLite-backed CLI dispatch (`DaemonRpcRouter._route` →
`striatum.api.invoke` → `striatum.db.connect` → SQLite). Even after
a successful `migrate-repo-local`, the daemon continues to read and
write SQLite for non-lifecycle verbs. The substrate flip is a
**facade** for single-repo operations.

Symptoms:
- After migrate-repo-local, the next daemon-mediated mutation sees
  `state.sqlite3` missing and creates a fresh empty SQLite (V1.6
  F-split-brain guards against this but reveals the underlying
  delegation issue).
- The `striatumd.*` Postgres tables for the migrated repo are
  read-only after migration unless the operator manually points
  the daemon at them.
- The Go daemon (RFC 0039) inherits the same delegation pattern —
  it registers RPC methods that return `not_implemented`, not real
  PG-backed handlers (codex F2 from dogfood-049).

## Goals (V2.0 phase)

- Port every single-repo mutation handler in `src/striatum/cli/`
  (mutations, evidence, recovery, worktree, run_summary, etc.) to
  read/write the daemon-owned Postgres schema directly, not through
  `striatum.db.connect`.
- Replace `DaemonRpcRouter._route` delegation to
  `striatum.api.invoke` with native PG-backed handlers.
- Implement the same mutation surface in `go/pkg/rpc/` / `go/pkg/apply/`
  so `--core go` actually services single-repo verbs (not just lifecycle).
- Remove the `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1`
  test-harness escape once the test suite has moved off SQLite-backed
  fixtures.

## Non-goals

- Removing SQLite as a development substrate entirely. The repo-local
  `.striatum/state.sqlite3` stays as the bootstrap path for `striatum
  init`; the daemon migrates it on first run.
- Hosted/cloud-mode daemon (deferred separately).
- Multi-tenancy enforcement (RFC 0027 follow-up).

## Phasing

This RFC is intentionally large. Implementation lands across multiple
dogfoods:

### Phase A — Method-by-method substrate port

- Port `mutations.py::register_session`, `claim_next`, `ack_work`,
  `complete_job`, `release_lease`, `block_job`, `record_verdict`,
  `submit_review`, `override_review_verdict` to PG-backed handlers
  in `src/striatum/daemon_pg/handlers/`.
- Port `recovery.py::stale_leases`, `requeue_stale`, `cancel_job`,
  `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`
  similarly.
- Port `evidence.py::evidence_export` to read from PG directly.

Each method is its own commit. The daemon RPC router swaps the
delegated handler for the PG-backed one as each lands. SQLite
delegation stays as a fallback for un-ported methods during the
transition.

### Phase B — Go core parity

- Implement every PG-backed handler in `go/pkg/rpc/` /
  `go/pkg/apply/` against `striatumd.*` Postgres tables.
- Wire `cmd/striatumd/main.go` to register the real handlers
  (currently registers fail-closed `not_implemented` returns per
  codex F2 from dogfood-049).
- Add cross-implementation parity tests:
  `make test-multi-repo CORE=python` and `CORE=go` produce
  byte-identical Postgres state for the same workflow input.

### Phase C — Migration sentinel & SQLite removal

- After all Phase A methods are ported, flip the default: daemon
  CLI verbs no longer fall back to SQLite delegation at all.
- Update `repo_local_migration.py` to flip
  `.striatum/state.sqlite3.migrated` sentinel to
  `.striatum/state.sqlite3.tombstone` immediately after the daemon's
  first successful PG write, eliminating the brief window where
  both substrates contain partial state.
- Deprecate the `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1`
  escape entirely; the test suite uses PG fixtures (existing
  `tests/_harness/pg.py` already provides this for multi-repo tests).

## Migration & rollout

- Each Phase A method ships in its own minor release (v1.5x.0).
- Phase B lands as v1.6x.0 (Go parity).
- Phase C lands as v2.0.0 (the actual substrate flip completion,
  superseding the V1.6 hardening).

Existing repos that have already run `migrate-repo-local` continue
to work — the daemon already has their PG state; Phase A handlers
just start reading it natively instead of routing through SQLite.

## Acceptance (per phase)

- Phase A acceptance per method: PG-backed handler passes the same
  pytest suite as the SQLite-backed equivalent, byte-identical state
  reads back, audit chain hashes match.
- Phase B acceptance: `make test-multi-repo CORE=go` runs the same
  workflow as `CORE=python` against the same PG instance, both
  produce identical state, audit chain hashes match across cores.
- Phase C acceptance: `STRIATUM_DAEMON_REQUIRED=0 +
  STRIATUM_TEST_HARNESS=1` no longer affects the test outcome (the
  pair becomes a no-op); the test suite is PG-fixture green
  without it.

## Open questions

1. Per-phase test fixture cost: Phase A and B both need running
   Postgres, which is currently optional in `make test`. Default:
   gate every Phase A test on the same `STRIATUM_PG_TEST_URL` /
   `STRIATUM_MULTI_REPO_REQUIRE_PG` sentinel pattern that V1.6
   F-ci closed in dogfood-049.
2. Operator UX during the transition: should the CLI surface a
   "this method is daemon-PG-backed" / "this method still routes
   through SQLite" indicator? Default: yes, via a `--explain` flag
   on `striatum doctor` and the daemon's startup banner.
3. Go core blocker: GH #2 + #5 lane evidence guard (RFC 0046) lands
   first; Phase B inherits it. No re-design needed.

## Provenance

- Striatum dogfood-050 (RFC 0043 V1.5) gemini A1 finding.
- Striatum dogfood-049 (RFC 0039 Phase 2) codex F2 finding.
- Striatum dogfood-052 (RFC 0043 V1.6) deferred A1 to V2.0.
- This RFC is the V2.0 design that closes both.
