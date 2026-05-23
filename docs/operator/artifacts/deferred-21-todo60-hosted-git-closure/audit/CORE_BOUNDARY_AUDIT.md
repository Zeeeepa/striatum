---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/SPEC.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0067-optional-git-pr-integration.md", "contracts/daemon_methods.json", "src/striatum/cli/daemon_rpc_route.py", "go/pkg/reads/git_snapshot.go", "go/pkg/mutations/git_commit_apply.go", "go/pkg/reads/git_snapshot_test.go", "go/pkg/mutations/git_commit_apply_test.go", "tests/test_cli_daemon_rpc_route.py", "tests/test_artifact_schemas.py", "tests/test_mcp_mutation_capabilities.py", "go/pkg/mcp/http_test.go"]
---

# TODO 60 Core Boundary Audit
author: todo60-auditor-codex-gpt-5-001

## Finding

No D127 source violation was found in the TODO 60 core Git/PR slice.

Core Striatum currently exposes two Git methods in the daemon method contract:
`git.snapshot` and `git.commit_apply`. There is no core hosted PR creation,
hosted PR update, provider push, provider login, provider SDK, or provider
credential loading surface in this slice.

## Evidence

- D127 says read-only local Git snapshots may proceed, durable commit/PR
  request artifacts may be added, local commit apply may create a local commit
  only after explicit operator confirmation, and hosted provider actions stay
  out of core until a later optional-plugin decision.
- `docs/SPEC.md` says `git snapshot --json` reports local branch, HEAD,
  dirty counts, changed paths, and bounded ancestry without fetching, pushing,
  committing, reading remote URLs, importing hosted-provider SDKs, or including
  diff hunks or commit bodies.
- `docs/SPEC.md` says `git commit-apply` is daemon-routed, requires `apply`
  capability, consumes a confirmed `commit_request` artifact, verifies base
  HEAD, branch, and dirty-path scope, creates a local commit only, disables
  hooks, and does not push, fetch, call hosted providers, import provider
  SDKs, or load provider credentials.
- `contracts/daemon_methods.json` and the Go registry list `git.snapshot`
  as read-capable and `git.commit_apply` as apply-capable. They do not list
  any hosted provider or PR mutation method.
- `go/pkg/reads/git_snapshot.go` restricts the snapshot handler to local
  `rev-parse`, `status`, and `log`; its argument validator rejects `fetch`,
  `pull`, `push`, `remote`, `ls-remote`, `commit`, checkout/switch, merge,
  rebase, reset, add, restore, and tag.
- `go/pkg/mutations/git_commit_apply.go` requires `confirm=true`, a matching
  `confirm_request_id`, and a confirmed request artifact before local commit
  creation. Its result explicitly reports `pushed: false`,
  `hosted_provider_invoked: false`, and `provider_credentials_loaded: false`.
- `go/pkg/reads/git_snapshot_test.go` includes a fake-git test proving
  `git.snapshot` only invokes read-only local commands and not provider,
  network, commit, or push commands.
- `go/pkg/mutations/git_commit_apply_test.go` proves explicit confirmation is
  required, unconfirmed requests do not move HEAD, dirty paths outside
  `included_paths` are refused, hooks are disabled, and provider/network
  commands are not invoked.
- `tests/test_cli_daemon_rpc_route.py` verifies the CLI routes only bounded
  read params for `git.snapshot` and explicit confirmation params for
  `git.commit_apply`.
- `tests/test_artifact_schemas.py` registers `commit_request` and
  `pr_request` as schema-bearing durable artifact kinds, not hosted actions.
- The provider SDK import scan for `go-github`, `go-gitlab`, `PyGithub`,
  `python-gitlab`, `github3`, `ghapi`, and `octokit` returned no matches in
  core source or dependency manifests.

## Ambiguity

`src/striatum/web/run_list.py` has a pre-existing GitHub tree-link
presentation helper that parses `remote.origin.url`. That is not TODO 60
hosted provider action work: it does not call a hosted provider API, import a
provider SDK, push/fetch, or load credentials. It is outside the landed
`git.snapshot` response and does not require a TODO 60 repair packet.

## Conclusion

The current source preserves D127's no-hosted-provider/no-SDK/no-push core
boundary. No source or test repair is required for this deferred closure.
