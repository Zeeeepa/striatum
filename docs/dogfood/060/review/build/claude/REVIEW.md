---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
---
author: reviewer-unknown-model-002

# RFC 0048 Phase C Read Handlers — Build Review (ergonomics_dx)

Scope: the 12 read RPC handlers under
`src/striatum/daemon_pg/handlers/reads/` and the matching tests under
`tests/daemon_pg/handlers/reads/`. Read from a first-time-operator
perspective: would migrating off the SQLite path leave operator
affordances discoverable, consistent, and faithful to the pre-migration
contract?

## Cross-posture mandatory checks

| # | Check | Result | Evidence |
|---|---|---|---|
| 1 | Every locked read method has a handler file | PASS | 12 handler modules present under `reads/`, listed in `reads/__init__.py:5-16`. |
| 2 | Every handler has a test file with parity assertion per synthesis strategy | **FAIL** | `tests/daemon_pg/handlers/reads/conftest.py:1-41` only ships `empty_ctx`. None of `ReadSeed`, `pg_ctx`, `sqlite_conn`, `parity_seed`, `assert_payload_parity` from the synthesis lock (`DESIGN_SYNTHESIS.md:29`, `DESIGN_SYNTHESIS.md:19`) is implemented. Committed tests are shape/error/empty smoke (e.g. `test_status.py:6-22`, `test_doctor.py:6-12`, `test_list_runs.py:9-16`) — explicitly the mode the synthesis rejected: "Shape-only smoke is rejected because `status`, `doctor`, and evidence shapes have changed recently enough that smoke would miss real regressions." |
| 3 | Every handler scopes by `ctx.repository_id` | PASS | Every SELECT in the new package includes a grep-visible `repository_id = %s` predicate; spot-checked in `_read_model.py:39-92`, `list_runs.py:19-34`, `list_sessions.py:23-39`, `list_jobs.py:22-41`, `why.py:138-147`. Joins repeat `repository_id` (e.g. `list_runs.py:25-27`). |
| 4 | `DaemonRpcRouter._route` actually picks up the new handlers | **PARTIAL FAIL** | Registration works: `src/striatum/daemon_pg/handlers/__init__.py:12` imports `reads`, and `tests/daemon_pg/handlers/reads/test_registration.py:28-31` asserts each of the 12 methods resolves. **But the operator-facing CLI translator was a synthesis prerequisite (`DESIGN_SYNTHESIS.md:18`) and is still unwired**: `src/striatum/cli/daemon_rpc_route.py:205-207` returns `(f"list.{sub}", {})` — `state`, `role`, `lane`, `workflow_job_id`, `kind`, `limit` are silently dropped at the CLI layer. And `("corpus", "export")` is missing from `_LOOKUP` (`daemon_rpc_route.py:391-426`), so `striatum corpus export` never reaches the new handler. HANDOFF acknowledges the gap (`HANDOFF.md:81-85`). |
| 5 | Tests run by default — no `RFC0048_PARITY` env-gating, no `@pytest.mark.skip` on parity | PASS-ish | No env-gate string in tests. The PG-fixture tests in `test_list_read_handlers.py:22` use `pytest.mark.multi_repo`, which the synthesis allows ("with the existing `tests/_harness/pg.py` reachability gate"). The deeper concern is mandatory #2 — there is nothing for the marker to skip, since per-key parity asserts aren't written. |

Two mandatory checks fail (#2, #4-CLI-leg). Verdict degrades to
`needs_revision` regardless of posture-specific findings below.

## Ergonomics_dx findings

### HIGH

**E1. `next_actions` is not parity with legacy `striatum status`.**
The PG handler synthesizes its own action vocabulary in
`_read_model.py:265-277`: `resolve_human_checkpoint`, `inspect_blockers`,
`inspect_review_verdicts`, `claim_next`, `inspect_status`. The legacy
strings, which the dashboard, web UI parity tests, and operator scripts
key on, are different:
`src/striatum/cli/introspect.py:857-913` returns
`claim_available_work`, `inspect_packet_with_inbox`,
`recover_orphan_supervisor`, `recovery_auto_publish`, `inspect_blocker`,
`export_run_evidence`, `derive_expected_byline`, … . The PG handler also
never passes `has_orphan_supervisor` or `has_stale_leases` into the
decision. Result for a first-time operator on a migrated repo:
`striatum status --json` `next_actions` becomes a strict subset with
renamed entries, dashboards lose self-heal hints, and any script keying
off the legacy strings silently stops triggering.
*Required follow-up:* call the existing
`striatum.cli.introspect.next_actions(...)` (or re-implement
key-for-key) from the new `status_payload`, and feed
`has_orphan_supervisor` / `has_stale_leases` from PG.

**E2. `doctor` block in `run.summary` and `evidence.export` is faked.**
`run_summary.py:49` hardcodes
`"doctor": {"ok": True, "schema_version": 5, "problems": []}` into
`run_summary_payload`. `evidence_export.py:22` does the same for the
exported evidence Markdown. The new `doctor` handler exists
(`reads/doctor.py:16-58`) and the synthesis lock for `run.summary`
listed the doctor read set in its tables-queried column
(`DESIGN_SYNTHESIS.md:76`). For an operator reading
`docs/.../RUN_SUMMARY.md` post-migration, the embedded doctor section
will always say "ok" even when there are open problems — a regression
from the SQLite path which embedded real doctor output.
*Required follow-up:* call the `doctor` PG read-model from
`run_summary_payload` and from `evidence_export.handle`, scope by
the run_id under review, propagate the same redactor for the evidence
case.

**E3. Synthesis-locked parity rig is absent.** The synthesis named the
exact shared helpers (`DESIGN_SYNTHESIS.md:29`,
`DESIGN_SYNTHESIS.md:19`) and the rejection of shape-only smoke.
`tests/daemon_pg/handlers/reads/conftest.py:1-41` only ships
`EmptyCursor`/`EmptyConnection`/`empty_ctx`. There is no
`assert_payload_parity` that prints a per-key diff with normalized
timestamps and export paths. Every committed test is either a shape
assertion (`test_status.py:6-22`, `test_dashboard.py:9-13`,
`test_list_runs.py:9-10`, `test_doctor.py:6-12`), a single-row spot
check (`test_list_read_handlers.py:181-287`), or a `RpcError` raise
test. From an ergonomics-DX view, a parity failure on `status`,
`run.summary`, or `evidence.export` would dump two opaque dicts at the
operator instead of the per-key diff the synthesis required for
debuggability.
*Required follow-up:* build the locked
`tests/daemon_pg/handlers/reads/conftest.py` rig (SQLite + PG seeded
from one fixture, run-then-diff helper, normalize timestamps and
generated paths) and replace the smoke asserts with per-key parity for
at least `status`, `dashboard`, `list.*`, `run.summary`,
`evidence.export`, and `corpus.export`.

**E4. CLI translator gap means filters silently no-op for the
operator.** `daemon_rpc_route.py:205-207` `_route_list` ignores every
filter the new handlers accept. `corpus.export` is not routed at all
(`daemon_rpc_route.py:391-426`). A first-time operator runs
`striatum list runs --state running --json` post-migration and gets all
runs back — the PG handler correctly validates and would filter, but
the CLI never asks it to. HANDOFF (`HANDOFF.md:81-85`) flags this as
out-of-scope; the ergonomics impact is still real: from the user's
perspective the operator escape (`STRIATUM_DAEMON_REQUIRED=0`
`STRIATUM_TEST_HARNESS=1`) is *required* for any filtered list call or
corpus export to behave correctly post-migration, contradicting the
review-packet acceptance bullet about the escape becoming optional.
*Required follow-up:* land the translator changes in a follow-up packet
that is explicitly scoped to `daemon_rpc_route.py`; until then, ship a
release note that the list filters and `corpus export` paths still go
through legacy SQLite.

### MEDIUM

**M1. Error envelopes do not cite remediation commands.** The
packet objective ("error messages on `repo_not_registered` cite the
remediation command") is not met inside `reads/`. Grep for
`migrate-repo-local`, `Did you`, or `try `: zero matches in the new
package. All `RpcError`s in `reads/_sql.py:55-148` and
`reads/why.py:123-126` are bare messages: `"run not found: {run_id}"`,
`"limit must be a positive integer"`, `"target id is not a known run,
job, message, blocker, artifact, verdict, session, or process"`. The
unknown-target message in `why.handle` is particularly cryptic for a
first-time operator — it lists eight target types but offers no hint
to run `striatum list runs --json` to find one.
*Required follow-up:* either route remediation hints through the
router envelope (preferred — keeps handlers terse) or extend the
shared `_sql.py` raise sites to include "next: <command>" suffixes.
At minimum, `not_found` for `run_id` should cite
`striatum list runs --json` and `unknown target` should cite
`striatum list jobs`/`status`.

**M2. `corpus.export` does not filter the audit chain by `since`.**
`reads/corpus_export.py:118-145` enumerates *all* events for the
repository via `events_for(ctx, limit=10000)`; the `since_commit`
parameter is consulted only for filesystem enumeration. Legacy
behavior in `striatum.corpus.export.export_corpus_bundle` filters
audit-chain entries by the same `since` ref, so an operator running
`striatum corpus export --since HEAD~1` post-migration will see a
bundle whose `audit_chain_entry` JSONL contains the entire repo
history. Bundle hash is sensitive to this — parity comparison against
the SQLite output would diverge if it existed.
*Required follow-up:* either filter `events_for` by
`created_at >= <since_commit_timestamp>` (the legacy criterion is
commit-time, normalize accordingly) or document the deliberate
deviation in HANDOFF so operators know to expect a larger audit
slice.

**M3. `list.jobs` skips the synthesis-locked lazy lease expiry.** The
synthesis lock (`DESIGN_SYNTHESIS.md:65`) explicitly listed
`leases(lease_id,run_id,resource_id,state,expires_at) for lazy expiry`
as part of the `list.jobs` read set. The handler in
`reads/list_jobs.py:22-41` issues no lease query and never expires
stale leases. From an operator perspective, `striatum list jobs` on a
migrated repo will keep returning a job in `claimed` state after its
lease has expired, until something else triggers expiry — a regression
from the SQLite path which expired before reading.
*Required follow-up:* either add an `expire_leases` call (re-using the
Track B helper the synthesis referenced) or document that lazy expiry
moved to a separate recovery sweep.

**M4. `evidence_artifact_summaries` has a no-op self-assignment.**
`_read_model.py:429` is literally
`item["artifact_kind"] = item.pop("artifact_kind")`. It's harmless but
it signals leftover refactor noise — a first-time reader has to puzzle
out the intent.
*Required follow-up:* delete the line.

**M5. `_REPOSITORY_SCOPE_PREDICATE` constant is dead code.**
`reads/dashboard.py:14` defines the module-level constant but never
references it (the SQL lives in `_read_model.dashboard_payload`).
Misleads anyone grepping for repository-scope discipline from this
handler.
*Required follow-up:* remove the constant.

### LOW

**L1. `doctor` only implements one of the legacy `DOCTOR_CHECKS`.**
`reads/doctor.py:29-54` runs the
`completed_job_missing_expected_artifact` check and nothing else. The
synthesis test contract (`DESIGN_SYNTHESIS.md:102-103`) asked for
"per-check parity for every stable `DOCTOR_CHECKS` name and an
all-clean baseline." HANDOFF (`HANDOFF.md:37`) acknowledges this is
deferred. From the operator perspective, `striatum doctor` post-
migration will return `ok=True` for repos that the legacy doctor would
have flagged for stuck queues, missing worktrees, supervisor mismatch,
plugin layout, etc. Listed LOW only because HANDOFF flagged it; the
operator-experience hit is real.
*Required follow-up:* port the rest of `DOCTOR_CHECKS` (or fold the
remaining checks behind explicit operator-visible "deferred" output
keys so they don't silently disappear from the output).

**L2. `_registry.read_only=True` wrapper is misleading.**
`reads/_registry.py:11-21` accepts `read_only=True` and discards it.
A future reader looking for the read-only enforcement seam at the
registry will conclude one exists; in reality the metadata is dropped.
HANDOFF documents this. The decorator argument should either land in
the shared registry (Phase D) or the wrapper should warn until it
does.
*Required follow-up:* either drop the parameter entirely and let
Phase D add it once `registry.py` is in scope, or store the metadata
on the function object as `handle.read_only = True` so test introspection
has something to read.

**L3. `why` raises `schema_invalid` with a wrong-field message.**
`reads/why.py:130-134` raises `schema_invalid` saying
`"target_id must be a non-empty string"` even when the operator passed
`id` (the legacy alias) as an empty string. The message ought to name
the field that was empty.
*Required follow-up:* validate `id` and `target_id` separately so the
operator-facing error names the field they actually used.

**L4. Front-matter `inputs` in HANDOFF lists the design REVIEW.md,
which the synthesis lock did not name as an implementer input.**
`HANDOFF.md:4` declares
`inputs: ["docs/dogfood/060/DESIGN_SYNTHESIS.md", "docs/dogfood/060/review/design/REVIEW.md"]`.
The packet contract for build expected the synthesis as the lock.
Listed LOW because it has no runtime impact; affects future provenance
audits.

## Summary

- Cross-posture mandatory: 1 PASS, 2 FAIL (per-key parity rig
  missing; CLI translator wiring missing), 1 PASS-with-caveat, 1 PASS.
- HIGH ergonomics regressions: 4 (`next_actions` parity, faked doctor in
  exports, no per-key diff rig, CLI filter no-ops).
- MEDIUM: 5. LOW: 4.

The work delivers correct repository-scoped SQL and a complete
12-handler footprint; the gaps are concentrated at the *parity surface*
the synthesis used as its acceptance criterion (per-key diffs against
legacy + faithful operator strings). The CLI translator gap is the
single largest first-time-operator regression: it makes the
`STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape effectively
mandatory for any filtered list call or corpus export on a migrated
repo, which inverts the packet's acceptance bullet about that escape
becoming optional.

## Verdict

`needs_revision` — mandatory-check failures (#2 parity rig, #4 CLI
translator wiring) plus HIGH ergonomics findings (E1 `next_actions`,
E2 faked doctor, E3 missing parity rig, E4 CLI filter no-ops).
