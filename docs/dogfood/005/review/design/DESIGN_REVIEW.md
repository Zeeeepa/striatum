---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["dogfood-005", "rfc-0014"]
---

# RFC 0014 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Target: `docs/dogfood/005/DESIGN_SYNTHESIS.md`
Read (fresh, artifact_augmented):
- target synthesis;
- `docs/dogfood/005/research/CURRENT_ADAPTER.md`;
- `docs/rfcs/0014-process-adapter-completion-guarantees.md`.

Verdict intent: **accept_with_findings**.

The synthesis is implementation-ready. The schema-drift discoveries
in research are correctly absorbed (migrations v8 + v9), the
diagnostic envelope is JSON-schemed and D028-safe, the blocker
vocabulary covers all five states with explicit priority, and the
test plan is exhaustive. Findings below are improvement-grade
refinements; nothing blocks acceptance.

## Assessment Against Review Criteria

### Diagnostic envelope contains zero child stdout/stderr

**Yes.** Synthesis § 2 enumerates exact fields; none reference
child output. The regression test
`test_envelope_contains_no_stdout_stderr` (synthesis § 10) adds
an explicit assertion. D028 preserved by construction.

### Envelope storage in existing-table column

**Yes (with one schema addition).** Synthesis § 1 adds
`blockers.payload_json` via migration v9. This is one new column,
not a new table. The research handoff convinced me this is
the right call — putting the envelope only on the event row would
make `striatum dashboard` and the proposed RFC 0013 web UI need a
join to render blockers.

### Blocker-reason vocabulary completeness

**Yes.** Five reasons covering exit-zero+missing-artifact,
exit-zero+missing-verdict, nonzero-exit, timeout, and
post-reconcile-with-missing-outputs. Priority order is explicit
(synthesis § 3) so multi-condition cases are deterministic.

### CLI surface lands cleanly

**Yes.** `--timeout-seconds` is additive (default unbounded);
`recovery process-reconcile` mirrors `requeue-stale`. Existing
callers see no behaviour change.

### Test plan covers reconciliation + reproduction

**Yes.** Synthesis § 10 includes
`test_reconcile_transitions_dead_pid_to_lost`,
`test_reconcile_keeps_alive_pid_running`, and
`test_issue_one_reproduction`. The reconciliation tests use
real subprocess + `os.kill(pid, 0)`, matching the production
path.

### Schema impact honestly accounted for

**Yes.** Two migrations explicitly listed (v8 enum + v9 column).
The research handoff caught the `payload_json` drift; the
synthesis adopts the migration. Migration test
`test_migration_v8_v9_idempotent` covers fresh-vs-migrated
parity.

### Deferrals are explicit

**Yes.** Synthesis § 13 lists six deferred items; § 14 acknowledges
reviewer-likely pushback on each.

## Findings

### F1 (low) — `process_lost_with_outputs_missing` priority unspecified

**Issue.** Synthesis § 3 lists priority for the four primary
reasons but doesn't address `process_lost_with_outputs_missing`.
That reason fires from the reconciler, which runs in a different
codepath than the inline post-exit validator. In principle, a
process could be `lost` AND have a non-zero exit code recorded
elsewhere (rare but possible); the priority is undefined.

**Recommendation.** Synthesis § 3 should add: "Reconciler-path
blockers (`process_lost_with_outputs_missing`) only fire from
`recovery process-reconcile` and never compete with inline-path
blockers. The reconciler skips rows that already have an open
blocker for the same job."

### F2 (low) — Single event type may obscure SSE filtering

**Issue.** Synthesis § 4 pins one event type
(`process_adapter.outputs_missing`) for all five blocker reasons.
A future SSE consumer (RFC 0013 web UI) may want to filter by
specific failure mode (e.g., highlight timeouts differently from
output-missing). Today's design forces consumers to inspect
`payload_json.envelope.blocker_kind`.

**Recommendation.** Either (a) keep one event type and document
that SSE consumers must inspect the envelope's `blocker_kind`,
or (b) emit per-kind events
(`process_adapter.outputs_missing`,
`process_adapter.timeout`,
`process_adapter.exit_nonzero`,
`process_adapter.lost`). I lean (a) for V1 (simpler;
SSE filtering is a UI concern); flag here for the implementer to
note in the BUILD_HANDOFF.md.

### F3 (medium) — `lanes.<id>.adapter_timeout_seconds` validation needs upper bound

**Issue.** Synthesis § 6 says "positive integer when present" but
gives no upper bound. An operator typo like
`adapter_timeout_seconds: 1800000` (intending 1800) would silently
become a 20-day timeout that effectively blocks the job's lease
expiry from cleaning up.

**Recommendation.** Cap at a reasonable upper bound (e.g., 86400
= 24 hours) and reject larger values at workflow validation time.
Anyone with a legitimate >24h need can override at CLI invocation
time with `--timeout-seconds`. Add a validation test.

### F4 (low) — Reproduction fixture path collides with write_scope

**Issue.** Synthesis § 12 places the reproduction fixture at
`examples/process-adapter-failure-fixture/workflow.json`. The
implementer's write_scope in dogfood-005 includes `examples/`,
so this is fine. But the fixture's job declares
`expected_artifacts.path: docs/demo/OUT.md`, which would collide
with any other workflow that writes to that path. Better to
namespace under
`examples/process-adapter-failure-fixture/out/OUT.md`.

**Recommendation.** Adopt the namespaced output path. Trivial
change; flag at implementer time.

### F5 (info) — `process_executions` schema lives in `process_adapter.py`, not `schema.py`

**Issue.** Research § "Drift 2" / § "Friction 2" notes the
`PROCESS_SCHEMA_SQL` lives inline in `process_adapter.py` rather
than `schema.py`. The synthesis's migration v8 must update both
places. The implementer should NOT only update the migration
without also updating the inline CHECK in `process_adapter.py`,
or fresh DBs will install the old constraint.

**Recommendation.** No design change; flagging for the
implementer's attention. Synthesis § 9 step 3 correctly says
"update PROCESS_SCHEMA_SQL enum to match," but it's worth
emphasizing in the BUILD_HANDOFF.md as a verified-not-skipped
step.

### F6 (info) — `recovery_commands` envelope field is shell-string

**Issue.** Synthesis § 2 specifies
`recovery_commands: [shell-string, ...]`. Web UI consumers
(RFC 0013) will want to render these as click-to-copy buttons.
Shell strings work; structured commands
(`{cmd: "publish-artifact", args: {...}}`) would be more
ergonomic for the UI. But they'd require more code in V1.

**Recommendation.** Keep shell-string format for V1
simplicity. RFC 0013's artifact viewer can use a simple
`navigator.clipboard.writeText(...)` on click. Defer the
structured form.

## Acceptance Recommendation

**accept_with_findings.** Design is implementation-ready; F1–F6 are
refinements the implementer should adopt during the build:

- F1: clarify reconciler priority text in the BUILD_HANDOFF.
- F2: keep single event type for V1; document the SSE filter rule.
- F3: cap `adapter_timeout_seconds` at 86400 with a workflow-
  validate error.
- F4: namespace the fixture's output path.
- F5: emphasize the dual-update of `process_adapter.py:PROCESS_SCHEMA_SQL`
  AND `schema.py` in the implementer's checklist.
- F6: keep shell-string `recovery_commands` for V1.

A human can reasonably record an acceptance decision against this
synthesis and proceed to implementation.
