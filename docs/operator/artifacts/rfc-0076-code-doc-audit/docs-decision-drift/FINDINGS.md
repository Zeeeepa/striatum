---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["docs-decision-drift", "operator-recovery", "rfc-0076"]
---

# Docs Decision Drift Findings
author: operator [self-declared: rfc0076-docs]

## Recovery Note

This artifact was produced by the operator after the original
`docs_auditor` Claude session stalled without writing the required artifact.
The run evidence records the recovery as an operator override rather than a
normal lane-produced artifact.

## Scope

This audit checked current docs, RFC index entries, roadmap claims, operator
brief claims, workflow-type docs, and terminology docs for drift against the
current source and the landed RFC 0050/RFC 0075 cutover slices.

## Findings

### AUD-001: RFC 0050 index status still says active implementation roadmap

severity: medium
category: docs_drift
status: open

claim: The RFC index understates the landed RFC 0050 state by keeping the
native Go MCP RFC status as an "accepted (active implementation roadmap)"
entry even though the roadmap and operator brief now describe Phase A-D,
agent-loop PTY bootstrap, and Python MCP deletion as landed.

evidence:

- `docs/rfcs/README.md:65` labels `0050-go-daemon-http-sse-mcp.md` as
  "accepted (active implementation roadmap; numbering collision)".
- `docs/ROADMAP.md:51-80` says native HTTP MCP Phase A-D is landed,
  `go/pkg/agentloop` is a PTY bootstrapper, and `src/striatum/mcp.py` is
  deleted.
- `docs/operator/BRIEF.md:19-31` makes the same landed-source claim.
- `test -e src/striatum/mcp.py` returned missing during this run, and
  `go/pkg/agentloop/bootstrap.go` / `go/pkg/agentloop/loop.go` contain the
  current PTY bootstrap implementation.

impact: Operators reading the RFC index get a weaker and older status than
the live source and roadmap, which can cause duplicate scaffolding or false
assumptions about remaining Phase A-D work.

recommended_action: Update the RFC index row to mark RFC 0050 MCP as
implemented through the landed native Go MCP/agent-loop/Python-deletion
slices while preserving the numbering-collision note and CLI-retirement
follow-up.

follow_up: docs fix

### AUD-002: RFC 0076 is only proposed even though the first runnable workflow now exists

severity: medium
category: docs_drift
status: open

claim: RFC 0076 remains documented as only a proposed shape, while this
branch now contains a validated operator workflow, prompts, and an initial
run artifact set for the shape.

evidence:

- `docs/rfcs/README.md:91` lists RFC 0076 as proposed.
- `docs/WORKFLOW_TYPES.md:369` says to start from RFC 0076 until a runnable
  example or generator shape lands.
- This branch adds
  `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`, which
  validates with `striatum workflow validate --allow-same-model-pairing`.
- The live run `run_741632aa8d7f8d7715f702070f3c1c77` has already completed
  the authority/runtime and operator/adoption audit lanes and is being
  recovered through this artifact for docs/decision drift.

impact: The docs still teach RFC 0076 as an abstract proposal rather than a
new reusable operator workflow seed. That makes the workflow harder to reuse
and weakens the evidence trail for accepting/cataloging the shape.

recommended_action: After the run completes, update the RFC 0076 index,
workflow-type docs, and operator brief to say the first scaffolded workflow
has run, with the Claude lane recovered by operator override and follow-up
work tracked from the remediation plan.

follow_up: docs fix

### AUD-003: "Private project memory" is used as audit vocabulary but is not defined

severity: low
category: terminology_gap
status: open

claim: RFC 0076 asks whether first adopters can operate without "private
project memory", but the ubiquitous language and context-hygiene docs do not
define the term.

evidence:

- `docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md:146` uses
  "private project memory" as a primary audit question.
- `docs/UBIQUITOUS_LANGUAGE.md` defines core Striatum vocabulary but has no
  matching entry.
- `docs/CONTEXT_HYGIENE.md` explains repo-side/session-side/model-side
  context quality practices but does not name the team-shared versus
  machine-local/private memory boundary.
- The operator/adoption lane independently raised the same gap as AUD-002.

impact: The audit criterion is useful but not yet portable to new operators.
Without a definition, reviewers can disagree about whether a missing fact
belongs in repository docs, `AGENTS.md`, local notes, or private operator
memory.

recommended_action: Add a concise vocabulary entry and a context-hygiene note
that distinguishes team-shared repository memory from private, machine-local
operator memory.

follow_up: docs fix

### AUD-004: RFC 0077 liveness draft is not reflected in current operator runway yet

severity: low
category: roadmap_drift
status: open

claim: The new liveness RFC draft exists, but the operator brief and roadmap
still phrase the next liveness slice as part of RFC 0075 rather than the
newer, narrower RFC 0077 draft.

evidence:

- `docs/rfcs/0077-mcp-activity-liveness-deadlines.md` now defines the
  narrower MCP activity timestamp and liveness-deadline slice.
- `docs/operator/BRIEF.md:59-61` still says the next action is to implement
  the next RFC 0075 slice for MCP activity timestamps and liveness.
- `docs/ROADMAP.md:75-79` lists the remaining liveness work under RFC 0075
  without pointing to RFC 0077.

impact: Operators may continue to expand RFC 0075 broadly instead of using
RFC 0077 as the bounded liveness implementation target.

recommended_action: Update the operator brief and roadmap pointers to route
the next MCP activity/deadline work through RFC 0077 while keeping RFC 0075
as the broader tmux-observable session umbrella.

follow_up: docs fix
