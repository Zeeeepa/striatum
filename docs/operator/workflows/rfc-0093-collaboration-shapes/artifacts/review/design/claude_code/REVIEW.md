---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags:
  - "ergonomics_dx"
  - "rfc-0093"
---

# RFC 0093 Design Synthesis — Ergonomics / DX Review (claude_code)

author: operator

Intended attested byline: `author: reviewer-claude-opus-4.7-001`
(this lane). Demoted to `author: operator` because the publish validator
computed `attested=false` for this session: the supervisor reports
`lane_attestation: unattested` and `lane_attestation_reason:
supervisor_stalled` with `stall_class: agent_mcp_discovery_stall`. This
is the same substrate gap the synthesis flags in §4.5 — no Go code path
writes `sessions.last_tools_list_at`, so every session trips
`agent_mcp_discovery_stall` after `DiscoverySeconds`. The review is
authored by the claude_code reviewer lane regardless; the friction is
filed for the operator and forwards the synthesis's existing call-out.

## Verdict

`accept_with_findings`

The synthesis is a sound first-time-user surface. The four-slice landing
order (contract → cycle router → gate → generator), the split structural /
semantic rubric, and the disjoint `write_scope` discipline are all
discoverable and consistent with existing Striatum patterns (`workflow
generate --shape <name>`, RFC 0064 lint-with-audited-override, RFC 0045
phase gating). One load-bearing ambiguity surfaced in the synthesis text;
the synthesizer pinned it down decisively in the single interrogation
round (see §"Interrogation").

The remaining findings (F2, F3) are surface-level affordance work for the
build phase. They do not justify `needs_revision` because (a) they are
purely additive operator-facing polish, (b) they do not change the
contract, gate semantics, or acceptance criteria, and (c) the synthesizer
cannot usefully resolve them without inventing build-phase spec under
interrogation. Build phase should treat them as recorded findings the
implementer addresses inside Slices 2 and 4.

## Posture and scope

Posture: `ergonomics_dx`. Evaluating the design from a first-time-user
perspective — operators reaching for `workflow generate --shape …`,
developers reading the Slice 1 PR, and operators inspecting a paused run.
Verdict acceptance means the affordances are discoverable and consistent;
it does not endorse correctness of the rubric logic itself (that is the
`threat_model` / `devils_advocate` reviewers' surface).

Read scope: the design synthesis only (`fresh` context per `review_policy`).
The interrogation answer below is also load-bearing for the build phase.

## Interrogation

Rounds used: **1 of 3**. Stop reason: **open findings resolved by the
answer; remaining findings (F2, F3) are build-phase surface decisions the
interrogation cannot usefully advance**.

Target session: `sess_187f1fca15b51e5debc33dae19f7d6f3` (synthesizer
claude_code, `awaiting_interrogation`). Interrogation
`intg_46e56b08ec71f9205496d52fa6f379ca`.

### Q1 (asked) — Where does the clearing-substance check live?

The synthesis read as self-contradictory:

- §1.2 / §5.1: the `collaboration_ledger` validator in
  `go/pkg/artifactcontracts/contracts.go` rejects ledgers whose verdict is
  `accept` / `accept_with_findings` and whose `entries[]` lacks ≥1 claim,
  ≥1 challenge, ≥1 rebuttal each with non-empty `refs[]`. Slice 1's
  file-touch list explicitly puts this in `contracts.go`.
- §4.2: "shape-check only in `artifactcontracts`" and "the split-rubric
  structural Go function lives in `go/pkg/collaboration/rubric.go`".

A first-time implementer reading Slice 1 cannot tell which function lives
where.

### A1 — Synthesizer pin-down (paraphrased; full text in interrogation turn 1)

The "structural Go function" is actually two distinct checks the
synthesis collapsed into one:

| Check | What it asserts | Postgres-bound? | Lives in | Lands in |
|---|---|---|---|---|
| **A. Count-of-kinds** | Clearing-verdict ledger has ≥1 claim, ≥1 challenge, ≥1 rebuttal, each with non-empty `refs[]`. | No | `go/pkg/artifactcontracts/contracts.go` | **Slice 1** |
| **B. Trajectory resolution** | Each `entries[].refs[]` `dialogue:<seq>` resolves to a real turn whose session id matches `entries[].by`. | Yes (reads `dialogue` trajectory) | `go/pkg/collaboration/rubric.go` (new package) | **Slice 3** |

Authoritative call path from `striatum publish-artifact` for
`kind=collaboration_ledger` (post-Slice-3):

1. CLI → daemon RPC `artifact.publish` →
   `go/pkg/mutations/artifact.go::HandleArtifactPublish`.
2. Handler parses markdown + extracts front matter via existing
   `artifactcontracts.FrontMatterBlock` /
   `artifactcontracts.ParseFrontMatterBlock`.
3. Handler calls `artifactcontracts.Validate(kind, frontMatter)` →
   schema-field validators + **Check A**. DB-free.
4. Only when `kind == collaboration_ledger` AND `verdict ∈ {accept,
   accept_with_findings}`: handler calls
   `collaboration.ResolveRefs(ctx, runner, repositoryID, runID, topic,
   frontMatter)` → **Check B**. Postgres read.
5. Check B's call site is `mutations/artifact.go`, **not** `contracts.go`
   — `contracts.go` stays DB-free per the codex package-boundary
   discipline.

Slice 1 does **not** introduce `go/pkg/collaboration/`. The package and
the trajectory-resolution check both land with Slice 3, alongside the
adjudicator role file (which carries the agy/§4.1 semantic prompt
questions) and the hand-authored fixture.

Operational consequence the implementer should know: between Slice 1
landing and Slice 3 landing, Check A is enforced but Check B is not.
A hand-authored workflow could publish a ledger whose `entries[].refs[]`
contains fake `dialogue:99` strings — Check A would pass. Acceptable
because no V1 shipped shape publishes a ledger before Slice 4 (the
generator), and Slice 3 lands before Slice 4. The contract is dormant
for V1 paths during the Slice 1 → Slice 3 window; off-V1 dogfoods that
publish a ledger by hand in that window get Check A only — operator
responsibility.

### Q1 — Finding (F1, resolved)

The synthesis text needs three editorial corrections folded into the
build phase, taken verbatim from the synthesizer's answer:

- **§1.2 second bullet** should read: "Structural (split across two
  layers): count-of-kinds in `contracts.go` (Slice 1); trajectory
  resolution in `collaboration/rubric.go` (Slice 3)."
- **§4.2** should state explicitly that `artifactcontracts` owns shape +
  count-of-kinds and `collaboration/rubric.go` owns trajectory resolution,
  with the package introduced in Slice 3.
- **§5.1** is correct for Check A but should add a one-line note that
  Check B lands in Slice 3 against the same publish path
  (`mutations/artifact.go` call site).

The synthesizer explicitly stated it will not edit the published
synthesis artifact; the **build phase should treat the interrogation
answer as the authoritative pin-down** and reflect it in the Slice 3 PR
description. F1 is **resolved by the answer** and consumes no further
revision budget.

### Why I did not ask Q2 / Q3

The two remaining open findings (F2, F3 below) are *surface-affordance
specs the synthesizer cannot resolve without inventing build-phase
spec*. The synthesis correctly leaves them to the build phase. Asking the
synthesizer would have either produced premature commitments or repeated
"defer to build phase" — neither advances the verdict. The findings are
recorded below for the build phase implementer.

## Findings

### F1 — Substance-check file ownership (resolved)

**Severity:** medium → resolved.
**Source:** §1.2, §4.2, §5.1 read as self-contradictory.
**Status:** RESOLVED via interrogation Q1/A1 above. The three editorial
corrections to §1.2, §4.2, §5.1 should land in the build-phase PR
descriptions (Slice 1 description notes Check A; Slice 3 description notes
Check B and the call site).

### F2 — Operator visibility on cycle-router state (build-phase action item)

**Severity:** low.
**Source:** §4.1 (build-phase resolution of `needs_revision` cycle bound)
+ Slice 2 acceptance gates.

The synthesis adopts codex's path: on `max_revision_cycles` exhaustion,
the cycle router opens "the existing human checkpoint" with reason
`cycle budget exhausted`. This preserves operator agency, which is good.
But the design does not specify what the operator sees that distinguishes
this checkpoint state from any other human checkpoint pause:

- Does `striatum run detail` (the JSON) expose `cycle_iteration`,
  `cycle_budget_remaining`, `cycle_exhaustion_reason`, or is the
  exhaustion-vs-cycling distinction inferable only from the checkpoint
  reason string?
- Does `striatum dashboard --run-id …` (the human terminal view)
  surface "cycle 1/1 — budget exhausted" or just the generic
  human-checkpoint pause indicator?
- What verbs does the operator have? `run.resume` after a manual fix? Is
  there a way to extend the cycle budget in-place, or does the operator
  have to author a new run?

**Recommendation:** Slice 2 PR should ship at minimum:

1. A `cycle_state` block in the verdict-bearing job's `job.detail` /
   `run.detail` output (cycle number, budget remaining, last verdict).
2. A distinct `human_checkpoint.reason` value (`cycle_budget_exhausted`,
   not just a free-text string) so the dashboard can render a specific
   chip.
3. A `striatum run detail`-readable indication that the run halted on
   cycle exhaustion vs. on operator-issued pause.

This is purely additive operator-facing polish. It does not change the
gate logic.

### F3 — Generator surface discoverability (build-phase action item)

**Severity:** low.
**Source:** §2 "carry-forward credit — generator options" + Slice 4
acceptance gates.

The synthesis enumerates the generator options:

- `topic` (required)
- `max_dialog_rounds` (default 3)
- `max_revision_cycles` (default 1)
- `falsifier_count` (default 2, `falsification_gate` only)
- `include_scribe` (default false, modifier)

But it does not specify how a first-time operator discovers them.
`striatum workflow generate --help` is the most common discovery surface.

**Recommendation:** Slice 4 PR should ensure:

1. `striatum workflow generate --shape falsification_gate --help` (and
   the same for `cross_examination`) prints each option with type,
   default, and required/optional state.
2. `docs/reference/workflow-types.md` (which Slice 4 already updates) has
   a per-shape options table aligned with `--help` output.
3. `workflow templates show <shape>` (if it exists) prints the same
   option metadata.
4. `include_scribe: true` is documented as a modifier on both
   shipping shapes, not as a standalone shape — matching the synthesis
   §4.4 ruling.

### F4 — Adjudicator role file location and authoring path (build-phase action item)

**Severity:** low.
**Source:** §2 carry-forward (claude_code/§2.2 "`adjudicator` as a
normal role file (`docs/agents/roles/adjudicator.md`) modeled on
`synthesizer.md`").

The synthesis names the location but not how an operator authoring a
custom collaboration shape discovers and consumes it.

**Recommendation:** Slice 3 PR should:

1. Cross-reference the adjudicator role file from
   `docs/reference/ubiquitous-language.md`'s new `adjudicator` entry.
2. State whether the shape pack ships with a default adjudicator role
   binding (so operators get it for free with `workflow generate
   --shape …`) or whether operators must opt in by binding the role in
   their own packs.
3. Cite the role file from the Slice 3 fixture so the example workflow
   demonstrates the canonical adjudicator wiring.

### F5 — `cycle_<N>` path template applies to what (build-phase action item)

**Severity:** low.
**Source:** §2 carry-forward (claude_code/§4.6 `cycle_<N>` path template
for the ledger).

The synthesis adopts the `cycle_<N>` path template "for the ledger" but
does not specify whether other artifacts in the same revision cycle
(re-generated holder synthesis, falsifier interrogation outputs, scribe
notes) share the namespacing, or only the ledger does.

**Recommendation:** Slice 3 / Slice 4 PRs should:

1. Pin down whether `cycle_<N>` is a ledger-only suffix or a per-cycle
   directory under each role's `write_scope.allowed_paths`.
2. Show the resulting directory shape in the example fixtures
   (`examples/collaboration-falsification-gate/` and
   `examples/collaboration-cross-examination/`).
3. If only the ledger uses `cycle_<N>`, document that other artifacts in
   subsequent cycles overwrite their predecessors (and that this is
   acceptable because they are not gating evidence the way the ledger
   is).

## Strengths (deliberately noted to inform future reviewers)

- **`workflow generate --shape <name>`** lines up cleanly with the
  existing generator surface — first-time users will not need a new
  vocabulary to find these shapes.
- **Split rubric** is a strong DX call: the CI-testable Go layer means
  the anti-theater bar is enforceable without a live model, which is
  ergonomic for both contributors writing tests and operators trusting
  the gate.
- **Independently-green slices** with explicit "scope NOT in this PR"
  lists (Slice 1 §5.2) are an excellent maintainer-facing affordance —
  a first-time Slice 1 implementer can land, deploy, and stop.
- **Disjoint `write_scope.allowed_paths`** under
  `artifacts/<shape>/<role>/<ordinal>/` is the correct response to the
  known `concurrent-gate-writescope-deadlock` failure class.
- **Lint-with-audited-override for same-model adjudicator** keyed on
  `role_id` OR expected artifact kind catches handcrafted workflows
  that drop the role label — robust against operator slip.
- **Cycle-budget-exhausted → existing human checkpoint** preserves
  operator agency and matches the existing recovery path (no new
  terminal run state to learn).
- **Substrate-friction note (§4.5)** correctly flags the
  `agent_mcp_discovery_stall` attestation gap as out-of-scope for
  RFC 0093 V1 while still recording it for operator follow-up — exactly
  the right scope discipline.

## What the build phase should treat as authoritative

1. The two-check decomposition (Check A in Slice 1 `contracts.go`,
   Check B in Slice 3 `go/pkg/collaboration/rubric.go`) from the Q1
   answer.
2. The exact `publish-artifact` call path enumerated in the Q1 answer
   (handler → `artifactcontracts.Validate` → conditional
   `collaboration.ResolveRefs`).
3. The synthesis text corrections to §1.2, §4.2, §5.1 listed in the
   F1 resolution.
4. F2 / F3 / F4 / F5 as Slice-N action items, not as revision
   requirements.

## Verdict justification (one paragraph)

The synthesis is acceptable as the design input to the build phase.
The single load-bearing ambiguity (F1) was resolved cleanly in one
interrogation round, with the synthesizer providing the authoritative
two-check decomposition and call path. The remaining findings (F2–F5)
are purely additive operator-facing affordance work that belongs in the
relevant Slice PRs, do not change the contract or gate semantics, and
do not require the synthesizer to revise the published artifact.
Issuing `needs_revision` over surface-polish findings the build phase
already needs to make would burn a revision cycle without changing the
design decisions. Verdict: `accept_with_findings`.
