---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["threat_model", "rfc-0042", "track-c", "design"]
---

author: reviewer-claude-opus-002

# Track C Design Review — Threat Model (RFC 0042: Repo-Local State → Postgres)

This is a fresh-context threat-model review of
`docs/dogfood/042/track_c/DESIGN_SYNTHESIS.md`. The four review criteria are
migration safety, schema integrity, audit-chain integrity preservation, and
the defensibility of the daemon-mandatory boundary. The trust-boundary
baseline is D083 (single OS user per machine, daemon socket owner-only,
capability tokens scoped per repository).

## 1. Trust Boundaries Enumerated

The synthesis introduces or shifts the following trust boundaries. Each is
named here, then the rest of the review judges whether it is acknowledged
and mitigated.

| # | Boundary | Direction of shift |
|---|---|---|
| TB-1 | CLI ↔ daemon (Unix socket, envelope-v1) | Tightened: every state-touching CLI verb now crosses this boundary; no SQLite-direct fallback. |
| TB-2 | Daemon process ↔ Postgres (TCP/Unix to `striatumd.*`) | Unchanged from RFC 0033, but the data surface protected by this boundary now includes every formerly-repo-local table. |
| TB-3 | Daemon process ↔ source `.striatum/state.sqlite3` | New: only the daemon process opens SQLite, and only inside one RPC verb. CLI never opens it. |
| TB-4 | Migration role ↔ append-only triggers on `striatumd.events` / `striatumd.artifacts` | New: the migration code path holds `UPDATE`/`DELETE` grants that runtime daemon clients lack. |
| TB-5 | `striatumd.repositories` row ↔ `repo_local_state_storage` setting | New: the bit that decides "this repo is migrated, refuse old code paths." |
| TB-6 | Per-repo audit chain ↔ format-version bump (v1 → v2) | New seam: `hash_format_version` discontinuity between historical and post-RFC-0042 rows. |
| TB-7 | `admin` capability ↔ `repo.migrate_local_state` (incl. `--force-replace-pg`) | New: admin capability becomes the only switch that can destroy a migrated repo's live state without going through the documented "full revert." |
| TB-8 | Cross-repo coordination tables ↔ per-participant composite FKs | Tightened: a participant repo cannot resolve cross-repo references unless its workflow-state has been migrated, refusing with exit 15. |

## 2. Migration Safety (Posture: rollback path defensible?)

### 2.1 Strengths

- **Daemon-only SQLite reader (TB-3).** §3.1 explicitly removes the CLI's
  access to the source file: "The CLI process never opens the SQLite file.
  The daemon process opens the SQLite file." This collapses the historical
  two-writer surface (CLI direct + daemon) into one process and one RPC
  call, gated by the `admin` capability.
- **Preflight order is correct.** §3.2 lays out path canonicalization →
  registration check → OS-owner check → destination preflight → source
  preflight (`PRAGMA integrity_check`, `PRAGMA user_version`) → fingerprint
  → optional dry-run → import. Each refusal point has a named exit code
  (14, 11, 16, 17, 18, 19) and a remediation string.
- **Source fingerprinting (TB-3, TB-6).** `source_sqlite_sha256` of file
  bytes + `source_events_rollup_sha256` of the canonical CSV gives the
  post-migration verifier two independent values to compare. With
  `--keep-sqlite-readonly`, doctor gets a third independent computation
  off the tombstone (§8).
- **Per-table transaction discipline.** §3.2 step 8 imports each table
  under its own transaction in FK order. A mid-table crash rolls back
  that table; the destination cannot end up with parent-rows-without-children
  or vice versa at table granularity. The advisory lock keyed on
  `repository_id` prevents two concurrent migrations of the same repo.
- **Cutover sentinel + Postgres row are dual-record (TB-5).** The
  `striatumd.repo_state_migrations` row is the authoritative "this repo
  is migrated" record. The SQLite-side `cutover_completed_at` sentinel
  is defense-in-depth so that a stray daemon binary opening the file
  refuses to mutate it. The mismatch case is named: exit 17
  `pg_repo_drift_detected`.
- **Drift detection has a verb.** `daemon doctor --repo <path>
  --check-migration` (§8) is named, scoped, and produces exit 17 on
  disagreement. This converts a silent corruption mode into a refuse-cleanly
  diagnostic.
- **`admin` capability gates the verb.** §3.1 names `admin` as the
  required capability. Other capability classes (`write`, `claim`,
  `review`, `apply`, `recovery`) cannot reach the migration RPC.

### 2.2 Findings — Migration Safety

- **F1 (low / clarity).** *Resumability vs idempotency contradiction.*
  §3.2 step 4 says "refuse if any row exists in any migrated table for
  this `repository_id` unless `--force-replace-pg` is set," but §7.1 says
  "The verb may be re-run; it restarts from the first table whose
  imported count does not match the source." If a mid-table crash leaves
  N tables fully imported and N+1 partially imported, the next invocation
  hits §3.2 step 4 ("rows exist for this `repository_id`") before it
  reaches §7.1's restart logic. The RFC body should resolve which rule
  wins for an *in-flight* migration (i.e. `repo_state_migrations.migration_completed_at
  IS NULL`) vs a *completed* migration (committed `migration_completed_at`).
  Suggested rule: when `migration_completed_at IS NULL`, retry without
  `--force-replace-pg` resumes from the first under-count table; otherwise
  §3.2 step 4 refuses.
- **F2 (low / spec precision).** *Canonical CSV format.* §3.2 step 6 and
  §8 both refer to "canonical CSV of the `events` rows in `event_id`
  order." Without a precise spec (column order, quoting, NULL encoding,
  line terminator), the two sides (source SQLite reader, PG-side
  recomputation) can disagree and surface a spurious exit 17. The RFC
  body should pin the canonical encoding (e.g. NDJSON of a fixed key
  order with `LF` terminators, or RFC 4180 CSV with column list explicitly
  enumerated).
- **F3 (low / threat surface).** *Migration-role privilege surface (TB-4).*
  Append-only triggers refuse `UPDATE`/`DELETE` for daemon-runtime clients
  but grant the migration role those rights. If the migration role's
  credentials live in long-lived configuration or are reachable from a
  non-migration code path, the append-only invariant collapses for that
  repo. The RFC body should require that the migration role's grants are
  scoped to the migration session only (e.g. a `SECURITY DEFINER` function
  the `admin`-capability RPC invokes, with no role membership granted at
  rest) and that `daemon doctor` confirms no runtime role can `UPDATE`
  or `DELETE` events/artifacts.
- **F4 (low / threat surface).** *Admin-capability as the new
  destruction switch (TB-7).* `--force-replace-pg` "wipes the Postgres
  rows for this `repository_id`" and archives the prior
  `repo_state_migrations` row. An AI agent that obtains an `admin`
  capability token (e.g. via an over-broad operator grant) can wipe a
  migrated repo's live state in a single RPC. D083 says the operator is
  trusted, but the synthesis should explicitly name the `admin` capability
  as the new high-blast-radius credential and recommend that operator
  workflows refuse to issue `admin` tokens to lane processes by default.
  The `repo_state_migrations_history` archive helps post-incident
  forensics but does not prevent the destruction.
- **F5 (low / clarity).** *Identity of `process_supervisor_pointers`.*
  §2.2 reads "`striatumd.process_supervisor_pointers` (added by repo-local
  migration v13 but already daemon-shaped) keeps its existing daemon-side
  definition and gains `(repository_id, run_id, session_id)` in its
  primary key if it does not already carry repository scope explicitly."
  The conditional "if it does not already" is unresolved. The current
  source state (`src/striatum/migrations.py`) should be inspected and the
  RFC body should commit to a single statement of whether the column is
  added or already present. This affects which composite-FK references in
  the rest of the schema validate at migration time.

## 3. Schema Integrity (composite-key isolation)

### 3.1 Strengths

- **Composite PK as containment.** §2.3's rule that "a code path that
  forgets to filter by `repository_id` raises an SQL error rather than
  silently returning the wrong repo's row" is the right structural
  defense for TB-2. Composite intra-table FKs make a wrong-repo join a
  type error, not a silent leak.
- **All hot-path indexes lead with `repository_id`.** Per-repo dashboard
  queries cannot accidentally scan another repo's rows; partial indexes
  on `(repository_id, state)` keep planner shape predictable per repo.
- **Cross-repo coordination tables stay global with composite participant
  FKs (TB-8).** `cross_repo_run_repositories.local_run_id` becomes
  `(repository_id, run_id)` referencing `striatumd.runs`. A cross-repo
  workflow cannot dangle a participant reference into a non-existent
  repo, and an unmigrated participant produces exit 15.
- **`ON DELETE RESTRICT` on `repository_id` FKs.** Repo removal never
  silently cascades; the documented `striatum daemon repo remove` flow is
  the only path that drops live state. This blocks an attacker who
  obtains `DELETE` on `striatumd.repositories` from chain-deleting all
  workflow state for free.
- **Datatype upgrades (booleans, `timestamptz`, `jsonb`, `bigint`).** No
  ambiguity-preserving casts that could change semantics under MVCC.

### 3.2 Findings — Schema Integrity

- **F6 (low / threat surface).** *Append-only enforcement under MVCC.*
  §2.3 says append-only is "enforced under Postgres roles plus row-level
  triggers refusing `UPDATE`/`DELETE` for daemon-runtime clients."
  Row-level triggers run inside transactions; a `SET LOCAL session_authorization`
  or `SET ROLE` to the migration role inside a runtime transaction would
  bypass them. The RFC body should require that the runtime daemon
  connection pool cannot `SET ROLE` to the migration role, or that the
  trigger asserts on `current_user` rather than the session role. Doctor
  should verify the connection-pool role lacks the migration grants.
- **F7 (low / clarity).** *`event_id` cardinality and the `bigserial`
  cutover.* §2.3 says "`event_id` becomes `bigserial` scoped to one
  daemon DB (the canonical key is `(repository_id, event_id)`)." For
  imported rows, the preserved primary identifier is the *SQLite*
  `event_id`, but `bigserial` assigns *new* values on insert. The RFC
  body must specify: imported events insert with `INSERT ... OVERRIDING
  SYSTEM VALUE` to preserve `event_id`, and the sequence is `setval`-bumped
  past `max(event_id)` per repo so post-migration events do not collide.
  Without this, doctor's PG-side rollup recomputation (§8) reads the wrong
  `event_id` ordering and exit 17 fires.

## 4. Audit-Chain Integrity Preservation

### 4.1 Strengths

- **Two-chain framing is explicit (§8).** The daemon hash chain
  (`striatumd.audit_log`) and the per-repo causal chain (`events`) are
  handled separately, with the synthesis acknowledging that the SQLite
  `events` table was never hash-chained and so cannot be "made into" one
  retroactively. Pragmatic and honest.
- **`hash_format_version` bump preserves chain continuity (TB-6).** §2.5
  spells out that v1 row hashes are not recomputed and the first v2 row
  links to the last v1 row's hash via `previous_hash`. The chain remains
  walkable across the format-version bump.
- **`migration_provenance` JSONB on imported events.** §8 records
  `source_sqlite_sha256`, `source_event_id`, and `migration_audit_id` per
  imported row. This converts the unhashed source events into hash-anchored
  rows post-migration: any single imported event can be cross-validated
  against the audit row that imported it.
- **Per-table audit row + summary row.** §3.2 step 10 appends one
  `repo.migrate_local_state` audit row per migrated table plus a summary
  row. Each row carries `repo_local_state_touched = true`,
  `params_sha256 = sha256(table_name)`, and metadata-only counts.
  Sensitive payload bodies (artifact bytes, prompts, blocker descriptions)
  are explicitly excluded.
- **Doctor verifies both directions.** `daemon doctor --check-migration`
  recomputes `source_events_rollup_sha256` PG-side, optionally tombstone-side,
  and walks the audit chain back from the summary row. Tamper-evidence is
  bidirectional.

### 4.2 Findings — Audit Chain

- **F8 (low / spec precision).** *PG-side rollup must read imported rows
  by `source_event_id`, not `event_id`.* See F7. The synthesis says "recomputes
  `source_events_rollup_sha256` on the Postgres side from the imported
  rows," but without explicit ordering-by-`migration_provenance->>'source_event_id'`,
  the recomputation diverges from the source CSV's ordering. The RFC body
  must pin this.
- **F9 (low / threat surface).** *No protection against `migration_audit_id`
  rewrite in `migration_provenance`.* `migration_provenance` is a `jsonb`
  column on a row whose `UPDATE` is blocked by triggers. As long as F6 is
  resolved, this is fine; if F6 is not resolved, a role with `UPDATE` on
  events can rewrite `migration_provenance` to point at the wrong audit
  row, breaking doctor's cross-link.
- **F10 (low / clarity).** *Hash-chain continuity audit at the v1→v2
  seam.* The synthesis asserts "v1 row hashes are not recomputed and the
  first v2 row links to the last v1 row's hash via `previous_hash`," but
  does not require that doctor explicitly verifies this. A regression in
  the audit-append code path could silently rebase v2 rows onto a wrong
  v1 anchor. The RFC body should require an acceptance test that walks
  the chain across the seam and asserts the v1 anchor hash matches a
  pre-RFC-0042 snapshot.

## 5. Daemon-Mandatory Boundary (TB-1)

### 5.1 Strengths

- **`--no-daemon` is deleted, not deprecated.** §5 names removal of the
  flag, its parser entry, and all branches keyed on it. This is the
  containment direction: the surface goes away, not just the docs.
- **Refuse-cleanly contract is fully enumerated.** §5 gives exit codes
  11/12/13/14/15/16/17/18/19, each with a class name, a meaning, and a
  named remediation. The `--json` envelope format is specified.
- **Read verbs route through the daemon.** §5 explicitly closes the
  "but `dashboard --once` could read SQLite directly" loophole by saying
  status/dashboard/audit show/why/run summary all go through RPC. The
  read-only direct-Postgres path is deferred (§11), not snuck in.
- **Pre-handshake whitelist is small.** Only `daemon start`, `daemon
  doctor`, and `daemon repo add` are allowed before the envelope-v1
  handshake. The third one is the registration on-ramp, which is the
  minimum necessary surface to bootstrap. This narrow whitelist limits
  the unauthenticated CLI surface.
- **No silent fallback on version mismatch.** §5 says the handshake
  failure produces exit 12 with no fallback; this matches RFC 0030's
  version-skew protocol and closes the "best-effort write" hole.
- **First-daemon-start refuses, doesn't auto-import (§3.3).** Unmigrated
  repos move to `state = 'needs_migration'` and any RPC method that
  targets them returns exit 15. Auto-migration is explicitly deferred to
  a follow-up. This is the threat-model-preferred choice: silent
  import is the failure mode that would hide a tampered SQLite file or
  an unintended cutover.
- **Go-daemon parity from day one (§6).** RFC 0039 is amended so the Go
  daemon implements the full repo-local verb table from Step 3/4, not
  later. The RFC 0035 multi-repo harness exercises both cores. This
  closes the split-brain window where the Go daemon owns registry but
  the Python CLI still owns repo-local SQLite.

### 5.2 Findings — Daemon-Mandatory Boundary

- **F11 (low / threat surface).** *Daemon socket permissions are asserted
  but not enforced in the migration verb.* §4 says doctor verifies
  `daemon.sock` is owner-only `0600`. The migration verb itself relies
  on the daemon process to be the OS owner (D083) but does not require
  the *CLI process* to verify socket ownership before connecting. A
  socket-impersonation attack (e.g. `STRIATUM_DAEMON_SOCKET` pointing at
  a non-daemon listener) could intercept the migration RPC. The RFC body
  should require the CLI to `stat()` the socket and refuse if the owner
  is not the expected operator UID, mirroring the doctor check.
- **F12 (low / clarity).** *Pre-handshake `daemon repo add` registers
  before authentication.* The pre-handshake whitelist (§5) allows `daemon
  repo add` so an operator can bootstrap. If `daemon repo add` performs
  any write to `striatumd.repositories` before envelope-v1 capability
  gating, an unauthenticated client on the socket could register
  arbitrary repos. The RFC body should clarify that `daemon repo add` still
  performs the OS-owner check (D083) before any write, and that it does
  not require a capability token because the OS-owner check substitutes
  for one.
- **F13 (low / threat surface).** *Capability-token scoping under
  cross-repo verbs.* §6's capability matrix says `worktree.*` requires
  `apply`, `recovery.*` requires `recovery`, etc., each scoped per
  `repository_id`. Cross-repo verbs (`run.prepare` against a multi-repo
  workflow) require a capability for every participant. The synthesis
  does not explicitly state this; without it, a token scoped to repo A
  could trigger work in repo B by naming it as a cross-repo participant.
  The RFC body should require: cross-repo RPC methods enforce capability
  presence for *every* named participant `repository_id`, not just the
  initiator.

## 6. Attack Surfaces Acknowledged But Out Of Scope

The synthesis correctly defers the following from V1 (§11). They are noted
here so the deferrals are intentional, not accidental:

- **Per-user auth, ACLs, shared-workstation hardening.** Out of scope per
  D083. The threat model relies on single-OS-user trust.
- **OS-keyring redesign, token-lifecycle redesign.** Capability-token
  storage and rotation are inherited unchanged from RFC 0030/0033.
- **Hosted mode / remote transports.** Owner-local Unix socket only.
- **Bundled or Dockerized Postgres.** Operator-managed Postgres remains
  the assumption; daemon doctor checks reachability and role privileges.
- **Read-only direct-Postgres path for CI.** Closing this loophole now
  preserves the "one substrate, one gateway" invariant; a follow-up RFC
  can reopen it under explicit read-only role gating.

These deferrals are defensible because each preserves the boundary RFC
0042 is trying to close. Opening any of them in V1 would reintroduce a
mutation or unmediated-read surface that the rest of the design exists to
remove.

## 7. Verdict

**Verdict: `accept_with_findings`.**

The synthesis is structurally sound on every primary threat-model criterion:
the daemon-mandatory boundary is defensible, composite-key schema isolation
gives compile-time-shaped containment of cross-repo leaks, the audit-chain
extension preserves continuity across the format-version bump, and the
migration verb has named refuse-cleanly behavior at every preflight gate.
The thirteen findings above are precision-and-clarity asks for the RFC body
(the next-stage deliverable), not architectural defects in the synthesis
itself.

The two findings worth the implementer's most careful attention are F1
(resumability vs idempotency contradiction — affects operator UX during a
mid-migration crash) and F3+F6+F9 (the migration-role privilege surface and
its interaction with append-only triggers — affects audit-chain integrity
under a compromised migration credential). These should be resolved in the
RFC body before implementation begins; they do not require redesigning the
synthesis.

The acceptance verdict acknowledges each enumerated trust boundary (TB-1
through TB-8) as either preserved (TB-2 unchanged from RFC 0033), tightened
(TB-1, TB-3, TB-8), or newly introduced with mitigation (TB-4 via F3/F6,
TB-5 via dual-record sentinel + Postgres row, TB-6 via explicit chain-link
spec, TB-7 via `admin` capability gating).
