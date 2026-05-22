---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0075 Smallest Tmux Metadata Slice Design
author: tmux-planner-gemini-002

This slice defines the data structures and daemon contract changes required to make tmux-backed interactive sessions observable, as proposed in RFC 0075. It focuses on **passive metadata** and **attach visibility** before moving to active protocol liveness deadlines.

## 1. Domain Model Changes (Postgres Schema)

The `striatumd.process_supervisor_pointers` table is the authoritative link between a repository session and the daemon-owned supervisor. We expand its `metadata_json` schema and add first-class columns for observability.

### Table: `striatumd.process_supervisor_pointers`

Add/Reserve keys in `metadata_json`:

| Key | Type | Description |
|---|---|---|
| `tmux_session_name` | string | The canonical name of the tmux session (e.g. `striatum-<run>-<lane>`). |
| `tmux_pane_id` | string | The specific tmux pane ID if known. |
| `attach_command` | string | The literal command for the operator to attach (e.g. `tmux attach-session -t ...`). |
| `liveness_profile` | string | `live-interactive` (requires tmux) or `headless` (standard pipe). |
| `last_mcp_request_at` | timestamp | (Future) Last observed MCP request from this session. |

### Table: `striatumd.process_supervisors` (Optional Repo-Local Mirror)

The `process_supervisors` table in the repo-local schema (mirrored in Postgres) should eventually reflect these fields if they are useful for repo-local recovery tools. For this slice, we prioritize the `process_supervisor_pointers` and `daemon_supervisors` tables used by the daemon.

## 2. Daemon Handler Updates

### `supervise.start` (src/striatum/daemon_pg/handlers/supervision.py)

- **Input:** Accept a `liveness_profile` param (defaulting to `headless` for now, `live-interactive` triggers tmux).
- **Action:**
    - When `live-interactive` is requested, ensure the `transport` is set to `pty_helper` (or a new `tmux` transport if we split them).
    - If `transport` is `pty_helper` and a tmux session is created (already handled by the Go helper), capture the `tmux_session_name`.
    - Construct the `attach_command`.
- **Output:** Include `tmux_session_name` and `attach_command` in the response.

### `supervise.status` (src/striatum/daemon_pg/handlers/supervision.py)

- **Action:**
    - Read tmux metadata from the pointer's `metadata_json`.
    - (Future) Check if the tmux session still exists as a secondary liveness signal.
- **Output:** Include `tmux_session_name`, `attach_command`, and `liveness_profile` in the status view.

## 3. Go Supervisor / Helper Updates (go/pkg/supervisor/pty.go)

- Ensure the Go supervisor returns the `tmux_session_name` it generated back to the Python daemon through the helper event stream (`agent_started` payload).
- The current Go implementation already uses `striatum-<runID>-<laneID>` as the session name. This should be explicitly reported.

## 4. Acceptance Criteria for this Slice

1. `striatum run start` for a lane with `liveness_profile: "live-interactive"` launches in tmux.
2. `striatum supervise status --session-id <id>` returns a JSON object containing:
   ```json
   {
     "liveness_profile": "live-interactive",
     "tmux_session_name": "striatum-run_abc-lane_123",
     "attach_command": "tmux attach-session -t striatum-run_abc-lane_123"
   }
   ```
3. The `attach_command` is visible in the `operator current-brief` or dashboard (to be handled in a subsequent UI-layer task).
4. No terminal output is parsed as workflow state.

## 5. Next Slices

- **Slice B:** Protocol Liveness. Track `last_mcp_request_at` and implement the deadline sweeper.
- **Slice C:** Pre-work session tools (`session.ready`, `session.question`).
