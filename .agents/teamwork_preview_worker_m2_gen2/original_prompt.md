## 2026-05-29T07:53:25Z

You are the teamwork_preview_worker (M2_Gen2).
Your role is: Workspace Security & Attestation Hardening Implementer.
Your working directory is: ~/git/striatum/.agents/teamwork_preview_worker_m2_gen2

### Objective:
Implement RFC 0090 (Workspace Security & Attestation Parity). You must carefully follow the detailed findings and step-by-step implementation strategy formulated by the Explorer in their handoff report:
`~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2/handoff.md`

### Tasks to Execute:
1. **Transition RFC 0090 Status**:
   - Open `~/git/striatum/docs/rfcs/0090-hardening-local-workspace-security-and-attestation-parities.md` and change its header `Status: proposed` to `Status: accepted`.
   - Update any necessary ubiquitous-language or documentation files.

2. **Scoped Symlink Path-Jail Resolver**:
   - In `go/pkg/mutations/artifact.go`, implement the `ValidateSandboxJail` function:
     - Absolute targets are computed and recursively checked using `filepath.EvalSymlinks`.
     - Ensure that any target path (or nearest existing parent directory if target doesn't exist yet) resolved canonical destination resides strictly inside the resolved canonical repository root (`sameOrInside`).
     - Throw a `symlink_traversal_blocked` or standard sandbox jail error if it escapes.
   - Replace the lexical boundaries check in `repoRelativePath` with `ValidateSandboxJail`.

3. **Dynamic Advisory Lock Derivation**:
   - In `go/pkg/db/migrations.go`, implement a lock key derivation function `deriveMigrationLockKey(ctx, runner)`:
     - Query active database name using `SELECT current_database()` and schema name.
     - Hash `"dbName:schemaName"` with SHA-256 and extract the first 8 bytes as a safe 64-bit signed integer compatible with Postgres `pg_advisory_lock` functions.
     - Use this dynamically derived key inside `ApplyMigrations` instead of the hardcoded static `332933` constant lock key.

4. **Supervisor Named-Pipe ENXIO Resilience Ring-Buffer**:
   - In `go/pkg/mutations/supervision_control.go`, implement a bounded, thread-safe queue of size 10 (`NamedPipeBuffer`).
   - Associate a thread-safe map of buffers (`map[string]*NamedPipeBuffer`) keyed by FIFO pipe path.
   - In `writeToPipe`, if opening the pipe yields `syscall.ENXIO` (meaning no reader is attached), queue the payload in the buffer.
   - When opening succeeds, flush all buffered packets in order down the pipe before writing the active payload.
   - Ensure that if queue overflows (exceeds 10 packets), the lane is marked degraded and fails closed to prevent infinite memory growth.

5. **Privilege Validation Test Harness Pools**:
   - In `go/pkg/pgtest/pgtest.go`, enhance the harness:
     - Setup the test database as superuser, run migrations, and then dynamically create an unprivileged role `striatumd_rw_test` with standard read-write permissions.
     - Grant DML revokes (`REVOKE UPDATE, DELETE ON striatumd.events, striatumd.artifacts`) on events/artifacts tables.
     - Return two connection pools: privileged for migrations/setup and unprivileged for test mutations.
     - Add integration tests verifying that attempting to update or delete rows under the unprivileged pool fails with insufficient privilege error `42501`.

6. **macOS Darwin Process Attestation Parity**:
   - In `go/pkg/supervisor/start_time_darwin.go`, implement macOS process start-time validation using Darwin's native `sysctl` interface:
     - Select `CTL_KERN`, `KERN_PROC`, `KERN_PROC_PID` selectors and fetch process `extern_proc.p_starttime` structures directly via standard sysctl calls.
     - Eliminate ps shell-outs (`/bin/ps`) completely, bringing Mac platforms to attestation parity with Linux PROC stat ticks.

7. **Dynamic Loopback Port Discovery**:
   - In `go/cmd/striatumd/main.go`, implement `writeDaemonDiscoveryFile`:
     - Securely write active loopback HTTP port, PID, client token, etc. into a JSON file `discovery.json` under dynamic cache path with `0o600` permissions.
     - Cleanly remove the file on shutdown.

### Verification Requirement:
- Run `go test -race ./...` to compile and pass the entire Go test suite.
- Ensure zero lint/typecheck errors.

### MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

### Completion Criteria:
- Code and documentation changes successfully integrated in Go codebase.
- Entire Go test suite compiles and passes cleanly with zero race conditions.
- Call send_message to report completion back to the Project Orchestrator (Gen 2).
