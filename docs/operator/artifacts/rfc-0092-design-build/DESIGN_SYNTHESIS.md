---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0092-design-build/design/codex/DESIGN.md","docs/operator/artifacts/rfc-0092-design-build/design/claude_code/DESIGN.md","docs/operator/artifacts/rfc-0092-design-build/design/gemini/DESIGN.md"]
---

# Design Synthesis
author: operator

## Objective
Implement ephemeral live agent conversation dialogue and PTY logs streaming in Striatum Web UI.

## Details
1. Implement Server-Sent Events (SSE) streaming for live dialogue on GET `/v1/runs/{runID}/live-dialogue`.
2. Implement Server-Sent Events (SSE) streaming for live raw PTY terminal logs on GET `/v1/sessions/{sessionID}/live-pty`.
