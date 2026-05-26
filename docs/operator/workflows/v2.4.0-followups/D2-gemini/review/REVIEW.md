---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---
author: operator

# D2 Gemini guidance review (ergonomics_dx, document_only)

Verdict: accept_with_findings. The "Agent-loop reliability (Gemini)" section
is present in both templates, the wording matches the surrounding guide, and
the four rules correctly target the failure mode in TASK.md: lease-expiry
(rule 1 = 900s lease, rule 4 = re-`await_packet` on lease error) and wasted
exploration (rule 2 = no grep before/while holding a packet). The two files
are mutually consistent and the HANDOFF accurately summarizes both edits.

Findings (non-blocking, ergonomics):

1. Loop-order inconsistency *within* the skill guide. The existing "Claim
   loop" pattern (lines 56–60) orders `work.ack` **before**
   `artifact.publish`, but new rule 4 orders `artifact.publish` **before**
   `work.ack`. A first-time Gemini reader gets two contradictory orderings in
   one file. Pick one (publish→ack→complete is fine) and align both spots.

2. Rule 4 omits the review-job path. The guide (lines 63–67) states review
   jobs skip `work.complete` and use `review.submit`/`verdict`; rule 4 only
   shows the `work.complete` loop, so a Gemini reviewer following it verbatim
   would hit the daemon's bare-completion rejection. One clause noting the
   review variant would close the gap.

Both are discoverability nits, not defects in the stated fix. The guidance is
correct and clear enough to ship.
