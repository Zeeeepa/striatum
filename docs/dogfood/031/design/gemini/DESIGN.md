# Design: Striatum Daemon and Multi-Repository Control Plane (V1)

author: designer-gemini-pro-001
Date: 2026-05-11
Status: draft
Reference: [RFC 0028](../../../rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md)

## 1. Overview

This document defines the implementation design for `striatumd`, the local long-running control plane for Striatum. The daemon evolves Striatum from a repo-local CLI into a resident orchestration service capable of managing multiple repositories, active workflows, and supervised agents from a single point of authority.

## 2. Architecture: Multi-Repository Registry (Option C)

Following RFC 0028's recommended Option C, `striatumd` uses a hybrid storage model:

- **Central Daemon Registry**: A single SQLite database (typically at `~/.local/share/striatum/daemon.sqlite3`) storing:
    - Registered repository paths and their unique IDs.
    - Connected client identities and their capability tokens.
    - Global scheduling metadata (e.g., across-repo "stuck runs" queue).
    - Active supervisor process registration.
- **Repository Run Stores**: Each target repository retains its `.striatum/state.sqlite3`. The daemon opens these databases on-demand to perform run-specific mutations and reads.

### 2.1 Schema Extensions

**Daemon Registry (`daemon.sqlite3`):**
- `repositories`: `repo_id`, `path`, `registered_at`, `status`.
- `clients`: `client_id`, `token_hash`, `name`, `capabilities_json`.
- `active_supervisors`: `supervisor_id`, `repo_id`, `session_id`, `pid`, `started_at`.
- `audit_log`: `timestamp`, `client_id`, `repo_id`, `command`, `result`.

## 3. Transports and Protocols

`striatumd` exposes three primary transport layers:

1.  **Unix-Domain Socket (Default)**: Located at a platform-specific runtime directory (e.g., `/run/user/<uid>/striatumd.sock` on Linux or `~/Library/Application Support/striatum/daemon.sock` on macOS). Owner-only permissions (`0600`).
2.  **Loopback HTTP**: Bound to `127.0.0.1:8614`. Used by the Web UI and remote-controlled CLI calls.
3.  **Integrated MCP Server**: Exposes MCP resources and tools over the socket/HTTP layer.

### 3.1 Socket Discovery
The CLI discovers the daemon by checking:
1.  `STRIATUM_DAEMON_SOCKET` environment variable.
2.  Platform-specific default paths.
3.  `~/.striatum/daemon.info` (JSON containing current socket/PID).

### 3.2 Capability-Based Authorization
Clients must provide a `token` to access the daemon.
- **Tokens**: Generated via `striatum daemon client add --name <client_name>`.
- **Capabilities**: A JSON list of allowed CLI verbs or patterns (e.g., `["status", "why", "recovery.*"]`).
- **Enforcement**: The daemon's request dispatcher checks the client's capabilities before invoking `api.invoke`.

```json
{
  "client_id": "cli_123",
  "repo_id": "repo_abc",
  "argv": ["recovery", "auto"],
  "token": "snt_..."
}
```

## 4. Cross-Platform Lifecycle Management

`striatumd` provides native integration with platform service managers.

### 4.1 Linux (systemd)
- **Unit Type**: systemd user unit (`~/.config/systemd/user/striatumd.service`).
- **Lifecycle**: `systemctl --user {start|stop|restart|enable} striatumd`.
- **Logs**: Directed to `journald`.

### 4.2 macOS (launchd)
- **Unit Type**: LaunchAgent (`~/Library/LaunchAgents/io.striatum.striatumd.plist`).
- **Lifecycle**: `launchctl {load|unload|start|stop} ...`.
- **Logs**: Directed to `~/Library/Logs/striatumd.log`.

### 4.3 Windows
- **Unit Type**: Windows Service (using `python-service` or a generic wrapper like NSSM) or Scheduled Task.
- **Logs**: Directed to Event Viewer or a local file.

### 4.4 Logging Implementation
- **Linux**: The `striatumd` process writes to `stdout`/`stderr`, which `systemd` captures and routes to `journald`.
- **macOS/Windows**: `striatumd` uses Python's `logging.handlers.RotatingFileHandler` to manage log files in `~/Library/Logs/striatumd/` or `%LOCALAPPDATA%\striatum\logs\`.
- **Operational Logs**: Level `INFO` for requests/results; `DEBUG` for internal scheduling; `ERROR` for crashes and DB issues.

## 5. Resident Recovery and Supervision

### 5.1 Integrated Supervision
The daemon takes over the role of the supervisor manager (RFC 0009).
- When `striatum supervise start` is called via the daemon, the daemon forks the process and keeps the PID in its registry.
- **Liveness**: The daemon's main loop pings PIDs every 30 seconds.
- **Persistence**: Supervisor rows in the registry survive daemon restarts. On boot, the daemon reconciles PIDs.

### 5.2 Multi-Repo Sweep
The daemon iterates through all registered repositories and runs `striatum.recovery.auto_sweep` within each repo's database context. Escalations (webhooks, markers) are handled by the daemon's central escalation service.

## 6. Log Handling and Audit

- **No Transcript Creep**: `striatumd` logs only its internal orchestration events and client request metadata. It **never** captures agent stdout/stderr into its own logs.
- **Audit Log**: Every mutating call is recorded in the registry's `audit_log` with client ID, target repo, and command.
- **Rotation**: Standard OS log rotation (logrotate, newsyslog) is used for file-based logs.

## 7. Packaging and Distribution

- `striatumd` is bundled in the `striatum-orchestrator` Python package.
- **Entry Points**:
    - `striatum daemon {start|stop|status|install|uninstall}`: CLI for managing the daemon.
    - `striatumd`: Internal entry point for the service process.

### 7.1 Platform Artifacts (Unit Templates)
Striatum ships Jinja2 templates for platform unit files under `src/striatum/daemon/templates/`:
- `systemd_user.service.j2`: Standard systemd user unit.
- `launchd_agent.plist.j2`: macOS LaunchAgent plist.
- `windows_service.xml.j2`: XML for WinSW or similar wrapper.

These templates are rendered during `striatum daemon install` with the correct absolute path to the `striatum` binary in the current Python environment.

## 8. Operational Lifecycle

- **Start**: Reads registry, verifies repository paths, binds sockets, starts scheduler.
- **Stop**: Drains active requests, signals supervisors (advisory), closes DB connections, unlinks socket.
- **Crash Recovery**: On restart, `striatumd` reconciles its `active_supervisors` table with the OS process table and resumes the scheduler.

## 9. Compatibility

### 9.1 Client Communication
The `striatum` CLI uses a `DaemonClient` class that:
1.  Attempts to discover the daemon socket.
2.  If found, sends the request (argv + token) over the socket.
3.  If no daemon is found (and no `STRIATUM_DAEMON_REQUIRED` env var is set), falls back to direct `api.invoke` on the local repo.

### 9.2 Direct CLI Mode
Direct CLI mode remains the fallback for isolated repository work or when the operator does not want a resident service. `striatum init` continues to work without a daemon.

## 10. Acceptance Criteria Verification

- [ ] Daemon registers two repos and shows a combined `dashboard`.
- [ ] `striatum status` works via the daemon socket.
- [ ] `recovery auto` runs as a daemon-internal loop.
- [ ] Logs show client IDs and capability checks.
- [ ] `systemctl --user start striatumd` works on Linux.
