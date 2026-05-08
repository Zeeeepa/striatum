---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["dogfood-001-v2", "harness-fixes"]
---

# Review finding — dogfood-001 v2 HARNESS fixes

Run: `run_4db045f7e3e643d6a75948dd1b86d6d6`
Branch: `striatum/dogfood-001-v2-harness-fixes`
Job: `review_change` (lease `lease_a8bbe780a93748f68c59259ef1334a55`)

## Independence note

This run dogfoods HARNESS-003 in the most direct way possible: the
reviewer session was registered with `--force-non-fresh --reason
"operator drove both lanes; HARNESS-001 working supervised lane not
yet shipped; documented in HARNESS-003"`. The reason is durable on
the session row (`sessions.non_fresh_reason`), so the evidence export
will record the explicit breach instead of pretending a fresh codex
process drove the review. As before, this artifact intentionally
omits an `author:` byline so the new `author_line` integrity check
records `null` and `RUN_SUMMARY.md` will render `author: <missing>`
for it. That validates the byline-integrity work end-to-end.

## Gate disposition (12 gates from `prompts/review.md`)

### HARNESS-001

1. **SPEC contract subsection.** **Pass.** `docs/SPEC.md` lines
   ~561-587 add the "Supervised Lane Command Contract" subsection
   under the multi-packet supervision block. It names all three
   requirements (alive, NDJSON stdin, calls back via CLI) and links
   to dogfood-001's HARNESS-001 as the source.
2. **Doctor `supervisor_lost_with_held_lease`.** **Pass.**
   `tests/test_harness_v2_fixes.py::test_doctor_flags_supervisor_lost_with_held_lease`
   fabricates the precondition (a `lost` supervisor row + an active
   lease whose `expires_at` is in the future) and asserts the new
   problem record fires with the right `id` and `context`. Verified
   the query joins on `leases.owner_session_id` (not
   `leases.session_id`, which doesn't exist; that bug surfaced
   during draft and was caught by the existing `test_supervise.py`
   suite before publish).
3. **Status next_action.** **Pass.**
   `test_status_surfaces_recover_orphan_supervisor_next_action`
   asserts `recover_orphan_supervisor` appears in
   `status.next_actions` for the same precondition. The
   `_has_supervisor_lost_with_held_lease` helper is a focused
   precheck so `status` does not run the full doctor pipeline every
   call.
4. **`supervise stop` idempotency.** **Pass.**
   `test_supervise_stop_is_idempotent_when_supervisor_already_lost`
   verifies exit 0 plus `note: "supervisor was already lost"`. The
   `_latest_terminal_supervisor` helper correctly short-circuits
   only when no active supervisor row exists, so it doesn't shadow
   normal stop.

### HARNESS-002

5. **Doctor `editable_install_outside_repo`.** **Pass with
   finding (info).** `test_doctor_flags_editable_install_outside_repo`
   passes. The author wisely added a precondition: the check only
   fires when the repo argument is *itself* a Striatum source tree
   (carries `src/striatum/migrations.py`). Without that guard, every
   legitimate "use striatum CLI on a target repo" call would
   trigger the warning. **F-1**: the precondition is sound but it
   means the foot-gun the dogfood originally hit is only caught when
   the operator is dogfooding *inside* the Striatum repo. That's the
   common case for the thing we're trying to protect against, but
   worth noting that a parallel check for "running install is older
   than its own bundled migrations" would also help — i.e. `striatum
   doctor` could compare `striatum.migrations.LATEST_VERSION` to a
   freshly-imported copy from `striatum.__file__`'s parent. Defer.
6. **`init` guard.** **Pass.**
   `test_init_refuses_when_install_lags_repo_migrations` constructs
   a fake `migrations.py` with `LATEST_VERSION = 999` and asserts
   exit 3 with the `pip install -e` pointer. `_read_repo_latest_version`
   correctly avoids importing the file (which would re-import the
   running install) and parses both the static-int form and the
   dynamic `Migration(version=N)` form.
7. **Makefile install path.** **Pass with finding (info).**
   `MAKEFILE_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))`
   is the canonical pattern; the install rule now invokes
   `pip install -e "$(MAKEFILE_DIR)[dev]"` and prints the path. Not
   covered by an automated test (Makefile behaviour requires a real
   `make` invocation in a foreign cwd). **F-2**: a small
   `tests/test_makefile_install_resolves_path.py` that subprocesses
   `make install` from a tmp cwd and asserts the printed path is
   the repo root would close this. Non-blocking.

### HARNESS-003

8. **SPEC reviewer-independence subsection.** **Pass.** `docs/SPEC.md`
   `### Reviewer Policy` now has two subsections: "Reviewer
   Independence (advisory)" enumerating the two observable breaches,
   the `--force-non-fresh --reason` escape hatch, and the operator
   obligation; "Byline Integrity" describing the
   declared-vs-actual distinction. Plain language; no jargon.
9. **Doctor `reviewer_independence_unverified`.** **Pass with
   finding (info).** Detection logic covers (a) two active sessions
   sharing supervisor pid and (b) reviewer unsupervised + author
   supervised. **F-3**: there is no test exercising case (a) or (b)
   in `tests/test_harness_v2_fixes.py`. The other doctor checks each
   have a focused test; these two surfaces are detected by code
   review only. Not blocking — case (a) requires fabricating two
   `process_supervisors` rows with the same pid, case (b) requires
   one supervised row plus a sessionless reviewer; both are
   straightforward fixtures. Defer to a follow-up.
10. **`register-session` policy.** **Pass.**
    `test_register_session_refuses_fresh_reviewer_without_force`
    covers all three branches (refuse without flag, refuse with
    flag but no reason, succeed with both, reason on session row).
    `_workflow_declares_fresh_reviewer` correctly checks both
    `reviewer_context_policy: fresh` and the legacy
    `fresh_session_required: true` form.
11. **Byline integrity.** **Pass.**
    `test_publish_artifact_records_missing_author_line` confirms
    the `author_line` column is `NULL` and the run summary renders
    `author: <missing>`. The `_first_author_line` helper canonicalises
    casing/whitespace so two artifacts that differ only in
    `Author:` vs `author:` collide cleanly. **F-4 (low)**: the
    canonicalisation lowercases the *suffix* too (e.g. `author:
    Author-Codex-001` → `author: author-codex-001`). That's mostly
    fine because the synthesised bylines are already lowercase, but
    if a downstream renderer ever wants to display
    role-specific casing, the original is gone. Probably correct
    (privacy-safe bylines are deliberately lowercase per AGENTS.md),
    but worth noting.

### HARNESS-004

12. **Reviewer doc fix + audit.** **Pass.**
    `docs/dogfood/001/roles/reviewer.md` now points at
    `docs/dogfood/001/review/HARNESS-NNN.md`. The new
    `test_reviewer_role_doc_paths_match_write_scope` walks every
    dogfood reviewer role doc, finds each
    `docs/dogfood/<id>/.../HARNESS-NNN.md` path it references, and
    asserts each path is contained in at least one review job's
    `write_scope.allowed_paths`. The `path_paragraph` helper
    correctly excludes "the author-side path is /findings/" contrast
    sentences from the assertion. v2's reviewer.md was already
    correct so the test currently asserts on dogfood-001's fix.

## Cross-cutting findings

- **F-5 (info) — Existing pid-gone path bypasses `report()`.** Lines
  `~990-1019` of `introspect.py` use `problems.append(...)` directly
  for `supervisor pid is gone` and `supervisor stdin pipe missing`,
  so those signals don't appear in `--verbose problem_records`. The
  draft handoff acknowledges this and calls it out as out of scope.
  Agreed; it's a pre-existing inconsistency, not something v2
  introduced.

- **F-6 (info) — `actual_author_line` field shape.** Both
  `author.line` and `author.actual_author_line` carry the same value
  in artifact summaries (or both are `None`). The handoff asks
  whether to drop one. I'd keep both: `line` preserves the existing
  field name for back-compat consumers; `actual_author_line` makes
  the integrity intent explicit. The tiny duplication is worth the
  back-compat. **No action.**

- **F-7 (info) — Independence enforcement aggressiveness.** The new
  `register_session` policy refuses on "any active author session in
  run", not just same-parent-pid. That's broader than the proposal's
  text but matches the spirit (the runner cannot tell parent-pid
  trustworthily, so it conservatively refuses and offers the
  documented escape hatch). The cost is one CLI flag for legitimate
  multi-host workflows; the override is recorded explicitly.
  Agreed with the handoff's choice. Defer parent-pid-only refusal
  to a future iteration if it becomes a friction point.

## Parity / scope / docs check

- **Parity**: every promised sub-point per the v2 prompt's "in scope"
  lists is either landed or explicitly deferred per the prompt. No
  silent scope expansion.
- **Determinism**: the new doctor problem records and next-action
  string are stable (`supervisor_lost_with_held_lease`,
  `editable_install_outside_repo`, `reviewer_independence_unverified`,
  `recover_orphan_supervisor`), suitable for scripts to grep.
- **Test coverage**: 8 new tests; all 4 fix categories covered with
  at least one focused test. F-3 (independence doctor cases) and
  F-2 (Makefile path) are the missing arms; both are info-only.
- **Scope hygiene**: changes are within the v2 author write_scope
  (`src/striatum/`, `tests/`, `docs/SPEC.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/dogfood/001-v2/`,
  `docs/dogfood/001/roles/`, `Makefile`, `README.md`,
  `CHANGELOG.md`). Confirmed via `git diff --stat`. No
  `docs/UBIQUITOUS_LANGUAGE.md` or `README.md` changes were needed.
- **Doc currency**: SPEC has both new subsections, CHANGELOG has
  per-fix entries.

## Verdict

`accept_with_findings`.

All 12 explicit gates pass. The seven findings (F-1 through F-7) are
all `info` or `low` severity and none are blockers. The largest
deferred items — RFC-0010-shaped working supervised lane (HARNESS-001
sub-point), parent-pid hard refusal (HARNESS-003 sub-point) — were
explicitly out of v2 scope per the draft prompt, so they are correct
deferrals, not regressions. Tests rose 143 → 151; lint and typecheck
clean. The cross-cutting "scaffold says X, runner enforces Y"
pattern that motivated v2 is now backed by either runner refusal
(init guard, register-session policy), runner warning (the three
new doctor checks + status next_action), or doc/test enforcement
(reviewer.md path + the new doc-vs-scope test).
