---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["adversarial", "security", "threat-model", "audit", "postgres", "injection"]
---
author: reviewer-gemini-pro-003

# Adversarial Review: Go Daemon Build (Track A)

This review evaluates the Go daemon implementation (Steps 1 and 2) from an adversarial threat-modeling perspective, focusing on trust boundaries, supply-chain hygiene, and potential exploit vectors.

## 1. Trust Boundaries & Attack Surface

The Go daemon (`striatumd-go`) introduces several critical trust boundaries:
- **Client <-> Daemon (Unix Socket):** The primary RPC interface. Attacks include schema bypass, request smuggling (if framing is weak), and privilege escalation.
- **Daemon <-> PostgreSQL (TCP/Unix Socket):** The persistence layer. Attacks include SQL injection and credential leakage.
- **Daemon <-> Shell (`psql`):** The daemon currently shells out to `psql` for database operations, introducing command injection and environment variable leakage risks.

## 2. Supply-Chain & Build Hygiene

- **Go Module Verification:** The project uses Go 1.23. Currently, `go.sum` is empty, indicating zero external dependencies. This is excellent for supply-chain security at this stage but must be monitored as the project grows.
- **Cross-Platform Consistency:** The build process is a simple `go build`. Since there are no external C dependencies (CGO), the build is highly portable and consistent across Linux and macOS.

## 3. Adversarial Findings & Threat Modeling

### [HIGH] SQL Injection via `psql` Shell-out
The current implementation of `PsqlRunner` in `go/pkg/db/connection.go` uses `exec.CommandContext` to run `psql -c "SQL"`. While `quoteLiteral` is used in some places (e.g., `migrations.go`), the `AuditRecorder` in `go/pkg/db/audit.go` manually constructs complex SQL strings using `fmt.Sprintf` and custom quoting logic.

**Exploit Vector:** If an attacker can influence fields that are not properly sanitized (e.g., `client_id`, `repository_id`, or `method` if not strictly validated against the registry), they could potentially break out of the SQL string or inject additional commands. Even with `quoteLiteral`, manual SQL construction is brittle and error-prone compared to parameterized queries.

**Recommendation:** Transition to a native Go Postgres driver (like `pgx`) that supports parameterized queries (`$1`, `$2`, etc.) and avoid shelling out to `psql` for data operations.

### [MEDIUM] Environment Variable Injection & Credential Leakage
The daemon reads `STRIATUM_DAEMON_DB_URL` and also accepts `--postgres-url`.
- **Leakage:** The `PsqlRunner` passes the raw URL as an argument to `psql`. On some systems, process arguments are visible to other users via `ps`.
- **Injection:** If the URL contains malicious parameters (e.g., `sslverify=none` or custom service names), an attacker with control over the environment or CLI arguments could redirect the daemon to a malicious database.

**Recommendation:** Pass the URL to `psql` via the `PGPASSWORD` or `PGSERVICE` environment variables rather than as a CLI argument, or better yet, use a native driver. Ensure `RedactURL` is used consistently in all logs and metadata outputs.

### [MEDIUM] Audit Chain Tamper Resistance
The audit chain implementation uses `previous_hash` and `row_hash` to ensure integrity.
- **Vulnerability:** The `RecordRPC` method in `audit.go` performs two separate queries: one to get the `last_hash` and another to `INSERT` and `UPDATE` the head. Although it uses a CTE for the update, the initial `SELECT` is not part of the same transaction as the `INSERT`. This could allow race conditions or "forking" the audit chain if multiple daemons or threads are active.
- **Tamper Alert:** The `VerifyRows` function returns as soon as it finds a problem, which is good for detection, but the daemon lacks an automated "panic" or "shutdown" mode if the audit chain is found to be compromised at runtime.

**Recommendation:** Wrap the entire audit recording logic in a single transaction with `SERIALIZABLE` isolation or use a trigger-based approach to ensure the chain cannot be bifurcated.

### [LOW] Unix Socket Permissions
The daemon creates the socket with `os.MkdirAll(..., 0o700)` and `os.Chmod(path, 0o600)`.
- **Finding:** This is generally secure, limiting access to the user running the daemon. However, if the daemon is run as `root` (which it shouldn't be), the socket would be in a root-controlled directory.

**Recommendation:** Document the requirement to run `striatumd` as a non-privileged user and verify directory ownership in the `doctor` command.

## 4. Verdict Intent

The implementation is a solid foundation but requires moving away from `psql` shell-outs and manual SQL construction to reach a "production-ready" security posture. The adversarial surface is currently manageable due to the lack of external dependencies.

**Status: Acknowledged with Mitigations Recommended.**
