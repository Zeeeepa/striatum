author: operator-claude-opus-1

# Dogfood-044 Phase 1 Operator Notes — RFC 0040 V1.5 Daemon-Side Dispatch + Composite Tools + Watcher

Run: `run_4fbb957eccfd4fc0aaaf91bc91b37c30`
Branch: `striatum/dogfood-044-rfc-0040-v1-5`
Workflow: `docs/dogfood/044/workflow.json` — 9-job single-track for
RFC 0040 V1.5 (F1-F6 codex findings from dogfood-040 build review
iteration 2)

## What shipped

RFC 0040 V1.5 closes the six dogfood-040 follow-up findings that were
deferred under the original cycle-exhaustion override. The fixes are
purely additive: the daemon RPC envelope-v1 wire protocol stays
unchanged, MCP `tools/list` and tool names + argument shapes stay
unchanged, and direct-Python `publish_on_behalf` callers see the same
success/refusal shapes (with two additive keys).

**F1 — Daemon MCP `tools/call` dispatch through the method registry.**
The previous stub that returned a fake `ok: true` is gone. A new
`src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
helper owns lookup, capability authorization, envelope build, and
routing through `DaemonRpcRouter.handle(...)`. `DaemonRpcRouter.handle`
gained `transport` (default `"rpc"`; MCP passes `"mcp"`) and
`require_handshake` (default `True`; MCP passes `False`). Audit rows
are post-dispatch: one `transport="mcp"` deny row on unknown method
or authorization denial, exactly one row from
`DaemonRpcRouter._record_and_return` on allowed call. MCP response
shape `{content, structuredContent, isError}` is preserved.

**F2 / F3 — `dogfood.publish_on_behalf` atomicity + verdict semantics.**
`publish_on_behalf` now runs ack / publish / verdict (or completion)
inside a single outer `with transaction(conn):` block via four new
transaction-free helpers. Review jobs require `verdict` up front;
`accept_with_findings`, `needs_revision`, etc. are validated against
the direct-Python enum; `findings_artifact_id` defaults from the
published artifact id when its kind is `finding`. On success exactly
one `dogfood.publish_on_behalf` event is inserted in-transaction with
`composition_steps`; on failure the transaction rolls back and a
best-effort `dogfood.publish_on_behalf_failed` event is written tagged
`outcome: "rolled_back"`.

**F4 — Watcher invocation in the daemon supervisor lifecycle.** New
`src/striatum/process_progress.py::progress_loop_once` runs one
bounded pass per repository, joined to the `runs` table so only
attached supervisors under running/paused runs tick. The daemon's
synchronous sweep loop owns the lifecycle — no per-supervisor
background threads. Heartbeat callbacks call
`striatum.cli.mutations.heartbeat` on the same repo connection.
Metadata-only events: `supervisor.progress_watcher_heartbeat`,
`supervisor.progress_watcher_idle`,
`supervisor.progress_watcher_lost`. Log contents are never read.

**F5 — Race / signal hardening.** `startup_grace_seconds` defaults to
60 s; within grace, a missing scratch path returns `waiting_for_log`
silently. The watcher tolerates `FileNotFoundError`/`OSError` while
scanning `*.log` files so rotated logs follow. The loop checks a
`should_stop` predicate between supervisors so SIGTERM cannot start a
new heartbeat after shutdown. `progress_advisory_lock` is shared with
`surgical_recovery` — watcher returns `lock_busy`, surgical recovery
returns `progress_lock_busy`. A PID-reuse guard via
`process_start_time(pid)` flips rows to `state='lost'` on mismatch.

**F6 — End-to-end execution-path tests.** New `test_mcp_dogfood_e2e.py`
drives MCP `tools/call` round-trips for `dogfood.publish_on_behalf`
covering completion + review-verdict paths (marked `multi_repo`).
`test_supervised_progress_watcher.py` extended with loop-wiring and
PID-reuse refusal cases. 42 passed / 10 skipped (multi_repo skips
without the PG harness).

## Stale-lease intervention — 30-min default-lease issue

Codex finished writing all the code for F1-F6, but the supervisor
lease expired before the implementer published the HANDOFF.md artifact.
The default 30-minute lease was not long enough for a packet of this
size (six findings across seven files plus tests + verification). The
operator composed `docs/dogfood/044/build/HANDOFF.md` on behalf of the
implementer by reading each per-finding implementation directly from
source. The byline `author: implementer-unknown-model-001` was used
because the operator did not have access to the codex session
metadata at the moment of publish.

This is the second time in recent dogfoods that the 30-min default
lease has bitten a long implementer packet. The harness gap is
separate from the V1.5 implementation scope, but it lives as an
ergonomic note for future workflow authors: when a packet covers
six independent findings, set `lease_seconds` higher or split the
packet. A future RFC could plumb a per-packet lease estimate into the
workflow generator.

## Fourth codex/codex anti-pattern instance

Build review verdicts:

- codex — `needs_revision`, high severity.
- claude — `accept_with_findings`, medium severity.
- gemini — `accept`, low severity.

D098 (`dec_242ea0b026d547c9baad9b353b149033`) overrides the codex
`needs_revision` verdict on the same grounds as D095 / D096 / D097:
when the codex implementer and a codex reviewer work on the same lane,
the reviewer's findings cluster around the implementer's own blind
spots. The other two cross-lane reviewers accept. With dogfood-044 we
now have **four** independent recurrences:

- D095 — dogfood-042 Track A (Go daemon core)
- D096 — dogfood-042 Track C (repo-local-state-to-Postgres design)
- D097 — dogfood-043 Python build (RFC 0045 V1)
- D098 — dogfood-044 build (RFC 0040 V1.5)

Four recurrences across three runs makes this the most reliably
reproducible review-pathology we have observed. The codex findings
from dogfood-044 are real — they describe useful tightenings, not
implementation gaps — and are absorbed into RFC 0040 V1.6 (TODO item
28) for a future dogfood.

TODO item 26 (forbid codex/codex implementer+reviewer pairing in the
workflow validator) is now the most-overdue harness improvement on
the list. The soft warning that landed in the dogfood-043 prep commit
fired again here. The full refuse-by-default behavior remains the
deferred half. After four instances, the empirical case is closed —
the question is no longer "is this a real pattern" but "what is the
validator override knob's UX". Suggested cut: validator rejects
same-model implementer↔reviewer pairs on the same lane unless
`workflow.review.allow_same_model_self_pair: true` is explicitly
declared in `workflow.json`.

## Gemini + claude byline-prefix bug — third instance

Both gemini and claude reviewers emit
`(role)-lane-unknown-model-NN` (e.g.
`reviewer-build-lane-unknown-model-001`) instead of the canonical
`(role)-unknown-model-NN` (e.g. `reviewer-unknown-model-001`). The
operator hand-edited the bylines before publication, but this is now
the **third of four** reviewed dogfoods where the operator has had to
make the same correction. The pattern looks like the lane id is being
spliced into the byline by the skill bundle's reviewer profile
fragment between the role and the model — a missing comma or pipe in
the byline template would explain the symptom. This belongs as its
own follow-up item adjacent to the codex/codex validator rule:
auditing the gemini and claude reviewer profile fragments in the
RFC 0015 skill bundle to ensure the byline prefix template emits
`<role>-<model>-<ordinal>` (no lane id), then either dropping the
operator hand-edit or making it explicit at the workflow level.

## Manual consolidate, dogfood-042/043 lesson applied

Dogfood-044's workflow intentionally did not include a `consolidate`
job. The operator writes the consolidate artifacts out-of-band as a
normal edit pass — this `PHASE_1_OPERATOR_NOTES.md`,
`BUILD_HANDOFF.md`, the changelog promotion, the RFC index status
bump, and the TODO follow-ups. The runner remains the source of truth
for what happened (`run_summary`, `OPERATOR_REPORT.md`, `D098`); the
operator is the right surface for the prose synthesis on top.

## Follow-ups

- **RFC 0040 V1.6** (TODO item 28): land codex needs_revision deltas
  from dogfood-044 review via a future dogfood.
- **TODO item 26** (codex/codex validator refuse-by-default): graduate
  the soft warning to a hard rule. Four-instance empirical case now
  closed.
- **Byline-prefix bug** (new follow-up): audit gemini + claude
  reviewer profile fragments in the RFC 0015 skill bundle so the
  byline template emits `<role>-<model>-<ordinal>` without splicing
  the lane id.
- **30-min default-lease issue** (new follow-up): allow per-packet
  lease tuning from the workflow generator for long implementer
  packets.

## Pointers

- `docs/dogfood/044/BUILD_HANDOFF.md` — combined handoff.
- `docs/dogfood/044/build/HANDOFF.md` — per-finding implementation
  handoff.
- `docs/dogfood/044/review/build/{codex,claude,gemini}/REVIEW.md` —
  three build review verdicts.
- `docs/dogfood/044/decisions/D098_cycle_exhaustion.md` — override
  decision artifact.
- `docs/dogfood/044/DESIGN_SYNTHESIS.md` — design synthesis input.
- `docs/dogfood/044/OPERATOR_REPORT.md` — per-intervention narrative
  authored during the run.
- `CHANGELOG.md` v1.33.0 — promotion entry.
- `docs/TODO.md` items 20 (✅ done) and 28 (V1.6 follow-up).
- `docs/rfcs/README.md` RFC 0040 row — status bumped to
  `accepted (V1 + V1.5 daemon-dispatch + watcher landed)`.
