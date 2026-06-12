---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: committer-unknown-model-001
workflow: rfc-0119-ratify
run_id: run_b089555bd70cd2dc2dc1d13c3cc35b53
cycle: 1
title: "RFC 0119 — Acceptance (accept_with_findings): status flip + binding constraints C1–C4 + proposed decision entry (D179, not D178)"
inputs:
  - docs/rfcs/0119-warm-tier-memory-boundary.md
  - docs/rfcs/0119/adjudicator/COLLABORATION_LEDGER_cycle_1.md
  - docs/rfcs/0119/HOLDER_CASE.md
  - docs/rfcs/0119/falsifier_1/FALSIFIER.md
  - docs/rfcs/0119/falsifier_2/FALSIFIER.md
---

# RFC 0119 — Acceptance

This is the committer synthesis for the RFC 0119 ratification gate
(run `run_b089555bd70cd2dc2dc1d13c3cc35b53`). It is published **only because the
adjudicator ledger recorded a clearing verdict**; it is not a refusal note.

## 1. Gate disposition — clearing verdict, with findings

The adjudicator ledger
(`docs/rfcs/0119/adjudicator/COLLABORATION_LEDGER_cycle_1.md`) recorded:

- `verdict: accept_with_findings`
- branches `state_transition_dependence`, `durable_provenance`, and
  `export_class` all `cleared_with_constraints`
- ledger body: "RFC 0119 is accepted with findings only."

That is a **clearing verdict**, so this acceptance is published. It is
`accept_with_findings`, **not** a clean accept: the gate clears *with four
binding constraints* (C1–C4 below). The acceptance is therefore
**binding-conditional** — the ledger requires that "RFC 0119 must be amended to
discharge C1–C4 before a decision record is filed or Go implementation begins."

## 2. RFC 0119 status flip

RFC 0119 flips `proposed → accepted` (with binding constraints C1–C4).

The literal edits — the `Status:` line at the top of
`docs/rfcs/0119-warm-tier-memory-boundary.md` and the matching row in
`docs/rfcs/README.md` — are **outside this job's write scope**
(`allowed_paths` = `docs/rfcs/0119/ACCEPTANCE.md`, `docs/decisions/D178.md`).
They are recorded here as the operator's mechanical follow-up and should be
applied **together with** the C1–C4 amendment and the decision-log merge in
§4, not before. Per the implementation envelope: if discharging C1–C4 needs
edits beyond the RFC text and these two files, that must be called out rather
than assumed to fit the frozen scope.

## 3. Binding constraints and where the RFC text must discharge them

**Honest status:** the RFC text at `docs/rfcs/0119-warm-tier-memory-boundary.md`
is **unamended** as of this acceptance and therefore **does not yet discharge
C1–C4**. Each is a binding, pre-implementation obligation with
`final_review_required: true`. The table maps each binding constraint to the
exact RFC location that must be amended to discharge it.

| Constraint | Posture / severity | Requirement (from ledger) | Discharged in RFC today? | Where the RFC must discharge it |
| --- | --- | --- | --- | --- |
| **C1** `C1-HOT-TIER-SEPARATE-CONNECTION` | state_transition_dependence / critical | Hot-tier recall must run on a separate connection, never the claim transaction; the digest write must be fail-soft outside the transition; and a recall-failure guardrail must assert claim **and** worktree-create still commit. | **No.** D3 still frames the seam as "this is **not** a state transition," which the falsifiers showed is false at `buildPacket` (`claim.go:229`, inside the claim `tx`). | Amend D3 ("Hot tier", RFC `:88`–`:99`): drop the bare "not a state transition" framing; require the recall read on a **separate connection** (never the claim/worktree-create `tx`); make the digest write fail-soft outside the transition; add the guardrail that **injects a recall-read failure and asserts claim + worktree-create still commit** (the C1 verification gate). |
| **C2** `C2-WORKTREE-ORPHAN-GUARD` | state_transition_dependence / high | The worktree-create digest render must be ordered and fail-soft so a render failure cannot leave a dangling git worktree without a `job_worktrees` row. | **No.** D3 names `HandleWorktreeCreate` as a render seam but does not order it relative to the physical `git worktree add` / row insert. | Amend D3's worktree-create seam: order the render **after** the `job_worktrees` row is recorded (or otherwise make render failure non-fatal and non-orphaning). Verification gate: inject a digest-render failure during worktree create and assert no unrecorded worktree remains. |
| **C3** `C3-EXHAUST-ALLOWLIST` | durable_provenance / critical | Exhaust must be an explicit per-kind allow-list aligned with RFC 0072, excluding `operator_report`, `decision`, `escalation`, `operator_brief`, `work_plan`, `collaboration_ledger`, and corpus-export durable-provenance kinds. | **No.** The only worked definition (Open Questions, RFC `:137`–`:141`) is a glob — "progress_note, operator_report, *_ledger, unsynthesized design candidates" — which evicts durable-provenance kinds and uses a `*_ledger` glob. | Replace the Open-Questions default with an **explicit per-kind allow-list** that is a strict subset of RFC 0072's blob set, **never a glob**; RFC 0119 must explicitly **reconcile with / supersede RFC 0072's boundary table** rather than silently contradict it. Verification: RFC text and decision record list allowed exhaust kinds explicitly and forbid broad ledger globs. |
| **C4** `C4-LANE-TRAJECTORY-CONTRACT` | export_class / high | `lane_trajectory` must explicitly supersede the transcript-denial guardrail/decision where required and either define deterministic redaction-normalization to bytes or stay outside the durable-provenance bundle. | **No.** D2 asserts `lane_trajectory` is "pull-only and deterministic" without naming the supersession or the redaction contract; it is sourced from exactly the transcript shapes `validateCorpusSourcePath` denies. | Amend D2 (RFC `:77`–`:86`): name the supersession of the enforced `validateCorpusSourcePath` transcript denial (`redaction.go:24-29`, `:84-87`; `exports.go:408`) **and D028**; specify the deterministic redaction-normalization-to-bytes contract, **or** keep `lane_trajectory` out of the corpus durable-provenance bundle. |

## 4. Proposed decision-log entry — assigned **D179** (ID collision: D178 is taken)

RFC 0119 and the commit prompt call the new decision **"D178"**, but **`D178`
is already an accepted decision** — `docs/decisions/decision-log.md:36` assigns
`D178` to the RFC 0117 `striatum worktree gc` surface, and `D178` is the latest
assigned decision ID. Reusing `D178` would collide with an accepted decision,
exactly the failure mode the log's own D119–D123 correction note exists to
prevent.

The proposed entry is therefore **assigned `D179`** (the next free ID) and
written to `docs/decisions/D178.md` — the path this job's write scope mandates —
for the operator to merge. **When pasting into
`docs/decisions/decision-log.md`, use `D179`, not `D178`.** This collision is
flagged per the implementation-envelope instruction (do not assume the frozen
scope/ID can absorb a collision).

The merge is gated: the `D179` row should land **only together with** the RFC
amendment that discharges C1–C4 (§3) and the status flip (§2). The ready-to-
paste row and the C1–C4 discharge checklist are in `docs/decisions/D178.md`.

## 5. No Go code lands in this run

This run files the acceptance synthesis and the proposed decision entry only.
**No `go/` change lands here.** The `RecallMemory` projection + scaffold-time
digest injection (RFC D3) and the `lane_trajectory` export class are the
**operator's follow-up**, gated on:

1. C1–C4 being discharged in the amended RFC and the `D179` row first; and
2. the corpus guardrails staying **green** — no `memory.*` in the daemon method
   registry, no external-consumer import in daemon source, and all state
   transitions succeeding with the warm tier absent.

Until both gates hold, no implementation work begins.
