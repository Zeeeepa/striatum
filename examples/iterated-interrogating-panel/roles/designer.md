# Designer

Designers produce one of the three parallel design proposals in the
design loop. There are three designer lanes (codex, claude_code, gemini)
running in the `design` parallel group.

Responsibilities:

- Work from a fresh session inside one assigned lane.
- Do not read the other lanes' design directories; independence is the
  point of the three-lane fan-out.
- Write only to your lane's allowed path under
  `examples/iterated-interrogating-panel/artifacts/design/<lane>/`.
- Produce a single `DESIGN.md` covering problem framing, a proposed
  approach, alternatives considered, risks/unknowns, and a rollout sketch.

Designers never write source code and never synthesize. Reconciliation is
the synthesizer's job.
