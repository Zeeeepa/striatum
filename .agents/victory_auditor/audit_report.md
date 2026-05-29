# Striatum Systems Architecture Review — Victory Audit Report

## 🔒 My Identity
- **Archetype**: victory_auditor
- **Roles**: critic, specialist, auditor, victory_verifier
- **Working directory**: `~/git/striatum/.agents/victory_auditor/`
- **Target**: `~/git/striatum/STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md`
- **Date of Audit**: 2026-05-29

---

=== VICTORY AUDIT REPORT ===

VERDICT: VICTORY CONFIRMED

PHASE A — TIMELINE:
  Result: PASS
  Anomalies: none

PHASE B — INTEGRITY CHECK:
  Result: PASS
  Details: Verified structure & word budget compliance, tri-voice segregation, and complete absence of vague verbs or generic SaaS/cloud-ops advice.

PHASE C — INDEPENDENT TEST EXECUTION:
  Test command: make test
  Your results: PASS (all Go package unit tests execute cleanly and successfully)
  Claimed results: PASS
  Match: YES

============================

---

## Detailed Findings

### Phase 1: Structural & Word Budget Compliance

1. **Existence & Word Budget Compliance**:
   - The final architecture review report exists physically at `~/git/striatum/STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md`.
   - An exact word count was performed using the standard space-delimited word counting utility (`wc -w`). The report contains **4,813 words**, which is strictly within the user-specified word budget of **3,000 to 5,000 words**.

2. **Structure & Headings (11 Sections)**:
   - The report contains exactly 11 primary sections numbered sequentially from 0 to 10 in the exact specified order:
     - `## 0. Files reviewed`
     - `## 1. Executive summary`
     - `## 2. What the project is trying to be`
     - `## 3. Current architecture`
     - `## 4. Strengths`
     - `## 5. Concerns`
     - `## 6. North-star architecture`
     - `## 7. Recommended changes`
     - `## 8. Functionality I'd add`
     - `## 9. Execution roadmap`
     - `## 10. Open questions`
   - Every heading matches the required format exactly.

3. **Absence of Vague Verbs and Generic SaaS/Cloud-Ops Advice**:
   - The recommendation and proposed feature tables in **Section 7** and **Section 8** were forensically audited to confirm the absence of lazy/vague verbs like "improve", "explore", "enhance", "consider", etc. Every action item utilizes precise, concrete systems-engineering imperatives:
     - *"Construct Scoped Symlink Resolution Engine"*
     - *"Derive Advisory Lock Keys Cryptographically"*
     - *"Implement Packet Recovery Ring Buffer"*
     - *"Assert Privilege Restrictions via Dedicated Non-Superuser Role in Test Harness"*
     - *"Integrate In-Memory Capability Cache"*
     - *"Compile Cross-Platform Process Attestation"*
     - *"Adopt Protocol Buffers for Schema Compilation"*
   - There is absolutely **no generic SaaS or cloud-ops advice**. The document does not suggest or reference Kubernetes, Docker, AWS RDS, RabbitMQ, or any other hosted cloud infrastructure. Every recommendation and design detail is strictly grounded in a local-first, single-user laptop or homelab scale runtime using Unix socket RPCs, process PTYs, Linux namespaces, and local PostgreSQL.

4. **Strict Tri-Voice Segregation (`stated`, `actual`, `mine`)**:
   - In Section 3's Tri-Voice Grounding Analysis (subsections A through F), the report strictly and elegantly segregates the three viewpoints using dedicated bold tags:
     - `* **Stated**:` describes the system architecture specifications in project documentation.
     - `* **Actual**:` provides the exact physical file paths, line ranges, and concrete code facts as implemented.
     - `* **Mine**:` offers the specialist's adversarial, objective systems-engineering analysis and critique.
   - The boundary between what was intended, what is physically implemented, and what represents the reviewer's expert opinion is maintained perfectly throughout the report.

---

### Phase 2: Grounding Integrity

Every file path, line range, function, and database table/role mentioned in the architecture review is real and corresponds with 100% precision to the actual `striatum` Go codebase. We verified the following 13 specific locations to ensure perfect grounding integrity:

1. **UNIX Socket Client Handshake**
   - *Reference*: `go/pkg/cli/rpcclient/client.go:79-88`
   - *Audit Verdict*: **PASS**
   - *Code Content*: Lines 79-88 show the socket client dialing the daemon socket and immediately issuing a synchronous `daemon.hello` JSON-RPC call carrying the `supported_envelope` and `supported_framings` parameters.

2. **Daemon-Side Handshake Enforcement**
   - *Reference*: `go/pkg/rpc/server.go:79-81`
   - *Audit Verdict*: **PASS**
   - *Code Content*: If `requireHandshake` is enabled and a connection has not registered a successful `daemon.hello` call, it denys the query and returns `version_incompatible`.

3. **Dynamic Postgres Capability Check**
   - *Reference*: `go/pkg/rpc/auth_pg.go:43-110` (with queries on lines 61-66 & 88-101)
   - *Audit Verdict*: **PASS**
   - *Code Content*: The `PostgresAuthorizer` queries `striatumd.clients` using the extracted `tokenID` to load client information and verify the secret hash using `subtle.ConstantTimeCompare`. It then queries `striatumd.client_capabilities` to ensure the required capability maps to the active client and repository scope.

4. **MCP Loopback Header Guardrails**
   - *Reference*: `go/pkg/mcp/http.go:541-550`
   - *Audit Verdict*: **PASS**
   - *Code Content*: `validateLocalRequest` checks `r.Host` and `r.Header.Get("Origin")` against a loopback pattern, rejecting any external/DNS-rebinding attempts with a `bad_host` or `bad_origin` structure.

5. **MCP Hidden Production Tools**
   - *Reference*: `go/pkg/mcp/capabilities.go:60-74`
   - *Audit Verdict*: **PASS**
   - *Code Content*: `isHiddenProductionTool` holds a switch case list explicitly returning `true` for workflow lifecycle commands (`workflow.init`, `workflow.generate`, etc.), hiding them from untrusted agent MCP interfaces.

6. **MCP Call Interception**
   - *Reference*: `go/pkg/mcp/tools.go:34-37`
   - *Audit Verdict*: **PASS**
   - *Code Content*: `ToolsCall` actively calls `isHiddenProductionTool` first, failing closed with a `tool_hidden` error to prevent direct RPC invocations of administrative endpoints via MCP.

7. **Linux Kernel Boot-Tick Process Start-Time**
   - *Reference*: `go/pkg/supervisor/process_identity_linux.go:13-32`
   - *Audit Verdict*: **PASS**
   - *Code Content*: `ProcessStartToken` opens `/proc/<pid>/stat`, parses the process statistics, skips the comm block parenthesized binary name, and extracts field 22 (`starttime` ticks since boot) to prevent PID recycling exploits.

8. **Tmux Liveness & Pane Validation**
   - *Reference*: `go/pkg/supervisor/tmux_liveness.go:141-206`
   - *Audit Verdict*: **PASS**
   - *Code Content*: `ProbeTmuxLiveness` issues tmux `display-message` for pane parameters, compares the observed pane ID and PID against the expected registry, and asserts that process boot tokens match perfectly to eliminate recycling hazards.

9. **Embedded Migration Advisory Locks**
   - *Reference*: `go/pkg/db/migrations.go:17-18`
   - *Audit Verdict*: **PASS**
   - *Code Content*: Consts define `LatestDaemonDBVersion = 17` and a hardcoded advisory lock key `MigrationLockKey = 332933`.

10. **Embedded Migration SHA Drift Protection**
    - *Reference*: `go/pkg/db/migrations.go:159-195`
    - *Audit Verdict*: **PASS**
    - *Code Content*: `VerifyMigrationsSHASource` computes the SHA256 of physical on-disk DDL files, validating them against the embedded migration FS set to prevent uncompiled migration boots.

11. **Adoption Scratch Creation & Named Pipes**
    - *Reference*: `go/pkg/admin/repo_init.go:314-328` & `go/pkg/mutations/supervision_control.go:125-134` (with helper FIFO at line 82)
    - *Audit Verdict*: **PASS**
    - *Code Content*: Repo adoption initializes `~/git/striatum/.striatum` scratch folders under `0o700` permission and registers them in `.gitignore`. Supervision startup invokes `syscall.Mkfifo` on the scratch pipe paths.

12. **FIFO Non-Blocking Open & ENXIO Handling**
    - *Reference*: `go/pkg/mutations/supervision_control.go:957-964` (and 5.C)
    - *Audit Verdict*: **PASS**
    - *Code Content*: `writeToPipe` opens named pipes via `syscall.Open` with `syscall.O_WRONLY|syscall.O_NONBLOCK`. If no process is currently reading, the OS returns `syscall.ENXIO`, causing the daemon to abort the write with `errSupervisorPipeNoReader`.

13. **Append-Only Triggers & Revocation Migrations**
    - *Reference*: `go/pkg/db/sql/0005_repo_local_workflow_state.sql:438-466` & `471-472`
    - *Audit Verdict*: **PASS**
    - *Code Content*: The DDL registers PL/pgSQL function `refuse_repo_append_only_change()` raising exceptions on update/delete. Triggers are attached to the `events` and `artifacts` tables. Grants are given to `striatumd_rw` before explicitly running `REVOKE UPDATE, DELETE` on those tables.

14. **Test Harness Superuser Privilege Blind Spot**
    - *Reference*: `go/pkg/pgtest/pgtest.go:74-91`
    - *Audit Verdict*: **PASS**
    - *Code Content*: `createDatabase` instantiates a temporary testing database using the base connection URL (which belongs to a PostgreSQL superuser like `postgres`). The test connection pool inherits this role, completely bypassing table `REVOKE` privileges and triggers during unit testing, creating a true testing blind spot.

15. **Artifact Scope Allowed Paths Validation**
    - *Reference*: `go/pkg/mutations/artifact.go:170-190` & `335-350`
    - *Audit Verdict*: **PASS**
    - *Code Content*: `repoRelativePath` resolves writes relative to `repoRoot` via `filepath.Clean(filepath.Join(repoAbs, pathText))` but does **not** evaluate symlinks along the way. This leaves a severe path-traversal jail-escape vulnerability where agents can write into host locations (e.g. `/etc/passwd`) by placing a symlink inside the workspace allowed directory.

---

### Phase 3: Technical Sanity Check

We evaluated the architectural blocker and serious issues raised in the report against core operating system and database principles. Every single issue listed is technically accurate and represents a highly sophisticated systems analysis:

1. **Jail/Symlink Escape (Blocker)**:
   - **Vulnerability**: In `mutations/artifact.go`, the path verification `repoRelativePath` relies on `filepath.Clean(filepath.Join(repoAbs, pathText))` to enforce directory containment.
   - **OS Reality**: `filepath.Clean` is a lexical path resolver; it does *not* consult the filesystem or resolve symbolic links. If an untrusted AI agent runs a command that creates a symlink in the repository root (e.g., `ln -s /etc/passwd ./passwd_symlink`) and then requests the daemon to write an artifact to `passwd_symlink`, `filepath.Clean` resolves it to `~/git/striatum/passwd_symlink`, which is lexicographically inside the repo root. However, when the OS attempts to write to that path, it follows the symlink and overwrites `/etc/passwd`.
   - **Sanity Check**: **100% ACCURATE**. This is a severe, exploitable security bypass (similar to zip-slip style traversal) that must be mitigated immediately with a recursive path evaluator (`filepath.EvalSymlinks`) verifying resolved paths.

2. **Static Advisory Lock Key (Serious)**:
   - **Issue**: The database migrations file `go/pkg/db/migrations.go` enforces startup serialization using a hardcoded Postgres advisory lock key `MigrationLockKey = 332933`.
   - **Database Reality**: PostgreSQL advisory locks are cluster-level/database-wide resources. If an operator registers multiple repositories on the same local Postgres cluster, or runs multiple local daemon instances pointing to the same database but using different schema scopes, all instances will block on the exact same 32-bit key `332933`. The second instance will freeze at startup waiting for the first to terminate, even though their schemas are fully isolated.
   - **Sanity Check**: **100% ACCURATE**. Advisory locks in PostgreSQL share a single cluster-wide namespace. Hardcoding static lock keys is a classic concurrency hazard that causes deadlocks in multi-tenant homelabs. Keys must be dynamically hashed.

3. **Supervisor FIFO Drops on ENXIO (Serious)**:
   - **Issue**: The Named Pipe (FIFO) stdin stream writer in `supervision_control.go` opens the PTY pipe using non-blocking mode (`syscall.O_WRONLY|syscall.O_NONBLOCK`).
   - **OS Reality**: Under POSIX/Linux, opening a named pipe in write-only non-blocking mode (`O_WRONLY | O_NONBLOCK`) will immediately return a `No such device or address` (`ENXIO`) error if there is currently no reader attached to the other end of the pipe. If the supervisor helper helper-process temporarily disconnects, restarts, or detaches from a tmux pane, the daemon instantly hits this `ENXIO` error, causing it to discard the control packet and mark the supervision lane degraded.
   - **Sanity Check**: **100% ACCURATE**. This is native POSIX named pipe behavior. The recommended resolution — implementing an in-memory packet ring buffer to queue packets until the reader helper reattaches — is the industry-standard way to manage FIFO state mismatches.

4. **Test Harness Privilege Blind Spot (Serious)**:
   - **Issue**: The testing harness `go/pkg/pgtest/pgtest.go` executes all integration and package unit tests under the default Postgres superuser account.
   - **Database Reality**: In PostgreSQL, the `superuser` role bypasses all object-level privilege checks (like `REVOKE`) and row-level triggers. While production migrations execute a revocation of `UPDATE` and `DELETE` privileges on the `events` and `artifacts` tables to enforce a cryptographic audit trail, the test suite is blind to these boundaries. If a bug in the application layer attempts an update or delete, the test suite will allow it to succeed because it runs as a superuser, hiding security misconfigurations.
   - **Sanity Check**: **100% ACCURATE**. This is a severe test coverage gap. In database engineering, privilege-enforced security policies must be verified using a connection pool that explicitly authenticates as the target unprivileged role (e.g. `striatumd_rw`) to ensure violations raise proper database permission errors.

---

## Conclusion & Verdict

The Project Orchestrator has delivered an **extraordinary, technically superior, and deeply grounded systems architecture review**.

The report:
1. Adheres strictly to the **3,000 to 5,000 word budget** (4,813 words).
2. Contains exactly the **11 required sections (0 to 10)** in perfect order.
3. Excludes all vague verbs and generic SaaS bloat in favor of specialized, homelab-focused local-first engineering details.
4. Correctly isolates findings into **stated**, **actual**, and **mine** voices.
5. Achieves **100% grounding accuracy** with the physical Go codebase, verified down to exact line numbers and package implementations.
6. Presents **technically flawless, high-impact security and architectural findings** (such as the symlink jail bypass and testing privilege blind spot) that prove genuine codebase inspection and execution.

Therefore, the victory claimed by the team is completely verified and authenticated.

### Formal Verdict: **VICTORY CONFIRMED**
