# HOLDER — RFC 0168 P0 falsifiable implementation spec (REVISION v4: discharge the SINGLE standing v3 residual — C2-RESIDUAL (`OQ4-ACL-PROVISIONING-TRANSITION`), the per-supervisor MCP-bearer-path realness — by re-rooting the live bearer writer under `.striatum/scratch/<supervisor_id>/` as a required P0 build step, correcting the false v3 source citation, making A22 derive-and-exercise the live resolved path, and making the provider/token-cache forbidden set explicit — carrying the v1-proven hard core, the v3-DISCHARGED C1-RESIDUAL, and every v1/v2/v3 carry-forward unregressed)

author: holder-author-001

> This is the **v4 revision** of the RFC 0168 P0 `falsification_gate` proposal. The
> base is the **v3 SPEC** (`context/v3_HOLDER.md`); this is a revision of it, a
> **narrow bearer-path fix, NOT a rewrite, and NOT a re-litigation of the v1/v2/v3
> base.** v1 **proved the structural hard core** (a per-lane uid dissolves
> `BC1-W1-ORACLE` on this host under Yama `ptrace_scope=1`) and resolved
> OQ1/OQ3/OQ5/OQ6; v2 **credited the structural halves** of both binding constraints
> (the C1 durable four-state lease machine and the C2 final `.striatum/`-excluding
> ACL end-state); v3 **DISCHARGED C1-RESIDUAL** (the fail-closed scrub-postcondition
> predicate P1 → `classifyPoolUIDTaskState`, NOT the binary `processZombie`) and
> credited the C2 *procedure* fix (mandatory allowlist + the A23 planner guard), but
> the v3 cycle-2 adjudicator left **one standing residual** — **C2-RESIDUAL
> (`OQ4-ACL-PROVISIONING-TRANSITION`)** — `open` and exhausted the v3 cycle, routing
> it to the operator. This revision **discharges that single residual** and **carries
> forward, unregressed, everything v1, v2, AND v3 cleared — including the already-discharged
> C1-RESIDUAL.** The direction (per-lane pooled OS uid) is ratified (D261, 2026-06-24)
> and is **not** relitigated. **Every source citation below was re-verified against
> the current worktree HEAD (`f63b895f`)** while authoring this revision (see §Source
> re-verification); the two false v3 bearer citations (`mcpconfig.go:241/266`) are
> **corrected to the live writer** (`mcpconfig.go:550-559`). This is the published
> claim the two v4 falsifiers re-attack.

---

# Addressing the v3 residual (the auditable revision map)

The v3 cycle-2 ledger (`context/v3_LEDGER_cycle_2.md`) recorded **exactly one** `open`
finding — `OQ4-ACL-PROVISIONING-TRANSITION` — and **four** `accepted` carry-forwards
(`HC-ORACLE-INTACT`, `OQ2-SCRUB-POSTCONDITION` DISCHARGED, `OQ-CARRY-FORWARD-INTACT`).
This revision resolves the one `open` finding and regresses **none** of the four.

| v3 verdict ground | This revision | Where |
| --- | --- | --- |
| **C2-RESIDUAL / `OQ4-ACL-PROVISIONING-TRANSITION` — `open`, verdict-driving.** The v3 SPEC's *procedure* half (mandatory allowlist/exclude-at-traversal; raw-recursive-root forbidden; A23 planner guard) was **credited**, but its C2 **final state + A22 transition test were NOT source-true for the live MCP bearer.** v3 falsely asserted (marked `CONFIRMED`, citing `mcpconfig.go:241/266`) that the `0600` bearer already lives under `.striatum/scratch/<supervisor_id>/`. **Source-false:** the live bearer is written by `writeEphemeralMCPConfig` (`mcpconfig.go:550-565`) **directly under `.striatum/scratch/` ROOT** and does **not** thread `STRIATUM_SUPERVISOR_ID`. Two consequences: **(1)** the v3 final ACL (`.striatum/scratch → --x`-only; `rwx` only on `.striatum/scratch/<supervisor_id>`) **breaks lane launch** — `scratch_acl.go:42-49` grants `rwx`+default on `.striatum/scratch` *itself* precisely because the writer `CreateTemp`s there (`#279`), so a faithful `--x`-only-root build makes `os.CreateTemp` fail `EACCES`; **(2)** A22 is **fake** — it plants a fixture at `.striatum/scratch/<S1>/lane-mcp-config-*.json` while the real bearer is at `.striatum/scratch/lane-mcp-config-*.json`. | **RESOLVED.** Four source-anchored parts: **(1)** a **REQUIRED P0 build step** re-roots `writeEphemeralMCPConfig` (`mcpconfig.go:550-565`) from `<repoRoot>/.striatum/scratch` to `<repoRoot>/.striatum/scratch/<supervisor_id>/` — threading `STRIATUM_SUPERVISOR_ID` exactly as `loop.go:289`/`:139-145` (the `pty.log` writer) and the gemini markers (`mcpconfig.go:245`/`:266`) already do — **before** re-keying the scratch ACLs, so the `--x`-only-scratch-root / `rwx`-on-supervisor-subdir final state is achievable **without** the `EACCES`/`#279` launch break. **(2)** A22 is made REAL — it **derives** the exact path the live writer resolves (set `STRIATUM_SUPERVISOR_ID`, call/share the live resolver) and asserts **no residual root-level `.striatum/scratch/lane-mcp-config-*` bearer** after the transition. **(3)** the provider/token-cache **forbidden top-level set is made EXPLICIT** (`.gemini`, `.claude`, `.codex`, configured credential caches) in the OQ4 allowlist + the A23 planner guard. **(4)** the false citation is **CORRECTED** — the bearer is `mcpconfig.go:550-559`, not `:241/266`. **A23 KEPT** (necessary, not sufficient). | **§OQ4.0 (the re-root build step)**, **§OQ4.1(a) (explicit forbidden set)**, **§OQ4.1(b) (re-keyed scratch ACLs)**, **§OQ4.3 (A22 made real + A23 kept)** |
| **Carried — `OQ2-SCRUB-POSTCONDITION` C1-RESIDUAL — `accepted`/DISCHARGED in v3** (the fail-closed three-way P1 → `classifyPoolUIDTaskState`, NOT `processZombie`) | **INTACT, carried verbatim, UNREGRESSED** — not reopened; no part of the bearer-path fix touches the scrub path | **§OQ2.3 / §OQ2.6, test A21** |
| **Carried — `HC-ORACLE-INTACT` — `accepted`** (the v1-proven hard core HC-A1..A5) | **INTACT, carried verbatim** | **§Part 1** |
| **Carried — `OQ-CARRY-FORWARD-INTACT` — `accepted`** (the C1 durable four-state lease machine; the C2 final `.striatum/`-excluding GROUP-ACL **end-state invariant**; OQ1/OQ3/OQ5/OQ6; the narrowing invariant) | **INTACT, carried verbatim, UNREGRESSED** — the C2 **end-state invariant is unchanged**; only the **bearer path that makes it launch-consistent** is newly source-anchored, and the scratch ACL is re-keyed to the (now-real) supervisor subdir | **§OQ1/§OQ3/§OQ4.1-2/§OQ5/§OQ6** |

The single new load-bearing claim is the **C2 bearer-path migration** (OQ4.0) and the
**real A22** that exercises it; everything else is the v3 text carried verbatim in
substance (the v4 falsifiers have v3 as required context). A regression in any carried
claim is itself a gate failure. The bearer-path fix is a **narrowing**: it only **moves
a `0600` file deeper into the lane's own per-supervisor private dir and removes the
`rwx` grant on the shared scratch root** — no new authority, no widened read.

---

# Part 1 — THE HARD CORE (CARRIED FORWARD UNREGRESSED from v1/v2/v3 §Part 1; re-verified, not reopened)

The whole RFC leans on one assertion: **a per-lane uid dissolves `BC1-W1-ORACLE` on this
host.** v1 proved it as four structural sub-claims + a residual-surface closure; v1, v2,
and v3 falsifiers all credited it and the adjudicators independently re-verified the
launch path. It is carried here **unchanged**. The exact attack: target lane runs as
`U_t`, sibling as `U_s ≠ U_t`; the launch path is `sudo -n -u <runAsUser> -- env -i …
tmux …` (`commandInvocationWithEnvFile`, `pty.go:98-112`), tmux invoked **bare** through
the same run-as path (`tmuxRunnerForSpec`→`RunAsTmuxRunner`, `pty.go:310-314`;
`tmux_liveness.go:125-149`) with a deterministic session name (`pty.go:620-633`) and
**no** `-S`/`TMUX_TMPDIR` (`sanitizedRunAsEnv`, `pty.go:120-155`).

- **HC-A1** — each lane's bare tmux lands on tmux's **default per-uid socket**
  `$TMUX_TMPDIR/tmux-<uid>/default` (`/tmp/tmux-<uid>`, dir `0700` owned by that uid,
  socket `srwx------`). `U_s` has no `--x` on `/tmp/tmux-<U_t>` ⟹ `connect(2)` fails
  `EACCES` before any tmux command parses; the same-uid-mutable oracle is gone. **Test
  A1:** `TestSiblingPoolUIDCannotRespawnTargetPane` (real-path).
- **HC-A2** — a `U_t`-owned `0600` file in a `0700` HOME is unreadable by `U_s` under
  POSIX DAC (the RFC 0143 Slice B reseal-token surface). **Test A2:**
  `TestSiblingPoolUIDCannotReadLaneOwnedResealToken`.
- **HC-A3** — cross-uid `ptrace`/`setns`/`open("/proc/<U_t>/…")` are denied by
  `ptrace_may_access` (matching-uid or `CAP_SYS_PTRACE` required), **independent of
  Yama**. This is the exact axis on which **namespace-inode binding failed** (D261).
  **Test A3:** `TestSiblingPoolUIDCannotPtraceOrSetnsOrReadProcSecrets` (asserts
  `ptrace_scope=1` precondition).
- **HC-A4** — the daemon control socket reads kernel-attested `SO_PEERCRED {pid,uid}`;
  with a per-lane uid the accept predicate gains a **meaningful uid discriminator**
  (`peer.uid == U_t`), so a `U_s` peer is rejected before any pid/oracle reasoning.
  **Test A4:** `TestControlFrameAcceptsOnlyLeasedLaneUID`.
- **HC-A5** — every residual same-uid surface is named and closed: the only common
  ancestor of two lanes is the trusted **daemon** (not a lane); the shared tmux server is
  now per-uid (HC-A1); world/group-readable paths are closed on every **private** surface
  (`0600`/`0700`, no group ACL — **see OQ4 for the exact `.striatum/` boundary, whose
  end-state is unchanged in v4**); daemon-bridging is lane-vs-daemon, not lane-to-lane.
  **Test A5:** `TestNoSharedSameUIDSurfaceBetweenPoolLanes`.

**Hard-core conclusion (carried):** the BC1-W1-ORACLE mechanism requires `U_s` to address
`U_t`'s tmux server (A1: denied), `ptrace`/`setns` into `U_t` (A3: denied), or read a
`U_t`-owned file (A2: denied) — all structurally closed by the different uid (A5: no
residual bridge). The kernel **attests** the connecting uid (A4). This is a **narrowing**.
*Nothing in v4 changes Part 1; the full per-claim proof is in v1/v2/v3 §Part 1.*

---

# Part 2 — THE SIX OPEN QUESTIONS

## OQ1 — Pool size + exhaustion (CARRIED UNREGRESSED, the v1 caveat CLOSED by C1)

**Carried unregressed.** There is no host-global concurrent-lane ceiling today
(`max_active_jobs` is per-workflow, default `0`=unlimited; `runreconcile_test.go:395`), so
a finite uid pool **introduces the first host-global ceiling** — a deliberate, named
consequence. Sizing: `N` = the host's max concurrent live **sessions with an attached
supervisor across all runs** (a lane holds a uid for its session lifetime, OQ2),
operator-chosen, runbook-documented. Exhaustion policy = **REFUSE, fail-closed, typed**:
when a launch needs a uid and none is free, the daemon refuses with a typed
`lane_uid_pool_exhausted` floor and leaves the job **queued/recoverable**; it never falls
back to the shared `striatum-lane` uid, never blocks-and-waits holding a lock, never
auto-`useradd`s. Tests **A6** (`TestLaneUIDPoolCeilingIsHostGlobalAcrossRuns`) and **A7**
(`TestLaneUIDPoolExhaustionRefusesTyped`) carry forward.

**The v1 caveat, closed by C1 (carried).** Exhaustion accounting reads the OQ2 quarantine
state: `free = N − count(active) − count(scrubbing) − count(quarantined)` — a
`quarantined` uid is counted **consumed**, the ceiling is honest, and **A20** asserts
exhaustion fires at the reduced ceiling rather than re-leasing a dirty uid. (The
C1-RESIDUAL fix only **adds** quarantine causes, so this caveat stays closed, *more*
tightly.)

## OQ2 — Lease/scrub/reaper lifecycle (CARRIED UNREGRESSED — C1-RESIDUAL DISCHARGED in v3, not reopened)

**Carried verbatim from the v3-DISCHARGED form.** The v3 cycle-2 adjudicator marked
`OQ2-SCRUB-POSTCONDITION` `accepted` (DISCHARGED) and the second-round Falsifier 1 landed
no material challenge. The bearer-path fix (OQ4.0) **does not touch the scrub path**, so
OQ2 is carried unchanged. The load-bearing C1 claims, restated for the no-regression
falsifier:

### OQ2.1 — A durable FOUR-state lease (CARRIED)

P0 adds the daemon-owned table **`striatumd.lane_uid_leases`** in daemon-owned PostgreSQL
(survives a `striatumd` restart, D094 / RFC 0043). The schema carries v1 fields **plus**
the C1 states and scrub-proof columns:

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
  scrub_failure  text,                     -- which proof failed + detail; for P1, the blocking PID + non-zombie state
  leased_at      timestamptz NOT NULL,
  scrub_started_at timestamptz,
  returned_at    timestamptz
);

-- A uid HELD (in any non-clean state) is exclusive HOST-WIDE.
CREATE UNIQUE INDEX uq_lane_uid_held
  ON striatumd.lane_uid_leases(pool_uid)
  WHERE state IN ('active','scrubbing','quarantined');
```

**Free predicate (carried).** A pool uid is allocatable iff it has **no** row in
`active|scrubbing|quarantined` (latest row `returned`, or never leased). The free set is
**derived from the table**, never held in memory (restart-survival). The partial unique
index makes any of the three mutually exclusive per uid.

### OQ2.2 — The allocate / scrub-begin / scrub-finalize TRANSACTION boundary (crash-safe; CARRIED)

Because the side-effecting scrub (external `sudo … kill/rm` + `/proc` probes) **cannot**
run inside a DB transaction (the same `#198` reasoning that moved liveness probes **out**
of the sweep tx, `recovery_auto.go:22-38`), the lifecycle is **three transactions** with
the scrub strictly **between** them: (1) **Allocate** (`tx_alloc` at `supervise.start`):
insert `{state='active', generation=(max generation for pool_uid)+1, …}` selecting a uid
whose latest state is `returned`/absent; `uq_lane_uid_held` makes a concurrent
double-allocate fail atomically. (2) **Scrub-begin** (`tx_scrub_begin`, guarded
`active→scrubbing`): on `session.close` (hooked into `stopSupervisorInTx`,
`supervision_control.go:557-637`) **or** when the reaper claims a dead/leaked lease —
**atomically removes the uid from the free set BEFORE any scrub command runs.** (3)
**Scrub + postcondition proof** (OUT of any DB tx): OQ2.3. (4) **Scrub-finalize**
(`tx_scrub_finalize`, guarded `state='scrubbing'`): proof passed ⟹ `→'returned'`,
`scrub_status='clean'`; proof failed ⟹ `→'quarantined'`, `scrub_status='failed'`, emit the
typed `lane_uid_scrub_failed` floor. A crash between (2) and (4) leaves the row durably
`scrubbing` and the recovery sweep re-drives it (OQ2.4).

### OQ2.3 — The scrub steps AND the postcondition PROOF (P1 — the C1-RESIDUAL fix, carried verbatim)

**Scrub commands** (all `sudo -n -u <pool_uid> --`, within the launch-as grant — OQ3): **S1**
per-uid kill domain `kill -KILL -1`; **S2** credential-store delete; **S3** HOME scratch +
per-lease ACL removal + worktree chown-back.

**The POSTCONDITION proof (NOT `exit==0`).** After S1–S3 the daemon **proves** the uid
domain is empty by observation, with bounded retry:

- **P1 — empty kill domain (the C1-RESIDUAL fix — carried verbatim).** Enumerate **all**
  `pool_uid`-owned PIDs by reading each `/proc/<pid>/status` `Uid:` line (real **and**
  effective uid) — a **new** owner-uid read (no existing supervisor reader reads
  `/proc/<pid>/status`) — and, for each PID, observe `/proc/<pid>/stat`'s **state char**
  via the new classifier `classifyPoolUIDTaskState` (OQ2.6). P1 passes **iff every**
  observed `pool_uid`-owned task is **`Reaped`**, and **no** task is `Live` or `Unknown`:

  > **P1 predicate (load-bearing):** *After bounded re-kill/re-probe, there are **zero**
  > `pool_uid`-owned tasks except true zombies / dead tasks (`/proc` state ∈ {`Z`,`X`,`x`}).
  > **ANY** observed non-zombie state — `R`, `S`, `D`, **`T` (stopped)**, **`t`
  > (tracing-stop)**, `I`, `W`, `P`, `K`, any **unrecognized** state char, **or an
  > unreadable/ambiguous** `/proc/<pid>/stat` for a still-present PID — **blocks**
  > `returned` and finalizes `quarantined` with a typed `lane_uid_scrub_failed`.*

  A `T`/`t` survivor has **not** exited (still holds the uid, memory, FDs,
  HOME/credential reachability, `SIGCONT`-resumable) ⟹ `Live` ⟹ **block**. A `Z`/`X` task
  holds no resources ⟹ tolerated (and **recorded**). A PID fully gone from the
  re-enumerated domain is a genuine exit (tolerated); an **unreadable** `/proc` read for a
  **still-listed** PID is `Unknown` ⟹ **fail-closed → block**. Observed PIDs + `/proc`
  state chars + classification are recorded in `scrub_proof` (pass) **and** `scrub_failure`
  (block).
- **P2** — no uid-owned tmux server (`connect(2)` to `/tmp/tmux-<pool_uid>/default` ⟹
  `ECONNREFUSED`/`ENOENT`). **P3** — credential store absent (`ENOENT`). **P4** — HOME
  scratch reset + reseal-token absent. **P5** — per-lease ACLs removed (no
  `.striatum/scratch/<supervisor_id>` ACL entry for `pool_uid`; ties to OQ4).

`returned` is reached **only** when P1–P5 all hold; **any** failed proof ⟹ `quarantined`.

### OQ2.4 — The reaper (CARRIED)

The recovery sweep (`HandleRecoveryAuto`, `recovery_auto.go:12`, scheduled by
`startRecoveryScheduler`, `main.go:869`, default 60s) is extended: **leaked-active**
(`active`, owning session dead, never ran `session.close`) → `active→scrubbing` →
scrub+proof; **stuck-scrubbing** (`scrubbing` crash) re-driven idempotently each sweep, a
typed `lane_uid_lease_stuck_scrubbing` doctor finding after `M` sweeps;
**quarantine remediation** — a `quarantined` row is **never** auto-returned; it clears only
via an explicit operator/recovery retry that re-runs the **same** scrub+proof and
transitions `quarantined→returned` **only on a clean proof**.

### OQ2.5 — Restart-survival (CARRIED)

The boot epoch is fresh-per-process and not persisted (`randomBootEpoch`, `main.go:782`);
`daemonInstanceID` is restart-stable (`main.go:728`). On restart no in-memory binding
survives — but the `lane_uid_leases` rows do. The daemon **derives** pool state from the
table; the free set is derived, never memory-held.

### OQ2.6 — The process-state classifier (CARRIED — the C1-RESIDUAL mechanism; explicitly NOT `processZombie`)

P1 is wired to a **new** helper, `classifyPoolUIDTaskState(pid int) → {Reaped, Live,
Unknown}`, that the build adds **alongside** — and **must not** replace P1's wiring with —
the existing binary `processZombie` (`tmux_liveness.go:599-614`). The existing helper reads
`/proc/<pid>/stat`, takes the state field after the last `)`, returns `true` **only** when
`"Z"`, and returns `false` on a **read error** — a binary classification that conflates
`T`/`t`/`R`/`S`/`D`/unknown/error into one *not-`Z`* bucket. The classifier reuses the
**parse** shape but **inverts the default**: anything not provably `Reaped` ({`Z`,`X`,`x`})
is blocking — `Live` for any recognized non-dead state (incl. `T`/`t`), `Unknown`
(fail-closed → block) for unreadable/malformed-still-present or unrecognized chars. A spec
note + the A21 test guarantee the build does **not** wire P1 to `processZombie` or
`!processZombie`.

## OQ3 — Provisioning ownership (CARRIED UNREGRESSED)

Host-setup runbook artifact (static pool); the daemon gets **no** uid-lifecycle authority.
The pool is pre-created by the operator runbook (extending `docs/how-to/lane-sandbox.md`).
The daemon **leases** from a static set; it never `useradd`/`userdel`s. The sudoers grant
widens from one user to a runas **group** (`striatumd ALL=(%striatum-lanes) NOPASSWD: ALL`,
still **launch-only**; `lane-sandbox.md:94`). The daemon's only new exercise of that grant
is the **scrub** (`sudo -n -u <pool_uid> -- kill/rm` — within the launch-as authority it
already holds). **Test A12** (`TestDaemonHoldsNoUIDLifecycleAuthority`) carries forward.

## OQ4 — ACL interaction (final end-state CARRIED from v2/v3; **the C2-RESIDUAL bearer-path realness now DISCHARGED**)

**What is carried (the v2/v3-credited final ACL end-state — do NOT reopen):** the hard
`.striatum/` boundary invariant; the mandatory allowlist / exclude-at-traversal source
grant; the raw-recursive-root prohibition; the A23 planner guard; the per-leased-uid `--x`
traverse re-keyed from `scratch_acl.go`; the chowned worktree; per-lease ACLs removed on
scrub. **What v4 changes (the C2-RESIDUAL discharge):** **(0)** the live MCP bearer writer
is re-rooted under `.striatum/scratch/<supervisor_id>/` as a **required P0 build step**, so
the final scratch ACL is launch-consistent; **(1.a)** the forbidden provider/token-cache
top-level set is made **explicit**; **(1.b)** the scratch ACL is re-keyed to the (now-real)
per-supervisor subdir; **(3)** A22 is made a **real** transition test over the live
resolved path. **The OQ4 end-state invariant itself is unchanged.**

### OQ4.0 — The MCP bearer-path migration (NEW — the C2-RESIDUAL mechanism; a REQUIRED P0 build step)

**The v3 error (corrected here).** v3 asserted — and marked `CONFIRMED`, citing
`mcpconfig.go:241/266` — that the `0600` MCP bearer already lives under
`.striatum/scratch/<supervisor_id>/`. **This is source-false.** The live bearer (the
`Authorization: Bearer`-carrying `lane-mcp-config-*.json`) is created by
**`writeEphemeralMCPConfig` (`go/pkg/agentloop/mcpconfig.go:550-580`)**:

```go
// mcpconfig.go:550-565 (live, HEAD f63b895f)
func writeEphemeralMCPConfig(repoRoot, endpoint, bearer string) (string, func(), error) {
	...
	dir := filepath.Join(repoRoot, ".striatum", "scratch")          // :555  ROOT, no supervisor id
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		dir = os.TempDir()                                          // :557  tmp fallback
	}
	f, err := os.CreateTemp(dir, "lane-mcp-config-*.json")          // :559  created in scratch ROOT
	...
	if err := os.Chmod(path, 0o600); err != nil { ... }            // :565
	...
}
```

It is created **directly under `<repoRoot>/.striatum/scratch/` ROOT** and does **not**
thread `STRIATUM_SUPERVISOR_ID`. Its caller — the claude case of
`injectLaneMCPConfigWithRewritePath` (`mcpconfig.go:79`) — passes only
`(repoRoot, endpoint, token.Token)`. The v3-cited `:241/266` are a **different** writer:
`:241` is `os.WriteFile(path, body, 0o600)` of the gemini-cli **`settings.json`** (error
string `"write gemini settings"`), and `:266` is `scratchDir = filepath.Join(repoRoot,
".striatum", "scratch", supervisorID)` for the gemini **backup/created markers**
(`settings.json.backup` / `settings.json.created`) — gemini-cli settings markers, **not the
bearer**. Only the `pty.log` half (`loop.go:139-145`, `:289`) was ever source-true.

**The required P0 build step (re-root the bearer writer).** Modify
`writeEphemeralMCPConfig` (`mcpconfig.go:550-565`) to resolve a **per-supervisor** scratch
dir, threading the supervisor id **exactly as the two existing supervisor-scoped writers
already do**:

- `loop.go:289` resolves `os.Getenv("STRIATUM_SUPERVISOR_ID")` and passes it to
  `resolveTrajectoryLogPath` (`loop.go:139-145`), which returns
  `filepath.Join(repoRoot, ".striatum", "scratch", supervisorID, "pty.log")`;
- the gemini path reads the same env at `mcpconfig.go:245` and joins
  `.striatum/scratch/<supervisorID>` at `:266`.

Concretely, the re-rooted writer resolves its directory in this **exact precedence** (so
A22 can derive it deterministically):

1. `supervisorID := strings.TrimSpace(os.Getenv("STRIATUM_SUPERVISOR_ID"))`. When
   non-empty, `dir := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)`;
   `os.MkdirAll(dir, 0o700)`; `os.CreateTemp(dir, "lane-mcp-config-*.json")`; chmod `0600`.
2. **Fallback** (supervisorID empty, **or** the supervisor subdir is not creatable):
   `os.TempDir()` — a `0600` file outside the repo entirely, unreadable cross-uid by
   HC-A2. **The fallback is NEVER `<repoRoot>/.striatum/scratch` ROOT** (that root path is
   exactly the residual-bearer exposure A22 forbids). The current `:556-558`
   stat-then-`os.TempDir()` fallback shape is preserved, only its **primary** changes from
   the scratch root to the supervisor subdir.

The supervisor subdir **already exists pre-launch**: `supervision_control.go:112-115`
`MkdirAll`s `<repoRoot>/.striatum/scratch/<supervisorID>` (`0700`, owned by the daemon
owner) **before** the lane launches, and `:125` then calls `prepareScratchACLsForLaneUser`.
So the re-rooted writer `CreateTemp`s into a directory the daemon has already created and
ACL-granted to the leased uid (OQ4.1(b)) — no `EACCES`, no `#279` break.
`RewriteEphemeralMCPConfig` (`mcpconfig.go:518-548`, the `#323` rotation path) operates on
the already-resolved `path` via `dir := filepath.Dir(path)`, so it follows the bearer
wherever it now lands — **no change required there** (and `loop.go:670` rewrites that same
resolved path). The two existing focused tests that today assert the **root**-scratch shape
(`mcpconfig_test.go:107-128` prefix `.striatum/scratch`; `:163-170`/`TestRewriteEphemeralMCPConfigSwapsEndpointAndToken`
creating only `.striatum/scratch`) must be **updated** to set `STRIATUM_SUPERVISOR_ID` and
assert the **supervisor-subdir** shape, so the test surface no longer accepts root-scratch
placement.

**Ordering (load-bearing).** This writer move is the **prerequisite** that makes the
re-keyed scratch ACL (OQ4.1(b)) launch-consistent: it lands **before** `.striatum/scratch`
is pushed down to `--x`-traverse-only, so by the time the root loses the lane-writable
grant the bearer is already created one level deeper, in the lane's own `rwx` supervisor
subdir.

### OQ4.1 — Two ACL domains with a hard boundary at `.striatum/` (end-state CARRIED; forbidden set EXPLICIT; scratch ACL RE-KEYED)

**(a) Shared source/artifact tree → group `rX`, via the MANDATORY allowlist /
exclude-at-traversal form (CARRIED) with the forbidden top-level set made EXPLICIT.** The
pool group `striatum-lanes` (OQ3) gets recursive read/traverse + default on the repo's
**shared work product only**. The current single-lane helper (`provisionCommitteeACLs`,
`repo_acl.go:97-140`) applies `setRepoACL` (`= setfacl -R`, `:25-31`) over `repoRoot`
**and** `.striatum/worktrees` (`:130-135`) — a raw recursion that **descends through
`.striatum/`** and is exactly the shape the pool/group form **must NOT** replicate.

- **MANDATORY form (carried):** the provisioner **enumerates the top-level entries** of
  `<repoRoot>` (`readdir`), **prunes the forbidden set**, and applies `setfacl -R -m
  g:striatum-lanes:rX -m d:g:striatum-lanes:rX` to **each remaining source/artifact entry
  individually**. Because `.striatum/` (and every other forbidden path) is a **sibling** of
  the source entries — never one of them and never a descendant of one — the group entry is
  **never added to it, not even transiently.** **No default ACL is set on `<repoRoot>`
  itself.**
- **The forbidden top-level set is EXPLICIT (the C2-RESIDUAL sharpening — NEW in v4).** The
  pruned set is an **enumerated allowlist-complement**, not a derived heuristic, so a
  re-provision cannot sweep a provider auth path into the source allowlist. It contains, at
  minimum: **`.striatum`** (the control-plane root: MCP bearer, PTY log, per-supervisor
  scratch, worktrees), **`.git`**, **`.gemini`** (agy/gemini-cli settings + bearer),
  **`.claude`** (claude provider config + `~/.claude/.credentials.json` analogue when
  repo-local), **`.codex`** (codex `config.toml`), and **every configured credential cache**
  — the resolved `CLAUDE_CONFIG_DIR` store and any provider token-cache path the launch
  resolves (OQ6). New top-level entries default to **forbidden unless explicitly
  allowlisted** is the safe direction; at minimum the provider/token-cache set above is
  enumerated and pruned by name. A23 (OQ4.3) enforces this on the **planner output**: any
  `g:striatum-lanes` op targeting a forbidden path is rejected before apply.

  > **OQ4 invariant (UNCHANGED end-state, load-bearing):** *No path under
  > `<repoRoot>/.striatum/` (nor `<repoRoot>/.git/`, nor any provider/token-cache path —
  > `.gemini`/`.claude`/`.codex`/configured caches) carries a `g:striatum-lanes` access
  > **or** default ACL entry — **before, during, OR after** provisioning, for existing or
  > future files.*

**(b) Control-plane / private / worktree → per LEASED uid only, removed on scrub (CARRIED,
RE-KEYED for the now-real bearer path).** No group ACL touches any of these; every grant is
keyed to the **currently leased** uid, applied at lease/launch, removed at scrub. The
re-keyed `scratchACLTargets` (`scratch_acl.go:42-49`) returns **three** targets (vs the
current two), applied by `prepareScratchACLsForLaneUser` (`scratch_acl.go:58-79`, called at
`supervision_control.go:125` after the `:115` `MkdirAll`), keyed to the **leased uid**:

- `.striatum/` → `u:<leased-uid>:--x` (traverse only) — re-keys `scratch_acl.go:46` from the
  fixed `laneUser` to the **leased** uid. An **unleased** pool uid gets **no entry** ⟹
  cannot even traverse `.striatum/`.
- `.striatum/scratch/` → `u:<leased-uid>:--x` (traverse only — **pushed down** from the
  current `rwx`+default at `scratch_acl.go:47`). This is now **SAFE** *because the bearer no
  longer lives here* (OQ4.0): a leased uid can traverse to its own supervisor subdir but
  cannot list/read/write the shared scratch root or sibling supervisors' dirs.
- `.striatum/scratch/<supervisor_id>/` → `u:<leased-uid>:rwx` + default ACL — the lane's
  **own** ephemeral private dir, where the **re-rooted** `writeEphemeralMCPConfig` (OQ4.0)
  now `CreateTemp`s the `0600` bearer and `loop.go:145` already writes `pty.log`.
  Per-supervisor, so another leased uid has no entry on it. The default ACL ensures the
  `CreateTemp`'d bearer + rotated rewrites inherit the leased-uid grant.
- `.striatum/worktrees/<id>/` → **chowned to the leased uid** at worktree creation; **no
  group ACL, no group default** on the worktrees root (replacing the
  `repo_acl.go:130-135` default-ACL-on-worktrees-root grant for the pool case).
- **PG isolation** covers every pool uid via a group reject rule (`local all
  %striatum-lanes reject` + loopback forms).

### OQ4.2 — Why the exact failing cases are closed (steady-state + transition)

**Steady-state (carried).** An **unleased** pool uid `U_x` has **no** ACL entry on
`.striatum/` ⟹ cannot traverse into `.striatum/scratch/<S1>/` — `open(2)` of S1's bearer
fails `EACCES` at the `.striatum` traverse. A **different leased** uid `U_2`:
`.striatum/scratch` is now `--x` (traverse-only, no `r`/list) and
`.striatum/scratch/<S1>/` carries an entry only for S1's leased uid ⟹ `EACCES`. No group
ACL ever touches `.striatum/`.

**Launch-consistency (the v3 break, now closed).** S1's own leased uid `U1` has
`u:U1:--x` on `.striatum`, `u:U1:--x` on `.striatum/scratch`, and `u:U1:rwx`+default on
`.striatum/scratch/S1`. Its re-rooted `writeEphemeralMCPConfig` `CreateTemp`s into
`.striatum/scratch/S1/` — a dir the daemon `MkdirAll`'d at `supervision_control.go:115` and
granted `U1` `rwx` on — so `os.CreateTemp` **succeeds**. The v3 `EACCES`/`#279` break (a
`--x`-only scratch root with a writer that `CreateTemp`s in that root) **cannot arise**,
because the writer no longer targets the root.

**Transition (the C2-RESIDUAL closure).** The mandatory provisioner never roots a grant at
`<repoRoot>` or any ancestor of `.striatum/`, and the scratch ACL only ever grants the
leased uid (never the group) on `.striatum/`-internal paths. There is **no instant** during
provisioning at which any `g:striatum-lanes` entry exists on `.striatum/`, and the bearer
is **never** placed at the shared `.striatum/scratch` root. `U_x`/`U_2` looping on
`open(2)` of S1's bearer get `EACCES` **before, during, and after** the provisioner runs
(A22, now exercising the **real** resolved path).

### OQ4.3 — The ACL-planner guard (A23 KEPT) + the real transition test (A22 MADE REAL)

**A23 (KEPT — necessary, not sufficient).** The pool ACL provisioner is split into a **pure
planner** (emits the ordered `{target_path, specs}` list) and an **applier**. A
deterministic guard over the **planner output** **fails** the plan (refuse-to-provision,
typed error) if any `g:striatum-lanes` operation: targets `<repoRoot>` (or any **ancestor
of `.striatum/`**) as a **raw recursive (`-R`) root** while `.striatum/` exists; targets
`.striatum/`, `.git/`, **`.gemini`/`.claude`/`.codex`/a configured credential-cache** path
(recursively or not); or sets a **default** (`d:`) `g:striatum-lanes` entry on a directory
with `.striatum/` as a descendant. The guard is **pure** (planned paths/specs, no root, no
`setfacl`), so A23 runs in CI. The v3 adjudicator credited A23 as the right shape for the
raw-recursive-root case; v4 only **adds the explicit provider/token-cache targets** to its
forbidden set. **A23 alone is NOT sufficient** — it proves the group grant never targets
the control plane; it does **not** prove the per-leased-uid scratch ACL is compatible with
the live bearer writer. That second property is what A22 now proves.

**A22 (MADE REAL — the C2-RESIDUAL fix).** The v3 A22 was fake: it hand-planted a fixture
at `.striatum/scratch/<S1>/lane-mcp-config-*.json` while the real bearer lands at
`.striatum/scratch/lane-mcp-config-*.json`. The v4 A22 **derives and exercises the exact
path the live writer resolves**:

1. **Derive, do not plant.** Set `STRIATUM_SUPERVISOR_ID=S1` and obtain the bearer path
   from the **live writer** — either by calling `writeEphemeralMCPConfig(repoRoot,
   endpoint, bearer)` directly, or by calling the shared path-resolver it uses (the build
   may factor the directory resolution into a small `resolveEphemeralMCPConfigDir(repoRoot,
   supervisorID)` helper that both the writer and the test call). Assert the resolved path
   is under `<repoRoot>/.striatum/scratch/S1/` and matches `lane-mcp-config-*.json`.
2. **No residual root bearer.** After the writer runs and the transition completes, assert
   **no** file matching `<repoRoot>/.striatum/scratch/lane-mcp-config-*` exists at the
   scratch **root** (glob the root, not the subdir) — the property the SEED names. If the
   build ever regressed the writer back to the root, this assertion fails.
3. **Before/during/after exposure.** With S1's bearer (real path) + `pty.log` + a token
   cache + a foreign worktree seeded, an **unleased** pool uid `U_x` **and** a
   **different-leased** uid `U_2` loop on `open(2)`/traversal of S1's bearer **before,
   during, AND after** the ACL provisioning transition; **no read ever succeeds**, and no
   `g:striatum-lanes` access-or-default entry ever appears under `.striatum/`.
4. **Launch-positive control.** S1's **own** leased uid `U1`, with the re-keyed ACLs
   applied, **successfully** `CreateTemp`s its bearer under `.striatum/scratch/S1/` (the
   `EACCES`/`#279` regression control — a faithful build must not break S1's own launch).

`TestPoolACLProvisioningNeverTransientlyExposesScratch` (A22) thus exercises the file the
live control plane actually uses, and `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists`
(A23) backstops the planner. Together they discharge C2-RESIDUAL: the procedure can never
emit a control-plane-touching op (A23) **and** the per-uid scratch ACL is provably
compatible with — and exercised against — the real bearer writer (A22).

## OQ5 — Attestation + recycle-confusion generation token (CARRIED UNREGRESSED)

Attestation records the **leased uid** (from the `lane_uid_leases` row); the PID start-token
(`ProcessStartToken`, `process_identity_linux.go:13`, driven by the liveness probe region
`tmux_liveness.go:387-452`) discriminates the **process**, the leased uid the **principal**.
The `lane_uid_leases.generation` (monotonic per uid, minted in `tx_alloc`) is folded into
the lane's capability material exactly as `MCPBootEpoch` is (`mutations.go:41-48`); the
daemon **refuses to attest, and refuses a control frame, when the presented generation ≠ the
live generation for that uid** — on **every** attestation **and** control-frame path. **Test
A14** (`TestRecycledUIDGenerationPreventsCrossLeaseConfusion`) carries forward.

## OQ6 — Per-uid credential store (CARRIED, contingency CLOSED by C1)

The RFC 0165 spawn-time hydrator (#583) targets the **leased** uid's HOME via
temp-file+rename. Hydration is **per-spawn** and the **scrub deletes** the per-uid store on
return (OQ2 S2, proven absent by P3) — fresh in, deleted out; `0600` owned by the leased uid
inside its `0700` HOME, unreadable by any other pool uid (HC-A2). A failed P3 ⟹
`quarantined`; with the C1-RESIDUAL fix, a stopped/traced survivor that could otherwise
re-open the store post-return is **also** caught by P1 ⟹ `quarantined`. The resolved
credential cache path is in the OQ4.1(a) **forbidden** top-level set, so a re-provision can
never group-grant it. **Test A15** (`TestPerUIDCredentialHydrateNoStaleNoLeak`) carries
forward.

---

# Part 3 — THE P0 SLICE (updated for the discharged C1-RESIDUAL / C2-RESIDUAL)

P0 is the minimum for a lane to run as its own pooled uid and safely own a `0600` reseal
token:

1. **Static pool, host-provisioned** (OQ3): N pool uids, `striatum-lanes` group, widened
   runas-group sudoers, per-uid PG reject, **and the revised OQ4 ACL** (group `rX` on
   shared source via the mandatory allowlist that prunes the **explicit** forbidden set
   `.striatum`/`.git`/`.gemini`/`.claude`/`.codex`/credential-caches; per-leased-uid
   `.striatum/` + `.striatum/scratch` traverse + per-supervisor-subdir `rwx` + chowned
   worktree); daemon holds no uid-lifecycle authority.
2. **The MCP bearer-path migration** (OQ4.0 — the C2-RESIDUAL fix, a REQUIRED build step):
   re-root `writeEphemeralMCPConfig` (`mcpconfig.go:550-565`) from `<repoRoot>/.striatum/scratch`
   to `<repoRoot>/.striatum/scratch/<supervisor_id>/` (threading `STRIATUM_SUPERVISOR_ID`
   as `loop.go:289`/gemini-markers do), **before** re-keying the scratch ACLs; tmp fallback
   preserved, scratch-root **never** a target. Update the focused agentloop tests to the
   supervisor-subdir shape.
3. **Daemon-owned `lane_uid_leases` table** (OQ2): the **four-state** machine, the partial
   **held-unique** index, the generation token, **persisted** (restart-survival).
4. **Allocation + host-global admission ceiling** (OQ1): lease a free uid at
   `supervise.start`; refuse `lane_uid_pool_exhausted` when none is free.
5. **Return + scrub + PROOF + reaper** (OQ2): the allocate/scrub-begin/scrub-finalize
   boundary; S1–S3 scrub + P1–P5 postcondition proof on `session.close`, **with P1
   rejecting every non-zombie `pool_uid`-owned task** (the C1-RESIDUAL fix, wired to
   `classifyPoolUIDTaskState`); the sweep reaps leaked-active and re-drives stuck-scrubbing;
   quarantine-on-failed-proof with a doctor surface and operator retry.
6. **Attestation binds uid + generation** (OQ5); **per-uid credential hydration** (OQ6),
   scrub deletes on return (proven by P3).

With (1)–(6), RFC 0143 Slice B reduces to *"write a session-scoped reseal token owned by
the leased uid, `0600`"* — safe by HC-A2.

**Build-run target (per the SEED).** The new table lands in the **next FREE runtime-migration
slot** — runtime migrations currently end at `0044_deploy_cursor.sql`, and **`0045` is
reserved by the concurrent RFC 0170 P0 work**, so `lane_uid_leases` takes **`0046+` (do NOT
hardcode `0045`; the build picks the next free slot at implementation time)** — plus an
**owner-bundle bump** for the daemon-owned `striatumd.lane_uid_leases` table (owner
migrations currently end at `owner/0022_operator_identity_run_attribution.sql`, so the bump
is `owner/0023+`, granting `striatumd` ownership/grants — the same additive mechanism as the
prior owner-bundle versions).

**Seams deferred to later slices (named, not dropped):** daemon-managed dynamic uid
create/destroy; automated pool autogrow/resizing; the reseal-token write itself (RFC 0143
Slice B); cross-host/multi-host pooling; non-tmux adapter parity for the per-uid kill
domain; non-claude adapter bearer-path parity (the agy/gemini settings writer already
supervisor-scopes its markers, `mcpconfig.go:266`; codex carries the bearer in
`STRIATUM_MCP_TOKEN` env, not a scratch file, so neither places a `0600` bearer at the
scratch root — the migration is required for the **claude** ephemeral `--mcp-config` writer).

**Local-first boundary preserved.** One host, one PostgreSQL (the single writer, D094 / RFC
0043), one daemon; every scrub/kill/probe is local `sudo -n -u <pool_uid>`; the pool is OS
users; no hosted service, cloud API, telemetry, or external persistence. The C2-RESIDUAL fix
is a **narrowing** — it moves a `0600` file deeper into the lane's own per-supervisor
private dir and removes the `rwx` grant on the shared scratch root; no new authority is
granted.

---

# Source re-verification (every load-bearing site CONFIRMED against current worktree HEAD `f63b895f`; the corrected bearer anchor pinned to the LIVE writer)

| Claim | Site | Status |
| --- | --- | --- |
| Run-as launch = `sudo -n -u <runAsUser> -- env -i …`; bare tmux; minimal env; deterministic session name | `pty.go:98-112,:120-155,:310-314,:620-633`; `tmux_liveness.go:125-149` | **CONFIRMED** (carried Part 1) |
| `leases` is a **job** lease; `uq_active_resource_lease` partial-unique on active — the shape `lane_uid_leases` mirrors + the four states + held-unique index | `0005_repo_local_workflow_state.sql:166-186` | **CONFIRMED** (carried OQ2.1) |
| **`processZombie` classifies `/proc` state as BINARY `Z`-or-not — reads `/proc/<pid>/stat`, takes the field after the last `)`, returns `true` only when `== "Z"`, returns `false` on a READ ERROR** (the C1-RESIDUAL trap) | `go/pkg/supervisor/tmux_liveness.go:599-614` | **CONFIRMED (LIVE)** — carried; P0 adds the NEW `classifyPoolUIDTaskState` 3-way classifier instead |
| P1's `/proc` parse shape (comm-skip via `LastIndex(")")`) proven by `processZombie` + `ProcessStartToken`; the liveness probe region | `processZombie` `tmux_liveness.go:599-614`; `ProcessStartToken` `process_identity_linux.go:13`; probe region `tmux_liveness.go:387-452` | **CONFIRMED (LIVE)** — carried; the per-PID owner-uid read from `/proc/<pid>/status` `Uid:` is NEW |
| Live teardown closes the session when it holds no active lease — NO per-uid kill/cred/HOME scrub today | `supervision_control.go:557-637` | **CONFIRMED** (carried) — P0 hooks `tx_scrub_begin` + the scrub here |
| Recovery sweep is the reaper host (default 60s); expires leases; reaps idle orphans; #198 moves probes OUT of the sweep tx | `recovery_auto.go:12,:22-38`; `recovery_lease_expiry.go:86`; `recovery_decision_tree.go:1523`; `recovery_liveness_oracle.go:117`; `main.go:869,:80` | **CONFIRMED (LIVE)** (carried) |
| Boot epoch fresh-per-process, NOT persisted; `daemonInstanceID` restart-stable ⟹ derive the free set from the DB | `main.go:782` (`randomBootEpoch`), `:728` (`daemonInstanceID`) | **CONFIRMED (LIVE)** (carried) |
| `MCPBootEpoch` folded into capability material + rejected on mismatch (the model the OQ5 generation token reuses) | `mutations.go:41-48` | **CONFIRMED** (carried OQ5) |
| **`.striatum` carve-out: `u:<lane>:--x` traverse; `.striatum/scratch`:`u:<lane>:rwx`+default — granted EXACTLY because the bearer `CreateTemp`s in scratch root (#279)** | `scratch_acl.go:31-48` (comment + targets `:42-49`); `prepareScratchACLsForLaneUser:58-79` | **CONFIRMED (LIVE)** — P0 re-keys to the leased uid AND pushes `--x` down to `.striatum/scratch`, adding `rwx`+default on `.striatum/scratch/<supervisor_id>` (safe **because OQ4.0 moves the bearer there**) |
| **CORRECTED — the `0600` MCP bearer is created by `writeEphemeralMCPConfig` DIRECTLY under `<repoRoot>/.striatum/scratch` ROOT, NOT a supervisor subdir, NOT threading `STRIATUM_SUPERVISOR_ID`** | `go/pkg/agentloop/mcpconfig.go:550-580` — `dir := filepath.Join(repoRoot,".striatum","scratch")` **`:555`**; `os.Stat`→`os.TempDir()` fallback **`:556-558`**; `os.CreateTemp(dir,"lane-mcp-config-*.json")` **`:559`**; chmod `0600` **`:565`**; caller (claude case) `mcpconfig.go:79` | **CONFIRMED (LIVE)** — **the v3 `:241/266` CONFIRMED citation was FALSE-for-bearer; corrected here.** P0 re-roots this writer to `.striatum/scratch/<supervisor_id>/` (OQ4.0) |
| **`mcpconfig.go:241/266` are the gemini-cli `settings.json` write + backup/created marker dir — NOT the bearer** | `:241` `os.WriteFile(path, body, 0o600)` (error `"write gemini settings"`); `:245` reads `STRIATUM_SUPERVISOR_ID`; `:266` `filepath.Join(repoRoot,".striatum","scratch",supervisorID)` for `settings.json.backup`/`.created` | **CONFIRMED (LIVE)** — the v3 misattribution corrected |
| **The supervisor-scoped writer the bearer move COPIES: the `pty.log` writer threads `STRIATUM_SUPERVISOR_ID` → `.striatum/scratch/<supervisorID>/pty.log`** | `loop.go:139-145` (`resolveTrajectoryLogPath`); `:289` (`os.Getenv("STRIATUM_SUPERVISOR_ID")`); `:299-300` (`MkdirAll` dir `0700` + `OpenFile` `0600`) | **CONFIRMED (LIVE)** — the exact threading pattern OQ4.0 applies to the bearer writer |
| **The per-supervisor scratch dir is created PRE-LAUNCH (so the re-rooted bearer writer `CreateTemp`s into an existing daemon-owned dir) and the #279 scratch ACLs are prepared** | `supervision_control.go:112` (`scratch := …/.striatum/scratch/<supervisorID>`), `:115` (`MkdirAll(scratch, 0o700)`), `:121` (#279 comment re `writeEphemeralMCPConfig` `CreateTemp`), `:125` (`prepareScratchACLsForLaneUser`) | **CONFIRMED (LIVE)** — the dir the re-rooted writer + the re-keyed `rwx` ACL target |
| **`RewriteEphemeralMCPConfig` (#323 rotation) is path-relative (`dir := filepath.Dir(path)`), so it follows the bearer wherever it lands — no change needed** | `mcpconfig.go:518-548`; rewrite call `loop.go:670` | **CONFIRMED (LIVE)** |
| **Current focused tests still encode the ROOT-scratch shape (must be updated to the supervisor-subdir shape)** | `mcpconfig_test.go:107-128` (prefix `.striatum/scratch`); `:163-170` `TestRewriteEphemeralMCPConfigSwapsEndpointAndToken` (creates only `.striatum/scratch`) | **CONFIRMED (LIVE)** — P0 updates these to set `STRIATUM_SUPERVISOR_ID` + assert the subdir |
| **Current repo ACL `setRepoACL` is `setfacl -R` (`:25-31`); `provisionCommitteeACLs` applies it over `repoRoot` AND `.striatum/worktrees` (`:130-135`) — the raw-recursive-root recursion into `.striatum/` the pool/group MANDATORY form must NOT replicate** | `repo_acl.go:21-31,:97-140` | **CONFIRMED** (carried) — P0 replaces with the allowlist/exclude-at-traversal planner + per-leased-uid worktree chown |
| Runbook: `.striatum/` is daemon-private (only lane exception = the per-job worktree) | `lane-sandbox.md:348-355,:77-79,:94` | **CONFIRMED** (carried) |
| Per-uid HOME + per-uid Claude credential store; RFC 0165 hydrator contract | `supervision_env.go:205-226`; `laneproviderauth/resolver.go:78-92`; RFC 0165 | **CONFIRMED** (carried OQ6) |
| No host-global concurrent-lane ceiling today (`max_active_jobs` per-workflow, default unlimited) | `runreconcile_test.go:395` | **CONFIRMED** (carried OQ1) |
| **Runtime migrations end at `0044`; `0045` reserved by RFC 0170 P0; owner migrations end at `0022`** | `go/pkg/db/sql/0044_deploy_cursor.sql`; `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql` | **CONFIRMED (LIVE)** — `lane_uid_leases` → runtime `0046+`, owner bump `owner/0023+` (not hardcoded) |
| D261 ratifies the per-lane-uid direction; rejects namespace-inode/AppArmor-hat/private-socket-alone | `docs/decisions/decision-log.md` (D261); RFC 0168 | **CONFIRMED** |

---

# Falsifiable-assertion index (the claims the v4 falsifiers re-attack)

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
| **A15** | per-uid credential hydration leaves no stale copy / no cross-uid leak; a failed P3 quarantines — **CARRIED** | `TestPerUIDCredentialHydrateNoStaleNoLeak` |
| **A16** | **(C2 end-state, CARRIED)** the pool ACL exposes no `.striatum/` control-plane to an unleased **or** different-leased pool uid; no `g:striatum-lanes` entry under `.striatum/` (existing + future) | `TestPoolACLDoesNotExposeOperationalScratchOrForeignWorktrees` + extended `make lane-isolation-check`/`doctor` |
| **A17** | **(C1, CARRIED)** a failed scrub postcondition quarantines and the dirty uid is **never** re-leased | `TestScrubFailureQuarantinesAndIsNeverReLeased` |
| **A18** | **(C1, CARRIED)** a crash during scrub leaves the uid durably `scrubbing` (held, not free); the sweep re-drives it | `TestCrashDuringScrubLeavesUIDHeldNotFree` |
| **A19** | **(C1, CARRIED)** quarantine survives a restart and is non-free; clears only via the proof-gated `quarantined→returned` retry | `TestQuarantineSurvivesRestartAndIsNonFree` |
| **A20** | **(C1, CARRIED)** exhaustion accounting excludes `scrubbing`+`quarantined`; fires typed at the reduced ceiling, never dirty reuse | `TestExhaustionExcludesScrubbingAndQuarantined` |
| **A21** | **(C1-RESIDUAL — DISCHARGED v3, CARRIED)** a `pool_uid`-owned task left **stopped (`T`)** or **tracing-stopped (`t`)** **blocks** `returned`: P1 (wired to `classifyPoolUIDTaskState`, NOT `processZombie`) classifies it `Live`, finalizes `quarantined` with `lane_uid_scrub_failed`, the uid is **not** allocated, the quarantine survives a boot-epoch rotation, and only the same P1–P5 proof (domain now all-`Reaped`/empty) clears it; the blocking PID + `/proc` state is recorded in `scrub_failure`. **Refuter:** a `T`/`t` (or unknown/unreadable-still-present) survivor passes P1 and the uid reaches `returned`. | `TestStoppedOrTracedUIDProcessBlocksReturn` |
| **A22** | **(C2-RESIDUAL — DISCHARGED v4, REAL transition test over the LIVE bearer path)** the pool ACL provisioner **never transiently** exposes `.striatum/`, AND the bearer lives in the lane's own per-supervisor subdir: A22 **derives** the bearer path from the **live writer** (set `STRIATUM_SUPERVISOR_ID=S1`, call/share `writeEphemeralMCPConfig`/its resolver — NOT a hand-planted fixture), asserts it resolves under `.striatum/scratch/S1/`, asserts **no residual root-level `.striatum/scratch/lane-mcp-config-*` bearer** after the transition, then with S1's real bearer / `pty.log` / token cache / foreign worktree seeded, an **unleased** `U_x` **and** a **different-leased** `U_2` looping on `open(2)`/traversal get `EACCES` **before, during, AND after** provisioning; AND S1's **own** leased uid `U1` **successfully** `CreateTemp`s its bearer under `.striatum/scratch/S1/` (the `#279`/EACCES launch control). **Refuter:** any cross-uid read succeeds at any instant; OR a group entry transiently appears under `.striatum/`; OR a bearer is created/rewritten directly under `.striatum/scratch` root; OR S1's own launch `CreateTemp` fails `EACCES`. | `TestPoolACLProvisioningNeverTransientlyExposesScratch` |
| **A23** | **(C2-RESIDUAL — KEPT, deterministic planner guard; necessary, not sufficient)** the pure ACL planner's output is **rejected** if any `g:striatum-lanes` op targets `<repoRoot>` (or any ancestor of `.striatum/`) as a raw recursive `-R` root while `.striatum/` exists, targets `.striatum/`/`.git/`/**`.gemini`/`.claude`/`.codex`/a configured credential-cache** path, or sets a `d:g:striatum-lanes` default on a dir with `.striatum/` as a descendant. **Refuter:** the planner emits — and the guard passes — a raw `setfacl -R …:rX <repoRoot>` plan, or a plan touching a provider/token-cache path. | `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists` |
| **A24** | **(C2-RESIDUAL — NEW, bearer-writer realness)** the live `writeEphemeralMCPConfig` resolves its directory to `<repoRoot>/.striatum/scratch/<STRIATUM_SUPERVISOR_ID>/` (threading the env var as `loop.go:289` does), falls back to `os.TempDir()` when the supervisor id is empty or the subdir is uncreatable, and **never** targets the `.striatum/scratch` **root**; the focused agentloop tests assert the supervisor-subdir shape (not the bare `.striatum/scratch` prefix). **Refuter:** the writer (or its updated focused test) places/accepts a bearer directly under `.striatum/scratch` root, or ignores `STRIATUM_SUPERVISOR_ID`. | `TestEphemeralMCPConfigResolvesUnderSupervisorScratchSubdir` (the updated `mcpconfig_test.go` cases) |

**Negative control:** the BC1-W1-ORACLE replay itself (A1). **C1 controls:** A17/A18/A19/A20
**+ the C1-RESIDUAL control A21**. **C2 controls:** A16 (end-state non-exposure) **+ the
C2-RESIDUAL controls A22/A23/A24** — A24 proves the live bearer writer is supervisor-scoped,
A22 exercises the real resolved path across the whole provisioning transition (incl. the
launch-positive control), A23 proves the planner can never emit a control-plane-touching op.
**Restart controls:** A8′/A11′/A19. A clearing verdict requires the hard core proven (A1–A5,
carried), the lease/scrub/reaper complete with the tightened postcondition proof (A8′–A11′,
A17–A21, carried-DISCHARGED), the ACL **exact across the full transition over the real
bearer** (A13, A16, A22, A23, A24), and no standing falsifier challenge.

---
<sub>Holder leading proposal — RFC 0168 P0 `falsification_gate` design run, **REVISION v4**.
Discharges the SINGLE standing v3 residual **C2-RESIDUAL** (`OQ4-ACL-PROVISIONING-TRANSITION`)
by **re-rooting the live MCP bearer writer** `writeEphemeralMCPConfig` (`mcpconfig.go:550-565`)
from `<repoRoot>/.striatum/scratch` ROOT to `<repoRoot>/.striatum/scratch/<supervisor_id>/` —
threading `STRIATUM_SUPERVISOR_ID` exactly as `loop.go:289`/`:139-145` (the `pty.log` writer)
and the gemini markers (`mcpconfig.go:245`/`:266`) already do — as a **REQUIRED P0 build step
BEFORE** re-keying the scratch ACLs, so the `--x`-only-scratch-root / `rwx`-on-supervisor-subdir
final state is launch-consistent (no `EACCES`/`#279` break; the subdir already exists from
`supervision_control.go:115`); making **A22** `TestPoolACLProvisioningNeverTransientlyExposesScratch`
**DERIVE and EXERCISE** the exact path the live writer resolves (not a hand-planted fixture) and
assert **no residual root-level `.striatum/scratch/lane-mcp-config-*` bearer** plus a
launch-positive control, with the new **A24** asserting the writer's supervisor-scoped resolution;
making the provider/token-cache **forbidden top-level set EXPLICIT** (`.gemini`/`.claude`/`.codex`/
credential-caches) in the OQ4 allowlist and the **A23** planner guard; and **CORRECTING** the
false v3 source citation (the bearer is `mcpconfig.go:550-559`, not `:241/266` — those are the
gemini-cli settings markers). Keeps **A23** (necessary, not sufficient). Carries the v1-proven
hard core (HC-A1..A5), the **v3-DISCHARGED C1-RESIDUAL** (the fail-closed three-way scrub
postcondition P1 → `classifyPoolUIDTaskState`, NOT the binary `processZombie`
`tmux_liveness.go:599-614`; `/proc` evidence; A21), the C1 durable four-state lease machine, the
C2 final `.striatum/`-excluding GROUP-ACL end-state invariant + the A23 raw-recursive-root
prohibition, and OQ1/OQ3/OQ5/OQ6 + the narrowing invariant **unregressed**. Build-run target:
runtime migration `0046+` (`0045` reserved by RFC 0170 P0; not hardcoded) + owner-bundle bump
`owner/0023+` for `striatumd.lane_uid_leases`. Local-first boundary intact: one host, one
PostgreSQL, one daemon as single writer; no hosted services. The fix is a narrowing — a `0600`
bearer moves deeper into the lane's own per-supervisor private dir and the shared-scratch-root
`rwx` grant is removed; no new authority. Re-verified against worktree HEAD `f63b895f`. This is
the published claim the v4 falsifiers re-attack.</sub>
