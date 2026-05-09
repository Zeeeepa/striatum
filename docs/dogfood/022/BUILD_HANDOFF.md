---
title: "RFC 0023 V1.5 build handoff (dogfood-022)"
date: 2026-05-09
---

# Build handoff: RFC 0023 V1.5 (chat tool use + briefing) + ride-along fixes

author: implementer-codex-gpt-5.5-001

## Scope

V1.5 ships per the synthesis: six closed-set read-only chat tools, tool-call loop, system-prompt briefing, prompt-injection defense via tool-result delimiters, plus three v1.12.0 ride-along fixes.

## Findings disposition

| # | Severity | Disposition |
| --- | --- | --- |
| F1 (acceptance-blocking) | security/devils_advocate | **Implemented**: tool results wrapped in `<tool_result_begin name="..." args="..."> ... <tool_result_end>` delimiters; system briefing instructs the model to treat content between delimiters as data. |
| F2 (note) | output caps | **Implemented**: `list_dir` capped at 1000 entries; `git_log` capped at 50; `read_file` + `git_diff` capped at 64 KB. |
| F3 (note) | briefing AGENTS.md cap | **Implemented**: 8 KB cap with truncation marker. |

## Files

### New

- `src/striatum/web/chat_tools.py` — six tool definitions with both `ANTHROPIC_TOOLS` and `OPENAI_TOOLS` flavor adaptations; `execute_tool` dispatcher with closed-set enforcement; `wrap_tool_result` for delimiter wrapping; per-tool implementations (`_tool_read_file`, `_tool_list_dir`, `_tool_striatum_status`, `_tool_striatum_why`, `_tool_git_log`, `_tool_git_diff`); `_safe_resolve` path-safety helper. ~280 LoC.
- `tests/test_chat_tools.py` — 17 unit tests covering closed-set, path safety, hidden-path refusal, size caps, git invocations, striatum API invocations, delimiter wrapping.

### Modified

- `src/striatum/web/chat_provider.py`:
  - `stream_chat_response` signature extended: `tools`, `system` keyword args. Yields `{type: "text"|"tool_call"|"stop"}` events instead of `(text, is_final)` tuples.
  - `_stream_anthropic` rewritten to handle tool-use content blocks (accumulate `input_json_delta` per index; emit `tool_call` event on `content_block_stop`).
  - `_stream_openai` rewritten to handle `tool_calls` in delta (accumulate arguments; emit `tool_call` events at end of stream).
- `src/striatum/service.py`:
  - `_handle_chat_send` rewritten as a 10-iteration tool-call loop. Per iteration: read history with flavor-specific projection, split system messages, stream response, persist text + tool calls + tool results to JSONL.
  - `_handle_chat_new` seeds the transcript with a system briefing.
  - `_read_chat_history(path, *, flavor)` projects JSONL → flavor-specific message list. New helpers `_project_history_openai`, `_project_history_anthropic`, `_split_system`.
  - `_build_chat_briefing(repo)` generates the briefing (repo path, branch, recent commits, top-level entries, AGENTS.md, active runs, tool-use guidance).
  - `_safe_git(repo, argv)` runs git commands with timeout; swallows errors.
  - `_render_chat_session_page` projects new JSONL roles (`tool_use`, `tool_result`) for template rendering; skips streaming-chunk replays.
  - `_render_job_detail_page` accepts both full `job_id` and `workflow_job_id` (fixes graph-node click 404).
- `src/striatum/web/templates/chat.html` — renders `tool_use` + `tool_result` blocks as `<details>` collapsibles.
- `src/striatum/web/templates/doctor.html` — rewritten to render `doctor.problems` and `doctor.problem_records` (was rendering non-existent `doctor.checks`).
- `src/striatum/web/static/chat.js` — drops the optimistic user-message append (fixes double-render); SSE handler renders `tool_use` and `tool_result` events.
- `src/striatum/web/static/base.css` — appends styles for `.chat-tool-call` (collapsible), `.problem-list` (doctor problems).
- `tests/test_web_chat.py` — `test_chat_send_anthropic_flavor_request_shape` updated for V1.5's content-block message projection.

### Docs

- `docs/DECISION_LOG.md` — D075 row.
- `docs/TODO.md` — F22 row.
- `docs/rfcs/0023-web-chat-and-browse.md` — status `accepted (V1+V1.5)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.13.0 — 2026-05-09` section.
- `pyproject.toml` and `__init__.py` — bumped to `1.13.0`.

## Tool catalog

| Tool | Schema | Caps |
| --- | --- | --- |
| `read_file(path)` | `{path: string}` | 64 KB; binary refused |
| `list_dir(path)` | `{path: string}` | 1000 entries |
| `striatum_status(run_id?)` | `{run_id?: string}` | 64 KB JSON |
| `striatum_why(target_id)` | `{target_id: string}` | 64 KB JSON |
| `git_log(limit?)` | `{limit?: int 1-50}` | default 10, max 50 |
| `git_diff(path?)` | `{path?: string}` | 64 KB |

All read-only. Path safety identical to `/view/<path>`. `.git/` and `.striatum/` hidden by default.

## Smoke

End-to-end against OpenRouter (`anthropic/claude-opus-4.5` model):

```
POST /chat/new                          → 303 → /chat/<id>
POST /chat/<id>/send  message="What's in this repo? Use list_dir..."
                                        → 204
```

Transcript captured (in scratch JSONL):

```
SYSTEM (briefing, 6022 bytes)
USER: What's in this repo? Use list_dir to find out.
ASSISTANT: I'll explore the repository structure to give you an overview.
TOOL_USE: list_dir({'path': '.'})
TOOL_RESULT: <tool_result_begin name="list_dir" args="..."> dir .claude dir .github ...
ASSISTANT: Let me explore a few key directories...
TOOL_USE: list_dir({'path': 'src'})
TOOL_RESULT: <tool_result_begin name="list_dir" ...> dir striatum dir striatum.egg-info ...
TOOL_USE: list_dir({'path': 'docs'})
TOOL_RESULT: <tool_result_begin name="list_dir" ...> dir design dir dogfood ...
TOOL_USE: read_file({'path': 'README.md'})
TOOL_RESULT: <tool_result_begin name="read_file" ...> # striatum  Local-first orchestration ...
ASSISTANT: ## What's in this repo: **Striatum** ...
```

Multi-turn loop fired 4 tool calls before the model produced a substantive answer. Each tool result is wrapped in delimiters per F1.

## Test results

- `tests/test_chat_tools.py`: 17 / 17 pass.
- `tests/test_web_chat.py`: 16 / 16 pass (one updated for V1.5 content-block shape).
- `tests/test_web_view.py`: 8 / 8 pass.
- `make lint`: clean.
- `make typecheck`: clean (70 source files).
- Full suite: 419 / 420 pass — the one failure is the predictable doc-link self-reference (D075 cites this BUILD_HANDOFF.md before the file was on disk).

## Out of scope (V2 candidates)

- Tool that mutates with explicit per-tool gating.
- Web-search / fetch tools (would need a separate cloud-call carve-out).
- Approval-required tools (operator confirms via UI).
- Per-tool budgets across the session.
- Briefing refresh per turn.
- Chat-history infinite scroll.
- Multi-agent side-by-side comparison.

## Acceptance summary

| V1.5 acceptance gate | How it's satisfied |
| --- | --- |
| Six closed-set read-only tools | `chat_tools.py` exports `TOOL_NAMES` frozenset; `execute_tool` rejects unknown names. |
| Path safety identical to `/view/<path>` | `_safe_resolve` mirrors the `/view/<path>` handler's checks. |
| Tool-call loop with iteration cap | `_handle_chat_send` runs `for iteration in range(10)`; on cap-hit, persists a system message and returns. |
| Briefing at chat creation | `_build_chat_briefing` writes a system entry to JSONL on `POST /chat/new`. |
| Per-flavor tool wiring | Both `_stream_anthropic` and `_stream_openai` updated with tool-use streaming + accumulation. |
| Prompt-injection defense (F1) | `wrap_tool_result` applied to every tool result. Briefing instructs the model. |
| Output caps (F2) | All six tools enforce caps. |
| AGENTS.md briefing cap (F3) | 8 KB cap with truncation marker. |
| Ride-along: graph-node click 404 | `_render_job_detail_page` SQL: `WHERE run_id = ? AND (job_id = ? OR workflow_job_id = ?)`. |
| Ride-along: doctor empty list | `doctor.html` rewritten to render `doctor.problems` + `doctor.problem_records`. |
| Ride-along: chat double-render | `chat.js` no longer optimistically appends. |
| End-to-end against OpenRouter | Smoke verified: 4 tool calls + final answer. |
| Test coverage | 17 new chat-tools tests + adjusted existing chat tests. |
