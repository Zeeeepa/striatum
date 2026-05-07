# Draft prompt — Land HARNESS-001/002/003/004 fixes

## Task

Land the recommended fixes from the four dogfood-001 harness improvement
proposals. The goal is *not* to ship every sub-point of every proposal —
it is to land the cheap, high-signal layer of each one so the next
dogfood (002 or 001 v3) can drive a supervised workflow without falling
back to operator-driven recovery.

## Context to read first

Required:

- `docs/dogfood/001/SYNTHESIS.md` — what dogfood-001 found and why each
  fix matters.
- `docs/dogfood/001/findings/HARNESS-001.md` — supervised lane gap.
- `docs/dogfood/001/findings/HARNESS-002.md` — editable-install pin.
- `docs/dogfood/001/findings/HARNESS-003.md` — independence and byline.
- `docs/dogfood/001/review/HARNESS-004.md` — reviewer scope/doc
  contradiction.

Likely useful:

- `src/striatum/doctor.py` — current `doctor` checks; HARNESS-001/002/003
  recommend new warnings here.
- `src/striatum/db.py` — `init_repo` and `connect`; HARNESS-002 recommends
  a guard in `init`.
- `src/striatum/cli/parser.py` — adding a new `--force-non-fresh` flag
  to `register-session`.
- `src/striatum/cli/dispatch.py` — wiring the new flag and the new
  `init` guard's error path.
- `src/striatum/sessions.py` (or wherever `register_session` lives) —
  adding the parent-pid policy.
- `src/striatum/artifacts.py` — byline-missing recording for HARNESS-003.
- `src/striatum/supervise.py` — surfacing dead-supervisor-with-held-lease
  as a `next_action`.
- `Makefile` — install-path resolution for HARNESS-002.
- `tests/test_cli_mvp.py`, `tests/test_supervise.py` — extend with a
  test per fix.

## Scope per HARNESS

### HARNESS-001 (defaults) — supervised lane

In scope for this draft:

- **Documentation.** Add a "Supervised lane command contract"
  subsection to `docs/SPEC.md`. It must state: command must stay alive
  across packets, must read newline-delimited JSON packets from stdin,
  must call back via `striatum` CLI for ack/heartbeat/publish/complete.
  Cite RFC 0009.
- **Doctor warning.** When a run has a `process_supervisors` row in
  state `lost` or `gone` and a held lease whose `expires_at` is in the
  future, emit a `doctor` problem record:
  `supervisor_lost_with_held_lease`, with `run_id`, `session_id`,
  `lease_id`, and `lease_expires_at` in `context`.
- **Status next_action.** When the same condition holds, surface a
  `next_action` in `striatum status --run-id <id> --json`
  recommending `striatum supervise stop` followed by the appropriate
  recovery command.
- **`supervise stop` tolerance.** `supervise stop` against a session
  whose supervisor record is in state `lost` or `gone` should succeed
  with `state: "stopped"` and `note: "supervisor was already lost"`,
  not exit 4. Idempotent stop is the right shape.

Out of scope for v2 (defer to a future dogfood after RFC 0010 lands):

- Shipping a working long-running supervised lane invocation. That
  requires the agent CLI to know the Striatum protocol, which is
  RFC 0010 territory (PTY supervisor + protocol skill).

### HARNESS-002 (defaults) — editable install foot-gun

In scope:

- **Doctor warning.** Compare `striatum.__file__` against the resolved
  repo argument. If the installed package is outside the repo, emit a
  problem record `editable_install_outside_repo` with both paths in
  context. Not an error; just a warning.
- **`init` guard.** When initializing a fresh state DB (no existing
  `.striatum/state.sqlite3`), compute the repo's source-tree
  `LATEST_VERSION` (read `src/striatum/migrations.py` from the repo
  argument, not from the running install). If the running install's
  `LATEST_VERSION` is lower, refuse with exit 3 and a clear message
  pointing at `pip install -e <repo>`.
- **Makefile resolution.** Change `install:` target to use
  `$(MAKEFILE_DIR)` (or equivalent) so `pip install -e <resolved>` is
  not cwd-dependent, and print the resolved path it used.

### HARNESS-003 (spec) — independence and byline

In scope:

- **SPEC text.** Add a "Reviewer Independence" subsection to
  `docs/SPEC.md` stating plainly that `fresh_session_required` and
  `reviewer_context_policy: fresh` are advisory at the runner level
  beyond session-id distinctness. List the threats and operator
  obligations.
- **Doctor warning (sibling sessions).** When two active sessions on
  the same run share a supervisor `pid` (or the reviewer registered
  with no supervisor on a run where the author had one), emit a
  problem record `reviewer_independence_unverified`.
- **`register-session --force-non-fresh --reason`.** When the targeted
  job declares `reviewer_context_policy: fresh`, refuse to register
  unless `--force-non-fresh` is passed with a non-empty `--reason`.
  Record the reason on the session row (new column, `non_fresh_reason
  TEXT`, NULL when not used). Pure session-id distinctness alone is
  not sufficient.
- **Byline integrity.** When the work packet declares an
  `author_line` and the published artifact has no `author:` line,
  record `author_line` as `null` (or `"missing"`) in the snapshot,
  not the workflow's declared expected byline. Today the snapshot
  silently records the declared byline regardless of file content;
  the run summary lies about who reviewed.

Out of scope (deliberately deferred):

- Hard parent-pid refusal even with `--force-non-fresh`. Doctor
  warning is enough for v2.
- Lane-id-to-byline anchor. Needs a richer `display_model` mapping
  and is not the cheap layer.

### HARNESS-004 (documentation) — reviewer doc contradicts scope

In scope:

- Update `docs/dogfood/001/roles/reviewer.md` to point harness
  proposals at `docs/dogfood/001/review/HARNESS-NNN.md` (the path
  the existing write_scope already allows).
- Audit any other reviewer role doc under `docs/dogfood/*/roles/`
  for the same contradiction; fix in place.
- Add a brief note in `docs/SPEC.md` "Reviewer scope" subsection (or
  the existing review-policy section) that reviewer harness
  proposals belong inside the reviewer's `allowed_paths`, not the
  author's findings directory.

## Tests

For each fix, add a focused test:

1. `test_doctor_flags_supervisor_lost_with_held_lease` — fabricate a
   `process_supervisors` row in state `lost` with an active lease,
   assert `doctor` reports the new check.
2. `test_doctor_flags_editable_install_outside_repo` — patch
   `striatum.__file__` so it points outside `tmp_path`, assert the
   new check fires.
3. `test_init_refuses_when_install_lags_repo_migrations` — create a
   fake `migrations.py` in `tmp_path/src/striatum/` with a higher
   `LATEST_VERSION`, assert `init --repo tmp_path` exits 3.
4. `test_supervise_stop_is_idempotent_when_supervisor_already_lost`
   — mark the supervisor row `lost`, call `supervise stop`, assert
   exit 0 with the new `note`.
5. `test_register_session_refuses_fresh_reviewer_without_force` —
   workflow declares `reviewer_context_policy: fresh`, register
   without `--force-non-fresh`, expect refusal; with the flag and
   `--reason "..."`, succeed; row carries the reason.
6. `test_publish_artifact_records_missing_author_line` — workflow
   packet declares `author_line`, file omits it, snapshot records
   `author.line` as `null`/missing.
7. `test_reviewer_role_doc_paths_match_write_scope` — walk
   `docs/dogfood/*/roles/reviewer.md`, parse any "file under <path>"
   instructions, assert each is contained in the corresponding
   review job's `write_scope.allowed_paths`.

## Acceptance

- `make lint typecheck test` passes (current baseline 143 → at least
  150 after the new tests).
- `striatum doctor --verbose --json` shows the three new check ids
  when their preconditions hold.
- `striatum supervise stop` is idempotent against a lost supervisor.
- `striatum register-session --role reviewer` refuses when the job
  declares `fresh` and the `--force-non-fresh` flag is not present.
- `docs/dogfood/001/roles/reviewer.md` no longer points at a path
  outside the review job's `write_scope`.
- `docs/SPEC.md` has the two new subsections (supervised lane
  contract, reviewer independence advisory).

## Handoff

Write `docs/dogfood/001-v2/DRAFT_HANDOFF.md` summarizing:

- Files changed (paths only).
- Test count before/after.
- Per-HARNESS: which sub-points landed, which were deferred, and why.
- Open questions for the reviewer.
- Any new harness friction surfaced during the work
  (cross-link `docs/dogfood/001-v2/findings/HARNESS-NNN.md` if you
  filed any).

Then publish the handoff (kind `handoff`, logical_name
`draft_handoff`) and call `striatum complete`.
