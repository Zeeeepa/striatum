# Iterated interrogating panel (design + build)

A reusable Striatum workflow with **two structurally identical loops** —
a **design loop** and a **build loop** — chained design → build. Each loop is:

```
fan-out (3 independent lanes)  →  synthesis  →  interrogating panel review
        ^                                              |
        |___________ revision cycle (needs_revision) __|
```

- **Fan-out (3 lanes).** Three independent agents (codex, claude_code,
  gemini) produce independent artifacts for the same objective with no
  cross-talk (`parallel_group`, disjoint write scopes). The design loop
  fans out three design proposals. The build loop's fan-out is on the
  **review** side — the implementation is single-author (one diff), but a
  3-wide panel reviews it, to avoid three conflicting diffs.
- **Synthesis / implementation.** One synthesizer reconciles the three
  design proposals into one buildable synthesis. In the build loop the
  matching node is the implementer producing the actual change plus a
  `HANDOFF.md`.
- **Interrogating panel review.** A `parallel_group` of three reviewers
  with distinct postures (`threat_model`, `ergonomics_dx`,
  `devils_advocate`). The reviewed node is `interrogable: true` and stays
  live (`awaiting_interrogation`) after it completes, so each reviewer can
  interrogate the author's **preserved context** before rendering a
  verdict.

## The two bounded-iteration concepts (do not conflate)

1. **Interrogation rounds — ≤ 3, early exit on resolved findings.**
   Within a single review, each panel reviewer runs an interrogation thread
   against the live reviewed session: open → up to **3** `ask`/`answer`
   rounds → close. The reviewer stops early the moment its open findings are
   resolved. The cap and early-exit are enforced by the **reviewer role
   prompt** (`roles/reviewer.md`), not the engine; the reviewer must state
   in its finding how many rounds it used and why it stopped.

2. **Revision cycle — bounded re-work.**
   If a panel reviewer returns `needs_revision`, the loop returns to the
   synthesis/implement node. Encoded as `cycles` with
   `on_verdict: needs_revision`, `max_iterations: 2`. Early exit is
   automatic: if no reviewer returns `needs_revision`, the cycle does not
   fire.

## Execution substrate — agent-loop-first

This pattern is **agent-loop-first**. Every lane declares
`adapter_capabilities.agent_loop: true` plus
`supervision.transport: pty_helper` (`require_tmux: true`), so the daemon
wraps each lane in the RFC 0088 agent-loop executor and the agent claims
work as an MCP client over the PTY. Interrogation requires **preserved
context** so the reviewed agent can answer from its own working memory; a
one-shot command (`--print`, `-p`, `codex exec`) spawns a fresh process per
packet (no memory), never claims work, and cannot be interrogated
truthfully — so none of those flags appear on any lane here.

The `lanes` block declares `adapter: process` with bare interactive agent
CLIs (`codex`, `claude`, `agy`); the agent-loop shape is expressed directly
through the `adapter_capabilities` and `supervision` lane fields. Do not add
one-shot flags to any lane. The interrogation target is always an agent-loop
lane (claude is the known-good baseline).

## Lanes, roles, and panel shape

| Loop   | Fan-out                          | Reviewed node             | Panel (`parallel_group`)            |
| ------ | -------------------------------- | ------------------------- | ----------------------------------- |
| Design | `design_codex/claude/gemini`     | `synth` (interrogable)    | `design_review` (3 postures)        |
| Build  | (review-side only)               | `implement` (interrogable)| `build_review` (3 postures)         |

Each panel reviewer is on a distinct lane and posture:
`threat_model` (codex), `ergonomics_dx` (claude_code), `devils_advocate`
(gemini). Reviewers use `reviewer_context_policy: fresh` and
`reviewer_access_scope: document_only` (the correct value — `artifact_only`
is invalid in this schema) and each holds the `interrogate` capability at
register-session time (`--capability interrogate`).

## How to run

The daemon is a hard prerequisite for every Striatum verb. Replace
`{{TASK}}` in the prompts with the concrete objective before starting.

```sh
# Validate the workflow file. The 3-lane / 3-posture panel necessarily
# includes the reviewed author's own lane in each panel, so the same-model
# pairing warning is expected; pass --allow-same-model-pairing.
striatum workflow validate --allow-same-model-pairing \
  examples/iterated-interrogating-panel/workflow.json

# Prepare a run from this workflow.
striatum run prepare --workflow examples/iterated-interrogating-panel/workflow.json

# Then start it and watch progress.
striatum run start --run-id <id>
striatum dashboard --run-id <id>
```

When launching the lanes via the agent-loop substrate, register the
reviewed (`synth`, `implement`) sessions as interrogable and grant the
panel reviewers the `interrogate` capability so they can open interrogation
threads against the live reviewed sessions.

## Artifact layout

```
examples/iterated-interrogating-panel/artifacts/
  design/{codex,claude_code,gemini}/DESIGN.md   # design fan-out (handoff)
  DESIGN_SYNTHESIS.md                           # synth (synthesis, interrogable)
  review/design/{codex,claude_code,gemini}/REVIEW.md
  build/HANDOFF.md                              # implement (handoff, interrogable)
  review/build/{codex,claude_code,gemini}/REVIEW.md
```

All write scopes are disjoint per job
(`parallelism.require_disjoint_write_scopes: true`).
