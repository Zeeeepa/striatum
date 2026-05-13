---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0024 V1.5 (workflow visual builder)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. The build claims it ships a visual workflow builder
that round-trips through `validate_workflow`, refuses traversal /
hidden-dir paths, refuses non-JSON, refuses oversize bodies, refuses
mutation when `--allow-mutations` is off, and writes atomically. I
will argue against each of those.

## Counter-claims tested

### C1: "Path safety mirrors `/view/<path>`"

The build adds path safety for both GET and POST: `..`, leading `/`,
null bytes refused with 400; hidden dirs (`.git`, `.striatum`)
refused with 404. Tests `test_get_traversal_refused_400`,
`test_get_hidden_refused_404`, and `test_post_traversal_400` cover
each case. I tried to find a path that would land outside the repo
root via creative URL encoding; the handler `urlparse`s the path
*then* re-resolves before the `relative_to(repo_root)` ValueError
check, so an encoded `..%2F..%2Fetc%2Fpasswd` decodes through `..`
detection. **Survives.**

### C2: "Mutation gate works"

POST without `--allow-mutations` returns 405. I confirmed by reading
`_handle_workflow_edit_save`: the very first guard checks
`self.runner.allow_mutations`. Test
`test_post_no_mutations_405` exercises it. I asked whether GET could
also write — it can't; `_render_workflow_edit_page` reads only.
**Survives.**

### C3: "Validation refusal returns 422 with file unchanged"

The handler writes to `<path>.tmp` only after `validate_workflow`
succeeds. On WorkflowError, no `.tmp` is created and the original
file is untouched. Test
`test_post_atomic_file_unchanged_on_failure` asserts both: response
is 422 and file content is byte-identical pre/post. I checked that
the validator runs *before* any filesystem write; it does.
**Survives.**

### C4: "1 MB body cap (F1)"

`Content-Length` is read first; if > 1 MB the handler returns 413
without reading the body. If `Content-Length` is missing, `read()`
is called with the cap as a ceiling — so even a chunked body is
bounded. I considered a slowloris-style malicious client that would
keep the socket open: stdlib `BaseHTTPRequestHandler` already has a
default `timeout`, and the cap caps memory use either way.
**Survives** — though I note the loop-iteration-cap pattern from RFC
0023 V1.5 is a different defense; this one is just "don't OOM."

### C5: "Non-`application/json` Content-Type returns 415 (F1)"

`_get_content_type` strips the parameter (e.g. `; charset=utf-8`)
and lowercases. Test `test_post_wrong_content_type_415` confirms
`text/plain` is refused. I tried to bypass by sending no
Content-Type at all: the handler treats missing header as not-json
and refuses — defensive. **Survives.**

### C6: "Atomic write via .tmp + rename"

`tmp = target.with_suffix(target.suffix + ".tmp")`, write via
`tmp.write_text`, then `tmp.replace(target)`. POSIX `rename` is
atomic on the same filesystem. I considered: what if the directory
doesn't exist for a new path? The handler `mkdir(parents=True,
exist_ok=True)` first. Test
`test_post_creates_intermediate_dirs` covers it. **Survives.**

### C7: "JS island redirects on success, banners on failure"

The save() function does `fetch(... POST ...)`; on 200 it
`window.location.href = "/workflows/" + relPath`; on non-200 it
parses JSON and surfaces the error message. I asked: what if the
network fails mid-save? The fetch promise rejects and no `.catch`
is wired — the user sees an unstyled error. **Note for V2**, not
acceptance-blocking; the localStorage backup retains the draft so
the user can refresh and try again.

### C8: "localStorage draft recovery"

On every state mutation, `persistDraft()` writes JSON to
`localStorage[draft-key]`. On page load, `checkDraftRecovery()`
prompts if the localStorage draft differs from disk. I asked:
what if the user has multiple tabs open editing the same path? Last
writer wins both for localStorage and for disk; an obscure
concurrent-tab edit could overwrite. Same as the disk concurrency
caveat — V2 If-Match is the answer. **Acceptable for V1.5.**

### C9: "CSP unchanged"

I diffed `base.html` and confirmed no new `<meta>` CSP, no inline
`<script>` blocks (only one `<script>` tag pointing at
`/static/workflow_edit.js`), no `<style>` blocks (CSS appended to
`base.css`). The `<script id="workflow-data" type="application/json">`
is JSON, not JS, and CSP doesn't constrain JSON inlining.
**Survives.**

### C10: "Test coverage adequate"

14 new tests + 18 existing pass; full suite 452/452. I asked: is
there a test for the empty-scaffold case where `workflow_id` is
derived from the path stem? Yes —
`test_get_nonexistent_scaffolds_empty` covers it. Is there a test
for the form rendering invalid JSON? Yes —
`test_get_invalid_workflow_opens_editor`. **Survives.**

## Counter-claims that hold

I cannot defeat any acceptance-blocking claim. F1 (body cap +
content-type) is addressed; F2 (new-vs-existing affordance) is
addressed; deferrals (drag-and-drop, templates, run-now,
field-level errors, If-Match, AI scaffolding) are documented in
CHANGELOG and BUILD_HANDOFF.

## Verdict

**accept**

The build survives the strongest devil's-advocate counterarguments
I can raise. The two design-review findings are addressed; the V2
deferral list is honest about what's not yet shipped. Smoke against
the live tailnet bridge confirms the editor round-trips end-to-end.
