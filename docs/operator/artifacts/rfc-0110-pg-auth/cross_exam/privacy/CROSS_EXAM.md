# RFC 0110 Privacy Cross-Exam
author: cross-examiner-gemini-3.5-flash-high-004
artifact_kind: handoff
logical_name: cross_examiner_3
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
posture: privacy

## Interrogation

Target attempted: `sess_a6beb21cc70189786cf7c45e63619068`

Challenge I attempted to put to the candidate owner:

> The proposed `daemon_auth_log` table captures arbitrary JSONB `detail` payloads on rotation and bootstrap events. How do we guarantee that connection strings, raw driver configuration structs, or credentials from failed DSNs do not bleed into these persistent logs, violating the RAM-only and credential secrecy guarantees of L0?

Structured turn reference: `interrogation.open` returned
`status: interrogation_unavailable`, `reason: panel_window_closed`,
`target_session_id: sess_a6beb21cc70189786cf7c45e63619068`,
`interrogable_job_id: job_run_8e14cb48342e929d30043d6be24f9101_convener_draft`, audit id `6634722`.
No interrogation id was created, so no `interrogation.ask` or target rebuttal was
possible.

Rebuttal reference: none. The unanswered interrogation is evidence for PR-002.

## findings[]

| id | severity | affected invariant | finding | closest acceptable answer | constraint shape required |
| --- | --- | --- | --- | --- | --- |
| PR-001 | high | L0 credentials remain strictly ephemeral and RAM-only in-memory. | The candidate stores the rotated database password in the static `pgxpool.Config` structure for reconnection, leaving plaintext credentials resident in the Go heap indefinitely. This increases the threat surface of process core-dumps or heap-inspection attacks. | Configure the database pool to retrieve the rotated password dynamically using a password callback or connection hook (e.g. `BeforeConnect`), and zero out transient credential strings immediately after connection establishment. | `C-DYNAMIC-CREDENTIALS`: the database connection pool utilizes dynamic password providers to minimize the lifetime of plaintext passwords in heap memory. |
| PR-002 | high | `daemon_auth_log` contains no secret credentials or sensitive parameters. | The proposed JSONB `detail` field in `daemon_auth_log` is unconstrained, introducing the risk that raw driver connection errors, unredacted DSNs, or failed connection parameters will be logged during bootstrap or rotation failures. | Define a strict serialization whitelist for `detail` JSONB keys. Explicitly sanitize and redact any error messages containing passwords, tokens, or DSN patterns prior to database insertion. | `C-AUTH-LOG-PRIVACY`: logs written to `daemon_auth_log` undergo strict DSN and credential redaction, validated by error-path serialization tests. |
| PR-003 | medium | L3 attribution labels are not visible to unauthorized local database sessions. | Setting custom GUCs using string-concatenated SQL queries (e.g. `SET LOCAL striatum.principal_id = '...'`) exposes the actual parameter values in `pg_stat_activity` query text, making session IDs and principal identifiers visible to other local database sessions. | Set GUC values exclusively using parameterized pgx queries or utility functions (e.g. `SELECT set_config('striatum.principal_id', $1, true)`) so the actual values are masked in system views. | `C-GUC-PARAMETERIZED`: custom GUC variables are configured using parameterized statements to prevent parameter leakage in database statistics. |
| PR-004 | medium | Sandboxed lane users cannot connect out-of-band to PostgreSQL. | Relocating UNIX sockets to a `0700` directory restricts connection to the daemon identity, but the daemon does not verify parent directory permissions at startup. Any manual override or drift in socket folder permissions allows local user bypass. | Enforce directory permission checks at daemon startup, refusing to bind the PostgreSQL client socket if the socket directory permissions are more permissive than `0700`. | `C-SOCKET-DIR-PERMS`: daemon initialization asserts socket folder permissions are exactly `0700` and reports any relaxed configurations as doctor violations. |
| PR-005 | high | Curated database records exclude raw provider transcripts (D028 compliance). | The `events` table stores arbitrary `payload_json` for auditing. Without database-level validation, LLM transcripts or terminal stdout/stderr could bleed into durable database columns via custom events. | Apply database-level constraints or validation rules to `payload_json` in the `events` table to ensure raw provider outputs and transcript-related keys are rejected. | `C-EVENT-NO-TRANSCRIPTS`: trigger-based or schema constraints verify that event payloads contain only curated metadata, rejecting stdout/stderr streams. |
| PR-006 | high | VerifyRows verifier passes for all historical and newly appended rows. | The audit chain hash is computed in Go using sorted alphabetical JSON keys. Re-implementing this JSON hashing in PL/pgSQL risks formatting or serialization mismatches, breaking `VerifyRows` validation. | Retain Go's `V2RowHash` only for reading/verifying historical rows. Introduce a length-prefixed binary format (v3) for all new PL/pgSQL-written rows, implemented identically in Go and PL/pgSQL. | `C-HASH-V3-TRANSITION`: PL/pgSQL hashing uses a canonical binary format (v3) to eliminate JSON encoding discrepancies. |

## Privacy posture summary

The privacy posture challenges the candidate synthesis to ensure that ephemeral secrets and private session identifiers do not escape their RAM-only bounds, and that the transition of hash computation from Go to PL/pgSQL does not break historic or future verification (PR-006). The load-bearing concerns lie in preventing credential leakage to the Go heap (PR-001) and ensuring unredacted connection parameters do not bleed into the `daemon_auth_log` (PR-002). Parameterizing custom GUCs (PR-003) and enforcing socket directory constraints (PR-004) are required to satisfy the D028/D151 boundaries.
