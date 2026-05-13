author: reviewer-unknown-model-001

# Build Review: Go Daemon Core (RFC 0039 Phase 2)

**Verdict:** `accept_with_findings`
**Posture:** `threat_model`
**Lane:** `gemini`

## Summary

The Go daemon core transition has established a solid RPC and supervision foundation. However, the adversarial review identifies several high-severity trust boundary regressions and forgery surfaces, primarily centered on incomplete verification logic in the Go implementation compared to the Python reference. The most critical findings are the PID-recycling race in liveness detection and the lack of cryptographic signature verification in apply receipts.

## Findings

### F1: PID-Recycling Race in Liveness Detection (High)
The Go liveness controller (`go/pkg/supervisor/liveness.go:121`) assesses process liveness using a simple signal-0 probe via `os.FindProcess(pid).Signal(0)`. It does **not** verify the `pid_start_time` (or `StartedAt` in `PointerRow`) against `/proc/<pid>/stat`. This allows a recycled PID to be mistaken for the original supervised process, enabling a "lost" process to be treated as "running" if the PID is reused by an unrelated process. This violates the process-identity guarantees required by RFC 0026 and RFC 0031.

### F2: Apply-Receipt Forgery Surface (High)
The Go apply service (`go/pkg/apply/service.go:41`) implements `VerifyReceipt` by simply loading the receipt from the database and returning `valid: true` if found. It does not perform any cryptographic signature verification against the daemon's signing key. An attacker with write access to the PostgreSQL `apply_receipts` table could forge receipts that would be accepted as valid by any client relying on the Go daemon's verification endpoint.

### F3: Insecure Scratch Directory Permissions (Medium)
The supervisor's `ensureFIFO` helper (`go/pkg/supervisor/pty.go:94`) creates supervisor-specific scratch directories with `0755` permissions. This permits other local users to list the directory and read the `pid` file (`0644`). While the parent `.striatum/` directory is typically protected, the explicit use of `0755` for transient operational state is a regression in the principle of least privilege and increases the visibility of supervisor IDs to local attackers.

### F4: Incomplete Envelope Handshake (Medium)
The `daemon.hello` handler (`go/pkg/rpc/server.go:175`) performs version checks on `supported_envelope` and `supported_framings`, but the `Server.handle` logic allows `daemon.hello` to be skipped if `requireHandshake` is false (though it is true in the default `Handle` path). More importantly, the version check is a "soft" check that returns `version_incompatible` but doesn't necessarily close the connection, and the client-side enforcement depends on the Python CLI's obedience to the response.

### F5: Implicit `daemon_core` flip via Env-Var Precedence (Low)
The CLI's core resolver (`src/striatum/cli/daemon.py:46`) prioritizes `STRIATUM_DAEMON_CORE` over any default. While standard for the project, an adversarial operator or script could set this environment variable to silently force a target repository to use the less-mature Go core without explicit user opt-in via the `--core` flag, bypassing the intended Phase 2 safety defaults.

### F6: CI Matrix Bypass Risk (Low)
The Go-specific tests (`tests/test_daemon_go_supervisor.py:38`) use `pytest.mark.skipif` based on the presence of the Go binary. If the `make daemon-go-build` step in CI fails or is tampered with such that the binary is missing, the Go-core matrix job will skip all tests and pass, potentially masking a build failure as a successful skip. The `STRIATUM_MULTI_REPO_REQUIRE_PG` sentinel protects against missing Postgres, but there is no equivalent hard-fail for a missing Go binary when `CORE=go`.

### F7: Skeleton Implementation of Sealed Apply (Advisory)
The `ReviewedPatch` mutation in `go/pkg/apply/service.go:27` is a skeleton that always returns `apply_gate_unsatisfied`. While consistent with the Phase 2 "foundation-only" scope, it means the primary security boundary for the "Apply" capability is currently unimplemented in the Go core, forcing users to stay on the Python core for real sealed-apply enforcement.

## Required Checks (Adversarial)

- [x] **--core flag escape paths:** Verified. Resolution in `src/striatum/cli/daemon.py` correctly handles flag vs env var, but lacks config-file support for the core itself.
- [x] **Mutation without registered Go method:** Verified. `go/pkg/rpc/registry.go` returns `method_unknown`, which results in a hard refusal in `Server.handle`. No silent fallback to Python daemon is implemented in the CLI client.
- [x] **Apply receipt forgery:** Identified as F2. The Go service currently lacks cryptographic verification.
- [x] **Supervisor packet-injection:** Identified as F3 risk. While using anonymous pipes mitigates some risk, the directory permissions are overly permissive.
- [x] **PID-recycling races:** Identified as F1. The Go liveness check is currently race-prone.
- [x] **Package-data binary substitution:** Checked. Resolution order in `resolve_go_binary` prioritizes packaged `_daemongo`, then `STRIATUMD_GO_BIN`, then in-tree. Attack surface is local-user only.
- [x] **CI matrix passing with Go binary missing:** Identified as F6. Skip logic in tests needs a hard-fail sentinel for the binary.
- [x] **daemon_core flip implicitly:** Identified as F5. Env-var precedence is the primary vector.
- [x] **Cross-core compatibility:** Checked. Handshake in `Server.buildWelcome` enforces envelope versioning, but verification is one-way (daemon refuses client).
