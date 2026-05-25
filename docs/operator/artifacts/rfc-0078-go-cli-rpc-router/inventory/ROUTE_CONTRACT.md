---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Go CLI RPC Router Contract
author: operator [self-declared: router-mapper-codex-gpt-5-001]

## Source

`contracts/daemon_methods.json` contains 67 `cli_routes[]` entries. This gate treats every listed route as daemon-backed unless it is absent from `cli_routes[]` and registered in the local-command boundary.

## Daemon Route Families

- Single-command reads: `status`, `why`, `doctor`, `dashboard`, `inbox`.
- Two-part reads: `git snapshot`, `repo list`, `workflow accepted-risks`, `list runs|sessions|jobs|artifacts|workflows`, `run summary|graph`, `evidence export`, `corpus export`, `archive create`, `escalation list|show`, `worktree list`, `supervise status|list`, `cross-repo list|describe|why`.
- Mutations and admin/recovery routes: `git commit-apply`, `repo add|remove`, `workflow accept-risk`, `run prepare|start|pause|resume|cancel|retry-job`, `register-session`, `session close`, `claim-next`, `ack`, `heartbeat`, `release`, `send`, `block`, `complete`, `publish-artifact`, `verdict`, `submit-review`, `override-verdict`, `recovery stale-leases|requeue-stale|cancel-job|process-reconcile|resume|auto|auto-publish|auto-finalize`, `decision record`, `checkpoint resolve`, `escalation resolve`, `branch confirm`, `worktree create|release`, `supervise start|send|stop`, `cross-repo cancel`.

## Local Exceptions

`workflow validate` remains a local workflow-authoring command in this gate. It validates a workflow file before daemon state necessarily exists, preserves the first RFC 0078 Go CLI slice, and is explicitly excluded from generated daemon route metadata. Other local authoring surfaces named in docs but not present in `cli_routes[]` are deferred to the workflow/artifact parity gate unless a future contract adds a daemon route.

## Parameter Groups

Read-style groups are routed through `go/pkg/cli/readparams`: `status`, `why`, `doctor`, `dashboard`, `git_snapshot`, `repo_list`, `workflow_accepted_risks_list`, `list`, `run_summary`, `run_graph`, `evidence_export`, `corpus_export`, `archive_create`, `inbox`, `escalation_list`, `escalation_show`, `worktree_list`, `supervise_status`, `supervise_list`, `cross_repo`.

Mutation-style groups are routed through `go/pkg/cli/mutationparams`: all non-read capabilities from the generated route metadata, including admin, claim, write, review, apply, and recovery routes.

## Validation Set

- `go generate ./pkg/cli/routes`
- `go test ./pkg/cli/routestest -run TestGeneratedRoutesMatchDaemonMethodContract -count=1`
- `go test ./pkg/cli/... ./pkg/rpc/...`
- `go test ./cmd/striatum` once the parallel `workflowauthoring` compile drift is resolved.
