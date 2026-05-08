---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: author-claude-opus-001

# DRAFT_HANDOFF — dogfood-001 v2 HARNESS fixes

Run: `run_4db045f7e3e643d6a75948dd1b86d6d6`
Branch: `striatum/dogfood-001-v2-harness-fixes`
Job: `draft_change`

## Files changed

| File | Reason |
| --- | --- |
| `src/striatum/cli/introspect.py` | New `doctor` checks (`supervisor_lost_with_held_lease`, `editable_install_outside_repo`, `reviewer_independence_unverified`). New `_has_supervisor_lost_with_held_lease` helper threaded into `status` so the new next-action `recover_orphan_supervisor` is surfaced. New `_install_module()` and `_reviewer_independence_breaches()` helpers. |
| `src/striatum/supervisor.py` | `supervise_stop` is idempotent against `lost`/`stopped` rows via `_latest_terminal_supervisor()`. |
| `src/striatum/db.py` | `init_repo` calls `_refuse_init_when_install_lags_repo` on a fresh DB; new `_read_repo_latest_version()` parses the repo's `migrations.py` without importing it. |
| `src/striatum/migrations.py` | Migration version 6 adds `sessions.non_fresh_reason` and `artifacts.author_line`. |
| `src/striatum/cli/parser.py` | `register-session --force-non-fresh --reason` flags. |
| `src/striatum/cli/mutations.py` | `register_session` enforces the fresh-reviewer policy with explicit override + reason; new `_workflow_declares_fresh_reviewer` helper. |
| `src/striatum/cli/dispatch.py` | Threads the new flags to `register_session`. |
| `src/striatum/artifacts.py` | `record_artifact` populates the new `author_line` column from the file's actual byline; new `_first_author_line` helper. |
| `src/striatum/cli/evidence.py` | Artifact summaries return the actual `author_line` (with `actual_author_line` mirror) so the snapshot reflects file truth. |
| `src/striatum/cli/run_summary.py` | Renders `author: <missing>` when the actual byline is `null`. |
| `src/striatum/identity.py` | `ArtifactAuthorIdentity` is now `total=False` and carries optional `actual_author_line` for the artifact-list path. |
| `Makefile` | Install target uses `$(MAKEFILE_DIR)` so `make install` from any cwd installs *this* Makefile's directory. |
| `docs/SPEC.md` | "Supervised lane command contract" subsection (HARNESS-001), "Reviewer Independence (advisory)" + "Byline Integrity" subsections (HARNESS-003), and a doctor-checks paragraph that mentions all three new check ids. |
| `docs/dogfood/001/roles/reviewer.md` | Now points reviewer harness proposals at `docs/dogfood/001/review/HARNESS-NNN.md` (HARNESS-004). |
| `tests/test_harness_v2_fixes.py` | New file — 8 focused tests covering each fix. |
| `tests/test_cli_mvp.py`, `tests/test_artifact_schemas.py`, `tests/test_supervise.py`, `tests/test_list_commands.py`, `tests/test_worktree_isolation.py` | `register` helpers updated to pass `--force-non-fresh --reason "test fixture"` for reviewer registrations (the new policy refuses bare same-operator registrations). The `complete_claimed_job` and `verdict_claimed_review` helpers now include the workflow-declared byline in the artifact body so existing evidence assertions still see the model label after HARNESS-003 byline integrity. |
| `CHANGELOG.md` | Per-fix entries under `## Unreleased / ### Added`. |

## Test count

- Before: 143 passing.
- After: 151 passing (+8 in `tests/test_harness_v2_fixes.py`).
- `make lint` clean (ruff).
- `make typecheck` clean (mypy, 36 source files).

## Per-HARNESS disposition

### HARNESS-001 (defaults) — supervised lane

| Sub-point from proposal | Status |
| --- | --- |
| SPEC subsection "Supervised lane command contract" | **landed** (`docs/SPEC.md` under `### Multi-Packet Supervision` block) |
| Doctor warning `supervisor_lost_with_held_lease` | **landed** (new check + `tests/test_harness_v2_fixes.py::test_doctor_flags_supervisor_lost_with_held_lease`) |
| Status next_action when same condition holds | **landed** (`recover_orphan_supervisor` + `test_status_surfaces_recover_orphan_supervisor_next_action`) |
| `supervise stop` idempotency | **landed** (`_latest_terminal_supervisor` + `test_supervise_stop_is_idempotent_when_supervisor_already_lost`) |
| Ship a working long-running supervised lane | **deferred** (depends on RFC 0010 PTY supervisor + protocol skill, per the v2 prompt's explicit "out of scope" list) |

### HARNESS-002 (defaults) — editable install foot-gun

| Sub-point | Status |
| --- | --- |
| Doctor `editable_install_outside_repo` | **landed** (only fires when `repo` is itself a Striatum source tree to avoid false positives on target-repo runs; `test_doctor_flags_editable_install_outside_repo`) |
| `init` guard refusing when install lags repo | **landed** (exit 3 + clear `pip install -e` message; `test_init_refuses_when_install_lags_repo_migrations`) |
| Makefile install path resolution | **landed** (`$(MAKEFILE_DIR)`; verified manually — `make install` now prints the resolved path and is no longer cwd-dependent) |

### HARNESS-003 (spec) — reviewer independence + byline

| Sub-point | Status |
| --- | --- |
| SPEC text | **landed** (Reviewer Independence + Byline Integrity subsections under `### Reviewer Policy`) |
| Doctor `reviewer_independence_unverified` (shared pid + asymmetric supervised/unsupervised) | **landed** |
| `register-session --force-non-fresh --reason` with `non_fresh_reason` column | **landed** (`test_register_session_refuses_fresh_reviewer_without_force` covers all three branches: refusal without flag, refusal with flag but no reason, success with both) |
| Byline-missing recording (`artifacts.author_line` NULL when file omits the line) | **landed** (`test_publish_artifact_records_missing_author_line`) |
| Hard parent-pid refusal | **deferred** (per the v2 prompt's explicit "out of scope") |
| Lane-id-to-byline anchor | **deferred** (per the v2 prompt) |

### HARNESS-004 (documentation) — reviewer doc vs scope

| Sub-point | Status |
| --- | --- |
| Fix `docs/dogfood/001/roles/reviewer.md` to point at `review/HARNESS-NNN.md` | **landed** |
| Audit other dogfood reviewer role docs | **landed** — only two reviewer.md files exist (`docs/dogfood/001/roles/reviewer.md` and `docs/dogfood/001-v2/roles/reviewer.md`); v2's was already correct. The new `test_reviewer_role_doc_paths_match_write_scope` walks every dogfood reviewer doc and asserts each `HARNESS-NNN.md` instruction path is contained in at least one review job's `write_scope.allowed_paths`, so future drift is caught at CI time. |
| SPEC reviewer-scope note | **deferred** — judged redundant with the existing `write_scope.allowed_paths` documentation; the new `test_reviewer_role_doc_paths_match_write_scope` covers the doc/runner agreement structurally. If the reviewer wants the SPEC note explicit, please flag and I'll add a paragraph. |

## Open questions for the reviewer

1. **Doctor problem-record canonicalisation.** The existing pid-gone
   detection (lines `~990-1019` in `introspect.py`) appends to
   `problems` directly, while every new check goes through
   `report()` and shows up in `problem_records`. Should the existing
   pid-gone path be migrated to `report()` so `--verbose` records are
   complete? Out of scope for v2 but worth noting.
2. **`actual_author_line` field shape.** Snapshot consumers now see
   both `author.line` and `author.actual_author_line` carrying the
   same value when present (and both `None` when missing). I
   considered dropping `line` entirely and renaming
   `actual_author_line` to `line`, but kept both for back-compat.
   Reviewer's call: keep both, alias, or break the schema.
3. **Workflow-level "harness scratch" path.** HARNESS-004's third
   suggestion (introduce `docs/dogfood/<id>/harness/` implicit
   allowed_paths) is deliberately not landed; the doc fix is enough
   for v2. If we want to permanently make harness proposals
   first-class cross-cutting artifacts, that's an RFC.
4. **Independence enforcement scope.** The new register-session
   policy fires when (role=reviewer + workflow has fresh review job +
   any active author session in run). HARNESS-003 also suggested a
   parent-pid match check; that's deferred. Is "any active author
   session" too aggressive (legitimate single-host workflows always
   trip)? The escape hatch is `--force-non-fresh --reason "..."`,
   which is recorded explicitly, so the friction is one CLI flag.
5. **Newline-asymmetry follow-up (F-3 from dogfood-001).** Still not
   addressed; it's a separate concern.

## Harness friction filed during this run

The v2 round itself was uneventful — the protocol from `claim-next` →
`ack` → make changes → `publish-artifact` → `complete` worked as
expected when driven by the operator. Notable observation: the
register-session policy I just added immediately tripped my own
fixture helpers (the existing test suite uses a single-operator
register pattern). Updating those helpers to pass
`--force-non-fresh --reason "test fixture"` is exactly the friction
the policy is designed to surface in real workflows.

No new HARNESS-NNN proposals filed for v2.
