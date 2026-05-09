# Design review: RFC 0021 V1.5

author: reviewer-claude-opus-001
date: 2026-05-09
verdict: accept

Devil's-advocate review of `DESIGN_SYNTHESIS.md`.

## Verdict

**accept** — V1.5 scope is tight, semantics are unambiguous, and
the status vocabulary is self-describing. No findings.

## Sweep

### 1. `--force` destructive overwrite

Counterargument: should `--force` only overwrite files that
still carry the RFC 0021 generation comment as their first
line, to avoid clobbering operator-edited stubs?

Survives? No — the synthesis is right. If an operator passes
`--force`, they've explicitly asked for the destructive
behavior. Adding a generation-comment check makes the flag
inconsistent: editing the comment line out of a stub would
silently disable `--force`. The `prior_sha256` field in the
envelope is the right safety net — operators can audit what
got overwritten.

The non-file-target carve-out (still `error`) is correct: that
class of "exists but isn't a file" likely indicates a
configuration problem the operator should resolve manually
before they ask the runner to overwrite.

### 2. `--dry-run` envelope shape

The `would_*` vocabulary is exhaustively partitioned from the
`{created, skipped, error, overwritten}` set. A consumer can
unambiguously branch on "is this a real run or a preview?" by
checking *either* the top-level `dry_run` flag *or* the per-row
status prefix. Belt-and-braces.

### 3. Composability semantics

`--force` + `--dry-run` → `would_overwrite`. Clean. The
"preview the destructive action" flow is the canonical
operator path; calling that out explicitly makes the synthesis
easier to verify.

### 4. Zero regression

V1 behavior preserved when both flags are False (their default).
The existing V1 tests assert byte-identical output; new V1.5
tests assert the new branches. Coverage looks complete.

### 5. Test plan

8 cases covering all four combinations (force off/on × dry-run
off/on), the non-file-target carve-out under force, the
envelope shape, and the CLI plumbing. Plus a guard test that
the V1.5 flags without `--with-ddd-layout` are no-ops.

### 6. Public API stability

`scaffold_ddd_layout(repo, *, force, dry_run)` keeps its V1
signature; V1's `force=False, dry_run=False` defaults match V1's
behavior. No deprecation. Tooling that imported the function
gets a strictly-additive set of behaviors.

## Decision

Accept V1.5 as written. The implementer can proceed against the
synthesis directly.

This run also exercises the RFC 0018 V1+step 3 stack again:
both reviews declare `review_posture: "devils_advocate"`, the
build job declares `required_review_postures:
["devils_advocate"]`, the workflow validator's reachability gate
accepted, and the verdict will record posture in the new
`verdicts.posture` column. Step 3's introspection will surface
the `devils_advocate=2` posture summary in the dashboard for
this run.
