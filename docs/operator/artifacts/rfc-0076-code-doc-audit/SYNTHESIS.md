---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["authority_runtime_findings", "docs_decision_drift_findings", "operator_adoption_findings"]
---

# RFC 0076 Audit Synthesis
author: synthesizer-codex-001
status: open
date: 2026-05-22

## Summary

The first RFC 0076 audit run found no critical live-state authority
regression and no evidence that production workflow state has fallen back to
repo-local SQLite, marker files, terminal output, or provider hooks. The
material work is a focused set of medium-severity fixes: one work-packet
reliability bug, one MCP authority-surface ambiguity, several current-docs
status updates, and two operator onboarding gaps.

The highest priority is to fix prompt-path resolution in work packets because
this run hit the issue directly: the packet pointed at
`prompts/synthesize_audit.md`, while the actual prompt lives under the
workflow tree at
`docs/operator/workflows/rfc-0076-code-doc-audit/prompts/synthesize_audit.md`.
The second priority is to decide and test whether hidden workflow-authoring
daemon MCP methods are callable by name. The remaining items are docs and
operator-guide improvements that should land with the remediation plan.

## Merge Table

| Synth ID | Source lane references | Disposition |
|---|---|---|
| SYN-001 | authority-runtime AUD-001 | Keep as material implementation gap. |
| SYN-002 | authority-runtime AUD-002 | Keep as material authority ambiguity. |
| SYN-003 | docs-decision-drift AUD-001 | Keep as docs drift. |
| SYN-004 | docs-decision-drift AUD-002 | Keep as RFC-status/docs drift, gated on this run completing. |
| SYN-005 | docs-decision-drift AUD-003; operator-adoption AUD-002 | Merge duplicate terminology/private-memory finding. |
| SYN-006 | docs-decision-drift AUD-004 | Keep as roadmap drift. |
| SYN-007 | operator-adoption AUD-001 | Keep as operator ergonomics. |
| SYN-008 | operator-adoption AUD-003 | Keep as operator ergonomics. |
| SYN-009 | operator-adoption AUD-004 | Keep as low-priority source/docs ergonomics. |
| SYN-010 | operator-adoption AUD-005 | Keep as operator onboarding docs gap. |

## Priority Order

1. **SYN-001: Resolve workflow-local prompt paths in work packets.**
   Medium severity, implementation gap, owner surface: packet generation,
   workflow validation/tests. Follow-up: source/test work.
2. **SYN-002: Clarify hidden MCP workflow-authoring execution.**
   Medium severity, authority category, owner surface: Go MCP tools/call,
   method exposure policy, tests/docs. Follow-up: decision clarification plus
   source/test work.
3. **SYN-003 and SYN-004: Update RFC/workflow status docs after this run.**
   Medium severity, docs drift/RFC status, owner surface: RFC index,
   workflow-type docs, operator brief. Follow-up: docs fix.
4. **SYN-007 and SYN-008: Add day-to-day operator watching and recovery
   triage guidance.**
   Medium severity, operator ergonomics, owner surface: human/operator docs.
   Follow-up: docs fix.
5. **SYN-010: Add non-Linux/Postgres setup notes.**
   Medium severity, operator ergonomics, owner surface: setup docs. Follow-up:
   docs fix.
6. **SYN-005, SYN-006, SYN-009: Clean up terminology, runway pointers, and
   starter-workflow rationale.**
   Low severity, owner surface: glossary/context docs, roadmap/brief, adopt
   output/docs. Follow-up: docs fix or small source work.

## Material Findings

### SYN-001: Work-packet prompt paths are not resolved relative to workflow files

severity: medium
category: implementation_gap
owner_surface: packet generation; workflow validation; Go/Python packet
builders
follow_up: source/test work
source_refs: authority-runtime AUD-001

The authority lane found that packet builders copy `task_prompt.path` as stored
in the workflow instead of resolving it relative to the workflow file. Evidence
cited includes `go/pkg/mutations/claim.go:272`,
`src/striatum/daemon_pg/handlers/context.py:765`,
`src/striatum/daemon_pg/handlers/run_lifecycle/run_prepare.py:134-137`, and
the RFC 0076 workflow entries such as
`docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json:119-121`.

This is confirmed by this synthesis packet: the work packet named
`prompts/synthesize_audit.md`, but that path does not exist at repo root; the
actual prompt is
`docs/operator/workflows/rfc-0076-code-doc-audit/prompts/synthesize_audit.md`.
The issue does not create a second live-state authority, but it makes fresh
agents infer path resolution and can cause unnecessary blocking.

Recommended action: define the resolution contract and emit either a
repo-relative resolved path or both `workflow_relative_path` and
`resolved_path`. Add coverage for workflow trees below
`docs/operator/workflows/...` with local prompt files.

### SYN-002: Hidden MCP workflow-authoring methods may still execute by name

severity: medium
category: authority
owner_surface: Go MCP `tools/call`; daemon method exposure policy; MCP tests
follow_up: decision clarification plus source/test work
source_refs: authority-runtime AUD-002

The authority lane found a policy mismatch: native Go MCP hides workflow
authoring methods from `tools/list`, but `tools/call` appears to forward
arbitrary registered method names to daemon RPC. Evidence cited includes
`go/pkg/mcp/capabilities.go:60-70`,
`go/pkg/mcp/tools.go:27-38`, `go/pkg/mutations/mutations.go:82-84`, and
`go/pkg/mcp/http_test.go:289-290`.

There is no token bypass in the finding: a write-capable token is still
required for file-writing methods. The ambiguity is whether hidden
production tools are unsupported execution surfaces or merely omitted from
discovery. The command authority matrix classifies workflow authoring helpers
as local authoring/service cleanup debt, so execution through daemon MCP needs
an explicit policy.

Recommended action: either deny hidden production tools in `tools/call` and
test denial for write-capable tokens, or document that hiding is only a UX
filter and update the authority matrix/MCP docs accordingly.

### SYN-003: RFC 0050 MCP index status lags landed source

severity: medium
category: docs_drift
owner_surface: `docs/rfcs/README.md`
follow_up: docs fix
source_refs: docs-decision-drift AUD-001

The docs lane found that the RFC index still describes
`0050-go-daemon-http-sse-mcp.md` as an accepted active roadmap, while
`docs/ROADMAP.md:51-80` and `docs/operator/BRIEF.md:19-31` say Phase A-D,
the agent-loop PTY bootstrap, and Python MCP deletion have landed. The lane
also verified `src/striatum/mcp.py` is absent and the Go agent-loop bootstrap
files exist.

Recommended action: update the RFC index to say the native Go MCP and
agent-loop bootstrap slices are implemented, while preserving the numbering
collision and remaining CLI-retirement follow-up.

### SYN-004: RFC 0076 docs still present the shape as abstract proposal only

severity: medium
category: rfc_status
owner_surface: RFC 0076 index row; `docs/WORKFLOW_TYPES.md`; operator brief
follow_up: docs fix after this run completes
source_refs: docs-decision-drift AUD-002

The docs lane found that RFC 0076 remains listed as proposed and
`docs/WORKFLOW_TYPES.md` still tells operators to start from the RFC until a
runnable example or generator shape lands. This branch now contains
`docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`, prompts, and
the artifact set from this first run. The lane also noted that the
docs/decision lane artifact was recovered by operator override after the
Claude lane stalled.

Recommended action: after this run reaches terminal state, update the RFC
index, workflow-type guide, and operator brief to reflect that the first
scaffolded operator workflow has run and that follow-up work is tracked from
the remediation plan.

### SYN-005: "Private project memory" needs a product-facing definition

severity: low
category: docs_drift
owner_surface: ubiquitous language; context hygiene docs; RFC 0076 wording
follow_up: docs fix
source_refs: docs-decision-drift AUD-003; operator-adoption AUD-002

Two lanes independently raised the same issue. RFC 0076 uses "private project
memory" as an adoption criterion, while `docs/UBIQUITOUS_LANGUAGE.md` and
`docs/CONTEXT_HYGIENE.md` do not define it. The adoption lane framed the
missing distinction as repo-shared context versus private, machine-local
operator memory.

Recommended action: add a glossary entry and context-hygiene note that define
private project memory and explain which facts belong in committed repository
docs, operator artifacts, `AGENTS.md`, or local/private notes.

### SYN-006: RFC 0077 should own the next liveness slice in current runway docs

severity: low
category: roadmap_drift
owner_surface: operator brief; roadmap
follow_up: docs fix
source_refs: docs-decision-drift AUD-004

The docs lane found that the new RFC 0077 liveness draft exists, but
`docs/operator/BRIEF.md:59-61` and `docs/ROADMAP.md:75-79` still frame the
activity timestamp/deadline work as the next RFC 0075 slice. RFC 0075 remains
the broader tmux-observable session umbrella, but RFC 0077 is the narrower
implementation target.

Recommended action: update the brief and roadmap pointers so MCP activity
timestamps and deadline classification route through RFC 0077.

### SYN-007: Operator docs need a concrete tmux/PTY watching guide

severity: medium
category: operator_ergonomics
owner_surface: `docs/USING_STRIATUM.md`; `docs/HOW_TO_HUMAN.md`;
CLI/reference docs
follow_up: docs fix
source_refs: operator-adoption AUD-001

The adoption lane found that docs mention supervision but do not give a
clear operator-facing "how to watch the agent" guide. Evidence cited includes
`docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`, and an internal issue spec
that mentions tmux naming but is not an operator guide.

Recommended action: add a short guide with the current tmux/session naming
contract, attach command shape, and the reminder that tmux panes are for
inspection only and are not workflow state.

### SYN-008: Recovery options need a dashboard/status triage table

severity: medium
category: operator_ergonomics
owner_surface: `docs/HOW_TO_HUMAN.md`; `docs/USING_STRIATUM.md`
follow_up: docs fix
source_refs: operator-adoption AUD-003

The adoption lane found that first-time operators can see many recovery verbs
without a direct mapping from common dashboard/status states to the right
action. The cited docs list recovery commands and escalation playbooks, but
do not provide a concise table for states such as stale leases.

Recommended action: add a triage table mapping visible states to commands
such as `recovery stale-leases`, `recovery requeue-stale`,
`recovery process-reconcile`, `recovery resume`, and escalation paths.

### SYN-009: Starter workflow suggestion needs rationale or a direct pointer

severity: low
category: operator_ergonomics
owner_surface: `striatum adopt` output; usage docs
follow_up: source work or docs fix
source_refs: operator-adoption AUD-004

The adoption lane found that `striatum adopt` mentions a suggested starter
workflow path without making the selection logic transparent. The risk is not
authority-related; it is that users may treat the suggestion as product
judgment instead of consulting `docs/WORKFLOW_TYPES.md`.

Recommended action: include a one-line rationale or link/pointer to
`docs/WORKFLOW_TYPES.md` in adopt output and usage docs.

### SYN-010: PostgreSQL setup docs need common non-Linux paths

severity: medium
category: operator_ergonomics
owner_surface: `docs/POSTGRES_TRANSITION.md`; getting-started docs
follow_up: docs fix
source_refs: operator-adoption AUD-005

The adoption lane found that the PostgreSQL provisioning docs assume the
operator can run `sudo -u postgres psql`. That is good for many Linux
systems, but less useful for Postgres.app/macOS or managed database setups.

Recommended action: add an alternative-platform note that explains common
non-Linux setup patterns while preserving the local-first, operator-owned
Postgres boundary.

## Conflicts And Resolutions

No lane-level findings directly contradict each other. The only material
overlap is the duplicated private-project-memory terminology gap:
docs-decision-drift AUD-003 and operator-adoption AUD-002 are the same
underlying issue and are merged as SYN-005.

There is one evidence-quality caveat: the docs-decision-drift artifact was
operator-produced after the original Claude lane stalled. Its file states the
recovery explicitly, and the packet/run evidence should treat it as an
operator override rather than a normal lane-produced artifact. The findings
are still useful because they cite concrete docs and source paths; the
remediation plan should preserve the provenance note.

The authority-runtime lane's AUD-002 remains intentionally unresolved at the
policy level. The evidence shows a discovery/execution mismatch, but the
correct behavior depends on whether hidden MCP workflow-authoring methods are
meant to be fail-closed or callable-by-name with write capability.

## Non-Material Or Historical-Only Items

The authority lane explicitly did not find current evidence of repo-local
SQLite reopening for production live workflow state, retained retired RPC
names such as `apply.reviewed_patch`, or marker/terminal authority. These are
guardrail confirmations, not new action items.

Historical dogfood artifacts and older prompts remain provenance. No finding
requires rewriting historical records; the requested changes target current
docs, current workflow packets, current MCP policy, and current operator
guides.

## Open Questions

- Should workflow-local `task_prompt.path` become a resolved repo-relative
  path in packets, or should packets carry both workflow-relative and resolved
  fields for compatibility?
- Are MCP hidden production tools unsupported execution surfaces or merely
  undiscovered tools that remain callable with the right capability?
- After the remediation plan lands, should RFC 0076 advance from proposed to
  accepted, or should acceptance wait for generator/catalog integration?
