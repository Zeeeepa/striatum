# Synthesis Prompt: RFC 0043 V1.5 (close D102 follow-up findings)

Produce `docs/dogfood/050/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/050/design/codex/DESIGN.md", "docs/dogfood/050/design/claude_code/DESIGN.md", "docs/dogfood/050/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 designs into ONE concrete plan for RFC 0043 V1.5. Choose; do not enumerate. If the three designs disagree, pick one and justify in one sentence.

- **F-crash crash-recovery shape**: pick (a) transactional rollback OR (b) checkpointed resume. Name the exact function signature change in `src/striatum/daemon_pg/repo_local_migration.py` (e.g. `_migrate_full(...) -> MigrationResult` with new sentinel path argument). Name the sentinel file path (e.g. `.striatum/state.sqlite3.migrated`) if (b). Name the lock primitive if (a). Name the regression test path (one file under `tests/daemon_pg/`).
- **F-escape CLI default-flip**: locked default behaviour — daemon-required is on by default. Does `STRIATUM_DAEMON_REQUIRED=0` opt OUT (for tests/CI only) or is the env var removed entirely? Pick one. Name the exact line change in `src/striatum/cli/daemon_required.py:resolve_requirement`. Enumerate the audit list of any other silent-SQLite-fallback paths discovered in the cli/ tree (or state "none beyond the one named").
- **F-parser subparser wiring**: exact subparser block in `src/striatum/cli/parser.py` (function name, argument list, help text). Exact dispatch arm in `src/striatum/cli/dispatch.py::_dispatch_daemon` (one elif). Smoke command operator runs to verify (`striatum daemon migrate-repo-local --help`).
- **F-test end-to-end exit-12**: locked test path (`tests/exit_codes/test_rfc0043_refusals.py` or `tests/cli/test_unmigrated_repo_refusal.py` — pick one). Fixture shape (use an existing `tests/fixtures/v1_repo_local_sqlite/` if present, else describe creation). Assertion shape (`SystemExit` vs `pytest.raises` vs `capsys.readouterr().err contains`).

Order the four items in implementation order (which gates which). Identify any cross-finding dependencies (e.g. does the parser wiring need to land before the exit-12 test? Does the default-flip break the existing test suite — what shim is needed?).

Backward-compat lock:
- Schema changes additive only — call out any new SQL file under `src/striatum/daemon_pg/sql/`.
- `--keep-sqlite-readonly` tombstone semantics preserved under every code path the V1.5 change touches.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
