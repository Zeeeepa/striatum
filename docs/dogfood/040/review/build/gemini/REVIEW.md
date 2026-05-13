---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat-model", "rfc-0040", "mcp-harness", "systems-review"]
---

author: reviewer-gemini-pro-001

# Threat-Model Build Review: RFC 0040 V1 (MCP-Driven Dogfood Harness)

I have completed a threat-modeling review of the RFC 0040 V1 implementation, focusing on the operator-side composite tools, the daemon-side supervised-progress watcher, and the `workflow upgrade` CLI verb. 

## Trust Boundaries and Attack Surfaces

The implementation introduces two major new attack surfaces at the trust boundary between the operator session and the daemon:

1.  **Composite RPC Surface (`dogfood.*`):** These methods perform atomic multi-step state mutations. They are gated by the existing capability system but bypass the normal "one RPC per state transition" audit granularity.
2.  **`surgical_recovery` Capability:** A new admin-bound capability that allows re-activating stale leases and re-attaching to "lost" supervisors, bypassing standard recovery safety policies.

## Adversarial Case Evaluation

### 1. Operator calling `surgical_recovery` while supervisor is still alive
**Status:** Mitigated.
The implementation in `striatum.dogfood.operator_tools._validate_recoverable_shape` explicitly queries the `process_supervisors` table for any supervisor in `starting`, `attached`, or `detached` states for the session. If found, it refuses the recovery with `concurrent_supervisor`. It correctly allows recovery when the supervisor is in the `lost` state, which is the intended fallback for lease-expiry during active work. PID identity (start time) is validated during re-attachment to prevent hijacking of recycled PIDs.

### 2. Concurrent composite-tool calls
**Status:** Mitigated.
Concurrency is handled at two levels:
- **Application Level:** `dogfood.surgical_recovery` and the `SupervisedProgressWatcher` both utilize a per-job `progress_advisory_lock` (implemented via `fcntl.flock`). `surgical_recovery` holds this lock for the duration of its transaction, causing the watcher to skip heartbeat ticks if they overlap.
- **Database Level:** Both `publish_on_behalf` and `surgical_recovery` use SQLite `BEGIN IMMEDIATE` transactions (via `striatum.db.transaction`) to serialize state checks and mutations. `publish_on_behalf` handles the "already acked" state gracefully, allowing for safe retries if a previous call partially succeeded.

### 3. Supervised-progress watcher polling a non-existent log file
**Status:** Mitigated.
The `SupervisedProgressWatcher` in `striatum.daemon_supervisor.progress_watcher.py` uses `newest_log_mtime`, which catches `FileNotFoundError` and returns `None`. The calling `tick()` logic handles this as a `no_log` condition and skips the heartbeat rather than crashing.

### 4. Workflow upgrade applied to a workflow already mid-run
**Status:** Mitigated.
The `striatum workflow upgrade` CLI verb (implemented in `striatum.cli.workflow.py`) performs a pre-mutation check using `_running_runs_for_workflow`. It queries the database for any non-terminal runs linked to the workflow file's snapshot history. If active runs are found, the command raises a `WorkflowError` and refuses to mutate the file (unless `--dry-run` is passed).

### 5. Capability token leakage across composite tool boundaries
**Status:** Mitigated.
- **Isolation:** Composite tools are implemented as internal Python functions that operate directly on the database connection. They do not invoke other RPC methods or pass capability tokens internally, eliminating the risk of token leakage between sub-operations.
- **Authorization:** The `METHOD_REGISTRY` correctly binds `dogfood.publish_on_behalf` to `write` and `dogfood.surgical_recovery` to the new `surgical_recovery` capability. The `DaemonRpcRouter` enforces these gates before dispatching to the composite handlers.
- **Audit:** Composite operations record a single `dogfood.*` event in the audit chain containing the operator's `reason`, while the underlying primitives (ack, publish, complete) continue to record their own discrete events, ensuring full traceability.

## Verdict: Accept

The implementation adheres to the defensive requirements of RFC 0040. Trust boundaries are well-defined, and the identified adversarial race conditions and error states are handled through a combination of advisory locks, serializing transactions, and strict state validation.

The `surgical_recovery` capability is correctly isolated as an admin-bound privilege. While the 15-minute TTL ceiling mentioned in the design docs was not found in the authorization logic (as it is an issuance-time constraint), the overall posture is sufficient for V1.
