---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0020 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-09
Verdict: `accept`

## Pinned contracts (verified)

- **D036 honored**: `is_repo_write` classifier is the gate.
  Spot-checked: a repo-write stale row never lands in
  `actions`; lands in `still_stuck` with
  `reason: repo_write_requires_operator_inspection`. ✓
- **No-policy regression**: a workflow without
  `recovery_policy` resolves to defaults; `autonomous_*`
  default `false`; sweep emits no `review_requeued` actions. ✓
  (`test_recovery_auto_no_policy_omits_autonomous_actions`)
- **`recovery auto` is a mutation**: not in
  `is_read_command`'s whitelist; gate covers it via the
  service's existing `--allow-mutations` rule. ✓
- **Module layout**: new `src/striatum/recovery/` package
  wraps existing `cli/recovery.py`. Old module unchanged —
  operator-facing verbs stay where they are; the autonomous
  loop is additive. ✓
- **Hook safety**:
  - marker_file refuses `.striatum/`, traversal, and
    out-of-tree paths. ✓
  - webhook returns `{wrote: false, error: ...}` on failure;
    sweep continues. ✓
  - shell pipes stdout/stderr to DEVNULL (D028 preserved). ✓
- **Tests**: 21 new (309 total); covers validator (8), hook
  runners (6), end-to-end sweep (4), workflow validation (1),
  policy resolution (3). All green.
- **Lint + typecheck**: clean (56 source files, +4 from the
  new package).

## Notes

- **`policy_source` envelope field** is the right debugging
  affordance — operators reading sweep logs immediately know
  whether the workflow declared a policy or inherited
  defaults.
- **Step 3 deferred** (the daemon). Cron + step 1 is enough
  for the overnight-stall case; the daemon adds long-lived
  process management we can defer until someone asks.
- **D066 row is intentionally tight**. Modeling what the
  upcoming cleanup pass will look like — one sentence per
  cell, with hyperlinks to the RFC and BUILD_HANDOFF for the
  detail. The walls of text in D063/D064/D065 are the next
  thing to retrofit.

## Decision

`accept`. Closes RFC 0020 V1 (steps 1+2). Engram-style
overnight stalls now have an autonomous response operators can
plug into cron without writing any new code.
