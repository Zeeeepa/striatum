# RFC 0102: Operator Attention Economy — one control surface, high-signal feedback, fewer things to stare at

Status: proposed
Date: 2026-05-31
author: proposer-claude-opus-4-8-001

Context:
- Striatum's robustness work (RFC 0101) treats the *run* as the unreliable
  distributed system and hardens the daemon against Byzantine lanes. This RFC
  addresses the **complementary** failure surface: the **operator** (human or
  AI) driving the run is *also* a bounded, unreliable processor, and the system
  currently overloads it. RFC 0101 makes the machine survive a bad lane; RFC
  0102 makes the operator survive a long run without degrading into error.
- The thesis is the human condition applied to operation: **attention is the
  scarce resource, and cognition degrades with the number of distinct surfaces
  one must watch and interact with.** Fewer control surfaces and higher-signal
  feedback are not ergonomic niceties — they are correctness mechanisms. An
  operator that must reconcile state across many windows, formats, and identifier
  spaces will, under load, substitute memory for verification and make
  confabulation-class errors.
- This is grounded in direct, logged evidence from the RFC 0101 Layer 2
  dogfood (2026-05-31, runs `run_bcaa1a7e` / `run_9925b250`), captured in the
  session post-mortem. The operator (an AI driving the run) produced a
  recurring error class **not** caused by lack of context capacity but by
  attention misallocation under low signal:
  - **Identifier confabulation.** Invented a plausible-but-wrong `run_id`
    (`run_5e8f8e75…` for the real `run_9925b250…`) and propagated it into a
    driver script and three scheduled prompts; guessed GitHub issue numbers
    (#113–118) that were actually #112–117; guessed RFC cross-reference
    filenames that did not exist — in each case *typing an identifier from
    memory instead of reading it back*.
  - **Result confabulation.** Narrated a `run_id`/`session_id`/status from CLI
    calls that had actually errored — reasoning forward from corrupted ground
    truth.
  - **Repeated identical mistakes.** `pkill -f '<pattern>'` self-matched the
    wrapper shell running it → exit 144 → cancelled the whole tool batch, three
    times, despite the lesson being known.
  - **Premature, retracted conclusions.** "lanes advancing", "lane_backend=tmux
    confirmed" (still `plain_pty`), an entire PATH/tmux root cause recorded as
    fact then retracted. **A destructive `git checkout -- .` cleanup run without
    first reading what it would discard, deleting uncommitted operator work.**
  The common root: when **verification is expensive and low-signal**, the
  operator's cost/benefit tips toward guessing. Every wrong guess then costs
  detect-and-unwind cycles. A ~1-hour task became a multi-hour grind.
- Two amplifiers turned individual slips into compounding cost:
  1. **Too many control surfaces / identifier spaces.** Driving one run touched
     the `striatum` CLI, hand-rolled Python driver scripts, raw `tmux`
     session/pane commands, `systemctl`, `gh`, `git`, raw `psql`-shaped state in
     `status` JSON, and ad-hoc `/tmp` scratch files — each with its own
     identifier vocabulary (`run_*`, `sess_*`, `sup_*`, `dsup_*`, `lease_*`,
     `msg_*`, `blk_*`, tmux `%pane`/`@window`, OS pids). Cross-referencing them
     by hand is exactly the multi-window reconciliation that degrades cognition.
  2. **Low signal from the tool.** Lanes ran `plain_pty` (DEVNULL) with no
     trajectory by default (RFC 0101 #112), so the operator was blind to *why* a
     lane misbehaved and had to infer from coarse, sometimes-dishonest state
     (`agent_mcp_discovery_stall` over a dead pane, #117; `running` over a parked
     welcome-screen agent, #121). `striatum why <job>` rejected a workflow
     job_id; `--json` returned empty on `why`; output arrived in delayed bursts
     and at times corrupted. Low signal directly *causes* the guessing in the
     error class above.
- Related work this composes with:
  [RFC 0050] (operator UI / native daemon surface), [RFC 0075]
  (tmux-observable sessions), [RFC 0097] (full run orchestration — removes the
  hand-rolled driver entirely), [RFC 0099] (constrained operator mode — a
  control-surface-only AI operator; this RFC defines *what that surface should
  be*), [RFC 0101] (machine-side robustness — the run; this is the
  operator-side complement). [`docs/decisions/decision-log.md`]: **D094/D005**
  (daemon authoritative, lanes scratch), **D028** (curated artifacts, not raw
  transcripts).

## Problem

Driving a Striatum run demands that the operator hold and reconcile state across
**many heterogeneous surfaces**, each speaking a different identifier dialect,
while the tool emits **low, sometimes dishonest, signal** about what is actually
happening. Under the sustained load of a long multi-phase run this overloads the
operator's attention and produces a characteristic, repeatable error class:
typing identifiers from memory, narrating unverified results, repeating known
mistakes, running destructive commands without reading their blast radius, and
asserting conclusions ahead of evidence. These are not capacity (context-window)
failures; they are **attention-allocation** failures driven by the *structure*
of the control surface. The cost is paid in detect-and-unwind cycles and in
operator (or AI) trust in its own ground truth.

The system optimizes the machine's robustness (RFC 0101) but treats the
operator as an infinite, reliable controller. It is neither.

## Goals

- **G1 — One control surface.** An operator can drive a run end to end through a
  **single, authoritative control client (the CLI / its daemon MCP)** without
  dropping to `tmux`, `systemctl`, raw `status` JSON spelunking, `psql`, or
  hand-rolled scripts for any *normal* operation (start, observe, recover,
  finalize). Other tools become diagnostics of last resort, not the main loop.
- **G2 — High signal per glance.** The single most-used view answers "what needs
  my attention right now, and why" in one frame: per-job state, the *one* blocked
  thing, its honest reason, and the *one* next action — without the operator
  assembling it from multiple commands.
- **G3 — Verification cheaper than guessing.** Reading back an identifier or a
  result must be a single, fast, reliable call, so the operator never has a cost
  incentive to retype from memory. Identifiers the operator must act on are
  copy-exact and machine-checkable.
- **G4 — Fewer identifier spaces in the operator's face.** The operator works in
  terms of **human-meaningful handles** (run + workflow-job-id) for normal
  operation; the `sess_/sup_/dsup_/lease_/msg_/blk_/pane` zoo stays *internal* to
  the daemon and surfaces only when an action specifically needs one — and then
  the command that needs it supplies it.
- **G5 — Honest, legible feedback.** Every state the operator sees is derivable
  from a real signal (RFC 0101 G2) and is *labeled in operator terms*, not
  internal stall-class jargon; lane trajectories are observable on demand through
  the one surface (RFC 0101 #112 / RFC 0075).
- **G6 — Within existing invariants.** Daemon stays authoritative (D094); lanes
  stay scratch (D005); curated artifacts, not raw transcripts (D028); no new
  cloud/hosted dependency. This is a *consolidation and signal* RFC, not a new
  persistence or service tier.

## Non-goals

- Not a GUI/visual-design rework (that is RFC 0050's lane); this is about the
  *number and signal* of surfaces, applicable equally to a terminal CLI and an
  AI operator over MCP.
- Not the autonomous orchestrator itself (RFC 0097) — though full orchestration
  is the ultimate attention reducer (zero surfaces during a healthy run). This
  RFC defines the surface for when an operator *is* in the loop.
- Not removing necessary human-decision gates (RFC 0099 distinguishes those).
- Not making the operator infallible — reducing the *structural* pressure toward
  error, not legislating attention.

## The attention model

Treat the operator as a bounded processor with a small working set. Two levers
move its effective cognition, and they are the two the operator named:

1. **Surface count (breadth).** Each distinct tool/window/identifier-space the
   operator must hold open is a context-switch tax and a reconciliation risk.
   Cognition improves roughly with *fewer things to stare at and interact with*.
   → **Narrow the control surface to one.**
2. **Signal-to-noise (depth).** For the surfaces that remain, each glance should
   deliver decision-relevant truth, not raw state to be assembled. Low signal
   pushes the operator toward memory/guessing.
   → **Raise the signal from the tool.**

The compounding is multiplicative: low signal makes verification expensive;
many surfaces make verification *also* require cross-referencing; together they
make guessing rational, and guessing is where the error class lives.

## Design — five tracks

### Track 1 — Narrow the control surface to the CLI (G1, G4)

The CLI (and its daemon MCP equivalent) is the **one** authoritative client.
Everything an operator does in the normal loop is a first-class verb; the
fallbacks the dogfood reached for become unnecessary:

- **Lane lifecycle without tmux/systemctl/scripts.** Today the operator
  hand-spawned lanes via `register-session` + `supervise start`, watched them via
  `tmux capture-pane`, restarted the daemon via `systemctl`, and re-implemented a
  driver loop in Python (v1→v4) to register/supervise jobs as they unblocked and
  evict stale sessions. **All of that should be daemon-owned** (this is the
  operator-facing half of RFC 0097): a `striatum run drive` /
  `run watch`-with-auto-supervise that the daemon runs, so the operator does not
  hand-roll orchestration. The hand-rolled driver was the single largest source
  of operator-authored bugs this session.
- **Recovery as named verbs, not raw state edits.** Every stuck state the
  operator hit needs a verb (and `next_actions` must name it): evicting a stale
  same-`(role,lane)` session that squats a slot on a revision cycle (the
  manual `session close` dance), resolving a `work.block` (today *no* verb
  exists — `recovery resume` is process-adapter only), and returning a
  dead-lane repo-write job to claimable on the same attempt (#121 — no verb
  exists today).
- **No raw identifier wrangling.** `supervise start --replace` returned a raw
  Postgres `23505` (RFC 0101 #116); operators should never see SQLSTATE. Verbs
  return typed, actionable errors with the remediation verb named.

Acceptance: a representative multi-phase run (design → review panel → build →
build panel) is driven start-to-finish using only `striatum` verbs — no `tmux`,
`systemctl`, `psql`, or operator scripts in the normal path.

### Track 2 — One high-signal "attention" view (G2, G5)

A single command answers "what needs me now": `striatum attention --run-id <id>`
(or it *is* the default `dashboard`/`status` frame). It shows, in one frame:
- run state + phase; per-job state counts;
- the **blocking frontier**: the specific job(s) gating progress, each with an
  **honest, operator-language reason** (not `agent_mcp_discovery_stall` — say
  "lane exited before first MCP call" / "awaiting reviewer verdict (0 of 1)" /
  "reviewer blocked: needs interrogate capability" / "implement lane parked, no
  heartbeat 12m");
- exactly **one recommended next action** per blocked item, expressed as a
  copy-runnable verb;
- nothing else. Healthy jobs collapse to a count. The operator stares at the
  frontier, not the field.

This is the antidote to assembling truth from `dashboard` + `list jobs` +
`status.open_blockers` + `supervise status` by hand.

### Track 3 — Make verification cheaper than guessing (G3)

- **Read-back is one call.** `striatum run last` / `run current` returns the
  active run id; ids the operator must reuse are emitted in a stable,
  copy-exact, greppable form (`KEY=value`), never reconstructed from prose.
- **Identifier guards.** Verbs that take an id validate it and fail loudly on a
  malformed/nonexistent one ("no such run_id; did you mean …") rather than
  silently operating on nothing — so a typo/confabulation is caught at the call,
  not three steps later. (Pairs with RFC 0101 #114 duplicate-key reject and
  #115 snapshot-divergence warning: silent acceptance of wrong input is the
  enemy.)
- **Honest output contract.** A failed call returns a clearly-failed result the
  operator/AI cannot mistake for success — addressing the result-confabulation
  mode at the source.
- **Guard destructive operations.** Destructive verbs (and the operator's own
  habits) should preview their blast radius before acting; the session that
  motivated this RFC lost uncommitted work to an unread `git checkout -- .`.

### Track 4 — Reduce the identifier zoo in the operator's face (G4)

Normal operation is expressed in **(run, workflow_job_id)** — the two handles a
human actually reasons about. The internal id families
(`sess_/sup_/dsup_/lease_/msg_/blk_/pane/window`, OS pids) stay daemon-internal;
when a verb genuinely needs one, the verb *resolves it itself* from
`(run, workflow_job_id)` rather than making the operator look it up and paste it.
`striatum why <workflow_job_id> --run-id <id>` must work (it did not this
session — it demanded an internal job_id and `--json` returned empty).

### Track 5 — Trajectory and liveness on the one surface (G5)

Lane trajectories are observable through the CLI on demand (depends on RFC 0101
#112 tmux-by-default + trajectory logging; RFC 0075): `striatum trajectory
--run-id <id> --job <wfjob> [--tail]` — so the operator never drops to raw
`tmux capture-pane` against an internal session name to see what a lane is doing.
Liveness labels are honest and in operator language (RFC 0101 G2).

## Relationship to existing RFCs

- **RFC 0097 (orchestration)** is the strongest form of Track 1: a healthy run
  needs *zero* operator surfaces. This RFC covers the still-necessary
  in-the-loop case and defines the surface RFC 0097 falls back to on escalation.
- **RFC 0099 (constrained operator)** consumes this: a control-surface-only AI
  operator is only safe and effective if that surface is *one* thing and
  *high-signal*. This RFC specifies what RFC 0099's operator is allowed to look
  at and act through.
- **RFC 0101 (robustness)** is the machine-side twin: 0101 makes the run not
  wedge silently; 0102 makes the operator not err under load. They share the
  "honest, derivable signal" principle (0101 G2 ⇒ 0102 G5) and several issues
  (#112 trajectory, #115 snapshot signal, #116 typed errors, #117/#121 honest
  liveness) serve both.
- **RFC 0050 (UI) / RFC 0075 (tmux-observable)** provide rendering and lane
  observability primitives this consolidates into the single attention surface.

## Open questions

- **OQ1.** Is "one surface" the CLI, or the daemon MCP with the CLI as a thin
  client (so a human and an AI operator share the exact same control surface)?
  Lean: MCP is the surface; CLI and any UI are equal thin clients over it.
- **OQ2.** Does `run drive` (daemon-owned auto-supervise) belong here or in RFC
  0097? Proposal: the *capability* is RFC 0097; the *operator-facing default*
  ("driving a run should not require a hand-rolled loop") is an acceptance
  criterion here.
- **OQ3.** How much internal id should ever surface? A strict rule ("operator
  verbs accept only run + workflow_job_id; internal ids are output-only
  provenance") vs. pragmatic exceptions. Measure against real recovery flows.
- **OQ4.** The `attention` view's "one recommended next action" requires the
  daemon to rank blockers by what unblocks the most downstream work. Is that a
  heuristic or derived from the job DAG? Lean: DAG-derived (the daemon already
  knows `blocked_downstream_jobs`).
- **OQ5.** Honest operator-language labels (Track 2/5) need a mapping from
  internal stall classes to operator phrasing. Where does it live — daemon
  projection or client? Lean: daemon, so all clients agree.

## Acceptance — how we will know operator load dropped

The bar is behavioral and measurable against a representative run:

1. **Single surface:** the run is driven start-to-finish with only `striatum`
   verbs; `tmux`/`systemctl`/`psql`/operator-scripts appear zero times in the
   normal path (Track 1).
2. **One-glance attention:** for any blocked state, a single command names the
   blocking job, an honest reason, and one runnable next action — no
   multi-command assembly (Track 2).
3. **Verification beats guessing:** every identifier the operator reuses is read
   back in one call and copy-exact; a malformed/confabulated id fails at the call
   site with a "did you mean" (Track 3/4).
4. **No internal-id wrangling:** `why` and the common recovery verbs operate on
   `(run, workflow_job_id)`; the operator never hand-resolves a `sess_/sup_/…`
   id for a normal action (Track 4).
5. **Trajectory on the surface:** a lane's trajectory is readable via one CLI
   verb, never via raw `tmux capture-pane` (Track 5).
6. **The error class shrinks:** re-running the kind of long, multi-phase drive
   that produced the 2026-05-31 post-mortem, the operator makes zero
   identifier-confabulation, result-confabulation, or repeated-known-mistake
   errors — because the structure no longer rewards guessing.

[RFC 0050]: 0050-operator-ui-rework-and-provenance-honesty.md
[RFC 0075]: 0075-tmux-observable-mcp-agent-sessions.md
[RFC 0097]: 0097-full-workflow-run-orchestration.md
[RFC 0099]: 0099-constrained-operator-mode.md
[RFC 0101]: 0101-robust-autonomous-workflow-execution.md
