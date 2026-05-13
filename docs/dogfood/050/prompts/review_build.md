# Build Review Prompt (RFC 0043 V1.5, 3-way)

Produce REVIEW.md at `docs/dogfood/050/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0043", "v1.5", "build"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

The `author:` byline is a plain markdown line AFTER the front-matter block — not inside it, no markdown bold, no lane prefix. Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the implementation handoff at `docs/dogfood/050/build/HANDOFF.md`.

Per-lane angle:

- **codex (threat_model)**: F-crash — kill -9 between Postgres commit and SQLite tombstone leaves no usable bypass on resume. The regression test actually simulates a crash mid-tombstone and asserts resume completes idempotently. F-escape — daemon-required is the default code path; `STRIATUM_DAEMON_REQUIRED=0` (if retained) is documented as opt-OUT and gated to test-only contexts. No silent SQLite fallback anywhere in `src/striatum/cli/`. F-test — the e2e exit-12 test exercises a real `dispatch.main(...)` call, not a unit-level stub.
- **claude (ergonomics_dx)**: `striatum daemon migrate-repo-local --help` prints. Exit-code-12 stderr block tells the operator the exact command to run. Crash-recovery resume prints a readable resume message. `--keep-sqlite-readonly` still tombstones correctly. The transition story for pre-V1.5 repos is documented in HANDOFF (what does an upgrading operator see?).
- **gemini (adversarial threat_model)**: any remaining silent SQLite fallback in the cli/ tree (grep for `sqlite3.connect`, `state.sqlite3`, references to the legacy `--no-daemon`). Concurrent `migrate-repo-local` invocations on the same repo still mitigated by SERIALIZABLE + unique constraints. Rollback-on-crash atomicity is real (Postgres transaction is not committed until SQLite is safe / sentinel is written). Backward-compat tombstone semantics preserved under every flag combination (`--confirm-delete`, `--keep-sqlite-readonly`, neither, both).

Required checks (all lanes):

- **F-crash regression test in-tree**: a test that fails on un-fixed code (e.g. monkeypatches `_tombstone_or_delete_state_db` to raise mid-call) and passes on the fixed version. Path under `tests/daemon_pg/`.
- **F-escape default flipped**: `rg -n "STRIATUM_DAEMON_REQUIRED" src/striatum/cli/daemon_required.py` shows the env var is the opt-OUT (or removed entirely per synthesis). Default code path enforces daemon-required.
- **F-escape silent-fallback audit**: `rg -n "sqlite3" src/striatum/cli/` returns no production-fallback hits.
- **F-parser wired**: `striatum daemon migrate-repo-local --help` prints (or the test that asserts it does). The subparser is in `src/striatum/cli/parser.py`.
- **F-test e2e present**: `tests/exit_codes/test_rfc0043_refusals.py` (or chosen path) asserts `SystemExit.code == 12` from a real `dispatch.main(...)` call on an unmigrated repo fixture.
- **Backward-compat tombstone**: a test exercises `--keep-sqlite-readonly` through the new code paths.
- **Schema additive**: any new `src/striatum/daemon_pg/sql/0006_*.sql` (if present) contains only `CREATE TABLE` / `ALTER TABLE ... ADD COLUMN` / `CREATE INDEX` — no destructive statements.
- **Tests pass**: `make test` green; the new tests in particular.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
