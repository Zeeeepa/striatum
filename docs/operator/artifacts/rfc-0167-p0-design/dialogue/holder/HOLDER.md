# HOLDER — RFC 0167 P0 falsifiable implementation SPEC (operator identity & run attribution)

author: holder-author-001

> This is the leading proposal for the RFC 0167 **P0** implementation SPEC, published as
> the claim the two falsifiers will re-attack. It hardens the accepted RFC's P0 (D260)
> against adversarial falsification and is the contract the downstream
> `rfc-0167-p0-build` `code_change` run executes. Every load-bearing claim below is stated
> as a **falsifiable assertion** paired with the **named test/check that would refute it**,
> anchored to **verified current-`main` source** (`go/pkg/...` `file:line`). I did not trust
> the RFC's anchors — I read the source, and §0 records the four places the RFC's anchors
> are **wrong or imprecise** and how this SPEC corrects them. Scope is **P0 only**; P1–P3
> seams are noted, never designed.

---

## How this SPEC discharges R1a / R1b / R1c / R2 / R3 / R4 (auditable coverage map)

| Req | What it demands | Where discharged | Load-bearing assertion(s) |
|-----|-----------------|------------------|---------------------------|
| **R1a** | Identity bound server-side, at token-mint, against the live token; never from tty/tmux/title/env/client name; every read surface resolves through `principal_id` and only snapshots the handle | §1 | A1–A5 |
| **R1b** | THE CRUX — one human = one `principal_id` across ~15 terminals; specify exactly how P0 still answers "which window", the deterministic collision-escalation rule, the exact run→handle join, and **prove two same-human terminals return two distinct answers** | §2 | A6–A11 |
| **R1c** | Heartbeat renews the existing lease via guarded UPDATE, never release-then-reacquire | §3 | A12 |
| **R2** | Owner-bundle migration at the next free ordinal; choose DB write-once enforcement and justify; pin retained runtime privileges; prove clean apply + write-once under the RFC 0142 two-role pgtest fixture; forward-only, watermark-consistent | §4 | A13–A19 |
| **R3** | Resolve all four open questions concretely (in-P0 / deferred + mechanism + why) | §5 | A20–A23 |
| **R4** | Ride RFC 0107 (operator-id IS `principal_id`); no parallel identity table; reuse principals/principal_clients/session liveness; product-boundary clean | §6 | A24–A26 |

The full assertion ledger (claim / supporting evidence / refuting observation) is consolidated in **§8**. The P0 boundary and P1–P3 seams are **§7**. The concrete build manifest is **§9**.

---

## §0 — Verified source baseline and four corrections to the RFC's anchors

The holder is directed to *verify, not trust*. I read current `main`. The substrate is as the RFC describes in shape, but **four anchors are wrong or imprecise** and the SPEC is built on the corrected facts, not the RFC's prose.

**Verified true (load-bearing, unchanged):**

- `LatestOwnerBundleVersion == 20` and `RequiredOwnerBundleVersion == LatestOwnerBundleVersion == 20` — `go/pkg/db/owner.go:23,35`. **The next free owner-bundle ordinal is 21.** The RFC's "current `LatestOwnerBundleVersion == 20`" is correct.
- A table is **owner-held by default** and becomes runtime-`ALTER`-able **only** once an owner bundle transfers its ownership to `striatumd_rw` — `go/pkg/db/owner_runtime_ownership.go:8-11`, with the transferred set *derived from the owner-bundle SQL* by `RuntimeOwnedTablesAlterable()` (`owner_runtime_ownership.go:37-76`). The runtime role is the string literal `striatumd_rw` (`owner.go`, the SQL bundles).
- The session capability token is minted **inside** the `session.register` transaction: `mintSessionBoundToken` "Runs inside the registration transaction so the token is committed atomically with the session row" — `go/pkg/mutations/session_token.go:48-53,60-97`, invoked from `HandleRegisterSession`'s `withTxRetryOnDeadlock` body (`go/pkg/mutations/lifecycle.go:36,79`). This is the transaction R1a/D1 require, and it is a real, single DB transaction.
- The caller's identity is resolvable **server-side from the live token** inside any authorized mutation: the RFC 0110 authority prelude installs it as a transaction-local GUC (`go/pkg/db/authority.go:116-120,135-158`), sourced from `rpc.AuthFromContext(ctx).ClientID` (`authority.go:75-85`) which the `PostgresAuthorizer` resolves by validating the presented bearer token against `striatumd.clients` (HMAC-SHA256, `revoked_at`/`expires_at` checks) — `go/pkg/rpc/auth_pg.go:49-157`.
- The two-role pgtest fixture exists: `pgtest.TwoRole(t) *TwoRoleFixture` with separate `OwnerPool`/`SUTPool` and owner/runtime role DSNs — `go/pkg/pgtest/two_role.go:47-78`; the `42501` privilege-gap oracle is `assertSQLState42501` — `go/pkg/db/two_role_pg_test.go:161-176`.

**CORRECTION C-1 — `runs.created_by_principal_id` is an OWNER-bundle change, and the RFC's reason is right for a subtly wrong stated cause.** The RFC calls `ALTER runs ADD COLUMN` an owner bundle because `runs` is "owner-held / FK-bearing." Verified: `runs` is created by **runtime migration 0005** (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:13-36`), but on a two-role deploy the `sql/NNNN_*.sql` migrations are applied **by the owner role** during fresh-bootstrap, so `runs` is **owned by the owner role**, and it is **not** in owner-bundle 0018's ownership-transfer cohort (`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:77-106` — the cohort is `job_recovery_state`, `barrier_staged_contributions`, `barrier_state`, `fanin_freeze_points`, `conversations`, `conversation_post_dialog_hooks`, `dissent_ledger`, `interrogations`, `job_workspaces`, `spawn_authorization_grants`; **`runs` is absent**). The runtime role has table-level DML on `runs` (`0005:467-475`) but **not** ownership, so a runtime-migration `ALTER runs` would die `42501 must be owner of table runs` and crash-loop (the #441/#442 / D248 trap, documented verbatim at `0018:8-22`). **Conclusion: the owner-bundle framing is correct; `ALTER runs ADD COLUMN` goes in owner bundle 0021.** (FK-bearing is true but not the operative reason — *ownership*, not FKs, is what forces the owner bundle.)

**CORRECTION C-2 — the authority GUC `striatum.principal_id` holds the `client_id`, not the RFC-0107 `principal_id`.** The prelude sets `set_config('striatum.principal_id', $3, true)` where `$3 = attr.PrincipalID = auth.ClientID` (`authority.go:78,116-120`). So the value in-transaction is the **client_id**. To stamp the real `principal_id`, the run-creation transaction must dereference `client_id → principal_id` via the **active** `principal_clients` link, exactly as `ResolvePrincipalForClient` does (`go/pkg/admin/principals.go:266-292`: `principal_clients pc JOIN principals p ... WHERE pc.client_id = $1 AND pc.unlinked_at IS NULL`). The SPEC stamps through this dereference; a naïve `created_by_principal_id := current_setting('striatum.principal_id')` would store a client_id and silently mis-key every join.

**CORRECTION C-3 — there is no periodic "migration-0033 reconcile sweep" that reaps stale sessions; migration 0033 reaps terminal-run *supervisors*.** The RFC (and SEED R4) say "reuse `sessions.last_session_heartbeat_at` + the migration-0033 reconcile sweep that already reaps stale sessions." Verified: (a) the column is `sessions.last_heartbeat_at` (`0005:44-70`), not `last_session_heartbeat_at`; (b) migration 0033 is `sql/0033_reap_terminal_run_supervisors.sql` and reaps **process/daemon supervisors** for terminal runs, **not sessions**; (c) graceful session teardown is `closeRemainingSessions` (`go/pkg/mutations/mutations.go:1432`), which runs in the run-termination transaction. There is **no** background session reaper. The SPEC therefore specifies handle-lease release as **(a) graceful** — folded into the existing session-close transaction — **plus (b) lazy expiry** via `leased_until`, matching the project's "lease expiry is lazy" policy and the existing `striatumd.leases` TTL pattern (`0005:166-186`), rather than leaning on a sweep that does not exist.

**CORRECTION C-4 — `created_by_principal_id` alone is insufficient, and the SEED's OQ2 backfill source carries no identity.** `branch_confirmed_by` holds the literals `'daemon'`/`'human'` (`run.go:1053`, `run.go:887-891`), **not** a `principal_id`. A "backfill from `branch_confirmed_by`" would therefore fabricate identity from a non-identity field — the exact dishonest stamp R1a forbids. This directly informs the OQ2 decision (§5.2). Separately, since one human = one `principal_id` across all terminals, `created_by_principal_id` cannot answer "which window" (R1b); the SPEC adds a **write-once per-session `runs.created_by_handle_id`** as the disambiguator (§2). R1b explicitly authorizes this ("decide and specify: does the run carry a handle or lease snapshot stamped at creation").

---

## §1 — R1a: identity is bound server-side, at token-mint, against the live token

### Design

P0 introduces two write paths and binds identity server-side at both.

**(1) The handle lease is acquired inside the token-mint transaction.** `mintSessionBoundToken` already runs inside `HandleRegisterSession`'s single transaction (`session_token.go:48-53`; `lifecycle.go:79,417`). P0 adds, in that **same transaction, after the mint**, a handle-lease acquisition into the new `operator_handles` table (§2 for the lease algorithm), keyed on:

- `principal_id` = `ResolvePrincipalForClient(current_setting('striatum.principal_id', true))` — the **registering caller's** live token dereferenced to its human principal via the active `principal_clients` link (`principals.go:266-292`). Never a client-supplied name, never a display signal. If the caller has no active principal link (pre-RFC-0107 / bootstrap admin), the lease is skipped and identity stays the bare id — honest unknown, not a guess.
- `leased_session_id` = the new `session_id` generated at `lifecycle.go:110` (`newID("sess")`).

Because mint + lease share one transaction, there is no window in which a token exists without its handle lease, and no client RPC can interpose a name between them.

> **Operator-session seam (verified gap, in-P0).** The RFC's D1 says bootstrap mints/leases the handle, but `striatum operator bootstrap` is today a **read-only CLI command** (`go/cmd/striatum/operator_bootstrap.go`), not a token-minting daemon RPC. The per-window granularity R1b needs requires each terminal to hold a **distinct session** whose token it presents on run creation. P0 therefore reuses the **existing** session-bound mint machinery (`mintSessionBoundToken`) for an **operator session**: bootstrap gains a daemon-side mint+lease RPC that, in one transaction, resolves/creates the caller's `principal` (kind `human`, RFC 0107 path), mints a session-bound operator token, and leases the handle keyed on the new session. The CLI carries that session token; subsequent `run.prepare`/`run.start` present it (so `auth.SessionID` is populated → the `app.session_id` GUC is set, `authority.go:79,120`). This rides existing code; it does **not** add a parallel identity store (R4). See §2 for why per-session is the only sufficient granularity and §7 for the P0/P1 boundary.

**(2) `runs.created_by_principal_id` is resolved from the live token at run creation, server-side.** The runs INSERT (`run.go:1056-1074`) is extended to set the new columns from in-transaction GUCs, never from envelope params:

```sql
INSERT INTO striatumd.runs (
  repository_id, run_id, workflow_snapshot_id, repo_root, state,
  branch_name, branch_base, branch_confirmed_at, branch_confirmed_by, created_at,
  created_by_principal_id, created_by_handle_id
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
  (SELECT pc.principal_id
     FROM striatumd.principal_clients pc
    WHERE pc.client_id = current_setting('striatum.principal_id', true)  -- = caller's client_id (C-2)
      AND pc.unlinked_at IS NULL),
  (SELECT oh.handle_id
     FROM striatumd.operator_handles oh
    WHERE oh.leased_session_id = current_setting('app.session_id', true)
      AND oh.released_at IS NULL)
);
```

`HandleRunPrepare` reads **only** `repository_id` and `workflow` from the envelope (`run.go:21-28`); there is no `created_by` parameter path, so a forged `created_by_principal_id` in the request is structurally ignored. The stamp is a server-side subquery over the prelude GUC, which is sourced from the validated live token (`auth_pg.go:49-157`).

**(3) Every read surface resolves through `principal_id` and only snapshots the handle.** `whose`, `status --mine`, `doctor`, and evidence export (§2.4, §5) render from a pure PG join on `created_by_principal_id` / `created_by_handle_id` / `operator_handles` / `principals`. No tty, pane, tmux, title, or env value appears in any authoritative answer.

### Falsifiable assertions

- **A1 (server-side stamp).** `runs.created_by_principal_id` equals the principal of the *live token presented on the run-creation RPC*, dereferenced server-side. *Refuting test:* `run_attribution_pg_test.go` — open two sessions whose tokens resolve to principals `P_A` and `P_B`; create a run on `P_A`'s token while passing envelope param `created_by_principal_id = P_B` **and** setting `STRIATUM_*`/tty/tmux/title spoofs; assert the stored value is `P_A`. If it is `P_B` (or the spoof leaks), A1 is refuted.
- **A2 (no client-name path).** No code path lets a client-supplied string become `created_by_principal_id` or a rendered handle. *Refuting test:* a static guard test greps the `run.prepare`/`session.register` handlers for any `stringParam(envelope, "created_by*"|"handle"|"operator*")` feeding the stamp/lease; presence refutes A2. (Today `operator_label` is a *display-only* label, `lifecycle.go:71,114-121`, never the attribution key — the guard pins that it stays display-only.)
- **A3 (mint+lease atomicity).** The handle lease is committed in the same transaction as the session token. *Refuting test:* inject a fault between `mintSessionBoundToken` and the lease INSERT and assert the whole `session.register` rolls back (no token row without its lease row). A token committed without a lease refutes A3.
- **A4 (live-token resolution).** Identity derives from `auth.ClientID` set by `PostgresAuthorizer.Authorize` from the validated token, not from any header/label. *Refuting test:* present a revoked/expired token to `run.prepare`; assert the RPC is rejected at `auth_pg.go:87-92` and no run row is created (so no stamp can be forged post-auth).
- **A5 (read surfaces cannot lie).** `whose`'s authoritative answer is a function of `{created_by_principal_id, created_by_handle_id}` joined to `operator_handles`/`principals` only. *Refuting test:* a unit test on the `whose` handler asserts its SQL/inputs reference no tty/pane/title/env; a column outside that join entering the answer refutes A5.

---

## §2 — R1b (THE CRUX): per-human `principal_id` vs per-terminal session granularity

This is the single most important resolution in the SPEC. If P0 renders only the principal-derived handle, it does **not** retire the stated problem.

### 2.1 Why `created_by_principal_id` alone fails

Under RFC 0107 one human is one `principal_kind='human'` `principal_id` (`0023_principals.sql:30-36`). The fifteen terminals of one human therefore share **one** `created_by_principal_id`, and the suffix `#7f3` (computed from `principal_id`) is **identical** for all fifteen. So neither the column nor the suffix can answer "which of these windows owns run X." The disambiguator must be **per session**, not per principal.

### 2.2 The mechanism: per-session leased handle + live-unique partial index

`operator_handles` (owner bundle 0021, §4) leases a **word per session**:

```sql
CREATE TABLE striatumd.operator_handles (
  handle_id         text PRIMARY KEY,
  repository_id     text NOT NULL REFERENCES striatumd.repositories(repository_id),
  principal_id      text NOT NULL REFERENCES striatumd.principals(principal_id),
  handle            text NOT NULL,              -- lowercase, privacy-safe (curated pool, §5.1)
  leased_session_id text NOT NULL,
  leased_at         timestamptz NOT NULL,
  leased_until      timestamptz NOT NULL,       -- lazy-expiry TTL (mirrors striatumd.leases, 0005:166-186)
  last_heartbeat_at timestamptz,
  released_at       timestamptz,
  release_reason    text
);
-- live-uniqueness scoped to un-released rows: the disambiguator engine.
CREATE UNIQUE INDEX operator_handles_live_uq
  ON striatumd.operator_handles (repository_id, lower(handle))
  WHERE released_at IS NULL;
-- one live lease per session, so run -> session -> handle is 1:1 if ever needed.
CREATE UNIQUE INDEX operator_handles_live_session_uq
  ON striatumd.operator_handles (repository_id, leased_session_id)
  WHERE released_at IS NULL;
```

The partial-unique pattern is **already proven in-repo**: `striatumd.leases` uses `CREATE UNIQUE INDEX uq_active_resource_lease ... WHERE state = 'active'` (`0005:184-186`) and renews leases by extending a TTL (`last_heartbeat_at`/`expires_at`, `0005:166-178`). `operator_handles` is the same shape, scoped to `released_at IS NULL`.

Because the index constrains only un-released rows, two concurrent same-human sessions **cannot both hold `maya`**: the second `INSERT` of `lower(handle)='maya'` raises `23505 unique_violation` and the session escalates (§2.3).

### 2.3 The lease algorithm and the deterministic collision-escalation rule

A deterministic, principal-seeded candidate sequence drives both the default and the escalation:

```
seed       = fnv64a(principal_id)                       -- stable per human, per repo
candidates = [ POOL[(seed + k) mod len(POOL)] | k = 0,1,2,... ]   -- a principal-seeded walk of the curated pool
```

Lease acquisition (inside the token-mint transaction, §1):

```
for k in 0,1,2,...:
    w := candidates[k]
    -- lazy expiry (C-3): reclaim an abandoned word whose TTL lapsed, no background sweep.
    UPDATE operator_handles SET released_at = now(), release_reason = 'lease_expired_lazy'
      WHERE repository_id = $r AND lower(handle) = w AND released_at IS NULL AND leased_until < now();
    try:
        INSERT operator_handles(handle_id, repository_id, principal_id, handle=w,
                                leased_session_id, leased_at=now(), leased_until=now()+TTL);
        return w                                          -- leased
    catch unique_violation (23505 on operator_handles_live_uq):
        continue                                          -- w is live-held; try candidates[k+1]
```

- **Deterministic default.** A lone session always lands on `candidates[0]` = `POOL[seed mod len(POOL)]`. Stable across reconnect: a new `session_id` re-runs the same walk, finds `candidates[0]` free, re-leases it. *(R3-OQ1)*
- **Deterministic escalation.** A second concurrent same-human session finds `candidates[0]` live-held → `23505` → leases `candidates[1]` (a *distinct curated word*, e.g. `theo`, not a numeric `maya2` — numerals collide visually with the `#suffix`; the identical `#suffix` already signals "same human"). On the escalated session's own reconnect, while the first still holds `candidates[0]`, the walk re-lands deterministically on `candidates[1]`. **Stable across reconnect.**
- **The only relabel is convergent and harmless.** If the first session dies, `candidates[0]` frees; the escalated session, on its next reconnect, converges to `candidates[0]`. This changes only the **live** word for that window — it does **not** rewrite any run's attribution, because runs carry a **frozen** `created_by_handle_id` snapshot (§2.4) protected write-once (§4.3).
- **No deadlock, one winner.** Two sessions racing for `candidates[0]` each attempt one INSERT; the partial-unique index serializes them — exactly one commits, the loser catches `23505` and advances. No row is locked across the contention; there is no lock-ordering cycle.

### 2.4 The exact run → handle join (the decision R1b demands)

**Decision: the run carries a write-once snapshot of the creating session's lease — `runs.created_by_handle_id` (FK → `operator_handles.handle_id`)** — *in addition to* `created_by_principal_id`. `whose` joins through it; it does **not** join `run → created_by_principal_id → live lease` (that join is ambiguous — one principal has up to fifteen live leases).

Rationale for snapshot-by-`handle_id` over the alternatives:
- vs. `run → created_by_session_id → current lease`: a session could, in a later phase, re-lease a different word; joining to "the session's current word" would relabel history. The `handle_id` FK pins the *exact lease row* that was live at creation.
- vs. a denormalized `created_by_handle text` snapshot: a `handle_id` FK is **verifiable** against handle history (RFC D5's "independently verifiable against handle history") and never drifts. `operator_handles` rows are **retained** (no DELETE granted, §4.4; released rows keep `released_at` set), so the FK never dangles and the snapshot word is permanently stable.

`whose <run-id>` is the pure join that cannot lie:

```sql
SELECT r.run_id, r.state, r.created_by_principal_id,
       oh.handle AS origin_handle, p.principal_kind, p.disabled_at
  FROM striatumd.runs r
  LEFT JOIN striatumd.operator_handles oh ON oh.handle_id = r.created_by_handle_id
  LEFT JOIN striatumd.principals       p  ON p.principal_id = r.created_by_principal_id
 WHERE r.repository_id = $1 AND r.run_id = $2;
```

Render rule:
- `created_by_principal_id IS NULL` → bare `run_id` + advisory `attribution_unknown` (pre-cutover / unattributed; §5.2, §5 D7).
- else `word = COALESCE(oh.handle, defaultHandle(created_by_principal_id))`, `suffix = hexPrefix(fnv(created_by_principal_id))` (computed, not stored, per RFC D1), render `word#suffix` + run state/phase + a paste-able switch hint.

### 2.5 PROOF — two same-human terminals return two distinct answers

Human `H` → principal `P`; `defaultHandle(P) = candidates[0] = "maya"`.

1. Session `S1` registers → mint txn → lease walk: `INSERT maya` succeeds → `S1` holds `maya` (`handle_id = h1`).
2. Session `S2` registers → mint txn → lease walk: `INSERT maya` → `23505` (live-held by `S1`) → `INSERT candidates[1]="theo"` succeeds → `S2` holds `theo` (`handle_id = h2`).
3. `S1` creates run `RA` → stamp `created_by_principal_id = P`, `created_by_handle_id = h1`.
4. `S2` creates run `RB` → stamp `created_by_principal_id = P`, `created_by_handle_id = h2`.
5. `whose RA` → `oh.handle='maya'`, `suffix=#7f3` → **`maya#7f3`**.
6. `whose RB` → `oh.handle='theo'`, `suffix=#7f3` → **`theo#7f3`**.

**Two distinct answers** (`maya#7f3` ≠ `theo#7f3`). The **word** disambiguates the two windows; the **identical suffix** correctly signals "same human." Had the SPEC stamped only `created_by_principal_id` and rendered `defaultHandle(P)#suffix(P)`, both would render `maya#7f3` — indistinguishable, and P0 would fail. The per-session `created_by_handle_id` + the live-unique partial index is the fix, and the proof is the `two_live_maya` pgtest below.

### Falsifiable assertions

- **A6 (live-unique forces distinct words).** Two concurrent same-human sessions hold two distinct live words. *Refuting test:* `two_live_maya` (two-role pgtest, §4.5) — register two sessions for `P`; assert exactly one live row with `lower(handle)='maya'` and the second on a distinct word; a duplicate live `maya` or a deadlock refutes A6.
- **A7 (distinct `whose` answers).** `whose RA != whose RB` for the two-terminals case above. *Refuting test:* the §2.5 scenario asserts `maya#7f3` vs `theo#7f3`; equal answers refute A7 (this is the gate-critical sufficiency proof).
- **A8 (deterministic default, stable across reconnect).** A lone session for `P` always leases `candidates[0]`. *Refuting test:* lease → release → re-lease for the same `P`; a different word on reconnect refutes A8.
- **A9 (deterministic escalation, stable across reconnect).** While `candidates[0]` is held by a peer, a session deterministically (re)leases `candidates[1]`. *Refuting test:* hold `candidates[0]`; register + reconnect a second session; a non-`candidates[1]` word, or a different word across the reconnect, refutes A9.
- **A10 (no silent relabel of a live/past run).** A reconnect never rewrites a run's `created_by_handle_id`. *Refuting test:* create `RB` under `theo`; kill `S1` (frees `maya`); reconnect `S2` (converges live to `maya`); assert `whose RB` still renders `theo#7f3`. A changed answer refutes A10.
- **A11 (one winner, no deadlock under race).** Concurrent lease of `candidates[0]` yields exactly one holder and a clean escalation. *Refuting test:* concurrent two-session lease in the two-role fixture; a `40P01` deadlock, a duplicate, or both-fail refutes A11.

---

## §3 — R1c: lease flap (heartbeat renews, never release-then-reacquire)

### Design

Heartbeat is a **guarded UPDATE** of the existing row; it never deletes, never sets `released_at`, and never re-INSERTs:

```sql
UPDATE striatumd.operator_handles
   SET leased_until = now() + $TTL, last_heartbeat_at = now()
 WHERE handle_id = $1 AND leased_session_id = $2 AND released_at IS NULL;
```

The guard `leased_session_id = $2 AND released_at IS NULL` means only the **owning, still-live** session renews, and the row **never transits through a released state** during renewal — so `operator_handles_live_uq` never frees the word mid-flap and a racing same-human session cannot steal it. This mirrors the `striatumd.leases` renewal idiom (extend `last_heartbeat_at`/`expires_at`, `0005:166-178`), not a release/reacquire.

### Falsifiable assertion

- **A12 (flap-resistance).** A heartbeat renewal cannot let another session steal the word. *Refuting test:* `lease_flap_steal` (two-role pgtest) — `S1` holds `maya`; interleave `S1`'s renewal UPDATE with `S2`'s attempt to lease `maya`; assert `S2` always gets `23505` and escalates, and `S1`'s row was never `released_at`-set. A successful steal during the flap refutes A12.

---

## §4 — R2: the owner-bundle migration (the gating, hardest-to-reverse change)

### 4.1 Ordinal and placement

Owner bundle **0021** (next free ordinal after `LatestOwnerBundleVersion == 20`, `owner.go:23`). New file `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql`, auto-discovered by the embedded loader (`owner.go:157` `//go:embed sql/owner/*.sql`; registry `ownerBundleLabels` + `OwnerBundles()`, `owner.go:159-224`), plus a label entry `21: "RFC 0167 P0 operator identity + run attribution (operator_handles + runs.created_by_* write-once)"` and `LatestOwnerBundleVersion = 21`. It is an **owner** bundle because both `operator_handles` (a new owner-held table) and `ALTER runs` touch tables the runtime role does not own (C-1).

### 4.2 Bundle SQL (additive — no privilege-stripping REVOKE)

```sql
-- owner bundle 0021 — applied OUT-OF-BAND as the owner via `striatum daemon owner-ddl apply`, THEN restart.

-- (1) operator_handles: owner-held lease/rendering layer over principal_id (R4). Schema + indexes per §2.2.
CREATE TABLE IF NOT EXISTS striatumd.operator_handles ( ... );          -- §2.2
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_uq         ...;  -- §2.2
CREATE UNIQUE INDEX IF NOT EXISTS operator_handles_live_session_uq ...;  -- §2.2

-- (2) runs origin stamp (owner-held table -> owner bundle, C-1).
ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS created_by_principal_id text REFERENCES striatumd.principals(principal_id);
ALTER TABLE striatumd.runs
  ADD COLUMN IF NOT EXISTS created_by_handle_id text REFERENCES striatumd.operator_handles(handle_id);

-- (3) write-once enforced at the DB (chosen mechanism: BEFORE UPDATE trigger, §4.3).
CREATE OR REPLACE FUNCTION striatumd.refuse_run_origin_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.created_by_principal_id IS DISTINCT FROM OLD.created_by_principal_id
     OR NEW.created_by_handle_id  IS DISTINCT FROM OLD.created_by_handle_id THEN
    RAISE EXCEPTION 'runs.created_by_* origin stamp is write-once (set at run creation, immutable thereafter)';
  END IF;
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS runs_origin_write_once ON striatumd.runs;
CREATE TRIGGER runs_origin_write_once
  BEFORE UPDATE ON striatumd.runs
  FOR EACH ROW EXECUTE FUNCTION striatumd.refuse_run_origin_change();

-- (4) runtime-role DML on the new owner-held table (the 0005 ALL-TABLES grant predates it).
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.operator_handles TO striatumd_rw;  -- no DELETE: rows retained
  END IF;
END $$;

-- (5) watermark/capability stamp (mirrors every bundle, e.g. 0018:108-110).
INSERT INTO striatumd.schema_authority(capability, requires_daemon_auth, bundle_version)
VALUES ('operator_identity_run_attribution', false, 21)
ON CONFLICT (capability) DO NOTHING;
```

Precedents: `ALTER ... ADD COLUMN` on an existing FK-bearing owner table is owner-bundle 0009's pattern (`sql/owner/0009_review_generation.sql` adds columns to `jobs`/`verdicts`); the ownership/grant idiom and the `EXISTS striatumd_rw` guard mirror owner-bundle 0018 (`0018:77-110`).

### 4.3 Write-once enforcement: CHOSEN = BEFORE UPDATE trigger (not column REVOKE)

R2 demands a pick + justification. **Chosen: a `BEFORE UPDATE` trigger that raises when either origin column changes.** Rejected alternative: `REVOKE UPDATE ON runs FROM striatumd_rw; GRANT UPDATE (<every other column>) ON runs TO striatumd_rw`.

Justification:
1. **`runs` is actively UPDATEd on six columns/paths** — `state`/`started_at`/`paused_*`/`completed_at`/`stop_reason`/`branch_*` (`run.go:116-119,450-453,487-490,657-660,781-784,887-891`), including `branch_confirmed_by` *after* creation (`run.go:887-891`). A blanket `REVOKE UPDATE ON runs` is impossible, and the column-grant alternative must **enumerate ~15 columns** and be re-maintained on **every future `runs ADD COLUMN`** — forget to re-grant and the runtime silently cannot update the new column (or you must ship another owner bundle just to make a column runtime-writable). That is a standing footgun.
2. **Direct in-repo precedent.** `go/pkg/db/sql/0010_artifact_blob_update_trigger.sql:19-49` already enforces *column-selective immutability at the DB* via a `BEFORE UPDATE` trigger (`allow_artifact_blob_reference_update` raises unless only blob columns changed). The write-once trigger is the same idiom, simpler (deny exactly two columns, allow all others) — a reviewed pattern, not novel machinery.
3. **Stronger invariant.** The trigger refuses the change for **any** role and **any** path (a future app bug, a manual fix), where a column GRANT only constrains the runtime role's privilege surface. The runtime role is non-superuser, so it cannot `SET session_replication_role = replica` to bypass triggers.
4. **Keeps bundle 0021 purely forward-additive.** A trigger needs no `REVOKE`, so 0021 carries **no privilege-stripping REVOKE** and is **not** subject to the revoke-last watermark-ordering rule that REVOKE-bearing bundles (0003–0006) must honor. Simpler and safer to sequence.

The stamp itself happens at **INSERT** (`run.go` INSERT, §1), which does not fire the `BEFORE UPDATE` trigger; post-insert the columns are immutable. (`IS DISTINCT FROM` also forbids a later `NULL → value` UPDATE, consistent with the OQ2 "no backfill" decision, §5.2 — the only legitimate write is the creation INSERT.)

### 4.4 Privileges the runtime role must RETAIN (pinned)

- **`runs`** — `SELECT, INSERT, UPDATE, DELETE` unchanged from the existing table-level grant (`0005:467-475`). INSERT stamps the new columns once at creation; UPDATE continues all six state-transition paths; the trigger blocks only changes to the two origin columns. **No REVOKE is issued**, so no privilege is lost.
- **`operator_handles`** — `SELECT` (render/join for `whose`/`status`/`doctor`), `INSERT` (lease acquisition), `UPDATE` (heartbeat renewal §3 + graceful release in `closeRemainingSessions` + lazy-expiry reclaim §2.3). **No `DELETE`** — rows are retained so `created_by_handle_id` snapshots never dangle (append/retain semantics, matching the `events`/`artifacts` no-delete philosophy at `0005:457-465`).

### 4.5 Proof under the RFC 0142 two-role fixture; named pgtests

All DB-boundary tests use `pgtest.TwoRole(t)` (`two_role.go:47-78`): bundle DDL is applied via `OwnerPool` (non-superuser owner), runtime behavior is exercised via `SUTPool` (`striatumd_rw`). A single-role pgtest would mask `42501`/privilege gaps; `assertSQLState42501` (`two_role_pg_test.go:161-176`) is the privilege oracle.

1. **`owner_bundle_0021_applies_clean`** — apply 0021 via `OwnerPool`; assert `operator_handles` + both partial indexes exist, `runs` has both columns, the trigger exists; then as `SUTPool` assert the runtime role **can** `INSERT` a run carrying `created_by_principal_id`, `INSERT`+`UPDATE` `operator_handles` (lease + heartbeat), and **cannot** `ALTER TABLE striatumd.runs ...` (`assertSQLState42501`) — proving the runtime retains exactly its needed privileges and no more.
2. **`forged_update_created_by_rejected`** — as `SUTPool`, `UPDATE striatumd.runs SET created_by_principal_id = '<other>'` on a stamped run; assert it raises the trigger exception (write-once). *(Note: this is a plpgsql `RAISE`, SQLSTATE `P0001`, not `42501` — asserted with a sibling helper, not `assertSQLState42501`.)*
3. **`two_live_maya`** — the §2.5 collision/escalation + distinct-`whose` proof (A6, A7, A11).
4. **`token_revoked_bare_id`** — revoke the creating client (`clients.revoked_at`) and close its session (release the lease, `closeRemainingSessions` path); assert the *live*-identity render (`status --mine` "your handle this session") falls back to the **bare id** (no live lease → "the name lapses to the id"), while `whose <past-run>` still renders the **frozen** historical `word#suffix` (history is honest, not erased).
5. **`lease_flap_steal`** — A12 (§3).

### 4.6 Forward-only and watermark consistency

- **Forward-only.** `applyPendingOwnerBundles` applies only bundles `> current` and skips `<= current` (`owner.go:305-322`); each commits its `owner_bundle_meta` watermark in the same transaction as its DDL (`owner.go:528-532`). 0021 is forward-only by construction.
- **Advance `RequiredOwnerBundleVersion` to 21 — justified.** The new serving binary's `run.prepare` INSERT references `created_by_principal_id`/`created_by_handle_id` and the lease path references `operator_handles`; it **hard-depends** on bundle 0021's schema. So the deploy path *does* demand the binary refuse to serve until 21 is applied. Setting both `LatestOwnerBundleVersion = 21` and `RequiredOwnerBundleVersion = 21` makes `CheckOwnerBundleWatermark` (`owner.go:124-154`, run before `ApplyMigrations` at `connection.go:349-351`) halt cleanly (`AwaitingOwnerDDLError`, the RFC 0142 Layer-2 "apoptosis not necrosis" interlock) if the daemon restarts before `owner-ddl apply` ran 0021 — instead of a `42501` crash-loop.
- **Deploy ordering (apply THEN restart).** `striatum daemon owner-ddl apply --owner-url …` (`runDaemonOwnerDDL`, `go/pkg/cli/localcommands/daemon.go:84-159`, → `db.ApplyOwnerBundles` at `:131`) applies 0021 as the owner; then restart onto the new binary. Restart-first → applied `20 < required 21` → clean halt with the exact remediation command.

### Falsifiable assertions

- **A13 (owner-only ALTER).** A runtime-role `ALTER TABLE runs` fails `42501`; the same ALTER under the owner succeeds. *Refuting test:* `owner_bundle_0021_applies_clean` (SUT ALTER → `assertSQLState42501`). A runtime ALTER that succeeds refutes A13 (and the owner-bundle framing).
- **A14 (write-once at the DB).** No role-routed UPDATE can change a stamped origin column. *Refuting test:* `forged_update_created_by_rejected`. A successful UPDATE refutes A14.
- **A15 (retained privileges complete).** The runtime role can do everything P0 needs (INSERT run w/ column; lease + heartbeat + release on `operator_handles`) and nothing more (no `ALTER`, no `DELETE` on handles). *Refuting test:* the positive+negative assertions in `owner_bundle_0021_applies_clean`. A missing needed grant (a `42501` on a needed op) or a surplus grant refutes A15.
- **A16 (clean apply under non-superuser owner).** 0021 applies via the two-role `OwnerPool` with no privilege gap. *Refuting test:* `owner_bundle_0021_applies_clean` apply step; any `42501`/`must be member of role`/`permission denied for schema` during apply refutes A16.
- **A17 (forward-only).** Re-applying 0021 is a no-op; 0021 is never applied below its watermark. *Refuting test:* apply twice; a second-apply error or a watermark regression refutes A17.
- **A18 (watermark interlock).** A binary built against 21 refuses to serve on a DB at watermark 20 with `AwaitingOwnerDDLError`, DB untouched. *Refuting test:* boot the new binary against a 20-watermark DB; serving (or a half-applied runtime migration) refutes A18.
- **A19 (no revoke-last hazard).** 0021 carries no privilege-stripping REVOKE. *Refuting test:* grep 0021 for `REVOKE`; presence refutes A19 (and would re-engage revoke-last ordering).

---

## §5 — R3: resolve all four open questions

### 5.1 OQ1 — Handle pool, default, escalation, denylist → **IN P0**

**Decision:** a **curated lowercase first-names pool** (~256 neutral given names), privacy-safe and memorable, shipped as a Go slice in one package. **Default** = `POOL[fnv64a(principal_id) mod len(POOL)]` (deterministic from `principal_id`, not tty — the RFC rejects tty seeds; reconnect-stable). **Escalation** = the principal-seeded walk `POOL[(seed + k) mod len(POOL)]` to the next **distinct curated word** (not numeric suffixes — they collide with `#suffix`). **Denylist** = reserved words (`daemon`, `scheduler`, `system`, `admin`, `root`, `unknown`, `anon`, `none`, and the `principal_kind` names) are **excluded from the pool entirely**, so they can never be generated, and service/`ai_operator` principals (RFC D2's `scheduler#a19`) draw from a disjoint reserved sub-pool — a human can never be auto-assigned a service word. **Operator-chosen naming is deferred** (§7): the deterministic default + escalation already retires the problem; a naming UI + denylist-on-input is P2 polish, not P0.

- **A20.** *Refuting test:* a golden test pins `defaultHandle(P)` stable across runs and reconnects, asserts no pool word is on the denylist, and asserts escalation yields distinct words; instability or a denied word refutes A20.

### 5.2 OQ2 — Backfill vs NULL → **NULL + advisory `attribution_unknown`, in P0**

**Decision:** historical runs below the cutover keep `created_by_principal_id = NULL`; the advisory (non-red) doctor rule `attribution_unknown` surfaces them. **No backfill.** **Why:** `branch_confirmed_by` holds `'daemon'`/`'human'` literals (C-4), **not** a `principal_id` — backfilling from it would fabricate identity from a non-identity field, the precise dishonesty R1a forbids; a guessed origin is worse than an honest "unknown." This matches RFC D7's `attribution_unknown` (advisory, kept out of the hard-integrity lane so it never blocks dogfoods).

- **A21.** *Refuting test:* a doctor test asserts a NULL-`created_by_principal_id` non-terminal run yields `attribution_unknown` as **advisory** (not in the red/integrity set) and that no migration writes a non-NULL `created_by_principal_id` to a pre-cutover run. A red classification, or a backfill write, refutes A21.

### 5.3 OQ3 — Cross-repo board → **per-repo only in P0; daemon-wide DEFERRED (P3)**

**Decision:** P0 is **per-repo**. `operator_handles` is keyed `(repository_id, …)`, the live-unique index is per-repo, and `whose`/`status --mine` are per-repo (the existing `status` is `single_repo`-scoped, `routes_generated.go`). A daemon-wide "all my operators across all repos" board is **deferred to P3** (situational-awareness polish), because (a) it needs a cross-repo read surface that does not exist today, and (b) per-repo scoping is what bounds the misattribution blast radius (RFC 0107 namespacing — the same word on a different repo is intentionally fine). The stated problem ("15 terminals, which owns this run") is fully answered per-repo.

- **A22.** *Refuting test:* assert no P0 surface performs a cross-repo identity aggregation (every `operator_handles`/`whose`/`status --mine` query is `repository_id`-scoped). A cross-repo aggregation in P0 refutes A22.

### 5.4 OQ4 — `@handle#suffix` artifact byline → **OUT of P0 (lands in P2)**

**Decision (scope only, per SEED):** the artifact-byline suffix is **out of P0**. P0 delivers run-origin attribution (`created_by_principal_id` + `whose` + manifest); the durable-byline suffix touches the append-only owner-held `artifacts` anchor metadata and the RFC 0026 byline-honesty surface (a separate, larger change), and the RFC phases it to **P2**. This is a scope decision, not a P2 design.

- **A23.** *Refuting test:* assert P0 changes no artifact `author_line`/anchor-metadata derivation. A byline-suffix change in P0 refutes A23.

---

## §6 — R4: ride RFC 0107; do not rebuild it

- **Operator-id IS `principal_id`.** `operator_handles.principal_id` is an FK to `striatumd.principals(principal_id)` (`0023_principals.sql:30-36`); the table stores **no** identity — only `(repository_id, principal_id, handle, lease)`. It is a rendering/lease layer, nothing more.
- **Reuse, don't duplicate.** Client→principal dereference reuses the active-link join (`ResolvePrincipalForClient`, `principals.go:266-292`); the lease shares the **existing** session-bound token mint (`mintSessionBoundToken`, `session_token.go:60-97`) and the `sessions` table (`0005:44-70`); release reuses the **existing** session-close transaction (`closeRemainingSessions`, `mutations.go:1432`) plus lazy `leased_until` expiry (the `striatumd.leases` TTL pattern, `0005:166-186`). **No parallel identity table, no new reaper** (C-3).
- **Product-boundary clean.** No hosted service, directory, telemetry, or external identity; single-human/single-daemon legibility only. tty/tmux/title/env are never read for state (A2/A5). `run_id` stays opaque — the handle lives in a separate column, never encoded into `run_id`.

- **A24 (no parallel identity).** *Refuting test:* assert `operator_handles` has no `display_name`/`kind`/auth columns and that identity is only ever read from `principals`; an identity attribute on `operator_handles` refutes A24.
- **A25 (no new reaper).** *Refuting test:* assert release happens in `closeRemainingSessions` + lazy acquisition-path expiry, with no new background goroutine/scheduler; a new periodic reaper refutes A25.
- **A26 (opaque run_id).** *Refuting test:* assert `run_id` generation is unchanged and contains no handle; a handle-bearing `run_id` refutes A26.

---

## §7 — P0 boundary and seams for P1–P3 (noted, not designed)

P0 ships: `operator_handles` + live-unique index (§2.2); `runs.created_by_principal_id` + `runs.created_by_handle_id` write-once (§4); the operator-session mint+lease at bootstrap riding `mintSessionBoundToken` (§1); `whose <run-id>` (new read RPC, registered in `contracts/daemon_methods.json` + `routes` + `docs/reference/command-authority-matrix.md` with its `registry_contract_test` guardrail, §9); `status --mine` manifest section; the `attribution_unknown` advisory doctor rule (§5.2). Seams left, explicitly **not** designed here:

- **P1 (custody).** `run_custody_log` will **append in the same transaction** as the state transition that triggers it (reap/resume/requeue/spawn). P0 leaves the run-termination and recovery transactions untouched except for the graceful handle release; the custody append hooks the same chokepoints.
- **P2 (honest bylines + handoff naming + chips + OSC title).** OQ4's byline suffix (§5.4), the `handle → {color, glyph}` chip function, `handoff_filename`, and the opt-in OSC-2 title land here; P0's `whose`/manifest deliberately render the bare `word#suffix` so P2 can layer chips without re-plumbing identity.
- **P3 (lineage + cross-repo board).** `runs.lineage_id` and OQ3's daemon-wide board (§5.3); P0's opaque `run_id` and per-repo scoping leave room for both.

---

## §8 — Consolidated falsifiable-assertion ledger

| # | Claim | Supporting evidence (anchor) | Refuting observation / named test |
|---|-------|------------------------------|-----------------------------------|
| A1 | Stamp = live-token principal, server-side | `run.go:1056-1074` INSERT subquery; `authority.go:116-120` | forged param / spoof leaks into stamp — `run_attribution_pg_test` |
| A2 | No client-name path to attribution | `run.go:21-28`; `lifecycle.go:71,114-121` | grep finds `created_by`/handle param feeding stamp/lease |
| A3 | Mint+lease atomic | `session_token.go:48-53`; `lifecycle.go:79,417` | token row committed without lease row |
| A4 | Identity from validated token only | `auth_pg.go:49-157,87-92`; `authority.go:75-85` | revoked token still creates a stamped run |
| A5 | Read surfaces cannot lie | §2.4 join; `whose` SQL | tty/pane/title/env in the authoritative answer |
| A6 | Live-unique forces distinct words | `operator_handles_live_uq` (§2.2); `0005:184-186` precedent | duplicate live `maya` / deadlock — `two_live_maya` |
| A7 | Two terminals → distinct `whose` | §2.5 proof | `whose RA == whose RB` (gate-critical) |
| A8 | Deterministic default, reconnect-stable | §2.3 walk; `fnv64a(principal_id)` | different word on reconnect |
| A9 | Deterministic escalation, reconnect-stable | §2.3 walk | non-`candidates[1]` / drift across reconnect |
| A10 | No silent relabel | write-once `created_by_handle_id` (§4.3) | `whose RB` changes after peer death/reconnect |
| A11 | One winner, no deadlock | partial-index serialization (§2.3) | `40P01` / duplicate / both-fail |
| A12 | Flap-resistant renewal | guarded UPDATE (§3) | steal succeeds during flap — `lease_flap_steal` |
| A13 | Owner-only ALTER | C-1; `0018:8-22`; `owner_runtime_ownership.go:8-11` | runtime ALTER succeeds |
| A14 | Write-once at the DB | trigger (§4.3); `0010:19-49` precedent | UPDATE changes a stamped column — `forged_update_created_by_rejected` |
| A15 | Retained privileges exact | §4.4; `0005:467-475` | needed op `42501` / surplus grant |
| A16 | Clean apply, non-superuser owner | two-role `OwnerPool` (`two_role.go:47-78`) | `42501`/`must be member`/schema-perm on apply |
| A17 | Forward-only | `owner.go:305-322,528-532` | second-apply error / watermark regression |
| A18 | Watermark interlock | `owner.go:124-154`; `connection.go:349-351` | serves on 20-watermark DB |
| A19 | No revoke-last hazard | §4.2 (no REVOKE) | `REVOKE` present in 0021 |
| A20 | Pool/default/escalation/denylist | §5.1 golden test | unstable default / denied word generated |
| A21 | NULL + advisory, no backfill | §5.2; C-4 | red classification / backfill write |
| A22 | Per-repo only in P0 | §5.3; `single_repo` scope | cross-repo aggregation in P0 |
| A23 | Byline suffix out of P0 | §5.4 | `author_line` change in P0 |
| A24 | No parallel identity table | §6; `0023_principals.sql:30-36` | identity attribute on `operator_handles` |
| A25 | No new reaper | §6; `closeRemainingSessions` `mutations.go:1432` | new periodic session reaper |
| A26 | Opaque run_id | §6 | handle encoded into `run_id` |

---

## §9 — Build manifest (P0 scope, for the downstream `code_change` run)

1. **Owner bundle** — `go/pkg/db/sql/owner/0021_operator_identity_run_attribution.sql` (§4.2); `owner.go` label entry + `LatestOwnerBundleVersion = 21` + `RequiredOwnerBundleVersion = 21` (§4.6).
2. **Lease layer** — a Go package owning `defaultHandle`/escalation walk (§2.3, §5.1 pool), lease acquisition + guarded heartbeat renewal (§3), graceful release wired into `closeRemainingSessions` (`mutations.go:1432`); lease acquisition wired into the operator-session mint at bootstrap and into `HandleRegisterSession`'s txn (`lifecycle.go:79`).
3. **Run stamp** — extend the `runs` INSERT (`run.go:1056-1074`) with the two server-side subqueries (§1); resolve `client_id → principal_id` per C-2.
4. **Operator-session mint** — bootstrap daemon RPC reusing `mintSessionBoundToken` (§1 seam), so `run.prepare` carries a session-bound token (`app.session_id` populated).
5. **`whose <run-id>`** — new read handler (§2.4 join) + `contracts/daemon_methods.json` + regenerated routes + `docs/reference/command-authority-matrix.md` row + `registry_contract_test` (per Change Discipline).
6. **`status --mine`** — manifest section + flag (`status.go`, `resolveStatusRunScope` :52), live-handle render with bare-id fallback (§4.5 test 4).
7. **Doctor** — `attribution_unknown` advisory rule (§5.2), following the `doctor_artifact_anchor.go` advisory pattern.
8. **pgtests** — the five named two-role tests (§4.5) + the §2/§3 assertions, all on `pgtest.TwoRole`.
9. **Docs** — update `docs/decisions/decision-log.md`, `docs/reference/spec.md`, `CHANGELOG.md`, and re-triage `docs/operator/rfc-roadmap.md` when P0 ships.

This is the published claim. Falsifiers: the gate-critical targets are **A7** (R1b sufficiency proof — two distinct `whose` answers) and **A13/A14/A16** (R2 owner-bundle two-role safety + DB write-once). The four anchor corrections in §0 are load-bearing; challenge them at source if you believe any is wrong.
