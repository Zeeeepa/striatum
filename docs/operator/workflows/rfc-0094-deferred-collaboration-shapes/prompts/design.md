# Design (one of three parallel lanes)

Task: {{TASK}}

You are one of three design lanes running in parallel. Do not coordinate
with the other lanes — independent perspectives are the point.

Produce a single `DESIGN.md` inside your lane's allowed write path. The
document should cover:

- Problem framing in your own words.
- Proposed approach, sliced smallest-blast-radius first (RFC 0094 §"Implementation
  Plan": post_dialog_hook + synaptic_prune; then Check-B + ledger v1.1; then
  work-packet type sequencing + fog_of_war_review; then second adjudicator).
- Two or three alternatives considered and why this one wins.
- Risks, unknowns, and an explicit "what could go wrong" section — pay
  special attention to the liveness race (emit-before-teardown ordering) and
  to keeping the collaboration_ledger schema change additive.
- A short rollout sketch (what lands first, what lands second).

Keep the document focused. Do not write code. Do not edit files outside
your lane's allowed paths.

When the design is complete, emit the `submit-handoff` packet that the
runner provided in your work packet's `commands` block.
