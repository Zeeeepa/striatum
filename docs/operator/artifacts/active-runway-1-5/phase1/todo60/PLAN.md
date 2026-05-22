---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/DECISION_LOG.md", "docs/operator/BRIEF.md", "docs/operator/plans/active-runway-1-5.md", "docs/rfcs/0067-optional-git-pr-integration.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/rfcs/0077-mcp-activity-liveness-deadlines.md"]
---

# TODO 60 Read-Only Git Snapshot Plan
author: git-snapshot-planner-codex-001
status: open
date: 2026-05-22

## Boundary

D127 unblocks only the read-only local Git snapshot slice. Striatum core must
not create commits, push branches, call hosted providers, import provider SDKs,
store hosted-provider identifiers, add telemetry, or make GitHub/GitLab/etc.
behavior part of the daemon contract.

This slice should answer one operator question: "What local Git state did the
daemon observe for this registered target repository?" It is a daemon read, not
a workflow mutation and not a commit or PR integration.

## Daemon Method Shape

Add canonical daemon method `git.snapshot`.

Method contract:

| Property | Value |
|---|---|
| required capability | `read` |
| repository scope | `single_repo` |
| audit class | `metadata` |
| params schema | V1 |
| production handler | Go read handler |
| Python handler | optional compatibility client helper only, not authority |
| MCP visibility | visible to read-capable repo-scoped tokens |

Request params:

```json
{
  "schema_version": 1,
  "include_ancestry": true,
  "ancestry_limit": 10
}
```

`include_ancestry` defaults to `true`. `ancestry_limit` defaults to `10`, must
be bounded, and should cap at `50` to keep MCP/UI responses compact.

Response fields:

```json
{
  "schema_version": "striatum.git_snapshot.v1",
  "repository_id": "repo_...",
  "repo_path": "/registered/target/repo",
  "observed_at": "2026-05-22T12:00:00Z",
  "git_available": true,
  "is_git_repository": true,
  "branch": {
    "name": "striatum/active-runway-1-5",
    "detached": false,
    "upstream": "origin/main",
    "ahead": 1,
    "behind": 0
  },
  "head": {
    "sha": "40-hex",
    "short_sha": "12-hex",
    "subject": "commit subject",
    "author_date": "2026-05-22T10:00:00Z",
    "committer_date": "2026-05-22T10:00:00Z"
  },
  "dirty": {
    "is_dirty": true,
    "tracked_modified": 2,
    "staged": 0,
    "untracked": 1,
    "conflicted": 0,
    "renamed": 0,
    "deleted": 0
  },
  "changed_paths": [
    {
      "path": "docs/TODO.md",
      "index_status": "M",
      "worktree_status": " ",
      "kind": "modified",
      "old_path": null
    }
  ],
  "ancestry": {
    "limit": 10,
    "commits": [
      {
        "sha": "40-hex",
        "short_sha": "12-hex",
        "parents": ["40-hex"],
        "author_date": "2026-05-22T10:00:00Z",
        "committer_date": "2026-05-22T10:00:00Z",
        "subject": "commit subject"
      }
    ]
  }
}
```

The snapshot deliberately excludes diff hunks, commit bodies, remote URLs,
credential material, hosted PR metadata, and transcript-like free text.
Subjects are acceptable local commit metadata; bodies should stay out of V1 to
avoid turning this into a broad provenance export.

Error behavior:

- no `git` binary: return `git_available: false` plus a stable read error code
  such as `git_unavailable` only when the caller requested strict failure;
- registered path is not a Git repository: return `is_git_repository: false`;
- Git command failure: fail the method with `git_snapshot_failed` and a bounded
  message that does not include environment dumps or credentials.

## Implementation Notes

Implement the production read in Go under a new package such as
`go/pkg/reads/gitsnapshot` or as a small read handler beside existing read
handlers. The handler should execute only local `git -C <registered path>`
commands from a closed allowlist:

- `rev-parse --abbrev-ref HEAD`
- `rev-parse --verify HEAD^{commit}`
- `rev-parse --short=12 HEAD`
- `status --porcelain=v1 --branch`
- `log -n <limit> --format=<NUL/US-delimited format>`
- optionally `rev-list --left-right --count HEAD...@{upstream}` when an
  upstream exists

Use `exec.CommandContext` with a short timeout. Do not invoke a shell. Do not
read `.git/config` directly for remotes. Do not run `fetch`, `pull`, `push`,
`commit`, `checkout`, `switch`, `merge`, `rebase`, `reset`, `add`, `restore`,
`tag`, `remote`, or provider CLIs.

Changed paths should be parsed from porcelain status. Preserve rename source in
`old_path` and destination in `path`. Normalize paths as repository-relative
strings and reject path traversal or absolute path output before returning.

## Surfaces

CLI read surface:

```bash
striatum git snapshot --json
striatum git snapshot --ancestry-limit 20 --json
```

The CLI route maps to `git.snapshot` through the daemon method contract. It
must fail closed when the daemon is unreachable or the repository is not
registered, matching other daemon-required reads.

MCP surface:

- expose `git.snapshot` through `tools/list` for read-capable tokens;
- keep the same response envelope as daemon RPC;
- require `repository_id` or the token's repo scope exactly as other
  single-repo reads do.

UI surface:

- add a compact read-only Git Snapshot panel on run detail and repository
  status views;
- show branch, HEAD short SHA, dirty counts, and changed-path list;
- link changed paths to existing local file view routes only;
- do not add commit, push, PR, provider login, or remote-link buttons.

## Tests

Minimum test set for the first slice:

- Go unit tests for clean repo, dirty tracked file, staged file, untracked file,
  deleted file, renamed file, conflicted status parsing, detached HEAD, no
  upstream, and bounded ancestry limit.
- RPC authorization tests: read token succeeds; wrong repo token fails; write
  capability alone does not imply read unless existing capability policy says
  so; unauthenticated call fails.
- MCP tests: `tools/list` exposes `git.snapshot` to read-capable tokens and
  hides it from unauthorized tokens; `tools/call` returns the same structured
  data as RPC.
- No-mutation tests using a fake `git` executable that records argv and fails if
  any mutating subcommand appears.
- No-network/provider tests that fail if the handler invokes `fetch`, `pull`,
  `push`, `remote`, `ls-remote`, `gh`, `glab`, `hub`, `curl`, or `ssh`.
- Dependency guardrail tests that reject hosted-provider SDK imports in Go,
  Python production source, and frontend source for this slice.
- Authority-matrix guardrail updates: `contracts/daemon_methods.json`,
  generated Go registry/tables, and
  `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` all agree on capability,
  scope, and no CLI fallback.

## Separation From Later Request Artifacts

Later commit-request and PR-request work must remain separate durable artifact
contracts, not hidden behavior behind `git.snapshot`.

Recommended future artifact boundaries:

- `commit_request`: desired commit message, included paths, base HEAD,
  snapshot id/hash, rationale, and explicit operator confirmation status.
- `pr_request`: target branch, summary, body draft, related commit request or
  local commit SHA, and provider/plugin target if a later decision accepts one.

Neither artifact should apply itself. Local commit apply requires a future
explicit operator-confirmed mutation. Hosted PR behavior requires a later
optional-plugin decision for provider operations, credentials, and confirmation
semantics.

## First Implementation Slice

Keep the first source patch small and disjoint from TODO 55/56/59:

1. Add `git.snapshot` to `contracts/daemon_methods.json`, regenerate the Go
   registry and daemon method tables, and update the authority matrix.
2. Implement the Go read handler with branch, HEAD, dirty summary, changed
   paths, and bounded ancestry.
3. Add the CLI route `striatum git snapshot --json` as a daemon-routed read.
4. Expose the method through existing MCP tool discovery/call behavior.
5. Add focused Go/RPC/MCP tests plus the fake-git no-mutation guardrail.

Suggested write scope for that implementation job:

- `contracts/daemon_methods.json`
- `go/pkg/rpc/registry_methods.go`
- `go/pkg/reads/**`
- `go/pkg/mcp/**` tests only if needed
- `src/striatum/cli/parser.py`
- `src/striatum/cli/daemon_rpc_route.py`
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- `docs/architecture/DAEMON_METHOD_TABLES.md`
- `tests/architecture/**`
- targeted Go tests under `go/pkg/**`

Leave UI polish for the next slice after the daemon read is stable. Leave
commit-request and PR-request artifacts out of this implementation packet.
