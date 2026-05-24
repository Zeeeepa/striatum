---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["contracts/daemon_methods.json", "src/striatum/cli/", "go/cmd/striatumd/", "go/pkg/"]
---

# Go CLI Cutover Handoff
author: operator [self-declared: cli-porter-codex-gpt-5-001]

## Finding

Before this workflow, Go provided `striatumd` and
`striatum-supervisor-helper`, but not the user-facing `striatum` CLI. The
Python console script in `pyproject.toml` still owned the operator command.

## Smallest Architecture

The Go CLI should be a thin router:

- parser and nested command table;
- daemon RPC client over the runtime Unix socket;
- route metadata generated from `contracts/daemon_methods.json`;
- typed parameter builders for daemon-routed commands;
- a small local package for workflow authoring/bootstrap commands that must
  touch local files.

The Go CLI must not duplicate daemon mutations or read PostgreSQL directly for
ordinary workflow state.

## Immediate RPC Candidates

Most work-loop, lifecycle, repository, status, review, recovery, evidence,
archive, escalation, worktree, supervision, cross-repo, and workflow-risk
commands already have daemon methods. Local authoring commands already have
Go package support for validation, lint, plan, graph, templates list/show,
generate, init, and upgrade. `workflow templates render-md` remains
Python-only.

## First Slice Landed

This workflow adds `go/cmd/striatum` as the first Go CLI scaffold and ports
`striatum workflow validate` for local authoring validation. It supports
`--json` and `--allow-same-model-pairing`, uses
`go/pkg/workflowauthoring`, and includes tests for valid JSON output and
same-model refusal behavior.

## Remaining Blockers

- Bootstrap/install commands: `init`, `adopt`, skills/plugins, service install.
- Local service/web command: `serve`.
- Offline archive/corpus verification.
- Live terminal helpers: non-JSON `dashboard`, `recovery watch`.
- Operator current-brief, `workflow templates render-md`, and retired parser
  compatibility commands.
