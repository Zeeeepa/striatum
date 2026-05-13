---
schema_version: striatum.handoff.v1
artifact_kind: handoff
title: "Dogfood-050 — RFC 0043 V1.5 Build Handoff (Claude lane)"
---
author: implementer-unknown-model-001

# Dogfood-050 — RFC 0043 V1.5 Build Handoff

Claude-lane implementation of the RFC 0043 V1.5 follow-up findings
folded under D102 (`dec_0b953435368e40109e793378e1a75054`,
accepted_with_follow_up, dogfood-048). The lane assignment was
deliberately NOT codex — D102 already flagged the 5th codex/codex
implementer-and-reviewer pairing risk, so the V1.5 implementation
sits in the Python codebase under the claude lane to keep the
reviewer surface independent.

Synthesis: `docs/dogfood/050/DESIGN_SYNTHESIS.md`
(`designer-unknown-model-002`).
Branch: `striatum/dogfood-050-rfc-0043-v1-5`.

## Scope shipped (F1–F4 from synthesis)

- **F-parser** — `daemon migrate-repo-local` is wired end-to-end:
  - `src/striatum/cli/parser.py` adds the subparser under
    `daemon_sub` immediately after the existing `daemon migrate`
    block. The flags exposed match the synthesis vocabulary
    verbatim: `--from {sqlite}`, `--to {pg}`, `--repo` (with
    `dest=repo_local_repo` so the top-level `--repo` still wins
    if the subparser flag is omitted), `--postgres-url`,
    `--dry-run`, `--keep-sqlite-readonly` (default `True`),
    `--no-keep-sqlite-readonly`, `--confirm-delete`, and
    `--json`.
  - `src/striatum/cli/dispatch.py::_dispatch_daemon` gets the
    first dispatch arm that imports
    `striatum.cli.daemon:dispatch_daemon` and forwards the
    parsed `Namespace`. This is the only edit inside
    `_dispatch_daemon`; the existing `migrate`, `start`,
    `doctor`, `status`, `stop`, `health`, `audit`, `sweep`
    arms are untouched.
  - `src/striatum/cli/daemon.py` resolves the repo argument
    with the top-level fallback per synthesis:
    `repo_arg = getattr(args, "repo_local_repo", None) or
    getattr(args, "repo", None)`. The existing
    `from_substrate / to_substrate` validation and PostgreSQL
    URL resolution are unchanged.

- **F-escape** — daemon-required is the default; the env var is
  the explicit opt-out:
  - `src/striatum/cli/daemon_required.py:resolve_requirement`
    now returns `None` only when *command* is on
    `DAEMON_OPTIONAL_COMMANDS` or
    `STRIATUM_DAEMON_REQUIRED == "0"`. Any other env value
    (including unset) returns a populated
    `DaemonRequirement(enforced=True, …)`.
  - Module docstring rewritten to describe the V1.5 default
    posture; the comment in `src/striatum/cli/dispatch.py` is
    rewritten so it no longer claims "default off."
  - CLI escape-path audit (per synthesis) confirmed the
    top-level `enforce_daemon_required()` gate is the only
    silent-fallback gate. The legacy SQLite implementations in
    `mutations.py`, `introspect.py`, `recovery.py`,
    `worktree.py`, `run_summary.py`, and `evidence.py` are
    reached only after `dispatch()` calls
    `enforce_daemon_required(args.command, repo)`.

- **F-test** — end-to-end exit-code-12 coverage in
  `tests/exit_codes/test_rfc0043_refusals.py`:
  - The autouse fixture still clears the env at function scope
    so each test exercises the new mandatory enforcement (the
    session-level conftest opt-out is overridden per-test).
  - `test_dispatch_returns_exit_12_for_unmigrated_repo` binds a
    Unix socket so the daemon-reachability check passes, drops
    an empty `.striatum/state.sqlite3` to present the
    pre-cutover disk signal, then asserts
    `dispatch_mod.main(["--repo", str(tmp_path), "status"])`
    returns `12`, that stderr contains `repo_not_migrated`,
    and that the remediation line names
    `striatum daemon migrate-repo-local --from sqlite --to pg
    --repo` with the resolved repo path.
  - `test_dispatch_exit_12_json_envelope` asserts the
    `--json` envelope shape and the structured `hint` naming
    the migrate command.
  - `test_resolve_requirement_returns_none_without_env` was
    inverted into
    `test_resolve_requirement_enforces_by_default_when_env_unset`
    plus a new `test_resolve_requirement_opt_out_with_env_zero`,
    per synthesis.
  - The existing `STRIATUM_DAEMON_REQUIRED=1` setenvs in the
    other tests were removed where the default flip makes them
    redundant.

- **F-crash** — sentinel-based crash-resume in
  `src/striatum/daemon_pg/repo_local_migration.py`:
  - `migrate_repo_local()` now computes
    `sentinel_path = repo / ".striatum" / "state.sqlite3.migrated"`
    and threads it through both `_migrate_full()` and the
    SQLite-missing early-return.
  - `_migrate_full()` writes the sentinel atomically (a `*.tmp`
    file + `os.fsync` + `replace`) between `pg_conn.commit()` and
    `_tombstone_or_delete_state_db()`. The sentinel JSON
    contains `repository_id`, `source_state_db_sha256`,
    `keep_sqlite_readonly`, `confirm_delete`, and `written_at`.
    After the tombstone/delete returns, the sentinel is cleared.
  - New helper
    `_resume_sqlite_finalization_after_checkpoint()` (called
    from both `already_migrated` early-return paths) verifies
    the source SHA against the checkpoint, resumes the
    sentinel-recorded action, and clears the sentinel. SHA
    mismatch raises `StriatumError(exit_code=8)` so the operator
    can investigate without losing data. Orphan sentinels are
    cleared idempotently. Fully-finalized state returns `None`.

## Files touched

Production sources (under `src/striatum/`):

- `src/striatum/cli/parser.py` — added the `migrate-repo-local`
  subparser block.
- `src/striatum/cli/dispatch.py` — added the dispatch arm in
  `_dispatch_daemon` and rewrote the top-level enforcement
  comment to describe the new default.
- `src/striatum/cli/daemon.py` — added the top-level `--repo`
  fallback in the repo-arg resolution.
- `src/striatum/cli/daemon_required.py` — flipped
  `resolve_requirement` to default-on; rewrote module docstring.
- `src/striatum/daemon_pg/repo_local_migration.py` — added
  sentinel + resume helper; `_migrate_full` signature gains
  `sentinel_path`; `migrate_repo_local` calls the resume helper
  from both `already_migrated` early-return paths. Added
  `import os`.

Tests (under `tests/`):

- `tests/conftest.py` — session-level autouse fixture that
  sets `STRIATUM_DAEMON_REQUIRED=0` so the existing SQLite-backed
  fixtures keep working under the flipped default. Tests that
  exercise the daemon-required surface (the
  `tests/exit_codes/test_rfc0043_refusals.py` set) override
  the env at function scope.
- `tests/exit_codes/test_rfc0043_refusals.py` — F-test changes
  (see above).
- `tests/daemon_pg/test_repo_local_migration_crash_resume.py`
  (new) — pure-Python helper coverage plus two
  `@pytest.mark.multi_repo` end-to-end tests that simulate the
  mid-finalization crash and the post-checkpoint corruption
  refusal.

Docs:

- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md`
  — appended a `## V1.5 deltas` section summarizing F-parser,
  F-escape, F-test, and F-crash. V1 sections are untouched.

Per the implement prompt's "no doc cascade" rule, no README /
TODO / CHANGELOG / RFC-index / SPEC / HOW_TO updates are
included; the operator handles those manually after the dogfood
lands.

## Deviations from the synthesis (each with one-line rationale)

- **Subparser help text wording.** Synthesis suggested
  "migrate one repo-local .striatum/state.sqlite3 into daemon
  PostgreSQL state"; shipped verbatim.
- **`_resume_sqlite_finalization_after_checkpoint` signature.**
  Matches the synthesis-locked positional + keyword shape; no
  `dry_run` parameter (dry-run is filtered upstream in
  `migrate_repo_local` before the helper is invoked, keeping
  the helper's contract simple).
- **`tests/conftest.py` session-level opt-out fixture.** Not
  named in the synthesis, but the synthesis explicitly preserves
  `STRIATUM_DAEMON_REQUIRED=0` as the opt-out for the existing
  SQLite-backed fixtures rather than rewriting them. A
  session-level conftest fixture is the least-invasive way to
  exercise the opt-out for the legacy test suites; per-test
  fixtures in `tests/exit_codes/test_rfc0043_refusals.py`
  override the env at function scope so the new default is
  still exercised end-to-end.
- **`written_at` field in sentinel payload.** Synthesis named
  `repository_id`, `source_state_db_sha256`,
  `keep_sqlite_readonly`, and `confirm_delete`; added
  `written_at` for operator-side audit trail when a sentinel is
  inspected on disk.
- **No `make test-rfc0043` target added.** The synthesis did
  not name one (the implement prompt's `Makefile` rule says
  "only if synthesis adds a new `make test-*` target"). Skipped.

## Verification

**Lint / typecheck / test status:** NOT EXECUTED inside this
supervised invocation. The operator's permission configuration
declined approval for every `Bash` tool call across the run,
including `striatum ack`, `make lint`, `make typecheck`,
`make test`, and the direct
`python -m striatum.cli daemon migrate-repo-local --help`
smoke command. The implement-prompt escape hatch ("If
`striatum ack` is denied, write the HANDOFF and exit normally")
governed the rest of the session.

Repository-side replay should run:

```bash
make lint
make typecheck
make test
# F-crash regression — requires STRIATUM_PG_TEST_URL:
pytest tests/daemon_pg/test_repo_local_migration_crash_resume.py
# F-test e2e (no PG required):
pytest tests/exit_codes/test_rfc0043_refusals.py
# F-parser smoke:
python -m striatum.cli daemon migrate-repo-local --help
```

**Expected F-crash regression behavior.** On un-fixed V1 code,
`test_rerun_after_crashed_tombstone_finishes_finalization` fails
because the un-fixed `_migrate_full` does not write or consult a
sentinel: the simulated crash leaves the source on disk, the
rerun's `already_migrated` branch returns without finalizing,
and the test's `tombstone.exists()` / `not source.exists()` /
`resumed_from_checkpoint is True` assertions all fail. On V1.5
code, the sentinel is written between the Postgres commit and
the tombstone, the resume helper picks it up on rerun, and the
test passes. The pure-Python helper tests
(`test_sentinel_roundtrip_…`,
`test_resume_finalizes_tombstone_when_source_matches`,
`test_resume_refuses_on_sha_mismatch_without_deleting`,
`test_resume_clears_orphan_sentinel`,
`test_resume_returns_none_when_already_finalized`) exercise the
same surface without requiring a system Postgres URL and should
pass on the V1.5 code unconditionally; they fail on V1 because
`_resume_sqlite_finalization_after_checkpoint` does not exist
yet.

**Expected F-test e2e behavior.** Both
`test_dispatch_returns_exit_12_for_unmigrated_repo` and
`test_dispatch_exit_12_json_envelope` rely on the V1.5 default
flip — they don't `setenv("STRIATUM_DAEMON_REQUIRED", "1")`.
On V1 code they would fail because `resolve_requirement` returns
`None` and `dispatch_mod.main(...)` would not raise the
`RepoNotMigratedError` (it would hit the SQLite path and likely
exit with a different code). On V1.5 code the flipped default
plus the test-level env clear yield the expected exit-12.

## Backward-compatibility constraints honored

- **No Postgres schema changes.** No new file under
  `src/striatum/daemon_pg/sql/`; `LATEST_DAEMON_DB_VERSION`
  remains `5`. The V1.5 fix is schema-additive in spirit but
  needs no schema delta.
- **`--keep-sqlite-readonly` tombstone preserved.** The
  rename-to-`.tombstone` + `chmod 0444` path is unchanged in
  the normal migration, the resumed migration, dry-run, and
  already-migrated paths. The crash-resume regression test
  asserts the 0444 mode bit explicitly.
- **No destructive index changes.** None.

## Downstream consequences

- Operators upgrading from V1 to V1.5 must either:
  - Unset `STRIATUM_DAEMON_REQUIRED` and bring up `striatumd`
    before invoking non-lifecycle verbs (recommended), OR
  - Set `STRIATUM_DAEMON_REQUIRED=0` to keep the SQLite-backed
    fallback while they migrate their environments.
  The exit-11 (`daemon_unreachable`) and exit-12
  (`repo_not_migrated`) stderr blocks plus the `--json` `hint`
  field point at the exact remediation commands.
- The five-time codex/codex anti-pattern remains the explicit
  reason this implementation sat in the claude lane. Any
  follow-up review should preserve the implementer/reviewer
  lane independence.
- The operator's "in-flight clients" guarantee (RFC 0030
  dotted vocabulary + deprecated undotted aliases) is
  unaffected.
- RFC 0039 Phase 2 remains unblocked.
