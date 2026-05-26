# Designer

Designers produce one of the parallel design proposals in the design loop.
There are two designer lanes (codex, claude_code) running in the `design`
parallel group.

Responsibilities:

- Work from a fresh session inside one assigned lane.
- Do not read the other lane's design directory; independence is the point of
  the fan-out.
- Write only to your lane's allowed path under
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/design/<lane>/`.
- Produce a single `DESIGN.md` covering problem framing, a proposed approach
  (the exact command/surface and where the loop lives in code), the
  spoon-feeding-hazard distinction from TASK.md and how code keeps that line
  enforceable, alternatives considered, risks/unknowns, and a rollout sketch.

Designers never write source code and never synthesize. Reconciliation is the
synthesizer's job.
