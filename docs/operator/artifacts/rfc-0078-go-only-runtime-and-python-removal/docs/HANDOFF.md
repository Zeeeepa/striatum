---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0078-go-only-runtime-and-python-removal.md", "docs/SPEC.md", "docs/DECISION_LOG.md", "docs/ROADMAP.md", "docs/TODO.md", "README.md"]
---

# Docs And Decision Cutover Handoff
author: operator [self-declared: docs-porter-codex-gpt-5-001]

## Current Rule

RFC 0078 is proposed and active for implementation, but not yet accepted as a
completed cutover. Current docs should not claim Python is gone until the
replacement gates pass.

## Required Supersession Edits On Acceptance

- Mark D018's V1 Python implementation preference superseded for active
  runtime.
- Add a new accepted decision for the Go-only runtime cutover.
- Update RFC 0068 to supersede the Python CLI/web-client carve-out.
- Update RFC 0070 to supersede the non-goal of removing the Python CLI.
- Rewrite SPEC, ROADMAP, README, getting-started, release, CLI reference,
  agent/human docs, and skill/plugin templates to Go-only install/runtime
  language.

## Historical Policy

Do not delete decision history. Historical RFCs and reviews may mention Python
as provenance, but active operator guidance should not prescribe Python after
the cutover. If the owner chooses strict no textual Python references in HEAD,
that must be an explicit historical-provenance cleanup gate because it exceeds
normal RFC supersession.

## This Workflow State

The scaffold and first implementation slice are current. The docs should say
RFC 0078 is in progress, not complete.
