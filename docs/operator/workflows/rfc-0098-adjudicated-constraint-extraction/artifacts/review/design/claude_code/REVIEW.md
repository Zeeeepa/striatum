---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0098", "design-review", "collaboration_ledger.v1.1"]
---

# RFC 0098 Design Synthesis — Ergonomics/DX Review

author: reviewer-claude-opus-4.8-001

**Posture:** `ergonomics_dx` — operator/developer experience of the design,
evaluated from a first-time-user perspective. Acceptance under this posture
means the affordances are **discoverable** and **consistent**.

**Verdict:** `needs_revision` (two bounded, pure-doc/contract revisions; both
conceded by the synthesizer under interrogation; neither depends on #66/#84;
each can land inside slice 1).

**Target reviewed:**
`artifacts/DESIGN_SYNTHESIS.md` (single buildable synthesis,
`schema_version: striatum.synthesis.v1`).

---

## Interrogation summary — 2 of 3 rounds, stopped early (findings resolved)

I hold the `interrogate` capability and interrogated the live synthesizer
session (`synthesizer-claude_code-1`) before this verdict, per the task
prompt. **I used 2 of the allowed 3 rounds and stopped early because both of
my open ergonomics findings were resolved** — I did not pad to the cap.

- **Round 1 (F1 — authoring-surface discoverability).** Asked what copyable,
  human-readable authoring surface exists for a first-time adjudicator writing
  a `collaboration_ledger.v1.1` ledger in the *slice-1-only* end state, given
  slice 2 (generator + `examples/adjudicated-constraint-extraction-flow/`) is
  explicitly deferrable. The synthesizer **conceded the gap** and committed a
  fix (see Finding 1).
- **Round 2 (F3 — surprising activation footgun).** Asked whether the
  `isV11Ledger = (schema_version==v1.1) OR (shape==adjudicated_constraint_extraction)`
  trigger creates a non-local surprise (a `schema_version: v1` ledger gated to
  the v1.1 rule purely because of an unrelated `shape` field) and whether the
  belt-and-suspenders OR earns its cost. The synthesizer **conceded** and
  committed a strictly better design (see Finding 2), noting the threat-model
  reviewer independently reached the same conclusion from the bypass-coupling
  side.

The document *as written* does not yet meet the DX bar on either axis;
the synthesizer's interrogation commitments live only in the interrogation
thread, not in the durable synthesis the builder will follow. The verdict
asks that both commitments be folded into the synthesis before build.

---

## What is strong (and should not be lost in revision)

The synthesis has genuinely good DX bones; this is a "tighten the surface,"
not a "rethink the design" verdict.

1. **One validator, three gate paths (§1).** Collapsing the gate to a single
   `validateCollaborationLedger` is the right ergonomic call: there is one
   place to read to understand the rule, and authors cannot hit a path where
   the rule silently doesn't apply. This is the opposite of the RFC 0093
   build-finding-#2 footgun and is correctly carried.
2. **Verdict enum stays the four daemon-routable values (§2).** This is the
   single best DX decision in the document. Keeping
   `accept | accept_with_findings | needs_revision | reject` and routing the
   two refined states (`blocked_pending_answer`, `defer_with_successor`) to
   `branches{}` dispositions means an author *cannot* write a front-matter
   verdict that wedges the daemon verdict state machine. Widening the enum
   (as two lanes proposed) would have created exactly the kind of
   silent-at-contract-time / wedge-at-`recordVerdict` trap that is hostile to
   first-time authors. Keep this.
3. **`branches{}` as a map, not codex's array (§4.4).** For a hand-author the
   `posture → disposition` map is the lower-ceremony shape. Reasonable default;
   the array form's per-branch `constraint_ids` linkage is a real loss for
   AC-6 posture-matrix rendering, but deferring it keeps slice 1 to one shape.
   Acceptable as scoped — flagged below as a watch-item, not a blocker.
4. **`needs_revision` ⇒ "productive row = `binding` OR `unresolved_question`"
   (§4.3).** The disjunction is the humane reading: it does not over-reject an
   honest "we cannot resolve this yet" refusal. Good.

---

## Findings (binding revisions)

### Finding 1 — Slice 1 ships no discoverable authoring surface (DX-blocking). `needs_revision`

**Severity: medium. Binding.**

In the slice-1-only end state the *only* written-down description of the
`collaboration_ledger.v1.1` shape — the constraint-row key set, the
`branches{}` disposition vocabulary, and the productive-row rule — lives in
Go validator source and test fixtures. A first-time adjudicator authoring a
v1.1 ledger by hand has **nothing to copy** and no human-readable schema doc.
Slice 2's generator/examples are the natural authoring surface, but they are
explicitly deferrable, so in the worst (and most likely) case the shape ships
with *zero* positive authoring affordance. The synthesis's own #79/#88 fixes
(natural front matter accepted + better error text) only lower the **cost of a
wrong guess**; they do not provide a **positive surface to copy from**.
Pointing an author at `contracts.go` is precisely the reverse-engineering
failure mode that produced #79/#88 — reproduced one layer up.

The synthesizer conceded this in round 1 ("you're right, and this is a gap in
my synthesis as written") and committed a fix that is **pure docs + a fixture,
no daemon, no #66/#84 dependency**. The revision is to make that commitment
binding in the synthesis:

**Required revision (fold into the synthesis §4.6 slice-1 Definition of Done /
acceptance criteria, not slice 2):**

1. A `docs/reference` schema entry for `collaboration_ledger.v1.1` covering:
   the constraint-row key set
   (`id / posture / severity / kind / binding / text / source_finding /
   source_refs / verification / final_review_required`), the `branches{}`
   disposition vocabulary, the productive-row rule
   (`binding || unresolved_question`), and — critically for DX — the
   **verdict-stays-four / refined-states-live-in-`branches{}`** split, so an
   author does not write `verdict: blocked_pending_answer` and wedge.
2. At least **one committed valid example ledger that is byte-identical to a
   slice-1 test fixture** (the doc example and the test corpus are the same
   bytes, validated under the same pgtest run), so the human-readable example
   cannot drift from the contract. Drift between advertised and enforced shape
   is the #88 root cause; binding the example to the test prevents recreating
   it. The example set must include a `needs_revision` ledger with a real
   `binding` constraint **and** an `unresolved_question` row, plus a clearing
   ledger.
3. Slice-1 rejection error messages reference that doc entry by name.

Until items 1–2 ship *in slice 1*, slice 1 reproduces the opaque-exit-6
failure mode for the new v1.1 fields and better error text only softens it.

### Finding 2 — `isV11Ledger` OR-trigger is a non-local activation surprise (consistency-blocking). `needs_revision`

**Severity: medium. Binding.**

§3/§4.3 scope the gate as
`schema_version == "…v1.1" OR shape == "adjudicated_constraint_extraction"`.
From a first-time author's seat this breaks the **locality** of
`schema_version`: an author who writes `schema_version:
striatum.collaboration_ledger.v1` + `shape: adjudicated_constraint_extraction`
+ `verdict: needs_revision` with empty `constraints[]` (perfectly valid under
v1 today — the existing canary proves it) is now silently held to the v1.1
productive-refusal rule and rejected. The same `schema_version: v1` value
behaves differently depending on an unrelated field. That teaches authors that
`schema_version` is not a trustworthy signal of which rules apply — a
corrosive DX outcome — and the rejection diagnoses a *symptom* (missing
constraints) rather than the *cause* (used an ACE shape without opting into
v1.1).

The synthesizer conceded in round 2 and committed a strictly better design
that the threat-model reviewer independently reached:

**Required revision (replace the §3/§4.3 OR):**

- Gate fires on `shape == "adjudicated_constraint_extraction"` **only**.
- Add a **shape→version consistency check, evaluated first**, asserting an
  ACE-shaped ledger MUST declare `schema_version:
  striatum.collaboration_ledger.v1.1`, with a local error naming **both**
  fields the author set and the rule binding them
  (e.g. *"shape adjudicated_constraint_extraction requires schema_version
  striatum.collaboration_ledger.v1.1 (got …v1)"*).

This keeps the exact bypass protection the OR was reaching for (a naked
`needs_revision` ACE refusal cannot escape the `constraints[]` requirement by
declaring/forgetting `v1`) while **restoring `schema_version: v1` as a
reliable local signal** ("v1 rules, no productive-refusal, full stop") and
making the rejection point at the cause. Fold this into synthesis §3/§4.3 and
the §9 verdict-wedge canary discussion.

---

## Non-blocking observations (record, do not gate)

- **N1 — `branches{}` map loses per-branch `constraint_id` linkage.** AC-6
  posture-matrix rendering wants to show which constraints belong to which
  posture disposition; the map form (§4.4) cannot carry that without a
  convention. Acceptable for V1, but note in the synthesis that AC-6 rendering
  is satisfied only at the disposition granularity, not constraint-to-branch,
  until codex's array form is adopted.
- **N2 — `accepted_risk` constraints need an owner/stage home.** §7 slice 3
  passes `final_review` on `accepted_risk` "with owner/stage", but the §4.2
  constraint-row key set has no `owner`/`stage` field (only an optional
  `verification: {expected_stage?, gate?}`). Slice 3 is deferred, so this is
  not a slice-1 blocker, but the synthesis should name where owner/stage will
  live so a slice-3 author isn't surprised. Record in §7.
- **N3 — error-message quality is now load-bearing, so test it.** Both
  findings' fixes lean on diagnostic error text. Recommend the slice-1 test
  corpus assert on the *substance* of the rejection messages (enumerated
  verdicts, named offending field/index, the shape→version mismatch, the doc
  reference), not merely the exit code, so the DX affordance is regression-
  protected. Strengthens §4.6 items 4–5.

---

## Verdict rationale

`needs_revision`. The design's architecture is sound and several of its DX
calls (single validator, four-verdict enum, productive-row disjunction) are
exactly right and must survive revision. But under the `ergonomics_dx` bar —
affordances discoverable **and** consistent — the synthesis as written fails
both: it provides no positive authoring surface in slice 1 (Finding 1, not
discoverable) and it breaks the locality of `schema_version` (Finding 2, not
consistent). Both fixes are bounded, pure docs/contract, free of #66/#84
dependency, and already conceded by the synthesizer under interrogation; both
make the *durable* spec — which the builder follows — meet the bar. This is a
one-pass tightening well within the workflow's two-revision budget, not a
rethink. Re-review on the revised synthesis should clear to `accept` if
Findings 1–2 are folded in as specified.
