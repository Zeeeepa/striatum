---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs:
  - "docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/design/claude_code/DESIGN.md"
  - "docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/design/codex/DESIGN.md"
  - "docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/design/agy/DESIGN.md"
---

# RFC 0093 V1 — Design Synthesis

author: operator

Intended attested byline: `author: synthesizer-claude-opus-4.7-001` (this lane).
Demoted to `author: operator` because the publish validator computed
`attested=false` for this session: the supervisor reports
`lane_attestation_reason: supervisor_stalled` and
`stall_class: agent_mcp_discovery_stall`. Root cause is the same one the
claude_code design lane filed alongside its DESIGN.md — no Go code path
writes to `sessions.last_tools_list_at`, so every session trips
`agent_mcp_discovery_stall` after `DiscoverySeconds`, and the publish
validator returns `operatorAuthorLine(operatorLabel)` instead of the
intended attested byline. This synthesis is authored by the claude_code
synthesizer lane regardless; the friction is filed for the operator and
documented inside §4.5 of this synthesis.

## 1. The decision

Build RFC 0093 V1 as **four ordered slices** landed in this sequence:

1. **`collaboration_ledger.v1` artifact contract** — front-matter schema +
   kind-specific structural substance check + `review.submit` verdict-consistency
   guard.
2. **Cycle router for verdict-capable jobs** — make `recordVerdict(...,
   "needs_revision", ...)` honour declared workflow cycles instead of
   unconditionally opening a human checkpoint.
3. **Substance-gate wiring** — `adjudicator` role + `phase_synthesis` gate job
   in a hand-authored fixture; anti-theater test pinned against the structural
   rubric and the cycle router.
4. **Generator + shape catalog** — `falsification_gate` and `cross_examination`
   shape ids, `scribe` as a participant modifier, docs + RFC 0083 catalog
   re-expression.

`fog_of_war_review` and `synaptic_prune` are **deferred cleanly** per the RFC
and TASK. No standalone `scribe` shape; modifier only. No new daemon method,
no floor-control primitive, no economy/reputation store.

### 1.1 Shape of the gate

The substance-gate is the existing `phase_synthesis`-class job, *not* a new
daemon RPC. The adjudicator is an ordinary `role` lane that:

- reads **only** the curated `dialogue` trajectory (RFC 0081), never raw
  provider output;
- emits exactly one `collaboration_ledger` artifact;
- its verdict (`accept | accept_with_findings | needs_revision | reject`)
  gates the downstream commit/proposal job via ordinary RFC 0045
  cross-phase-dependency rules.

### 1.2 Shape of the rubric (the key contested call)

The rubric is **split** between a structural publish-time check and a
semantic prompt-time judgment:

- **Structural (Go, deterministic, CI-testable):** in
  `go/pkg/artifactcontracts`, the `collaboration_ledger` validator rejects any
  ledger whose front-matter `verdict` is `accept` / `accept_with_findings` and
  whose `entries[]` does not contain at least one `claim`, one `challenge`, and
  one `rebuttal`, each with non-empty `refs[]`. The validator further refuses
  the publish if `review.submit --verdict` does not equal the front-matter
  `verdict`. This is the **necessary** condition.
- **Semantic (adjudicator role prompt):** the prompt encodes "what counts as a
  landed challenge" and decides whether the structurally-valid ledger
  *deserves* an `accept` or should be downgraded to `needs_revision` /
  `reject`. The model can demote a structurally-clearing draft, but it **cannot
  promote** a structurally-unclearing one (the publish refuses).

This is the load-bearing synthesis call: it makes the anti-theater bar
**CI-testable without a live model** (against seeded ledger payloads) while
keeping the upper bound in the model's hands. Both forms of theater
(round-counter-satisfaction and confident-non-rebuttal) are blocked.

## 2. Carry-forward credit

### From `claude_code/DESIGN.md`

- **Four-slice landing order** (contract → bar fixture → gate → generator) and
  the framing that each slice is independently green and revertible (§2.5,
  §6).
- **Split rubric** (necessary-but-not-sufficient Go function + sufficient
  prompt, prompt can only demote) (§3.2). Adopted because it makes the
  anti-theater bar CI-testable.
- **`phase_synthesis` for the gate; not a new daemon RPC** (§3.1 rejected
  alternative A, §2.2). Adopted — the RFC explicitly forbids a new dialog
  primitive and this is the cheapest mapping onto existing scheduler
  semantics.
- **`adjudicator` as a normal role file** (`docs/agents/roles/adjudicator.md`)
  modeled on `synthesizer.md`, with the prompt declaring the
  trajectory-only read constraint (§2.2). Adopted.
- **Sequential falsifier turns** in V1 to stay inside proven concurrent-
  interrogation behavior (§4.2). Adopted; this matches all three designs.
- **Disjoint `write_scope.allowed_paths`** under
  `artifacts/<shape>/<role>/<ordinal>/` to dodge the
  `concurrent-gate-writescope-deadlock` failure mode (§4.4). Adopted —
  this is a real, recorded incident class the build must respect.
- **`cycle_<N>` path template** for the ledger so a `needs_revision` re-loop
  does not collide with the previous cycle's publish (§4.6). Adopted.
- **`agy` participates only as third design/review seat in V1, not in the
  bar fixture's holder/falsifier rotation** (§4.3). Adopted — agy's owned-
  PTY path is the newest (#52, #55) and V1 should be calibrated on the
  known-good `claude + codex` pair first.

### From `codex/DESIGN.md`

- **Critical pre-work: the cycle router** (§2). Adopted as **slice 2**, before
  the gate. `recordVerdict("needs_revision", ...)` currently opens a human
  checkpoint unconditionally; without the cycle router the gate's "routes
  back into a bounded dialog round" acceptance criterion is unsatisfiable.
  None of the other designs noticed this. The router scope (reset only the
  cycle slice reachable from `cycle.to` up to and including `cycle.from`,
  cancel stale claimable messages, terminate by opening the existing human
  checkpoint with a "cycle budget exhausted" reason when
  `max_iterations` is reached) is adopted verbatim.
- **`review.submit` verdict-consistency check** for `kind=collaboration_ledger`:
  the submitted `verdict` must match the front-matter `verdict`, otherwise the
  submit refuses (§1). Adopted — without this guard a lane could publish a
  `needs_revision` ledger and pass `--verdict accept` at the submit layer,
  defeating the structural rubric.
- **Kind-specific substance check at the publish layer**: clearing verdicts
  require at least one `claim`/`challenge`/`rebuttal` triple with refs; a
  `needs_revision` ledger may carry an unrebutted challenge; a `reject` ledger
  may carry unrebutted challenges or constraints (§1). Adopted as the
  structural half of the split rubric (§1.2 above).
- **`dialogue:<seq>` as the stable `entries[].refs` shape**, shape-checked in
  `artifactcontracts` (no DB resolution at the contract layer, no Postgres
  dependency in the validator package) (§1, §"Risks and unknowns"). Adopted.
- **Generator options** (§3): `topic` (required), `max_dialog_rounds`
  (default 3), `max_revision_cycles` (default 1), `falsifier_count`
  (default 2 for `falsification_gate`), `include_scribe` (default false).
  Adopted with one rename: `max_revision_cycles` is the same idea as
  `claude_code/§4.6`'s `cycle_<N>`; same number, single name.
- **`refuse single_agent` for these shapes** unless the operator uses the
  existing same-model override path (§3). Adopted — the whole point of an
  adjudicator is independent judgment.
- **Front-matter requires the block for `collaboration_ledger`** even though
  existing schemas permit optional front matter (§1). Adopted — a
  `collaboration_ledger` without a front-matter block is not a ledger.
- **Theater-bypass test** (publish `verdict: accept` without
  challenge/rebuttal pair → publish fails before any downstream job can
  enqueue) (§6). Adopted as the third anti-theater fixture, complementing
  the hollow-vs-landed pair from `claude_code/§2.3`.
- **D028 guard via "unknown fields rejected" structurally** (§1). Adopted in
  preference to `claude_code/§2.1`'s ANSI/PTY-byte scanner (see §3
  rejection).

### From `agy/DESIGN.md`

- **Explicit adjudicator rubric questions** ("What was the exact claim?",
  "What was the exact challenge?", "Did the rebuttal directly address the
  challenge's premise with evidence, or did it merely restate the claim?")
  (§4.1). Adopted verbatim into the adjudicator role prompt template — they
  are the cleanest articulation of the semantic check the structural rubric
  cannot replace.
- **No dynamic runtime-spawned dialogue loops** (§3 Alternative B rejected).
  Adopted: V1 sticks to static `workflow.json` graphs with bounded feedback
  cycles, as both other designs assume.
- **Reviewer independence enforced at workflow lint time** (§4.3 +
  `codex/§3`). Both designs converge on this; agy frames it most clearly.

## 3. What we are **not** carrying forward

### From `claude_code/DESIGN.md`

- **ANSI/PTY-byte D028 scanner** (§2.1). Rejected in favor of codex's
  "unknown-fields-rejected" structural approach. **Why:** the scanner is a
  heuristic with false-positive risk on legitimate authored text (a curated
  summary may legitimately mention `\r\n` or escape sequences in code spans).
  The schema-level "no transcript / stdout / stderr / payload field exists"
  guard is exhaustive at the contract layer; the bytewise sniff adds risk
  without adding coverage. If a leak appears later, a publish-time scanner
  can be added as a defense-in-depth pass, but not in V1.
- **Adopting `--topic` as a `trajectory export` CLI flag** (§4.7, §7.4).
  Rejected for V1 in favor of the cheaper "generator passes
  `--session-ids`" alternative the same section proposes. **Why:** the
  generator already knows the participating session ids at packet-build
  time; a new flag is a new surface to maintain. If `--topic` becomes
  ergonomic later for human operators, it lands as a follow-up.
- **`cross_examination` as a "more conservative read" deferral candidate**
  (§7.3). Rejected because the RFC's Acceptance Criteria item 1 explicitly
  requires both `falsification_gate` AND `cross_examination` in V1.

### From `codex/DESIGN.md`

- **Standalone `--shape scribe`** as a small conversation-plus-scribe
  fixture (§4 "If a standalone `--shape scribe` is needed"). Rejected
  unless it lands for free. **Why:** the RFC names `scribe` as a
  "participant modifier" (§3 catalog table, §"V1 shape catalog" row 5);
  promoting it to a top-level shape risks scope creep. Modifier only.

### From `agy/DESIGN.md`

- **No specific load-bearing claims rejected.** The agy design is sound but
  less concrete than the other two; nothing it advocates is dropped, but
  the implementation-level specifics come from `claude_code` and `codex`
  where agy's design did not reach implementation depth.

## 4. Unresolved contradictions and how the build phase should handle them

### 4.1 `needs_revision` cycle bound — terminal vs human-checkpoint

- **`claude_code/§2.2` test 2:** when `max_cycles` is exhausted, the run halts
  with a *typed terminal status*.
- **`codex/§2` step 2:** when `max_iterations` is exhausted, open the
  *existing human checkpoint* with a clear "cycle budget exhausted" reason.

**Build phase resolution:** adopt codex's path. The human-checkpoint surface
already exists; it preserves operator agency and matches the existing
verdict-bearing-job behavior the cycle router replaces only conditionally.
A terminal-failure status would be a new run-state addition without a clear
operator recovery path. Build phase ships codex's behavior and tests it.
If the dogfood surfaces operators who want a terminal status instead, that
is a V2 conversation.

### 4.2 Reference resolution depth for `entries[].refs`

- **`codex/§1` + §"Risks and unknowns":** shape-checked only (no DB
  resolution at the contract layer), to keep `artifactcontracts` free of a
  Postgres dependency.
- **`claude_code/§2.1`:** specifies that "refs resolve into the dialogue
  trajectory" as part of the structural Go rubric, which implies DB
  resolution.

**Build phase resolution:** shape-check only in `artifactcontracts` per
codex's stricter package-boundary discipline. The dialogue-trajectory
resolution belongs in the adjudicator's *publishing job* (it has the run
context and the trajectory read), not in the artifact contract validator
shared across all kinds. The split-rubric structural Go function lives in
`go/pkg/collaboration/rubric.go` (as `claude_code/§3.2` proposes), and that
function may read the trajectory at publish time; the contract validator
underneath it only shape-checks `dialogue:<seq>` strings.

### 4.3 Lint vs hard refusal for same-model adjudicator

- **`claude_code/§2.2`:** generator emits a lint error if adjudicator
  `lane_id` == any same-phase holder/proposer `lane_id`; operator override
  emits the standard RFC 0064 audited override artifact.
- **`codex/§3`:** "narrow lint rule" extends to ordinary
  `phase_synthesis`-class jobs that have a `collaboration_ledger` expected
  artifact, keyed on `role_id: "adjudicator"` OR the expected artifact kind.

**Build phase resolution:** adopt codex's keying (role OR expected artifact
kind) on top of claude_code's lint-with-audited-override semantics. The
narrow scope (don't reclassify every `phase_synthesis` job) is correct;
keying on either signal catches operator-handcrafted workflows that drop
the explicit `role_id: adjudicator` but keep the expected
`collaboration_ledger` artifact. RFC 0064 same-model refusal applies; the
override path is the standard audited artifact.

### 4.4 Scribe scope (modifier vs standalone shape)

- **`codex/§4`:** modifier-only for V1; standalone only if it composes for
  free.
- **`claude_code/§5`:** "in V1 as participant modifier"; no standalone.
- **`agy/§2.4`:** lists `scribe` alongside `falsification_gate` and
  `cross_examination` as a V1 shape, which reads as standalone.

**Build phase resolution:** modifier only, per the RFC's catalog row
("`scribe` participant modifier") and the two designs that agree. A
standalone `--shape scribe` is not in V1. The participant modifier surfaces
as an `include_scribe: true` generator option on the two shipping shapes,
producing a `scribe_note` job whose role prompt forbids hypothesizing and
emits `progress_note`-shaped turns reading the `dialogue` trajectory.

### 4.5 Substrate friction — claude_code lane attestation flap

`claude_code/§"Substrate friction note"` documents that the claude_code
design lane flipped to `unattested` (`agent_mcp_discovery_stall`) ~43s into
its claim because no Go code path writes
`sessions.last_tools_list_at`. **This is not a contradiction with the other
designs; it is a substrate bug discovered during the design phase.**

**Build phase resolution:** out of RFC 0093 V1 scope but **flag for
immediate operator follow-up**. The build phase should not block on this;
RFC 0093 V1 ships and the attestation gap is filed as a separate issue.
Suggested fix from `claude_code/§"Substrate friction note"` option (b) —
"gate the discovery-stall classifier on a signal that *is* written
(fall back to `LastMCPRequestAt`) when the column is null and the session
has produced other MCP activity" — is the cheaper path; option (a)
(wire the MCP transport's `tools/list` handler to update the column) is
the cleaner one. Either is non-RFC-0093 work.

## 5. Smallest implementable scope (the first PR)

A single implementer can land **Slice 1 — the artifact contract** as
**one self-contained, independently-green, independently-revertible PR**
that does not require the cycle router, the gate wiring, or the generator.
This is the lowest-risk first land per all three rollout sketches.

### 5.1 Files touched

- `go/pkg/artifactcontracts/contracts.go` — add `collaboration_ledger` to
  `allowedKinds`; register the `Schema` entry with required fields
  (`schema_version` = `striatum.collaboration_ledger.v1`, `artifact_kind` =
  `collaboration_ledger`, `shape` ∈ `{falsification_gate, cross_examination,
  fog_of_war_review, synaptic_prune}`, `topic`, `verdict` ∈ the standard
  four, `rationale`, `participants` non-empty string list) and parse the
  `entries[]` body shape (each entry has exactly `kind` ∈ `{claim, challenge,
  rebuttal, constraint, nomination}`, `by` ∈ `participants`, `refs[]`
  non-empty `dialogue:<seq>`-shaped strings, `text` non-empty string;
  reject unknown fields).
- `go/pkg/artifactcontracts/contracts.go` — add the kind-specific clearing
  substance check: if `verdict` is `accept` or `accept_with_findings`,
  `entries[]` must contain ≥1 `claim`, ≥1 `challenge`, and ≥1 `rebuttal`,
  each with non-empty `refs[]`.
- `go/pkg/artifactcontracts/contracts_test.go` — eight unit tests:
  1. valid clearing ledger (all five `entries[].kind` values exercised).
  2. missing front matter → invalid.
  3. unknown front-matter field → invalid (D028 structural guard).
  4. invalid `entries[].kind` (e.g. `gossip`) → invalid.
  5. clearing verdict without challenge/rebuttal → invalid.
  6. `entries[].by` not in `participants` → invalid.
  7. invalid `verdict` value → invalid.
  8. malformed `entries[].refs` (not `dialogue:<seq>`-shaped) → invalid.
- `go/pkg/mutations/artifact_publish*.go` — route `collaboration_ledger`
  through the existing front-matter validator path (one-line addition to
  the kind-list mapping); exit 6 on invalid front matter for parity with
  every other front-matter-carrying kind.
- `go/pkg/mutations/review.go` (or wherever `review.submit` lives) — when
  the submitted artifact's `kind == collaboration_ledger`, the submitted
  `--verdict` must equal the artifact's front-matter `verdict`; on
  mismatch the submit refuses with a clear error.
- `go/pkg/mutations/review_test.go` — two tests:
  1. `review.submit --verdict accept` against a ledger whose front-matter
     `verdict: needs_revision` → refused.
  2. matching verdict → accepted.
- `docs/reference/ubiquitous-language.md` — add four entries:
  `collaboration shape`, `substance-gate`, `adjudicator`,
  `collaboration_ledger`.
- `docs/reference/spec.md` — record the new artifact kind in the artifact
  contract section (no shape catalog entries yet — those land with the
  generator).

### 5.2 Scope explicitly NOT in this PR

- The cycle router (Slice 2).
- The `adjudicator` role file (Slice 3).
- The generator + shape catalog (Slice 4).
- Any `examples/` fixture (those land with Slice 3 and Slice 4).
- Any RFC 0034 catalog entry tagged `shape_family: "collaboration"`.
- RFC 0083 re-expression as a catalog entry (Slice 4).
- Any docs changes to `workflow-types.md` beyond glossary entries (Slice 4
  brings the shape rows).

### 5.3 Acceptance gates for Slice 1

- `make -C go check`, `make test`, `make lint`, `make typecheck` all green.
- All eight artifact-contract tests pass; both review-submit tests pass.
- `striatum publish-artifact` on a malformed ledger fixture exits 6.
- `striatum publish-artifact` on a valid clearing ledger fixture exits 0.
- Glossary entries reviewable in isolation; no shape catalog claim made.

### 5.4 Sequence after Slice 1

- **Slice 2:** cycle router in `go/pkg/mutations/review.go` per
  `codex/§2`. Three tests: cycle requeue on first `needs_revision`,
  human-checkpoint on `max_iterations` exhaustion, accept leaves cycle
  path untouched. Reset only the cycle slice; cancel stale claimable
  messages. Single PR, independently green.
- **Slice 3:** `adjudicator` role file + hand-authored fixture exercising
  the gate end-to-end against the in-memory scheduler. Anti-theater fixture
  (hollow vs landed vs accept-bypass-attempt). Reviewer-independence test.
  Single PR.
- **Slice 4:** `workflow generate --shape falsification_gate` and
  `--shape cross_examination` produce v1.1 graphs that pass
  `workflow validate` / `workflow lint`. `include_scribe` modifier.
  `examples/collaboration-falsification-gate/` and
  `examples/collaboration-cross-examination/` fixtures.
  `docs/reference/workflow-types.md` + `docs/reference/spec.md` shape list
  + RFC 0083 catalog re-expression. Single PR.
- **Decision log entry:** one decision per landed shape + one for the
  adjudicator-as-`phase_synthesis` choice (records the rationale from
  §1.1 above so future agents do not relitigate it).
- **RFC status flip:** `proposed → accepted` after Slice 4 lands green.

## 6. What the panel may interrogate me on

For my own working memory across the interrogation that follows this
publish, the choices most likely to be probed and the reasoning behind
each:

1. **Why split the rubric instead of pure Go or pure prompt?** Pure Go
   either over-strict (the false-positive class `claude_code/§4.9` flags)
   or under-expressive (cannot judge whether a challenge is *material*).
   Pure prompt is uncheckable in CI without a live model. Split is the
   only configuration that makes the bar testable while preserving
   semantic judgment.
2. **Why `phase_synthesis` over a `review` job for the gate?** A `review`
   job operates over an artifact; the adjudicator operates over a
   *trajectory*, which has no single backing artifact. Forcing a
   synthetic transcript artifact would either re-introduce raw-transcript
   capture (D028 violation) or add a zero-value publish job. (Adopted
   from `claude_code/§3.3`.)
3. **Why is the cycle router Slice 2 and not Slice 3?** Without the
   router, `needs_revision` opens a human checkpoint instead of cycling,
   so the gate's acceptance criterion 2 ("`needs_revision` routes back
   into a bounded dialog round") is unsatisfiable at the runtime layer.
   The router must land before any gate test can prove the loopback.
4. **Why no ANSI scanner for D028?** The structural "unknown fields
   rejected" guard at the contract layer is exhaustive — there is no
   `transcript` / `raw_output` / `pty_log` field defined. The bytewise
   scanner adds false-positive risk on legitimate authored text (curated
   summaries may legitimately mention `\r\n` or escape codes in code
   spans). Defense-in-depth at the publish layer is a follow-up if a
   leak appears; not a V1 dependency.
5. **Why defer `fog_of_war_review` and `synaptic_prune`?** Both require
   surface beyond pure composition: `fog_of_war_review` needs work-packet
   *type sequencing* in the generator; `synaptic_prune` needs the
   `post_dialog_hook` conversation-fixture field to dodge the liveness
   race. The RFC explicitly authorizes the deferral and the TASK
   constraints permit it.
6. **Why does `agy` not seat as holder/falsifier in the bar fixture?**
   The agy owned-PTY agent-loop path landed today (#52, #55) and is the
   newest of the three. V1 should be calibrated on the
   known-good claude+codex pair first; agy participates as the third
   design/review seat (this run), with promotion to the runtime fixture
   roster after Slice 4 dogfoods green.
7. **Why does the modifier-only `scribe` not delay the two required
   generated shapes?** It is a single conversation-participant flag
   (`include_scribe: true`) that adds one `scribe_note` job to the
   dialogue phase, with no gate authority and no impact on the
   collaboration_ledger. It composes for free with both shipping shapes
   and can land in Slice 4 without slowing the gate-bearing surfaces.
8. **Why cycle-budget-exhausted opens a human checkpoint instead of
   failing terminally?** The human checkpoint is the existing operator
   surface and preserves operator agency. A new terminal run state would
   add a recovery surface with no operator path. If dogfood surfaces
   operators who want hard failure, that is a V2 conversation.
