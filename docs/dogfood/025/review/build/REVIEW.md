---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0024 V2 (devils_advocate)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. The build claims it ships run-now, If-Match, and
field-level errors with mutation gating, CSP-safe, and full backward
compat. I will argue against each claim.

## Counter-claims tested

### C1: "Run-now is mutation-gated"

`_handle_workflow_run_now` first checks `self.state.allow_mutations`
and returns 405 otherwise. Test
`test_run_now_without_mutations_returns_405` exercises it. I tried
to find a path where the mutation gate could be bypassed (e.g. via
the validation phase reading the file then triggering side effects)
— it can't; nothing happens before the gate. **Survives.**

### C2: "Run-now path safety mirrors edit"

The handler refuses `..`, leading `/`, null bytes (400) and hidden
dirs `.git/.striatum` (404). Tests
`test_run_now_traversal_returns_400`,
`test_run_now_missing_path_returns_404` cover both. I checked: the
path is resolved *after* the basic-string guards, then
`relative_to(repo_root)` ensures no symlink escape. **Survives.**

### C3: "Run-now wraps WorkflowError into 422 + errors[]"

If `create_run` raises `WorkflowError` (validate fails), the handler
catches it and surfaces both `error.message` (V1.5 compat) and
`error.errors[]` if `field_path` is set. Test
`test_run_now_invalid_workflow_returns_422` confirms a tagged raise
site (`jobs[0].role_id`) appears. I asked: what about untagged raise
sites? `errors[]` is empty; client falls back to message. Same
graceful degradation as edit. **Survives.**

### C4: "Run-now handles dirty-tree / branch confirmation"

When `branch.mode == auto`, the handler drives `branch_confirm`; on
`BranchConfirmationError` returns 409. When `branch.mode == confirm`,
the handler observes `state == needs_branch_confirmation` and
returns 200 with that status (operator finishes out-of-band). I
asked: what if branch_confirm succeeds but `run_start` then errors?
The transaction wrapping create_run is committed before
branch_confirm runs (it's outside `with transaction(conn)`), so a
later `run_start` failure leaves a prepared run with a confirmed
branch — recoverable from CLI. **Survives** with a note: V3 should
consider wrapping the whole flow in a single transaction to make the
operation atomic.

### C5: "If-Match precondition is correct"

GET stamps disk sha256; POST reads `If-Match`; on stale, returns 412
with `current_sha256`. The handler also re-checks the sha
*immediately before* `tmp.replace` to narrow the TOCTOU window. Test
`test_edit_post_if_match_stale_returns_412` and
`test_edit_post_if_match_matching_succeeds` cover both. I asked:
what if `If-Match: ""` (empty quotes)? The handler strips quotes,
sees empty, treats as opt-out — backward-compat preserved. What if
`If-Match: *` (RFC 7232 wildcard)? V2 does not implement wildcard;
it's treated as a literal sha that won't match. Acceptable — V3
can add wildcard support.

**Survives.**

### C6: "If-Match missing → V1.5 compat"

V1.5 clients never send the header; V2 treats absence as opt-out.
Test `test_edit_post_if_match_missing_is_v15_compat` confirms 200
even on stale-disk. I confirmed by reading the handler. **Survives.**

### C7: "Field-level errors don't break the V1.5 banner"

The 422 body always carries `error.message` (V1.5 path) AND
`error.errors[]` (V2 path; possibly empty). The editor JS calls
`showError(msg)` always and `highlightFieldErrors(errors)` on top.
V1.5 clients (which don't read errors[]) keep working. **Survives.**

### C8: "8 raise sites tagged are the highest-traffic"

The synthesis listed 8: schema_version, duplicate id, unknown role,
unknown lane, unknown artifact path, cycle from/to, cycle
max_iterations. I checked: these are the most likely to fire from a
form-driven editor (operator typo in role/lane name; bad path;
malformed cycle). The remaining ~22 raise sites are mostly internal
(harness profile schema, lane constraint enforcement) and unlikely
to be operator-typed. **Survives.**

### C9: "CSP unchanged"

I diffed `base.html` and `workflow_detail.html` — no inline `<script>`
blocks, no `<style>`. The new run-now button delegates behavior to
`/static/workflow_run.js` (separate file). The new
`workflow-sha256` script tag is `type="application/json"`, not JS.
**Survives.**

### C10: "Tests cover the surface"

23 new tests (9 + 7 + 7 V2 additions to existing edit). Full suite
474/474 pass. I asked: is there a test for a stale If-Match where
the disk file is missing? Not explicitly, but the handler short-
circuits with `target.is_file()` so the missing-file case treats
If-Match as no-op (writes the new file). Acceptable for V2.
**Survives.**

## Counter-claims that hold

I cannot defeat any acceptance-blocking claim. F1 (workflow-trust
note), F2 (banner-and-fields dual signal), and F3 (dirty-tree 409)
are addressed or honestly deferred per the design review.

## Verdict

**accept**

The build survives the strongest devil's-advocate counterarguments
I can raise. All three findings from the design review are addressed
or explicitly deferred to V3 with rationale. CSP is unchanged. The
backward-compat surface holds: V1.5 clients (no If-Match, no
errors[]) continue to work without modification.
