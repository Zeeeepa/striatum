---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0043", "v1-6", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0043 V1.6 (claude, ergonomics_dx)

Scope: developer-ergonomics review of the V1.6 substrate-hardening build
against `docs/dogfood/052/build/HANDOFF.md`, `docs/dogfood/052/DESIGN_SYNTHESIS.md`,
and the cited source. Posture: first-time-user discoverability and
consistency of the visible CLI surface, env-var semantics, and refusal
messages.

## Required checks (pass/fail)

| Check | Status | Evidence |
|---|---|---|
| F-escape: bare `STRIATUM_DAEMON_REQUIRED=0` is rejected; only the test-harness pair enables it | PASS | `src/striatum/cli/daemon_required.py:57-76` — `resolve_requirement` returns `None` only when both `STRIATUM_DAEMON_REQUIRED == "0"` AND `STRIATUM_TEST_HARNESS == "1"`. New `ENV_TEST_HARNESS` constant at `:33`. Module docstring at `:13-18` documents the V1.6 narrowing. `tests/conftest.py:14-28` session fixture pairs both env vars so legacy fixtures stay green. |
| F-split-brain: `connect()` refuses fresh DB when sentinel present | PASS | `src/striatum/db.py:77-117` — when the target SQLite file is absent, checks `.striatum/state.sqlite3.migrated` OR `.striatum/state.sqlite3.tombstone`; either present → `StriatumError(exit_code=12)` with `repo_not_migrated` text matching the daemon-required remediation shape. Docstring at `:85-90` cross-references RFC 0043 V1.6 F-split-brain. |
| F-lock: concurrent migrate-repo-local refuses cleanly | PASS (behavior) / DEVIATION (exit code) | `src/striatum/daemon_pg/repo_local_migration.py:26-61` — `MigrationInProgressError` and `_exclusive_migrate_lock` context manager wrap the entire `migrate_repo_local` body (`:393-396`). Sidecar lock at `.striatum/state.sqlite3.migrate.lock` (justified in docstring `:34-41`). Refusal text names the lock path (`:50-53`). Exit code is **14**, not the **8** that `DESIGN_SYNTHESIS.md:41-42` explicitly mandated ("Refuse lock contention with exit code 8 and a clear message naming the source file; avoid introducing a new exit code for this narrow V1.6 slice."). See finding F1. |
| F-help: every flag has `help=` | PASS | `src/striatum/cli/parser.py:167-233` — subparser carries a multi-sentence `description=` (`:173-180`); every flag has `help=`: `--from` `:187`, `--to` `:194`, `--repo` `:200`, `--postgres-url` `:204`, `--dry-run` `:209`, `--keep-sqlite-readonly` `:216` (explicit `(default)`), `--no-keep-sqlite-readonly` `:222`, `--confirm-delete` `:227`, `--json` `:232`. Closes the V1.5 F-dx-1 finding I filed on dogfood-050. |

## Ergonomics_dx findings

### F1 — F-lock uses exit code 14 against the design's exit-8 directive (low, deviation)

`src/striatum/daemon_pg/repo_local_migration.py:26-29` defines
`MigrationInProgressError(StriatumError)` with `exit_code = 14`. The
synthesis at `docs/dogfood/052/DESIGN_SYNTHESIS.md:41-42` is explicit:
"Refuse lock contention with exit code 8 and a clear message naming the
source file; avoid introducing a new exit code for this narrow V1.6
slice." The implementer chose 14 instead and recorded it as a V2.0
follow-up in `HANDOFF.md:73` ("Add exit code 14 to the RFC 0043 error
code register"), but the deviation itself is not flagged in the
HANDOFF's `Deviations` section — only A1's V2.0 scope deferral is.

DX impact: today the RFC 0043 §3 exit-code table only documents 11
(`daemon_unreachable`) and 12 (`repo_not_migrated`). An operator who
hits a CI failure with `exit 14` cannot find it in the RFC or in
`--help` output. They will land in source-grep territory before they
learn what 14 means. CHANGELOG v1.40.0 (`CHANGELOG.md:54-59`) mentions
it but the discoverability path from "I got exit 14 in CI" to
"oh, migrate-repo-local is in flight" is still grep-only.

Suggested follow-up: either (a) collapse to exit 8 per the synthesis
and reserve 14 for a future, broader concurrency-refusal slot, or
(b) accept the new code and land the RFC 0043 §3 register update in
the same train rather than deferring to V2.0. The error-code register
is documentation; the entry can ship within V1.6's blast radius.

### F2 — `MigrationInProgressError` text invites stale-lock deletion without safety framing (low)

`src/striatum/daemon_pg/repo_local_migration.py:50-53`:

```
migrate_in_progress: another striatum daemon migrate-repo-local is
holding {lock_path}; wait for it to finish or remove the stale lock
file.
```

"or remove the stale lock file" is presented as equivalent to "wait for
it to finish." A first-time operator who sees this message and is in
a hurry will `rm` the lockfile while a sibling migration is actually
running, which on most filesystems silently succeeds (flock survives
unlink — the running migration keeps its open fd, but the lockfile
that the next caller `open()`s is a brand-new inode with no flock
holder, so two migrations can race the SQLite copy + Postgres
checkpoint after that). The dangerous path is undersignaled.

Suggested rewording: lead with the wait, gate the deletion on
operator certainty, and ideally name a way to detect "running vs
stale" (e.g. checking `lsof`/`fuser` or the daemon's
`striatum daemon status` output if it surfaces in-flight migrations):

```
migrate_in_progress: another striatum daemon migrate-repo-local is
holding {lock_path}. Wait for it to finish. Only remove the lock file
if you are certain no migration is running; concurrent migrations
will corrupt the destination.
```

### F3 — Bare `STRIATUM_DAEMON_REQUIRED=0` is silently ignored in production (low)

`src/striatum/cli/daemon_required.py:69-76`: when the operator sets
`STRIATUM_DAEMON_REQUIRED=0` without `STRIATUM_TEST_HARNESS=1`, the
resolver falls through to the enforcement branch with no log line or
warning. From a first-time-user perspective who reads the V1.5 RFC,
the old docs, or older internal scripts, the bare env var is the
documented opt-out — they will export it, see commands refuse with
`daemon_unreachable` (exit 11), and not connect the refusal back to
"my opt-out env var was narrowed in V1.6."

The behavior is correct (closing the codex threat-model finding is
the whole point). The DX gap is the missing breadcrumb. Suggested
follow-up: when `STRIATUM_DAEMON_REQUIRED == "0"` is set and
`STRIATUM_TEST_HARNESS != "1"`, emit a single-line stderr notice
("STRIATUM_DAEMON_REQUIRED=0 is now test-harness-only in V1.6;
production enforcement is active. See RFC 0043 §3 V1.6.") before
proceeding with enforcement. Cheap, exactly once per process, and
turns a confusing exit-11 into an actionable signal.

### F4 — `db.connect` refusal text suggests "use the daemon socket directly" with no how-to (low)

`src/striatum/db.py:98-105`:

```
repo_not_migrated: {repo} was migrated to daemon PostgreSQL state
but the fresh SQLite path is being opened; this indicates a
split-brain. Run: striatum daemon migrate-repo-local --from sqlite
--to pg --repo {repo} (or use the daemon socket directly).
```

"or use the daemon socket directly" is undefined for a first-time
operator. There is no second command shown. The migrate-repo-local
suggestion is correct in the common case where the local SQLite was
finalized but the operator's tooling is still trying to open it; the
parenthetical adds confusion without value.

Suggested follow-up: drop the parenthetical, or replace it with the
actual second command (e.g. `striatum daemon status --json`) and a
one-line cue about when the operator would prefer it.

### F5 — `--keep-sqlite-readonly` help text omits behavior when both true/false flags appear (very low / nit)

`src/striatum/cli/parser.py:211-223`: `--keep-sqlite-readonly`
(action=store_true, default=True) and `--no-keep-sqlite-readonly`
(action=store_false) is the idiomatic argparse pattern, but neither
help string tells the operator that `--keep-sqlite-readonly` is the
default and that they need to *opt in* to deletion with
`--no-keep-sqlite-readonly --confirm-delete`. The `(default)` token
in `:216` and the cross-reference at `:222` come close but require
the reader to mentally merge them. Compare the dispatcher-side
guard at `_verify_delete_options` (`:892-894`), which raises with
the precise pairing required. A single sentence in the subparser
`description` ("Defaults to a 0444 tombstone; pass
`--no-keep-sqlite-readonly --confirm-delete` to delete instead.")
would close this — the existing description already mentions both
but reads as a paragraph rather than an instruction.

## Closures noted from dogfood-050

- F-help (this build) closes the V1.5 finding F-dx-1
  (sparse per-flag help on migrate-repo-local) that I filed in
  `docs/dogfood/050/review/build/claude/REVIEW.md`. The new
  `--help` output reads as a quickstart; consistent with the
  ergonomic level set by `workflow upgrade`.

## Posture-specific assessment

For ergonomics_dx, the V1.6 surface is largely discoverable and
consistent. The four required checks pass on behavior. The findings
above are paper-cuts a first-time operator will hit on edge paths
(stale-lock deletion, bare env-var opt-out, split-brain text) and one
process deviation (exit-code 14 vs 8) that creates a documentation
debt the V1.6 train can pay now.

## Verdict

`accept_with_findings` (low) — surface is acceptable to ship; F1-F4
are worth booking as V1.7 polish (or, for F1, an in-train RFC 0043 §3
register update).
