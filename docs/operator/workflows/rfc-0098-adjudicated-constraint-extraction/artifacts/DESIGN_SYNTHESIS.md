---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0098 Design Synthesis — Adjudicated Constraint-Extraction Loop

author: synthesizer-claude-opus-4.8-001

Single buildable synthesis reconciling the three lane designs
(`design/claude_code/DESIGN.md`, `design/codex/DESIGN.md`, `design/agy/DESIGN.md`)
into one implementable plan for RFC 0098, sliced smallest-blast-radius first.
Every architectural claim below is checked against the code as it stands today
(`go/pkg/artifactcontracts/contracts.go`, `go/pkg/mutations/collaboration_ledger.go`,
`go/pkg/mutations/review.go`), not against the RFC's illustrative YAML.

---

## 0. Decision in one paragraph

**Build slice 1 only as the V1 target: an additive `collaboration_ledger.v1.1`
schema (`constraints[]` + `branches{}` + `cycle`) plus a v1.1-scoped
productive-refusal gate, landed entirely inside the one contract function
`validateCollaborationLedger`, with zero new daemon methods.** The spine is
**claude_code's** design — it is the only lane grounded in the actual contract
code and it correctly identifies the single load-bearing architectural fact
(one validator, three gate paths). I adopt **two corrections from codex** that
are build-blocking and that claude_code and agy both got wrong: (a) **do not
widen the front-matter `verdict` enum** — the two refined states
(`blocked_pending_answer`, `defer_with_successor`) become `branches{}`
dispositions, because the front-matter verdict is coupled to the daemon verdict
state machine and widening it wedges the run; (b) **`posture` is a non-empty
string, not a closed enum**, because RFC 0098 §2 lets a workflow declare its own
posture set. I **reject agy's literal Go** (it makes `schema_version` require
`v1.1` and gates `needs_revision` unconditionally — both break additivity and the
existing canary test), while keeping agy's clear framing of "productive refusal
as a constraint-generating event." Slices 2 and 3 are **deferred at a recorded
boundary**: slice 2 is gated on the live daemon's behaviour for #66 (the
`adjudication → revision_synthesis` back-edge) and slice 3 depends on slice 2.

---

## 1. The load-bearing fact (carried from claude_code, verified)

Every write path that can admit a `collaboration_ledger` funnels through the
same contract function, so slice 1 is genuinely "pure contract/validation work,
no daemon method." I re-traced all three paths in source:

| Path | Entry | Reaches |
|---|---|---|
| `publish-artifact` | `validateArtifactFrontMatter` → `ValidateFrontMatter` | `validateCollaborationLedger` (`contracts.go:550`) |
| `submit-review` precheck | `prevalidateSubmitReviewArtifactVerdict` → `ParseAndValidateFrontMatter` | same |
| primitive `review.verdict` | `recordVerdict` → `enforceCollaborationLedgerVerdict` (`collaboration_ledger.go:89`) → `ParseAndValidateFrontMatter` | same |

`ValidateFrontMatter` (`contracts.go:285`) dispatches kind-specific cross-field
checks through `validateKindSpecific` → `validateCollaborationLedger`
(`contracts.go:493`). **Therefore the productive-refusal gate belongs in exactly
one place — `validateCollaborationLedger` — and all three paths enforce it for
free,** including the primitive `review.verdict` path that the RFC 0093 build had
to close separately (build finding #2). The error is already
`rpc.NewError("artifact_error", …)`, which the CLI maps to **exit 6** — we
inherit the code, we do not add one. No new RPC, no command-authority-matrix
route, no guardrail-test change. **All three lanes independently reached this
"contract is the only unbypassable point" conclusion** (claude_code §2, codex
"hook path", agy §3); it is the shared, correct foundation.

---

## 2. The decisive contradiction: the `verdict` enum (codex over claude_code + agy)

This is the one disagreement with real build consequences, and the lanes split.

- **claude_code §3.3** and **agy §3** both propose *widening the front-matter
  `verdict` enum* to add `blocked_pending_answer` and `defer_with_successor`
  (agy's snippet literally rewrites the `verdict` field to
  `oneOfValue("accept", … , "blocked_pending_answer", "defer_with_successor")`).
- **codex** explicitly refuses: "I would keep the runtime verdict vocabulary
  unchanged for slice 1 … Widening `recordVerdict` to new top-level verdicts is
  not 'pure contract/validation work'; it changes gate semantics." codex routes
  the two refined states to `branches{}` dispositions instead.

**codex is right, and the code proves it.** `enforceCollaborationLedgerVerdict`
(`collaboration_ledger.go:126-129`) requires the *recorded* review verdict to
**equal** the ledger's front-matter `verdict`:

```
if ledgerVerdict != recordedVerdict {
    return rpc.NewError("artifact_error",
        "recorded verdict %q must match collaboration_ledger front matter verdict %q", …)
}
```

That recorded verdict flows into `recordVerdict`'s switch (`review.go:478-531`),
which routes only `accept` / `accept_with_findings` (complete + enqueue
downstream), `needs_revision` (route revision cycle / human checkpoint), and
`reject` (fail). **Anything else hits `default → rpc.NewError("invalid_transition",
"unknown verdict")` (`review.go:529-530`).** So if an adjudicator writes
`verdict: blocked_pending_answer` in a v1.1 ledger, the only verdict that
satisfies `enforceCollaborationLedgerVerdict` is `blocked_pending_answer`, and
recording it throws `invalid_transition` — **the adjudication job wedges with no
legal way to clear.** This is exactly the lifecycle incoherence the project has
been burned by before, and it is silent at contract-test time (the contract would
accept the ledger; the daemon rejects the verdict).

**Resolution for the build:** the front-matter `verdict` enum stays exactly the
four daemon-routable values — `accept`, `accept_with_findings`, `needs_revision`,
`reject`. The two RFC 0098 "refinements" live as **`branches{}` dispositions**
(`blocked_pending_answer`, `defer_with_successor`), which are pure ledger metadata
the daemon never routes on. Promoting them to first-class verdicts is a
*daemon-verdict-state-machine* change (widen the `recordVerdict` switch + cycle
routing + state tests) and belongs to a later slice with explicit state-machine
tests — never to "additive contract" slice 1. RFC §4's verdict list and Open
Question 2 (#77 absorption) are satisfied at the **disposition** layer for V1.

---

## 3. The additivity contradiction: opt-in scoping (claude_code + codex over agy)

All three lanes agree the schema must stay additive so every RFC 0093 `v1` ledger
still validates (AC 7). But **agy's literal Go breaks it**: its snippet sets
`"schema_version": {true, equalsValue("striatum.collaboration_ledger.v1.1")}`
(rejects every existing `v1` ledger) and gates `if verdict == "needs_revision"`
**unconditionally** (rejects the existing canary
`TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`,
`contracts_test.go:204`, which is a `v1`/`falsification_gate` `needs_revision`
ledger with zero constraints). agy's *prose* says "relax `schema_version` to
accept both" and "only validated structurally if present" — so the intent is
right, but the build must follow **claude_code's precise scoping, not agy's
code.**

**Resolution (claude_code §3.2, confirmed against the canary):**

- Widen `schema_version` from `equalsValue("…v1")` to
  `oneOfValue("striatum.collaboration_ledger.v1", "striatum.collaboration_ledger.v1.1")`.
- Register all new fields `Required: false`.
- Scope the gate: it fires **only** for a v1.1 ledger, where v1.1 means
  `schema_version == "…v1.1"` **OR** `shape == "adjudicated_constraint_extraction"`
  (belt-and-suspenders, so a shape-tagged ledger that forgot to bump the version
  still gets gated).
- Keep `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`
  **unmodified as the canary**: it is `v1` + `falsification_gate`, so it is not
  gated and stays green. If it ever goes red, the scoping is wrong.

I verified the canary's fixture (`validCollaborationLedger("needs_revision")`,
`contracts_test.go:210-232`): `schema_version: striatum.collaboration_ledger.v1`,
`shape: falsification_gate`, `verdict: needs_revision`, no constraints. Opt-in
scoping leaves it valid by construction.

---

## 4. Slice 1 — concrete, buildable spec (the V1 target)

All edits live in `go/pkg/artifactcontracts/contracts.go` + tests in
`contracts_test.go` and `go/pkg/mutations/*_test.go` (live PG, RFC 0080 pgtest).

### 4.1 Schema changes to `Schemas["collaboration_ledger"]` (`contracts.go:229`)

```
schema_version : oneOfValue("striatum.collaboration_ledger.v1",
                            "striatum.collaboration_ledger.v1.1")   # widened, additive
shape          : add "adjudicated_constraint_extraction" to the existing oneOfValue
verdict        : UNCHANGED  → accept | accept_with_findings | needs_revision | reject   (§2)
# --- new optional v1.1 fields, all Required:false ---
cycle          : optional non-negative int            (advisory; #84 substitution keys on attempt, not this field)
constraints    : optional list<constraint-row>        (NEW — the productive-refusal table)
branches       : optional map<posture-string → disposition>   (NEW — posture-disposition matrix)
findings       : optional list<finding-row>           (NEW — typed home for the cross-exam rows that broke #79)
```

The flat front matter is the real contract; **do not transcribe the RFC's nested
`collaboration_ledger:` mapping** (claude_code's risk note, confirmed: a nested
mapping would itself trip the unknown-field guard at `contracts.go:313-322`).
`schema_version` is the version axis — **no separate `version` field** (drop
claude_code's optional advisory `version`; it is redundant with `schema_version`
and adds a second source of truth).

The front-matter parser (`ParseFrontMatterBlock` → `parseYAMLNode`,
`contracts.go:361/394`) is `yaml.v3` and already recurses through `MappingNode`
and `SequenceNode`, so nested `branches{}` and lists-of-maps `constraints[]` /
`findings[]` parse natively. New validators follow the existing
`collaborationLedgerEntryList` list-of-maps pattern (`contracts.go:711`).

### 4.2 `constraints[]` row validator (`isConstraintListValue`, structural only)

Per Non-Goal "no semantic scoring" — validate *present, typed, sourced,
dispositioned*, never *good*:

- `id` — non-empty string (required in a row)
- `posture` — **non-empty string** (NOT a closed enum — see §5; RFC 0098 §2
  allows workflow-authored postures)
- `severity` — `low | medium | high | critical`
- `kind` — `invariant | gate | schema | policy | non_goal | accepted_risk | unresolved_question`
- `binding` — bool
- `text` — non-empty string
- `source_finding` — optional string (a `findings[]` id by convention; **not**
  required to resolve, because findings may live in a sibling `findings_ledger`)
- `source_refs` — optional list of `dialogue:<seq>` refs (reuse `isDialogueRef`)
- `verification` — optional map `{expected_stage?, gate?}`
- `final_review_required` — optional bool
- **Unknown keys in a row are rejected** (D028 no-stdout guard extends to rows —
  a `stdout:`-style raw field cannot ride in on a constraint).

### 4.3 The productive-refusal gate (heart of slice 1)

Inside `validateCollaborationLedger`, after the existing clearing-verdict block:

```
if isV11Ledger(parsed) && verdict == "needs_revision" {
    if countProductiveRows(parsed["constraints"]) == 0 {
        return error("needs_revision verdict on an adjudicated_constraint_extraction
                      ledger requires a non-empty constraints[] table (≥1 binding
                      constraint or ≥1 unresolved_question row); allowed verdicts: ...")
    }
}
```

- `isV11Ledger` = `schema_version == "…v1.1" || shape == "adjudicated_constraint_extraction"`.
- **A "productive row"** (codex's precise predicate) = `binding == true` **OR**
  `kind == "unresolved_question"`. This is the structural reading of AC 2's "≥1
  binding constraint **or** unresolved-question row" and prevents over-rejecting
  an honest "we cannot resolve this yet" refusal (claude_code's risk note).

### 4.4 `branches{}` validator (`isPostureDispositionMatrixValue`)

`map<posture-string → disposition>` where disposition ∈ `cleared`,
`cleared_with_constraints`, `blocked`, `blocked_pending_answer`,
`defer_with_successor`. **This is where the two RFC §4 refined states live**
(§2). Canonical form is the map (matches RFC §4 YAML). codex's richer array form
`[{posture, disposition, constraint_ids}]` is **deferred** unless it proves
trivially additive during the build — keeping slice 1 to one shape avoids scope
creep; the map already discharges AC 6's posture-matrix rendering need.

### 4.5 Closing #88 and #79 *for this shape*

- **#88 (advertised clearing verbs).** Root cause: the prompt advertised a verb
  the contract's enum rejected, with an opaque error. Fix = **advertise exactly
  what we enforce**: (a) the `verdict` enum stays the four daemon-routable values
  (§2), and the *shape prompts* (slice 2) must say `accept_with_findings`, never a
  bare `clear`; (b) when the `verdict` check fails, return an error that
  **enumerates the allowed verdicts** instead of the generic `field "verdict" is
  invalid`. We do **not** add `clear` (RFC §4: ambiguous bare `clear` stays
  disallowed). claude_code's enum-listing error message + codex's "don't widen the
  verdict" combine here — the #88 fix is *clarity + prompt/contract agreement*,
  not new verdicts.
- **#79 (natural front matter rejected).** Root cause (claude_code §3.3): the only
  structured home was `entries[]`, whose validator demands exactly
  `{kind,by,refs,text}` with `dialogue:<seq>` refs
  (`isCollaborationLedgerEntriesValue`, `contracts.go:674`) and flat-rejects
  everything else. Fix = give the adjudicator's rich metadata its **own typed
  homes** (`constraints[]`, `findings[]`, `branches{}`) rather than loosening
  `entries[]` (which would weaken the trajectory-provenance guarantee D028
  protects). Additionally improve the `entries`-invalid and unknown-fields errors
  to name the offending index/keys so a future mismatch self-documents. codex's
  "accept natural multiline/nested YAML through the existing parser + regression
  tests" is satisfied automatically because `yaml.v3` already parses those shapes
  (§4.1) — the regression tests are the deliverable, not a second parser.

### 4.6 Slice 1 definition of done (tests prove both directions)

1. **Additive canary:** a verbatim RFC 0093 `v1` ledger (incl. the existing
   `needs_revision`-with-unrebutted-challenge fixture) still validates — AC 7.
   `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge` stays
   green unmodified.
2. **Productive-refusal, both directions:** a `…v1.1` /
   `adjudicated_constraint_extraction` ledger with `verdict: needs_revision` and
   empty/absent `constraints[]` is **rejected (exit 6)**; the same ledger with ≥1
   `binding: true` row **or** ≥1 `kind: unresolved_question` row is **accepted** —
   AC 2.
3. **All three paths:** the gate fires on `publish-artifact`, `submit-review`, and
   the primitive `review.verdict` path (assert via a `go/pkg/mutations` pgtest
   that the primitive path rejects the naked refusal) — closes the build-finding-#2
   bypass class.
4. **#88:** a rejected-verdict error enumerates the allowed verdicts (regression
   fixture).
5. **#79:** a "natural" ledger carrying `constraints[]` / `findings[]` /
   `branches{}` with multiline/nested YAML validates; the improved error names the
   field on a genuine mismatch.
6. **D028 guard:** a `constraints[]` row carrying an unknown/raw-output key is
   rejected.

**Verification commands** (from TASK, reviewers re-run):

```sh
PATH="$HOME/go/bin:$PATH" make -C go check
make test && make lint && make typecheck
STRIATUM_PG_TEST_URL=postgres:///postgres go -C go test ./pkg/artifactcontracts/... ./pkg/mutations/...
```

---

## 5. Other reconciled contradictions

- **`posture` — closed enum vs free string.** claude_code §3.1 makes `posture` a
  closed five-value enum; **codex makes it a non-empty string**. **codex wins:**
  RFC 0098 §2 says the default posture set is "overridable per workflow," and a
  closed enum would reject a workflow that declares its own postures. The default
  five postures (product, implementation, privacy, eval, operations) are a
  **prompt/pack convention**, not a contract enum. Structural check = present +
  non-empty.
- **`kind` — explicit `unresolved_question`.** claude_code implied an
  unresolved-question row via `non_goal`/an `open_question` shape; **codex adds an
  explicit `unresolved_question` kind.** codex wins — it makes AC 2's "or
  unresolved-question row" structurally precise (the productive-row predicate is
  unambiguous) instead of overloading `non_goal`.
- **`severity` values.** claude_code includes `info`; codex uses
  `low|medium|high|critical`. Adopt **codex's four** — an `info`-severity *binding*
  constraint is incoherent, and the RFC's examples are all `high`.
- **`findings[]` in the ledger vs a sibling `findings_ledger`.** claude_code puts
  `findings[]` in the ledger (so `source_finding` is locally checkable); the RFC
  §4 allows either. Adopt **claude_code's in-ledger `findings[]`** as the home for
  #79's cross-exam rows, but keep `source_finding` an **optional, non-resolving**
  string (so a separate `findings_ledger` is still legal) — this is the codex-style
  "don't over-couple V1" guardrail applied to claude_code's structure.
- **#84 republish — already half-built.** claude_code §4 and codex both note the
  `${cycle}` substitution (`cycleSegmentForAttempt` / `resolveExpectedArtifactCycles`,
  `collaboration_ledger.go:27-74`) is already wired into claim, required-artifact
  verification, the submit-review precheck, and verdict enforcement. **Carried
  forward:** #84's runtime half exists; slice 2's job is only to make the
  *generator emit* `${cycle}` placeholders for cycle-scoped phases. The `cycle`
  field added in slice 1 is advisory metadata, not the substitution key.

---

## 6. Unresolved contradictions and how the build handles them

1. **Slice 2 back-edge (#66) — the real blocker, defer cleanly.** claude_code
   names #66 (not #84) as the slice-2 blocker: `run.prepare` rejects an edge into
   a *later* phase's `phase_synthesis`, but `adjudication → revision_synthesis` is
   exactly that. claude_code says "gate slice 2 on a resolved/decided #66 and
   defer if the running daemon rejects the edge." codex offers a **concrete
   workaround**: route `adjudication` to a non-synthesis *intake* job inside the
   `revision_synthesis` phase (e.g. `revision_constraints_intake`) that then feeds
   that phase's synthesis — legal without relaxing `run.prepare`. **Build
   directive:** attempt slice 2 only after slice 1 is green; first try codex's
   intake-job routing so the graph passes **both** `workflow validate` **and**
   `run.prepare` (AC 1). If the live daemon still rejects the edge and the intake
   indirection does not satisfy it, **defer slice 2 and record it in the handoff**
   — do not bet the run on relaxing `run.prepare`.
2. **#77 (adjudicator absorbs the cycle).** The shape wants `needs_revision` to
   route straight to `revision_synthesis`, not spawn a human checkpoint.
   `recordVerdict` already routes `needs_revision` to a matched workflow cycle and
   only opens a checkpoint when no cycle matches (`review.go:490-520`) — so a
   correctly-generated slice-2 graph gets absorption for free. **Slice 1 does not
   depend on #77** (contract-only). Confirm in the live daemon before slice 2.
3. **`branches` map vs array (codex).** Resolved to **map-only for V1** (§4.4);
   the array form is an optional later tolerance, not a slice-1 requirement.
4. **Adjudicator can emit one trivial constraint to clear the gate.** Inherited
   RFC 0093 OQ2 limitation; accepted (Non-Goal: no semantic scoring). Mitigation
   is unchanged: the adjudicator is reviewer-independent and itself interrogable,
   and §7 coverage metrics (observability, not a gate; out of slice 1) make
   low-substance refusals visible.

---

## 7. Scope: what to land next (smallest implementable unit)

**Land slice 1 and stop there if the run risks wedging.** Slice 1 is a clean,
self-contained, single-implementer unit that closes #88/#79 for this shape and
ships the productive-refusal gate with no daemon-method, route, or guardrail
change. It has **no dependency on #66/#84/#77** — those only bind slices 2–3.

- **Slice 2 (stretch):** register `adjudicated_constraint_extraction` in the
  collaboration shape pack (`go/pkg/workflowgenerate`,
  `go/pkg/workflowtemplates/catalog.json`) + 8-phase generator + emit `${cycle}`
  placeholders + `examples/adjudicated-constraint-extraction-flow/`. **Gated on
  the live #66 behaviour** (codex intake-job routing first; defer + record if it
  still fails `run.prepare`).
- **Slice 3 (stretch, on top of 2):** `final_review` + a `constraint_discharge`
  block on an ordinary `finding`/`findings_ledger` (**not** a new kind) that
  fails closed on any undischarged `binding: true && final_review_required: true`
  constraint, passing only on `discharged` (with evidence) or `accepted_risk`
  (with owner/stage), **without re-running prior phases**. Composes on slice 1's
  schema; another structural validator, no daemon method.
- **Slice 4 (deferred, unchanged from RFC §6/§7):** first-class `constraint.*`
  objects + coverage metrics — only once a second cross-run consumer exists. All
  three lanes agree.

---

## 8. Per-lane ledger (carried / rejected)

- **claude_code — spine (carried).** One-validator/three-path architecture; v1.1
  opt-in scoping via `isV11Ledger`; dedicated `constraints[]`/`findings[]` blocks
  as the #79 fix instead of loosening `entries[]`; enumerate-allowed-verdicts
  error for #88; flat-not-nested front matter; #84 runtime-half-already-exists;
  #66 is the true slice-2 blocker. *Rejected:* widening the `verdict` enum (§2);
  the redundant advisory `version` field (§4.1); the closed `posture` enum (§5);
  `info` severity (§5).
- **codex — two decisive corrections (carried).** Keep the `verdict` enum at the
  four daemon-routable values and put refined states in `branches{}` (§2 — the
  build-blocking catch); `posture` as a free non-empty string (§5); explicit
  `unresolved_question` kind + the `binding || unresolved_question` productive-row
  predicate (§4.3/§5); the `revision_constraints_intake` job to route around #66
  (§6); accept natural YAML via the existing parser with regression tests (§4.5).
  *Rejected:* accepting both map and array `branches` in V1 (deferred, §4.4).
- **agy — framing (carried), code (rejected).** Carried: the crisp "productive
  refusal = a constraint-generating event," the alternatives table (v2 schema and
  first-class daemon objects both correctly rejected/deferred), and the additive-
  preservation intent. *Rejected literally:* its Go snippet makes `schema_version`
  require `v1.1` (breaks every v1 ledger) and gates `needs_revision`
  unconditionally (breaks the canary and AC 7) — replaced by claude_code's opt-in
  scoping (§3).

---

## 9. Top risks (and the canary for each)

- **Additivity regression (highest value).** Any new field `Required:true`, or an
  unscoped gate, breaks AC 7. *Canary:*
  `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge` stays green
  unmodified; add an explicit verbatim-v1-ledger-still-valid test.
- **Verdict wedge (§2).** Re-introducing `blocked_pending_answer` /
  `defer_with_successor` as front-matter verdicts silently passes contract tests
  and wedges at `recordVerdict`. *Canary:* a `go/pkg/mutations` pgtest asserting
  the primitive verdict path clears a v1.1 ledger end-to-end using only the four
  routable verdicts; keep the two refined states in `branches{}` only.
- **Literal RFC transcription.** The RFC's nested YAML and full six-verdict list
  are illustrative; the flat contract and the four-verdict daemon reality govern.
- **Slice-2 over-reach.** Do not relax `run.prepare` to force the back-edge; try
  the intake-job routing, else defer and record (§6).
