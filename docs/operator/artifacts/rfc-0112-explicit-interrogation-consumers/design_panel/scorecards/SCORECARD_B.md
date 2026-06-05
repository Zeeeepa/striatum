# Scorecard B — Scorecard for Proposal B (Option B)

---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: accept_with_findings
severity: info
tags: [scorecard, rfc-0112, option_b]
author: scorekeeper-gemini-3.5-flash-high-001
---

## 1. Executive Summary & Verdict

**Verdict**: `accept_with_findings`

We score Option B (Proposal B) as the preferred structural design for explicit interrogation consumers. By prioritizing a **lifecycle-first, schema-less derivation model**, Option B successfully addresses the two-sided failure (early window closure and leaked sessions) without introducing database migrations, table schema drift, or duplicate sources of truth. 

The implementation details are exceptionally clean, and the proposed `markJobTerminal` choke point and AST guard test provide a robust defense-in-depth against future leak regressions.

---

## 2. Structured Dimensions Evaluation

### Maintainability: 9.5 / 10
* **Finding**: The decision to keep the consumer targets declarative and derive the liveness state dynamically via the snapshot's `workflow_json` is a major win. It completely bypasses the need for database migrations, avoiding owner-migration socket authorization bottlenecks.
* **Minor Finding**: The AST guard test is a creative check, but it adds code-level complexity that must be maintained as Go compiler/AST versions upgrade. However, the recovery sweep acts as a robust convergence backstop if the AST guard fails.

### Migration Risk: 10 / 10
* **Finding**: Zero database-level migrations. In-flight runs and old snapshots fail safe (vacuously empty matching) with no performance degradation, keeping simple review panels backward-compatible.

### Reversibility: 10 / 10
* **Finding**: Since no database tables are modified, reverting this implementation requires only a codebase commit rollback.

### Operator Legibility: 9 / 10
* **Finding**: Surfacing target states (`available`, `unavailable`, `not_ready`) inside the work-packet projection is clear, highly diagnostic, and consistent with target liveness signals.

### Lifecycle Correctness: 10 / 10
* **Finding**: Swapping out direct database updates for a central `markJobTerminal` choke point ensures that *all* terminal transitions (including cancel/failure/manual overrides) trigger the release hook. This directly addresses the leaked session problem for build-class consumers (like cross-examiners).

### Fixture Rigor: 9.5 / 10
* **Finding**: The proposed test cells under `go/pkg/adapterconformance/` cover the happy path, revision reopen cascades, and recovery dead-lane sweep. Using the production generator output instead of a hand-built mock graph ensures genuineness.

---

## 3. Completeness & Answering Open Questions

Proposal B answers all six panel questions completely and cleanly:
1. **Field Shape**: Confirms `{workflow_job_id, required}` array on the consumer job.
2. **`required` Semantics**: Correctly identifies that a hard V1 gate on `work.complete` would introduce deadlock risks under revision reopen cascades; advisory packet-instruction with transitional event logging (`required_skipped`) is the correct V1 path.
3. **Multiple Targets**: Plurality is correctly supported with set-based hooks.
4. **Terminal Paths**: Plural mutation paths are inventoried and covered under a single choke point.
5. **Fixture Plan**: Three clear conformance fixtures are designed under `go/pkg/adapterconformance/`.
6. **Work-Packet Projection**: Defines target states in the packet projection namespace.

---

## 4. Minor Corrective Findings for Implementation

1. **AST Guard Robustness**: The AST static analyzer must be designed to parse raw Go code simply, ensuring it does not false-positive on unrelated SELECT/UPDATE statements or queries against different databases/schemas.
2. **Read Performance**: Querying the snapshots table for every terminal transition should be profiled to ensure that PG JSONB lookup scales appropriately for runs with deep revision histories.
