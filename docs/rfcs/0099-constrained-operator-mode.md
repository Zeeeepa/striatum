# RFC 0099: Constrained Operator Mode — control-surface-only AI operator

Status: proposed
Date: 2026-05-30
author: proposer-claude-opus-4-8-001

Context:
- [#92](https://github.com/halbritt/striatum/issues/92) — feature request from the
  Engram RFC 0069 Stage 1 run: today an AI **operator** runs arbitrary local
  shell and file edits, and Striatum only *audits* workflow state **around** the
  work packet. The asker wants a stricter mode where the operator is not in
  broad/yolo repository-edit mode and can act **only through the Striatum control
  surface**.
- [#91](https://github.com/halbritt/striatum/issues/91) — the concrete pain that
  motivates it: write-scope drift is discovered only at `work.complete`, after
  useful work is done. Constrained mode wants violations **prevented before
  mutation**, not reported after.
- [RFC 0096](0096-supervised-lane-trust-boundary.md) — the **sibling** boundary.
  RFC 0096 hardens the *supervised lane* (a process the daemon spawns) →
  control-plane boundary. This RFC hardens the *operator* (the human-driving AI
  that issues `run.*`/`session.*`/`supervise.*` verbs) → repository boundary.
  Different principal, shared mechanisms (scoped writes, capability-gated shell,
  evidence recording).
- [`docs/decisions/decision-log.md`](../decisions/decision-log.md) — **D005** (the
  coordinator owns next-action selection and invoking workflow commands; it does
  **not** write source patches or bypass deterministic gates unless assigned a
  job), **D009/D010** (agents act only through CLI/MCP that enforce invariants),
  **D028** (no transcript capture). Constrained operator mode is D005 made
  enforceable rather than advisory.

## Problem

A Striatum operator is an AI with its **own** tool harness (Claude Code, Codex,
…). Striatum gates *workflow state* (claims, leases, verdicts, artifact
validation), but it does not — and structurally **cannot** — gate the operator's
ambient authority: the operator can write any file, run any shell command, and
mutate the target repo outside any work packet. Three consequences:

1. **Late write-scope discovery.** A scaffold omission (a missing `allowed_paths`
   entry) only surfaces at `work.complete` (#91), after minutes of work.
2. **No mediated mutation.** Filesystem changes happen through the harness's raw
   `Edit`/`Write`/`Bash`, never through a Striatum-mediated, scope-checked verb,
   so Striatum has no pre-write veto and no structured record of *what changed
   why*.
3. **Implicit escape hatches.** Ambient authority is the default; stepping
   outside the control surface requires no explicit, audited decision.

The honest constraint: Striatum **cannot forcibly sandbox a process it did not
spawn**. So constrained operator mode is necessarily a **cooperative contract**
— the operator's harness opts into a restricted tool set — *plus* a Striatum-side
mediated surface that makes the restricted path ergonomic and the escapes
explicit. This RFC designs that surface and the cooperative protocol; it does not
claim to imprison a non-cooperating operator.

## Goals

- An opt-in **constrained operator profile** in which the operator receives work
  packets, context, and expected artifacts through Striatum and performs
  repository mutations through a **Striatum-mediated, scope-checked write path**.
- **Pre-mutation** write-scope checking: an out-of-scope write is refused before
  it lands, not at `work.complete` (subsumes #91's `scope-check`).
- Shell/test execution, when permitted, routed through **declared capabilities**
  and recorded as structured evidence (ties into RFC 0046 lane-evidence).
- Escape hatches that are **explicit, audited operator decisions** (a
  `striatum.decision`-style record), not ambient authority.

## Non-Goals

- Removing the human/AI operator or the harness — this is a *mode*, not a new
  runtime.
- A hardware/VM sandbox. Enforcement strength is bounded by the cooperating
  harness's tool restrictions plus Striatum's mediated surface.
- Changing supervised-lane behavior — that is RFC 0096.

## Proposal

### 1. Constrained operator profile + capability assertion
A run (or an operator session) may declare `operator_mode: constrained`. The
daemon records the assertion and exposes it in `status`/`run.summary` so the
boundary is visible. Constrained mode narrows the operator's **capability set**
to control-plane verbs plus the mediated write/exec verbs below; broad `admin`
stays available only behind an explicit escape decision.

### 2. Mediated, scope-checked write verb
A `repo.write`-family mutation (preview-then-apply, mirroring the RFC 0036 MCP
idiom): given an active packet, it accepts a path + content/patch, checks it
against the packet's `write_scope` **before** applying, and refuses out-of-scope
writes with the same vocabulary `work.complete` uses today. This is the
enforcement counterpart to #91's read-only `scope-check` diagnostic (which
Phase 1 ships first as the advisory half).

Initial implementation: `repo.write` accepts exact UTF-8 content for one
repo-relative file, validates the active session/lease and `repo_write`
`write_scope`, targets the active per-job worktree when present, and refuses
out-of-scope paths before any filesystem mutation. Patch preview/apply remains a
future Phase 2 extension.

### 3. Capability-gated shell + evidence
Permitted command classes are declared (build/test/lint); execution is recorded
as structured evidence rows (reuse RFC 0046's `process_executions`/evidence
surface) rather than vanishing into PTY scratch (D028). Undeclared commands
require an escape decision.

### 4. Explicit escape hatches
Leaving the constrained surface (a raw write, an undeclared command) requires an
audited `striatum.decision`-style record naming the reason — turning ambient
authority into a logged, reviewable exception.

Initial implementation: `decision.record` can mark a run-level decision as a
constrained-operator escape with `escape_decision: true`, `escape_surface`, and
`escape_action` in both the decision artifact front matter and
`decision.recorded` event payload. Enforcement that refuses specific future
surfaces absent such a decision remains a Phase 3 extension.

## Acceptance Criteria

- A run can declare `operator_mode: constrained`; it is surfaced in
  `status`/`run.summary`.
- The pre-mutation scope check refuses an out-of-scope write with the
  `work.complete` violation vocabulary, before the write lands (shares code with
  #91).
- Permitted shell classes execute and record evidence; undeclared ones are
  refused absent an escape decision. Initial implementation: `process.run`
  executes command arrays for active jobs that declare
  `capability_requirements.process_execution: true`, uses the supervised-lane
  allowlist environment, and records `process_executions` evidence plus
  `process.started`/`process.completed` events without durable stdout/stderr
  transcripts.
- Every escape is an audited decision record.
- A regression fixture drives a constrained operator session end-to-end (scoped
  write accepted, out-of-scope write refused pre-apply, escape decision logged).

## Phased plan

- **Phase 1 (no schema):** the read-only `scope-check` diagnostic (#91) — git
  diff vs active packet `write_scope`, reported while work is in progress — plus
  the `operator_mode` assertion + surfacing. Advisory only.
- **Phase 2:** the mediated `repo.write` verb with pre-apply scope enforcement.
  The exact-content single-file slice has landed; patch preview/apply remains.
- **Phase 3:** capability-gated shell + evidence recording + escape-decision
  records; cooperating-harness profile docs (restricted tool sets per adapter).
  The typed escape-decision record and mediated `process.run` evidence slices
  have landed; escape enforcement and end-to-end constrained fixtures remain.

## Open Questions

1. **Enforcement seam.** Should restriction live in the operator harness (e.g. a
   Claude Code restricted tool profile / hook) with Striatum providing the
   mediated surface, or can Striatum meaningfully gate at all without owning the
   process? (Leaning: cooperative harness profile + Striatum surface; name the
   limit honestly.)
2. **Relationship to RFC 0096.** If a future operator runs *as a daemon-spawned
   lane*, do 0096's OS-user sandbox mechanisms simply apply, collapsing this into
   0096? Or is the operator always a distinct, human-attached principal?
3. **Granularity.** Per-run vs per-session vs per-phase constraint scope.
