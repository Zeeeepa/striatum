# Project Plan: Striatum Architecture Review

## Architecture of Review
This is an expert systems architecture review of the Striatum codebase at `~/git/striatum`. The review will analyze the Go-only standalone architecture, boundaries, daemon/MCP/CLI separation, and PostgreSQL transition state, producing a 3,000–5,000 word markdown report.

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | **Codebase Inventory & Deep Audit** | Spawn an `explorer` subagent to analyze the repo, list all files, inspect daemon implementation (`go/pkg/daemon`), CLI, DB transition docs/code, and build/test posture. Produce an inventory and analysis report. | None | DONE |
| 2 | **Draft Architecture Report** | Spawn a `worker` subagent to write a 3,000–5,000 word draft of sections 0-10 in `STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md` using the findings from the Explorer and tri-voice grounding. | M1 | DONE |
| 3 | **Review & Refinement** | Spawn a `reviewer` subagent to challenge findings, verify line range accuracy, and perform style and word-count checking on the draft. | M2 | DONE |
| 4 | **Forensic Audit & Verification** | Spawn a `teamwork_preview_auditor` to run integrity forensics on the generated file to ensure no cheating (e.g. dummy/hardcoded logic, plagiarism, or AI hallucination) occurred. Verify layout and format. | M3 | DONE |
| 5 | **Sentinel Handover** | Report final completion to the Sentinel. | M4 | DONE |

## Core Deliverable Specification
- **File Path**: `~/git/striatum/STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md`
- **Sections**: 0 to 10 in exact order.
- **Word Count**: 3,000 to 5,000 words.
- **Tone**: Highly dense, technical, expert-maintainer targeted, fluff-free.
- **Style**: Tri-voice grounding (`stated`, `actual`, `mine`). Argue/disagree where necessary. Absolutely no generic cloud-ops/SaaS-ops advice.

## Quality Checks
- Check that the output exists at `~/git/striatum/STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md`.
- Verify word count of the file is between 3,000 and 5,000 words.
- Verify every non-trivial claim has line-grounded links.
- Confirm all 11 sections exist in order.
