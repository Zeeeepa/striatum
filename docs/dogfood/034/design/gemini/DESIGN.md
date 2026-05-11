# Implementation Design: RFC 0030 + RFC 0031 (Gemini)

author: designer-gemini-pro-001
date: 2026-05-11

This document specifies the paired implementation of RFC 0030 (Daemon RPC Server) and RFC 0031 (Daemon-Owned Supervision and Sealed Apply).

## 1. System Architecture

The Striatum Daemon (`striatumd`) transitions from a lifecycle marker to a full orchestration service.

### 1.1 Process Model
- The daemon runs as a long-lived user-level process.
- It becomes the parent process for all supervised agent lanes.
- It owns the signing key and mediates all repository mutations via RPC.

### 1.2 Cross-Platform Supervision (Service Managers)

The daemon will support native service managers to ensure persistence across reboots and easy management.

| Platform | Service Manager | Configuration Path | Management Command |
| :--- | :--- | :--- | :--- |
| **Linux** | `systemd` (user) | `~/.config/systemd/user/striatumd.service` | `systemctl --user` |
| **macOS** | `launchd` | `~/Library/LaunchAgents/io.striatum.striatumd.plist` | `launchctl` |
| **Windows** | Windows Service | Native Service (requires `pywin32`) | `sc.exe` |

#### Linux `systemd` Unit Template
```ini
[Unit]
Description=Striatum Daemon
After=network.target

[Service]
ExecStart=%h/.local/bin/striatum daemon start --foreground
Restart=on-failure
Environment=STRIATUM_DAEMON_RUNTIME_DIR=%t/striatum

[Install]
WantedBy=default.target
```

#### macOS `launchd` Plist Template
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.striatum.striatumd</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/striatum</string>
        <string>daemon</string>
        <string>start</string>
        <string>--foreground</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

#### Windows Service
On Windows, `striatumd` will utilize `win32serviceutil` (via `pywin32`) to register as a native service.
- **Service Name**: `StriatumDaemon`
- **Display Name**: `Striatum Daemon`
- **Startup Type**: Automatic (Delayed Start)
- **Recovery**: Restart the service on first and second failures.

If `pywin32` is unavailable at install time, `striatum daemon init` will fallback to creating a Scheduled Task:
```powershell
Register-ScheduledTask -Action (New-ScheduledTaskAction -Execute "striatum" -Argument "daemon start --foreground") -Trigger (New-ScheduledTaskTrigger -AtLogOn) -TaskName "StriatumDaemon" -Description "Striatum Daemon Task"
```

### 1.3 Transports

| Platform | Primary Transport (Local) | Path |
| :--- | :--- | :--- |
| **Linux/macOS** | Unix Domain Socket (UDS) | `${XDG_RUNTIME_DIR}/striatum/daemon.sock` |
| **Windows** | Named Pipe | `\\.\pipe\striatumd` |

- **Permissions**: `0600` for UDS; Owner-only ACL for Named Pipes.
- **Loopback HTTP**: Optional, enabled via `--http 127.0.0.1:<port>`. Mandatory for the Web UI. Authorization via `Authorization: Bearer <token>`.

## 2. Protocol and Security

### 2.1 Version Skew Protocol (RFC 0030 §3)
The daemon and client perform a handshake to negotiate features and ensure compatibility.

1. **`daemon.hello`**: Client sends its name, version, supported envelope versions (`[1]`), and supported framings (`["json"]`).
2. **`daemon.welcome`**: Daemon responds with its version, selected envelope version, framing, and `methods_etag`.
3. **Refusal Rules**:
   - If `envelope` versions do not overlap: Client receives `version_incompatible` and must exit (code 10).
   - If the daemon is newer (Major/Minor): The client may continue but must respect that some methods it expects might not be present in the `daemon.describe` output.
   - If the client is newer (Major): The daemon refuses the connection.

### 2.2 Signing-Key Custody
The daemon uses an Ed25519 keypair for signing apply receipts.

- **Primary**: OS Keyring (via Python `keyring` library).
  - Service: `striatum`
  - Account: `daemon-signing-key`
- **Fallback**: `0600` file at `${XDG_STATE_HOME}/striatum/daemon/signing_key.pem`.
- **Degraded Trust**: The daemon emits a warning if the fallback is used.

### 2.3 Capability Model
The daemon enforces route-bound capabilities:
- `read`: `status`, `doctor`, `why`, `dashboard`, `supervise.status`
- `write`: `session.register`, `supervise.start`, `supervise.send`
- `apply`: `apply.reviewed_patch` (requires explicit grant)
- `admin`: `repo.add`, `daemon.shutdown`, `token.create`

## 3. Daemon-Owned Supervision (RFC 0031)

### 3.1 Supervision Lifecycle
1. **`supervise.start`**: Daemon calls `subprocess.Popen` with `start_new_session=True`.
2. **Identity Capture**: Daemon records `pid` and `pid_start_time`.
   - **Linux**: `/proc/<pid>/stat` (field 22).
   - **Cross-platform**: `psutil.Process(pid).create_time()`.
3. **Reattach Logic**: On daemon restart, it iterates over `attached` supervisors in the DB.
   - For each, it compares the current `create_time` of the `pid` with the recorded `pid_start_time`.
   - If it matches, the supervisor remains `attached`.
   - If it mismatches or the process is gone, it transitions to `lost`.

### 3.2 Sealed Apply Boundary
- **Worktree Isolation**: The daemon maintains a private worktree for `apply.reviewed_patch`.
- **Receipt Verification**: Before applying, the daemon verifies the hash chain: `patch digest` -> `reviewer verdict` -> `base-tree hash`.
- **Atomic Receipt**: The receipt is recorded in the daemon's Postgres substrate (RFC 0033) before the worktree is modified to prevent "ghost applies" on crash.

## 4. Packaging and Onboarding

### 4.1 Packaging Deltas
- **New Dependencies**: `keyring`, `psutil`, `pywin32` (optional, Windows only).
- **Entry Points**: `striatumd` is added as an alias for `striatum daemon start --foreground`.

### 4.2 Operator Onboarding
1. `striatum daemon init`:
   - Checks/Generates signing key.
   - Detects OS and suggests service manager commands (e.g., `systemctl --user enable ...`).
   - Creates the initial `admin` token and stores it in the operator's keyring.
2. `striatum daemon start`: Launches the server.
3. `striatum repo add <path>`: CLI detects the daemon and routes the registration.

## 5. Adversarial Test Cases

### 5.1 Hostile MCP Client
- **Token Theft**: An agent steals a `write`-only token and attempts `apply.reviewed_patch`.
  - *Assertion*: Daemon refuses with `capability_missing` and logs a `denied` audit row.
- **Path Traversal**: An agent attempts `supervise.start` with a `cwd` outside the repository.
  - *Assertion*: Daemon validates `cwd` against the registered repository root and refuses.

### 5.2 Supervisor Reattach
- **Scenario**: Daemon is SIGKILL'd while a lane is active. Daemon restarts.
  - *Assertion*: Daemon identifies the surviving lane process, verifies `pid_start_time`, and resumes supervision without interrupting the lane.
- **PID Recycling**: Daemon crashes. Another process takes its lane's PID.
  - *Assertion*: `pid_start_time` check fails; daemon marks the supervisor as `lost` and refuses to send packets to the recycled PID.

### 5.3 Hostile MCP Resource Request
- **Scenario**: MCP client requests `striatum://repos` without a token.
  - *Assertion*: Daemon refuses or returns an empty list depending on whether `read` is public (it shouldn't be, per RFC 0030 §4).

## 6. Implementation Phases

1. **Phase 1: RPC Plumbing**: Implement the envelope, handshake, and UDS/Named Pipe transports.
2. **Phase 2: Supervision Migration**: Move `process_supervisors` to the daemon DB and implement `supervise.*` RPCs.
3. **Phase 3: Sealed Apply**: Implement key custody, `apply.reviewed_patch`, and receipt generation.
4. **Phase 4: Service Management**: Implement `daemon init` and cross-platform service templates.
