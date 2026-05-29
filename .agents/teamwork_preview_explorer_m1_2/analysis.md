# Codebase Inventory and Security Audit Report

## Executive Summary
This report presents a deep architectural inventory and security audit of the **Striatum** codebase at `~/git/striatum`. The investigation confirms that Striatum is a local-first workflow runner with a robust, audited Go daemon (`striatumd`) serving as the single source of truth backed by a native PostgreSQL database. Security boundaries between the CLI, the daemon, Model Context Protocol (MCP) clients, and subprocesses are rigidly defined, ensuring authenticated access, dynamic capability scope validation, loopback restriction, and strong process isolation with start-time tokens.

---

## 1. Audited Files Catalog

The following core files and packages were reviewed in detail during the audit:

| File Path | Description / Role |
| :--- | :--- |
| `docs/reference/command-authority-matrix.md` | Authoritative matrix mapping daemon RPC methods, capabilities, and scopes. |
| `go/cmd/striatum/main.go` | Entry point for the Striatum Go CLI. Handles local vs daemon-routed commands. |
| `go/cmd/striatumd/main.go` | Entry point for the Go daemon server (`striatumd`). Bootstraps server, DB, recovery scheduler. |
| `go/cmd/striatumd/handler_coverage_test.go` | Coverage test enforcing that 100% of contract RPC methods have active Go handlers. |
| `go/cmd/striatum-supervisor-helper/main.go` | Entry point for the supervisor PTY helper process. |
| `go/pkg/rpc/server.go` | RPC server handling UNIX sockets, handshake logic, request deduplication, and audits. |
| `go/pkg/rpc/registry.go` & `registry_methods.go` | Definition of registered RPC methods, required capabilities, and repository scopes. |
| `go/pkg/rpc/envelope.go` | Structure of JSON-framed envelopes for requests/responses (newline-delimited). |
| `go/pkg/rpc/capability.go` | Memory-backed authorizer interface for testing capability scopes. |
| `go/pkg/rpc/auth_pg.go` | Production Postgres-backed capability token and scope authorizer. |
| `go/pkg/cli/dispatch/dispatch.go` | Main CLI runner that maps arguments, resolves repositories, and invokes RPCs. |
| `go/pkg/cli/localcommands/localcommands.go` | Registry of out-of-band CLI commands that bypass the daemon. |
| `go/pkg/cli/routes/routes.go` & `routes_generated.go` | Metadata mapping CLI inputs to daemon RPC methods. |
| `go/pkg/cli/rpcclient/client.go` | CLI Unix socket dialer, handshake executor, and RPC requester. |
| `go/pkg/mcp/http.go` | Model Context Protocol server exposing JSON-RPC 2.0 and SSE streams. |
| `go/pkg/mcp/capabilities.go` | Dynamic tool discovery based on client capability token validation. |
| `go/pkg/mcp/tools.go` | Dynamic tool execution and fail-closed security wrappers. |
| `go/pkg/supervisor/helper.go` & `helper_protocol.go` | Decoupled process executor communicating via structured control events. |
| `go/pkg/supervisor/pointer.go` | Metadata pointer storage and atomic pidfile generation. |
| `go/pkg/supervisor/process_identity_linux.go` | Linux procfs PID start-time token retriever. |
| `go/pkg/supervisor/tmux_liveness.go` | Tmux session/pane liveness and start token verification. |
| `go/pkg/mutations/supervision.go` | Supervise control event recorder and supervisor status updates. |
| `go/pkg/mutations/supervision_control.go` | Supervise process launcher, packet pipeline writer, and PTY manager. |
| `go/pkg/reads/supervision.go` | Read-only supervision status and liveness projection engine. |

---

## 2. CLI Subsystem and Repository Resolution

The CLI (`go/cmd/striatum/main.go` and `go/pkg/cli`) acts as a wrapper around the daemon RPC server for all mutations, and handles file-level authoring locally.

### Local vs. Daemon-Routed Commands
1. **Local Commands**: Evaluated out-of-band in `go/pkg/cli/localcommands/localcommands.go` (lines 9-20). These include:
   - `workflow validate`, `workflow generate`, `workflow templates` (local file-level authoring, requiring no live state).
   - `skills install`, `plugin install`, `plugin uninstall` (filesystem template rendering).
   - `daemon install`, `daemon uninstall`, `daemon status`, `daemon migrate-db` (bootstrap helpers).
2. **Daemon-Routed Commands**: If a command is not classified as local, `go/cmd/striatum/main.go` calls `runDaemonRoute` (lines 78-91), delegating to `dispatch.Run` (in `go/pkg/cli/dispatch/dispatch.go`).

### Routes and CLI Lookup
- `dispatch.Run` (lines 56-64) uses `routes.Lookup(globals.CommandArgs)` from `go/pkg/cli/routes/routes_generated.go` to match commands to their corresponding daemon RPC method, parameter groups, required capability, and repository scope.
- **Example**: `striatum claim-next` maps to `work.claim_next` with `claim` capability and `single_repo` scope.

### Unix Socket Communication & Handshake
- `rpcclient.Client` (in `go/pkg/cli/rpcclient/client.go` lines 63-104) reads the token from the filesystem (`$STRIATUM_DAEMON_TOKEN_FILE` or `~/.striatum/retired-local-state` legacy path cutovers) and dials the Unix socket at `Config.SocketPath` (typically `/run/user/<uid>/striatum/daemon-go.sock`).
- Every connection must execute a **synchronous handshake** (`daemon.hello` RPC) before executing any other commands:
  ```go
  // go/pkg/cli/rpcclient/client.go lines 79-88
  if _, err := send(ctx, conn, reader, rpc.Envelope{
      SchemaVersion: rpc.SupportedEnvelopeVersion,
      RequestID:     requestID("hello"),
      Method:        "daemon.hello",
      Params: map[string]any{"client": map[string]any{
          "name":               "striatum-go-cli",
          "supported_envelope": []int{rpc.SupportedEnvelopeVersion},
          "supported_framings": []string{rpc.DefaultFraming},
      }},
      DeadlineMS: c.Config.DeadlineMS,
  }); err != nil {
      return nil, err
  }
  ```

### Repository Resolution
- For commands with `single_repo` scope, if a `--repository-id` flag is omitted, the CLI resolves the repository ID using the `repo.resolve` RPC (in `go/pkg/cli/dispatch/dispatch.go` lines 246-267). It sends the local current working directory (`cwd`) or `--repo` path to the daemon, which performs a query and returns the registered `repository_id`.
- This eliminates the need for direct PostgreSQL imports from the CLI, strictly maintaining a service boundary.

---

## 3. Daemon Subsystem and Bootstrap

The Go daemon (`striatumd` under `go/cmd/striatumd/main.go`) operates the central PostgreSQL-backed state engine.

### DB Initialization and Migrations
- The daemon **refuses to bind its socket** if no Postgres URL is configured (lines 173-175).
- Upon startup, it executes database connection and triggers schema migrations:
  ```go
  // go/cmd/striatumd/main.go lines 179-185
  pool, version, err = db.ConnectAndMigrate(ctx, config.URL, daemonVersion)
  ```
  This applies all 17 embedded baseline SQL files sequentially (located in `go/pkg/db/sql/`).

### Global Token Bootstrapping
- `admin.BootstrapRuntimeTokenIfNeeded` (lines 194-204) verifies if a resident client exists or generates one. It outputs a cleartext admin-scoped bearer token (formatted as `tokenID.secret`) into `$STRIATUM_DAEMON_RUNTIME_DIR/daemon.token` (read-only 0600 by the owner).
- This bootstrapped runtime token acts as the single bearer for the resident scheduler, local MCP interfaces, and mounted HTTP service interfaces.

### Resident Recovery Scheduler
- `startRecoveryScheduler` (lines 476-504) initializes a background loop running `recovery.sweep` at configurable intervals (`sweep-interval-seconds`, defaulting to 60s) to finalise stalled jobs, requeue stale leases, and reconcile active supervisor rows.

---

## 4. RPC Subsystem & Command-Authority Matrix

All daemon actions are registered within a strict command-authority schema mapping capabilities, repository scopes, and parameters.

### RPC Envelope & Parsing
- All socket frames represent newline-delimited JSON objects parsed by `rpc.DecodeEnvelope` (in `go/pkg/rpc/envelope.go` lines 47-58).
- The maximum allowed envelope size is capped at **8 MiB** (`MaxEnvelopeBytes = 8 * 1024 * 1024` in `go/pkg/rpc/server.go` line 153) to support base64-encoded historical dogfood/archive blobs.

### Capability Token Verification (Postgres)
- Token verification in `PostgresAuthorizer.Authorize` (in `go/pkg/rpc/auth_pg.go` lines 43-136) performs secure, constant-time database-backed audits:
  1. Parses incoming token `tokenID.secret`.
  2. Queries `striatumd.clients` (lines 61-66) to retrieve `client_id`, `token_hash`, and `token_salt`.
  3. Recompute hash: `hmacHexSecret(tokenSalt, secret)` and check equality with `subtle.ConstantTimeCompare` (lines 73-76).
  4. Asserts that the client is not revoked (`revoked_at` is null) and is not expired (`expires_at` is in the future).
  5. Queries `striatumd.client_capabilities` (lines 88-101) to verify that the client possesses the required capability (`read`, `write`, `claim`, `review`, `apply`, `admin`, `recovery`) either globally or explicitly scoped to the requested `repository_id`.
  6. On failure, returns explicit denial reasons (`token_invalid`, `token_expired`, `capability_missing`, `capability_scope_mismatch`) which map directly to unique process exit codes.

### RPC Handler Coverage Assertion
- To prevent architectural regressions, `go/cmd/striatumd/handler_coverage_test.go` exercises `TestGoDaemonMethodCoverageIsExplicit` (lines 40-69).
- This test dynamically compiles all contract methods listed in `contracts/daemon_methods.json` (compiled via `registry_methods.go`), invokes each handler with synthetic parameters, and **fails the build** if any method is unregistered or responds with a `not_implemented` placeholder.
- Currently, **100% of production daemon contract methods** are fully implemented in Go.

---

## 5. Model Context Protocol (MCP) Boundary

Model Context Protocol (MCP) exposes Striatum's workflow primitives directly to terminal-based AI agents via a loopback HTTP/SSE server.

### Security Constraints and Loopback Enforcement
1. **Loopback Binding**: The MCP server binds exclusively to `localhost` / loopback IPs. `listenMCPHTTP` (in `go/cmd/striatumd/main.go` lines 375-398) explicitly splits the host/port and fails to launch if binding to non-loopback addresses.
2. **CORS & Host Header Guardrails**: In `validateLocalRequest` (in `go/pkg/mcp/http.go` lines 541-550), the server parses incoming HTTP headers:
   ```go
   func validateLocalRequest(r *http.Request) *localRequestError {
       if !isLoopbackHost(r.Host) {
           return &localRequestError{Code: "bad_host", Message: "Host must be loopback"}
       }
       origin := strings.TrimSpace(r.Header.Get("Origin"))
       if origin != "" && !isLoopbackOrigin(origin) {
           return &localRequestError{Code: "bad_origin", Message: "Origin must be loopback"}
     }
     return nil
   }
   ```
   If either the Host header or the CORS Origin header resolves to a non-loopback address, the server rejects the request with a **403 Forbidden** HTTP status and a `jsonrpcForbidden` JSON-RPC error.
3. **Bearer Authentication**: The MCP SSE channel requires bearer token authorization. It extracts the capability token via `bearerToken` (lines 371-381) and rejects requests with a **401 Unauthorized** status if missing or malformed.

### Dynamic Tool Discovery
- The MCP server does not blindly expose all RPC methods. `VisibleTools` (in `go/pkg/mcp/capabilities.go` lines 17-40) dynamically lists tools by verifying the client's bearer token against each method's required capability.
- **Internal methods** (`daemon.hello`, `daemon.describe`) and **deprecated methods** are omitted.
- **Workflow authoring methods** are explicitly classified as hidden:
  ```go
  // go/pkg/mcp/capabilities.go lines 60-74
  func isHiddenProductionTool(method string) bool {
      switch method {
      case "workflow.validate",
          "workflow.plan",
          "workflow.graph",
          "workflow.templates.list",
          "workflow.templates.show",
          "workflow.init",
          "workflow.generate",
          "workflow.upgrade":
          return true
      default:
          return false
      }
  }
  ```
- If an agent attempts to execute a hidden tool, `ToolsCall` (in `go/pkg/mcp/tools.go` lines 34-37) intercepts the request and terminates it immediately, returning `tool_hidden` ("MCP tools/call does not execute hidden production tools").

---

## 6. Supervisor Subsystem and PTY/Process Control

PTY and process execution are managed by `striatum-supervisor-helper` and `go/pkg/supervisor`, separating high-risk command execution from database mutation.

### Decoupled Helper Architecture
- `RunHelper` (in `go/pkg/supervisor/helper.go` lines 81-183) handles subprocess coordination under a PTY without directly importing PostgreSQL or executing daemon RPCs.
- It receives a process specification via stdin (`HelperLaunchSpec`), launches the child process via `helperLaunch`, and establishes concurrent pumping channels:
  - `pumpPTYProgress` (lines 264-311): Reads child stdout/stderr bytes from the PTY and emits progress events (`HelperEventProgress`) back to stdout.
  - `forwardPacketStream` (lines 200-245): Reads input packet frames from stdin or a named FIFO pipe (`PacketInputPath`), forwards them to the PTY's stdin, and emits event frames (`HelperEventPacketAccepted`).

### Tmux Liveness & Attestation Probing
- For lanes utilizing tmux-backed isolation (`require_tmux`), liveness is verified by `ProbeTmuxLiveness` (in `go/pkg/supervisor/tmux_liveness.go` lines 141-206):
  1. Issues `has-session -t <sessionName>` to ensure the session is active.
  2. Queries `display-message -p -t <paneID> "#{pane_id}\|#{pane_pid}\|#{pane_dead}\|#{pane_start_time}"`.
  3. Verifies that the PID start time token matches the record to prevent PID recycling issues.
- Probes return structural states: `tmux_ok`, `tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`, `tmux_pane_pid_mismatch`, `tmux_unavailable`.

### Linux PID Start Time Token Guardrail
- On Linux, PID liveness alone is vulnerable to PID recycling. To prevent a new, unrelated process from being mistakenly identified as an active supervisor, Striatum queries the kernel start-time tick:
  ```go
  // go/pkg/supervisor/process_identity_linux.go lines 13-32
  func ProcessStartToken(pid int) (string, bool) {
      if pid <= 0 {
          return "", false
      }
      data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
      if err != nil {
          return "", false
      }
      text := string(data)
      endComm := strings.LastIndex(text, ")")
      if endComm < 0 || endComm+1 >= len(text) {
          return "", false
      }
      fields := strings.Fields(text[endComm+1:])
      const starttimeIndex = 22 - 3
      if len(fields) <= starttimeIndex {
          return "", false
      }
      return fields[starttimeIndex], true
  }
  ```
- Field 22 in `/proc/<pid>/stat` counts the clock ticks since system boot at process initialization. This is treated as a cryptographically secure, immutable start-time token verified during all liveness assertions.

---

## 7. Supervision State Transitions

Lanes transition between states to coordinate asynchronous operations securely.

```
       [Start Config Loaded]
                 │
                 ▼
       ┌───────────────────┐
       │     starting      │  ◄── Advisory Transaction Lock on
       └─────────┬─────────┘      "striatum:supervise_start:<repo>:<session>"
                 │
                 │  Helper reports "agent_started" with PID & Start Time
                 ▼
       ┌───────────────────┐
       │     attached      │  ◄── Supervised Send writes JSON to FIFO pipe
       └────┬───────────┬──┘      (fails closed if ENXIO / reader missing)
            │           │
            │           │  Helper reports "attach_client_exited"
            │           │  but TMUX pane is still alive
            │           ▼
            │     [attached, degraded]
            │
            │  Helper reports "agent_exited" or PID gone
            ▼
       ┌───────────────────┐
       │      stopped      │
       └───────────────────┘
```

### 1. `starting` State
- **Invocation**: Triggered by `HandleSuperviseStart` (in `go/pkg/mutations/supervision_control.go` lines 104-254).
- **Mutual Exclusion**: Acquires a transaction-level advisory lock `pg_advisory_xact_lock` using a key composed of `striatum:supervise_start:<repo_id>:<session_id>` (line 641). This ensures that concurrent start operations for the same lane fail immediately, preventing race conditions.
- **Action**: Inserts baseline rows into database tables (`process_supervisors`, `process_supervisor_pointers`, `daemon_supervisors`) with `starting` state and writes a `supervisor.starting` event. It then launches the PTY helper asynchronously.

### 2. `attached` State
- **Liveness Attestation**: Once the helper outputs `agent_started` and the daemon reads the child PID and kernel start-time token, the database records are updated to `attached` state.
- **Supervised Send**: When writing packets (`HandleSuperviseSend` lines 256-361):
  - Calls `drainHelperEvents` (lines 1285-1326) to catch up on any asynchronous helper exits from `helper-events.jsonl`.
  - Verifies process liveness via `reconcileSupervisorForDelivery`. If the process is gone or its start token mismatched, it automatically marks the supervisor `lost` in the DB and fails closed.
  - Opens the pipe non-blockingly (`O_WRONLY|O_NONBLOCK`). If it fails with `ENXIO` (no reader is listening), the helper has exited; the daemon marks delivery as degraded (`attach_client_exited` / `stdin_reader_missing`) and fails closed.
  - Writes the JSON payload with a newline terminator.
  - If the stdin delivery mode is `"one_shot_eof"`, it removes the FIFO pipe immediately after the write, preventing further packet injections.

### 3. Degraded / Reattach States
- If the PTY helper's attach client exits (e.g. user detaches tmux), the helper emits `attach_client_exited`.
- `HandleSuperviseReport` captures this. It probes liveness. If the tmux pane is still active, the supervisor state remains `attached`, but its metadata records `"delivery_degraded": true` and `"delivery_liveness": {"class": "degraded", "healthy": false, "reason": "attach_client_exited"}`.
- Subsequent `supervise.send` calls fail closed, requiring the operator to rebridge or reattach.

### 4. `stopped` State
- **Invocation**: Triggered by child process exit (helper emits `agent_exited` or liveness check fails) or an explicit `supervise.stop` RPC.
- **Cleanup**: `HandleSuperviseStop` (lines 363-466) sends `kill-session` to tmux or sends SIGTERM to the process. It deletes the named FIFO pipe and updates the supervisor state to `stopped` in all database rows, recording the exact `ended_at` and `stop_reason` timestamps.

---

## 8. Conclusion
The audit reveals that **Striatum** maintains a highly secure, transaction-safe, and robust service boundary. The daemon acts as the authoritative boundary for all actions, enforcing dynamic token privilege scopes and verifying lane liveness using Linux procfs start times and tmux pane identity. The CLI behaves exclusively as a client to this boundary, resolving target repositories through RPC and delegating file-authoring commands locally. Subprocesses are isolated through decoupled helpers, PTY abstraction, and environment scrubbing, providing strong protection against privilege escalation or unauthorized local modifications.
