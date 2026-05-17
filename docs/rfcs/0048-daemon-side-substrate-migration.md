# RFC 0048 — Daemon-side substrate migration

**Status:** accepted / completed (v1.49.0-v1.55.0)
**Scope:** completed production handler-port and fallback-closure work
**Closes:** gemini A1 finding from dogfood-050 (substrate mismatch)

Completion note: RFC 0048 is no longer pending V1.5 hardening. By
v1.55.0 production mapped verbs are PG-native/fail-closed through daemon
RPC, `CLI_ROUTES` fallback is empty, and legacy SQLite remains only for
migration/test-fixture compatibility paths.

## V1 Phase A landing summary (2026-05-14, v1.49.0)

dogfood-057 landed the Python-side handler port for all 16 single-repo
mutation methods under `src/striatum/daemon_pg/handlers/`:

- **Track A** (`workflow_loop/`, codex): `register_session`, `claim_next`,
  `ack_work`, `complete_job`, `release_lease`, `block_job`,
  `record_verdict`, `submit_review`, `override_review_verdict`.
- **Track B** (`recovery_evidence/`, claude): `stale_leases`,
  `requeue_stale`, `cancel_job`, `process_reconcile`, `resume_blocker`,
  `auto_publish_stale_artifacts`, `evidence_export`.
- Shared infrastructure: `handlers/__init__.py`, `handlers/registry.py`,
  `handlers/context.py`. `DaemonRpcRouter._route` resolves the PG
  handler before the legacy `CLI_ROUTES` fallback. Decorator-based
  self-registration is the integration boundary between tracks.

**Operator landing footnote.** The dogfood ran in legacy SQLite mode
(`STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`) because
RFC 0048 itself addresses the daemon-RPC accept-loop / substrate-facade
gap; that gap made the daemon-required CLI path non-functional on the
branch. State for the live run was lost when the per-repo `striatum
serve` was restarted mid-run; the on-disk artifacts and ported handler
code survived because the work-packet model writes durable artifacts
before any callback. The repo-local SQLite ended in a corrupted state
under concurrent migration + serve + supervisor writes; it was
quarantined as `.striatum/state.sqlite3.corrupt` and reset via
`striatum init`. Postgres `striatum_daemon` retains the pre-rollback
73-run snapshot.

## V1.5 follow-up — completed hardening

The build-review verdicts on V1 Phase A flagged real findings that did
not block the V1 landing. They were addressed during the v1.49.0-v1.55.0
RFC 0048 completion work:

- **F1 (codex threat_model)** — fail-closed routing rule: once a method
  is registered as PG-backed, all PG handler exceptions, capability
  denials, and parameter validation failures must return an RPC error
  and must NOT fall back through `striatum.api.invoke` /
  `striatum.db.connect` / SQLite-backed dispatch. Add a per-method
  negative test that monkeypatches the PG handler to raise and asserts
  no SQLite read/write occurs.
- **F2 (codex threat_model)** — capability-denial test coverage for
  every PG write handler: missing token, revoked, expired, wrong
  capability, wrong repository scope, replay. Assert no workflow-table
  mutation, no audit-row append on the allow path, and a denied audit
  row with the documented reason.
- **F3 (codex threat_model)** — audit-chain concurrency: each PG write
  handler appends audit + workflow events in a short `SERIALIZABLE`
  transaction or an explicit row-locking protocol. Add a concurrent
  test for overlapping allowed and denied requests across claim,
  publish-artifact, verdict, complete, recovery paths; verify a single
  contiguous audit chain and no orphan workflow mutations.
- **F4 (codex threat_model)** — append-only role enforcement: privilege
  tests that the daemon read-write role cannot update or delete
  `striatumd.events` or `striatumd.artifacts`. Audit per-handler use of
  `ON CONFLICT DO UPDATE` patterns on append-only rows.
- **#1 (claude ergonomics_dx, HIGH)** — actual byte-equivalence parity
  tests: the parity rig advertised in
  `tests/daemon_pg/handlers/recovery_evidence/conftest.py` (with `Seed`
  dataclass + `pg_ctx` + `sqlite_conn`) is unused. Wire the seven
  Track B tests + nine Track A tests to assert per-key state diffs
  between PG and SQLite paths on the same input fixture. Remove the
  `RFC0048_PARITY` skipif gating so parity runs by default once parity
  is achieved.
- **#2 (claude ergonomics_dx, HIGH)** — dead code paths: `complete_inline`
  and `ack_inline` are referenced but never defined; `recovery.resume
  --complete` and `recovery.auto` live mode are unreachable from any
  caller. Define + wire, or delete.
- **#4, #6 (claude ergonomics_dx, MEDIUM)** — the synthesis-mandated
  `striatumd.events.previous_hash` / `row_hash` column migration was
  deferred; chain metadata currently lives inside
  `payload_json._event_chain`. Land the schema migration and re-anchor
  existing rows. Update `docs/POSTGRES_TRANSITION.md` to reflect the
  new substrate path for the ported methods.

A separate `dogfood-058-rfc-0048-v1-5` fix-up dogfood will scope these
items explicitly.

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

### Phase B — Go read/support parity

Phase B shipped read-handler parity and selected mutation plumbing in the Go
support tree. D105 temporarily narrowed Go away from a peer production daemon
core, but D107 / RFC 0068 supersedes that boundary and makes remaining Go
daemon parity active port work. This does not reopen RFC 0048's completed
Python/Postgres substrate cutover.

### Phase C — Migration sentinel & SQLite removal (completed)

- Daemon CLI verbs no longer fall back to SQLite delegation in production.
- `repo_local_migration.py` flips
  `.striatum/state.sqlite3.migrated` sentinel to
  `.striatum/state.sqlite3.tombstone` immediately after the daemon's
  first successful PG write, eliminating the brief window where
  both substrates contain partial state.
- The `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1` escape is
  test-harness compatibility only and not an operator run mode.

## Migration & rollout

RFC 0048 completed across v1.49.0-v1.55.0. Phase A handler porting, Phase C
CLI fail-closed routing, and V1.5 hardening are no longer pending. Go parity
work now continues under D107 / RFC 0068, while this RFC remains limited to
the completed Python/Postgres substrate flip.

Existing repos that have already run `migrate-repo-local` continue
to work — the daemon already has their PG state; Phase A handlers
just start reading it natively instead of routing through SQLite.

## Acceptance (completed)

- Phase A acceptance per method: PG-backed handlers pass the same
  pytest suite as the SQLite-backed equivalent, byte-identical state
  reads back, audit chain hashes match.
- Phase B acceptance: Go read/support parity was retained as developer-harness
  evidence; D107 / RFC 0068 owns production Go daemon parity.
- Phase C acceptance: production mapped verbs fail closed without the daemon;
  the paired SQLite escape is test-harness/migration-only.

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
