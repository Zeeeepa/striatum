# Coordinator Role (Dogfood 050 — RFC 0043 V1.5, close D102 follow-up findings)

You keep the operator-driven dogfood-050 moving. 9 jobs total, single
track addressing the four follow-up findings recorded in
`docs/dogfood/048/decisions/D102_cycle_exhaustion.md`. The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on the F-crash / F-escape / F-parser / F-test deltas.
2. **1 synthesis** — codex picks one path from the three designs and
   locks the implementation order across the four findings.
3. **1 design review** — claude `ergonomics_dx` posture gates the
   synthesized design before implement.
4. **1 implementer** — **claude** on Python (`src/striatum/` and
   `tests/`). Sub-agents aggressively, one per finding.
   **NOT codex** — deliberate avoidance of the 5-time codex/codex
   anti-pattern (D102 flagged this risk).
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial threat_model`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** consolidate job in this workflow. The operator does the RFC
index, TODO, CHANGELOG, SPEC, and HOW_TO updates by hand once the
dogfood lands (dogfood-042 cascade lesson).

**Scope (the four findings):**

- F-crash (CRITICAL) Crash-recovery persistence gap in
  `src/striatum/daemon_pg/repo_local_migration.py` — when
  `migrate-repo-local --confirm-delete` is interrupted between Postgres
  commit and SQLite tombstone, the resume path must idempotently
  finish the tombstone (sentinel-based checkpoint or transactional
  rollback per synthesis).
- F-escape (MAJOR) CLI escape path closure in
  `src/striatum/cli/daemon_required.py` — daemon-required must be the
  default; the env-gated `STRIATUM_DAEMON_REQUIRED=1` opt-IN is a
  bypass. Audit the rest of `src/striatum/cli/` for any other silent
  SQLite fallback.
- F-parser (MODERATE) `daemon migrate-repo-local` subparser wiring in
  `src/striatum/cli/parser.py` + dispatch arm in
  `src/striatum/cli/dispatch.py::_dispatch_daemon`. The verb is
  implemented but not reachable from the CLI.
- F-test (LOW) End-to-end exit-code-12 test gap — no `dispatch.main(...)`
  assertion for an unmigrated repo. Land under
  `tests/exit_codes/` or `tests/cli/` per synthesis.

Allowed write scope (enforced by the validator):

- `src/striatum/daemon_pg/` — migration body + (additive only) SQL.
- `src/striatum/cli/` — parser, dispatch, daemon_required, daemon.
- `src/striatum/errors.py` — only if a new error class is required.
- `tests/` — regression + e2e tests.
- `Makefile` — only if synthesis adds a new test target.
- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` — V1.5 deltas section only.
- `docs/dogfood/050/build/HANDOFF.md` — handoff.

Backward-compat lock (non-negotiable):
- Postgres schema changes are additive only (new columns NULLable, new
  tables only). No `ALTER TABLE ... DROP COLUMN`, no destructive index
  changes.
- `--keep-sqlite-readonly` tombstone (rename to `.tombstone`, mode 0444)
  must keep working under every code path the V1.5 change touches.

Gemini is reserved for design and adversarial review only. Never
implementer.
