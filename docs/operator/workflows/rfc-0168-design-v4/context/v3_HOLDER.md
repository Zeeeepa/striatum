# HOLDER — RFC 0168 P0 falsifiable implementation spec (REVISION v3: discharge the TWO v2 residual holes — the scrub-postcondition process predicate (C1-RESIDUAL) and the ACL provisioning transition (C2-RESIDUAL) — carrying the v1+v2 hard core and the credited structural halves forward unregressed)

author: holder-author-002

> This is the **v3 revision** of the RFC 0168 P0 `falsification_gate` proposal. The
> base is the **v2 SPEC** (`docs/operator/workflows/rfc-0168-design-v3/context/v2_HOLDER.md`);
> this is a revision of it, **NOT a rewrite, and NOT a revision of the v1 base**. v1
> **proved the structural hard core** (a per-lane uid dissolves `BC1-W1-ORACLE` on this host
> under Yama `ptrace_scope=1`) and resolved OQ1/OQ3/OQ5/OQ6. v2 **credited the structural
> halves of both binding constraints** — the C1 durable four-state lease machine and the C2
> final `.striatum/`-excluding ACL end-state — but the v2 cycle-1 adjudicator returned
> `needs_revision` on **two narrow, source-anchored residual holes**
> (`OQ2-SCRUB-POSTCONDITION`, `OQ4-ACL-PROVISIONING-TRANSITION`;
> `v2_COLLABORATION_LEDGER_cycle_1.md` findings, authoritative). This revision **discharges
> the two residual holes** — a tightening of **two predicates** (the scrub postcondition's
> process classifier P1, and the ACL provisioning procedure) — and **carries forward,
> unregressed, everything v1 AND v2 cleared.** The direction (per-lane pooled OS uid) is
> ratified (D261, 2026-06-24) and is **not** relitigated. Every source citation below was
> re-verified against the current worktree HEAD (`977f19a5`) while authoring this revision (see
> §Source re-verification); where a carried citation had gone stale this revision pins the
> **live** site — specifically the v2 ledger's binary `/proc` classifier (`processZombie` is now
> `tmux_liveness.go:599-614`, not `:576-591`) **and** the recovery-sweep / boot-epoch citations,
> which drifted because `recovery.go` was split into `recovery_auto.go` / `recovery_lease_expiry.go`
> / `recovery_decision_tree.go` / `recovery_liveness_oracle.go` since v2 (the v2
> `recovery.go:553/:2516/:565-587` and `main.go:665-690/:722/:731` numbers no longer resolve; the
> live sites are pinned below). This is the published claim the two v3 falsifiers re-attack.

---

# Addressing the v2 residual constraints (the auditable revision map)

| v2 verdict ground | This revision | Where |
| --- | --- | --- |
| **C1-RESIDUAL / `OQ2-SCRUB-POSTCONDITION`** — the v2 scrub-postcondition proof predicate **P1** blocks `returned` only on a `pool_uid`-owned process in `R`/`S`/`D` and records `Z` zombies without blocking, so a uid-owned task left in **`T`** (stopped) or **`t`** (tracing-stop) — which has **not** exited, still holds the uid, memory, FDs, and HOME/credential reachability, and is `SIGCONT`-resumable — **passes P1**, the proof finalizes `returned`, and the dirty uid is re-leased = the exact same-uid cross-lease residue leak RFC 0168 exists to close. Buildable-critical: the existing `processZombie` helper (`go/pkg/supervisor/tmux_liveness.go:599-614`) classifies `/proc` state as **binary `Z`-or-not** (and returns *not-zombie* on a read error), so an implementer wiring P1 off it inherits the leak. | **RESOLVED** — P1 is re-stated as a **fail-closed three-way classification**: tolerate **only** true zombie/dead tasks that cannot execute and hold no resources (state ∈ {`Z`,`X`,`x`}, or a PID that has fully left the domain), and **block** on **every** other observed state — `R`/`S`/`D`/`T`/`t`/`I`/`W`/`P`/`K`/…, an **unrecognized** state char, **and an unreadable/ambiguous** `/proc` read — finalizing `quarantined` with a typed `lane_uid_scrub_failed`. The proof is wired to a **new** classifier `classifyPoolUIDTaskState` that is **explicitly NOT** the binary `processZombie`; observed PIDs + `/proc` state chars + classification are recorded in `scrub_proof` so `doctor` distinguishes tolerated `Z` residue from a non-zombie quarantine cause. New negative test **A21 `TestStoppedOrTracedUIDProcessBlocksReturn`**. | **§OQ2.3 P1 (tightened)**, **§OQ2.6 (the classifier)**, test **A21** |
| **C2-RESIDUAL / `OQ4-ACL-PROVISIONING-TRANSITION`** — the v2 SPEC gated only on the auditable **end-state** and **blessed** `setfacl -R -m g:striatum-lanes:rX <repoRoot>` then a strip over `.striatum/`. But the first `-R` necessarily adds group `r` to existing `.striatum/` `0600` control-plane files (the MCP bearer `mcpconfig.go:241/266`; PTY logs `loop.go`) **before** the strip, so on a live repo `.striatum/` is **transiently group-readable** and a bearer/PTY-log read in that window is **irreversible** cross-lane exfiltration replayable as another lane; the A16 after-state test misses it. | **RESOLVED** — the **allowlist / exclude-at-traversal** provisioning form is now **MANDATORY**: the group grant **NEVER** applies `g:striatum-lanes:rX` (or its default) to `.striatum/`, `.git/`, or provider/token-cache paths, **even transiently**; `setfacl -R …:rX <repoRoot>`-then-strip is **explicitly FORBIDDEN** as a live-repo path. A16 is extended into the **transition** test **A22 `TestPoolACLProvisioningNeverTransientlyExposesScratch`** (an unleased **and** a different-leased pool uid attempt `open(2)`/traversal **before / during / after** provisioning; no read ever succeeds), and a deterministic **ACL-planner guard** + unit test **A23 `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists`** fails if any `g:striatum-lanes` op targets `<repoRoot>` (or any ancestor of `.striatum/`) as a raw recursive root while `.striatum/` exists. | **§OQ4.1 (procedure made mandatory)**, **§OQ4.3 (the planner guard)**, tests **A22 / A23** |
| **Carried — v2 CREDITED structural half of C1** (the durable lease machine) | **INTACT, restated, unregressed** | **§OQ2.1/.2/.4/.5** |
| **Carried — v2 CREDITED structural half of C2** (the final `.striatum/`-excluding ACL end-state + the per-leased-uid layout) | **INTACT, restated, unregressed** — the **end-state invariant is unchanged**; only the **procedure that reaches it** is now constrained | **§OQ4.1(b)/.2** |
| **Carried — v1 PROVEN hard core `HC-A1..A5`** | **INTACT, restated, re-verified** against `pty.go`/`tmux_liveness.go` HEAD | **§Part 1** |
| **Carried — v1+v2 credited `OQ1`/`OQ3`/`OQ5`/`OQ6` + the NARROWING invariant** (and the v1 OQ1 exhaustion-accounting caveat + OQ6 stale-store contingency, both **closed** by C1) | **INTACT, restated, unregressed** | **§OQ1/OQ3/OQ5/OQ6** |

The two predicate tightenings (OQ2.3-P1 + the new classifier; OQ4.1 mandatory procedure + the
planner guard) below are the only new load-bearing claims. **Everything else is the v2 text
carried verbatim in substance** (the v3 falsifiers have v2 as required context); a regression
in any carried claim is itself a gate failure. Neither residual fix touches the launch path
(Part 1), the lease state machine (OQ2.1/.2/.4/.5), the **final** ACL end-state (the OQ4
invariant), or OQ1/OQ3/OQ5/OQ6 — both are **narrowings** that only **remove** the residual
surface the v2 adjudicator named.

---

# Part 1 — THE HARD CORE (CARRIED FORWARD UNREGRESSED from v1/v2 §Part 1; re-verified, not reopened)

The whole RFC leans on one assertion: **a per-lane uid dissolves `BC1-W1-ORACLE` on this
host.** v1 proved it as four structural sub-claims + a residual-surface closure; both v1 and v2
falsifiers credited it and the adjudicators independently re-verified the launch path. It is
carried here **unchanged**. The exact attack: target lane runs as `U_t`, sibling as
`U_s ≠ U_t`; the launch path is `sudo -n -u <runAsUser> -- env -i … tmux …`
(`commandInvocationWithEnvFile`, `pty.go:98-112`), tmux invoked **bare** through the same
run-as path (`tmuxRunnerForSpec`→`RunAsTmuxRunner`, `pty.go:310-314`;
`tmux_liveness.go:125-149`) with a deterministic session name (`pty.go:620-633`) and **no**
`-S`/`TMUX_TMPDIR` (`sanitizedRunAsEnv`, `pty.go:120-155`).

- **HC-A1** — each lane's bare tmux lands on tmux's **default per-uid socket**
  `$TMUX_TMPDIR/tmux-<uid>/default` (`/tmp/tmux-<uid>`, dir `0700` owned by that uid,
  socket `srwx------`). `U_s` has no `--x` on `/tmp/tmux-<U_t>` ⟹ `connect(2)` fails
  `EACCES` before any tmux command parses; the same-uid-mutable oracle is gone. **Refuter /
  test A1:** `TestSiblingPoolUIDCannotRespawnTargetPane` (real-path) — `U_s`'s
  `respawn-pane -k` against `U_t`'s pane must fail and the pane pid must be unchanged.
- **HC-A2** — a `U_t`-owned `0600` file in a `0700` HOME is unreadable by `U_s` under POSIX
  DAC (the RFC 0143 Slice B reseal-token surface). **Test A2:**
  `TestSiblingPoolUIDCannotReadLaneOwnedResealToken`.
- **HC-A3** — cross-uid `ptrace`/`setns`/`open("/proc/<U_t>/…")` are denied by
  `ptrace_may_access` (matching-uid or `CAP_SYS_PTRACE` required), **independent of Yama**.
  This is the exact axis on which **namespace-inode binding failed** (D261 — a shared-uid
  sibling passes `ptrace_may_access` and `setns`s in; a different uid cannot). **Test A3:**
  `TestSiblingPoolUIDCannotPtraceOrSetnsOrReadProcSecrets` (asserts `ptrace_scope=1`
  precondition).
- **HC-A4** — the daemon control socket reads kernel-attested `SO_PEERCRED {pid,uid}`; with
  a per-lane uid the accept predicate gains a **meaningful uid discriminator**
  (`peer.uid == U_t`), so a `U_s` peer is rejected before any pid/oracle reasoning. **Test
  A4:** `TestControlFrameAcceptsOnlyLeasedLaneUID`.
- **HC-A5** — every residual same-uid surface is named and closed: the only common ancestor
  of two lanes is the trusted **daemon** (not a lane); the shared tmux server is now per-uid
  (HC-A1); world/group-readable paths are closed on every **private** surface
  (`0600`/`0700`, no group ACL — **see OQ4 for the exact `.striatum/` boundary, whose
  end-state is unchanged in v3**); daemon-bridging is lane-vs-daemon, not lane-to-lane.
  **Test A5:** `TestNoSharedSameUIDSurfaceBetweenPoolLanes`.

**Hard-core conclusion (carried):** the BC1-W1-ORACLE mechanism requires `U_s` to address
`U_t`'s tmux server (A1: denied), `ptrace`/`setns` into `U_t` (A3: denied), or read a
`U_t`-owned file (A2: denied) — all structurally closed by the different uid (A5: no residual
bridge). The daemon no longer infers identity from a mutable oracle; the kernel **attests** the
connecting uid (A4). This is a **narrowing**. *Nothing in v3 changes Part 1; the full per-claim
proof and source citations are in v1/v2 §Part 1.*

---

# Part 2 — THE SIX OPEN QUESTIONS

## OQ1 — Pool size + exhaustion (CARRIED UNREGRESSED, the v1 caveat CLOSED by C1)

**Carried unregressed.** There is no host-global concurrent-lane ceiling today
(`max_active_jobs` is per-workflow, default `0`=unlimited; `runreconcile_test.go:395`), so a
finite uid pool **introduces the first host-global ceiling** — a deliberate, named consequence.
Sizing: `N` = the host's max concurrent live **sessions with an attached supervisor across all
runs** (a lane holds a uid for its session lifetime, OQ2), operator-chosen, runbook-documented
(`N ≥ Σ_runs(expected concurrent distinct-lane supervisors)`). Exhaustion policy = **REFUSE,
fail-closed, typed**: when a launch needs a uid and none is free, the daemon refuses with a
typed `lane_uid_pool_exhausted` floor and leaves the job **queued/recoverable**; it never falls
back to the shared `striatum-lane` uid, never blocks-and-waits holding a lock, never
auto-`useradd`s. Tests **A6** (`TestLaneUIDPoolCeilingIsHostGlobalAcrossRuns`) and **A7**
(`TestLaneUIDPoolExhaustionRefusesTyped`) carry forward.

**The v1 caveat, closed by C1 (carried from v2).** Exhaustion accounting reads the OQ2
quarantine state: the free predicate is *"no row in `active|scrubbing|quarantined`"*, so
`free = N − count(active) − count(scrubbing) − count(quarantined)` — a `quarantined` uid is
counted **consumed**, the ceiling is honest, and **A20** asserts exhaustion fires at the reduced
ceiling rather than re-leasing a dirty uid. (The C1-RESIDUAL fix only **adds** quarantine
causes — a stopped/traced survivor now quarantines instead of returning dirty — so this caveat
stays closed, *more* tightly.)

## OQ2 — Lease/scrub/reaper lifecycle (structural machine CARRIED from v2; **scrub-postcondition predicate P1 TIGHTENED to discharge C1-RESIDUAL**)

**What is carried from v2 (the v2-credited structural half — do NOT reopen):** the durable
four-state machine, the partial held-unique index, the three-transaction boundary, the reaper,
the doctor surface, quarantine-survives-restart, and dirty-excluding exhaustion. **What v3
changes:** the **process-domain predicate P1** inside the postcondition proof (OQ2.3) and the
**classifier it is wired to** (OQ2.6). Nothing else in OQ2 moves.

### OQ2.1 — A durable FOUR-state lease, with the free set excluding every non-clean state (CARRIED)

P0 adds the daemon-owned table **`striatumd.lane_uid_leases`** (a new owner-bundle version, the
same additive mechanism that added `jobs.recovery_generation` /
`leases.reseal_grace_extended_at`). It lives in daemon-owned PostgreSQL, so it **survives a
`striatumd` restart** (D094 / RFC 0043). The schema (v1 fields **plus** the C1 states and
scrub-proof columns; the `scrub_proof`/`scrub_failure` columns now also carry the per-PID
`/proc` observations the C1-RESIDUAL fix requires):

```sql
CREATE TABLE striatumd.lane_uid_leases (
  lease_id       text PRIMARY KEY,         -- host-global (the pool is a HOST resource, OQ1)
  pool_uid       integer NOT NULL,         -- the leased OS uid
  generation     bigint  NOT NULL,         -- monotonic per pool_uid (OQ5 anti-recycle token)
  repository_id  text    NOT NULL,         -- which repo this lease currently serves (ACL scope, OQ4)
  session_id     text    NOT NULL,
  supervisor_id  text    NOT NULL,
  state          text    NOT NULL
                 CHECK (state IN ('active','scrubbing','quarantined','returned')),
  scrub_status   text    CHECK (scrub_status IN ('clean','failed')), -- null until finalize
  scrub_proof    jsonb,                    -- P1..P5 observations; P1 records observed PIDs + /proc state chars + classification
  scrub_failure  text,                     -- which proof failed + detail (quarantined rows); for P1, the blocking PID + non-zombie state
  leased_at      timestamptz NOT NULL,
  scrub_started_at timestamptz,
  returned_at    timestamptz
);

-- A uid HELD (in any non-clean state) is exclusive HOST-WIDE: the structural guard
-- that no second session can be allocated a uid that is active, mid-scrub, or dirty.
CREATE UNIQUE INDEX uq_lane_uid_held
  ON striatumd.lane_uid_leases(pool_uid)
  WHERE state IN ('active','scrubbing','quarantined');
```

**Free predicate (carried).** A pool uid is allocatable iff it has **no** row in
`active|scrubbing|quarantined` (latest row `returned`, or never leased) — never v1's "no active
row". The free set is **derived from the table**, never held in memory (restart-survival). The
partial unique index makes any of the three mutually exclusive per uid, so two transactions
cannot both hold one uid.

### OQ2.2 — The allocate / scrub-begin / scrub-finalize TRANSACTION boundary (crash-safe; CARRIED)

The side-effecting scrub (external `sudo … kill/rm` + `/proc` probes) **cannot** run inside a
DB transaction (the same `#198` reasoning that moved liveness probes **out** of the sweep tx,
`recovery_auto.go:22-38` — "pre-probe every supervised agent's liveness OUTSIDE the transaction").
So the lifecycle is **three transactions** with the scrub strictly
**between** them:

1. **Allocate** (`tx_alloc`, at `supervise.start`): insert
   `{state='active', generation=(max generation for pool_uid)+1, …}` selecting a uid whose
   latest state is `returned`/absent; `uq_lane_uid_held` makes a concurrent double-allocate
   fail atomically. Launch as `RunAsUser = pool_uid`. *No scrub here.*
2. **Scrub-begin** (`tx_scrub_begin`, guarded `state='active'→'scrubbing'`,
   `scrub_started_at=now`): on `session.close` (hooked into `stopSupervisorInTx`,
   `supervision_control.go:557-637`) **or** when the reaper claims a dead/leaked lease. This
   **atomically removes the uid from the free set BEFORE any scrub command runs.**
3. **Scrub + postcondition proof** (OUT of any DB tx): the steps + proof in OQ2.3.
4. **Scrub-finalize** (`tx_scrub_finalize`, guarded `state='scrubbing'`): proof passed ⟹
   `→'returned'`, `scrub_status='clean'`, `scrub_proof=<observations>`, `returned_at=now`.
   Proof failed ⟹ `→'quarantined'`, `scrub_status='failed'`, `scrub_failure=<detail>`, emit the
   typed `lane_uid_scrub_failed` floor.

**Crash-safety (carried).** A crash between (2) and (4) leaves the row durably `scrubbing` —
not free, not re-leasable — and the recovery sweep re-drives it (OQ2.4).

### OQ2.3 — The scrub steps AND the postcondition PROOF (observation, not exit codes; **P1 TIGHTENED for C1-RESIDUAL**)

**Scrub commands** (all `sudo -n -u <pool_uid> --`, within the launch-as grant the daemon
already holds — OQ3; the live teardown at `supervision_control.go:557-637` does **none** of
these today beyond `CleanupGeminiSettings`/`CleanupClaudeScheduledTasksLock`, so P0 adds this as
a new scrub primitive):

- **S1 — per-uid kill domain:** `kill -KILL -1` (every process owned by `pool_uid`), reaping
  the uid-owned tmux **server** (HC-A1) and any stray/daemonized lane processes. Safe only
  because the uid is private to this lease.
- **S2 — credential store:** delete `~<pool_uid>/.claude/.credentials.json` and the resolved
  `CLAUDE_CONFIG_DIR` store (OQ6).
- **S3 — HOME scratch + per-lease ACLs:** remove the lane's writable HOME contents and the
  per-leased-uid `.striatum/` ACL grants (OQ4) and release/chown-back the per-job worktree.

**The POSTCONDITION proof (the heart of C1 — NOT `exit==0`).** After S1–S3 the daemon
**proves** the uid domain is empty by *observation as the daemon/owner*, with a bounded retry
(re-`kill`+re-probe, `M` attempts, short backoff) before declaring failure:

- **P1 — empty kill domain (TIGHTENED — the C1-RESIDUAL fix).** Enumerate **all**
  `pool_uid`-owned PIDs by reading each `/proc/<pid>/status` `Uid:` line (real **and** effective
  uid) — a **new** owner-uid read: no existing supervisor helper reads `/proc/<pid>/status` today
  (the package's `/proc` readers all read `/proc/<pid>/stat`), so the build adds it — and, for
  each enumerated PID, observe its `/proc/<pid>/stat` **state character** via the new classifier
  `classifyPoolUIDTaskState` (OQ2.6). The state read reuses the exact `/proc/<pid>/stat`
  comm-skip parse shape (`strings.LastIndex(text, ")")` then the field after it) that the existing
  `processZombie` (`tmux_liveness.go:599-614`, state char) and `ProcessStartToken`
  (`process_identity_linux.go:13`, `/proc/<pid>/stat` field 22 start-time, called by the liveness
  probe region `tmux_liveness.go:387-452`) already use — so P1's state parse is the codebase's
  proven `/proc/<pid>/stat` shape, only the classification (OQ2.6) and the owner-uid enumeration
  are new. P1 passes **iff every** observed `pool_uid`-owned task
  is **`Reaped`** (a true zombie/dead task that cannot execute and holds no resources), and
  **no** task is `Live` or `Unknown`:

  > **P1 predicate (the load-bearing assertion):** *After bounded re-kill/re-probe, there are
  > **zero** `pool_uid`-owned tasks except true zombies / dead tasks (`/proc` state ∈
  > {`Z`,`X`,`x`}) — which cannot execute and hold no resources. **ANY** observed non-zombie
  > state — `R`, `S`, `D`, **`T` (stopped)**, **`t` (tracing-stop)**, `I`, `W`, `P`, `K`, any
  > **unrecognized** state char, **or an unreadable/ambiguous** `/proc/<pid>/stat` for a PID
  > still present in the domain — **blocks** `returned` and finalizes `quarantined` with a typed
  > `lane_uid_scrub_failed`.*

  This closes the v2 gap: a `T`/`t` survivor has **not** exited — it still holds the uid,
  memory, FDs, and HOME/credential reachability and is `SIGCONT`-resumable — so it is `Live`
  and **must** block. A `Z`/`X` task holds no resources and cannot run code, so it is tolerated
  (and **recorded**, below). A PID that has fully disappeared from the re-enumerated domain is a
  genuine exit (tolerated); an **unreadable** `/proc` read for a **still-listed** PID is
  `Unknown` and **fail-closed → block** (the v2 binary helper's "read error → not-zombie → not
  blocking" trap is closed). **The observed PIDs, their `/proc` state chars, and their
  classification are recorded in `scrub_proof`** (on pass — tolerated `Z` residue) **and in
  `scrub_failure`** (on block — the non-zombie PID + state that caused quarantine), so `doctor`
  distinguishes tolerated zombie residue from a real quarantine cause.
- **P2 — no uid-owned tmux server:** assert `connect(2)` to `/tmp/tmux-<pool_uid>/default`
  fails `ECONNREFUSED`/`ENOENT` (server gone, not merely signaled).
- **P3 — credential store absent:** `stat(2)` the resolved per-uid credential path(s) ⟹
  `ENOENT`.
- **P4 — HOME scratch reset + reseal-token absent:** assert the per-uid writable HOME scratch
  is gone and the reseal-token path is absent.
- **P5 — per-lease ACLs removed:** assert no `.striatum/scratch/<supervisor_id>` ACL entry for
  `pool_uid` remains (ties to OQ4).

`returned` is reached **only** when P1–P5 all hold (recorded in `scrub_proof`). **Any** failed
proof ⟹ `quarantined`. The negative path is **defined and proven** by observation, not asserted
positively — and P1's process predicate now rejects the **entire** non-zombie class, not just
`R/S/D`.

### OQ2.4 — The reaper (leaked-active + stuck-scrubbing) and quarantine remediation (CARRIED)

The recovery sweep is the reaper host (`HandleRecoveryAuto`, `recovery_auto.go:12`, scheduled by
`startRecoveryScheduler`, `main.go:869`, default every 60s — `--sweep-interval-seconds` default
`60.0`, `main.go:80`; it already expires leases — `expireLeases`, `recovery_lease_expiry.go:86` —
and reaps idle orphans — `reapIdleOrphanSessions`, `recovery_decision_tree.go:1523` — using
`buildRunLivenessOracle`, `recovery_liveness_oracle.go:117`). P0 extends it:

- **Leaked-active reaper:** an `active` lease whose owning session is dead and that never ran
  `session.close` (daemon died mid-lease) is transitioned `active→scrubbing` (tx_scrub_begin)
  and driven through scrub+proof (→`returned`/`quarantined`). **No uid leaks active.**
- **Stuck-scrubbing reaper:** a `scrubbing` row (crash between begin and finalize) is re-driven
  **idempotently** each sweep until it finalizes; a `scrubbing` row that outlives `M` sweeps
  surfaces a typed `lane_uid_lease_stuck_scrubbing` doctor finding.
- **Quarantine remediation:** a `quarantined` row is **never** auto-returned. It stays out of the
  free set across restarts. It clears only via an explicit operator/recovery retry (a `recovery`
  verb / MCP recovery method) that re-runs the **same** scrub+proof and transitions
  `quarantined→returned` **only on a clean proof** (the single `quarantined→returned` edge,
  never a blind clear). *Because P1 is now tighter, a uid quarantined by a stopped/traced
  survivor stays quarantined until that survivor is gone and the same P1 re-classifies the
  domain as all-`Reaped`/empty.*

### OQ2.5 — Restart-survival (CARRIED)

The boot epoch is fresh-per-process and not persisted (`randomBootEpoch`, `main.go:782`, via
`daemonBootEpoch`, `main.go:772`); `daemonInstanceID` is restart-stable (`main.go:728`). On restart **no
in-memory binding survives** — but the `lane_uid_leases` rows do (PostgreSQL). The daemon
**derives** pool state from the table: a live `active` lease whose lane is still alive keeps its
uid (the OQ5 generation re-binds attestation); a dead `active` is reaped; a `scrubbing` row is
re-driven; **a `quarantined` row stays `quarantined`**. The free set is derived, never
memory-held, so a restart cannot double-lease or strand a uid.

### OQ2.6 — The process-state classifier (NEW — the C1-RESIDUAL mechanism; explicitly NOT `processZombie`)

P1 is wired to a **new** helper, `classifyPoolUIDTaskState(pid int) → {Reaped, Live, Unknown}`,
that the build adds **alongside** — and **must not** replace P1's wiring with — the existing
binary `processZombie` (`go/pkg/supervisor/tmux_liveness.go:599-614`). The existing helper reads
`/proc/<pid>/stat`, takes the state field after the last `)` (the comm-skip parse at `:607-613`),
and returns `true` **only** when that field is `"Z"`, and `false` on a read error
(`:604-605`) — a **binary** classification that conflates `T`/`t`/`R`/`S`/`D`/unknown/error into
one *not-`Z`* bucket. P1 must distinguish three buckets:

- **`Reaped`** — state ∈ {`Z` (zombie), `X`/`x` (dead)}: cannot execute, holds no resources.
  **Tolerated** (and recorded).
- **`Live`** — state ∈ {`R`,`S`,`D`,`T`,`t`,`I`,`W`,`P`,`K`,…} (any recognized non-dead state,
  **including stopped `T` and tracing-stop `t`**): still holds the uid/resources, is
  resumable. **Blocks** → `quarantined`.
- **`Unknown`** — `/proc/<pid>/stat` unreadable or malformed for a PID **still present** in the
  re-enumerated domain, or an **unrecognized** state char. **Fail-closed → blocks** →
  `quarantined`. (A PID that has fully vanished from the re-enumerated domain is a genuine exit,
  not `Unknown`; the bounded re-probe re-enumerates before classifying so a transient race does
  not falsely quarantine, and a stable non-zombie survivor cannot pass.)

The classifier reuses the **parse** shape of `processZombie` (skip the `comm` field via the last
`)`, read the first whitespace field after it) but **inverts the default**: anything that is not
provably `Reaped` is blocking. A spec note + the A21 test guarantee the build does **not** wire
P1 to `processZombie` (or to `!processZombie`, which would mis-handle the read-error case as
*not-blocking*).

## OQ3 — Provisioning ownership (CARRIED UNREGRESSED)

Host-setup runbook artifact (static pool); the daemon gets **no** uid-lifecycle authority. The
pool is pre-created by the operator runbook (extending `docs/how-to/lane-sandbox.md`), exactly
as the single lane user is today. The daemon **leases** from a static set; it never
`useradd`/`userdel`s. The sudoers grant widens from one user to a runas **group**
(`striatumd ALL=(%striatum-lanes) NOPASSWD: ALL`, still **launch-only**;
`lane-sandbox.md:94`). The daemon's only new exercise of that grant is the **scrub** (`sudo -n
-u <pool_uid> -- kill/rm` — within the launch-as authority it already holds, no new privilege
class). This keeps RFC 0168 a **narrowing**. **Test A12**
(`TestDaemonHoldsNoUIDLifecycleAuthority`) carries forward.

## OQ4 — ACL interaction (final end-state CARRIED from v2; **provisioning procedure made MANDATORY-safe to discharge C2-RESIDUAL**)

**What is carried from v2 (the v2-credited final ACL end-state — do NOT reopen):** the hard
`.striatum/` boundary invariant, the per-leased-uid `--x` traverse re-keyed from
`scratch_acl.go`, the per-supervisor scratch `rwx` to the leased uid alone, the chowned
worktree, and per-lease ACLs removed on scrub. **What v3 changes:** the **procedure** that
reaches that end-state — the allowlist / exclude-at-traversal form is now **MANDATORY**, the
raw-recursive-root-then-strip form is **FORBIDDEN**, plus a transition test and a planner guard.
**The end-state invariant itself is unchanged.**

### OQ4.1 — Two ACL domains with a hard boundary at `.striatum/` (end-state CARRIED; **procedure now mandatory-safe**)

**(a) Shared source/artifact tree → group `rX`, via a MANDATORY allowlist / exclude-at-traversal
form that NEVER touches `.striatum/`.** The pool group `striatum-lanes` (OQ3) gets recursive
access + default read/traverse on the repo's **shared work product only**, never on
`.striatum/`, `.git/`, or provider/token-cache paths. The current single-lane helper
(`provisionCommitteeACLs`, `repo_acl.go:97-140`) applies `setRepoACL` (`= setfacl -R`, `:25-31`)
over `repoRoot` **and** the `.striatum/worktrees` root (`:130-135`) — a raw recursion that
**descends through `.striatum/`** and is exactly the shape the pool/group version **must NOT
replicate**. The pool provisioner is build-bearing:

- **MANDATORY form (the C2-RESIDUAL fix):** the provisioner **enumerates the top-level entries**
  of `<repoRoot>` (`readdir`), **prunes** the forbidden set (`.striatum`, `.git`, and any
  configured provider/token-cache top-level path), and applies
  `setfacl -R -m g:striatum-lanes:rX -m d:g:striatum-lanes:rX` to **each remaining
  source/artifact entry individually**. Because `.striatum/` is a **sibling** of the source
  entries — never one of them and never a descendant of one — the group entry is **never added
  to it, not even transiently.** Equivalently, an **exclude-at-traversal** walk that prunes the
  forbidden subtrees **before** applying any grant is permitted. **No default ACL is set on
  `<repoRoot>` itself** (a default on the root would let a later-created `.striatum/` inherit the
  group entry); defaults live only on the allowlisted source entries.
- **EXPLICITLY FORBIDDEN (the C2-RESIDUAL prohibition):** `setfacl -R -m g:striatum-lanes:rX
  <repoRoot>` (or any `-R` grant rooted at `<repoRoot>` or any **ancestor of `.striatum/`**)
  followed by a strip is **not** an acceptable live-repo provisioning path. The first `-R`
  necessarily adds group `r` to existing `.striatum/` `0600` control-plane files **before** the
  strip — a transient group-readable window on a session-bound bearer that is **irreversible**
  exfiltration. The v2 SPEC's blessing of this form ("gates on the auditable end-state, not the
  procedure") is **withdrawn**.

  > **OQ4 invariant (UNCHANGED end-state, the load-bearing assertion):** *No path under
  > `<repoRoot>/.striatum/` (nor `<repoRoot>/.git/`, nor a provider/token-cache path) carries a
  > `g:striatum-lanes` access **or** default ACL entry — **before, during, OR after**
  > provisioning, for existing or future files.* (v3 adds the **during** clause to what was a
  > before/after invariant; the mandatory allowlist form is what makes the **during** clause
  > true by construction.)

  A non-leased pool uid that is a group member can read/traverse shared **source** — **not** a
  new exposure (the single shared uid does so today); the security boundary that matters
  (control-plane + private secrets) is **never** group-granted, at any instant.

**(b) Control-plane / private / worktree → per LEASED uid only, removed on scrub (CARRIED
UNREGRESSED).** No group ACL touches any of these; every grant is keyed to the **currently
leased** uid, applied at lease/launch, removed at scrub (OQ2 S3 / proof P5):

- `.striatum/` → `u:<leased-uid>:--x` (traverse only) — re-keys `scratch_acl.go:46` from a fixed
  `laneUser` to the **leased** uid. An **unleased** pool uid gets **no entry** ⟹ cannot even
  traverse `.striatum/`.
- `.striatum/scratch/` → `u:<leased-uid>:--x` (traverse only — **not** `rwx`; pushed down from
  `scratch_acl.go:47`) so a leased uid cannot list/read **sibling** supervisors' scratch dirs.
- `.striatum/scratch/<supervisor_id>/` → `u:<leased-uid>:rwx` + default ACL — the lane's **own**
  ephemeral MCP config dir (where `mcpconfig.go` writes the `0600` bearer and `loop.go` writes
  `pty.log`). Per-supervisor, so another leased uid has no entry on it.
- `.striatum/worktrees/<id>/` → **chowned to the leased uid** at worktree creation; **no group
  ACL, no group default** on the worktrees root (replacing the current
  `repo_acl.go:130-135` default-ACL-on-worktrees-root grant for the pool case). The *only* lane
  exception under `.striatum/`, matching `lane-sandbox.md:352-355`.
- **PG isolation** covers every pool uid via a group reject rule
  (`local all %striatum-lanes reject` + loopback forms, the pool analogue of
  `lane-sandbox.md:77-79`).

### OQ4.2 — Why the exact failing cases are closed (steady-state + transition)

**Steady-state (carried from v2).** An **unleased** pool uid `U_x` has **no** ACL entry on
`.striatum/` (only the *currently leased* uid does, and only `--x`), so it cannot traverse into
`.striatum/scratch/<S1>/` — `open(2)` of S1's bearer fails `EACCES` at the `.striatum` traverse.
A **different leased** uid `U_2` still cannot read S1's bearer: `.striatum/scratch` is `--x`
(traverse-only, no `r`/list), and `.striatum/scratch/<S1>/` carries an ACL entry only for S1's
leased uid ⟹ `EACCES`. No group ACL ever touches `.striatum/`, so the
`-R …:rX`-adds-group-`r`-to-`0600` problem cannot arise.

**Transition (the C2-RESIDUAL closure).** Because the mandatory provisioner **never roots a
grant at `<repoRoot>` (or any ancestor of `.striatum/`)** and only applies grants to allowlisted
**source** entries that do **not** contain `.striatum/`, there is **no instant** during
provisioning at which any `g:striatum-lanes` entry exists on `.striatum/` or its contents. The
transient window the v2 recursive form opened **does not exist**: `U_x`/`U_2` looping on
`open(2)` of S1's bearer get `EACCES` **before, during, and after** the provisioner runs (A22).
The deterministic planner guard (OQ4.3) proves no provisioning op could ever target the
control-plane root.

### OQ4.3 — The ACL-planner guard (NEW — the C2-RESIDUAL mechanism)

The pool ACL provisioner is split into a **pure planner** (emits the ordered list of
`{target_path, specs}` setfacl operations) and an **applier** (runs them). A deterministic guard
runs over the **planner output** and **fails** the plan (refuse-to-provision, typed error) if
**any** `g:striatum-lanes` operation:

- targets `<repoRoot>` (or any **ancestor directory of `.striatum/`**) as a **raw recursive
  (`-R`) root** while `.striatum/` exists under it, **or**
- targets `.striatum/`, `.git/`, or a configured provider/token-cache path (recursively or not),
  **or**
- sets a **default** (`d:`) `g:striatum-lanes` entry on a directory that has `.striatum/` as a
  descendant.

The guard is **pure** (operates on planned paths/specs, needs no root, no `setfacl`), so its
unit test (A23) runs in CI. It is the build-time backstop that makes the "no transient exposure"
property checkable **without** waiting for a runtime audit; the runtime `getfacl` doctor /
`make lane-isolation-check` checks (carried, A16) remain **necessary but not sufficient** — they
prove the cleanup/end-state, the planner guard proves the **procedure** never exposes.

## OQ5 — Attestation + recycle-confusion generation token (CARRIED UNREGRESSED)

Attestation records the **leased uid** (from the `lane_uid_leases` row) so it answers *"is this
the lane we leased `U_t` to?"* — the PID start-token (`ProcessStartToken`,
`process_identity_linux.go:13`, driven by the liveness probe region `tmux_liveness.go:387-452`)
discriminates the **process**, the leased uid the **principal**. The
`lane_uid_leases.generation` (monotonic per uid, minted in `tx_alloc`, OQ2.2) is folded into the
lane's capability material exactly as `MCPBootEpoch` is (`mutations.go:41-48`); the daemon
**refuses to attest, and refuses a control frame, when the presented generation ≠ the live
generation for that uid** — on **every** attestation **and** control-frame path. **Test A14**
(`TestRecycledUIDGenerationPreventsCrossLeaseConfusion`) carries forward.

## OQ6 — Per-uid credential store (CARRIED, contingency CLOSED by C1)

The RFC 0165 spawn-time hydrator (#583) targets the **leased** uid's HOME via temp-file+rename
(verifying destination owner==leased-uid, mode `0600`, parseable OAuth, expiry lead, source
generation stability). Hydration is **per-spawn** and the **scrub deletes** the per-uid store on
return (OQ2 S2, proven absent by P3) — fresh in, deleted out, never N stale copies; `0600` owned
by the leased uid inside its `0700` HOME, unreadable by any other pool uid (HC-A2). The v1
contingency (*a failed scrub leaves a credential store*) is **closed**: a failed P3 ⟹
`quarantined` — and, with the C1-RESIDUAL fix, a stopped/traced survivor that could otherwise
have re-opened the store post-return is **also** caught by P1 ⟹ `quarantined`. **Test A15**
(`TestPerUIDCredentialHydrateNoStaleNoLeak`) carries forward, extended to assert an injected P3
failure quarantines rather than re-leasing (overlaps A17).

---

# Part 3 — THE P0 SLICE (updated for the discharged C1-RESIDUAL / C2-RESIDUAL)

P0 is the minimum for a lane to run as its own pooled uid and safely own a `0600` reseal token:

1. **Static pool, host-provisioned** (OQ3): N pool uids, `striatum-lanes` group, widened
   runas-group sudoers, per-uid PG reject, **and the revised OQ4 ACL** (group `rX` on shared
   source via the **mandatory allowlist / exclude-at-traversal** provisioner that never touches
   `.striatum/`/`.git/`; per-leased-uid `.striatum/` traverse + per-supervisor scratch + chowned
   worktree); daemon holds no uid-lifecycle authority.
2. **Daemon-owned `lane_uid_leases` table** (OQ2): the **four-state** machine, the partial
   **held-unique** index, the generation token, **persisted** (restart-survival).
3. **Allocation + host-global admission ceiling** (OQ1): lease a free uid (free = no
   `active|scrubbing|quarantined` row) at `supervise.start`; refuse `lane_uid_pool_exhausted`
   when none is free.
4. **Return + scrub + PROOF + reaper** (OQ2): the allocate/scrub-begin/scrub-finalize boundary;
   S1–S3 scrub + P1–P5 postcondition proof on `session.close`, **with P1 now rejecting every
   non-zombie `pool_uid`-owned task** (the C1-RESIDUAL fix, wired to `classifyPoolUIDTaskState`);
   the recovery sweep reaps leaked-active and re-drives stuck-scrubbing;
   quarantine-on-failed-proof with a doctor surface and operator retry.
5. **Attestation binds uid + generation** (OQ5).
6. **Per-uid credential hydration** (OQ6): scrub deletes on return (proven by P3).

With (1)–(6), RFC 0143 Slice B reduces to *"write a session-scoped reseal token owned by the
leased uid, `0600`"* — safe by HC-A2.

**Build-run target (per the SEED).** The new table lands in the **next FREE runtime-migration
slot** — runtime migrations currently end at `0044_deploy_cursor.sql`, and **`0045` is reserved
by the concurrent RFC 0170 P0 `cullable_entity` work**, so `lane_uid_leases` takes **`0046+`
(do NOT hardcode `0045`; the build picks the next free slot at implementation time)** — plus an
**owner-bundle bump** for the daemon-owned `striatumd.lane_uid_leases` table (owner migrations
currently end at `owner/0022_operator_identity_run_attribution.sql`, so the bump is the next free
owner slot, `owner/0023+`, granting `striatumd` ownership/grants — the same additive mechanism as
the prior owner-bundle versions).

**Seams deferred to later slices (named, not dropped):** daemon-managed dynamic uid
create/destroy; automated pool autogrow/resizing; the reseal-token write itself (RFC 0143
Slice B); cross-host/multi-host pooling; non-tmux adapter parity for the per-uid kill domain.

**Local-first boundary preserved.** One host, one PostgreSQL (the single writer, D094 / RFC
0043), one daemon; every scrub/kill/probe is local `sudo -n -u <pool_uid>` within the launch-as
grant; the pool is OS users; no hosted service, cloud API, telemetry, or external persistence.
Both residual fixes are **narrowings** — C1-RESIDUAL only **adds** quarantine causes (a
stopped/traced survivor can no longer return dirty), C2-RESIDUAL only **removes** a transient
exposure window; no new authority is granted.

---

# Source re-verification (every load-bearing site CONFIRMED against current worktree HEAD; the two residual-fix anchors pinned to the LIVE line numbers)

| Claim | Site | Status |
| --- | --- | --- |
| Run-as launch = `sudo -n -u <runAsUser> -- env -i …`; bare tmux; minimal env; deterministic session name | `pty.go:98-112,:120-155,:310-314,:620-633`; `tmux_liveness.go:125-149` | **CONFIRMED** (carried Part 1) |
| `leases` is a **job** lease (`resource_type CHECK IN ('job')`, `state CHECK IN ('active','released','expired')`); `uq_active_resource_lease` partial-unique on active | `0005_repo_local_workflow_state.sql:166-186` | **CONFIRMED** — the new `lane_uid_leases` mirrors this shape + the four states + the held-unique index |
| **`processZombie` classifies `/proc` state as BINARY `Z`-or-not — reads `/proc/<pid>/stat`, takes the field after the last `)`, returns `true` only when `== "Z"`, and returns `false` on a READ ERROR** (the C1-RESIDUAL trap: an implementer wiring P1 off this tolerates `T`/`t`/unknown) | `go/pkg/supervisor/tmux_liveness.go:599-614` (state parse `:607-613`; read-error→false `:604-605`) | **CONFIRMED (LIVE line numbers — the v2 ledger's `:576-591` was stale)** — P0 adds the NEW `classifyPoolUIDTaskState` 3-way classifier instead |
| P1's `/proc` mechanism: the **state-char** parse shape P1 reuses (`/proc/<pid>/stat`, comm-skip via `LastIndex(")")`) — proven by `processZombie` and `ProcessStartToken`; the liveness probe region that drives PID identity | `processZombie` `tmux_liveness.go:599-614`; `ProcessStartToken` `process_identity_linux.go:13` (`/proc/<pid>/stat` field 22); probe region `tmux_liveness.go:387-452` (`ProbeLaneLiveness`/`PIDLiveWithStartToken`) | **CONFIRMED — re-pinned: `ProcessStartToken` is defined in `process_identity_linux.go:13` (the probe region only CALLS it), and the per-PID owner-uid read from `/proc/<pid>/status` `Uid:` is NEW (no existing supervisor reader; all current `/proc` readers read `/proc/<pid>/stat`)** |
| **Live teardown does tmux-kill / `terminateProcessWithStartToken` + `CleanupGeminiSettings`/`CleanupClaudeScheduledTasksLock`, closes the session only when it holds no active lease — NO per-uid kill / cred / HOME scrub** | `supervision_control.go:557-637` | **CONFIRMED** — P0 hooks `tx_scrub_begin` + the scrub here |
| Recovery sweep is the reaper host (default 60s); expires leases; reaps idle orphans; builds a liveness oracle; #198 moves probes OUT of the sweep tx | `recovery_auto.go:12` (`HandleRecoveryAuto`), `:22-38`/`:577` (#198 probes/drain out-of-tx); `recovery_lease_expiry.go:86` (`expireLeases`); `recovery_decision_tree.go:1523` (`reapIdleOrphanSessions`); `recovery_liveness_oracle.go:117` (`buildRunLivenessOracle`); `main.go:869` (`startRecoveryScheduler`), `:80` (`--sweep-interval-seconds` default `60.0`) | **CONFIRMED (LIVE — re-pinned; the v2 `recovery.go:553/:2516/:565-587`, `recovery_decision_tree.go:1474`, `main.go:81` are stale: `recovery.go` was split into per-concern files since v2)** — P0 extends it with the leaked-active + stuck-scrubbing reaper |
| Boot epoch fresh-per-process, NOT persisted; `daemonInstanceID` restart-stable ⟹ derive the free set from the DB, not memory | `main.go:782` (`randomBootEpoch`), `:772` (`daemonBootEpoch`), `:728` (`daemonInstanceID`) | **CONFIRMED (LIVE — re-pinned; the v2 `main.go:722/:731/:665-690` are stale)** (restart-survival) |
| `MCPBootEpoch` folded into capability material + rejected on mismatch (the model the OQ5 generation token reuses) | `mutations.go:41-48` | **CONFIRMED** |
| **`.striatum` carve-out: `u:<lane>:--x` traverse only, `.striatum/scratch`:`u:<lane>:rwx`+default, under "never broaden read access to private operator state"** | `scratch_acl.go:38-47` | **CONFIRMED** — P0 re-keys to the leased uid + pushes `--x` down to `.striatum/scratch` and `rwx` to `.striatum/scratch/<supervisor_id>` |
| **Current repo ACL `setRepoACL` is `setfacl -R` (`:25-31`); `provisionCommitteeACLs` applies it over `repoRoot` AND `.striatum/worktrees` (`:130-135`) — the raw-recursive-root recursion into `.striatum/` the pool/group MANDATORY form must NOT replicate** | `repo_acl.go:21-31,:97-140` | **CONFIRMED** — P0 replaces with the allowlist/exclude-at-traversal planner + per-leased-uid worktree chown |
| **`0600` MCP bearer + PTY log live under `.striatum/scratch/<supervisor_id>/`** (the transiently-exposed control-plane the C2-RESIDUAL fix protects) | `mcpconfig.go:241` (`WriteFile … 0o600`), `:266` (`scratch/<supervisorID>`); `loop.go:145` (`pty.log`), `:300` (`0o600`) | **CONFIRMED** — the exact files A22 plants and asserts unreadable before/during/after |
| Runbook: `.striatum/` is daemon-private (only lane exception = the per-job worktree) | `lane-sandbox.md:348-355,:77-79,:94` | **CONFIRMED** — P0 widens to the pool/group analogues with the carve-out preserved |
| Per-uid HOME + per-uid Claude credential store; RFC 0165 hydrator contract | `supervision_env.go:205-226`; `laneproviderauth/resolver.go:78-92`; RFC 0165 | **CONFIRMED** (OQ6) |
| No host-global concurrent-lane ceiling today (`max_active_jobs` per-workflow, default unlimited) | `runreconcile_test.go:395` | **CONFIRMED** (OQ1) |
| **Runtime migrations end at `0044`; `0045` reserved by RFC 0170 P0 `cullable_entity`; owner migrations end at `0022`** | `go/pkg/db/sql/0044_deploy_cursor.sql`; `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql` | **CONFIRMED** — `lane_uid_leases` → runtime `0046+`, owner bump `owner/0023+` (not hardcoded) |
| D261 ratifies the per-lane-uid direction; rejects namespace-inode/AppArmor-hat/private-socket-alone; blocker #585 | `docs/decisions/decision-log.md` (D261); RFC 0168 | **CONFIRMED** |

---

# Falsifiable-assertion index (the claims the v3 falsifiers re-attack)

| ID | Claim | Refuting test/check |
| --- | --- | --- |
| **A1–A5** | the hard core (per-uid tmux socket; `0600` DAC; cross-uid ptrace/setns//proc denial; `SO_PEERCRED` uid discriminator; no residual same-uid surface) — **CARRIED** | `TestSiblingPoolUIDCannotRespawnTargetPane` / `…ReadLaneOwnedResealToken` / `…PtraceOrSetnsOrReadProcSecrets`; `TestControlFrameAcceptsOnlyLeasedLaneUID`; `TestNoSharedSameUIDSurfaceBetweenPoolLanes` |
| **A6** | pool ceiling host-global across all runs; no double-lease — **CARRIED** | `TestLaneUIDPoolCeilingIsHostGlobalAcrossRuns` |
| **A7** | exhaustion refuses typed, queues, never shares/grows; frees on return — **CARRIED** | `TestLaneUIDPoolExhaustionRefusesTyped` |
| **A8′** | uid binds to the session and persists across restart (reconstructed from PostgreSQL) — **CARRIED** | `TestUIDLeaseBindsSessionAndPersistsAcrossRestart` |
| **A9′** | return scrubs (S1–S3) **and proves** an empty domain (P1–P5); `returned` only on a clean proof — **CARRIED** | `TestUIDReturnScrubsAndProvesEmptyDomain` |
| **A10′** | leaked-active uid reaped `active→scrubbing→returned/quarantined` by the sweep — **CARRIED** | `TestLeakedActiveUIDReapedToScrubbingThenProven` |
| **A11′** | binding reconstructed after a boot-epoch rotation — **CARRIED** | `TestUIDLeaseReconstructedAfterBootEpochRotation` |
| **A12** | daemon holds no uid-lifecycle authority (launch-as only) — **CARRIED** | `TestDaemonHoldsNoUIDLifecycleAuthority` |
| **A13** | group ACL grants shared **source** read only; private secrets + worktree write per-uid; all pool uids PG-denied — **CARRIED** | `TestPoolACLGrantsSharedReadNotPrivateOrCrossWrite` + `make lane-isolation-check` |
| **A14** | recycled-uid generation token refuses a stale-lease actor (every attestation **and** control-frame path) — **CARRIED** | `TestRecycledUIDGenerationPreventsCrossLeaseConfusion` |
| **A15** | per-uid credential hydration leaves no stale copy / no cross-uid leak; a failed P3 quarantines — **CARRIED + extended** | `TestPerUIDCredentialHydrateNoStaleNoLeak` |
| **A16** | **(C2 end-state, CARRIED)** the pool ACL exposes no `.striatum/` control-plane to an unleased **or** different-leased pool uid; no `g:striatum-lanes` entry under `.striatum/` (existing + future) | `TestPoolACLDoesNotExposeOperationalScratchOrForeignWorktrees` + extended `make lane-isolation-check`/`doctor` |
| **A17** | **(C1, CARRIED)** a failed scrub postcondition quarantines and the dirty uid is **never** re-leased | `TestScrubFailureQuarantinesAndIsNeverReLeased` |
| **A18** | **(C1, CARRIED)** a crash during scrub leaves the uid durably `scrubbing` (held, not free); the sweep re-drives it | `TestCrashDuringScrubLeavesUIDHeldNotFree` |
| **A19** | **(C1, CARRIED)** quarantine survives a restart and is non-free; clears only via the proof-gated `quarantined→returned` retry | `TestQuarantineSurvivesRestartAndIsNonFree` |
| **A20** | **(C1, CARRIED)** exhaustion accounting excludes `scrubbing`+`quarantined`; fires typed at the reduced ceiling, never dirty reuse | `TestExhaustionExcludesScrubbingAndQuarantined` |
| **A21** | **(C1-RESIDUAL — NEW)** a `pool_uid`-owned task left **stopped (`T`)** or **tracing-stopped (`t`)** — non-zombie, resource-holding, `SIGCONT`-resumable — **blocks** `returned`: P1 (wired to `classifyPoolUIDTaskState`, NOT `processZombie`) classifies it `Live`, finalizes `quarantined` with `lane_uid_scrub_failed`, the uid is **not** allocated, the quarantine survives a boot-epoch rotation, and only the same P1–P5 proof (domain now all-`Reaped`/empty) clears it; the blocking PID + `/proc` state is recorded in `scrub_failure`. **Refuter:** a `T`/`t` (or unknown/unreadable-still-present) survivor passes P1 and the uid reaches `returned`/re-lease. | `TestStoppedOrTracedUIDProcessBlocksReturn` |
| **A22** | **(C2-RESIDUAL — NEW)** the pool ACL provisioner **never transiently** exposes `.striatum/`: with S1's `0600` bearer / `pty.log` / token cache / foreign worktree seeded, an **unleased** pool uid `U_x` **and** a **different-leased** uid `U_2` looping on `open(2)`/traversal get `EACCES` **before, during, AND after** provisioning; no read ever succeeds; no `g:striatum-lanes` access-or-default entry ever appears under `.striatum/`. **Refuter:** any read succeeds at any instant, or a group entry transiently appears on `.striatum/`. | `TestPoolACLProvisioningNeverTransientlyExposesScratch` |
| **A23** | **(C2-RESIDUAL — NEW, deterministic planner guard)** the pure ACL planner's output is **rejected** if any `g:striatum-lanes` op targets `<repoRoot>` (or any ancestor of `.striatum/`) as a raw recursive `-R` root while `.striatum/` exists, targets `.striatum/`/`.git/`/provider-token-cache, or sets a `d:g:striatum-lanes` default on a dir with `.striatum/` as a descendant. **Refuter:** the planner emits — and the guard passes — a raw `setfacl -R …:rX <repoRoot>` plan. | `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists` |

**Negative control:** the BC1-W1-ORACLE replay itself (A1). **C1 controls:** A17/A18/A19/A20
(failed/crashed/quarantined scrub provably yields a held/quarantined uid, never a dirty
re-lease) **+ the C1-RESIDUAL control A21** (a stopped/traced non-zombie survivor provably
quarantines, wired off a non-binary classifier). **C2 controls:** A16 (end-state non-exposure)
**+ the C2-RESIDUAL controls A22/A23** (no read succeeds across the whole provisioning
transition; the planner can never even emit a control-plane-touching op). **Restart controls:**
A8′/A11′/A19. A clearing verdict requires the hard core proven (A1–A5, carried), the
lease/scrub/reaper complete with the **tightened** postcondition proof (A8′–A11′, A17–A21), the
ACL **exact across the full transition** (A13, A16, A22, A23), and no standing falsifier
challenge.

---
<sub>Holder leading proposal — RFC 0168 P0 `falsification_gate` design run, **REVISION v3**.
Discharges the two v2 cycle-1 residual constraints: **C1-RESIDUAL** (`OQ2-SCRUB-POSTCONDITION`)
by tightening the scrub-postcondition predicate **P1** to *zero `pool_uid`-owned tasks except
true zombies/dead tasks* — every non-zombie state (`T`, `t`, unknown, or an unreadable
still-present PID) **blocks** `returned` and finalizes `quarantined` with `lane_uid_scrub_failed`
— wired to a **new** 3-way classifier `classifyPoolUIDTaskState` (explicitly NOT the binary
`processZombie`, `tmux_liveness.go:599-614`), recording observed PIDs + `/proc` states in
`scrub_proof`/`scrub_failure`, with the new negative test
`TestStoppedOrTracedUIDProcessBlocksReturn` (A21); and **C2-RESIDUAL**
(`OQ4-ACL-PROVISIONING-TRANSITION`) by making the **allowlist / exclude-at-traversal**
provisioning form **MANDATORY** (the group grant never touches `.striatum/`/`.git/`/token-cache,
even transiently), **forbidding** `setfacl -R …:rX <repoRoot>`-then-strip on a live repo,
extending A16 into the transition test
`TestPoolACLProvisioningNeverTransientlyExposesScratch` (A22), and adding a deterministic
ACL-planner guard `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists` (A23). Carries the
v1-proven hard core (HC-A1..A5), the v2-credited C1 durable four-state lease machine and C2 final
`.striatum/`-excluding ACL end-state, and OQ1/OQ3/OQ5/OQ6 + the narrowing invariant
**unregressed** (the v1 OQ1 exhaustion caveat and OQ6 stale-store contingency stay closed, now
more tightly). Build-run target: runtime migration `0046+` (`0045` reserved by RFC 0170 P0
`cullable_entity`; not hardcoded) + owner-bundle bump `owner/0023+` for
`striatumd.lane_uid_leases`. Local-first boundary intact: one host, one PostgreSQL, one daemon as
single writer; no hosted services. Both fixes are narrowings — only surface is removed. This is
the published claim the v3 falsifiers re-attack.</sub>
