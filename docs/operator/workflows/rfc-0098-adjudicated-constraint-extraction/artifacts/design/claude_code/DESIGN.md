# RFC 0098 Implementation Design — claude_code lane

author: designer-claude-opus-4.8-001

Independent design for landing RFC 0098 (Adjudicated Constraint-Extraction
Loop). This lane was asked to prioritize **slice 1**:
`collaboration_ledger.v1.1` additive schema + the productive-refusal gate, with
no new daemon method. Slices 2–3 are sketched as stretch. The design is grounded
in the code as it stands today (`go/pkg/artifactcontracts/contracts.go`,
`go/pkg/mutations/`), not in the RFC's illustrative YAML.

---

## 1. Problem, in my own words

RFC 0093 already made refusal **honest**: the `adjudicator` role reads the
curated `dialogue` trajectory and emits a `collaboration_ledger` with a typed
verdict; a hollow exchange scores `needs_revision` and the downstream commit
stays withheld. The contract enforces that *clearing* verdicts
(`accept` / `accept_with_findings`) are not free — `validateCollaborationLedger`
in `contracts.go:550` requires at least one `claim`, one `challenge`, and one
`rebuttal` entry before it lets a clear through.

What it does **not** do is constrain a *refusal*. A bare `needs_revision`
verdict is currently the weakest possible output:

1. **Refusal loses the work.** The load-bearing objections live only as prose in
   review artifacts. The next revision re-reads the prior artifact and "addresses
   feedback" — averaging the criticism back into a blander synthesis. There is no
   discrete, trackable obligation that survives the cycle boundary. The existing
   test `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`
   (`contracts_test.go:204`) *proves* this: today a `needs_revision` ledger with
   one unrebutted challenge and **zero extracted constraints** validates happily.
2. **Closure is unverifiable.** "Final review" re-litigates the design from
   scratch. Nothing checks that each objection the adjudicator found load-bearing
   actually landed in the final spec as testable text or a gate.

The Engram entity-relationship forum run (#89) closed both gaps *by hand*: the
adjudicator extracted a numbered constraint table, the convener discharged each
row, and the final reviewer typechecked discharge instead of relitigating. That
discipline produced materially better RFC content. **Productive refusal** is the
promotion of that discipline into the contract: a `needs_revision` adjudication
must *compile its objections into binding constraints* (or explicit
unresolved-question rows), the revision must discharge each, and final review
must verify discharge without re-running the forum.

Two real bugs the same run filed are in scope for slice 1 because they block the
shape on its own substrate:

- **#88** — the prompt told the adjudicator a "clearing verdict" was allowed; it
  wrote `verdict: clear`; the contract's enum
  (`accept|accept_with_findings|needs_revision|reject`, `contracts.go:237`)
  rejected it, and the error didn't name the allowed values. Prompt and contract
  disagree, and the rejection is opaque.
- **#79** — the adjudicator wrote a *natural* ledger (`entries` shaped to taste,
  richer metadata) and got `field "entries" is invalid` with no hint why. The
  entry validator (`isCollaborationLedgerEntriesValue`, `contracts.go:674`) is
  strict to the point of opacity: exactly the four keys `{kind,by,refs,text}`,
  `refs` must match `dialogue:<seq>`, anything else is a flat reject.

---

## 2. The load-bearing insight: one validator, three gate paths, zero new methods

The reason slice 1 can be "pure contract/validation work, no daemon method" is
that **every** write path that could admit a `collaboration_ledger` already
funnels through the same contract function. I traced all three:

| Path | Entry point | Calls | Lands in |
|---|---|---|---|
| `publish-artifact` | `artifact.go:93` `validateArtifactFrontMatter` | `ValidateFrontMatter` | `validateCollaborationLedger` (`contracts.go:550`) |
| `submit-review` precheck | `review.go:603` `prevalidateSubmitReviewArtifactVerdict` → `:623` | `ParseAndValidateFrontMatter` | same |
| primitive `review.verdict` | `review.go:424` → `enforceCollaborationLedgerVerdict` (`collaboration_ledger.go:122`) | `ParseAndValidateFrontMatter` | same |

`ParseAndValidateFrontMatter` itself calls `ValidateFrontMatter`
(`contracts.go:341`), which dispatches kind-specific cross-field checks through
`validateKindSpecific` → `validateCollaborationLedger` (`contracts.go:493`).

**Therefore the productive-refusal gate belongs in exactly one place:
`validateCollaborationLedger`.** Adding the `needs_revision ⇒ non-empty
constraints[]` rule there makes all three paths enforce it for free — including
the `enforceCollaborationLedgerVerdict` primitive-path that build finding #2 of
the RFC 0093 run had to close separately. No new RPC, no new route in the
command-authority matrix, no daemon-method registration. The exit code is
already correct: the contract returns `rpc.NewError("artifact_error", …)`, which
the CLI maps to exit 6 (the same code `collaboration_ledger` validation produces
today). We inherit it; we don't add a code.

This is the single most important architectural fact in the design and it is why
slice 1 is genuinely low-blast-radius.

---

## 3. Slice 1 — `collaboration_ledger.v1.1` + productive-refusal gate

### 3.1 Where the schema lives and how it stays additive

The schema is the `"collaboration_ledger"` entry in the `Schemas` map
(`contracts.go:229-240`). The additivity hazard is concrete and **already
locked by tests**: `ValidateFrontMatter` rejects *any* field not declared in
`schema.Fields` with an `unknown fields` error (`contracts.go:313-322`), and
that strictness is asserted by `TestLedgerFrontMatterRejectsUnknownFields`
(`artifact_contract_migration_test.go:66`) and
`TestCollaborationLedgerRejectsUnknownTopLevelField` (`contracts_test.go:140`).

So "additive" has a precise meaning here: **register the new fields as
`Required: false` entries in the same `Schemas["collaboration_ledger"]` map, with
validators that no-op when the field is absent.** A bare RFC 0093 v1 ledger
carries none of them, so it still validates unchanged — satisfying AC 7 (and the
existing strict tests keep passing because the new keys become *known* optional
fields, not new required ones).

Concretely, slice 1 adds these optional top-level fields:

```
collaboration_ledger schema (v1.1 additions, all Required:false):
  schema_version : oneOf("striatum.collaboration_ledger.v1",
                         "striatum.collaboration_ledger.v1.1")   # widened, additive
  shape          : add "adjudicated_constraint_extraction" to the enum
  verdict        : add "blocked_pending_answer", "defer_with_successor"
  version        : optional string ("1.1")           # mirrors RFC YAML, advisory
  cycle          : optional non-negative int          # cycle-aware, see #84
  constraints    : optional list<constraint-row>      # NEW (gated, see 3.2)
  branches       : optional map<posture,disposition>  # NEW posture matrix
  findings       : optional list<finding-row>         # NEW, gives #79 a home
```

`constraints[]` rows validate structurally (present, typed, sourced,
dispositioned — never semantically, per Non-Goal): `id` non-empty string;
`posture` ∈ the five-posture set; `severity` ∈ info|low|medium|high|critical;
`kind` ∈ invariant|gate|schema|policy|non_goal|accepted_risk; `binding` bool;
`text` non-empty; `source_finding` references a `findings[]` id (or is allowed
empty for an unresolved-question row); optional `verification.{expected_stage,
gate}` and `final_review_required` bool.

`branches{}` is `map<posture-string, oneOf(cleared|cleared_with_constraints|
blocked)>`.

### 3.2 The gate (the four-line heart of slice 1)

Inside `validateCollaborationLedger`, after the existing clearing-verdict block,
add the productive-refusal rule — **scoped to v1.1 ledgers only**:

```
if isV11Ledger(parsed) && verdict == "needs_revision" {
    if len(constraintRows(parsed["constraints"])) == 0 {
        return error("needs_revision verdict requires a non-empty constraints[]
                      table (binding constraints or explicit
                      unresolved-question rows)")
    }
}
```

`isV11Ledger` = `schema_version == "…v1.1"` **OR**
`shape == "adjudicated_constraint_extraction"`. This opt-in scoping is the design
decision that reconciles AC 2 (reject naked `needs_revision`) with AC 7 (every
v1 ledger still validates). It is **not** cosmetic:

- A bare RFC 0093 `falsification_gate` ledger with `verdict: needs_revision` and
  no constraints **must keep validating** — that is exactly
  `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`. If we gated
  unconditionally we'd break that test *and* AC 7. Scoping to v1.1 keeps it green
  with no edit, because its fixture is `shape: falsification_gate`,
  `schema_version: …v1`.
- The new behavior is proven by a *new* test: a `shape:
  adjudicated_constraint_extraction` / `…v1.1` ledger with `verdict:
  needs_revision` and empty `constraints[]` is rejected (exit 6); the same ledger
  with ≥1 binding constraint **or** ≥1 unresolved-question row is accepted. That
  is the seeded-both-directions proof AC 2 asks for.

### 3.3 Closing #88 and #79 in the same slice

- **#88 (clearing verbs).** The contract is the authority; the *prompt* must name
  the exact enum, and the *error* must list it. Two moves: (a) extend the
  `verdict` enum with `blocked_pending_answer` and `defer_with_successor`
  (RFC §4) so the adjudicator's natural clearing/holding vocabulary is accepted;
  (b) when `oneOfValue` rejects `verdict`, return an error that *enumerates the
  allowed values* instead of the generic `field "verdict" is invalid`. The RFC is
  explicit that ambiguous bare `clear` stays **disallowed** — so we do not add
  `clear`; we fix the prompt (slice-2 shape prompts) to say
  `accept_with_findings`, and we make the rejection self-explanatory. Advertise
  exactly what we enforce.
- **#79 (natural front matter).** Root cause is that the only structured home is
  `entries[]`, whose validator demands *exactly* `{kind,by,refs,text}` with
  `dialogue:<seq>` refs and rejects everything else opaquely. Rather than loosen
  `entries[]` (which would weaken the D028/provenance guarantee that refs point
  into the trajectory), give the rich adjudicator metadata its **own** optional
  home: the new `findings[]` and `constraints[]` blocks (3.1). The cross-exam
  finding rows the RFC shows (`id/severity/posture/status/challenge/…`) live in
  `findings[]`; the constraint table lives in `constraints[]`; `entries[]` stays
  the trajectory-anchored evidence rows it already is. Then improve the
  `entries`-invalid error to name the offending entry index and the allowed key
  set, so a future mismatch self-documents instead of forcing a source dive. This
  makes the "natural ledger" the adjudicator wanted *expressible* without
  weakening any existing invariant.

### 3.4 D028 / provenance guard stays intact

The RFC requires a D028 no-stdout guard over the new fields. The new
`constraints[].text`, `findings[].challenge`, etc. are authored adjudicator
prose referencing trajectory turns — same posture as `entries[].text`. The guard
is structural: the new validators accept only the typed fields enumerated above
and reject unknown keys (inherited from the strict-field machinery), so a
`stdout:`-style raw-stream field cannot ride in on a constraint row any more than
it can on a top-level field. A test asserts a `constraints[]` row carrying a raw
provider-output field is rejected.

---

## 4. Slices 2 and 3 (stretch) — sketch, with their real blockers

### Slice 2 — shape fixture + generator (`adjudicated_constraint_extraction`)

Register the shape in the collaboration shape pack
(`go/pkg/workflowgenerate`, `go/pkg/workflowtemplates/catalog.json`) so
`workflow generate --shape adjudicated_constraint_extraction` emits the 8-phase
graph (survey → convener_synthesis → cross_exam → adjudication →
revision_synthesis → constraint_discharge_review → spec_publication →
final_review), plus a starter fixture under
`examples/adjudicated-constraint-extraction-flow/`.

**Good news the RFC under-credits: #84's runtime half already exists.** The
`${cycle}` placeholder substitution (`collaboration_ledger.go:25-74`,
`cycleSegmentForAttempt`/`resolveExpectedArtifactCycles`) is already wired into
every consumer of `expected_artifacts_json`: work-packet build (`claim.go:220`),
required-artifact verification (`mutations.go:316`), the submit-review precheck
(`review.go:569`), and verdict enforcement (`collaboration_ledger.go:91`). So a
revised synthesis republished under `..._synthesis_${cycle}` already resolves to
a distinct `cycle_<attempt>` logical name and dodges the content-hash collision
that deadlocks revision loops. **What slice 2 must add is the generator emitting
those `${cycle}` placeholders** in the cycle-scoped phases' expected artifacts —
not the runtime machinery. That materially de-risks the RFC's "defer slice 2 if
#84 unresolved" boundary: the dependency is mostly met.

**The real slice-2 blocker is #66, not #84.** `run.prepare` rejects an edge that
targets a *later* phase's `phase_synthesis` job, but the
`adjudication → revision_synthesis` constraint edge is a same-cycle back-edge
into a synthesis phase. Until that rule is expressed legally (or relaxed for
same-cycle revision), `workflow validate` will pass a graph that `run.prepare`
rejects — which *is* #66. **Recommendation: gate slice 2 on a resolved/decided
#66**, and defer cleanly if the running daemon still rejects the edge, recording
it in the build handoff. Slice 1 has no such dependency.

### Slice 3 — discharge-verifying `final_review`

Add the `final_review` job type/prompt and the `constraint_discharge` finding
shape (a `finding`/`findings_ledger`, *not* a new kind — reuse). The phase
**fails closed** if any `binding: true && final_review_required: true` constraint
is `missing` or unaccepted-`partial`, and passes when every such constraint is
`discharged` or `accepted_risk` with owner/stage — *without* re-running prior
phases. Mechanically this is another structural validator: read the latest
clearing ledger's `constraints[]`, read the final-review finding's
`constraint_discharge[]` table, and verify coverage. It composes on slice 1's
schema and needs no daemon method either; its main dependency is slice 2 having
produced a real ledger to typecheck against, so it lands last.

---

## 5. Alternatives considered

**A. `v1.1`-additive vs a new `v2` schema family.** *Chosen: v1.1, additive.* A
`v2` would mean a parallel `Schemas["collaboration_ledger_v2"]` entry, a second
kind in `allowedKinds`, fork the three gate paths, and migrate fixtures — large
blast radius for fields that are genuinely optional add-ons. Because the strict
unknown-field machinery already exists, "additive" is cheap: declare the new
keys `Required:false`. Every v1 ledger validates unmodified (AC 7), and the gate
is scoped by an opt-in (`…v1.1`/new shape), so v1 semantics are untouched. `v2`
would only be justified if `binding`/`final_review_required` had to change the
*meaning* of an existing field; they don't — they're new fields. The RFC's Open
Question 1 lands the same way.

**B. Artifact-only vs first-class `constraint.*` objects.** *Chosen:
artifact-only for V1; `constraint.*` explicitly deferred (RFC §6).* First-class
durable constraint objects (`constraint.record/list/discharge/verify`) would be
a new RPC family and a new aggregate — exactly the "new daemon authority" the
Non-Goals forbid for V1, and unjustified until a *second* workflow needs to read
constraints across runs. V1 derives everything from the
`collaboration_ledger.v1.1` artifact the run already persists. Final review
re-parses the ledger; that is acceptable for one consumer.

**C. Gate the refusal in the mutation layer vs in the contract.** *Chosen:
contract (`validateCollaborationLedger`).* Putting the rule in
`go/pkg/mutations` would mean re-implementing it (or calling it) at each of the
three paths in §2 — and history shows that's how the primitive-path bypass
(build finding #2) happened in the first place. One contract function is the only
place that is *unbypassable by construction*, and it keeps slice 1 free of any
daemon-method or route change.

**D. Overload `entries[].kind: constraint` vs a dedicated `constraints[]`
block.** *Chosen: dedicated block.* `entries[]` is intentionally
trajectory-anchored (every row needs `dialogue:<seq>` refs and the strict 4-key
shape — that's the provenance guarantee). A binding constraint has a different
shape (id, posture, severity, binding, verification, disposition) and is not a
single dialogue turn. Forcing it into `entries[]` is exactly the impedance
mismatch that produced #79. A separate optional block models it honestly and
leaves `entries[]` semantics intact.

---

## 6. Risks, unknowns, and what could go wrong

- **Additivity regression (highest-value risk).** If any new field is registered
  `Required:true`, or the gate is applied unconditionally rather than scoped to
  v1.1, every existing v1 ledger and the RFC 0093 fixtures break (AC 7 fails).
  *Mitigation:* all new fields `Required:false`; gate scoped via `isV11Ledger`;
  keep `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`
  unmodified as the canary — if it goes red, scoping is wrong. Add the additive
  guarantee as an explicit test: a verbatim RFC 0093 v1 ledger validates after
  the change.
- **The RFC's YAML is nested; the contract is flat.** RFC §4 shows
  `collaboration_ledger:` as a nested mapping with `version:`. The real contract
  is *flat* front matter (`schema_version`, `shape`, `topic`, `entries`, …). The
  implementer must map onto the flat form (use `schema_version` as the version
  axis, add a flat `constraints:`/`branches:`), not introduce a nested mapping —
  doing the latter would itself trip the unknown-field guard. Called out so the
  build lane doesn't transcribe the RFC literally.
- **#66 cross-phase edge (slice 2 only).** `adjudication → revision_synthesis` is
  a back-edge into a synthesis phase that `run.prepare` may reject while
  `workflow validate` accepts. *Mitigation:* slice 2 is gated on #66; defer
  cleanly and record it if the running daemon rejects the edge. Slice 1 is
  unaffected.
- **#84 republish (slice 2).** Lower risk than the RFC implies — the runtime
  substitution is already wired (see §4). Residual risk is only whether the
  generator emits `${cycle}` for *all* cycle-scoped phases (convener_synthesis,
  revision_synthesis, the per-cycle ledgers); miss one and that phase collides on
  content hash across cycles. *Mitigation:* fixture test that runs two cycles and
  asserts distinct `cycle_1`/`cycle_2` logical names per re-opened phase.
- **#88 prompt/contract drift could recur.** Adding verdicts to the enum without
  updating the shape prompts re-creates the exact mismatch. *Mitigation:* land
  the enum change and the prompt wording in the same slice; the error message
  must enumerate allowed verdicts so any future drift is self-diagnosing.
- **Unresolved-question rows vs binding constraints.** AC 2 accepts a
  `needs_revision` ledger with "≥1 binding constraint *or* unresolved-question
  row." The gate must treat an explicit unresolved-question row (e.g.
  `kind: non_goal`/an `open_question` row) as satisfying non-emptiness, or it
  will over-reject honest "we can't resolve this yet" refusals. *Mitigation:* the
  emptiness check counts any constraint row, and the row schema admits an
  unresolved-question shape; a test seeds that direction.
- **#77 assumption (adjudicator absorbs the cycle).** The shape wants
  `needs_revision` to route straight to `revision_synthesis`, not spawn a
  checkpoint. Slice 1 doesn't depend on this (it's contract-only), but slice 2's
  graph does. Flagged as a slice-2 precondition to confirm in the running daemon.
- **Adjudicator reliability (inherited from RFC 0093 OQ2).** The gate validates
  that constraints are present/typed/sourced/dispositioned, never that they are
  *good*. A lazy adjudicator can emit one trivial constraint to clear the gate.
  This is an accepted limitation (Non-Goal: no semantic scoring); the mitigation
  is the same as RFC 0093's — the adjudicator is reviewer-independent and itself
  interrogable, and §7 coverage metrics make low-substance refusals visible.

---

## 7. Rollout sketch

1. **First (slice 1, the V1 target).** In `contracts.go`: widen
   `schema_version` and `shape`/`verdict` enums; register `version`, `cycle`,
   `constraints`, `branches`, `findings` as optional fields with structural
   validators; add the v1.1-scoped `needs_revision ⇒ non-empty constraints[]`
   rule to `validateCollaborationLedger`; enumerate allowed verdicts in the
   rejection error. Tests in `contracts_test.go` +
   `go/pkg/mutations/*_test.go` (under live PG per RFC 0080 pgtest): additive v1
   still-valid; both productive-refusal directions; #88 verdict + clear error;
   #79 natural metadata accepted in `findings[]`/`constraints[]`; D028 no-stdout
   on the new fields; and the primitive-path (`enforceCollaborationLedgerVerdict`)
   inherits the gate. Verify: `make -C go check`, `make test`, the
   `./pkg/artifactcontracts/... ./pkg/mutations/...` pgtest line. **This ships on
   its own and closes #88/#79 for the shape.**
2. **Second (slice 2, gated on #66/#84/#77).** Register the shape + generator +
   `examples/adjudicated-constraint-extraction-flow/`; emit `${cycle}`
   placeholders; prove `workflow validate` **and** `run.prepare` both pass.
   Defer cleanly if #66 blocks the back-edge in the running daemon.
3. **Third (slice 3, on top of 2).** `final_review` + `constraint_discharge`
   typecheck that fails closed on undischarged binding constraints without
   re-running prior phases.
4. **Deferred (slice 4).** First-class `constraint.*` objects + coverage metrics
   — only when a second cross-run consumer exists.

A single implementer should land **slice 1 cleanly and stop there if the run
risks wedging** on the slice-2 generator dependencies; slices 2–3 are bonus.
That sequencing keeps the smallest blast radius first and never bets the run on
the #66 edge.
