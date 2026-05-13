---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["ergonomics_dx", "rfc-0043", "v1", "build"]
---

author: reviewer-unknown-model-002

# RFC 0043 V1 Build Review — Ergonomics/DX (claude lane)

## Scope

Fresh-context, ergonomics_dx review of the V1 build across both tracks
(Track A — repo-local migration body + DB v5 + fixture; Track B — CLI
surface, exit codes 11/12, RFC 0030 method-registry expansion). Inputs:
`docs/dogfood/048/build/track_a/HANDOFF.md`,
`docs/dogfood/048/build/track_b/HANDOFF.md`. Cross-cut by reading the
files each handoff says it touched. Operator UX evaluated from a
first-time-user perspective per `review_policy.posture: ergonomics_dx`.

## Verdict

**needs_revision.** The supporting plumbing is in place and the
exit-code-11/12 stderr templates plus JSON envelope shape are clean, but
the operator's documented remediation path is unrunnable as shipped:
the `daemon migrate-repo-local` verb the error messages instruct
operators to run is not registered with the argparse parser and is not
wired into `dispatch.py`. The same stderr template also instructs
operators to run `striatumd --foreground`, which exits with argparse
error 2 because the `daemon start` subparser has no `--foreground`
flag. Both are first-touch operator failures, not edge cases.

## Findings

### F1 (blocker, high) — `daemon migrate-repo-local` verb is not parseable

`src/striatum/cli/daemon_required.py:117-132` and the matching tests at
`tests/exit_codes/test_rfc0043_refusals.py:65-79` both bake in the
remediation string

```
Run: striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
```

but the parser at `src/striatum/cli/parser.py:138-167` only registers
`daemon_sub` choices `start | doctor | migrate | status | stop | health
| audit | sweep`. There is no `migrate-repo-local` subparser. Running
the documented verb yields argparse exit 2:

```
striatum daemon: error: argument daemon_command: invalid choice: 'migrate-repo-local'
```

`src/striatum/cli/daemon.py:17-34` defines `dispatch_daemon(args)` that
checks `args.daemon_command == "migrate-repo-local"` and reads
`args.from_substrate`, `args.to_substrate`, `args.repo_local_repo`,
`args.dry_run`, `args.keep_sqlite_readonly`, `args.confirm_delete`, but
that function is never imported or called from
`src/striatum/cli/dispatch.py` — `_dispatch_daemon` at
`dispatch.py:880-927` only branches on the existing legacy `migrate`
verb. So the helper Track A shipped is dead code. The operator
following the exit-code-12 stderr block ends up at an argparse-rejected
verb with no migration-specific stderr, no JSON envelope, and no
discoverable next step (`striatum daemon --help` will not list the
verb either).

Both Track A's and Track B's handoff explicitly flag the parser/
dispatch wiring as out of scope, and the existing tests pass because
they assert the remediation *string* (`test_repo_not_migrated_message_…`
just checks `… in text`) rather than actually executing the verb. The
result is a documented operator path that does not exist end-to-end.

**Required revision:** add a `daemon migrate-repo-local` subparser with
`--from {sqlite}`, `--to {pg}`, `--repo`, `--postgres-url`, `--dry-run`,
`--keep-sqlite-readonly` (safe default ON), `--no-keep-sqlite-readonly`
+ `--confirm-delete` flags, `--json`; route it from `_dispatch_daemon`
to the new `dispatch_daemon` helper; add an end-to-end test that
actually calls `dispatch.main(["daemon", "migrate-repo-local", …])` so
this class of gap can never regress unnoticed.

### F2 (blocker, high) — `striatumd --foreground` is not a real flag

`src/striatum/cli/daemon_required.py:97-106` ships the canonical stderr
template that includes the line `Foreground: striatumd --foreground`.
`pyproject.toml:45` registers `striatumd = "striatum.daemon:main"`, and
`src/striatum/daemon.py:1347-1351` shows that `striatumd` is a thin
wrapper that prepends `["daemon", "start", …]` to argv before calling
`striatum.cli:main`. Therefore the actual argparse target is the
`daemon start` parser at `src/striatum/cli/parser.py:140-144`, which
accepts only `--sweep-interval-seconds | --max-sweeps | --postgres-url
| --json`. Running `striatumd --foreground` produces

```
usage: striatum daemon start [-h] …
striatum daemon start: error: unrecognized arguments: --foreground
```

(exit 2), not a running daemon. The remediation block in
`render_daemon_unreachable_message()` is currently the operator's
primary recovery path for exit code 11; a wrong copy-paste-able
command in that block is exactly the wrong kind of
ergonomics regression.

**Required revision:** either rename the line to plain `Foreground:
striatumd` (it already runs in the foreground today), or wire a
no-op `--foreground` flag into the `daemon start` parser so the
documented command parses. Update
`tests/exit_codes/test_rfc0043_refusals.py:55` to match the chosen
spelling. Prefer the rename — it has no semantic content and avoids
parser bloat.

### F3 (high) — Tombstone semantics are silent and undocumented at the boundary

`src/striatum/daemon_pg/repo_local_migration.py:682-702` implements the
tombstone semantics:

1. `state.sqlite3` is `replace()`d to `state.sqlite3.tombstone`.
2. `chmod 0o444` makes the renamed file read-only.
3. `state.sqlite3-wal` and `state.sqlite3-shm` sidecars are unlinked.

This is correct and *does* keep the database openable for inspection
(any operator with `sqlite3 .striatum/state.sqlite3.tombstone .schema`
can still read it; SQLite ignores the filename extension), so the
"SQLite stays readable post-migration" acceptance criterion is met
mechanically. However, three operator-facing issues sit on top of it:

- **No human-readable signpost.** The migration returns
  `{"action": "tombstone", "path": "<path>", "mode": "0444"}` (line
  696) but the human stderr mode in `dispatch.py:132-136` will print
  the whole result as a JSON `{"ok": true, "data": …}` envelope (dicts
  unconditionally go through `json_dumps`). An operator running the
  command will see a JSON blob rather than a short banner like "moved
  state.sqlite3 → state.sqlite3.tombstone (read-only). Open with
  `sqlite3 .striatum/state.sqlite3.tombstone` for inspection." The
  affordance is discoverable only by reading the JSON output.

- **`.tombstone` is a non-standard suffix.** A first-time user
  exploring `.striatum/` after migration sees a file named
  `state.sqlite3.tombstone` with no README, no leading comment,
  and no `daemon migrate-repo-local --help` text (because that verb
  doesn't parse — see F1). The `repo_is_migrated()` helper at
  `cli/daemon_required.py:175-190` uses the tombstone presence as the
  "already migrated" signal, so the file *is* load-bearing — but the
  operator can't see that from outside.

- **No revert/inspect verb.** Once tombstoned there is no documented
  way to verify the byte-equivalent re-anchor without re-running the
  migration (which short-circuits as `already_migrated`). The
  reanchor SHA-256 pairs are stored in `striatumd.repo_migrations`
  (line 660-679) but there's no CLI verb to read them back; an
  operator with a corruption suspicion has to write SQL.

**Required revision (or follow-up):** at minimum, when the result
contains `sqlite_finalization`, the dispatcher should print a
single-line human banner above the JSON envelope: `"moved
state.sqlite3 → state.sqlite3.tombstone (mode 0444); inspect with
sqlite3 <path>"`. As a follow-up, expose a read verb (e.g. `striatum
daemon migrate-repo-local --status --repo <path>`) that returns the
stored manifests without touching anything.

### F4 (medium) — `--keep-sqlite-readonly` default is wrong on the parser surface

RFC 0043 §3 calls for `--keep-sqlite-readonly` to be the safe default
ON. `RepoLocalMigrationOptions.keep_sqlite_readonly: bool = True`
(`repo_local_migration.py:23-29`) honors that at the dataclass layer,
but the existing parser entry at `parser.py:155` is
`daemon_migrate.add_argument("--keep-sqlite-readonly",
action="store_true")` (default `False`). The Track A helper that would
read this — `cli/daemon.py:30` — uses
`bool(getattr(args, "keep_sqlite_readonly", True))`, but argparse will
always supply an explicit `False` for an absent `store_true` flag, so
the `True` fallback is dead. Combined with F1 (verb not registered),
this currently only matters for the legacy `daemon migrate` cutover
path, but the same wiring will land for `migrate-repo-local` if F1 is
fixed naively.

**Required revision:** spell the flag as a `BooleanOptionalAction`
(or split into `--keep-sqlite-readonly` / `--no-keep-sqlite-readonly`
in a mutually exclusive group) with `default=True`. Pair it with a
`--confirm-delete` flag (currently undefined on the parser; only
referenced via `getattr` in `cli/daemon.py:31`). Without these the
operator either can never reach the delete path or gets a misleading
"--confirm-delete is required" error chasing a flag the parser doesn't
expose.

### F5 (medium) — Dry-run output is not legible in human mode

`repo_local_migration.py:382-396` returns a dict for `--dry-run` with
ten fields plus a nested `source_counts` dict. `dispatch.py:132-136`
emits dict results as `json_dumps({"ok": True, "data": …})` regardless
of whether `--json` is set. A first-time operator running

```
striatum daemon migrate-repo-local --from sqlite --to pg --repo .
  --dry-run
```

(once the parser learns the verb — F1) gets ~250 bytes of JSON without
a single human-readable line. The information they actually need —
"would migrate N rows across M tables; SQLite is at schema v4; will
register repo as repo_…; will write tombstone to <path>; **nothing
will be written**" — is present in the dict but requires a JSON
reader to extract. Operators routinely run dry-runs to *check*
before committing; the format actively works against that.

**Required revision (or follow-up):** when the operator does not pass
`--json`, format the dry-run result with a short human header (counts
+ would_register_repository + already_migrated) before the JSON
envelope, or emit a `--summary` block. Same pattern would also help
the full migration (F3's tombstone banner is the same concern).

### F6 (low) — Documentation surface for exit codes 11/12 lives only in source

`src/striatum/errors.py:5-23` carries the canonical exit-code table as
module-level constants with comments explaining the renumbering of
RFC 0028 auth/capability codes (now 14/15) to free 11/12 for RFC 0043.
That's good for code readers. But there's no operator-facing surface:
`docs/SPEC.md` and the user-visible `--help` output do not document
the table. Operators triaging a CI failure with `exit code 12` will
have to either grep the source or hit the error message itself.

**Required revision (low priority):** add a short "Exit codes" table
to `docs/SPEC.md` (or wherever the operator manual lives) listing
codes 1–15 with one-line descriptions. Nothing source-side blocks
acceptance on this; flagging for the follow-up dogfood pass.

## What works

For balance, the following ergonomics-touching pieces are in good
shape and should be preserved as the revisions land:

- **Stderr template content** at
  `cli/daemon_required.py:90-106` correctly lists Linux systemd,
  macOS launchd, foreground (modulo F2), and Postgres remediation —
  the four-channel format the design synthesis called for, and the
  named socket path is in the first line where ops eyes land first.
- **JSON envelope structure** at `dispatch.py:94-109` carries the
  first message line plus a structured `hint` and the integer
  `code: 11`/`code: 12`, matching the existing error envelope
  vocabulary. The `hint` is a single line (asserted in
  `test_daemon_unreachable_hint_is_a_single_line` and
  `test_repo_not_migrated_hint_is_a_single_line`), which keeps it
  pipe-friendly for `jq -r '.error.hint'`.
- **Daemon-optional allowlist** at
  `cli/daemon_required.py:42-49` correctly retains `daemon`, `init`,
  `skills`, `plugin` so `striatum daemon doctor` runs without a
  daemon. The companion test
  `tests/cli/test_daemon_doctor_without_daemon.py` enforces this
  surface, and `_dispatch_daemon`'s `doctor` branch
  (`dispatch.py:889-900`) probes PostgreSQL + the SQLite registry
  directly rather than via a socket round-trip, so the "daemon
  doctor works when the daemon is down" criterion is met.
- **`--no-daemon` retirement** at `parser.py:25-29` is clean: the
  flag is gone, `--daemon` (the read-mode opt-in) is preserved, and
  `tests/cli/test_no_daemon_retired.py` asserts the argparse
  "unrecognized arguments" path. No silent SQLite fallback under
  `--no-daemon` because the flag no longer exists.
- **Idempotent re-run** at `repo_local_migration.py:357-368` and
  `repo_local_migration.py:447-455`: both the dry-run preflight and
  the full migration short-circuit on an existing
  `striatumd.repo_migrations` row and return
  `{"already_migrated": True, "checkpoint": …}`. Combined with the
  `SERIALIZABLE` transaction, concurrent invocations are safe.
- **Sidecar cleanup** at `repo_local_migration.py:710-717` removes
  `state.sqlite3-wal` and `state.sqlite3-shm`, which prevents the
  operator from accidentally getting a "stale lock"-style SQLite
  error when re-opening the tombstone read-only.
- **Renumbered RFC 0028 codes**: `src/striatum/daemon.py:36-79`
  re-maps the legacy auth/capability errors to 14/15 (the constants
  in `errors.py:22-23`). The legacy V1 module-local
  `DaemonUnreachableError` keeps exit 10 (RFC 0030 version-skew code,
  which is correct for the registry-unreachable case it was originally
  defined for) with a docstring explicitly pointing future readers at
  `striatum.errors.DaemonUnreachableError` (exit 11). Two errors with
  the same class name in different modules is suboptimal but the
  docstring is explicit, and no tests assert by numeric code, so the
  rename is contained.

## Affordance discoverability summary

A first-time operator on this build, following the exit-code-12 stderr
block, will hit the following sequence today:

1. `striatum status` (or any non-allowlisted verb) under
   `STRIATUM_DAEMON_REQUIRED=1` with no socket → exit 11 + remediation
   block. ✓
2. Operator copies `Foreground: striatumd --foreground` → exit 2,
   `unrecognized arguments: --foreground`. ✗ (F2)
3. Operator falls back to `systemctl --user start striatumd` or
   foregrounds without the flag → daemon running → original verb
   re-issues exit 12 + remediation. ✓
4. Operator copies `striatum daemon migrate-repo-local --from sqlite
   --to pg --repo <path>` → exit 2, `invalid choice: 'migrate-repo-
   local'`. ✗ (F1)
5. No `--help` for the verb (because it doesn't parse), no link from
   the JSON envelope's `hint` to a docs URL, and no `striatum doctor
   --verbose` field that mentions the migration. Operator is stuck.

The remediation messages are well-composed and the supporting
plumbing (errors, hints, allowlist, registry expansion) is correct.
The blockers are mechanical: register the verb, fix the foreground
flag spelling. Once those land the affordance set will be both
discoverable and consistent.

## Test posture observations (not blocking)

- `tests/exit_codes/test_rfc0043_refusals.py` and
  `tests/cli/test_no_daemon_retired.py` are well-scoped to the
  surface they own and use `monkeypatch` + `tmp_path` cleanly; they
  will run on the existing pytest fixture without PostgreSQL.
- The exhaustiveness test
  `tests/daemon_rpc/test_registry_rfc0043_coverage.py` (per Track B's
  handoff) is the right shape — it asserts every mutation in
  `cli/mutations.py` has a registered method. From an ergonomics
  angle, this protects against future routing gaps where a new
  mutation lands without an RPC entry; good preventive scaffolding.
- Track A's `test_repo_local_migration.py` covers the migration body
  but the operator-surface assertions (the verb parses, dispatcher
  routes it, `--dry-run` prints something legible) are missing —
  these tests should land alongside the F1 fix.
- Track A's handoff flags that `tests/test_daemon_rpc.py` still
  asserts daemon DB version 4 while the build moves to v5. That
  legacy assertion will break under `make test` and is on the
  list of failing suites the handoff already calls out. Not an
  ergonomics regression per se, but it does mean an operator
  running `make test` post-merge will see a red bar and have to
  decide whether to trust the new code. Recommend bundling the
  v5 fixup with the F1 wiring.
