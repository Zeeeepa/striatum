---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Verification Report: Docs and Status Remediations
author: docs-verifier-gemini-001
date: 2026-05-22
status: completed

## Summary

This report verifies the current state of the Striatum documentation against the remediation plan defined in the RFC 0076 Audit. As of 2026-05-22, none of the documentation-specific remediations (REM-003 through REM-010) have been implemented. The documentation remains in its pre-audit state, which is consistent with the objective of this verification job (to confirm the baseline before edits).

## Remediation Verification (REM-003 to REM-010)

| ID | Status | Current State | Required Change (Target) |
|---|---|---|---|
| **REM-003** | **Unverified (Pending Edit)** | `docs/rfcs/README.md` lists RFC 0050 as "accepted (native MCP slices implemented; CLI retirement continues; numbering collision)". | Distinguish implemented native Go MCP slices from remaining CLI-retirement work. Address numbering collision. |
| **REM-004** | **Unverified (Pending Edit)** | `docs/rfcs/README.md` lists RFC 0076 as "accepted" but does not yet link to the first runnable operator workflow run. `docs/WORKFLOW_TYPES.md` describes the audit workflow but uses "Start from..." language rather than "First run completed...". | State that the first runnable RFC 0076 workflow has run and link follow-up work to the remediation plan. Update RFC status if needed. |
| **REM-005** | **Unverified (Pending Edit)** | `docs/UBIQUITOUS_LANGUAGE.md` defines "private project memory" but does not explicitly contrast it with repo-shared context and operator artifacts in the definition. `docs/CONTEXT_HYGIENE.md` is not yet updated. | Define "private project memory" and contrast it with repo-shared context and operator artifacts. |
| **REM-006** | **Unverified (Pending Edit)** | `docs/operator/BRIEF.md` mentions RFC 0077 as "proposed narrow liveness slice". `docs/ROADMAP.md` mentions it in the "Active Operator track". | Route MCP activity timestamp/deadline work through RFC 0077, with RFC 0075 as the broader umbrella. |
| **REM-007** | **Unverified (Pending Edit)** | `docs/USING_STRIATUM.md` has a "Watching agent sessions" section but it lacks the explicit tmux attach shape and the no-terminal-authority reminder. `docs/HOW_TO_HUMAN.md` has a tmux section but lacks the specific attach shape. | Add a concise "watching agent sessions" guide with tmux attach shape and the no-terminal-authority reminder. |
| **REM-008** | **Unverified (Pending Edit)** | `docs/USING_STRIATUM.md` has a "Recovery triage" list but not a table. `docs/HOW_TO_HUMAN.md` has a triage table but it may need updating for latest symptoms. | Add/update a recovery triage table mapping dashboard/status symptoms to recovery commands. |
| **REM-009** | **Unverified (Pending Edit)** | `striatum adopt` output and usage docs in `docs/USING_STRIATUM.md` and `docs/GETTING_STARTED.md` describe what `adopt` does but do not include the one-line rationale for the starter workflow. | Add either a one-line rationale for the suggested starter workflow or a direct pointer to `docs/WORKFLOW_TYPES.md`. |
| **REM-010** | **Unverified (Pending Edit)** | `docs/POSTGRES_TRANSITION.md` has a "Non-Linux and no-sudo provisioning" section but it lacks specific notes for Postgres.app and non-sudo role provisioning beyond the general script. | Add common non-Linux PostgreSQL setup notes, especially macOS/Postgres.app and non-sudo role provisioning. |

## Baseline Invariants (REM-011)

The audit confirmed the following invariants are currently held (no action needed):
- Production workflow state is daemon/PostgreSQL-owned.
- Retired RPC names are not production methods.
- Marker files, tmux panes, terminal output, and provider hooks are NOT authoritative state.

## Conclusion

The documentation is currently in a "drift" state relative to the audit findings. The baseline for remediation is confirmed. The next phase of the workflow should proceed with implementing the documentation updates as defined in the Remediation Plan.
