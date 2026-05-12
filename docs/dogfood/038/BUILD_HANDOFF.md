author: implementer-codex-gpt-5.5-001

# RFC 0036 MCP Harness Build Handoff

Status: implemented
Date: 2026-05-12

## Summary

Implemented the accepted RFC 0036 V1 slice: the `striatum-mcp` skill now
ships through loose skill install and plugin bundles, chat exposes workflow
generation preview/write tools over the RFC 0034 generator, and workflow
writes require `--allow-mutations`, `confirm_write: true`, and a separate
operator confirmation gesture before files are written.

## Code Changes

- Added `mcp` to `CLAUDE_CODE_SKILLS` and `_PROFILE_SKILLS`, with packaged
  templates for Claude Code, Codex, Gemini, and generic guide coverage.
- Added chat tool schemas/helpers for `generate_workflow_preview` and
  `generate_workflow_write`, including mutation-aware tool list filtering
  and fallback `mutations_disabled` refusal.
- Updated chat send dispatch to pass filtered tool schemas, queue pending
  workflow writes for browser confirmation, and execute writes only through
  the same `write_generated_workflow` helper used by RFC 0034 service writes.
- Added daemon method-registry entries for `workflow.generate.preview` and
  `workflow.generate` so daemon MCP effective tool lists can expose the
  generator methods under the existing `read` and `write` capabilities.
- Updated web chat rendering for pending confirmation records and added a
  confirmation route for one-shot operator approval.
- Updated docs: README, SPEC, MCP, HOW_TO_AGENT, HOW_TO_HUMAN,
  UBIQUITOUS_LANGUAGE, CLI_REFERENCE, TODO, RFC 0034, RFC 0036, RFC index,
  and CHANGELOG.

## Tests

Added or extended tests for:

- skill install fan-out and required MCP guidance terms;
- plugin bundle regeneration across Claude Code, Codex, and Gemini;
- chat tool filtering by mutation posture;
- workflow preview/write dispatch behavior;
- web-chat operator confirmation before workflow writes;
- daemon MCP method-registry exposure through existing capability tests.

Verification run:

```text
make install
make lint
make typecheck
make test
make smoke
```

Final results:

- `make install`: up to date after earlier editable install.
- `make lint`: passed.
- `make typecheck`: passed.
- `make test`: 630 passed in 404.43s.
- `make smoke`: passed; emitted existing deprecated `needs` fixture warnings.

## Delegation

Used two native explorer sub-agents:

- skill/plugin installer inspection and test-pattern summary;
- chat/service workflow-generation integration-point summary.

The parent session owned all file edits, integration, verification, and this
handoff.

## Deferred Items

No new product decisions were required. Deferred items remain those named by
the accepted synthesis: no new examples workflow, no `daemon describe
--workflow` explainer, no per-chat capability-token issuance, and no web
workflow chooser UI.
