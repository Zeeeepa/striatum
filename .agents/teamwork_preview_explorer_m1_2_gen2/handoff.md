# RFC 0090 Planning Handoff Report: Workspace Security & Attestation Parity

## 1. Observation

This read-only investigation analyzed the Striatum Go codebase to plan the implementation of RFC 0090. Below are the direct observations from the codebase:

### Hardening Dimension 1: Path-Jail Symlink Resolver
* **File & Function:** `go/pkg/mutations/artifact.go`, function `repoRelativePath` (lines 335–352).
* **Verbatim Code:**
  ```go
  func repoRelativePath(repoRoot, pathText string, allowState bool) (string, error) {
	if filepath.IsAbs(pathText) {
		return "", fmt.Errorf("artifact path must be repo-relative")
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	resolved := filepath.Clean(filepath.Join(repoAbs, pathText))
	if !sameOrInside(resolved, repoAbs) {
		return "", fmt.Errorf("artifact path must stay inside the repository")
	}
	statePath := filepath.Join(repoAbs, ".striatum")
	if !allowState && sameOrInside(resolved, statePath) {
		return "", fmt.Errorf("artifact path cannot be under .striatum")
	}
	return resolved, nil
  }
  ```
* **Analysis:** Standard lexical clean via `filepath.Clean` does not resolve symlinks on disk. If a symlink points to a path outside the repository, `repoRelativePath` passes lexical bounds checks, but subsequent file system access (e.g., `os.ReadFile(path)` or `os.Stat(path)`) breaks the sandbox jail.

### Hardening Dimension 2: Dynamic Advisory Lock Derivation
* **File & Lines:** `go/pkg/db/migrations.go` (lines 16–19, 83–91, 118–121).
* **Verbatim Code:**
  ```go
  const (
	LatestDaemonDBVersion = 17
	MigrationLockKey      = 332933
  )
  ```
  ```go
  func ApplyMigrations(ctx context.Context, runner Runner, daemonVersion string) (int, error) {
	if err := runner.Exec(ctx, "SELECT pg_advisory_lock($1)", MigrationLockKey); err != nil {
		return 0, err
	}
	unlocked := false
	defer func() {
		if !unlocked {
			_ = runner.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", MigrationLockKey)
		}
	}()
  ```
* **Analysis:** The constant lock key `332933` is hardcoded. Parallel executions on the same database server but with different schemas (e.g., in multi-tenant environments) can block each other.

### Hardening Dimension 3: Supervisor Named-Pipe ENXIO Resilience Ring-Buffer
* **File & Lines:** `go/pkg/mutations/supervision_control.go`, functions `writeToPipe` (lines 1186–1219) and `launchPipeProcess` (lines 1250–1256).
* **Verbatim Code:**
  ```go
  func writeToPipe(ctx context.Context, pipePath string, payload []byte) (int, error) {
	fd, err := syscall.Open(pipePath, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ENXIO) {
			return 0, errSupervisorPipeNoReader
		}
		return 0, err
	}
  ```
  ```go
  func launchPipeProcess(ctx context.Context, config supervisionStartConfig, supervisorID, pipePath string) (supervisionLaunchResult, error) {
	fd, err := syscall.Open(pipePath, syscall.O_RDWR, 0)
	if err != nil {
		return supervisionLaunchResult{}, fmt.Errorf("open stdin fifo: %w", err)
	}
  ```
* **Analysis:** On Linux, `syscall.Open` with `O_WRONLY|O_NONBLOCK` on a FIFO returns an `ENXIO` error when no process has the FIFO open for reading. When this occurs, payloads are lost, and `errSupervisorPipeNoReader` is returned immediately.

### Hardening Dimension 4: pgtest Unprivileged Connection Pool
* **File & Lines:** `go/pkg/pgtest/pgtest.go` (lines 1–100).
* **Analysis:** The `pgtest.go` helper only provides administrative access via superuser connections:
  ```go
  testURL, drop := createDatabase(t, ctx, baseURL)
  ...
  pool, version, err := db.ConnectAndMigrate(ctx, testURL, "pgtest")
  ```
  There are currently no provisions to test mutations under unprivileged PostgreSQL roles, which prevents the verification of security constructs like `REVOKE UPDATE, DELETE ON striatumd.events`.

### Hardening Dimension 5: macOS Darwin Process Attestation Parity
* **File & Lines:** `go/pkg/supervisor/start_time_darwin.go` (lines 23–40).
* **Verbatim Code:**
  ```go
  func readProcessStartTimeOS(pid int) (time.Time, bool) {
	out, err := exec.Command("/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return time.Time{}, false
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse("Mon Jan  2 15:04:05 2006", value)
  ...
  ```
* **Analysis:** The Darwin start-time check relies on shelling out to `/bin/ps`, which is slow, susceptible to manipulation/mocking, and does not provide process attestation parity with Linux's direct `/proc` parsing.

### Hardening Dimension 6: Dynamic Free Loopback Port Discovery
* **File & Lines:** `go/cmd/striatumd/main.go` (lines 329–338, 717–722).
* **Verbatim Code:**
  ```go
  func defaultMCPHTTPAddr() string {
	if value := os.Getenv("STRIATUM_DAEMON_MCP_HTTP_ADDR"); strings.TrimSpace(value) != "" {
		return value
	}
	return "127.0.0.1:0"
  }
  ```
  ```go
	listener, err := listenMCPHTTP(value)
	if err != nil {
		return nil, err
	}
	endpoint := mcpEndpointURL(listener.Addr())
	endpointPath, err := writeMCPEndpointFile(endpoint)
  ```
* **Analysis:** The daemon already binds dynamically to port `:0` by default. However, there is no unified, secure discovery format containing socket paths, dynamic ports, and credentials (client token) written atomically under a permission-guarded structure (`0o600`).

---

## 2. Logic Chain

1. **Path-Jail breakout vulnerability** -> Standard `filepath.Join` and `filepath.Clean` evaluate paths lexically but do not resolve symlinks. A malicious process could place a directory symlink inside the workspace pointing to `/etc/` or other secure system directories. When the user requests an artifact path like `symlink/passwd`, `repoRelativePath` resolves it as `repoRoot/symlink/passwd` (which structurally resides inside the repo), but the operating system follows the symlink outside the repository root. Evaluating all paths via `filepath.EvalSymlinks` against an evaluated `repoRoot` will securely enforce the sandbox jail.
2. **Dynamic lock derivation** -> The constant lock key `332933` can block parallel migration instances on multi-tenant DB hosts. Deriving a lock key by hashing the active PostgreSQL database name (`current_database()`) concatenated with the schema name (`striatumd`), and taking the first 8 bytes as a signed `int64` key, provides guaranteed isolation for parallel schemas.
3. **Supervisor ENXIO resilience** -> When the child process starts slowly, opening the FIFO yields `ENXIO`. Associating a thread-safe memory-backed `NamedPipeBuffer` with each FIFO path allows the daemon to buffer writes during `ENXIO`, flushing all accumulated packets in order the moment the child reader successfully binds to the pipe.
4. **pgtest unprivileged connections** -> To guarantee that table immutability (`REVOKE UPDATE, DELETE`) is operational and verified, `pgtest` must support establishing unprivileged user credentials, applying granular table privilege controls, and validating that unprivileged writes trigger SQL exceptions.
5. **Darwin process attestation** -> Relying on shell-outs to `/bin/ps` introduces command-injection risks and poor runtime latency. Replacing this with BSD native `sysctl` kernel MIB tokens (`CTL_KERN`, `KERN_PROC`, `KERN_PROC_PID`) fetches process `extern_proc.p_starttime` structures directly via standard syscalls, achieving bulletproof process attestation.
6. **Port discovery** -> To allow clients (like the CLI or interactive agents) to dynamically discover a random loopback port, a secure JSON-encoded `discovery.json` file must be written atomically to the `XDG_RUNTIME_DIR/striatum` cache directory with `0o600` permissions.

---

## 3. Caveats

* **macOS sysctl Structure Alignment:** The `kinfo_proc` struct layout depends on the target CPU architecture (x86_64 vs arm64) and system word size. The offsets of structural fields like `tv_sec` and `tv_usec` inside `extern_proc.p_starttime` must be verified on both Apple Silicon and Intel hardware to prevent incorrect parsing.
* **Temporary Non-Existent Files in Symlink Check:** `filepath.EvalSymlinks` requires all intermediate directories to exist. If checking a path that has not yet been created, we must evaluate symlinks on the nearest existing parent directory.

---

## 4. Conclusion

RFC 0090 can be fully and securely implemented in the Go codebase. The planned modifications are entirely self-contained, high-performance, and drastically raise the attestation and security posture of the workspace boundary.

### Recommended Step-by-Step Implementation Strategy

#### Dimension 1: Path-Jail Symlink Resolver
* **Target File:** `go/pkg/mutations/artifact.go`
* **New Function:** `ValidateSandboxJail`
  ```go
  func ValidateSandboxJail(repoRoot, pathText string) (string, error) {
	if filepath.IsAbs(pathText) {
		return "", fmt.Errorf("path must be repo-relative")
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	repoRootResolved, err := filepath.EvalSymlinks(repoAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve repo root: %w", err)
	}
	targetAbs := filepath.Clean(filepath.Join(repoRootResolved, pathText))
	targetResolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(targetAbs)
			dirResolved, dirErr := filepath.EvalSymlinks(dir)
			if dirErr == nil {
				targetResolved = filepath.Join(dirResolved, filepath.Base(targetAbs))
			} else {
				return "", dirErr
			}
		} else {
			return "", err
		}
	}
	if !sameOrInside(targetResolved, repoRootResolved) {
		return "", fmt.Errorf("path escapes repository jail via symlinks")
	}
	return targetResolved, nil
  }
  ```
* **Integration:** Replace the lexical check block inside `repoRelativePath` with `ValidateSandboxJail`.

#### Dimension 2: Dynamic Advisory Lock Derivation
* **Target File:** `go/pkg/db/migrations.go`
* **New Function:** `deriveMigrationLockKey`
  ```go
  func deriveMigrationLockKey(ctx context.Context, runner Runner) (int64, error) {
	dbName, err := runner.QueryScalar(ctx, "SELECT current_database()")
	if err != nil {
		return 0, err
	}
	schemaName := "striatumd"
	sum := sha256.Sum256([]byte(dbName + ":" + schemaName))
	var val uint64
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(sum[i])
	}
	return int64(val), nil
  }
  ```
* **Integration:** Call `deriveMigrationLockKey` inside `ApplyMigrations` and pass the returned `lockKey` dynamically to `pg_advisory_lock` / `pg_advisory_unlock` instead of the static `MigrationLockKey`.

#### Dimension 3: Supervisor Named-Pipe ENXIO Resilience Ring-Buffer
* **Target File:** `go/pkg/mutations/supervision_control.go`
* **New Type:** `NamedPipeBuffer`
  ```go
  type NamedPipeBuffer struct {
	mu         sync.Mutex
	capacity   int
	queue      [][]byte
	totalBytes int
	maxBytes   int
  }
  ```
  Provide `Push` (supporting FIFO dropping on capacity/byte limits) and `PopAll` functions.
* **Integration:** Maintain a package-level synchronized map of pipe buffers (`map[string]*NamedPipeBuffer`). In `writeToPipe`, if opening the pipe yields `ENXIO`, call `Push(payload)`. When opening succeeds, call `PopAll` and flush all buffered payloads to the FIFO in order before writing the active payload.

#### Dimension 4: pgtest Unprivileged Connection Pool
* **Target File:** `go/pkg/pgtest/pgtest.go`
* **New Function:** `UnprivilegedPool(t *testing.T) *db.Pool`
  1. Bootstraps the test database using standard administrative credentials.
  2. Runs migrations and creates a fresh unprivileged role: `striatum_unprivileged`.
  3. Revokes UPDATE and DELETE privileges:
     ```sql
     REVOKE UPDATE, DELETE ON TABLE striatumd.events FROM striatum_unprivileged;
     REVOKE UPDATE, DELETE ON TABLE striatumd.artifacts FROM striatum_unprivileged;
     ```
  4. Returns a separate connection pool authenticated under the unprivileged role credentials.
  5. Add test suites to verify that executing update/delete queries on these tables throws SQL errors.

#### Dimension 5: macOS Darwin Process Attestation Parity
* **Target File:** `go/pkg/supervisor/start_time_darwin.go`
* **Implementation:** Rewrite `readProcessStartTimeOS` to invoke `sysctl` with the `CTL_KERN`, `KERN_PROC`, `KERN_PROC_PID` MIB tokens via standard `syscall.Syscall6`. Parse `p_starttime` structures out of the returned `kinfo_proc` struct directly from kernel memory.

#### Dimension 6: Dynamic Free Loopback Port Discovery
* **Target File:** `go/cmd/striatumd/main.go`
* **New Function:** `writeDaemonDiscoveryFile`
  Creates a JSON structure containing:
  - `pid`
  - `socket_path`
  - `mcp_http_url`
  - `mcp_http_port`
  - `client_token` (securely fetched via `readRuntimeTokenFile`)
  Atomic write is performed using temporary files and `os.Rename` under `0o600` permissions.
* **Integration:** Call `writeDaemonDiscoveryFile` immediately after binding the dynamic port inside `startMCPHTTPServer`. Remove the file cleanly on shutdown.

---

## 5. Verification Method

To verify the planned implementation independently, an engineer can perform the following steps:

1. **Verify Symlink Jailbreak Prevention:**
   * Create a directory symlink: `ln -s /etc/ sandbox_breakout` inside the repository.
   * Attempt to publish an artifact using path `sandbox_breakout/passwd`.
   * Assert that `artifact.publish` returns `escapes repository jail via symlinks` error.
2. **Verify Dynamic Lock Independence:**
   * Run two daemon migration instances concurrently targeting isolated schemas on the same PG database.
   * Verify they migrate simultaneously without blocking each other on the static advisory lock.
3. **Verify named-pipe ENXIO buffer resilience:**
   * Write payloads to the pipe before launching the child reader.
   * Start the reader and verify that all buffered packets are successfully received by the child stdin.
4. **Verify unprivileged connection testing:**
   * Run the test suite:
     ```bash
     make test
     ```
   * Assert that tests leveraging `UnprivilegedPool` successfully throw permission exceptions on unauthorized mutations.
5. **Verify Darwin attestation:**
   * Run process start-time verification tests on macOS and assert they succeed without invoking `/bin/ps`.
6. **Verify discovery file security:**
   * Check that `$STRIATUM_DAEMON_RUNTIME_DIR/discovery.json` exists during execution, has `-rw-------` (`0o600`) permissions, and is cleanly deleted upon daemon exit.
