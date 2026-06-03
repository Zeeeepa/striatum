# RFC 0110 — Survey: prior art, evidence, and existing constraints

author: convener-claude-opus-4.8-001
artifact_kind: handoff
logical_name: survey_scan
topic: daemon → PostgreSQL authentication and the database-enforced write boundary
rfc: docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md
decision: D164 (accepted, design RFC)

## Charge and scope of this survey

This is the **survey_scan** stage of the RFC 0110 adjudicated-constraint-extraction
panel. Its job is to **record what is already known and what is contested** about
the daemon→PG auth boundary, grounded in live source, so the downstream
convener/spec_author can harden and sequence the accepted L0–L3 plan into an
implementation-ready spec.

It deliberately does **not** synthesize a solution, choose between open options,
or author spec text. Where a design decision is still open it is recorded as a
contested question with the evidence on each side, not resolved here. The
load-bearing L1 hash-canonicalization risk is documented to the byte so the next
stage can resolve it from fact rather than re-deriving it.

All file:line citations are against the run branch
`striatum/rfc-0110-pg-auth-panel` at survey time.

---

## 1. Prior art (the substrate this RFC sits on)

| Source | What it establishes | Relevance to 0110 |
| --- | --- | --- |
| RFC 0033 | PostgreSQL is the sole authoritative substrate; §3 append-only audit invariant | L1 generalizes §3 from "revoke UPDATE/DELETE" to "revoke all direct DML" |
| RFC 0043 | daemon-required runtime (`--no-daemon` retired) | The daemon is the only legitimate writer; 0110 makes that true in the DB, not just the process |
| RFC 0079 §5 | owner-applied migrations; runtime role cannot DDL owner tables | Supplies the L0 owner-PEER bootstrap path and the L1 `SECURITY DEFINER` function delivery (`--as-owner`) |
| RFC 0096 | supervised-lane trust boundary; session-bound capability tokens; env allowlist | The per-session substrate; L2 hardens its #87 residue; L3 RLS reuses its session binding |
| RFC 0107 | multi-principal trust model (several humans + AI operators, possibly remote PG) | Consumes L3 attribution + L1 isolation; 0110 supplies the substrate, not the principal layer |
| GH #87 | lane PG-reachability (a same-OS-user lane can open PG directly) | The open trust hole L2 closes structurally |

These are settled context, not in dispute. 0110 is a **design RFC**: it was
accepted (D164) as a phased plan, not as shipped code. No code has landed yet.

---

## 2. Current-state evidence (what the code does today)

### 2.1 How the daemon authenticates to PG (L0 surface)

- `db.ResolveConfig` (`go/pkg/db/connection.go:25`) resolves the runtime DSN in
  precedence order: explicit `--postgres-url` → `STRIATUM_DAEMON_DB_URL`
  (`EnvDaemonDBURL`, line 17) → `postgres_url` line in `daemon.toml`
  (`readConfigURL`, line 243) → empty.
- `db.Connect` (`connection.go:189`) parses the DSN with `pgxpool.ParseConfig`,
  forces `application_name=striatumd-go/<version>`, a default
  `statement_timeout=60000`, and **`QueryExecModeSimpleProtocol`** (line 207) so
  multi-statement migration files run unchanged. The password (if any) lives in
  the parsed `pgxpool.Config` for the life of the pool.
- `db.RedactURL` (`connection.go:50`) already redacts userinfo passwords and
  `password|pass|token|sslpassword` query keys for logging.
- The **live deployment** is owner-PEER, no password
  (`postgres://halbritt@/postgres?host=/var/run/postgresql`), i.e. the daemon
  authenticates as its own OS user over the unix socket. The documented
  alternative is a password-bearing least-privilege `striatumd_rw` role
  (`docs/how-to/postgres-transition.md`).

**Known:** there is no credential rotation today; whatever DSN `ResolveConfig`
returns is used as-is for the pool's lifetime. **Known:** migrations needing the
owner role use a separate `--as-owner`/owner-PEER path (RFC 0079 §5).

### 2.2 The write-contract enforcement gap (L1 surface)

- The hash-chained audit log is written by `AuditRecorder.RecordRPC` /
  `RecordRPCTransport` (`go/pkg/db/audit.go:55`/`:64`). It opens a transaction,
  `SELECT last_hash … FOR UPDATE` on the singleton `audit_chain_head`
  (`audit.go:96`), builds the row, computes `row_hash`, `INSERT`s into
  `audit_log`, and `UPDATE`s `audit_chain_head` — all atomically. The `FOR
  UPDATE` lock is what serializes the chain under concurrent RPC traffic.
- Append-only is enforced **at the trigger level** in `0001_baseline.sql`:
  `refuse_audit_change()` (line 149) is wired to `audit_log_no_update`
  (line 158) and `audit_log_no_delete` (line 163); closed segments and segment
  deletes are similarly guarded (lines 168–188). These triggers fire for **all**
  roles, including the owner.
- **But** `striatumd_rw` still holds **direct `INSERT`** on `audit_log`: migration
  `0005_repo_local_workflow_state.sql:470` does
  `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO
  striatumd_rw`, then revokes only `UPDATE, DELETE` on `events` (line 471) and
  `artifacts` (line 472). There is **no `REVOKE INSERT` on `audit_log`**. So a
  process authenticated as `striatumd_rw` can today `INSERT` a forged audit row
  directly (the trigger blocks update/delete, not insert).
- Other append-only REVOKEs already in place: `events`/`artifacts` UPDATE+DELETE
  (`0005`); `repo_event_chain_heads` DELETE (`0006:112`);
  `workflow_accepted_risks` UPDATE+DELETE (`0013:72`). `artifacts` allows a
  narrow blob-only UPDATE grant + trigger (`0010`).

**Known:** the "RPC/artifact API is the sole write path" invariant is enforced
**only in the daemon process**. Anything authenticating as `striatumd_rw` can
write authoritative tables directly. This is precisely what L1 (revoke direct
DML; expose writes only through owner-owned `SECURITY DEFINER` functions) closes.

### 2.3 Lane reachability (L2 surface)

- `laneSandboxDoctorBlock` (`go/pkg/reads/doctor_lane_sandbox.go:48`) emits the
  **advisory-only** `lane_pg_reachable` warning. It is a *configuration-posture
  proxy* (`"no live PostgreSQL probe"`, line 54), keyed on
  `STRIATUM_LANE_OS_USER` (`laneOSUserEnv`, line 13). It clears only when a
  distinct, existing lane OS user is configured (lines 62–66); otherwise it warns
  that lanes run as the daemon's user and can open PG directly via PEER. It never
  returns a hard `problems` failure.
- The supervised-lane env is a **strict positive allowlist**:
  `supervisedEnvPassThrough` (`go/pkg/mutations/supervision_control.go:2558`)
  filters `os.Environ()` down to `supervisedEnvAllowlistKeys` (line ~2510) plus
  `LC_*`. Everything else — every `*DSN*`/`*POSTGRES*`/`PG*`/`DATABASE_URL`
  var — is dropped (comment at `:2422`/`:2503`; gate
  `TestSupervisedLaneEnvDropsBannedKeepsRequired`,
  `conformance_env_test.go`). Note `STRIATUM_MCP_TOKEN` is *deliberately not*
  allowlisted (RFC 0096 V2 / #135): the lane is injected its own session-bound
  token instead.

**Known:** the **DSN-leak half is already closed** by the allowlist. **Known:**
the residual hole is that **PEER auth needs no secret** — scrubbing env vars does
nothing against a lane that simply opens the socket as the shared OS user. That
residual same-OS-user reachability is #87, and L2 is its structural close
(distinct lane identity + `0700` socket directory).

### 2.4 Attribution (L3 surface)

- The audit row records `client_id`, `repository_id`, `method`, `request_id`,
  `transport`, `decision` (`audit.go:141`–`:157`), i.e. it already knows the RPC
  and (via token) the client. It does **not** carry a transaction-scoped
  `rpc_id`/`principal_id` set on the *connection* — there is no
  `pgxpool.BeforeAcquire`/`AfterRelease` hook in `connection.go` today (the pool
  is built plainly at `connection.go:208`).

**Known:** L3 (per-transaction `SET LOCAL striatum.rpc_id/principal_id` cleared
on release) is **additive and standalone** — it changes no existing behavior and
delivers immediate RFC 0107 attribution value. It is the cheapest layer.

---

## 3. The load-bearing L1 risk — hash canonicalization (documented, not resolved)

This is the single most consequential constraint on L1 and is recorded here to
the byte so the spec stage resolves it from fact.

### 3.1 What the Go hash actually is

`V2RowHash` (`audit.go:25`) builds a `map[string]any` of exactly **15 keys** and
passes it to `CanonicalHash` (`audit.go:16`), which is
`hex(sha256(json.Marshal(material)))`. Because `material` is a Go map,
`encoding/json` emits its keys in **ascending Unicode-code-point order**:

```
client_id, daemon_version, decision, denial_reason, exit_code,
hash_format_version, method, params_sha256, previous_hash, repository_id,
request_id, schema_version, segment_id, transport, ts
```

Serialization details that any re-implementation must match exactly:

- **Compact, no spaces** between tokens (`{"client_id":...,"daemon_version":...}`).
- **HTML escaping is ON.** `encoding/json.Marshal` escapes `<`, `>`, `&` as
  `<`, `>`, `&`, and U+2028/U+2029 as ` `/` `. (Daemon
  version strings, repository ids, denial reasons could in principle contain
  `&`/`<`.)
- **`null` for Go `nil`.** `client_id`, `repository_id`, `denial_reason`,
  `previous_hash` go through `nullString`/`nullableFromPtr` → `nil` → `null`.
  `exit_code` is `nil` when `response.OK`, else integer `10` (`audit.go:133`–`:138`).
- **Plain integers, unquoted, no decimal point:** `schema_version=1`,
  `hash_format_version=2`, `exit_code=10`, `segment_id` (a Go `int64`).
- **`ts` is a string**, not a timestamp value: `time.Now().UTC().Truncate(time.Second)`
  formatted as RFC3339 (`audit.go:140`), e.g. `"2026-06-03T15:00:00Z"`.

### 3.2 Why PostgreSQL-native JSON does not match out of the box

- **`jsonb` reorders keys** (length-then-bytewise), *not* by Unicode code point,
  and may normalize values — so `to_jsonb`/`jsonb` text **cannot** be used directly.
- **`json` / `json_build_object`** preserves argument order (so the author could
  hand-order keys) and is compact, **but does not HTML-escape** `<`/`>`/`&` — it
  emits them literally — so it diverges from Go for any value containing those
  characters.
- Number and null handling can be made to match with care, but the type of each
  argument must be pinned (integer vs numeric vs text) to avoid `1.0`/`"1"`
  artifacts.

### 3.3 The two-sided constraint (the part that is easy to miss)

The hash is verified on **read** by `VerifyRows` (`audit.go:213`), which
**recomputes `V2RowHash` in Go** from the columns read back, comparing against the
stored `row_hash` and chaining `previous_hash`. Therefore an L1 PL/pgSQL write
function's computed `row_hash` must equal **both**:

1. the write-time Go material (so the chain is consistent at insert), and
2. the read-time Go recomputation over the **column values as read back**.

The sharpest sub-risk here is `ts`: it is *hashed as an RFC3339 second-truncated
UTC string* but *stored as `timestamptz`*. Any hash computed in-DB must format
the `timestamptz` to the **identical** string the Go read path reconstructs, or
`VerifyRows` breaks every chain even though nothing was tampered with.

### 3.4 The open question (contested — for the spec stage to decide)

D164's "Revisit" clause already names the fork:

- **Option A — port the canonicalization into PL/pgSQL.** Build the canonical
  string explicitly (hardcoded alphabetical key order + manual value encoding,
  including HTML-escaping of `<`/`>`/`&` and control chars exactly as Go does)
  and `digest(..., 'sha256')`. Achieves true in-engine enforcement but must
  reproduce Go's `encoding/json` byte-for-byte and stay pinned to it forever.
- **Option B — keep hash computation in Go behind a trusted-parameter
  `SECURITY DEFINER` function.** The function accepts the already-computed
  `row_hash` (and `previous_hash`) as trusted parameters and only enforces the
  *locking + append* invariant in-DB. Removes the canonicalization-parity
  burden, but the hash is still computed by the (trusted) daemon process, so the
  "DB independently recomputes the hash" property is weaker.

This survey records both with their trade-offs. It does **not** pick one — that
is the convener/spec_author's call, and is the highest-leverage open decision in
the whole RFC.

---

## 4. What is already known (settled, low-dispute)

1. The architecture is **layered and independently shippable** (L0–L3); the
   sequencing in the RFC (L3 + L0 doctor probe first → L0 rotation + L1 Phase 0 →
   L2 behind a flag → cross-host certs deferred) is accepted.
2. **No C extension, no from-scratch PG wire proxy, no SaaS/IdP/telemetry** — all
   recorded as rejected alternatives in the RFC appendix; in-DB enforcement is
   stock PL/pgSQL + RLS + GRANT/REVOKE only.
3. The **DSN-leak vector is already closed** by the strict env allowlist; the
   remaining lane vector is PEER same-OS-user reachability (#87).
4. `audit_log` UPDATE/DELETE is already trigger-blocked; the *missing* control is
   `REVOKE INSERT` + a write-only `SECURITY DEFINER` path (L1 Phase 0 target).
5. **L3 is additive and standalone** and the cheapest first move.
6. **L2 must be upgrade-safe**: a default-false `security.pg_socket_hardened`
   flag, with default-on as a separate announced minor — existing PEER installs
   must not be stranded on upgrade day.

## 5. What is contested / open (for downstream stages, not resolved here)

| # | Open question | Evidence anchors |
| --- | --- | --- |
| Q1 | **L1 hash parity**: port canonicalization to PL/pgSQL (Option A) vs trusted-parameter function keeping hash in Go (Option B). | §3; `audit.go:16-44,213`; D164 Revisit |
| Q2 | **`ts` representation under L1**: how the in-DB path reproduces the RFC3339 second-truncated UTC string from a `timestamptz` so write-hash == read-recompute. | `audit.go:140`, `:213` |
| Q3 | **Phase-0 scope of the `SECURITY DEFINER` rewrite**: does the function also take over the `FOR UPDATE` chain-head lock + segment open/create logic (`audit.go:96-127`), or only the final INSERT? | `audit.go:82-204` |
| Q4 | **L0 remote-PG escape hatch shape**: `STRIATUM_OWNER_DB_URL` plaintext vs systemd `LoadCredentialEncrypted=`/`sd_get_credentials()` as the recommended floor; single-role dev guard (skip rotation with WARN when owner==runtime). | RFC §L0; `connection.go:25` |
| Q5 | **L0 in-memory password hygiene**: how/whether to zero the password after `pgxpool.ParseConfig` consumes it given pgx retains it in `Config` for reconnects. | `connection.go:189-208` |
| Q6 | **L2 lane-identity adoption friction**: `STRIATUM_LANE_OS_USER` is opt-in advisory today; what is the migration story to make a distinct lane user + `0700` `unix_socket_directories` the default without breaking single-user dev. | `doctor_lane_sandbox.go`; RFC §L2 |
| Q7 | **L1 RLS scope**: which per-session tables (`leases`, `sessions`) get RLS keyed on `current_setting('app.session_id')`, and how it composes with L3's `SET LOCAL` and pooled-connection reset. | RFC §L1/§L3 |
| Q8 | **`daemon_auth_log` + doctor owner-fallback read**: schema and the owner-connection read path that survives a dead runtime credential. | RFC §L3 |
| Q9 | **CI gate placement**: the `pgtest` `42501` negative-path test runs `striatumd_rw_<db>` per-test — confirm it can assert the post-REVOKE direct-INSERT failure without disturbing the positive write path. | RFC §L1 acceptance |

## 6. Constraints to carry into constraint-extraction (candidate rows)

Recorded as candidate constraints for the panel, not as adjudicated constraints:

- **C-HASH (binding):** any in-DB `row_hash` computation must be byte-identical
  to Go `encoding/json` over the 15-key material (alphabetical-by-codepoint keys,
  HTML escaping on, compact, `null` for nil, unquoted integers, RFC3339
  second-truncated UTC `ts` string) AND must agree with the Go read-side
  `VerifyRows` recomputation. A mismatch silently breaks every chain.
- **C-INSERT-REVOKE:** L1 Phase 0 must `REVOKE INSERT ON audit_log FROM
  striatumd_rw` and route inserts through an owner-owned `SECURITY DEFINER`
  function; today that INSERT grant is live (`0005:470`).
- **C-OWNER-DDL:** all L1 function/REVOKE DDL is owner-applied (RFC 0079 §5);
  `striatumd_rw` cannot create or own these objects.
- **C-UPGRADE-SAFE:** L2 enforcement ships default-false; default-on is a
  separate announced minor. PEER installs must keep working on upgrade.
- **C-NO-NATIVE:** no C extension, no compiled `.so`, no wire proxy, no external
  identity service (rejected alternatives, RFC appendix).
- **C-ATTR-RESET:** L3 provenance (`rpc_id`/`principal_id`) must reset across
  pooled checkouts so it never bleeds between RPCs (test-pinned).

---

## 7. Explicit non-actions of this survey

- No solution chosen for Q1–Q9; both L1 hash options are left open with
  evidence.
- No spec text, migration, or function authored.
- No code touched outside this artifact's write scope
  (`docs/operator/artifacts/rfc-0110-pg-auth/survey/scan/`).

Handoff: the convener/spec_author stage should open from §3 (resolve Q1/Q2
first — they gate L1) and §6 (candidate constraints), using the §2 file:line
anchors as the implementation map.
