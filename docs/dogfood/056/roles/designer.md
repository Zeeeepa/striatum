# Designer Role (Dogfood 050)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0043 V1.5 — the four follow-up findings recorded in
`docs/dogfood/048/decisions/D102_cycle_exhaustion.md`. Synthesis picks
one path and locks implementation order. Cite the existing code that
your design changes — do not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — current V1 spec.
- `docs/dogfood/048/decisions/D102_cycle_exhaustion.md` — the override.
- `docs/dogfood/048/review/build/codex/REVIEW.md` — codex threat-model review (trust-boundary framing).
- `docs/dogfood/048/review/build/gemini/REVIEW.md` — gemini adversarial review (PRIMARY source for F-crash / F-escape / F-parser).
- `docs/dogfood/048/BUILD_HANDOFF.md` — V1 build context.
- `src/striatum/daemon_pg/repo_local_migration.py` — F-crash surface. Look at `migrate_repo_local`, `_migrate_full`, `_tombstone_or_delete_state_db`, and the `already_migrated` early-return.
- `src/striatum/cli/daemon_required.py` — F-escape surface. Look at `resolve_requirement` and `enforce_daemon_required`.
- `src/striatum/cli/parser.py` — F-parser surface. Look at the `daemon` subparser construction.
- `src/striatum/cli/dispatch.py` — F-parser surface. Look at `_dispatch_daemon`.
- `src/striatum/cli/daemon.py` — F-parser helper.
- `src/striatum/errors.py` — `DaemonUnreachableError` (exit 11), `RepoNotMigratedError` (exit 12).
- `tests/exit_codes/test_rfc0043_refusals.py` (existing) — F-test surface; design says either extend this or add a sibling file.

Address each finding with: exact file path, exact function or symbol
touched, locked interface contract, and the test that proves the fix.

**Backward-compat note**: Postgres schema changes must be additive
(new tables, new NULLable columns). `--keep-sqlite-readonly` tombstone
must keep working. Call this out in your design — the synthesis will
require it.

Out of scope: RFC 0043 V2 work (hosted mode, multi-tenancy, bundled
Postgres distribution), Go daemon work (RFC 0039 V1.6), doc updates
beyond build/HANDOFF and the RFC 0043 V1.5 deltas section.
