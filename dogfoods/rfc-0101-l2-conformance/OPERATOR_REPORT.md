# Operator Report — RFC 0101 Layer 2 conformance dogfood

Date: 2026-05-31
Run: `run_bcaa1a7ed308f79b2dcaf62fa09d75f6` (branch `striatum/rfc-0101-l2-conformance-v2`)
Outcome: **design phase complete; wedged at design→build boundary on a real
product bug (#120). Conformance-harness code NOT built (implement never ran).**

## What this dogfood was

Scaffolded from an `/adhd` decomposition that selected **RFC 0101 Layer 2 —
adapter-conformance harness + lane-env hardening (#76/#85/#70)** as the first
buildable slice (no daemon restart needed to design; it is the meta-unblock that
makes L1/L3 provable). Shape: 2 independent designers (codex + claude_code) →
interrogable synthesis → 3-seat adversarial design panel (threat_model /
ergonomics_dx / devils_advocate) → interrogable implementation → 3-seat
interrogating build panel, with bounded `needs_revision` cycles.

## What worked (proven live)

- **tmux-backed lanes with logged trajectories.** After correcting the lane shape
  (bare-interactive command + `adapter_capabilities.agent_loop` +
  `supervision.{transport: pty_helper, require_tmux}`), every lane came up
  `lane_backend: tmux`, `pane: tmux_ok`, and the trajectory was readable via
  `tmux capture-pane`. This is the visibility the operator asked for and the target
  shape of #112.
- **Real multi-model design + adversarial review.** Both designers produced
  substantive proposals; the synthesizer reconciled them; the design panel ran
  genuine adversarial review and drove **two `needs_revision` cycles** (synth
  reached attempt 3), which the runner self-advanced — the revision loop that
  wedged every prior dogfood worked here.
- **6 artifacts produced:** `design/{codex,claude_code}/DESIGN.md`,
  `DESIGN_SYNTHESIS.md`, `review/design/{claude_a,claude_b,codex}/REVIEW.md`.

## Where it wedged (the finding)

`review_design_codex` self-`work.block`ed twice: the review prompt requires
*interrogating the live synthesizer before a verdict*, but the `document_only`
reviewer session **lacks the `interrogate` capability** —
`interrogation.open` returns `capability_denied`. The codex reviewer honestly
refused to render a verdict it could not support (it published its finding with
`verdict_intent: needs_revision` + two real findings, then blocked). The two
claude reviewers did *not* honor the requirement and voted anyway — so the
requirement is simultaneously **unenforced** (claude) and **unsatisfiable**
(codex). `implement` requires the `threat_model` posture (codex), so the whole
build phase is blocked. No operator verb resolves a `work.block` blocker
(`recovery resume` is process-adapter-only; `review.submit`/`verdict` need a live
lease the blocked session no longer holds; closing the session does not free the
job). Filed as **#120**.

This is the RFC 0097/0101 loop exactly: the runner-fix cannot be dogfooded through
the wedged runner. The proper fix is daemon/workflow code (grant the `interrogate`
capability to review jobs targeting an `interrogable` upstream, or validate-reject
the mismatch at authoring, or escalate loudly instead of wedging) — then re-run.

## Findings filed (GitHub)

From this session's friction (all `rfc-0101`):
- **#112** — Supervised lanes should use the tmux backend by default + extract/log
  trajectories *(the operator's headline ask)* (`enhancement` `rfc-0101` `rfc-0091`
  `rfc-0075`)
- **#113** — Bundled example workflows ship retired one-shot lane commands
  (`--print`/`codex exec`) so lanes never claim (`bug`)
- **#114** — `workflow validate` silently accepts duplicate top-level keys (`bug`)
- **#115** — Editing `workflow.json` has no effect on a prepared run (frozen
  snapshot) with no operator signal (`enhancement` `documentation`)
- **#116** — `supervise start --replace` raw Postgres unique-constraint error on a
  stale supervisor row (`bug`)
- **#117** — `agent_mcp_discovery_stall` misleads when the lane exited/blocked at
  spawn (`enhancement` `rfc-0091`)
- **#120** — Interrogating-panel reviewer lacks `interrogate` capability → reviewer
  `work.block`s → whole panel wedges (`bug` `rfc-0096`) — the wedge that stopped
  this run.

## Reusable output

`dogfoods/rfc-0101-l2-conformance/` is the corrected, validating
interrogating-panel template: tmux-backed `agent_loop` lanes,
`allow_same_model_review_pairing`, `inputs` on document_only reviews, bounded
revision cycles. It is the working shape #112 targets and a starting point for the
re-run once #120 is fixed.

## Next step

Fix #120 (grant the `interrogate` capability path for interrogating-panel
reviewers; ideally also #114 duplicate-key reject and #115 snapshot signal), then
prepare a fresh run from this scaffold to actually build + review the conformance
harness.
