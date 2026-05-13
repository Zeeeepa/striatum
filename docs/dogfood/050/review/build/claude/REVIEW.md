---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1.5", "build"]
---

author: reviewer-unknown-model-002

# Build review — dogfood-050 (RFC 0043 V1.5), ergonomics_dx posture

Scope: developer-ergonomics review of the Claude-lane V1.5 build. Read the
handoff at `docs/dogfood/050/build/HANDOFF.md` and the production /
test surfaces it cites. Verdict considers whether the affordances are
discoverable and consistent for a first-time operator upgrading from V1.

## Required checks (pass/fail)

| Check | Status | Evidence |
|---|---|---|
| F-parser wired (`migrate-repo-local --help` prints) | PASS | `src/striatum/cli/parser.py:167-199` registers the subparser under `daemon_sub`; argparse provides `--help` for free. |
| F-parser dispatch arm | PASS | `src/striatum/cli/dispatch.py:881-887` routes `migrate-repo-local` into `striatum.cli.daemon:dispatch_daemon`; `src/striatum/cli/daemon.py:24-44` implements it. |
| F-escape default flipped | PASS | `src/striatum/cli/daemon_required.py:53-66` — `resolve_requirement` returns `None` only on `DAEMON_OPTIONAL_COMMANDS` or `STRIATUM_DAEMON_REQUIRED == "0"`; unset env now enforces. |
| F-escape silent-fallback audit (production path) | PASS for the gate | `src/striatum/cli/dispatch.py:196` calls `enforce_daemon_required` before the `ensure_initialized(repo)` SQLite path. `rg sqlite3 src/striatum/cli/` returns many hits in `mutations.py`, `evidence.py`, `recovery.py`, `workflow.py`, `worktree.py`, `introspect.py`, etc. — all reached only after the gate at `dispatch.py:196` passes. No second `sqlite3.connect` bypass that skips the gate. |
| F-test e2e exit-12 against real dispatch | PASS | `tests/exit_codes/test_rfc0043_refusals.py:207-243` calls `dispatch_mod.main(["--repo", str(tmp_path), "status"])` against a tmp repo with a `.striatum/state.sqlite3` and a listening socket. Asserts rc == 12 and `striatum daemon migrate-repo-local --from sqlite --to pg --repo` in stderr. JSON envelope variant at `:246-271`. |
| F-test resolve_requirement default | PASS | `:184-189` asserts unset env → `DaemonRequirement(enforced=True, ...)`; `:192-196` asserts env=="0" → `None`. The synthesis-named pair lands as specified. |
| F-crash regression test fails on un-fixed code | PASS | `tests/daemon_pg/test_repo_local_migration_crash_resume.py:193-250` monkeypatches `_tombstone_or_delete_state_db` to raise mid-finalization, asserts the source + sentinel persist after the crash, then asserts the rerun lands `tombstone.stat().st_mode & 0o777 == 0o444`, removes the source, clears the sentinel, and reports `resumed_from_checkpoint: True`. On un-fixed V1 code, neither the sentinel nor the resume helper exists, so the test fails to import (`_write_sentinel_atomically`, `_resume_sqlite_finalization_after_checkpoint`) — i.e. it fails hard on un-fixed code, which is the desired regression behavior. Pure-Python helper coverage at `:54-186` runs without Postgres. |
| Backward-compat tombstone under all flag combos | PASS | Existing `tests/daemon_pg/test_repo_local_migration.py:99-110` (`test_full_run_copies_rows_checkpoints_and_tombstones`) asserts `0o444` on the normal path; `:139-167` covers `--no-keep-sqlite-readonly` + `--confirm-delete` (delete path) and `--no-keep-sqlite-readonly` without confirm (refusal). New `tests/daemon_pg/test_repo_local_migration_crash_resume.py:73-105` (`test_resume_finalizes_tombstone_when_source_matches`) confirms tombstone semantics survive the resume helper. |
| Schema additive | PASS (no delta) | `src/striatum/daemon_pg/repo_local_migration.py:1-20` does not add a `0006_*.sql`; `tests/daemon_pg/test_repo_local_migration.py:45-71` keeps `LATEST_DAEMON_DB_VERSION == 5`. F-crash is schema-additive in spirit but needs no SQL change. |
| `make test` green | UNVERIFIED | Bash tool was denied throughout this review. The handoff (lines 190-198) reports the same denial. Cannot independently confirm green CI; the static reading of the test files matches the assertion shapes the handoff promises. |

## Ergonomics_dx findings

### F-dx-1 — Per-flag help text on `migrate-repo-local` is sparse (low)

`src/striatum/cli/parser.py:167-199` registers the subparser but only
`--keep-sqlite-readonly` (`:186-191`) and `--no-keep-sqlite-readonly`
(`:192-197`) carry `help=...` strings. `--from`, `--to`, `--repo`,
`--postgres-url`, `--dry-run`, `--confirm-delete`, and `--json` have no
per-flag help. A first-time operator running
`striatum daemon migrate-repo-local --help` will see flag names and
argument types but no per-flag explanation — they will not learn from the
help output that `--from` is currently the only valid value `sqlite` (the
parser knows but does not tell them in help), that `--repo` falls back to
the top-level `--repo`, or that `--postgres-url` overrides
`STRIATUM_DAEMON_DB_URL`. Compare with `workflow upgrade` at
`src/striatum/cli/parser.py:270-303`, which carries a `description=` and
per-flag help that reads as a quickstart.

Suggested fix: add a `description="..."` on the subparser block and
`help="..."` on every flag.

### F-dx-2 — Tombstone default is implicit and not announced in help (low)

`--keep-sqlite-readonly` defaults to `True` (parser.py:186-191), and the
flag-pair (`--keep-sqlite-readonly` / `--no-keep-sqlite-readonly`) is the
right shape. But the help text reads "rename state.sqlite3 to
state.sqlite3.tombstone and chmod it 0444" with no mention that this is
the default — an operator may assume they must pass the flag explicitly
to get the tombstone, and may try `--no-keep-sqlite-readonly` first
thinking it is "off by default." Append " (default)" to the
`--keep-sqlite-readonly` help string; mirror " (off by default)" on
`--no-keep-sqlite-readonly`.

### F-dx-3 — Default-flip migration story is in the HANDOFF, not the operator-facing docs (low)

The RFC `## V1.5 deltas` section in
`docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:627-642`
documents the env-var flip as the opt-OUT. The HANDOFF
(`docs/dogfood/050/build/HANDOFF.md:259-274`) names the two operator
upgrade paths. But the implement prompt's "no doc cascade" rule means
`README.md`, `docs/SPEC.md`, `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
and `CHANGELOG.md` were not updated. An operator upgrading
`pip install --upgrade striatum` without reading the RFC will see exit
11/12 with no breadcrumb explaining "what changed since the previous
release." Recommend a one-paragraph CHANGELOG entry in the cascade work
the handoff defers to the operator.

### F-dx-4 — Exit-11 remediation block does not name the legacy opt-out (low)

`src/striatum/cli/daemon_required.py:84-100`
(`render_daemon_unreachable_message`) lists four remediation channels:
systemctl, launchd, foreground, and the Postgres URL hint. It does not
mention `STRIATUM_DAEMON_REQUIRED=0`. That omission is defensible —
the env var is documented as a test-only opt-out in `daemon_required.py:13-15`
and the RFC §3 boundary is "no silent SQLite fallback in production." But
operators upgrading a long-lived target repo will hit exit 11 before they
have a daemon running, and the remediation block won't tell them the
test-mode escape hatch exists. If the design holds that escape hatch as
test-only, the docstring at `daemon_required.py:13-15` already says so;
the trade-off is acceptable but worth confirming intentional.

### F-dx-5 — Exit-12 remediation block is excellent (positive note)

`src/striatum/cli/daemon_required.py:111-118`
(`render_repo_not_migrated_message`) produces a single copy-pasteable
command with the resolved repo path:

```
repo_not_migrated: /path/to/repo has not been migrated to daemon PostgreSQL state
Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo /path/to/repo
```

`tests/exit_codes/test_rfc0043_refusals.py:233-241` asserts the resolved
path appears so operators can copy verbatim. The `--json` envelope's
`hint` (`daemon_required.py:121-126`) is a single line that names the
same command, suitable for structured remediation surfaces. This is the
right shape for the V1.5 flip and the strongest ergonomic signal in the
build.

### F-dx-6 — Crash-resume reports `resumed_from_checkpoint: True` (positive note)

`src/striatum/daemon_pg/repo_local_migration.py:577-587` decorates the
resumed finalization result with `"resumed_from_checkpoint": True`, and
the orphan-sentinel path returns `{"action": "cleared_orphan_sentinel"}`.
An operator running `migrate-repo-local` after a crash will see in the
JSON envelope (or printed result) exactly what the rerun did. Good
operator-visible "what happened" signal.

### F-dx-7 — `tests/conftest.py` session-level opt-out fixture is well-scoped (positive note)

The session-level `STRIATUM_DAEMON_REQUIRED=0` opt-out in
`tests/conftest.py:13-31` is documented inline with the synthesis
reasoning (incremental migration of SQLite-backed fixtures). The
per-function `monkeypatch.delenv` in
`tests/exit_codes/test_rfc0043_refusals.py:37-43` correctly overrides it
to assert the new default. This pattern is reusable as more test files
adopt the daemon-required surface.

## Operator-flow walkthrough

I traced a first-time-operator upgrade flow:

1. Operator upgrades `striatum`; runs `striatum status` against a V1 repo.
   - No daemon yet → `enforce_daemon_required` (dispatch.py:196) raises
     `DaemonUnreachableError`. `dispatch.py:94-109` catches and prints
     the multi-line stderr block + JSON hint. Exit 11. Operator follows
     the systemd/launchd/foreground hint to start the daemon.
2. Operator re-runs `striatum status`.
   - Daemon reachable but no `state.sqlite3.tombstone` exists →
     `repo_is_migrated` (`daemon_required.py:169-184`) returns False →
     `RepoNotMigratedError` raised. Stderr names
     `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>`.
     Exit 12. Operator copy-pastes the suggested command.
3. Operator runs `striatum daemon migrate-repo-local --from sqlite --to pg --repo .`
   - `_dispatch_daemon` (`dispatch.py:881`) routes to
     `cli/daemon.py:dispatch_daemon`. The default
     `--keep-sqlite-readonly=True` causes the tombstone path. On
     success, the source becomes `state.sqlite3.tombstone` with mode
     0444. On crash mid-finalization, the sentinel
     `.striatum/state.sqlite3.migrated` stays on disk and the operator
     re-runs the same command — `_resume_sqlite_finalization_after_checkpoint`
     completes the tombstone idempotently.
4. Operator re-runs `striatum status` — daemon-required gate passes
   (`repo_is_migrated` returns True once the tombstone exists). Normal
   command behavior resumes.

The flow is coherent end-to-end. The two operator-visible friction points
are F-dx-1 (sparse `--help`) and F-dx-3 (no CHANGELOG breadcrumb). Both
are documentation issues, not behavior issues.

## Recommendation

**Accept with findings.** All required checks pass on static reading. The
ergonomic surface is correct and the exit-12 remediation block is the
strongest operator-facing signal in the V1.5 build. The findings above
are low-severity polish items that fit the cascade work the handoff
already defers to the operator:

- F-dx-1, F-dx-2 — parser polish (one PR-sized edit to `parser.py`).
- F-dx-3 — CHANGELOG / SPEC / HOW_TO_HUMAN cascade the HANDOFF defers.
- F-dx-4 — design confirmation on the exit-11 stderr block.
- F-dx-5, F-dx-6, F-dx-7 — positive notes; preserve in the next round.

`make test` could not be run inside this review (Bash denied for every
invocation, mirroring the implementer's experience). The static reading
of the new tests matches their stated assertions, and the new code paths
all exist where the HANDOFF claims. Operator-side replay of `make test`
and `pytest tests/daemon_pg/test_repo_local_migration_crash_resume.py`
(per HANDOFF lines 202-212) is the unblocking step before merge.
