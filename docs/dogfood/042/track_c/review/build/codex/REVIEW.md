---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0042", "repo-local-pg", "build", "track_c"]
---

author: reviewer-codex-gpt-5.5-002

# Track C Build Review: RFC 0042

Verdict: needs_revision

## Trust Boundaries Reviewed

RFC 0042 introduces four load-bearing boundaries: the daemon becomes the only workflow-state authority; system Postgres becomes the live substrate for formerly repo-local workflow state; `.striatum/` becomes scratch plus optional tombstone material; and the one-shot migration path becomes the only code path allowed to open source SQLite. The attack surfaces are source-state tamper during migration, partial import or broken foreign-key promotion, daemon audit-chain continuity, repo-scoped capability confusion, and fallback paths that keep direct SQLite mutation alive.

The RFC is directionally aligned with D082, D086, D087, D088, D093, and D083. It explicitly preserves the single-user trust boundary, keeps transcripts and free-text workflow prose out of daemon audit rows, defines `.striatum/` as scratch, removes silent direct-SQLite fallback for state-touching verbs, and requires RFC 0039 to treat the Go daemon as the gateway for all workflow operations. Those parts are concrete enough for a build job.

## Findings

### F1 - Source SQLite is both read-only and mutated during cutover

Severity: high

RFC 0042 says the CLI never opens SQLite and that the daemon opens the source SQLite database read-only for `repo.migrate_local_state` (lines 212-215). The required behavior repeats that SQLite is opened read-only (lines 225-227). Later, the same required behavior mandates writing a cutover sentinel into the source SQLite `schema_meta` (lines 245-247), then renaming/chmodding or deleting the source file (lines 248-250). Section 5 also says the only allowed SQLite opening in V1 is the daemon-owned read-only source scan (lines 330-334).

That is not implementable as written. A read-only connection cannot write `schema_meta`, and a strictly read-only source-scan boundary cannot also mutate or delete the source file. This matters under the threat-model posture because the cutover sentinel is the drift-detection anchor for `pg_repo_drift_detected`; if implementers must infer an unwritten writable phase, different implementations can produce different tamper windows and audit evidence.

Required fix: split the migration into explicit phases. For example, define a read-only preflight/import scan that computes source hashes and imports rows, then an exclusive post-commit source-finalization phase that either writes a sentinel through a writable locked connection before tombstoning or avoids mutating SQLite entirely and records the tombstone filename/hash as the marker. Also update the "only allowed SQLite opening" sentence so it matches the chosen model.

### F2 - The specified import order violates existing foreign keys

Severity: high

RFC 0042 says real import uses staging tables "in foreign-key order" (lines 233-236), but the required import order places `work_packets` before `queue_messages` and `leases` (lines 237-242). The current SQLite schema defines `work_packets.message_id` as a foreign key to `queue_messages(message_id)` and `work_packets.lease_id` as a foreign key to `leases(lease_id)` in `src/striatum/schema.py` lines 151-161. Promoting rows in the RFC's order will either fail composite foreign-key validation or force implementers to disable/defer constraints in a way the RFC does not specify.

This is a schema-integrity blocker, not a polish issue. The acceptance criteria require preserved identifiers, preserved event ordering, and validated composite foreign keys, but the normative order cannot satisfy that on existing data containing claimed or acked work packets.

Required fix: make the order match the dependency graph. At minimum, import `queue_messages` and `leases` before `work_packets`. The RFC should also say whether staging tables have immediate constraints, deferred constraints, or no constraints plus explicit post-load validation, because that choice affects partial-import recovery and replay safety.

### F3 - Audit hash payload version conflicts with existing v2 rows

Severity: high

RFC 0042 says adding `repo_local_state_touched` bumps the audit hash payload format from version 1 to version 2 and includes that boolean in canonical hashes (lines 176-186). Current daemon Postgres audit rows already use `hash_format_version: 2` in `src/striatum/daemon_rpc/request_log.py` lines 95-112, and the current `v2_row_hash()` material in `src/striatum/daemon_pg/audit.py` lines 30-48 does not include `repo_local_state_touched`.

That means RFC 0042 cannot safely redefine v2 in place without breaking verification semantics for already-written v2 audit rows. Under the audit-chain threat model, this is a compatibility bug: two valid rows with `hash_format_version = 2` would have different canonical payload definitions depending on whether they were written before or after RFC 0042.

Required fix: introduce `hash_format_version = 3` for rows that include `repo_local_state_touched`, or specify an explicit compatibility rule keyed by schema version and column presence. The doctor/audit verifier acceptance criteria should require both existing v2 rows and new migration rows to verify across the v2-to-v3 boundary.

## Required Checks

RFC 0042's acceptance criteria are mostly concrete enough for a future dogfood once F1 through F3 are corrected. The D006/D007/D028 supersession statement is explicit and names D093 as the consolidation decision. The `.striatum/` scratch boundary is clear. The migration verb includes `--dry-run` and `--keep-sqlite-readonly`. RFC 0039 is revised so the Go daemon must gateway all former repo-local operations from day one. D083's single-user, single-machine trust boundary remains unchanged.

## Recommended Disposition

Return RFC 0042 for revision before build. After the read-only/sentinel contradiction, import-order FK hazard, and audit hash-version conflict are fixed, the RFC should be acceptable from the Track C systems threat-model angle.
