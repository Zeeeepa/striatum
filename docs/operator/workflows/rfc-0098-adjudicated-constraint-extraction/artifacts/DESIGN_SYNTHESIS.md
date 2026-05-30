---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0098 Design Synthesis — Adjudicated Constraint-Extraction Loop

author: synthesizer-claude-opus-4.8-001

Single buildable synthesis reconciling the three lane designs
(`design/claude_code/DESIGN.md`, `design/codex/DESIGN.md`, `design/agy/DESIGN.md`)
into one implementable plan for RFC 0098, sliced smallest-blast-radius first.
Every architectural claim is checked against the code as it stands today
(`go/pkg/artifactcontracts/contracts.go`, `go/pkg/mutations/collaboration_ledger.go`,
`go/pkg/mutations/review.go`), not against the RFC's illustrative YAML.

**Revision 2** (attempt 2) discharges the design-review panel's findings
(codex threat-model `needs_revision`; agy devil's-advocate `accept_with_findings`).
The discharge table is §A; the substantive changes are folded into §3–§4.

---

## A. Discharge of design-review findings (attempt 2)

Mirroring RFC 0098's own constraint-discharge discipline — each review finding is
a binding constraint on this synthesis, discharged below.

| Finding | Sev | Disposition | Where discharged |
|---|---|---|---|
| **codex F1** — ambiguous v1.1 gate scope; state the truth table | high | **folded in** | §3 rewritten: gate is **shape-only**; `schema_version` stays a pure format axis; explicit truth table + required tests in §3.2/§4.6 |
| **codex F2** — `findings[]` row contract unspecified | high | **folded in** | §4.2b: full `findings[]` row validator + D028 unknown-key rejection + fixtures |
| **codex F3** — review-submission idempotency bypass untested | medium | **folded in** | §4.6 test 7: idempotent/already-published submit cannot clear a naked refusal |
| **agy F1** — hollow-constraint evasion path | low | **folded in (hardened to required)** | §4.2: `binding:true` ⇒ `source_finding` resolves to a high/critical `findings[]` row **and** non-empty `verification` |
| **agy F2 / codex (slice-2 edge risk)** | med | **accepted, already-aligned** | §6: slice 2 gated on live `run.prepare`; slice 1 has no graph dependency |
| **DX reviewer R1/R2** — no slice-1 authoring surface; non-local activation surprise | — | **folded in** | §3 (locality), §4.7 (docs/reference entry + tested example ledger as a slice-1 deliverable) |

Two reviewers (codex threat-model F1, DX reviewer R2) hit the **same** gate-scope
smell from opposite directions (bypass-coupling vs. non-local surprise). The
single revision in §3 resolves both.

---

## 0. Decision in one paragraph

**Build slice 1 only as the V1 target: an additive `collaboration_ledger.v1.1`
schema (`constraints[]` + `branches{}` + `findings[]` + `cycle`) plus a
**shape-scoped** productive-refusal gate, landed entirely inside the one contract
function `validateCollaborationLedger`, with zero new daemon methods.** The spine
is **claude_code's** code-grounded architecture. I adopt **two corrections from
codex** that are build-blocking: (a) **do not widen the front-matter `verdict`
enum** — it is coupled to the daemon verdict state machine and widening it wedges
the run (§2); (b) **`posture` is a non-empty string, not a closed enum** (§5). I
**reject agy's literal Go** (it breaks additivity and the canary) while keeping
its framing. Per the panel: the gate triggers on `shape ==
adjudicated_constraint_extraction` **only** (not on `schema_version`), with an
explicit `ACE ⇒ v1.1` consistency check closing the forgot-the-bump bypass (§3);
binding constraints are structurally grounded in a high/critical finding + a
verification gate (§4.2); `findings[]` has a fully specified row contract (§4.2b).
Slices 2–3 defer at a recorded boundary (slice 2 gated on live `run.prepare`/#66).

---

## 1. The load-bearing fact (claude_code, verified)

Every write path that can admit a `collaboration_ledger` funnels through the same
contract function, so slice 1 is "pure contract/validation work, no daemon
method." Re-traced in source:

| Path | Entry | Reaches |
|---|---|---|
| `publish-artifact` | `validateArtifactFrontMatter` → `ValidateFrontMatter` | `validateCollaborationLedger` (`contracts.go:550`) |
| `submit-review` precheck | `prevalidateSubmitReviewArtifactVerdict` → `ParseAndValidateFrontMatter` | same |
| primitive `review.verdict` | `recordVerdict` → `enforceCollaborationLedgerVerdict` (`collaboration_ledger.go:89`) → `ParseAndValidateFrontMatter` | same |

The gate belongs in exactly one place — `validateCollaborationLedger` — and all
three paths enforce it for free, including the primitive `review.verdict` path
that RFC 0093's build had to close separately. The error is already
`rpc.NewError("artifact_error", …)` → CLI **exit 6**; we inherit the code. No new
RPC, no command-authority-matrix route, no guardrail-test change. All three lanes
independently reached this "contract is the only unbypassable point" conclusion.

---

## 2. Decisive contradiction: the `verdict` enum (codex over claude_code + agy)

claude_code §3.3 and agy §3 widen the front-matter `verdict` enum to add
`blocked_pending_answer` / `defer_with_successor`. **codex refuses, and the code
proves codex right.** `enforceCollaborationLedgerVerdict`
(`collaboration_ledger.go:126-129`) forces the *recorded* review verdict to
**equal** the ledger's front-matter `verdict`; that recorded verdict flows into
`recordVerdict`'s switch (`review.go:478-531`), which routes only `accept`,
`accept_with_findings`, `needs_revision`, `reject` — anything else hits
`default → rpc.NewError("invalid_transition", "unknown verdict")`. So a
`verdict: blocked_pending_answer` ledger wedges the adjudication job with no legal
clearing path.

**Resolution:** the front-matter `verdict` enum stays the four daemon-routable
values. The two RFC 0098 refinements live as **`branches{}` dispositions** (pure
metadata the daemon never routes on). Promoting them to verdicts is a
daemon-state-machine change for a later slice. (Panel endorsed: codex + agy both
list this as a positive finding.)

---

## 3. Gate scope: **shape-only**, with an explicit `ACE ⇒ v1.1` check (discharges codex F1 + DX R2)

The attempt-1 synthesis scoped the gate with
`isV11Ledger = schema_version == "…v1.1" OR shape == "adjudicated_constraint_extraction"`.
The panel correctly flagged the OR as both a bypass-coupling smell (codex F1) and
a non-local activation surprise (DX R2): the same `schema_version: …v1` value
would behave differently based on an unrelated `shape` field, and any future
non-ACE v1.1 ledger would be silently swept into the productive-refusal rule.

### 3.1 The decision

**The productive-refusal gate triggers on `shape == adjudicated_constraint_extraction`
only.** `schema_version` is a pure *format* axis (which fields may appear); the
gate is a *shape-behavior* axis. They are decoupled. The forgot-the-bump bypass
is closed not by an over-broad OR but by an explicit, ordered consistency check:

```
# evaluated inside validateCollaborationLedger, in this order:
1. if shape == "adjudicated_constraint_extraction" && schema_version != "…v1.1":
       reject: "shape adjudicated_constraint_extraction requires
                schema_version striatum.collaboration_ledger.v1.1 (got <X>)"
2. if shape == "adjudicated_constraint_extraction" && verdict == "needs_revision":
       if countProductiveRows(constraints) == 0: reject (exit 6, productive-refusal)
```

Check 1 runs **first**, so an `ACE + v1` ledger is rejected for the version
mismatch (a local error naming both fields the author set, diagnosing the real
mistake) rather than for a downstream missing-constraints symptom. An ACE ledger
therefore *cannot exist at v1* — it is rejected loudly, never silently ungated —
which closes codex F1's bypass while restoring `schema_version: v1` as a reliable
"v1 rules, no productive-refusal" signal.

### 3.2 The truth table (codex F1 required revision — stated explicitly, tested in §4.6)

| shape | schema_version | verdict | constraints[] | result | why |
|---|---|---|---|---|---|
| falsification_gate | v1 | needs_revision | empty | **accept** | canary; AC 7 additivity (gate not triggered) |
| falsification_gate | v1 | accept | claim+challenge+rebuttal | accept | existing clearing-substance rule unchanged |
| adjudicated_constraint_extraction | v1.1 | needs_revision | empty | **reject** | productive-refusal gate |
| adjudicated_constraint_extraction | v1.1 | needs_revision | ≥1 binding **or** ≥1 unresolved_question | **accept** | productive row present |
| adjudicated_constraint_extraction | **v1** (forgot bump) | needs_revision | (any) | **reject** | consistency check: ACE ⇒ v1.1 (closes the bypass) |
| future non-ACE shape | v1.1 | needs_revision | empty | **accept** | **ACE-only semantics — deliberate**; productive-refusal does not apply to non-ACE shapes |
| any | v1.1 | (any) | + unknown top-level field | reject | unknown-fields guard intact (additivity) |

Row 6 is the explicit decision codex F1 demanded: a non-ACE ledger that adopts
v1.1 fields for unrelated reasons is **deliberately accepted** without a
constraints[] requirement. This avoids promising a global "all v1.1 must be
productive" semantic that we would later have to walk back (breaking the additive
discipline) when a second shape wants v1.1 fields.

---

## 4. Slice 1 — concrete, buildable spec (the V1 target)

All edits in `go/pkg/artifactcontracts/contracts.go` + tests in `contracts_test.go`
and `go/pkg/mutations/*_test.go` (live PG, RFC 0080 pgtest).

### 4.1 Schema changes to `Schemas["collaboration_ledger"]` (`contracts.go:229`)

```
schema_version : oneOfValue("striatum.collaboration_ledger.v1",
                            "striatum.collaboration_ledger.v1.1")   # widened, additive
shape          : add "adjudicated_constraint_extraction" to the existing oneOfValue
verdict        : UNCHANGED → accept | accept_with_findings | needs_revision | reject   (§2)
# --- new optional v1.1 fields, all Required:false ---
cycle          : optional non-negative int            (advisory; #84 substitution keys on attempt)
constraints    : optional list<constraint-row>        (§4.2)
branches       : optional map<posture-string → disposition>   (§4.4)
findings       : optional list<finding-row>           (§4.2b — typed home for #79 cross-exam rows)
```

The flat front matter is the real contract — **do not** transcribe the RFC's
nested `collaboration_ledger:` mapping (a nested mapping trips the unknown-field
guard at `contracts.go:313-322`). `schema_version` is the version axis — **no
separate `version` field**. The parser (`ParseFrontMatterBlock` → `parseYAMLNode`,
`contracts.go:361/394`) is `yaml.v3` and already recurses through maps and
sequences, so nested `branches{}` and lists-of-maps `constraints[]`/`findings[]`
parse natively; new validators follow the `collaborationLedgerEntryList` pattern
(`contracts.go:711`).

### 4.2 `constraints[]` row validator (`isConstraintListValue`, structural only)

Per Non-Goal "no semantic scoring" — validate *present, typed, sourced,
dispositioned*, never *good*:

- `id` — non-empty string
- `posture` — **non-empty string** (not a closed enum — §5)
- `severity` — `low | medium | high | critical`
- `kind` — `invariant | gate | schema | policy | non_goal | accepted_risk | unresolved_question`
- `binding` — bool
- `text` — non-empty string
- `source_finding` — string; **required when `binding: true`** (see hardening below)
- `source_refs` — optional list of `dialogue:<seq>` refs (reuse `isDialogueRef`)
- `verification` — optional map `{expected_stage?, gate?}`; **required non-empty when `binding: true`**
- `final_review_required` — optional bool
- **Unknown keys in a row are rejected** (D028 guard; a `stdout:`-style raw field cannot ride a row).

**Hardening for `binding: true` rows (discharges agy F1 + codex F1 — promoted from
optional to required):**

1. `source_finding` is **required and must resolve** to a `findings[]` row (same
   ledger) whose `id` matches **and** whose `severity ∈ {high, critical}`. A
   binding constraint must be grounded in a load-bearing cross-exam finding.
2. `verification` must be present with at least one non-empty of `{gate,
   expected_stage}`.

This makes "C1: ensure code is correct" structurally invalid as a binding
constraint — it would require fabricating a high/critical `findings[]` row to
source from (itself typed and `dialogue:`-traceable) **and** a concrete
verification gate — without any semantic/LLM judgment, staying fail-closed and
SDK-free. `unresolved_question` rows (which are not dischargeable obligations)
keep `source_finding`/`verification` optional, so an honest "we can't resolve this
yet" refusal still satisfies the gate.

### 4.2b `findings[]` row validator (`isFindingRowListValue`) — discharges codex F2

Specified before build, same D028 strictness as `constraints[]`:

- **Required keys:** `id` (non-empty string), `severity` (`low|medium|high|critical`),
  `posture` (non-empty string), `status`
  (`open | answered | accepted | rejected | converted_to_constraint | deferred_with_owner`
  — the RFC §3 objection lifecycle), `challenge` (non-empty string).
- **Optional keys:** `closest_acceptable_answer` (string),
  `affected_invariants` (list of strings), `requested_constraint_shape` (map,
  e.g. `{kind}`), `requires_convener_rebuttal` (bool),
  `source_refs` (list of `dialogue:<seq>`).
- **Unknown keys rejected** (D028). A `findings[]` row carrying a `stdout:`-style
  raw-output key is rejected exactly like a `constraints[]` row.

D028 regression fixtures (codex F2 required): (a) a `findings[]` row with a raw
`stdout:` key → rejected; (b) a natural multiline `challenge` + a YAML-list
`affected_invariants` → accepted (proves natural cross-exam authoring works
without recreating #79 in a new field).

### 4.3 The productive-refusal gate

Inside `validateCollaborationLedger`, after the existing clearing-verdict block,
in the order from §3.1: consistency check (ACE ⇒ v1.1) first, then —

```
if shape == "adjudicated_constraint_extraction" && verdict == "needs_revision" {
    if countProductiveRows(constraints) == 0 {
        return error("adjudicated_constraint_extraction needs_revision requires a
                      non-empty constraints[] (≥1 binding constraint or ≥1
                      unresolved_question row); see docs/reference/<v1.1 entry>")
    }
}
```

`countProductiveRows` = rows where `binding == true` **or** `kind ==
"unresolved_question"`. The error names the rule and points at the docs entry
(§4.7) — advertise-what-you-enforce, the #88 discipline.

### 4.4 `branches{}` validator (`isPostureDispositionMatrixValue`)

`map<posture-string → disposition>`, disposition ∈ `cleared`,
`cleared_with_constraints`, `blocked`, `blocked_pending_answer`,
`defer_with_successor`. **This is where the two RFC §4 refined states live** (§2).
Canonical = map (matches RFC §4). codex's richer array form is **deferred** unless
trivially additive; the map discharges AC 6's posture-matrix rendering.

### 4.5 Closing #88 and #79 *for this shape*

- **#88:** keep the four daemon-routable verdicts (§2); make the rejected-verdict
  error **enumerate the allowed verdicts**; fix slice-2 shape prompts to advertise
  `accept_with_findings`, never bare `clear` (which stays disallowed, RFC §4).
- **#79:** rich adjudicator metadata gets typed homes (`constraints[]`,
  `findings[]`, `branches{}`) rather than loosening trajectory-anchored
  `entries[]`. Improve the `entries`-invalid / unknown-fields errors to name the
  offending index/keys. Natural multiline/nested YAML already parses (§4.1); the
  regression tests are the deliverable.

### 4.6 Slice 1 definition of done (tests prove every truth-table branch + both bypass classes)

1. **Additive canary:** the existing
   `TestCollaborationLedgerAllowsNeedsRevisionWithUnrebuttedChallenge`
   (`contracts_test.go:204`; `v1`/`falsification_gate`/`needs_revision`/no
   constraints) stays green **unmodified**; add a verbatim-v1-ledger-still-valid
   test — AC 7.
2. **Truth table (codex F1):** one test per row of §3.2, including row 5
   (`ACE + v1` rejected for version mismatch) and row 6 (`non-ACE + v1.1 +
   needs_revision + empty` **accepted**).
3. **Productive-refusal both directions:** `ACE/v1.1/needs_revision` with empty
   constraints rejected (exit 6); with ≥1 binding **or** ≥1 unresolved_question
   accepted — AC 2.
4. **Binding-row hardening (agy F1):** a `binding:true` row whose `source_finding`
   does not resolve to a high/critical `findings[]` row → rejected; one with no
   `verification` → rejected; a well-grounded one → accepted.
5. **`findings[]` row + D028 (codex F2):** raw-key row rejected; natural
   multiline/nested row accepted.
6. **All three paths:** the gate fires on `publish-artifact`, `submit-review`, and
   primitive `review.verdict` (pgtest in `go/pkg/mutations`).
7. **Submission idempotency (codex F3):** an already-published / idempotent
   `review.submit` (and the `publish-artifact` + `review.verdict` re-clear path)
   **cannot** record or clear a v1.1 naked-`needs_revision` ledger without
   re-running `validateCollaborationLedger` on the front matter — i.e. validation
   is not skipped on the friendly/idempotent path.
8. **#88 / D028:** rejected-verdict error enumerates allowed verdicts; a
   `constraints[]` row with an unknown/raw key is rejected.

**Verification commands** (TASK):

```sh
PATH="$HOME/go/bin:$PATH" make -C go check
make test && make lint && make typecheck
STRIATUM_PG_TEST_URL=postgres:///postgres go -C go test ./pkg/artifactcontracts/... ./pkg/mutations/...
```

### 4.7 Slice-1 authoring surface (discharges DX R1 + agy actionable #1) — a required deliverable, not deferred

Independent of slice 2's generator, slice 1 **must** ship a discoverable,
contract-faithful authoring surface:

- A `docs/reference/` schema entry for `collaboration_ledger.v1.1`: the
  `constraints[]` key set, the `findings[]` key set (§4.2b), the `branches{}`
  disposition vocabulary, the productive-row rule, the `ACE ⇒ v1.1` rule, and the
  verdict-stays-four / refined-states-in-`branches` split (so an author never
  writes `verdict: blocked_pending_answer` and wedges the verdict path).
- **Two complete worked examples**: a `needs_revision` ledger with a grounded
  binding constraint + an `unresolved_question` row, and a clearing ledger. The
  example bytes **are** a slice-1 test fixture (validated under the same pgtest),
  so the doc cannot drift from the contract — drift is the #88 root cause.
- Slice-1 error messages link the author to that doc entry.

Without §4.7, slice 1 reproduces the #79/#88 opaque-exit-6 failure mode one layer
up; better error text only softens it.

---

## 5. Other reconciled contradictions

- **`posture`** — free non-empty string, not a closed enum (**codex** over
  claude_code): RFC §2 allows workflow-authored postures. The default five
  postures are a prompt/pack convention. (Panel positive finding.)
- **`kind`** — explicit `unresolved_question` (**codex**) makes AC 2's "or
  unresolved-question row" structurally precise.
- **`severity`** — `low|medium|high|critical` (**codex**); an `info`-severity
  *binding* constraint is incoherent.
- **`findings[]` in-ledger** (**claude_code**) so `source_finding` resolves
  locally — now load-bearing for the binding-row hardening (§4.2).
- **#84 republish — already half-built** (claude_code §4 / codex): the `${cycle}`
  substitution (`collaboration_ledger.go:27-74`) is wired into claim, required-
  artifact verification, the submit-review precheck, and verdict enforcement.
  Slice 2's job is only to make the generator *emit* `${cycle}` placeholders. The
  `cycle` field added in slice 1 is advisory metadata. (Panel positive finding.)

---

## 6. Unresolved contradictions and how the build handles them (panel-endorsed)

1. **Slice 2 back-edge (#66) — the real blocker; defer cleanly.** The illegal edge
   is `adjudication → revision_synthesis.phase_synthesis` (into a later phase's
   synthesis job). codex's workaround: route `adjudication → revision_constraints_intake`,
   a **non-synthesis** first job *inside* `revision_synthesis` (the immediate-next
   phase), which then feeds that phase's synthesis intra-phase. This is a forward
   DAG edge, **not** a cycle — the revision "loop" lives in the **attempt**
   dimension (RFC 0095 re-open + #84 cycle-aware names), not as a static back-edge.
   **Residual risk (codex/agy F2):** if the live `run.prepare` rule is stricter
   than "no edge into a later synthesis job" (e.g. cross-phase edges must be
   synthesis→synthesis), the intake target is also rejected. **Build directive:**
   attempt slice 2 only after slice 1 is green; validate the generated graph
   against **both** `workflow validate` **and** `run.prepare` on the live daemon;
   **defer slice 2 and record it** if the edge is rejected. Slice 1 has no graph
   dependency. (Both reviewers endorsed this sequencing.)
2. **#77 (adjudicator absorbs the cycle).** `recordVerdict` already routes
   `needs_revision` to a matched cycle and only opens a checkpoint when none
   matches (`review.go:490-520`). Slice 1 is contract-only; confirm in the live
   daemon before slice 2.
3. **`branches` map vs array (codex).** Map-only for V1; array form deferred.
4. **Residual hollow-constraint surface.** The §4.2 hardening raises the structural
   cost substantially (high/critical sourced finding + verification gate), but a
   determined adjudicator can still fabricate a grounded-looking finding. Accepted
   (Non-Goal: no semantic scoring); the interrogable, reviewer-independent
   adjudicator + §7 coverage metrics remain the backstop.

---

## 7. Scope: what to land next (smallest implementable unit)

**Land slice 1 and stop there if the run risks wedging.** Slice 1 (now including
the §4.2 hardening, the §4.2b `findings[]` contract, the §4.7 docs+example, and
the §4.6 tests) is a self-contained, single-implementer unit that closes #88/#79
for the shape and ships the productive-refusal gate with no daemon-method, route,
or guardrail change, and **no dependency on #66/#84/#77**.

- **Slice 2 (stretch):** register `adjudicated_constraint_extraction` in the
  collaboration shape pack + 8-phase generator + `${cycle}` placeholders +
  `examples/adjudicated-constraint-extraction-flow/`. **Gated on live #66** (codex
  intake routing first; defer + record if it still fails `run.prepare`).
- **Slice 3 (stretch, on slice 2):** `final_review` + a `constraint_discharge`
  block on an ordinary `finding`/`findings_ledger` (not a new kind) that fails
  closed on any undischarged `binding: true && final_review_required: true`
  constraint, passing only on `discharged` (with evidence) or `accepted_risk`
  (with owner/stage), without re-running prior phases.
- **Slice 4 (deferred, RFC §6/§7):** first-class `constraint.*` objects + coverage
  metrics — only once a second cross-run consumer exists.

---

## 8. Per-lane ledger (carried / rejected)

- **claude_code — spine (carried).** One-validator/three-path architecture;
  dedicated `constraints[]`/`findings[]` blocks as the #79 fix; enumerate-allowed-
  verdicts error for #88; flat-not-nested front matter; #84 runtime-half-exists;
  #66 is the true slice-2 blocker. *Rejected:* widening `verdict` (§2); the
  advisory `version` field (§4.1); the closed `posture` enum (§5); `info` severity;
  **and the `schema_version`-OR gate scoping (revised to shape-only per panel, §3).**
- **codex — corrections (carried).** Keep the four verdicts + refined states in
  `branches{}` (§2); free-string `posture` (§5); explicit `unresolved_question`
  kind (§4.2); the `revision_constraints_intake` route around #66 (§6); natural
  YAML via the existing parser (§4.5). **Plus the attempt-2 review findings F1–F3
  (§A).** *Rejected:* map+array `branches` in V1 (deferred, §4.4).
- **agy — framing (carried), code (rejected); review findings carried.** Carried:
  "productive refusal = a constraint-generating event"; the alternatives table
  (v2 schema + first-class objects rejected/deferred). **Plus review F1 hollow-
  constraint hardening (§4.2).** *Rejected literally:* its Go snippet forces
  `schema_version == v1.1` and gates `needs_revision` unconditionally — replaced by
  shape-only scoping + the consistency check (§3).

---

## 9. Top risks (and the canary for each)

- **Additivity regression (highest value).** Any new field `Required:true`, or a
  gate not scoped to `shape == ACE`, breaks AC 7. *Canary:* the unmodified
  unrebutted-challenge test + a verbatim-v1-ledger test.
- **Verdict wedge (§2).** Re-introducing a fifth front-matter verdict passes
  contract tests and wedges `recordVerdict`. *Canary:* a `go/pkg/mutations` pgtest
  clearing a v1.1 ledger end-to-end with only the four routable verdicts.
- **`findings[]`/`constraints[]` under-validation (codex F2).** A loose row
  validator lets `stdout:`-shaped keys ride in. *Canary:* the §4.2b/§4.6 D028
  fixtures.
- **Slice-2 over-reach.** Do not relax `run.prepare` to force the back-edge; try
  intake routing, else defer and record (§6).
