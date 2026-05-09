---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0020 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-09
Verdict: `accept`

## Pinned contracts (verified)

- **Sweep order**: deterministic 5-step sequence with the
  classifier (`is_repo_write` + `recovery_policy` field)
  pinning what's safe to autonomously requeue. ✓
- **D036 honored**: `repo_write` jobs are *never* autonomously
  requeued; the existing `requeue_stale` refusal is the
  authoritative gate. ✓
- **Module layout**: new `src/striatum/recovery/` package
  wraps existing `cli/recovery.py` helpers. Old module stays
  unchanged — operator-driven verbs remain operator-driven; the
  autonomous loop is additive. ✓
- **Workflow policy defaults to off**: `autonomous_review_requeue`
  and `autonomous_process_reconcile` default to `false` so a
  workflow without an explicit policy gets diagnostic-only
  output; CLI flags override for one-shot operator use. This
  preserves the no-policy regression. ✓
- **Hooks honor adapter constraints**: shell hook routes
  through the process adapter so lane `constraints` apply;
  stdout/stderr go to DEVNULL (D028). ✓
- **marker_file refuses `.striatum/`** and out-of-repo paths.
  Same boundary `evidence export` already enforces. ✓
- **Doctor surface**: `blocker_recovery_eligible` adds a
  structured record with `recovery_command` operators copy-paste.
  Mirrors RFC 0014's pattern. ✓
- **Test plan**: 17 new tests across two files cover dry-run
  idempotency, requeue cap, hook kinds, doctor surfacing, and
  the no-policy regression. The byte-diff test against a
  v1.4.1 baseline catches accidental envelope drift in
  packets. ✓

## Notes

- **`striatum recovery auto` as a mutation verb**. The
  synthesis correctly flags that the gate's `is_read_command`
  whitelist needs updating to recognise `recovery auto` as a
  mutation. Without that, the service would treat it as a
  read and skip the gate, defeating the point.
- **`policy_source` field in the envelope** is a small but
  good debugging hint — operators reading sweep logs can
  immediately tell whether the workflow declared a policy or
  inherited defaults.
- **`eligible_after_seconds` (default 600)** is per-policy,
  not a runner constant. Workflows that want quick triage
  (overnight runs in dev) can set it lower; production runs
  may want hours. Sensible default.
- **Step 3 deferral**: the synthesis sticks to the RFC's
  acceptance criterion ("V1 = steps 1+2"). The daemon adds
  long-lived process management; cron + step 1 covers the
  Engram-stalled-overnight case without it.

## Decision

`accept`. The design is the minimum correct addition: a thin
orchestrator over existing primitives, no new aggregates, no
schema change, defaults preserve today's flow. Closes the
overnight-stall failure mode the RFC was written to address.
