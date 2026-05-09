# Research: chat tool-use + briefing shape

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Existing surfaces

### `chat_provider.py` (RFC 0023 V1)

- `ChatProviderConfig` — env-var-driven config (base_url, api_key, model, flavor).
- `stream_chat_response(config, messages, *, max_tokens, timeout) -> Iterator[(text, is_final)]` — yields text chunks. **No tool-use support today.**
- Two flavor functions: `_stream_anthropic` (POST /v1/messages with `messages: [{role, content}]`); `_stream_openai` (POST /v1/chat/completions same shape).
- `_parse_sse_events(response) -> Iterator[{event, data}]` — generic SSE parser.

### Tool-use API shapes

#### Anthropic Messages tool use

Request adds:
```json
{
  "model": "...",
  "messages": [...],
  "tools": [
    {
      "name": "read_file",
      "description": "Read the contents of a file in the repo.",
      "input_schema": {
        "type": "object",
        "properties": {"path": {"type": "string"}},
        "required": ["path"]
      }
    }
  ],
  "stream": true,
  "max_tokens": 4096
}
```

Response stream yields events including:
- `content_block_start` with `content_block.type: "tool_use"` + `id`, `name`, `input` (incremental via `input_json_delta`).
- `content_block_delta` with `delta.type: "input_json_delta"` and partial `partial_json`.
- `content_block_stop`, then `message_delta` with `stop_reason: "tool_use"`.

Tool result fed back as a fresh request:
```json
{
  "messages": [
    ...,
    {"role": "assistant", "content": [{"type": "tool_use", "id": "<id>", "name": "...", "input": {...}}]},
    {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "<id>", "content": "<result text>"}]}
  ]
}
```

#### OpenAI Chat tool use

Request adds:
```json
{
  "model": "...",
  "messages": [...],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "read_file",
        "description": "Read the contents of a file in the repo.",
        "parameters": {
          "type": "object",
          "properties": {"path": {"type": "string"}},
          "required": ["path"]
        }
      }
    }
  ],
  "stream": true
}
```

Response stream's `delta` includes `tool_calls: [{index, id, type: "function", function: {name, arguments}}]`. Arguments arrive incrementally as JSON string fragments.

Tool result fed back:
```json
{
  "messages": [
    ...,
    {"role": "assistant", "tool_calls": [{"id": "<id>", "type": "function", "function": {"name": "...", "arguments": "<json string>"}}]},
    {"role": "tool", "tool_call_id": "<id>", "content": "<result text>"}
  ]
}
```

OpenRouter, vLLM, Ollama, etc. follow this shape.

### Briefing inputs

- Repo path: `self.state.repo`.
- Current branch: `git rev-parse --abbrev-ref HEAD`.
- Recent commits: `git log -10 --oneline`.
- Top-level dir: `os.listdir(repo)` filtered (skip `.git`, `.striatum`).
- AGENTS.md content: `(repo / "AGENTS.md").read_text()` if exists.
- Active runs: SQLite `SELECT run_id, state FROM runs WHERE state IN ('running', 'ready') ORDER BY created_at DESC`.

### Tool execution surface

- `read_file`: same path-safety as `_render_view_path`.
- `list_dir`: directory listing with `.git`/`.striatum` filtered.
- `striatum_status`: `striatum.api.invoke(["status", "--run-id", id], repo=self.state.repo)`.
- `striatum_why`: same with `why <id>`.
- `git_log`: `subprocess.run(["git", "log", "-10", "--oneline"], cwd=repo, ...)`.
- `git_diff`: `subprocess.run(["git", "diff"], cwd=repo, ...)`.

All read-only. Tool execution lives in a new module `striatum.web.chat_tools`.

## Ride-along fixes (separate from V1.5 scope but bundled in v1.13.0)

### F1: Graph-node route tolerance (already in working tree)

`_render_job_detail_page` now accepts either full `job_id` or `workflow_job_id` via `OR` clause. SVG graph node clicks were 404'ing because the renderer emits `workflow_job_id` in the href.

### F2: Doctor template field mismatch

`doctor.html` references `doctor.checks` but `doctor()` returns `doctor.problems` (list[str]) + `doctor.problem_records` (list[dict]). Template needs to render against the actual shape.

### F3: Chat double-render

`chat.js` optimistically appends user messages on form submit, then the SSE stream replays the same message from JSONL — user sees the message twice (once timestamped, once not). Fix: drop the optimistic append; let the SSE round-trip render it.

## Test precedent

- `tests/test_web_chat.py` — fake HTTP server fixture; assert request shapes for both flavors. Extend with tool-use round-trip tests.
- New: `tests/test_chat_tools.py` for `execute_tool` directly (path safety, tool dispatch, error surfaces).

## Summary table

| V1.5 surface | File | Action |
| --- | --- | --- |
| Tool definitions | `src/striatum/web/chat_tools.py` (new) | TOOL_SCHEMAS dict + execute_tool |
| Tool-use streaming | `src/striatum/web/chat_provider.py` | Add tool support to both `_stream_*` functions; new return shape |
| Tool-call loop | `src/striatum/service.py` `_handle_chat_send` | While tool calls, execute + re-request, cap 10 iterations |
| Briefing | `src/striatum/service.py` `_handle_chat_new` | Generate + persist as system message |
| JSONL extension | `src/striatum/service.py` | New role values: `tool_use`, `tool_result` |
| UI rendering | `src/striatum/web/templates/chat.html` + `static/chat.js` | Render tool blocks; drop optimistic append |
| Tests | `tests/test_chat_tools.py` (new), extend `tests/test_web_chat.py` | Both flavors + execute_tool |
| Ride-along F1 | (route fix already applied) | bundle into v1.13.0 |
| Ride-along F2 | `src/striatum/web/templates/doctor.html` | Render `doctor.problems` instead of `doctor.checks` |
| Ride-along F3 | `src/striatum/web/static/chat.js` | Remove optimistic appendMessage on submit |
