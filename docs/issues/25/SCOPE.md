---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/25/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md", "src/striatum/cli/dispatch.py", "src/striatum/cli/parser.py", "src/striatum/cli/daemon_required.py", "src/striatum/cli/daemon_rpc_route.py", "contracts/daemon_methods.json", "go/pkg/repositories/service.go", "src/striatum/daemon_pg/repositories.py"]
---

author: triager-unknown-model-001

# GH #25 - Scope

Bound scope for GH #25, "`striatum repo list` without `--json` returns
`repo_not_migrated` for an already registered repository." This is a
read-side CLI cleanup only: `repo list` must query the daemon registry and
render a human table in non-JSON mode, while `repo list --json` keeps its
current payload shape.

## Finding

`repo list` is registered as daemon RPC method `repo.list` in
`contracts/daemon_methods.json`. The daemon-side implementation is already
PostgreSQL-backed:

- Go: `go/pkg/repositories/service.go`, `Service.List`, registered as
  `repo.list`, selects from `striatumd.repositories`.
- Python PG helper: `src/striatum/daemon_pg/repositories.py`,
  `repo_list_pg`, selects from `striatumd.repositories`.

The misleading refusal is in the CLI preflight, not the registry query.
`src/striatum/cli/dispatch.py` calls `enforce_daemon_required(...)` before
daemon routing. `src/striatum/cli/daemon_required.py` then probes the daemon
socket and also checks local `.striatum/state.sqlite3` via
`repo_is_migrated(...)`. That local SQLite check is valid for setup/mutation
paths that need a target repository, but it is wrong for `repo list`: listing
registered repositories is a daemon-global read and should not depend on
the current working tree's retired SQLite state.

There is a second visible gap: successful daemon-routed `repo list` returns a
dict. `src/striatum/cli/dispatch.py` prints dict results as JSON even when
`--json` is absent. GH #25 needs a human table for the non-JSON path only.

## Files In Scope

The implementer should keep this change tightly bounded to the registry-list
read path and focused tests.

- `src/striatum/cli/dispatch.py`
  - Detect `args.command == "repo"` and `args.repo_command == "list"` before
    the daemon-required preflight.
  - Keep the daemon socket reachability requirement for `repo list`.
  - Skip only the repo-local SQLite migration check for this command.
  - Ensure successful non-JSON `repo list` returns a string table rather than
    a dict that `main()` serializes as JSON.
- `src/striatum/cli/daemon_required.py`
  - Add the narrow seam needed by `dispatch.py`, for example
    `enforce_daemon_required(..., check_repo_migration=False)` or an
    equivalent helper that still raises `daemon_unreachable` when the daemon
    socket is unavailable.
  - Leave the default behavior unchanged for every other command.
- `src/striatum/cli/daemon_rpc_route.py`
  - Keep `_params_repo_list(...)` returning `{}`.
  - Add or call a human formatter for `repo.list` only when
    `args.json` is false, or let `dispatch.py` format the routed payload.
  - Do not change the JSON data returned from daemon RPC.
- `src/striatum/cli/parser.py`
  - Read-only unless a help string is needed for the table behavior. No new
    flag is expected.
- `tests/test_cli_daemon_rpc_route.py`
  - Add a focused unit test that `repo list --json` still routes to
    `repo.list` with empty params and returns the daemon payload unchanged.
  - Add a focused unit test for non-JSON `repo list` table formatting if the
    formatter lives in `daemon_rpc_route.py`.
- `tests/exit_codes/test_rfc0043_refusals.py`
  - Add or adjust a test so `repo list` with a reachable daemon socket and a
    local `.striatum/state.sqlite3` does not emit `repo_not_migrated`.
  - Add a test so `repo list` with an unreachable daemon reports
    `daemon_unreachable`, not `repo_not_migrated`.
  - Keep existing `status` / mutation-command `repo_not_migrated` tests
    intact.
- `tests/daemon_pg/test_repo_registration.py`
  - Existing RPC coverage for `repo.list` can remain; extend only if the
    implementer needs daemon-backed registry evidence.
- `tests/architecture/test_authority_guardrails.py`
  - Update only if the implementation introduces a new direct PG import or
    changes the direct-admin allowlist. A clean daemon-routed solution should
    not require changing the authority matrix.

## Files Out Of Scope

Do not touch these unless a test reveals a direct contradiction to GH #25.

- `src/striatum/day_zero.py`, `src/striatum/daemon_pg/repositories.py`
  `repo_add_pg`, and `go/pkg/repositories/service.go` `Service.Add`:
  `adopt` and `repo add --init` should continue refusing a live
  `.striatum/state.sqlite3` because setup is where the SQLite-retirement
  check belongs.
- `src/striatum/cli/daemon.py` and daemon lifecycle/admin commands.
- `go/pkg/repositories/service.go` `Service.List`, unless Go conformance
  shows the payload itself is wrong. The current daemon query is already the
  right authority.
- `contracts/daemon_methods.json`: `repo list` is already mapped to
  `repo.list`; no new RPC method is needed.
- Other `repo_not_migrated` call sites that protect target-repository
  workflow verbs, workflow upgrade running-run checks, mutation verbs, or
  retired SQLite import diagnostics.
- `docs/DECISION_LOG.md`: this is an implementation bug fix under existing
  D094/D107/D113 decisions, not a new product decision.
- `.striatum/`, legacy SQLite fixtures, historical prompts, dogfood history,
  and unrelated repo/cross-repo commands.

## Table Format

Non-JSON `striatum repo list` should render a compact table from the
`repo.list` daemon payload.

Columns:

```text
  DISPLAY_NAME  STATE   LAST_SEEN_AT           REPOSITORY_ID  REPO_ROOT
* striatum      active  2026-05-19T...Z        repo_a89ecd16  /path/to/striatum
```

Rules:

- Prefix the row for the current `--repo` / cwd repository with `*`; all
  other rows get a blank marker.
- Sort the current repository first when present, then sort remaining rows by
  `display_name`, `repo_root`, and `repository_id` for stable human output.
- Show `repository_id` as a short stable prefix: `repo_` plus the first
  eight hex characters after the prefix, or the first 13 characters if the
  shape is unexpected.
- Render missing `display_name`, `state`, or `last_seen_at` as `-`.
- Preserve the daemon payload exactly for `--json`; do not add marker fields,
  sorted order, or short IDs to JSON.

## Acceptance Checklist

Each item maps 1:1 to `docs/issues/25/SPEC.md` "Acceptance / Definition of
done".

1. **GH25-1 (human table).** `striatum repo list` on a daemon-registered
   repository prints a table with display name, repo root, state,
   `last_seen_at`, and shortened repository id; the current repo is marked
   with `*` and sorted first when present.
2. **GH25-2 (no SQLite preflight on list).** `repo list` does not call
   `repo_is_migrated(...)` or inspect `.striatum/state.sqlite3`; it uses the
   daemon `repo.list` registry as the sole state source after the socket
   reachability check.
3. **GH25-3 (`--json` unchanged).** `striatum repo list --json` still routes
   to `repo.list`, sends `{}` params, and prints the same
   `{"ok": true, "data": {"repositories": [...]}}` envelope and repository
   row fields existing consumers parse today.
4. **GH25-4 (no `repo_not_migrated` from list).** `repo list` never reports
   `repo_not_migrated`. When the daemon is unavailable, it reports
   `daemon_unreachable` with the normal daemon startup guidance.
5. **GH25-5 (tests).** Tests cover a daemon-registered table render and the
   daemon-unreachable path, and they assert the legacy SQLite-state error is
   not emitted for `repo list`.

## Verification Commands

Run at least:

```bash
make lint
make typecheck
pytest tests/test_cli_daemon_rpc_route.py tests/exit_codes/test_rfc0043_refusals.py tests/daemon_pg/test_repo_registration.py
```

Manual daemon round trip:

```bash
striatum daemon doctor --apply-migrations --json
striatum daemon start
striatum repo list
striatum repo list --json
```

Manual expected results:

- `striatum repo list` prints the human table and marks the current
  registered repo with `*`.
- `striatum repo list --json` has the same `data.repositories[]` shape as
  before the change.
- With `STRIATUM_DAEMON_SOCKET` pointed at a missing socket,
  `striatum repo list` exits through `daemon_unreachable`.

## Risks

- Operator scripts that incorrectly parsed human-mode `repo list` stderr for
  `repo_not_migrated` will change behavior. That is acceptable because the
  previous error was misleading for a read-only registry command; JSON mode is
  the stable scripting interface and remains unchanged.
- The active tests that intentionally assert `repo_not_migrated` should stay
  focused on non-list commands in `tests/exit_codes/test_rfc0043_refusals.py`.
  The older direct legacy-SQLite split-brain fixture has been retired; add a
  `repo list` exception test beside the active refusal coverage.
- If the implementer formats the table inside `daemon_rpc_route.py`, keep the
  formatter pure and testable. Avoid routing non-JSON mode through
  `client_admin.repo_list()`, which would reintroduce a direct PG/admin path
  where daemon RPC is already available.
