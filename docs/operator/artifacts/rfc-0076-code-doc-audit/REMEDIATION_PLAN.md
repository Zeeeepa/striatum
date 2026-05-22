---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["audit_synthesis", "authority_runtime_findings", "docs_decision_drift_findings", "operator_adoption_findings"]
---

# RFC 0076 Audit Remediation Plan
author: planner-codex-001
status: open
date: 2026-05-22

## Summary

The RFC 0076 audit found no critical authority regression and no evidence
that production workflow state has reopened repo-local SQLite, marker files,
terminal output, or provider hooks as live authority.

The remediation path is therefore bounded:

1. Fix the work-packet prompt-path reliability bug.
2. Resolve the MCP hidden-tool execution policy and pin it with tests.
3. Update current docs to reflect landed RFC 0050/RFC 0076/RFC 0077 state.
4. Add operator-facing guidance for tmux watching, recovery triage, and
   common PostgreSQL setup variants.
5. Record the confirmed no-action guardrail items as historical-only or
   already-covered, not as new work.

No high or critical findings were reported. All medium findings below are
material and should be tracked; low findings can batch with adjacent doc or
ergonomics work.

## Follow-Up Map

| ID | Severity | Source finding(s) | Follow-up path | Owner surface | Required action |
|---|---|---|---|---|---|
| REM-001 | medium | SYN-001; authority-runtime AUD-001 | Source/test work; existing RFC 0076 follow-up | Go/Python packet builders, run-prepare storage, workflow validation tests | Define task-prompt path resolution and emit reliable packet paths. |
| REM-002 | medium | SYN-002; authority-runtime AUD-002 | Decision clarification plus source/test work | Go MCP `tools/call`, MCP docs, command authority matrix | Decide whether hidden MCP workflow-authoring methods fail closed or remain callable by name; implement and document the chosen policy. |
| REM-003 | medium | SYN-003; docs-decision-drift AUD-001 | Docs fix | `docs/rfcs/README.md` | Mark RFC 0050 MCP slices as implemented through native Go MCP, agent-loop PTY bootstrap, and Python MCP deletion while preserving CLI-retirement follow-up. |
| REM-004 | medium | SYN-004; docs-decision-drift AUD-002 | Docs fix; possible RFC status decision after completion | RFC 0076, `docs/rfcs/README.md`, `docs/WORKFLOW_TYPES.md`, operator brief | After this run completes, state that the first runnable RFC 0076 workflow has run and link follow-up work to this plan. |
| REM-005 | low | SYN-005; docs-decision-drift AUD-003; operator-adoption AUD-002 | Docs fix | `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CONTEXT_HYGIENE.md` | Define "private project memory" and contrast it with repo-shared context and operator artifacts. |
| REM-006 | low | SYN-006; docs-decision-drift AUD-004 | Docs fix | `docs/operator/BRIEF.md`, `docs/ROADMAP.md` | Route MCP activity timestamp/deadline work through RFC 0077, with RFC 0075 as the broader umbrella. |
| REM-007 | medium | SYN-007; operator-adoption AUD-001 | Docs fix | `docs/USING_STRIATUM.md`, `docs/HOW_TO_HUMAN.md`, CLI reference if needed | Add a concise "watching agent sessions" guide with tmux attach shape and the no-terminal-authority reminder. |
| REM-008 | medium | SYN-008; operator-adoption AUD-003 | Docs fix | `docs/HOW_TO_HUMAN.md`, `docs/USING_STRIATUM.md` | Add a recovery triage table mapping dashboard/status symptoms to recovery commands. |
| REM-009 | low | SYN-009; operator-adoption AUD-004 | Small source/docs ergonomics | `striatum adopt` output and usage docs | Add either a one-line rationale for the suggested starter workflow or a direct pointer to `docs/WORKFLOW_TYPES.md`. |
| REM-010 | medium | SYN-010; operator-adoption AUD-005 | Docs fix | `docs/POSTGRES_TRANSITION.md`, getting-started docs | Add common non-Linux PostgreSQL setup notes, especially macOS/Postgres.app and non-`sudo` role provisioning. |
| REM-011 | info | Authority guardrail confirmations | Wontfix/no new action | Existing guardrails and architecture docs | Keep as evidence only: no current SQLite, retired RPC, marker-file, terminal-output, or provider-hook authority regression was found. |

## Implementation Order

### 1. Packet prompt-path fix

Address REM-001 first because this run directly hit the bug. The packet
exposed workflow-local prompt paths as repo-root paths, causing missing
prompt reads even though the workflow tree contains the prompt files.

Recommended shape:

- Define the compatibility contract in docs or code comments before editing
  both implementations.
- Prefer emitting a repo-relative resolved path while preserving enough
  information to diagnose the workflow-local source path.
- Update both packet builders cited by the authority audit:
  `go/pkg/mutations/claim.go` and
  `src/striatum/daemon_pg/handlers/context.py`.
- Add regression coverage using a workflow stored below
  `docs/operator/workflows/...` with prompts in a sibling `prompts/`
  directory.

This is source/test work. It does not require a new RFC unless path semantics
need a backward-incompatible packet schema change.

### 2. MCP hidden-tool policy

Address REM-002 before expanding the MCP control-plane surface further. The
finding is not a capability bypass; it is a policy mismatch between discovery
and execution for hidden workflow-authoring methods.

Decision options:

- **Fail closed:** hidden production tools are not callable through MCP
  `tools/call`, even with write capability. Implement a denial path in Go MCP
  and add tests for write-capable tokens.
- **Callable by name:** hidden tools are omitted from discovery only, but may
  be invoked by a caller with the required capability. Update MCP docs and
  `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` so operators do not treat
  `tools/list` as the full execution policy.

This should produce either a decision-log row or a narrow RFC/status note if
the chosen policy changes the public MCP contract.

### 3. Current-status docs batch

Batch REM-003, REM-004, and REM-006 after this workflow reaches terminal
state:

- Update the RFC 0050 MCP index row to distinguish implemented native Go MCP
  slices from remaining CLI-retirement work.
- Update RFC 0076 docs and workflow-type guidance to say the first runnable
  operator workflow exists and has completed with one operator-recovered lane.
- Update operator runway docs so RFC 0077 owns the MCP activity timestamp and
  liveness-deadline implementation slice.

This is docs-only work unless the owner decides RFC 0076 should move from
`proposed` to `accepted`; that status change should update both the RFC file
and `docs/rfcs/README.md`.

### 4. Operator adoption docs batch

Batch REM-005, REM-007, REM-008, and REM-010:

- Add a glossary/context-hygiene definition for private project memory.
- Add a practical tmux/PTY watching section for operators.
- Add a recovery triage table keyed by visible dashboard/status symptoms.
- Add PostgreSQL setup alternatives for operators without Linux-style
  `sudo -u postgres` access.

These are docs-only corrections and do not require new product decisions.
Keep the no-transcript/no-terminal-authority boundary explicit in the tmux
watching guide.

### 5. Starter-workflow ergonomics

REM-009 can land as a small source/docs improvement. The minimal source
change is to make `adopt` output explain why it suggests a starter path or
point directly at `docs/WORKFLOW_TYPES.md`. Tests should assert the output
contains the rationale or pointer so the guidance does not drift.

## No-Action Items

REM-011 is explicitly no new work. The audit confirmed existing guardrails:
production workflow state remains daemon/PostgreSQL-owned; retired RPC names
are not production methods; marker files, tmux panes, terminal output, and
provider hooks are not authoritative state.

Do not rewrite historical dogfood artifacts or older prompts for these
confirmations. Preserve them as provenance unless a current doc claims their
behavior is live.

## Tracking Recommendation

Track REM-001 and REM-002 as separate source/test work items because they
touch packet reliability and MCP authority policy. Track REM-003 through
REM-010 as one docs/ergonomics cleanup branch if the operator wants a small
follow-through pass.

After REM-001 through REM-004 land, RFC 0076 should be eligible for an owner
decision on whether to mark the workflow shape accepted or wait for
generator/catalog integration.
