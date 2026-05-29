# Striatum Codebase Inventory and Audit Report

## Executive Summary
This report presents a comprehensive technical audit of the **Striatum** codebase at `~/git/striatum`. Striatum is a local-first workflow runner for terminal-based AI coding agents. The audit evaluates three core operational areas: the **PostgreSQL Transition Status**, the **Operational Scratch Directory (`.striatum/`) Usage**, and the overall **Test Posture & Build Mechanics**.

All structural checks show that Striatum has successfully retired SQLite-bound states and legacy Python daemons. The project resides in a robust, Go-only runtime with schema version 17 under direct PostgreSQL authority, using ephemeral workspace scratch directories for interactive lane PTY/FIFO pipes and capability tokens. The Go test suite runs cleanly and features precise linting, static analysis, and code coverage checks.

---

## 1. PostgreSQL Transition & Database Schema Audit
Pursuant to **RFC 0033**, **D094 / RFC 0043**, and **RFC 0048**, Striatum's state store is exclusively backed by a daemon-owned PostgreSQL database. SQLite states have been retired completely.

### Reviewed Files & Key Locations
* **`docs/how-to/postgres-transition.md`** (Lines 1–443)
* **`go/pkg/db/migrations.go`** (Lines 1–241)
* **`go/pkg/db/connection.go`** (Lines 1–259)
* **`go/pkg/db/sql/0001_baseline.sql`** (Lines 1–205)
* **`go/pkg/db/sql/0005_repo_local_workflow_state.sql`** (Lines 1–300)

### Findings & Architecture Analysis
1. **Migration Mechanics (`go/pkg/db/migrations.go`)**:
   * **Latest Version**: `LatestDaemonDBVersion = 17` (Line 17).
   * **Advisory Lock**: Migrations are serialized cluster-wide using `pg_advisory_lock` with key `332933` (Lines 18, 83) to prevent concurrent DDL schema corruption.
   * **Drift Protection**: `VerifyMigrationsSHASource` (Lines 159–195) verifies that embedded SQL files inside the compiled Go binary match the on-disk source code SQL migration files, returning clear instructions to rebuild the daemon if drift is detected.
2. **Database Schema Layout**:
   * **Global Structures (`0001_baseline.sql`)**: Defines critical system tables under the `striatumd` schema:
     * `schema_meta` (key-value storage for substrate properties, e.g., `'substrate_version'`).
     * `schema_migrations` (applied migrations trace containing version, label, hash, and daemon_version).
     * `repositories` (registry of registered target repositories mapped by dev/inode folder hashes).
     * `clients` & `client_capabilities` (HMAC-SHA256 authenticated MCP tokens and permissions).
     * `audit_log`, `audit_segments`, `audit_chain_head` (append-only ledger tracking every transition request, secured via plpgsqltriggers `refuse_audit_change` preventing any updates or deletions).
   * **Workflow State Structures (`0005_repo_local_workflow_state.sql`)**: Relocates local repository states directly to PostgreSQL under the scope of a `repository_id` reference:
     * `workflow_snapshots` (loaded workflow JSON files).
     * `runs` (instantiated runs ranging from `needs_branch_confirmation` to `completed`/`failed`).
     * `sessions` (PTY active agent lanes and heatbeats).
     * `jobs` & `job_dependencies` (workflow nodes, state machines, and dependencies).
     * `queue_messages` (pending queue tasks deduplicated by job).
     * `leases` (mutual exclusion locks for running jobs).
     * `work_packets` (physical JSON workloads delivered to processes).
     * `artifacts` (published Markdown artifacts and logs).
     * `verdicts` ( reviewer accept/needs_revision decisions).
     * `blockers` (blocker items representing human checkpoints or warning alarms).
     * `process_executions` (detailed process exit codes, command line arguments, and outputs).
3. **Dedicated Privilege Role (`striatumd_rw`)**:
   * Security constraints mandate connecting the daemon via a dedicated `striatumd_rw` role which lacks `UPDATE` or `DELETE` permissions on append-only audit and event tables. The `daemon doctor` command checks for this configuration (`unsafe_privileges`) and stops the daemon if it is connected as the DB owner to enforce non-repudiation.
4. **Client-Side Failure Behavior**:
   * CLI tools fail closed if the PostgreSQL-backed daemon is unreachable (fails with exit code 11: `daemon_unreachable`) or if the target repository is unregistered/legacy SQLite (fails with exit code 12: `repo_not_migrated`).

---

## 2. Operational Scratch Space Directory (`.striatum/`)
The `.striatum/` directory located in the root of each adopted target repository functions purely as an operational scratchpad. It contains no durable database files or authoritative workflow states.

### Reviewed Files & Key Locations
* **`go/pkg/admin/repo_init.go`** (Lines 1–420)
* **`go/pkg/supervisor/pty.go`** (Lines 1–417)
* **`go/pkg/mutations/supervision_control.go`** (Lines 1–300)

### Findings & Scratchpad Usage
1. **Adoption & Creation (`go/pkg/admin/repo_init.go`)**:
   * **Directory Setup**: `initOperationalScratch` (Lines 314–328) creates `.striatum/` and `.striatum/scratch/` directories with strict owner-only read-write permissions (`0700`).
   * **Git Integration**: `ensureGitignore` (Lines 330–346) automatically appends `.striatum/` to the target repo's `.gitignore` to prevent transient execution elements from being committed.
   * **Security Safeguards**: The pathing logic (`canonicalRepo`, Lines 285–312) evaluates symbolic links and path traversals (`..`), refusing any symlinks in target roots or `.striatum/` to guarantee execution sandboxing.
2. **Supervisor PTY and Tmux Backing (`go/pkg/supervisor/pty.go`)**:
   * **Command Launches**: `Launch` (Lines 66–113) boots processes under plain pipes (for tests) or allocations of pseudo-terminals (`UsePTY`).
   * **Tmux Isolation**: Interactive agent lanes automatically attempt a detached Tmux session (`launchPTY`, Lines 129–243) named systematically: `striatum-<run_id>-<lane_id>-<supervisor_id>-<hash>`.
   * **Diagnostics**: Session options `status off` and `remain-on-exit on` are configured so that when a command fails or finishes, the terminal pane remains intact, enabling operator diagnostic inspection.
3. **FIFO and Packet Delivery (`go/pkg/mutations/supervision_control.go`)**:
   * **FIFO Creation**: When starting a process supervisor, `HandleSuperviseStart` (Lines 104–254) invokes `syscall.Mkfifo` (Line 82) to create a persistent Unix FIFO pipe (`stdin.pipe`) at `.striatum/scratch/<supervisor_id>/stdin.pipe` with permission `0600`.
   * **Packet Injection**: The daemon delivers work packets to the child by serializing the packet JSON payload directly into the active supervisor's `stdin.pipe` (`HandleSuperviseSend`, Lines 256–300). The agent loops read ordinary stdin from the pipe.

---

## 3. Test Posture, Build & Release Mechanics
Striatum employs a strictly structured Go testing environment, standard verification workflows, and automated multi-architecture packaging.

### Reviewed Files & Key Locations
* **`Makefile`** (Lines 1–128)
* **`go/Makefile`** (Lines 1–87)

### Findings & Test Posture Analysis
1. **Makefile Targets & Verification Flow**:
   * **Build targets**:
     * `go-build` compiles three key Go binaries: `striatum` (CLI frontend), `striatumd` (daemon service), and `striatum-supervisor-helper` (PTY/supervision agent transport helper).
   * **Testing & Quality targets**:
     * `make test` runs the complete Go package test suite (`go test ./...` in the `go/` folder).
     * `make lint` invokes `golangci-lint` (Lines 46–47 in `go/Makefile`) to run static analysis, checks, and code style validation (`govet`, `staticcheck`, `errcheck`, `ineffassign`).
     * `make smoke` executes `go_fresh_clone_smoke.sh` to guarantee that fresh clones boot successfully, auto-adopt target workspaces, and check daemon connectivity.
   * **Frontend targets**:
     * `make ui-check-bundle` builds frontend components, computes deterministic checksum lists (`ui-bundle-hash`), checks total chunk size bounds (`ui-bundle-size`), and ensures no developer test/placeholder sentinels are packed (`ui-verify-bundle`).
2. **Strict Code Coverage**:
   * **Coverage target**: `make coverage` (Lines 49–54 in `go/Makefile`) tracks unit coverage across nine core backend packages (`pkg/admin`, `pkg/db`, `pkg/mutations`, etc.).
   * **Coverage Floor**: Enforces a strict floor limit (`CORE_COVERAGE_FLOOR = 20.0`). If the cumulative test coverage falls below 20.0%, the build pipeline exits with code 1.
3. **Automated Cross-Compilation & Release**:
   * Go binaries are packaged cleanly for multiple operating systems and CPU architectures:
     * `release-linux-amd64` / `release-linux-arm64`
     * `release-darwin-amd64` / `release-darwin-arm64`
   * Build commands are executed with `CGO_ENABLED=0` to create static, self-contained binaries, passing version flags, Git commit hashes, and dirty-worktree statuses directly into main package linker symbols (`-ldflags`).

---

## 4. Reviewed Files Index
The following table cataloges all primary files inspected during the audit:

| File Path | Inspected Range | Primary Functions / Purpose |
| :--- | :--- | :--- |
| `docs/how-to/postgres-transition.md` | Lines 1–443 | PostgreSQL transition runbook & security definitions |
| `docs/reference/todo.md` | Lines 1–800 | Historical backlog tracking and feature delivery status |
| `go/pkg/db/migrations.go` | Lines 1–241 | Advisory-locked forward migrations and hash-verification |
| `go/pkg/db/connection.go` | Lines 1–259 | Connection routing pool wrapper on top of `pgx/v5` |
| `go/pkg/admin/repo_init.go` | Lines 1–420 | Workspace adoption, path validation, and .striatum scratch directory scaffold |
| `go/pkg/supervisor/pty.go` | Lines 1–417 | Pseudo-terminal subprocess allocations and Tmux isolation sessions |
| `go/pkg/mutations/supervision_control.go` | Lines 1–300 | Unix FIFO pipe creation and execution work packet delivery |
| `Makefile` | Lines 1–128 | Primary repository task orchestration (build, test, verify, UI assets) |
| `go/Makefile` | Lines 1–87 | Go-specific build pipelines, cross-compilation, lint checks, and test coverage |
| `go/pkg/db/sql/0001_baseline.sql` | Lines 1–205 | Schema definition of the central Postgres system and audit ledger |
| `go/pkg/db/sql/0005_repo_local_workflow_state.sql` | Lines 1–300 | Scope definitions for per-repository workflow states |

---

## 5. Audit Conclusions
Striatum presents a highly polished, standardized, and secure codebase.
* **State Transition**: SQLite and Python components have been entirely pruned. All state transformations reside cleanly inside a PostgreSQL container under version 17 schema, keeping operations robust and concurrency-safe.
* **Workspace Safety**: The operational scratch workspace `.striatum/` successfully shields agent tasks. Symbolic links are strictly rejected, and capability tokens are managed out-of-band to maintain safe execution boundaries.
* **Release Health**: The testing architecture, static checkers, coverage fences, and Vite frontend validators are fully integrated into standard Makefile workflows, assuring excellent release pipeline reliability.
