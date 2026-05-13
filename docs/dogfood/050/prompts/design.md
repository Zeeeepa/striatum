# Design Prompt: RFC 0043 V1.5 (close D102 follow-up findings)

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/050/design/<lane>/`).

Design **RFC 0043 V1.5 deltas** closing the four follow-up findings recorded in `docs/dogfood/048/decisions/D102_cycle_exhaustion.md`. Read these first:

- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — current V1 spec.
- `docs/dogfood/048/decisions/D102_cycle_exhaustion.md` — the override that folded these findings to V1.5.
- `docs/dogfood/048/review/build/codex/REVIEW.md` — codex threat-model review (F1 RFC 0039 contradiction is for a different RFC; focus on the trust-boundary surfaces it names).
- `docs/dogfood/048/review/build/gemini/REVIEW.md` — gemini adversarial review (primary F2.1 / F2.2 / F2.3 source).
- `docs/dogfood/048/BUILD_HANDOFF.md` — the build context (Track A schema/migration body, Track B CLI surface).

Cover each finding concretely, with pinpoint citations to the existing source under `src/striatum/daemon_pg/` and `src/striatum/cli/`:

- **F-crash (CRITICAL) Crash-recovery persistence gap** — `_migrate_full` in `src/striatum/daemon_pg/repo_local_migration.py` commits Postgres state first, then tombstones the SQLite source. A kill -9 between the two steps leaves an active SQLite file alongside a `repo_migrations` row, and the `already_migrated: True` early-return on re-run does not re-attempt the tombstone (gemini REVIEW line 204 reference). Spec one of two shapes and justify:
  - (a) **Transactional rollback**: Postgres commit deferred until after SQLite is locked / renamed; on crash the Postgres transaction rolls back and the next run starts from scratch. Name the lock primitive (`fcntl.flock` vs `LOCK TABLE` vs a `.striatum/state.sqlite3.migrating` sentinel).
  - (b) **Checkpointed resume**: keep the Postgres-commit-first ordering, but add a sentinel (`.striatum/state.sqlite3.migrated`) written between the Postgres commit and the tombstone. The `already_migrated` re-run path inspects sentinel-vs-file state and finishes the tombstone idempotently.
  Cite the exact `migrate_repo_local` re-entry path and the `_tombstone_or_delete_state_db` helper. Spec the regression test that fails on the un-fixed code (simulate the crash via `_tombstone_or_delete_state_db` raising mid-call).
- **F-escape (MAJOR) CLI escape path closure** — `src/striatum/cli/daemon_required.py:resolve_requirement` is env-gated on `STRIATUM_DAEMON_REQUIRED=1`. Default behaviour silently falls through to the legacy SQLite path. RFC 0043 §3 requires daemon-required to be the default. Audit the cli/ tree for any other silent-SQLite-fallback paths. Cite the exact lines. Spec the default-flip: does the env var still exist as an opt-OUT for tests, or is it removed entirely? What happens to repos that have not yet run `migrate-repo-local` — does the CLI refuse with exit code 12 (and the user is told how to migrate), or does it auto-migrate silently? Pick one with one-sentence rationale.
- **F-parser (MODERATE) `migrate-repo-local` subparser wiring** — the verb is implemented in `src/striatum/daemon_pg/repo_local_migration.py` and `src/striatum/cli/daemon.py` but the subparser is not registered in `src/striatum/cli/parser.py`. The dispatch arm in `src/striatum/cli/dispatch.py::_dispatch_daemon` is not wired. Cite the existing `daemon` subparser construction. Spec the exact subparser block (argument names, help text, `--confirm-delete`, `--keep-sqlite-readonly`, `--dry-run`) and the dispatch arm. Verify `striatum daemon migrate-repo-local --help` will print the expected usage.
- **F-test (LOW) End-to-end exit-code-12 test gap** — claude's accept_with_findings noted no `dispatch.main(...)` end-to-end assertion for exit code 12 against a real unmigrated repo. Spec the exact test path under `tests/exit_codes/` or `tests/cli/`, the fixture shape (a `.striatum/state.sqlite3` at the pre-cutover schema version), and the assertion (`SystemExit.code == 12`, stderr contains the canonical remediation block). Optionally pair with a sibling exit-11 (daemon-unreachable) test if scope allows.

Backward-compat (hard constraint):
- Any Postgres schema change must be additive (new columns NULLable, new tables only). No `ALTER TABLE ... DROP COLUMN`, no destructive index changes.
- The `--keep-sqlite-readonly` tombstone path (rename to `.tombstone`, mode 0444) must keep working under every flag combination, including the new crash-recovery resume path.

Cite existing code (function names, line ranges where helpful). Hand-waving "we add a sentinel" without a pinpoint citation is grounds for design review to bounce.

Out of scope: RFC 0043 V2 work (hosted mode, multi-tenancy, bundled Postgres distribution), Go daemon work (RFC 0039 V1.6 has its own follow-up), doc updates beyond the RFC 0043 V1.5 deltas section and `build/HANDOFF.md`.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
