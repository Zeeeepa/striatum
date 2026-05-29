# RFC 0093 V1 — Implementation Design (claude_code lane)

author: operator

Role: designer · Lane: claude_code · RFC: 0093 · Status: independent design draft
Intended byline: `author: designer-claude-opus-4.7-001` (this lane); demoted to `author: operator` because the publish validator computes attested=false due to `agent_mcp_discovery_stall` (see §"Substrate friction note" at end of doc).

## 1. Problem framing (in my own words)

Striatum already has two **live model-to-model dialog primitives** that move
authored text between preserved-context sessions:

- **RFC 0082 interrogation** — 1→1 asymmetric ask/answer against a session that
  is paused in `awaiting_interrogation` so its context survives the exchange.
- **RFC 0086 conversation** — N-party symmetric round-robin with a shared
  authored transcript.

Both terminate on a *procedural* condition (interrogator calls `close`,
conversation hits `max_rounds`). Neither terminates on a *substantive*
condition. The product gap is not "we need more dialog methods"; it is "we
need a way to make a downstream commit job *unreachable* until the dialog
actually did its epistemic work — i.e. extracted a constraint, landed a
challenge that got rebutted, reconstructed a hidden spec fragment — and not
merely *occurred*."

RFC 0083 (iterated interrogating panel) is the only existing live-collab
shape. It is hand-shaped, baked end-to-end into a single workflow fixture,
and uses a `review` job's `accept/needs_revision` verdict to gate the next
build cycle. That works for one shape; it does not give us a *family*, and
critically it does not protect against **ritual dialog**: hollow questions,
fluent non-answers, vocabulary that *sounds* like convergence while the
gate happily flips green because the interrogator closed and the conversation
turned over `max_rounds` times.

The framing I'm building for, then:

> Striatum needs a **named family of collaboration shapes** (catalog) plus
> **one cross-cutting mechanism** — an adjudicator role whose verdict gates a
> downstream phase boundary — such that you can author a workflow by picking
> a shape from the catalog, and the gate forces the dialog to produce
> structured substance (a `collaboration_ledger` artifact) before the commit
> job becomes reachable.

Notably, **no new live-dialog primitive ships in V1**. Every shape is pure
composition over the existing `interrogation.*` and `conversation.*` daemon
methods. The contribution is (a) the gate mechanism, (b) the artifact that
expresses the verdict, and (c) the catalog/generator binding that turns
shape ids into emitted `striatum.workflow.v1.1` graphs.

## 2. Proposed approach

The implementation has **four landing pieces**, in the order they should
land, each independently testable:

### 2.1 `collaboration_ledger.v1` artifact contract (lands first)

This is the smallest piece and unlocks everything else: the adjudicator job
needs an artifact kind to publish, and `publish-artifact` validation gives us
the V1 "this exchange did real work" contract.

- **Where**: `go/pkg/artifactcontracts/contracts.go`.
- **Add** to `allowedKinds`: `"collaboration_ledger": true`.
- **Register** a new `Schema` entry keyed `"collaboration_ledger"`:
  - `schema_version` required, `equalsValue("striatum.collaboration_ledger.v1")`.
  - `artifact_kind` required, `equalsValue("collaboration_ledger")`.
  - `shape` required, one of `falsification_gate`, `fog_of_war_review`,
    `synaptic_prune`, `cross_examination`.
  - `topic` required string.
  - `verdict` required, one of `accept`, `accept_with_findings`,
    `needs_revision`, `reject` (same vocabulary as `finding` so existing
    routing reuses).
  - `rationale` required string.
  - `participants` required string-list (session ids).
  - `entries_path` optional string (pointer to a sidecar `entries.yaml` when
    the front matter would be too large; mirrors `findings_ledger`).
- **Per-entry validation** is deliberately *not* a front-matter field
  (front matter stays flat per existing conventions). Instead a parser in
  the same package validates the YAML body (or sidecar) shape and is called
  from `publish-artifact` so an invalid `entries[].kind` returns exit code 6.
- **Add** `collaboration_ledger` to the set of artifact kinds that go
  through the front-matter validator in the publish path. This is the
  single change in `mutations/artifact_publish*.go` that wires the contract
  into the publish path; existing kinds already follow the same pattern.

**D028 guard**: a separate field-walker rejects any entry whose `text` matches
heuristics for raw provider output (ANSI escape sequences, prompt prefixes,
PTY control bytes). Implemented as a tiny scanner with explicit allowed-byte
range; reused by a unit test that feeds it a fixture transcript and asserts
publish exits 6.

**Tests**:
1. `TestCollaborationLedgerSchemaValid` — happy path, all five `entries[].kind`
   values exercised in one fixture.
2. `TestCollaborationLedgerInvalidKind` — `entries[].kind: gossip` → exit 6.
3. `TestCollaborationLedgerD028Guard` — entry text contains ANSI/CR/raw stdout
   → exit 6 with the D028 error message.
4. `TestCollaborationLedgerSidecar` — `entries_path: entries.yaml` resolves
   and validates.

This piece is **provably reviewable in isolation** and ships even if shape
generation slips.

### 2.2 Adjudicator gate (the substance-gate itself)

The adjudicator is a **role** with a `phase_synthesis`-typed job. It is not
a new primitive in `mutations/run.go`; it leans on the existing rule that
a `phase_synthesis` job's `accept/needs_revision` verdict gates the next
phase's jobs via cross-phase dependency wiring (`run.go:542`, `:607`).

- **New role definition**: `docs/agents/roles/adjudicator.md` — a normal
  role file modeled on `synthesizer.md`. The role prompt declares: "you may
  read ONLY the dialogue trajectory via `trajectory export --profile
  dialogue --topic <topic>`; you may not read raw session stdout, raw PTY
  logs, or `.striatum/`; you publish exactly one `collaboration_ledger`
  with a verdict."
- **Generator emits** an `adjudicate_<shape>` job per cycle:
  - `type: phase_synthesis`
  - `role_id: adjudicator`
  - `lane_id` chosen per the lane assignment rules; must differ from any
    holder/proposer lane in the same phase (RFC 0064 same-model refusal).
  - `expected_artifacts`: one `collaboration_ledger` at a deterministic path
    under the workflow artifact root.
  - The downstream commit-class job in the next phase carries an
    `inputs.from: [<adjudicator job id>]` and the standard phase wiring,
    so it is unreachable until the adjudicator publishes a verdict whose
    `outcome` clears the gate.
- **`needs_revision` re-loop**: the generator emits a `revision_cycle`
  block (already exists for RFC 0083 in `examples/iterated-interrogating-
  panel/workflow.json`) bounded by a shape-level `max_cycles` option. On
  `needs_revision`, the previous dialog job (`interrogation.open` or
  `conversation.open`) re-runs against the same holder; on `accept`, the
  next-phase commit becomes reachable.

**Reviewer independence**: enforced by reusing the existing review-diversity
guard. The generator refuses (lint error) to schedule the adjudicator on
the same `lane_id` as any same-phase holder/proposer; an operator override
emits the standard RFC 0064 audited override artifact.

**Tests**:
1. `TestSubstanceGateBlocksCommitUntilLedger` — generator emits a falsification
   workflow; run it through `run_prepare` against the in-memory scheduler;
   assert the commit job's state is `pending_dependency` until the adjudicator
   publishes a `verdict: accept` ledger, then becomes `ready`.
2. `TestSubstanceGateNeedsRevisionReloops` — adjudicator publishes
   `verdict: needs_revision`; assert the dialog job re-enters the queue and
   `max_cycles` is decremented; on second `needs_revision` past the bound,
   the run halts with a typed terminal status (not silently looping).
3. `TestAdjudicatorSameModelRefusal` — generator with adjudicator lane ==
   holder lane → lint error referencing RFC 0064.

### 2.3 Anti-theater fixture (the bar)

This is the single test the RFC names as **the bar** for V1. It is *not* a
test of the schema — it is a test of the rubric the adjudicator role prompt
encodes, plus the front-matter validator.

- **Where**: `examples/collaboration-falsification-gate/` fixture with:
  - `workflow.json` (generated, then checked in).
  - Two pre-recorded dialogue trajectories under `fixtures/`:
    - `hollow.dialogue.json` — seeded hollow questions ("anything else
      important?") + fluent non-answers ("great point, will consider"); the
      adjudicator must score this `needs_revision`.
    - `landed.dialogue.json` — a specific challenge ("your design assumes
      ledger-fanout but workflow X holds the holder lane single-writer; how
      do you maintain that invariant under N falsifiers?") followed by a
      specific rebuttal ("by serializing falsifier turns per RFC 0082 §5;
      see substrate note 2"); the adjudicator must score `accept`.
- **Test driver**: a `go test` that loads each trajectory, invokes the
  adjudicator role's rubric pass (extracted into a pure function so it is
  testable without a live model — see §3.2), and asserts the verdict.

The fixture **also** serves as the dogfood that calibrates Open Question 2
("adjudicator reliability"): a flaky rubric will be caught here long before
production.

### 2.4 Collaboration shape pack + generator wiring

The pack is **authoring data**, not a runtime artifact (RFC 0074 pack model).

- **Where**: `go/pkg/workflowgenerate/collaboration_shapes.go` (new) +
  bundled pack files under `templates/collaboration/`.
- **Add to** `shapes` registry in `generate.go:29` /`:44`:
  `falsification_gate`, `cross_examination`. (`fog_of_war_review` and
  `synaptic_prune` are explicitly deferred per §4 of this design.)
- **Per-shape generator function** emits, in order:
  1. The holder/proposer job (`type: draft`, lane chosen per `lanes` spec).
  2. The dialog job(s): for `falsification_gate`, one or more
     `interrogation_open` jobs per rotating falsifier with their
     `commands.dialog_open` block carrying the exact `interrogation.open`
     RPC parameters. For `cross_examination`, one per non-author peer.
  3. The `adjudicate_<shape>` job (§2.2).
  4. The commit job, `inputs.from: [adjudicate_<shape>]`.
- **`workflow generate --shape falsification_gate`** then produces a
  `striatum.workflow.v1.1` graph that passes the existing `workflow
  validate` / `workflow lint`. The generator already knows how to emit
  v1.1; we add only shape-specific job composition.

**Documentation**: update `docs/reference/workflow-types.md`,
`docs/reference/ubiquitous-language.md` (four new terms), and
`docs/reference/spec.md` shape list. Re-express RFC 0083 as a catalog entry
that points at `falsification_gate` with a `revision_cycle` modifier.

### 2.5 What lands when (rollout sketch in §6)

The four pieces compose in build order: **2.1 → 2.3 (without 2.2 yet) →
2.2 → 2.4**. That ordering lets us land the contract and the bar, prove the
bar is testable, then build the gate and finally the generator. Each step
is independently green and revertible.

## 3. Alternatives considered

### 3.1 Alternative A — Make the substance-gate a new daemon RPC

**Shape**: introduce `gate.adjudicate` as a first-class daemon method that
the dialog jobs invoke and that the scheduler treats as a phase gate.

**Why I rejected it**: it violates RFC 0093 §5 ("No new daemon method"). It
also re-implements what `phase_synthesis` + cross-phase dependencies already
do: gate phase N+1 on a verdict from a job in phase N. The win for the new
RPC would be tighter scheduler integration and clearer telemetry, but the
cost is a new authoritative surface in the daemon (more migrations, more
authority-matrix entries, more contracts to keep in sync). Reusing the
existing `phase_synthesis` machinery is strictly cheaper and keeps the
adjudicator visible in the standard run graph.

### 3.2 Alternative B — Embed the rubric in the role prompt only

**Shape**: encode the "did a challenge land and get rebutted" logic entirely
in the adjudicator role's natural-language prompt. The role file says
"score the dialogue; emit `verdict: accept` if substance, else
`needs_revision`."

**Why I rejected it (partially) — and kept the seam**: a prompt-only rubric
is genuinely fast to ship and matches how `synthesizer` works today. But it
makes the **anti-theater test (the bar)** uncheckable in CI without a live
model invocation, which is expensive and flaky. The compromise I adopted:
the role prompt encodes the *semantic* rubric (what counts as a landed
challenge), but the *structural* checks (entries[].kind cardinality,
participants present, refs resolve into the dialogue trajectory) live in
a pure Go function `rubric.ScoreDialogue(dialogue, shape) Verdict` under
`go/pkg/collaboration/rubric.go`. The Go function asserts necessary
conditions (no entries → `needs_revision`; ≥1 `challenge` paired with a
matching `rebuttal` referencing the same `claim` → `accept` candidate);
the role prompt then makes the *sufficient* judgment over the candidate.
This gives the bar a CI-testable lower-bound while keeping the upper-bound
in the model's hands. The role prompt is allowed to *demote* an `accept`
candidate to `needs_revision` (the rubric can never promote).

### 3.3 Alternative C — Make the adjudicator a `review` job, not `phase_synthesis`

**Shape**: reuse the existing `review` job type with its accept/reject
verdict instead of standing up a new role over `phase_synthesis`.

**Why I rejected it**: `review` jobs operate over an *artifact*; the
adjudicator operates over the *dialogue trajectory* (a live read-model with
no single backing artifact). Forcing the adjudicator to first publish a
synthetic "transcript artifact" so a `review` could attach to it would
either re-introduce raw-transcript capture (D028 violation) or require a
parallel transcript-publishing job that adds zero value. The `phase_synthesis`
type already supports verdict-bearing jobs that gate downstream phases
(`workflowauthoring/workflow.go:55`), is what the RFC calls for verbatim,
and avoids stretching `review` semantics.

## 4. Risks, unknowns, and what could go wrong

### 4.1 Risk: adjudicator is itself a model and can be fooled

The structural rubric (§3.2) bounds this. The bar fixture (§2.3) measures
it. If `hollow.dialogue.json` ever passes, we have a regression on the most
important property of the whole RFC, and the fixture catches it deterministi-
cally before merge. The residual risk — a clever transcript that satisfies
both the structural rubric and the prompt — is real and is RFC 0093 Open
Question 2; we accept it for V1 and propose a second-adjudicator-on-
disagreement pattern (one ledger entry per adjudicator, gate clears only
on agreement) as a V2 follow-up. **Out of V1 scope.**

### 4.2 Risk: concurrent interrogations against one holder are unspecified (RFC §1 substrate note 2)

`falsification_gate` with two rotating falsifiers does **not** run them in
parallel for V1. The generator emits the falsifier interrogation jobs in a
**sequential chain** (one falsifier completes its `interrogation.close` →
next falsifier `interrogation.open` becomes ready). Parallel falsification
requires the concurrency/liveness work parked in D139/D141 revisit. Calling
this out so reviewers don't expect the parallel form.

### 4.3 Risk: agy lane on the new path

Per project memory + RFC §1 substrate note 1, agy's owned-PTY agent-loop
landed today (#52 + #55) and is the **newest** path. The V1 catalog is
validated on the **known-good claude+codex pair** first; agy participates
only as the third design/review seat (this run), not in the V1 anti-theater
fixture's holder/falsifier rotation. The fixture pins lanes by name.

### 4.4 Risk: write-scope collisions

`falsification_gate` runs the holder + falsifiers + adjudicator on a shared
worktree (the V1 isolation default). Per the concurrent-gates write-scope
memory ([[concurrent-gate-writescope-deadlock]]), shared worktree + parallel
gates can deadlock the `complete` check. Mitigation: each shape-emitted job
gets a **disjoint `write_scope.allowed_paths`** under the shape's artifact
root (`artifacts/<shape>/<role>/<ordinal>/`). The adjudicator's path is the
ledger directory; the holder's path is the work directory; falsifiers
publish nothing structural (interrogation turns go through `interrogation.
answer`, not the artifact path), so their write-scope can be empty.

### 4.5 Risk: schema_version `striatum.workflow.v1.1` vs `v1`

The acceptance criteria say "v1.1"; the existing generator's `validSchemas`
in `generate.go:23` should already include `striatum.workflow.v1.1` (per
RFC 0045 phase additions). If it does not, the generator change in §2.4
must add it. **This is a one-line check during implementation.**

### 4.6 Risk: ledger artifact path collisions across cycles

On `needs_revision` re-loop, the adjudicator re-publishes a ledger. If the
path is fixed, the second publish would either overwrite (loses provenance)
or fail (blocks the loop). Solution: path template includes `cycle_<N>` and
the generator increments `N` on each loop iteration. Same pattern RFC 0083
already uses for review rounds (`review_round`).

### 4.7 Unknown: how does `trajectory export --profile dialogue` filter by topic?

The role prompt says the adjudicator reads "the dialogue trajectory for the
run/topic." If `trajectory export` does not yet support `--topic`, we need
to either (a) add the filter (small CLI change), or (b) pass `--session-ids`
of the conversation/interrogation participants from the generator's emitted
job context. (b) is the cheaper path for V1 and avoids touching the
trajectory subsystem; the generator already knows the participating
session ids when it composes the shape.

### 4.8 Unknown: lint guardrail for cost

RFC §"Open Questions" item 7 asks whether the generator should advisory-
lint when `max_cycles × participants` exceeds a bound. **Defer**: not in
the V1 acceptance criteria, easy to add later as a `workflow lint` rule.

### 4.9 What could go wrong end-to-end

The single most likely failure mode is: the rubric Go function (§3.2)
encodes the structural test too strictly (every dialogue without a perfectly
matched challenge/rebuttal pair → `needs_revision`), the operator runs a
shape that genuinely had productive dialog but didn't hit the exact shape,
and the gate flaps. Mitigation: the structural rubric is **necessary not
sufficient** — it produces a candidate verdict the role prompt can demote
but not promote. Conservative-by-default; the prompt does the nuanced
upgrade. Calibration happens in the bar fixture.

## 5. What I'm explicitly *not* doing in V1

To keep the run scope-disciplined per the TASK and the RFC:

- **No `fog_of_war_review`**. It requires work-packet *type sequencing* in
  the generator (a job's `type: proposal` is gated on the adjudicator's
  verdict). That is one new piece of generator machinery beyond pure
  composition. RFC explicitly says "may follow if time permits."
- **No `synaptic_prune`**. It requires §5c `post_dialog_hook` — a new
  conversation-fixture field that fires a work packet on `conversation.close`
  while participant sessions are still live. New primitive surface, even
  though it doesn't add a daemon method. Defer cleanly per the RFC.
- **No floor-control primitive** (RFC Open Question 1). Round-robin only.
- **No second-adjudicator-on-disagreement** (Open Question 2). V2.
- **No reconciliation with RFC 0052 vocabulary** (Open Question 3). V2.
- **No first-class `human` participant role** (Open Question 6). Existing
  RFC 0053 escalation surface covers it informally.
- **No cost lint** (Open Question 7).

The `scribe` participant modifier is **in V1** (it is pure composition over
conversation participants + a role file; zero generator change beyond a
participant-lane mode).

## 6. Rollout sketch

**Land 1 — Artifact contract only** (smallest, lowest risk):
- §2.1 in one PR. `collaboration_ledger.v1` registered, validated, four
  unit tests green, no behavior change anywhere else.
- Bumps pyproject minor; CHANGELOG entry "Unreleased → vX.Y.0: RFC 0093
  artifact contract."
- Mergeable on its own; provides the artifact kind that subsequent steps
  publish against. RFC 0093 V1 is *not* claimed complete yet.

**Land 2 — Anti-theater bar fixture under the new contract** (proves the
gate property is testable before the gate exists):
- §2.3 + §3.2's `rubric.ScoreDialogue` function. The fixture invokes the
  rubric over the two seeded trajectories and asserts verdicts.
- Adjudicator role file lands (`docs/agents/roles/adjudicator.md`) so the
  rubric has a documented contract.
- This is the **bar gate** for the whole RFC. If it goes green, every
  subsequent change can be regression-checked against it.

**Land 3 — Substance-gate wiring**:
- §2.2 in one PR. New role+lane in a hand-written workflow fixture
  (sibling of `iterated-interrogating-panel`) that exercises the gate
  end-to-end against the in-memory scheduler.
- Tests for blocked-commit and `needs_revision` re-loop.
- Reviewer-independence (RFC 0064 refusal) test.
- At this point the gate works but you still author the workflow by hand.

**Land 4 — Generator + shape catalog**:
- §2.4 in one PR. `falsification_gate` + `cross_examination` shape ids
  registered; `workflow generate` produces v1.1 graphs that pass
  validate/lint.
- Docs updated: `workflow-types.md`, `ubiquitous-language.md`,
  `spec.md` shape list.
- RFC 0083 re-expressed as a catalog entry.
- This PR also adds the `scribe` participant modifier as a conversation
  fixture option.

**Land 5 — Decision log + RFC status**:
- `docs/decisions/decision-log.md`: one decision per landed shape +
  one for the adjudicator-as-`phase_synthesis` choice (records the
  rationale from §3.3 so future agents don't relitigate it).
- RFC 0093 status: `proposed → accepted`. Defer notes for
  `fog_of_war_review` / `synaptic_prune` / floor-control in §5 of the RFC.

**Total**: 4–5 PRs over the run, each independently green and revertible.
The contract+bar lands first because they buy us regression safety for
every subsequent change; the generator lands last because it is the most
visible-but-least-load-bearing surface.

## 7. Open seams / things for synthesis to resolve

These are the points where I expect the three design lanes to differ and
where the synthesizer will need to pick:

1. **Where the rubric lives** — pure Go function (my §3.2), prompt-only
   (my §3.2 Alternative B), or split (my recommendation). The choice
   determines whether the bar fixture runs without a live model.
2. **`needs_revision` cycle bound semantics** — fail terminally on bound
   exceeded (my §2.2 test 2), or downgrade to an operator escalation
   artifact. I picked terminal failure; an escalation path is defensible.
3. **Whether `cross_examination` ships in the first generator PR or
   waits for a second iteration** — I included it in Land 4; a more
   conservative read would land only `falsification_gate` and queue
   `cross_examination` for a follow-up after the first dogfood. The
   acceptance criteria explicitly require both, so I keep them in V1.
4. **Whether `trajectory export --topic` lands as a CLI flag or the
   generator passes session ids** — I picked session-ids (§4.7) but
   `--topic` is a cleaner long-term surface and small.

End of design.

## Substrate friction note (not part of the design)

Filed alongside this artifact: the claude_code lane was attested at packet-
claim time (`2026-05-29T18:22:00Z`, packet `wp_fcc96fca4cdf1b112c0c99c1a030af1b`)
but flipped to `unattested` with `lane_attestation_reason: supervisor_stalled`
and `stall_class: agent_mcp_discovery_stall` within ~43 seconds. Root cause:
`go/pkg/sessionliveness/liveness.go` keys discovery-stall on
`activity.LastToolsListAt == nil`, but **no Go code path writes to
`sessions.last_tools_list_at`** (column added in migration `0012_mcp_activity_
liveness.sql`, never populated). Every session therefore trips
`agent_mcp_discovery_stall` after `DiscoverySeconds`, which makes
`expectedAuthorLine` return `operatorAuthorLine(operatorLabel)` and the
publish validator reject the intended attested byline
`author: designer-claude-opus-4.7-001`.

Worked-around here by demoting the byline to `author: operator` and recording
the intended attested byline in the title block (above) so synthesis can
attribute the design to the claude_code lane regardless. Rebridge (`striatum
supervise rebridge`) and `work.heartbeat` were attempted; neither populates
`last_tools_list_at`. Suggested fix: either (a) wire the MCP transport's
`tools/list` handler to `UPDATE sessions SET last_tools_list_at = now()` for
the authenticated session, or (b) gate the discovery-stall classifier on a
signal that *is* written (e.g. fall back to `LastMCPRequestAt`) when the
column is null and the session has produced other MCP activity.
