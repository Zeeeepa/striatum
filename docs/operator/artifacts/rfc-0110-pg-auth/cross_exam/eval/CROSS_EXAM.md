# RFC 0110 Evaluation Cross-Exam
author: cross-examiner-gemini-3.5-flash-high-004
artifact_kind: handoff
logical_name: cross_examiner_4
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
posture: eval

## Interrogation

Target attempted: `sess_a6beb21cc70189786cf7c45e63619068`

Challenge I attempted to put to the candidate owner:

> The test harness (`pgtest.Pools`) executes ad-hoc `GRANT` and `SET ROLE` statements in Go to configure the unprivileged test role, overriding the schema privileges applied by migrations. How can we ensure that privilege tests like `T-42501` are not testing a polluted harness-specific state rather than the actual migrations-enforced production privileges? If pgtest modifies schema privileges in-Go, any migration drift will go undetected by the test suite.

Structured turn reference: `interrogation.open` returned
`status: interrogation_unavailable`, `reason: panel_window_closed`,
`target_session_id: sess_a6beb21cc70189786cf7c45e63619068`,
`interrogable_job_id: job_run_8e14cb48342e929d30043d6be24f9101_convener_draft`, audit id `6633220`.
No interrogation id was created, so no `interrogation.ask` or target rebuttal was
possible.

Rebuttal reference: none. The unanswered interrogation is evidence for EV-001.

## findings[]

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape required |
| --- | --- | --- | --- | --- | --- |
| EV-001 | high | Harness fidelity: tests run under the true production security model. | The test harness (`pgtest.Pools`) runs ad-hoc `GRANT` and `SET ROLE` statements in Go code to configure the unprivileged role, overriding schema privileges applied by migrations. This pollutes the test environment and prevents the test suite from validating that the migrations themselves correctly enforce privileges (such as revoking `INSERT`). | Remove ad-hoc privilege modifications from Go test helpers. Database roles and privileges must be created and managed strictly by owner-applied migrations, and `pgtest` must only connect using those predefined roles to verify the security model is correctly deployed by SQL schema files. | `C-HARNESS-PRIVILEGES`: `pgtest` connections use roles whose permissions are defined solely by schema migrations; Go test code is prohibited from calling `GRANT`/`REVOKE` to patch permissions during run. |
| EV-002 | high | L2 isolation prevents untrusted lane access to PostgreSQL. | The candidate includes positive tests for L2 isolation (doctor blocks under flag) but has no negative tests asserting that a process in the lane's sandbox is actually blocked from PG. Without negative validation, configuration drift in `pg_hba.conf` or UNIX socket directory permissions will go undetected. | Add a negative test `T-LANE-ISOLATION-NEG` where a mock lane process attempts to connect to PostgreSQL over UNIX socket and loopback TCP, asserting that all attempts fail with permission/connection errors. | `C-L2-NEG-TEST`: the test suite runs a non-privileged runner process that attempts out-of-band DB access, proving it is blocked under the hardened default. |
| EV-003 | medium | `VerifyRows` correctly validates mixed v2-to-v3 and tampered chains. | Verifying the transition from v2 to v3 row hashes is a load-bearing requirement, but there is no mechanism in the harness to write malformed or mixed-version rows for validation. Modifying production code to allow test-only bypasses degrades design integrity. | Equip the `pgtest` harness with a privileged backdoor utility to write raw rows directly to `audit_log` (bypassing the `SECURITY DEFINER` functions) to construct mixed and tampered chains for the verifier. | `C-TEST-ROW-WRITER`: a test-only DB utility can write arbitrary invalid/tampered rows to verify `VerifyRows`' mixed-format and tamper-detection coverage. |
| EV-004 | high | L3 attribution GUCs are reliably reset on connection reuse. | Attribution labels (`rpc_id`/`principal_id`) must be reset on pool release. However, if a transaction panics, aborts, or times out, the `pgx` `AfterRelease` hook might be bypassed or leave GUCs in a dirty state, leading to cross-transaction info leaks. | Write a test that checks out a connection, sets attribution GUCs, simulates a transaction failure/panic, and then verifies that a subsequent checkout of the same connection has GUCs reset to defaults. | `C-ATTR-RESET-FAIL`: `T-ATTR-RESET` must prove attribution labels are cleared even after transaction aborts, query cancellations, and driver panics. |
| EV-005 | medium | V3 timestamp formatting is locale- and timezone-independent. | The DB-computed timestamp relies on `to_char` to match Go's RFC3339 format. PostgreSQL's `to_char` behavior for timestamps can vary based on server timezone configuration (`TimeZone` GUC), risking hash mismatch in remote-PG or skewed host setups. | Run `T-TS` tests under multiple active PostgreSQL timezone configurations (e.g. `UTC`, `EST`, `Asia/Kolkata`, `Australia/Lord_Howe`) and assert that in-DB hashing produces identical output byte-for-byte. | `C-TZ-INDEPENDENT-HASH`: `T-TS` runs hashing checks against diverse DB `TimeZone` settings to prove formatting and hashing are timezone-independent. |
| EV-006 | high | VerifyRows verifier passes for all historical and newly appended rows. | Re-implementing the audit chain hash in PL/pgSQL introduces a parity gap: Go's V2RowHash relies on sorted alphabetical JSON serialization, which is fragile to recreate in SQL. The test suite needs to verify that the PL/pgSQL hashing logic and Go's hashing logic produce identical results. | Introduce a dedicated test (T-HASH-PARITY) that asserts byte-for-byte identity between Go's V3RowHash and the DB's hashing function across a comprehensive array of edge cases (UTF-8, HTML characters, nulls). | `C-HASH-PARITY-TEST`: the test suite asserts byte-for-byte hash identity between Go and PostgreSQL implementations. |

## Evaluation posture summary

The evaluation posture focuses on validating that security boundaries are not only theoretically sound but dynamically tested and free from harness-induced false positives. The current test harness `pgtest` has a critical gap: it constructs the unprivileged security model imperatively in Go rather than verifying the declarative migrations. Addressing this privilege pollution (EV-001) alongside negative validation of the L2 boundary (EV-002), robustness checks for connection pooling (EV-004), and rigorous cross-implementation hash parity testing (EV-006) is required to make the RFC 0110 implementation verifiable.
