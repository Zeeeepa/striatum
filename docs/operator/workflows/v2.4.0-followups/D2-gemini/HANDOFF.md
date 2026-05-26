# D2 Gemini reliability followup

I have updated the Gemini agent guidance to improve agent-loop reliability.

## Changes

- **Updated `go/pkg/installers/templates/skills/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`**:
    - Added "Agent-loop reliability (Gemini)" section with four core rules.
- **Updated `go/pkg/installers/templates/plugins/gemini/GEMINI.md.tmpl`**:
    - Added "Agent-loop reliability (Gemini)" section with consistent rules.

## Reliability Rules Implemented

1.  **Request a long lease (900s)** on `work.await_packet`.
2.  **No repository exploration** while holding a packet.
3.  **Explicit `repository_id` and `session_id`** on every MCP call.
4.  **Prompt loop completion** and lease error handling.

These changes ensure future Gemini lanes are more dependable when driving Striatum workflows.

author: implementer-gemini-1
date: 2026-05-26
