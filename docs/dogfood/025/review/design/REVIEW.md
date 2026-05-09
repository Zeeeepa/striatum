---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# Design review: RFC 0024 V2 (devils_advocate)

author: reviewer-claude-opus-001

## Posture

Devil's advocate. Argue against the synthesis: scope, safety,
backward-compat, and the specific raise-site picks.

## Counter-claims

### C1: "Run-now POSTs are safe"

The synthesis claims POST `/workflows/run/<path>` is safe because
it's mutation-gated and respects branch_confirm. **Concern:** an
operator with `--allow-mutations` on can spawn arbitrary runs by
clicking through the UI, including against malicious workflow.json
files committed to the repo by a teammate. The current dogfood
review postures don't gate *which* workflows can be run, only what
the runs do.

**Counter:** The same concern already exists for `striatum run
prepare --workflow <path>` from the CLI — V1 introduced no new
attack surface. An operator with shell access to the repo can run
any workflow. Web parity is acceptable.

**Survives** with a finding (F1, below).

### C2: "Each click → fresh run is acceptable"

The synthesis explicitly accepts the "click twice → two runs"
behavior and defers idempotency to V3. **Concern:** an operator
double-clicking the button creates two runs of the same workflow on
the same branch — and both runs will fight over the lease/branch
state.

**Counter:** Each run gets its own `run_id`; they don't share state.
The branch is shared (auto-mode confirms it), but `branch_confirm`
is idempotent — the second call observes the first run already on
the branch and short-circuits or warns. The dashboard shows both
runs, the operator can cancel the duplicate.

**Survives.** Document the double-click behavior in the
BUILD_HANDOFF + tooltip.

### C3: "If-Match TOCTOU race is microseconds"

Design claims read-twice (read sha → validate → re-read sha →
rename) is sufficient. **Concern:** the validate step itself can
take noticeable time on large workflows; concurrent edit could land
between the first sha read and the validate completion.

**Counter:** The synthesis already specifies re-read *immediately
before* the rename, after validation. Validation can take 100ms
worst-case; the race window between re-read and `tmp.replace` is
sub-millisecond. **Survives.** Note: this is honest about the
single-operator local-first model.

### C4: "Missing If-Match → opt-out is safe"

Concern: An older editor (V1.5) sends no If-Match. A V2 client
opens the same file, makes edits, and saves with If-Match=current.
The V1.5 editor then saves without If-Match, blowing away the V2
client's work.

**Counter:** This is exactly the V1.5 behavior. V2 is strictly
additive; opt-in. If both editors are V2, both send If-Match and
the second loses (412 + reload). If one is V1.5, the operator
chose to use the older editor. The synthesis is honest:
"backward-compatible".

**Survives.** Note for V3: deprecate missing-If-Match after one
release.

### C5: "Field-path coverage is partial"

Synthesis updates 8 raise sites; ~22 remain unchanged. **Concern:**
operators encountering unconverted errors get *only* the top-of-form
banner, breaking the consistency promise.

**Counter:** Synthesis is explicit: "field_path stays None so the
editor falls back to the top-of-form banner." This is intentionally
graceful. V3 finishes the sweep.

**Finding (F2, non-blocking).** The 8 picks should cover the
*highest-traffic* errors operators see — duplicate ids, unknown
roles, unknown lanes, bad artifact paths. Confirmed by the table.
Survives.

### C6: "WorkflowError extension is backward compatible"

Concern: tests that catch `WorkflowError` and inspect `str(exc)`
keep passing, but tests that instantiate `WorkflowError(message)`
positionally with the field_path? — None do; the API surface
adds keyword-only `field_path=None`. **Survives.**

### C7: "422 body adds errors[] without breaking V1.5 clients"

Concern: V1.5 client's editor JS always falls back to
`body.error.message`; never reads `body.error.errors`. Adding a new
key in the JSON shape can't break a strict client.

**Counter:** Confirmed. **Survives.**

### C8: "branch_confirm 409 is right"

Concern: a 409 implies "conflict with existing resource"; a 422
"unprocessable" might be more semantically accurate for "dirty
tree". **Counter:** 409 is conventional for "state conflict that the
client must resolve" — dirty-tree fits. 412 is reserved for
If-Match.

**Survives.**

## Findings

### F1 (note, non-blocking): Document the workflow-trust model

The synthesis should state explicitly that "Run now" trusts every
`workflow.json` file in the repo. Add a one-liner to the BUILD_HANDOFF
and a SPEC note: "operators with --allow-mutations can launch any
committed workflow.json; this matches the CLI surface."

### F2 (note, non-blocking): Field-path tooltip should also surface globally

When the editor highlights a field, also surface the same message in
the top-of-form banner so screen-readers and operators with the
field scrolled out of view see it.

### F3 (note, non-blocking): Dirty-tree 409 should include `git_status`

The 409 body should carry the `git status --short` output (or its
first ~3 lines) so the operator sees *what's* dirty without
context-switching to a terminal. Optional but materially improves UX.

## Verdict

**accept_with_findings**

Three findings, all non-blocking. The scope is tight, the contracts
are clean, and backward-compat is honest. Implementer should fold
F1-F3 into the build but should not block on them.
