---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["rfc-0048", "phase-a", "ergonomics_dx", "build-review"]
---
author: reviewer-unknown-model-003

# RFC 0048 Phase A — Build Review (ergonomics_dx, claude lane, attempt 2)

Scope: developer-ergonomics review of the 16 ported daemon RPC handlers
(9 Track A workflow-loop + 7 Track B recovery/evidence), the
`DaemonRpcRouter` delegation swap, the test surface under
`tests/daemon_pg/handlers/`, and the operator runbook
`docs/POSTGRES_TRANSITION.md`. Reading scope was limited to both
`HANDOFF.md` files, `DESIGN_SYNTHESIS.md`, and the source/test trees
the synthesis points at.

## Cross-posture mandatory checks

| # | Check | Result |
|---|---|---|
| 1 | All 16 Phase A methods have a handler at the synthesis-locked path | PASS — 9 modules under `src/striatum/daemon_pg/handlers/workflow_loop/`, 7 under `recovery_evidence/`. |
| 2 | Each handler has a test file at the synthesis-locked path | PASS — 9 + 7 test files under `tests/daemon_pg/handlers/`. |
| 3 | `DaemonRpcRouter._route` routes those 16 names to the new handlers | PASS — `src/striatum/daemon_rpc/server.py:227-243` calls `resolve_pg_handler(envelope.method)` before the `CLI_ROUTES` fallback; the swap is one greppable call site and one greppable decorator. |
| 4 | Audit chain stays unbroken | PARTIAL — Track A's `context.append_event()` chains `previous_hash`/`row_hash` (`src/striatum/daemon_pg/handlers/context.py:108-205`), but stores them inside `payload_json._event_chain` because migration 0005 has no top-level chain columns. The synthesis (L23) mandated a Phase A migration adding `striatumd.events.previous_hash`, `row_hash`, and a `striatumd.repo_event_chain_heads` head table; that migration did not land. See finding #4. |
| 5 | Tests assert byte-equivalence vs SQLite-backed equivalent, not just "no exception" | FAIL — no test compares PG state to SQLite state with a per-key diff. See finding #1. |

Mandatory check #5 fails → verdict cannot reach `accept_with_findings`.

## Findings (ergonomics_dx posture)

### #1 — No byte-equivalence test asserts a diff on regression (HIGH)

- Evidence: `tests/daemon_pg/handlers/recovery_evidence/conftest.py:1-20`
  advertises a parity rig ("write the same fixture shape into both
  stores so handlers can be invoked on each side and asserted
  byte-equal"). The conftest defines `pg_ctx`, `sqlite_conn`, and a
  `Seed` dataclass — but no test in the seven Track B test files uses
  both fixtures together. The advertised `parity_seed` helper has no
  matching `@pytest.fixture` definition.
- The seven Track B test files (`test_stale_leases.py`,
  `test_requeue_stale.py`, …) are largely registration + signature
  + constant-list smoke (e.g.
  `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py:28,36,48-49`).
  The two non-smoke parity tests
  (`test_stale_leases.py:54-81`, `test_requeue_stale.py:55-68`) are
  gated by `@pytest.mark.skipif(not os.environ.get("RFC0048_PARITY"))`
  and skipped by default — and even when enabled they call only the
  PG handler and assert shape, never the SQLite equivalent.
- Track A's nine `tests/daemon_pg/handlers/workflow_loop/` files
  exercise PG state only (e.g. `test_ack_work.py:56-78`). No parity
  rig, no `assert pg_state == sqlite_state`, no per-key diff helper.
- Operator/maintainer cost: a future refactor to either substrate
  can silently diverge their outputs without a failing test. From a
  DX standpoint there is no signal that catches PG-vs-SQLite drift.
  The task prompt called out "no `assert state_a == state_b` without
  per-key diff"; this build has neither the comparison nor the
  diff helper.
- Required follow-up: implement the promised `parity_seed` fixture
  in `conftest.py`. Add at least one parity test per handler that
  (a) seeds the same fixture on both substrates, (b) invokes both
  paths, (c) diffs each row dict per repo-local table and per
  emitted event payload, and (d) raises an `AssertionError`
  containing the per-key diff (e.g. via `dictdiffer.diff` or a
  hand-rolled helper). The `RFC0048_PARITY=1` gate may stay, but
  the gate must run on at least one CI lane that has a
  CREATE-DATABASE-capable Postgres.

### #2 — `complete_inline` / `ack_inline` are referenced but never defined; `recovery.resume --complete` and `recovery.auto` live mode are dead code paths (HIGH)

- Evidence: `src/striatum/daemon_pg/handlers/recovery_evidence/resume_blocker.py:294-309`
  imports `complete_inline` from
  `striatum.daemon_pg.handlers.workflow_loop.complete_job`, but
  `complete_job.py` defines only `handle`. `auto_publish_stale_artifacts.py:337-351`
  makes the same import; `auto_publish_stale_artifacts.py:353-364`
  also imports `ack_inline` from `ack_work`. A
  `grep -rn "def complete_inline\|def ack_inline" src/striatum/`
  returns zero hits. Only `publish_artifact_inline` actually exists
  (`submit_review.py:80`).
- Behavioral consequence: every call to `recovery.resume --complete`
  raises
  `InvalidTransitionError("inline completion requires Track A's PG
  complete_job helper; rerun without --complete and call
  work.complete explicitly")`. Every live `recovery.auto` candidate
  raises
  `InvalidTransitionError("recovery.auto live mode requires Track
  A's PG publish/complete helpers; rerun with dry_run=true to
  preview, then publish + complete via the registered RPC verbs")`.
  The error strings are operator-actionable (good — see finding #7),
  but the features are unreachable.
- Track B's HANDOFF L240-258 flags the dependency clearly. Track A's
  HANDOFF does not call out that the cross-track helpers were not
  extracted — the integration is silently incomplete.
- Required follow-up: extract `complete_inline(ctx, *, session_id,
  job_id, lease_id, summary) -> dict[str, Any]` from
  `workflow_loop/complete_job.handle` and `ack_inline(ctx, *,
  session_id, message_id, lease_id) -> None` from
  `workflow_loop/ack_work.handle`. Update both Track B call sites
  to use them. Add a regression test that calls
  `recovery.resume --complete` and `recovery.auto` (live mode)
  against the PG fixture and asserts `completed_inline=True` plus a
  populated `complete` payload.

### #3 — Two divergent PG implementations of the same primitives (MEDIUM)

- Evidence: `expire_leases`, `maybe_complete_run`, and
  `is_repo_write` each exist twice with subtly different semantics:
  - Track A: `context.maybe_complete_run` at `context.py:437-503`
    emits `stop_reason="all_jobs_canceled"` and
    `session.closed.reason="run_canceled"`.
  - Track B: `_sql.maybe_complete_run` at
    `recovery_evidence/_sql.py:281-374` emits
    `stop_reason="job_canceled"` (or `"all_jobs_terminal"`) and
    `session.closed.reason="run_terminal"`.
  - Track A: `context._expire_leases` (inlined in
    `claim_next.py:159-216`) and `context.is_repo_write`
    (`context.py:294-296`).
  - Track B: `_sql.expire_leases` (`_sql.py:148-278`) and
    `_sql.is_repo_write_scope` (`_sql.py:103-127`).
- Maintainer cost: a maintainer touching either path has to know
  which copy applies. The two `maybe_complete_run` variants emit
  different `run.canceled` payloads and different `session.closed`
  reasons depending on which RPC method triggered the run-terminal
  state. This is precisely the silent divergence finding #1's
  parity test would catch.
- Required follow-up: lift these primitives to a single module
  (`daemon_pg/handlers/_primitives.py` or absorb the Track B copies
  into `context.py`) and delete the duplicates. Add a unit test
  asserting `recovery.cancel_job → run.canceled` and
  `work.complete → run.canceled` emit byte-equivalent event payloads
  when all jobs are canceled.

### #4 — Synthesis-mandated chain migration deferred; chain lives in `payload_json` only (MEDIUM)

- Evidence: `DESIGN_SYNTHESIS.md` L23 locks "add a Phase A migration
  for `striatumd.events.previous_hash`, `striatumd.events.row_hash`,
  and `striatumd.repo_event_chain_heads(...)`." Track A's
  `HANDOFF.md` L21-28 acknowledges the migration was not added, and
  `context.append_event` stores `_event_chain.previous_hash` /
  `_event_chain.row_hash` inside the JSON payload as a workaround.
  Synthesis L219-222 also requires
  `tests/daemon_pg/handlers/test_event_hash_chain.py`; that file
  does not exist in the tree.
- Operator cost: the chain is verifiable only by re-decoding
  `payload_json` of every event row. Downstream consumers
  (`evidence.export`, `striatum doctor`, audit dashboards) cannot
  `SELECT row_hash` directly; they must materialize JSON. Any
  future indexing or query optimization assumed by the synthesis
  cannot land until the migration ships. The DB-level guard the
  synthesis describes ("a handler that skipped `append_event`
  would be caught at INSERT") is not yet in force.
- Required follow-up: ship the chain-columns migration (operator
  write scope) and rewrite `context.append_event` so the chain
  hashes are columns, not nested JSON. Add the missing
  `test_event_hash_chain.py`. Until then, document the deviation in
  `POSTGRES_TRANSITION.md` (see finding #6) so operators inspecting
  events know where to look.

### #5 — `striatum doctor` / `daemon.describe` does not surface which methods are PG-backed vs SQLite-backed (MEDIUM)

- Evidence: `src/striatum/cli/introspect.py:1204-1248` defines the
  doctor check vocabulary. None of the 24 stable checks reports the
  routing status of any RPC method.
  `src/striatum/daemon_rpc/registry.py:18-46` builds
  `MethodEntry.public_dict()` from `(method, required_capability,
  repository_scope, repository_scope_mode, params_schema_version,
  audit_class, min_envelope, deprecated)` — no `substrate` or
  `pg_backed` flag. `daemon.describe` returns the same shape.
- A grep for `resolve_pg_handler` returns hits only inside
  `daemon_rpc/server.py`; the registry is invisible to the doctor
  and to `daemon.describe`. There is no way an operator can ask the
  running daemon "for this method, do you call a PG handler or fall
  through to the CLI?"
- Operator cost: during Phase A the routing is intentionally
  heterogeneous — 16 methods PG-native, ~30 still on `CLI_ROUTES`.
  A first-time operator who hits a bug cannot answer "is this
  method PG-backed yet?" without reading the source tree.
- Required follow-up: extend `MethodEntry.public_dict()` with a
  `substrate: "pg" | "cli_fallback"` derived at module import time
  from `_PG_HANDLERS.keys()`. Add a doctor check
  `pg_handler_coverage` listing each Phase A method alongside its
  resolved substrate; surface it in both `striatum doctor` and
  `striatum daemon doctor --verbose --json`.

### #6 — `docs/POSTGRES_TRANSITION.md` does not reflect the new substrate path for the ported methods (MEDIUM)

- Evidence: `docs/POSTGRES_TRANSITION.md:30-38` and L248-267
  characterize RFC 0048 as proposed and not started ("RFC 0048
  ports each handler to read/write the per-repo Postgres tables
  directly instead of delegating to `striatum.api.invoke` and
  `striatum.db.connect`. Until those phases land, an operator can
  still hit SQLite-backed code paths in test harnesses").
  Grepping the runbook for any of the 16 ported method names
  (`work.ack`, `work.claim_next`, `recovery.requeue_stale`,
  `recovery.stale_leases`, `evidence.export`, etc.) returns zero
  matches. The runbook has no entry for `daemon_pg.handlers.*`.
- The work-packet objective for this review states explicitly that
  "documentation under `docs/POSTGRES_TRANSITION.md` reflects the
  new substrate path for the ported methods." This is unmet.
- Operator cost: an operator following the runbook today is given
  the impression that no Phase A work has shipped, that all
  single-repo verbs still hit SQLite-backed code paths under the
  test-harness escape, and that no PG-native handlers exist.
- Required follow-up: add a "Phase A — ported methods" section
  between L247 and L248 enumerating the 16 methods with handler
  paths; cross-link to the doctor surface from finding #5; rewrite
  L30-38 so the framing distinguishes (a) ported, (b) still
  delegating, (c) Phase B Go parity.

### #7 — Error messages cite operator-actionable next commands (PASS — keep)

- Spot checks (good, no follow-up):
  - `register_session.py:62-64` →
    `"…pass --force-non-fresh --reason \"...\" to register a non-fresh reviewer explicitly"`.
  - `record_verdict.py:558-562` →
    `"…recovery: striatum supervise start --session-id {session_id}"`.
  - `resume_blocker.py:145-149` →
    `"…pass --force for GH #7 legacy process-adapter blockers on terminal jobs"`.
  - `resume_blocker.py:295-302` →
    `"rerun without --complete and call work.complete explicitly"`.
  - `auto_publish_stale_artifacts.py:347-351` →
    `"rerun with dry_run=true to preview, then publish + complete via the registered RPC verbs"`.
  - Track B's `next_actions` arrays (e.g.
    `stale_leases.py:88-104`, `resume_blocker.py:254-260,273-274`)
    give operators a concrete remediation path on success too.
- Weak spots (LOW; could be tightened but not blocking):
  - `complete_job.py:30` — `"job must be running before completion"`
    could name `work.ack` as the precursor verb.
  - `submit_review.py:111-116` — three back-to-back state-only
    refusals without naming the verb that would correct each state.
  - `process_reconcile.py` (read indirectly via `_sql.py`) —
    `"recovery.process_reconcile requires run_id"` could spell out
    `--run-id` for the CLI operator.

### #8 — Delegation-swap pattern is greppable (PASS — keep)

- The swap is one greppable call site
  (`src/striatum/daemon_rpc/server.py:227-243`) and one greppable
  decorator (`register_pg_handler("<method>")`). A
  `grep -rn "@register_pg_handler(" src/striatum/daemon_pg/handlers/`
  enumerates all 16 methods (plus the legacy undotted aliases
  declared on the same line, e.g. `ack_work.py:13` →
  `@register_pg_handler("work.ack", "ack")`). No decorators on
  `DaemonRpcRouter` hide which method routes where. The
  ergonomics_dx affordance lands.

### #9 — Stale test stubs for `workflow_loop.{record_verdict, submit_review, override_review_verdict}` (LOW)

- Evidence: `tests/daemon_pg/handlers/recovery_evidence/conftest.py:47-72`
  installs `sys.modules` stubs for the three Track A modules so
  the parent package `striatum.daemon_pg.handlers` can finish its
  `__init__` during collection. Track A's HANDOFF and the on-disk
  source tree confirm all three modules now exist (and Track A's
  own workflow-loop tests reach them via
  `tests/daemon_pg/handlers/workflow_loop/test_register_session.py:38-58`).
- The stubs no longer have a purpose but remain in the test
  harness. A future maintainer reading the conftest will assume
  Track A is still incomplete and may build new workarounds on
  top.
- Required follow-up: delete the stub block + comment; either
  re-import the real package or note the historical reason in the
  synthesis decision log.

### #10 — `handlers/__init__.py` silently swallows `recovery_evidence` import errors (LOW)

- Evidence: `src/striatum/daemon_pg/handlers/__init__.py:7-10`
  catches a bare `ImportError` from
  `from . import recovery_evidence` and sets `recovery_evidence =
  None`. A typo or syntax error in any Track B module would make
  `resolve_pg_handler` silently return `None` for every recovery
  method, causing the router to fall back to `CLI_ROUTES`
  (SQLite-backed) without any operator-visible diagnostic.
- This is precisely the silent-fallback failure mode RFC 0048 was
  created to surface. From an ergonomics standpoint a developer
  who breaks a Track B import would see "workflow_loop tests still
  pass" and no other signal.
- Required follow-up: replace the silent except with a one-time
  `logging.warning` at import time naming the failing module, and
  surface "expected PG handlers missing" in the doctor check from
  finding #5.

### #11 — `recovery.auto` vs `recovery.auto_publish_stale_artifacts` naming dual-life (LOW)

- Evidence: the synthesis section header at L171 is
  `recovery.auto_publish_stale_artifacts`. The registry entries in
  `daemon_rpc/registry.py:121-122` are `recovery.auto` (active)
  and `recovery.auto_publish_stale_artifacts` (deprecated alias).
  The handler file is `auto_publish_stale_artifacts.py`, but the
  decorator argument inside is `"recovery.auto"`. `CLI_ROUTES`
  (`server.py:83-84`) maps both names to the same CLI prefix.
- A developer grepping for `recovery.auto` finds three hits (the
  registry entry, the CLI_ROUTES entry, the decorator). Grepping
  for `recovery.auto_publish_stale_artifacts` finds the deprecated
  registry entry, the synthesis section header, the file name, and
  the second `CLI_ROUTES` mapping. They do not converge on a
  single canonical surface.
- Required follow-up: either fully retire
  `recovery.auto_publish_stale_artifacts` from the registry (one
  line; it is already deprecated) and rename the handler file to
  `auto.py`, or leave the dual-life but add a one-line comment at
  each greppable hit pointing at the canonical name.

## Verdict

`needs_revision`. The build does port all 16 methods and exposes a
greppable delegation surface with operator-actionable error
messaging (findings #7, #8), but multiple ergonomics_dx
affordances and one mandatory cross-posture check fail:

- Mandatory cross-posture #5 (byte-equivalence regression test
  with per-key diff) is absent — finding #1 (HIGH).
- `recovery.resume --complete` and `recovery.auto` live mode raise
  on every call because `complete_inline` was never extracted —
  finding #2 (HIGH).
- The operator-facing discovery surfaces (`striatum doctor` /
  `daemon.describe` and `docs/POSTGRES_TRANSITION.md`) give no
  indication that PG-native handlers exist, so a first-time
  operator cannot discover the new substrate path — findings #5
  and #6 (MEDIUM).
- Two divergent PG copies of `maybe_complete_run` / `expire_leases`
  / `is_repo_write` are a discovery and behavior-drift hazard —
  finding #3 (MEDIUM).
- The synthesis-mandated chain migration is deferred and the
  promised `test_event_hash_chain.py` is missing — finding #4
  (MEDIUM).

Re-review after findings #1 and #2 land and findings #5, #6 land in
docs; findings #3, #4, #9, #10, #11 are appropriate as
accepted-with-followups once the four primary issues are addressed.
