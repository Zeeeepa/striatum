# RFC 0110 — Candidate synthesis (cycle 1): implementation-ready spec

author: convener-claude-opus-4.8-001
artifact_kind: handoff
logical_name: convener_candidate_cycle_1
workflow: rfc-0110-pg-auth-panel
run_id: run_8e14cb48342e929d30043d6be24f9101
cycle: 1
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/scan/SURVEY.md
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/synthesis/SURVEY_SYNTHESIS.md
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md

This is the candidate synthesis for RFC 0110. It takes firm positions on every
open question from the survey/framing (Q1–Q9) and sequences L0–L3 into an
implementation-ready spec. It is offered for cross-examination; each load-bearing
decision states the position, the reasoning, and the falsifiable test that proves
it. Decision criteria are in `../survey/synthesis/SURVEY_SYNTHESIS.md` §4.

---

## 0. The one decision that gates everything: L1 hash (Q1/Q2/Q3)

**Position — Option A, realized via a new `hash_format_version = 3`
canonicalization.** The audit `row_hash` is computed **inside** the owner-owned
`SECURITY DEFINER` function, not supplied by the caller; and the canonical input
for new rows is a **length-prefixed field encoding** designed to be byte-identical
in Go and PL/pgSQL without reproducing `encoding/json`'s key ordering or
HTML-escaping. `VerifyRows` dispatches per row on `hash_format_version`, so
existing v2 (Go-JSON) rows keep verifying unchanged.

### 0.1 Why Option A over Option B (trusted caller-supplied hash)

Both roles can `EXECUTE` the function, so the function — not the call site — is
the trust boundary. If the function trusts a caller-supplied `row_hash`
(Option B), a process holding the runtime DSN can pass a **deliberately wrong**
hash; the function links it into the chain, and `VerifyRows` (`go/pkg/db/audit.go:213`)
later reports the chain as *broken* — a denial-of-service on the integrity check,
and exactly the "tamper with the audit chain" outcome L1 exists to prevent. The
only way Option B avoids this is to **recompute and validate** the supplied hash
in-DB — which *is* Option A. So Option A is the only option that actually closes
the threat; Option B is recorded rejected (§7).

### 0.2 Why a new `hash_format_version = 3` instead of porting `encoding/json`

The current v2 material (`V2RowHash`, `audit.go:25`) is
`sha256(json.Marshal(map[string]any{...}))`. Reproducing that in PL/pgSQL means
matching Go's `encoding/json` **forever**: ascending-codepoint key order, compact
spacing, `null` for nil, unquoted integers, and **HTML escaping** of `<` `>` `&`
(and U+2028/U+2029). Those characters are unlikely in the 15 fields today, but
"unlikely" is a latent chain-break (C-HASH): one `&` in a `daemon_version` or
`denial_reason`, years from now, silently breaks `VerifyRows` for every row after
it. We refuse to stake the audit chain on that.

`hash_format_version` already exists as a first-class column (`audit_log.hash_format_version`,
`0001_baseline.sql:87`) and is part of the hashed material, so the format is
*designed* to be versioned. v3 introduces a canonical form with no escaping and
no ordering ambiguity (§0.3), trivially identical in both languages.

### 0.3 The v3 canonical form (normative)

For the **same 15 fields** as v2, in this **fixed declared order** (not
alphabetical — order is pinned by the spec, not by a language's map iteration):

```
ts, schema_version, hash_format_version, daemon_version, client_id,
repository_id, method, decision, denial_reason, transport, request_id,
exit_code, params_sha256, previous_hash, segment_id
```

Encode each field as `Enc(f)` and concatenate in order; `row_hash = lower_hex(sha256(concat))`:

- **string `s`:** `octet_length(s)` in decimal ASCII, then `:`, then the raw
  UTF-8 bytes of `s`. (Length-prefixing makes the encoding injective — no
  delimiter can be forged by field content.)
- **integer `n`** (`schema_version`, `hash_format_version`, `exit_code`,
  `segment_id`): decimal ASCII of `n`, then `:`, then the same decimal ASCII
  bytes (encode as the string form of the integer, length-prefixed like a string).
- **null** (`client_id`/`repository_id`/`denial_reason`/`previous_hash`/`exit_code`
  when absent): the 3 literal bytes `-1:` and nothing else. `-1` is unreachable
  by `octet_length`, so null is unambiguously distinct from any value including
  the empty string (`0:`).
- **`ts`:** the RFC3339 second-truncated UTC **string**
  (`2026-06-03T15:00:00Z`), length-prefixed as a string. The function derives it
  from the `timestamptz` via
  `to_char(ts AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`, which equals
  Go's `time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)` (`audit.go:140`).
  **Q2 resolved.**

Go gets a `V3RowHash(row)` that builds the identical byte string with
`strconv`/`len([]byte(...))`; PL/pgSQL builds it with `octet_length(convert_to(v,'UTF8'))`
+ `||`. A golden cross-check test (§0.5) pins them equal.

### 0.4 Function scope (Q3 resolved)

The owner-owned `SECURITY DEFINER` function — call it
`striatumd.append_audit_row(...)` — owns the **entire** append, not just the
INSERT: it takes the row *content* as typed parameters (method, decision,
denial_reason, transport, request_id, exit_code, params_sha256, client_id,
repository_id, daemon_version), then inside one transaction it

1. `SELECT last_hash FROM audit_chain_head WHERE singleton FOR UPDATE` (the
   serializing lock, today at `audit.go:96`),
2. resolves/opens the audit segment (today `audit.go:103-127`),
3. computes `row_hash` via the v3 form over content + `previous_hash` + segment,
4. `INSERT`s the row, and
5. `UPDATE`s `audit_chain_head`.

`AuditRecorder.RecordRPCTransport` (`audit.go:64`) becomes a thin caller: it
passes content and receives `audit_id`. The lock + segment + chain logic move
**into** the function so the invariant lives in the database. `striatumd_rw`
keeps only `EXECUTE` on the function; direct `INSERT` is revoked (§1).

### 0.5 Falsifiable tests for §0 (this is what the cross-examiner should attack)

- **T-HASH-PARITY:** a Go test feeds a fixed set of representative rows
  (including values containing `<`, `>`, `&`, empty strings, and nulls) to both
  `V3RowHash` (Go) and the PL/pgSQL function's hash expression (run against a real
  PG), asserting equal hex. Fails if either side diverges on any byte.
- **T-VERIFY-MIXED:** seed a chain with v2 rows then v3 rows across a segment
  boundary; `VerifyRows` (dispatching per `hash_format_version`) returns no
  problems; mutate one v3 row's content → it flags `daemon_pg_audit_row_hash`.
- **T-TS:** assert `to_char(... 'YYYY-MM-DD"T"HH24:MI:SS"Z"')` == Go RFC3339 for
  a battery of truncated UTC instants (incl. midnight, leap-second-adjacent).

---

## 1. L1 — PostgreSQL enforces the write contract

**Phase 0 — `audit_log` (highest value, single call site).**

- Owner migration (RFC 0079 §5, applied `--as-owner`): create
  `striatumd.append_audit_row(...)` `SECURITY DEFINER` owned by the owner role
  (§0.4); `REVOKE INSERT ON striatumd.audit_log FROM striatumd_rw`;
  `GRANT EXECUTE ON FUNCTION striatumd.append_audit_row(...) TO striatumd_rw`.
  Concrete starting fact: `striatumd_rw` holds direct INSERT **today** via the
  `0005_repo_local_workflow_state.sql:470` `GRANT ALL`; append-only is only
  trigger-enforced for UPDATE/DELETE (`0001_baseline.sql:158-166`). Phase 0
  removes the INSERT and routes through the function. **C-INSERT-REVOKE.**
- New rows are written with `hash_format_version = 3` (§0.3). The cutover opens a
  **new audit segment** (`audit_segments`) so the v2→v3 transition aligns to a
  segment boundary; `previous_hash` still links across the boundary (a row's
  `previous_hash` is the prior row's `row_hash` regardless of format), and
  `VerifyRows` dispatches per-row.
- **CI gate (Q9 resolved):** the `pgtest` harness already runs mutations as a
  per-test `striatumd_rw_<db>` role. Add **T-42501**: after migrations, a direct
  `INSERT INTO striatumd.audit_log(...)` as that role must fail with SQLSTATE
  `42501` (insufficient_privilege); the positive path (`append_audit_row`)
  succeeds and `VerifyRows` stays green. Runs on every migration-forward.

**Phase 1 — `artifacts`.** Move artifact writes behind a `SECURITY DEFINER`
function enforcing attempt-scope immutability in-DB (today enforced in Go +
`0018_artifact_attempt_scope.sql`); `REVOKE INSERT` (UPDATE/DELETE already
revoked, `0005:472`, except the narrow blob grant `0010`); keep that blob grant
or fold it into the function. Provable-green before Phase 2.

**Phase 2 — `events`.** Same treatment; `REVOKE INSERT` (UPDATE/DELETE already
revoked, `0005:471`).

**RLS — second tier (Q7 resolved).** After the function phases, enable
Row-Level Security on the per-session tables `leases` and `sessions`, keyed on
`current_setting('app.session_id', true)` set per transaction (the missing-key
form returns NULL rather than erroring). Functions own the *write path*; RLS
limits *which rows* a valid call may touch. It composes with L3 (§3): both use
`SET LOCAL`, and the pool reset (§3) clears them across checkouts. RLS is
**last** in the L1 sequence — it is defense-in-depth, not the primary gate.

Each phase is independently shippable and provable-green before the next (DC2).

---

## 2. L0 — Credential: ephemeral, owner-bootstrapped, RAM-only

At startup `striatumd`:

1. Opens a short-lived **owner** connection. Default: the existing owner-PEER
   unix-socket path (same as `--as-owner` migrations). Remote-PG (Q4): if
   `STRIATUM_OWNER_DB_URL` is set, use it; the recommended at-rest form for that
   secret is a **systemd encrypted credential** (`LoadCredentialEncrypted=` +
   `sd_get_credentials()`, TPM/host-key encrypted, never in `/proc/<pid>/environ`),
   with a plaintext file as the documented-but-discouraged floor.
2. Runs `ALTER ROLE striatumd_rw PASSWORD '<crypto/rand>'`, closes the owner
   connection.
3. Opens the runtime `pgxpool` as `striatumd_rw` with that password.

- **Single-role dev guard (Q4):** if the resolved owner role *is* the runtime
  role, skip rotation and emit a `WARN` (`db_credential_rotation_skipped_single_role`)
  rather than rotating the password out from under the bootstrapping connection.
- **In-memory hygiene — honest scope (Q5):** the password lives only in process
  memory; it is never written to disk, env, or `daemon.toml`. We zero the
  **transient** `crypto/rand` string after `pgxpool.ParseConfig` consumes it, but
  **pgx retains a copy in `pgxpool.Config` for reconnects** for the pool's
  lifetime — we do **not** claim to zero that, because the pool must reconnect on
  PG restart. The security property is **RAM-only + rotation-on-restart**, not
  zeroization: every `systemctl --user restart striatumd` re-rotates, so a
  password scraped from a prior run's core dump/swap is already dead. (The
  cross-examiner should test this claim; we are deliberately not over-promising.)
- **Doctor posture probe (cheapest, lands first):** `daemon doctor` gains
  `db-credential-posture`, asserting a password is set
  (`SELECT rolpassword IS NOT NULL FROM pg_authid WHERE rolname='striatumd_rw'`)
  **without ever reading or logging the value**; the daemon emits one structured
  `db_credential_rotated` line on success. **DC6.**

---

## 3. L3 — Attribution: every mutation names its RPC and principal

Add a `pgxpool.BeforeAcquire`/`AfterRelease` hook pair in `connection.go`
(today the pool is built plainly at `connection.go:208`):

- `BeforeAcquire`: `SET` (transaction-local at use sites via `SET LOCAL`, or
  session-level here then) `striatum.rpc_id` and `striatum.principal_id` for the
  acquired connection from the current RPC context.
- `AfterRelease`: `RESET ALL` / `DISCARD ALL`-equivalent for the custom GUCs (and
  `app.session_id` used by RLS) so provenance **never bleeds** across pooled
  checkouts. **C-ATTR-RESET.**

The `append_audit_row` function (§0.4) reads `current_setting('striatum.rpc_id', true)`
/ `striatum.principal_id` and stores them, so every authoritative mutation is
attributable to its originating RPC **and** principal — closing the RFC 0107
attribution gap. This layer changes no externally-visible behavior and is the
cheapest; it lands **first** alongside the L0 doctor probe.

- **`daemon_auth_log` (Q8):** an **owner-owned** table (created by owner
  migration; `striatumd_rw` cannot own it, per `0017`/`0023` precedent) recording
  auth posture/transitions: `(auth_event_id, ts, event, daemon_version,
  detail jsonb)` where `event ∈ {bootstrap, rotated, rotation_skipped_single_role,
  rotation_failed}` and `detail` carries **no secrets**. `daemon doctor` reads it
  over the **owner** connection, so "we rotated and the runtime credential is now
  broken" is diagnosable even when the runtime pool cannot connect.
- **Test (DC7):** T-ATTR-RESET pins that two sequential RPCs on the same pooled
  connection see only their own `rpc_id` (the second never observes the first's).

---

## 4. L2 — Isolation: lanes cannot reach PG out-of-band, by default

- Promote the dedicated **PG-less lane OS user** from opt-in advisory
  (`STRIATUM_LANE_OS_USER`, `doctor_lane_sandbox.go:13`) to the hardened default,
  backed by a filesystem boundary: PG's socket moves into a `0700`
  `unix_socket_directories` owned by a daemon identity **distinct** from the lane
  identity, so a lane cannot traverse the directory to the socket.
- `doctor_lane_sandbox.go` escalates `lane_pg_reachable` from a warning to a
  **startup-blocking** error when no distinct lane user is configured — gated
  behind `security.pg_socket_hardened` (**default-false** on upgrade; flip to
  default-on a later announced minor). **C-UPGRADE-SAFE / DC3.** Existing PEER
  installs are not stranded on upgrade day.
- Scrub `PGHOST` from the lane env (defense-in-depth; the env is already a strict
  positive allowlist — `supervisedEnvPassThrough`, `supervision_control.go:2558` —
  so this is belt-and-suspenders against a lane handing a guessed socket path to
  a libpq tool).
- **Adoption story (Q6):** ship default-false + a how-to (create the lane user,
  relocate the socket to the `0700` dir, set `STRIATUM_LANE_OS_USER`); doctor's
  block message names the exact remediation; flip default-on only after the
  how-to has shipped a release.

This is the structural close of **#87** (today only advisory).

---

## 5. Sequencing (DC4)

1. **L3 attribution + L0 doctor posture probe** — no behavior change, immediate
   RFC 0107 value. Lands first.
2. **L0 credential rotation + L1 Phase 0** (`audit_log`: v3 canonical +
   `REVOKE INSERT` + `append_audit_row` + T-42501) — the security core.
3. **L1 Phase 1 (`artifacts`) → Phase 2 (`events`)** — each provable-green first.
4. **L1 RLS** (`leases`, `sessions`) — second tier.
5. **L2** behind `security.pg_socket_hardened` (default-false; default-on is a
   separate announced minor).
6. **Cross-host client TLS certs** — deferred until a real multi-host deployment.

Sequenced **after** the RFC 0104/0105 reliability foundation; does **not** block
RFC 0103's remaining work. No native code, no wire proxy, no hosted identity
(DC8 / C-NO-NATIVE).

---

## 6. Acceptance (maps to RFC §Acceptance)

- **L0:** a fresh `striatumd` start rotates `striatumd_rw` with no on-disk secret
  in the default PEER posture; a DSN captured before a restart fails after it;
  `daemon doctor` reports posture without leaking the secret; remote-PG path
  documented (`STRIATUM_OWNER_DB_URL` / systemd credential).
- **L1:** `striatumd_rw` cannot directly `INSERT` into `audit_log` (then
  `artifacts`, `events`) — proven by **T-42501** (`42501`); `VerifyRows` holds
  end-to-end across the v2→v3 boundary (**T-VERIFY-MIXED**); Go/PL-pgSQL hash
  parity pinned (**T-HASH-PARITY**, **T-TS**).
- **L2:** with the hardened default configured, a lane cannot open the daemon's
  PG socket; `daemon doctor` **blocks** (not warns) when no distinct lane
  identity is configured under the flag.
- **L3:** every mutation carries attributable `rpc_id` + `principal_id`;
  provenance resets across pool checkouts (**T-ATTR-RESET**); `daemon doctor`
  diagnoses auth failure via the owner connection when the runtime credential is
  dead.
- `docs/reference/spec.md` carries the daemon→PG auth model + the
  database-enforced write-boundary invariant; `decision-log.md` records each
  per-phase decision on landing.

---

## 7. Rejected within this synthesis

- **Option B (trusted caller-supplied `row_hash`)** — lets a leaked-DSN caller
  inject a chain-breaking hash that `VerifyRows` flags (integrity DoS); a
  validating variant collapses to Option A. (§0.1)
- **Porting Go `encoding/json` into PL/pgSQL for v2 parity** — must reproduce
  HTML-escaping + codepoint key order forever; one stray `<`/`>`/`&` silently
  breaks the chain (C-HASH). v3 length-prefixed canonical removes the hazard. (§0.2)
- (Inherited from the RFC appendix, still rejected: PG C extension/background
  worker, user-namespace PG, `seccomp` socket block, PAM ppid module, quorum
  gate, `pg_notify`/`SD_LISTEN_FDS` to a client, abstract-namespace socket.)

---

## 8. Open residue (explicitly flagged, not hidden)

- The v2→v3 cutover requires `VerifyRows` to dispatch per `hash_format_version`;
  the Go `V2RowHash` path stays for legacy verification **permanently** (it is
  the only reader of pre-cutover rows). This is accepted cost, not tech debt to
  delete.
- L0 cannot make the runtime password unrecoverable from a live process's memory
  (pgx holds it for reconnect, §2/Q5); the guarantee is rotation-on-restart, not
  in-process secrecy. If the panel wants stronger in-process hygiene it is a
  separate, larger effort (and likely out of scope for local-first).

**Standing live for cross-examination.** The most attackable claims are §0.1
(Option A necessity), §0.3 (v3 byte-parity actually being reproducible in
PL/pgSQL), and §2/Q5 (the honest limits of "RAM-only").
