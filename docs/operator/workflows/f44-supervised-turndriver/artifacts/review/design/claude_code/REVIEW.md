---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["f44","design-review","ergonomics_dx"]
---

author: operator

# F44 design synthesis — ergonomics/DX review

Reviewed `DESIGN_SYNTHESIS.md` against `TASK.md` from a first-time-operator
posture: (1) does the fix remove the operator systemd `path.conf` workaround,
(2) is generator-failure escalation clear to an operator, (3) is the design
simple to follow. Verdict: **accept_with_findings** — the core fix is sound and
the workaround is genuinely removed, but the synthesis as written leaves a real
discoverability hole in the failure path. One round of interrogation surfaced and
closed it; the synthesis text must be amended before implementation.

## What's good (accept)

- **Removes the systemd workaround — clearly.** §1 augments the shared
  `supervisedEnvEntries` PATH builder ($HOME/.local/bin + .npm-global/bin,
  appended, deduped, `STRIATUM_SUPERVISED_PATH_DIRS`-overridable). §5(a) lives the
  proof (remove `striatumd.service.d/path.conf`, restart, confirm gemini
  resolves) and §6.1 records "Removes the need for the
  `striatumd.service.d/path.conf` drop-in." For an operator, the workaround goes
  away and the replacement is generic, testable, and not user-home-hardcoded. No
  finding here.
- **Append-not-prepend is the right DX default** (system tools win; local is
  fallback) and is justified inline. Good.
- **Design is simple to follow.** Three numbered defects → three numbered fixes →
  merged surface (§4) → verification (§5) → DECISION_LOG note (§6) → DoD mapping
  (§7). The conflict-resolution framing (what each lane proposed, what was picked,
  why) is unusually legible. The §3 escape hatch is explicitly scoped. A reader
  can trace every decision to a file and a test.

## Finding (medium) — escalated failure is durable but not discoverable from the operator's own view

**Where:** §2 (exit non-fatally → `return nil`) interacting with §3 (liveness).

**The gap.** §2 correctly declines to overload the exit code (per the product
boundary, exit codes / terminal output are not authoritative state — the
escalation artifact via `OnFailure` → `conversation.ReportFailure` is). But §3 as
written makes `supervise.status` report `liveness: gone` after **both** a healthy
conversation close **and** an escalated generator failure. `gone` alone does not
disambiguate. A first-time operator watching the one view they're already on gets
no in-place hint that a turn failed and was escalated — they only find out if they
already know to open `escalation.list`/the ledger. The failure is durable and
queryable, but not *discoverable*. This is precisely the ergonomics_dx concern:
the affordance to notice the failure is missing from the surface the operator is
on.

**Confirmed by interrogation (2 rounds, see below).** The synthesizer agreed this
is "a real DX hole, not just theoretical" and proposed surfacing
`gone (escalated: <reason>)` vs `gone (completed)`. The first proposal hung the
signal off §3.1's reaper — which is exactly the item the §3 escape hatch is most
likely to defer, re-opening the hole. On the second round the synthesizer
withdrew that framing and converged on the robust home.

**Required amendment (agreed during interrogation).** Make the discoverable
"gone + escalation reason" signal a **read-side join in `HandleSuperviseStatus`
(and the dashboard) against open/recent escalations keyed by `session_id`/`run_id`**,
owned by §3.3 — explicitly **reaper-independent**. This is strictly better, not
just escape-hatch-proof:

1. It lands in §3.3 + §3.2 (zombie-aware probe yielding `gone`), both of which
   ship even under the escape hatch — so the signal no longer depends on §3.1.
2. The escalation is written synchronously *before* `Loop.Run` returns nil, so the
   cause is visible even before exit and even if the child never reaps.
3. The reason derives from the authoritative escalation ledger, not a duplicated
   supervisor column that can drift.

The reaper (§3.1) keeps only its original narrow job (reap the zombie); any
exit-disposition it records is enrichment, not the failure carrier. §4 should add
a test: a session with an open escalation projects `gone` + the escalation
reason/id in `supervise.status` **with the reaper disabled**. §6.3 should record
that the discoverability signal is read-side and reaper-independent.

This is a medium finding: it does not block the §1/§2 core fix (the F44 required
scope) and the resolution is small and adjacent to code §3.3 already touches — but
left unamended, the "graceful failure" the task asks for is silent to the exact
operator the ergonomics_dx posture exists to protect.

## Minor / non-blocking

- §5's live-verification step (b) says the operator "sees the escalation"; with
  the amendment above, tie that observation to `supervise.status` showing
  `gone (escalated: …)`, not just "an escalation is emitted," so the live check
  exercises the discoverable path.

## Interrogation rounds used

**2 of 3.** Round 1 established the discoverability gap (synthesizer conceded it).
Round 2 tested the proposed fix against the §3 escape hatch and exposed a coupling
defect (signal hung off the deferrable reaper); the synthesizer fully conceded and
relocated the signal to a reaper-independent read-side projection in §3.3.
**Stopped at 2** because the exchange converged on a concrete, agreed amendment —
a third round would not change the verdict, and the remaining DX checks
(workaround removal, design simplicity) are well-covered by the synthesis text
itself. The accepted parts stand; the one finding is captured above with its
agreed fix.
