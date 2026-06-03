---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: convener-claude-opus-4.8-001
workflow: rfc-0110-pg-auth-panel
phase: survey_synthesis
run_id: run_8e14cb48342e929d30043d6be24f9101
title: "RFC 0110 — Survey synthesis: problem framing, goals, non-goals, decision criteria"
inputs:
  - docs/operator/artifacts/rfc-0110-pg-auth/survey/scan/SURVEY.md
  - docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md
---

# RFC 0110 — Survey synthesis: framing the implementation-ready spec

This artifact frames the problem, goals, non-goals, and **decision criteria**
that the candidate synthesis (and the spec that follows) must satisfy. It is
derived from the survey (`../scan/SURVEY.md`) and the accepted design RFC 0110
(D164). It deliberately **does not choose** the open L1 option or author spec
text — it defines the scope the candidate synthesis is obligated to address and
the bar by which it will be judged.

---

## 1. Problem (distilled)

The invariant "the RPC/artifact API is the sole write path to the authoritative
PostgreSQL state" is enforced **only inside the daemon process** today. Any
process that authenticates to PG as the runtime role can write the authoritative
tables directly, bypassing the artifact/RPC API and the hash-chained audit log.

Two live postures, each with a hole (survey §2.1):

- **owner-PEER, no password** (the live deployment): conflates the daemon's
  identity with its OS user; a supervised lane spawned as that same OS user can
  open PG directly over the socket and forge/tamper — the **#87** trust hole.
  The env allowlist cannot help because PEER needs no secret (survey §2.3).
- **password as `striatumd_rw`**: trades the #87 socket reach for a **standing
  file secret** that survives restarts and never rotates.

Three further forces are unaddressed (RFC §Problem): migrations need a
higher-privileged owner role; the future is multi-principal and possibly
remote-PG (RFC 0107); the audit log knows the daemon wrote a row but not which
RPC/principal caused it.

**Framing claim the spec must honor:** authentication must not be the only thing
standing between an untrusted same-host process and the audit chain. The design
rule is **"make a leaked runtime credential uninteresting,"** not "achieve
perfect secrecy."

## 2. Goals (what the implementation-ready spec must achieve)

Each goal is independently shippable and independently valuable (RFC's layered
model). The spec must keep them decoupled.

- **G0 — Credential (L0):** the runtime `striatumd_rw` credential becomes
  ephemeral, owner-bootstrapped, RAM-only, and re-rotates every daemon restart,
  with **no on-disk secret** in the default local-PEER posture; a documented
  remote-PG path (`STRIATUM_OWNER_DB_URL` / systemd encrypted credential); a
  single-role dev guard; and a `daemon doctor` posture probe that asserts a
  password is set **without reading or logging it**.
- **G1 — Enforcement (L1):** PostgreSQL itself guards the write contract —
  revoke direct DML and expose writes only through owner-owned
  `SECURITY DEFINER` PL/pgSQL functions (+ RLS as a second tier on per-session
  tables), phased **audit_log → artifacts → events**, each provable-green before
  the next, with the hash-chain `VerifyRows` invariant preserved end-to-end.
- **G2 — Isolation (L2):** supervised lanes cannot reach PG out-of-band by
  default — a dedicated PG-less lane OS user + a `0700` `unix_socket_directories`
  owned by a daemon identity distinct from the lane identity; `daemon doctor`
  escalates `lane_pg_reachable` from warning to a **startup block** under an
  enforcement flag; `PGHOST` scrubbed from lane env as defense-in-depth.
- **G3 — Attribution (L3):** every authoritative mutation carries an
  attributable `rpc_id` + `principal_id` (via `pgxpool` BeforeAcquire/AfterRelease
  `SET LOCAL`), provenance reset across pooled checkouts; a `daemon_auth_log`
  table readable by `daemon doctor` over the **owner** connection even when the
  runtime credential is dead.

## 3. Non-goals (the spec must not drift into these)

- **Not SaaS / hosted identity.** No external IdP/SSO, no hosted control plane,
  no telemetry, no new hosted services (reaffirms RFC 0107). The local
  self-signed CA and systemd credentials are operator-owned.
- **Not a PostgreSQL C extension or background worker.** In-database enforcement
  uses only stock PL/pgSQL + RLS + GRANT/REVOKE — no compiled `.so`, no
  PG-version-fragile native code.
- **Not a from-scratch PG wire proxy.** L2 is a `0700` socket directory +
  distinct OS identity, not a new in-process protocol server.
- **Does not itself deliver the RFC 0107 multi-principal model** — it supplies
  the credential/enforcement/isolation/attribution substrate that model builds on.
- **No cross-host client-TLS-cert work now** — deferred until a real multi-host
  deployment exists (purely additive later).

## 4. Decision criteria (how the candidate synthesis will be judged)

The candidate synthesis is **accepted** only if it satisfies all of:

- **DC1 — Resolve the load-bearing L1 hash risk (binding).** It must pick and
  justify one of:
  - **Option A:** port the canonicalization into PL/pgSQL, **byte-identical** to
    Go `encoding/json` over the 15-key material — alphabetical-by-codepoint keys,
    HTML escaping of `<`/`>`/`&` (+ U+2028/U+2029), compact, `null` for nil,
    unquoted integers, and the RFC3339 second-truncated UTC `ts` **string** — and
    must agree with the Go read-side `VerifyRows` recomputation; **or**
  - **Option B:** keep hash computation in Go behind a trusted-parameter
    `SECURITY DEFINER` function that enforces only the lock + append invariant.

  Either way the spec must name the **parity test** that proves write-hash ==
  read-recompute, and must address **ts representation** (Q2) and whether the
  function also owns the `FOR UPDATE` chain-head lock + segment open/create
  logic (Q3). This is the single highest-leverage decision in the RFC; an
  unresolved or hand-waved DC1 is an automatic `needs_revision`.
- **DC2 — Enforcement is machine-verified and phased.** A `pgtest` negative-path
  test asserting a direct `INSERT` from `striatumd_rw` fails with SQLSTATE
  **`42501`** must exist and gate every migration forward; the phasing
  (audit_log → artifacts → events) must be explicit, each phase provable-green
  before the next. Note the concrete starting point: `striatumd_rw` **today holds
  direct INSERT on `audit_log`** (the `0005` `GRANT ALL`; append-only is only
  trigger-enforced for UPDATE/DELETE) — Phase 0 must `REVOKE INSERT` and route
  through the function.
- **DC3 — Upgrade safety.** L2 enforcement ships behind a default-false
  `security.pg_socket_hardened` flag; the flip to default-on is a separate,
  announced minor version; existing PEER installs must not be stranded on
  upgrade day.
- **DC4 — Sequencing honored.** L3 + the L0 doctor posture probe land first (no
  behavior change, immediate RFC 0107 value); then L0 rotation + L1 Phase 0
  (the security core); then L2 behind the flag; cross-host certs deferred. The
  work is sequenced **after** the RFC 0104/0105 reliability foundation and does
  **not** block RFC 0103's remaining work.
- **DC5 — Owner-applied DDL.** All L1 function/REVOKE DDL and any role changes
  are owner-applied (RFC 0079 §5); `striatumd_rw` cannot create or own these
  objects. The spec must say how the DDL is delivered.
- **DC6 — Secret hygiene is provable.** The L0 in-memory password is never
  written to disk/env/`daemon.toml`; `daemon doctor` asserts posture without
  reading the value; the spec addresses zeroing the password after
  `pgxpool.ParseConfig` consumes it given pgx retains it for reconnects (Q5).
- **DC7 — Attribution resets, test-pinned.** L3 `rpc_id`/`principal_id` must
  reset across pool checkouts so provenance never bleeds between RPCs, proven by
  a pinned test; the `daemon_auth_log` owner-fallback read path is specified.
- **DC8 — Non-goals respected.** No native code, no wire proxy, no hosted
  identity/telemetry anywhere in the proposed implementation.

## 5. Scope the candidate synthesis must address (must-discharge questions)

Carried forward from survey §5 — the candidate synthesis must take a position on
each (resolve, or explicitly defer with justification and a follow-up owner):

| # | Must-resolve question | Gates |
| --- | --- | --- |
| Q1 | L1 hash parity: Option A (PL/pgSQL byte-parity) vs Option B (trusted-parameter, hash stays in Go). | DC1 — L1 |
| Q2 | `ts` representation: how in-DB hashing reproduces the RFC3339 second-truncated UTC string so write-hash == read-recompute. | DC1 |
| Q3 | `SECURITY DEFINER` scope: does the function own the `FOR UPDATE` chain-head lock + segment open/create, or only the final INSERT? | DC1/DC2 |
| Q4 | L0 remote-PG escape hatch: `STRIATUM_OWNER_DB_URL` plaintext vs systemd `LoadCredentialEncrypted=`; single-role dev guard (skip rotation w/ WARN). | DC6 — L0 |
| Q5 | L0 in-memory password hygiene (zeroing vs pgx retention for reconnect). | DC6 |
| Q6 | L2 lane-identity adoption story: opt-in advisory → default without breaking single-user dev. | DC3 — L2 |
| Q7 | L1 RLS scope: which per-session tables (`leases`, `sessions`), and composition with L3 `SET LOCAL` + pooled-connection reset. | DC1/DC7 |
| Q8 | `daemon_auth_log` schema + doctor owner-fallback read when the runtime credential is dead. | DC7 — L3 |
| Q9 | `pgtest` `42501` gate placement: assert post-REVOKE direct-INSERT failure without disturbing the positive write path. | DC2 |

### Binding candidate constraints (for constraint extraction)

These are the candidate constraint rows the cross-examination/adjudication stages
should sharpen into binding constraints (survey §6):

- **C-HASH** — in-DB `row_hash` must be byte-identical to Go `encoding/json` over
  the 15-key material **and** agree with the Go read-side `VerifyRows`
  recomputation; a mismatch silently breaks every chain.
- **C-INSERT-REVOKE** — L1 Phase 0 must `REVOKE INSERT ON audit_log FROM
  striatumd_rw` and route inserts through an owner-owned `SECURITY DEFINER`
  function (the INSERT grant is live today).
- **C-OWNER-DDL** — all L1 DDL is owner-applied (RFC 0079 §5).
- **C-UPGRADE-SAFE** — L2 enforcement default-false; default-on is a separate
  announced minor; PEER installs keep working on upgrade.
- **C-NO-NATIVE** — no C extension, no wire proxy, no external identity/telemetry.
- **C-ATTR-RESET** — L3 provenance resets across pooled checkouts (test-pinned).

## 6. Explicitly out of scope for this framing

- No selection between Option A and Option B (that is the candidate synthesis's
  job, judged against DC1).
- No spec text, migration SQL, PL/pgSQL function, or code authored here.
- No change outside this artifact's write scope
  (`docs/operator/artifacts/rfc-0110-pg-auth/survey/synthesis/`).

**Handoff:** the candidate synthesis should open at DC1/Q1–Q3 (the L1 core),
treat §4 as its acceptance bar and §5 as its obligation list, and use the
survey's §2 file:line anchors as the implementation map.
