# Design synthesis: RFC 0023 V1.5 (chat tool use + briefing)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1.5 ships the headline RFC 0023 V1.5 deferral (tool use) plus the system-prompt briefing that pairs naturally with it. Three ride-along fixes that surfaced during operator dogfooding of v1.12.0 are bundled into the same release.

## 1. Closed tool set

V1.5 ships **six** tools — all read-only:

| Name | Purpose | Schema (input) | Output |
| --- | --- | --- | --- |
| `read_file` | Read a repo-relative file. Same path-safety as `/view/<path>`. | `{path: string}` | File contents, or error string. Capped at 64 KB; longer files are truncated with a marker. |
| `list_dir` | List directory entries. `.git` and `.striatum` hidden by default. | `{path: string}` | One entry per line: `<type> <name>` where `<type>` is `dir` or `file`. |
| `striatum_status` | Run state. | `{run_id?: string}` | JSON-rendered envelope from `striatum.api.invoke(["status", "--run-id", id])`. When `run_id` omitted, all-runs status. |
| `striatum_why` | Investigate a job/artifact/blocker. | `{target_id: string}` | JSON envelope from `striatum.api.invoke(["why", "--id", target_id])`. |
| `git_log` | Recent commits with messages. | `{limit?: int}` | Newline-joined `<short-sha> <subject>` lines. Default limit 10; cap 50. |
| `git_diff` | Working-tree diff. | `{path?: string}` | Unified diff. Capped at 64 KB. |

No tool that mutates state (`run prepare`, `complete`, `submit-review`, etc.). The closed-set membership check is enforced in `execute_tool`; unknown tool names return an error string rather than executing.

## 2. Tool execution module

New `src/striatum/web/chat_tools.py`:

```python
TOOL_SCHEMAS: list[dict] = [...]   # JSON schema per tool, flavor-neutral
ANTHROPIC_TOOLS = [...]            # adapted for Anthropic shape
OPENAI_TOOLS = [...]               # adapted for OpenAI shape

def execute_tool(name: str, args: dict, *, repo: Path) -> str:
    """Closed-set dispatch. Returns the tool result as a string.
    On error: returns a short error string the model can reason about.
    Never raises; all exception paths produce an error-as-result."""
```

Path safety identical to `_render_view_path`: `..`, leading `/`, null bytes, symlinks escaping the repo all return error strings.

## 3. Tool-call loop in `_handle_chat_send`

Pseudocode:

```
history = _read_chat_history(path)
for iteration in range(MAX_TOOL_ITERATIONS):  # cap = 10
    response = stream_chat_with_tools(config, history, tools)
    text_chunks, tool_calls = collect(response)
    append assistant text/tool_use entries to JSONL + SSE
    if not tool_calls:
        break
    for call in tool_calls:
        result = execute_tool(call.name, call.args, repo=self.state.repo)
        append tool_result entry to JSONL + SSE
        history.append({role: "tool" or "user-with-tool_result", content: ...})
```

`MAX_TOOL_ITERATIONS = 10` prevents runaway. On hit, append a system message: "tool-call loop hit iteration cap; conversation halted."

## 4. Per-flavor request/response handling

### Anthropic Messages

- Request: add `tools: ANTHROPIC_TOOLS` to the body.
- Response: parse `content_block_start` with `type: "tool_use"` to get `id` + `name`; accumulate `input_json_delta` partials per block; at `content_block_stop`, parse the accumulated JSON.
- Stop reason: `stop_reason: "tool_use"` triggers the loop continuation.
- Re-request shape: append assistant turn with `content: [{type: "tool_use", id, name, input}]` then user turn with `content: [{type: "tool_result", tool_use_id, content}]`.

### OpenAI Chat

- Request: add `tools: OPENAI_TOOLS`.
- Response: parse `delta.tool_calls[].function.{name, arguments}`. Arguments arrive as JSON-string fragments; concatenate.
- Stop reason: `finish_reason: "tool_calls"` triggers loop continuation.
- Re-request shape: append assistant turn with `tool_calls: [...]` then `role: "tool"` turn with `tool_call_id` + `content`.

Both flavor adaptations are flavor-specific functions; the orchestrator (`stream_chat_with_tools`) is shared.

## 5. Briefing at chat creation

`_handle_chat_new` writes one `system`-role JSONL entry containing:

```
You are a chat assistant running inside striatum, a local-first
orchestration tool. You have read-only tool access to the repo at
<repo-path>; ask for files via the `read_file` tool when you need
their contents. The operator's repo is currently:

  Repo: <repo-path>
  Branch: <branch>
  Recent commits:
    <up to 10 lines: short-sha + subject>

  Top-level entries:
    <listing, .git/.striatum hidden>

  Active runs:
    <run_id (state)>  ×0..N

  AGENTS.md (if present):
    <verbatim content, capped at 8 KB>

When asked about the project: read AGENTS.md (above), or call
read_file('docs/SPEC.md') / read_file('docs/PRD.md') for more
context. Use list_dir(...) to explore. Use git_log/git_diff to
understand recent work. Use striatum_status() to see active runs.
```

Generated server-side once at session creation. Persists in the JSONL transcript, so reload sees it. The system message renders as a regular `system`-styled bubble in the chat history (via the existing `chat-role-system` CSS).

## 6. JSONL extensions

New `role` values in `transcript.jsonl`:

- `tool_use` — model requested a tool. Content shape: `{tool_use_id, tool_name, tool_input}` (JSON-stringified).
- `tool_result` — server executed the tool. Content shape: `{tool_use_id, tool_name, result}` (JSON-stringified).

The existing `_read_chat_history` helper extends to project these into the per-flavor message-history shape sent on the next request.

## 7. UI rendering of tool blocks

`chat.html` + `chat.js` render `tool_use` and `tool_result` entries as collapsed-by-default `<details>` blocks:

```html
<details class="chat-tool-call">
  <summary>🔧 read_file({"path": "docs/SPEC.md"})</summary>
  <pre class="tool-result">...</pre>
</details>
```

CSS: `.chat-tool-call` muted background; `summary` shows tool name + truncated args; `pre.tool-result` shows the result body.

## 8. Ride-along fixes (bundled in v1.13.0)

### F1: Graph-node route tolerance

Already in working tree: `_render_job_detail_page` accepts both full `job_id` and `workflow_job_id`. Tests added.

### F2: Doctor template field mismatch

`doctor.html` rewritten to render `doctor.problems` (list[str]) and `doctor.problem_records` (list[dict]) — NOT the non-existent `doctor.checks`.

### F3: Chat double-render

`chat.js`: drop the optimistic `appendMessage("user", ...)` on form submit. The textarea clears immediately and the SSE stream renders the message after the JSONL append. UX trade-off: ~250ms of latency before the user's own message appears, but no duplication.

## 9. Test plan

`tests/test_chat_tools.py` (new):
1. `test_execute_tool_closed_set_refused` — unknown name → error string.
2. `test_execute_tool_read_file_path_safety` — `..`, absolute, symlink-escape → error.
3. `test_execute_tool_read_file_dotgit_hidden` → error.
4. `test_execute_tool_list_dir_filters_hidden` — `.git/.striatum` not in output.
5. `test_execute_tool_read_file_size_cap` — large file truncated.
6. `test_execute_tool_git_log_default_limit` — 10 entries.
7. `test_execute_tool_git_diff` — works on a dirty tree.
8. `test_execute_tool_striatum_status_invocation` — invokes the API.
9. `test_execute_tool_striatum_why_invocation` — invokes the API.

`tests/test_web_chat.py` extension:
10. `test_chat_send_anthropic_tool_use_round_trip` — fake server returns `stop_reason: tool_use`, server executes the tool, fake server returns assistant text on the second request.
11. `test_chat_send_openai_tool_use_round_trip` — same for OpenAI flavor.
12. `test_chat_briefing_appears_in_transcript` — POST /chat/new produces a JSONL with the system briefing as the first entry.

`tests/test_web_doctor.py` (new): `test_doctor_page_renders_problems` — POST creates a session that produces problems → doctor page renders the list.

`tests/test_web_chat_no_double_render.py` (new): `test_chat_history_has_one_user_entry_per_send` — after a send, the JSONL has exactly one user entry (no duplication).

## 10. Documentation surface

- `docs/SPEC.md` § "Local Web UI" — add tool-use subsection, briefing, JSONL role extensions.
- `docs/UBIQUITOUS_LANGUAGE.md` — `chat tool`, `tool result`, `chat briefing`.
- `docs/DECISION_LOG.md` — D075 row.
- `docs/TODO.md` — F22.
- `docs/rfcs/0023-web-chat-and-browse.md` — status `accepted (V1+V1.5)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.13.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bump to `1.13.0`.

## 11. Out of scope (V2 candidates)

- Tool that mutates (e.g., `striatum_verdict`) — V2 with explicit per-tool gating.
- Web-search tool, fetch tool — V2 with separate cloud-call carve-out.
- Approval-required tools (model proposes; operator confirms via UI) — V1.6 if the closed read-only set proves limiting.
- Tool-call streaming UI (show partial input as it arrives) — V1.6.
- Briefing refresh on each send (V1 generates once at chat creation) — V1.6 if state drifts mid-conversation.
- Per-tool budgets (e.g., max bytes returned per tool call across the session) — V1.6.

## 12. Zero-regression contract

- Without env vars: chat-index empty-state unchanged from v1.12.0.
- Without tool calls in the response: V1.5 behaves byte-identically to V1.0 (the loop runs once, appends assistant text, returns).
- JSON API + SSE for non-chat surfaces unchanged.
- CSP byte-identical.
- Existing `tests/test_web_chat.py` + `tests/test_web_view.py` continue to pass.

## 13. Boundary impact

The chat tools surface introduces a *narrow* new threat boundary: the model can request file reads / git diffs that the operator may not have explicitly authorized. Mitigations:

1. Closed tool set — no general-purpose `exec` or `eval`.
2. Path safety identical to `/view/<path>`.
3. Read-only — no tool mutates state.
4. Same `--allow-mutations` gate to even start a chat.
5. Tool calls + results are recorded in the transcript JSONL — operator can audit after the fact.

D028 (transcripts off) carve-out from D074 still holds: tool calls / results live in scratch JSONL, not SQLite, not as artifacts.
