# GH #25 — striatum repo list (no --json) returns repo_not_migrated for already-registered repos

Source: https://github.com/halbritt/striatum/issues/25

## Summary

`striatum repo list` (without `--json`) refuses with `repo_not_migrated: <path> has not been migrated to daemon PostgreSQL state` even when the repo IS active in the daemon's registry. The same command with `--json` works correctly and returns the registered repository row.

```
$ striatum repo list
repo_not_migrated: /home/halbritt/git/striatum has not been migrated to daemon PostgreSQL state
SQLite import windows are closed. Archive or remove any legacy .striatum/state.sqlite3 file, then register the repository with `striatum adopt --repo /home/halbritt/git/striatum` or `striatum repo add --init /home/halbritt/git/striatum`.

$ striatum repo list --json | jq '.data.repositories[0]'
{
  "display_name": "striatum",
  "repository_id": "repo_a89ecd1664...",
  "repo_root": "/home/halbritt/git/striatum",
  "state": "active",
  ...
}
```

The non-JSON path is pre-flighting an on-disk `.striatum/state.sqlite3` check and refusing if it's absent (or present-but-not-migrated), instead of consulting the daemon's PostgreSQL repo registry. The `--json` path correctly queries the daemon.

## Why this matters

This is the first command an operator runs on a fresh shell to figure out which repos are registered. The non-JSON path's misleading error sends them down the wrong path — "register with adopt or repo add" — when the answer is "your repo is already registered; use --json to see it."

Hit during the GH #22 / #23 / #24 operator session on 2026-05-18 → 2026-05-19. The error pointed at archiving `.striatum/state.sqlite3` (which is now retired operational state), which then made `adopt` fail with `active repository path is occupied by a different repo identity` because the daemon already knew about the repo with a different inode signature. The whole detour was unnecessary — the repo was already registered.

## Acceptance / Definition of done

A solution must satisfy each of:

1. **`striatum repo list` (no --json) on a daemon-registered repo prints a human-readable table** of registered repositories (display_name, repo_root, state, last_seen_at, repository_id-short) — not the misleading `repo_not_migrated` error. The repo at the operator's cwd should be highlighted or sorted first if it's in the list.
2. **The SQLite-presence pre-flight check is removed from the `list` path.** The check belongs in `striatum adopt` / `striatum repo add --init` where the operator is actually trying to set up a repo. A read-only listing command must NOT depend on local file state at all; it should query the daemon's repo registry as its sole source of truth.
3. **`striatum repo list --json` semantics unchanged.** This already works correctly and must keep working byte-for-byte for any consumer parsing it.
4. **Operator never sees `repo_not_migrated` from `repo list`** anymore. If the daemon is unreachable, the error must be `daemon_unreachable: <details>`, not a SQLite-state error.
5. **Tests cover both:** (a) `repo list` on a daemon-registered repo prints the table; (b) `repo list` when the daemon is unreachable reports `daemon_unreachable` (or the equivalent connection-error message), not the legacy SQLite-state error.

## Suggested fix

Find the dispatch path that handles `repo list` (no `--json`) in `src/striatum/cli/` and:

1. Remove the SQLite-presence pre-flight.
2. Call the same daemon RPC the `--json` path uses (`striatumd repository.list` or similar — confirm during triage).
3. Format the result as a small table; if the cwd happens to be one of the listed repos, mark it (e.g., `*` prefix).
4. Surface daemon-connection failures with a clean `daemon_unreachable` message.

If there's a shared helper that pre-flights the SQLite check across multiple read verbs, the fix may need to drop that helper from the read path or gate it on a mutation flag. Triage should confirm.

## Provenance

Filed during the GH #22 / #23 / #24 operator session 2026-05-18 → 2026-05-19. Specifically: when investigating whether the local striatum repo was registered with the daemon before launching the issue workflows, `repo list` (no --json) misdirected; `repo list --json` revealed the truth. See https://github.com/halbritt/striatum/issues/25.

Severity: low (workaround is `--json`), but it's the kind of paper-cut that compounds operator-onboarding friction.
