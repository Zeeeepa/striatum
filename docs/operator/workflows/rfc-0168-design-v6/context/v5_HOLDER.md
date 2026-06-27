# HOLDER — RFC 0168 P0 falsifiable implementation spec (REVISION v5: discharge the SINGLE standing v4 residual — the provider/credential-cache ANCESTRY sub-part of `OQ4-ACL-PROVISIONING-TRANSITION` — by making the provider credential-cache forbidden set ANCESTRY-AWARE and ENFORCED at three coordinated chokepoints, carrying the v1-proven hard core, the v3-DISCHARGED C1-RESIDUAL, the v4-DISCHARGED C2 bearer-path sub-part, and every v1/v2/v3/v4 carry-forward unregressed)

author: holder-author-001

> This is the **v5 revision** of the RFC 0168 P0 `falsification_gate` proposal. The
> base is the **v4 SPEC** (`context/v4_HOLDER.md`); this is a revision of it, a
> **narrow ancestry fix, NOT a rewrite, and NOT a re-litigation of the v1/v2/v3/v4
> base.** v1 **proved the structural hard core** (a per-lane uid dissolves
> `BC1-W1-ORACLE` on this host under Yama `ptrace_scope=1`) and resolved
> OQ1/OQ3/OQ5/OQ6; v2 **credited the structural halves** of both binding constraints
> (the C1 durable four-state lease machine and the C2 final `.striatum/`-excluding
> ACL end-state); v3 **DISCHARGED C1-RESIDUAL** (the fail-closed scrub-postcondition
> predicate P1 → `classifyPoolUIDTaskState`, NOT the binary `processZombie`); v4
> **DISCHARGED the C2 bearer-path sub-part** (re-rooted `writeEphemeralMCPConfig`
> under `.striatum/scratch/<supervisor_id>/`, made A22 real, corrected the false
> `mcpconfig.go:241/266` citation to `:550-559`). The v4 cycle-1 adjudicator left
> **one standing residual** — the **provider/credential-cache ANCESTRY** sub-part of
> **`OQ4-ACL-PROVISIONING-TRANSITION`** — `open`, and exhausted the v4 cycle, routing
> it to the operator. This revision **discharges that single residual** and **carries
> forward, unregressed, everything v1, v2, v3, AND v4 cleared — including the
> already-discharged C1-RESIDUAL and the already-discharged C2 bearer-path fix (kept
> verbatim in substance).** The direction (per-lane pooled OS uid) is ratified (D261,
> 2026-06-24) and is **not** relitigated. **Every source citation below was
> re-verified against the current run-branch worktree HEAD (`621312c4`)** while
> authoring this revision (see §Source re-verification). This is the published claim
> the two v5 falsifiers re-attack.

---

# Addressing the v4 residual (the auditable revision map)

The v4 cycle-1 ledger (`context/v4_LEDGER_cycle_1.md`) recorded **exactly one** `open`
finding — the **provider/credential-cache ANCESTRY** sub-part of
`OQ4-ACL-PROVISIONING-TRANSITION` — and marked **three** carry-forwards `accepted`
(`HC-ORACLE-INTACT`, `OQ2-SCRUB-POSTCONDITION` DISCHARGED, `OQ-CARRY-FORWARD-INTACT`),
with the **C2 bearer-path sub-part** of `OQ4` explicitly credited as DISCHARGED by both
falsifiers. This revision resolves the one `open` finding and regresses **none** of the
discharged work.

| v4 verdict ground | This revision | Where |
| --- | --- | --- |
| **`OQ4` provider/credential-cache ANCESTRY sub-part — `open`, verdict-driving.** The v4 "explicit forbidden provider set" was explicit by **name** but **not ancestry-aware**: it assumed every forbidden path is a **SIBLING** of a source top-level. A **configured** credential cache can **NEST UNDER** an allowlisted source top-level. Source-verified chain (re-confirmed at HEAD `621312c4`): `command_env CLAUDE_CONFIG_DIR` is allowed (`validateLaneCommandEnvKey` bars only empty/`PATH`/`STRIATUM_*`, `supervision_lane_config.go:440-451`), merged into the lane env (`applyLaneLaunchEnv`→`mergeEnvReplacing`, `supervision_env.go:110-118`), survives the run-as filter (`sensitiveRunAsEnvKey` drops only `TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN` substrings + a small denylist — `CLAUDE_CONFIG_DIR` matches none, `supervision_env.go:303-318`), and is taken authoritatively by the Claude branch of `ResolveCredential` (`laneproviderauth/resolver.go:78-85`) to place `.credentials.json` inside it. So `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` nests the cache **UNDER** allowlisted `docs/`, and the OQ4.1(a) mandatory form (top-level prune + recursive `setfacl -R` on each remaining source entry) sweeps a `g:striatum-lanes` access+default ACL onto that cache and its `.credentials.json`. A23 (v4) passes because its target is `docs/`, not a forbidden path, and its only ancestor rule covers `<repoRoot>`/ancestors of `.striatum/`. | **RESOLVED.** The forbidden set is made **ANCESTRY-ENFORCED** via **ONE coherent invariant** — *no provider credential cache is ever reachable by a `g:striatum-lanes` ACL* — enforced at the **three** chokepoints that each hold the information the others cannot: **(a) launch-time resolution-domain ban (the load-bearing, per-lane fix):** a configured provider credential/cache directory MUST resolve (real path) **OUTSIDE** the group-ACL domain `<repoRoot>`; the supervised launch REFUSES fail-closed (typed `lane_credential_cache_inside_repo`, job queued/recoverable) otherwise. This is load-bearing **because** the per-lane `command_env` cache path is known **only at launch**, while the group source ACL is provisioned **repo-globally** (`provisionCommitteeACLs`, `repo_acl.go:97-140`) with **no** per-lane knowledge — so a planner-only guard cannot see it. **(b) planner/A23 ancestry rejection (the static, CI-checkable backstop):** the pure ACL planner rejects any `g:striatum-lanes` op whose target is **equal-to, a descendant-of, OR an ANCESTOR-of** any *statically-configured* credential-cache path — the **same** ancestor semantics A23 already applies to `.striatum/`. **(c) physical (no-symlink-follow) recursive apply:** the applier recurses physically (`setfacl -RP` / explicit walk that does not descend through symlinks), so a symlink planted inside a source tree can never be followed into a cache. **A25** (new) exercises all three over the ledger's exact `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` case plus symlink and created-after-provisioning probes. The OQ4 invariant text is updated to state the forbidden set is ancestry-enforced. | **§OQ4.1(a) (ancestry-aware forbidden set + the 3-chokepoint mechanism)**, **§OQ4.1.1 (the launch-time resolution-domain ban, NEW)**, **§OQ4.3 (A23 extended ancestry + physical apply; A25 added)**, **§OQ4 invariant**, **§OQ6 (carry-forward sentence re-backed)** |
| **Carried — C2 bearer-path sub-part of `OQ4` — DISCHARGED in v4** (re-rooted `writeEphemeralMCPConfig` under `.striatum/scratch/<supervisor_id>/`; real A22; A24; corrected `:550-559` citation; launch-consistent final ACL) | **INTACT, carried verbatim in substance, UNREGRESSED** — the ancestry fix touches the **source-tree group ACL** and the **launch-time credential resolution**, **not** the scratch ACL re-key or the bearer writer | **§OQ4.0, §OQ4.1(b), §OQ4.2, §OQ4.3 (A22/A24)** |
| **Carried — `OQ2-SCRUB-POSTCONDITION` C1-RESIDUAL — `accepted`/DISCHARGED in v3** (the fail-closed three-way P1 → `classifyPoolUIDTaskState`, NOT `processZombie`) | **INTACT, carried verbatim, UNREGRESSED** — the ancestry fix does not touch the scrub path | **§OQ2.3 / §OQ2.6, test A21** |
| **Carried — `HC-ORACLE-INTACT` — `accepted`** (the v1-proven hard core HC-A1..A5) | **INTACT, carried verbatim** | **§Part 1** |
| **Carried — `OQ-CARRY-FORWARD-INTACT` — `accepted`** (the C1 durable four-state lease machine; the C2 procedure fix + A23 raw-recursive-root prohibition + the `.striatum/`/`.git/`-excluding GROUP-ACL **end-state invariant**; OQ1/OQ3/OQ5/OQ6; the narrowing invariant) | **INTACT, carried verbatim, UNREGRESSED** — the ancestry fix is the **newly-enforced extension** of the GROUP-ACL end-state invariant (SEED fix), not a change to any carried claim | **§OQ1/§OQ3/§OQ4.1-2/§OQ5/§OQ6** |

The single new load-bearing claim is the **provider/credential-cache ancestry invariant**
(OQ4.1(a)/OQ4.1.1) and the **real A25** that exercises it; everything else is the v4 text
carried verbatim in substance (the v5 falsifiers have v4 as required context). A regression
in any carried claim is itself a gate failure. The ancestry fix is a **narrowing**: it
**removes** a reachable surface (a credential cache can no longer be inside the group-ACL
domain, and the group grant can no longer escape its planned in-repo source targets) — no new
authority, no widened read.

---

# Part 1 — THE HARD CORE (CARRIED FORWARD UNREGRESSED from v1/v2/v3/v4 §Part 1; re-verified, not reopened)

The whole RFC leans on one assertion: **a per-lane uid dissolves `BC1-W1-ORACLE` on this
host.** v1 proved it as four structural sub-claims + a residual-surface closure; v1–v4
falsifiers all credited it and the adjudicators independently re-verified the launch path. It
is carried here **unchanged**. The exact attack: target lane runs as `U_t`, sibling as
`U_s ≠ U_t`; the launch path is `sudo -n -u <runAsUser> -- env -i … tmux …`
(`commandInvocationWithEnvFile`, `pty.go:98-112`), tmux invoked **bare** through the same
run-as path (`tmuxRunnerForSpec`→`RunAsTmuxRunner`, `pty.go:310-314`;
`tmux_liveness.go:125-149`) with a deterministic session name (`pty.go:620-633`) and **no**
`-S`/`TMUX_TMPDIR` (`sanitizedRunAsEnv`, `pty.go:120-155`).

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
  (`0600`/`0700`, no group ACL — **see OQ4 for the exact `.striatum/`/credential-cache
  boundary, whose end-state is unchanged in v5 except for the newly-enforced ancestry
  extension**); daemon-bridging is lane-vs-daemon, not lane-to-lane. **Test A5:**
  `TestNoSharedSameUIDSurfaceBetweenPoolLanes`.

**Hard-core conclusion (carried):** the BC1-W1-ORACLE mechanism requires `U_s` to address
`U_t`'s tmux server (A1: denied), `ptrace`/`setns` into `U_t` (A3: denied), or read a
`U_t`-owned file (A2: denied) — all structurally closed by the different uid (A5: no
residual bridge). The kernel **attests** the connecting uid (A4). This is a **narrowing**.
*Nothing in v5 changes Part 1; the full per-claim proof is in v1/v2/v3/v4 §Part 1.*

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
exhaustion fires at the reduced ceiling rather than re-leasing a dirty uid.

## OQ2 — Lease/scrub/reaper lifecycle (CARRIED UNREGRESSED — C1-RESIDUAL DISCHARGED in v3, not reopened)

**Carried verbatim from the v3-DISCHARGED form.** The ancestry fix (OQ4.1.1/OQ4.1(a)) **does
not touch the scrub path**, so OQ2 is carried unchanged. The load-bearing C1 claims, restated
for the no-regression falsifier:

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
  effective uid) — a **new** owner-uid read — and, for each PID, observe `/proc/<pid>/stat`'s
  **state char** via the new classifier `classifyPoolUIDTaskState` (OQ2.6). P1 passes
  **iff every** observed `pool_uid`-owned task is **`Reaped`**, and **no** task is `Live` or
  `Unknown`:

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

## OQ4 — ACL interaction (final end-state CARRIED from v2/v3/v4; **the provider/credential-cache ANCESTRY sub-part now DISCHARGED**)

**What is carried (the v2/v3/v4-credited final ACL end-state — do NOT reopen):** the hard
`.striatum/` boundary invariant; the mandatory allowlist / exclude-at-traversal source
grant; the raw-recursive-root prohibition; the A23 planner guard; the per-leased-uid `--x`
traverse re-keyed from `scratch_acl.go`; the chowned worktree; per-lease ACLs removed on
scrub; **and the entire v4 bearer-path discharge (OQ4.0/OQ4.1(b)/OQ4.2/A22/A24) — kept
verbatim in substance.** **What v5 changes (the provider/credential-cache ancestry
discharge):** **(1.a)** the forbidden provider/credential-cache set is made
**ANCESTRY-ENFORCED**, not merely explicit-by-name; **(1.1)** a NEW launch-time
**resolution-domain ban** keeps every configured credential cache OUTSIDE the group-ACL
domain; **(3)** A23 is extended with ancestor-of-credential-cache rejection and the applier
is pinned to **physical (no-symlink-follow)** recursion; a NEW **A25** exercises the whole
ancestry mechanism over the live env chain. **The OQ4 `.striatum/`/`.git/` end-state
invariant itself is unchanged; the credential-cache clause is newly enforced.**

### OQ4.0 — The MCP bearer-path migration (CARRIED VERBATIM from v4 — the C2 bearer-path discharge; a REQUIRED P0 build step)

**Kept verbatim in substance (DISCHARGED in v4, both falsifiers credited it).** The live
`0600` MCP bearer (the `Authorization: Bearer`-carrying `lane-mcp-config-*.json`) is created
by `writeEphemeralMCPConfig` (`go/pkg/agentloop/mcpconfig.go:550-580`) **directly under
`<repoRoot>/.striatum/scratch/` ROOT** (`dir := filepath.Join(repoRoot,".striatum","scratch")`
at `:555`; `os.Stat`→`os.TempDir()` fallback `:556-558`; `os.CreateTemp(dir,
"lane-mcp-config-*.json")` `:559`; chmod `0600` `:565`) and does **not** thread
`STRIATUM_SUPERVISOR_ID`. `mcpconfig.go:241/266` are the **gemini** `settings.json` write +
backup/created markers, **not** the bearer (the v3 misattribution, corrected in v4).

**The required P0 build step (re-root the bearer writer).** Modify `writeEphemeralMCPConfig`
to resolve a **per-supervisor** scratch dir, threading the supervisor id **exactly as the two
existing supervisor-scoped writers do** (`loop.go:289`/`:139-145`, the `pty.log` writer;
`mcpconfig.go:245`/`:266`, the gemini markers). Precedence (so A22 can derive it
deterministically): **(1)** `supervisorID := strings.TrimSpace(os.Getenv("STRIATUM_SUPERVISOR_ID"))`;
when non-empty, `dir := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)`;
`os.MkdirAll(dir, 0o700)`; `os.CreateTemp(dir, "lane-mcp-config-*.json")`; chmod `0600`.
**(2)** Fallback (supervisorID empty, **or** the subdir is not creatable): `os.TempDir()` — a
`0600` file outside the repo, unreadable cross-uid by HC-A2. **The fallback is NEVER
`<repoRoot>/.striatum/scratch` ROOT.** The supervisor subdir already exists pre-launch
(`supervision_control.go:112-115` `MkdirAll`s it `0700` before launch, then `:125`
`prepareScratchACLsForLaneUser`), so the re-rooted writer `CreateTemp`s into a daemon-created,
leased-uid-ACL'd directory — no `EACCES`, no `#279` break. `RewriteEphemeralMCPConfig`
(`mcpconfig.go:518-548`) is path-relative (`dir := filepath.Dir(path)`) so it follows the
bearer — no change there. The focused tests that encode the **root**-scratch shape
(`mcpconfig_test.go:107-128`; `:163-170`) are updated to set `STRIATUM_SUPERVISOR_ID` and
assert the **supervisor-subdir** shape.

**Ordering (load-bearing).** This writer move lands **before** `.striatum/scratch` is pushed
down to `--x`-traverse-only, so by the time the root loses the lane-writable grant the bearer
is already created one level deeper, in the lane's own `rwx` supervisor subdir.

### OQ4.1 — Two ACL domains with a hard boundary at `.striatum/` (end-state CARRIED; forbidden set ANCESTRY-ENFORCED; scratch ACL RE-KEYED per v4)

**(a) Shared source/artifact tree → group `rX`, via the MANDATORY allowlist /
exclude-at-traversal form (CARRIED) with the forbidden set made ANCESTRY-ENFORCED.** The
pool group `striatum-lanes` (OQ3) gets recursive read/traverse + default on the repo's
**shared work product only**. The current single-lane helper (`provisionCommitteeACLs`,
`repo_acl.go:97-140`) applies `setRepoACL` (`= setfacl -R`, `:21-31`) over `repoRoot` **and**
`.striatum/worktrees` (`:130-138`) — a raw recursion that **descends through `.striatum/`**
and is exactly the shape the pool/group form **must NOT** replicate.

- **MANDATORY form (carried):** the provisioner **enumerates the top-level entries** of
  `<repoRoot>` (`readdir`), **prunes the forbidden set**, and applies `setfacl -RP -m
  g:striatum-lanes:rX -m d:g:striatum-lanes:rX` to **each remaining source/artifact entry
  individually** (the `-P` physical-recursion requirement is OQ4.1(c) below). **No default
  ACL is set on `<repoRoot>` itself.**
- **The forbidden set is ANCESTRY-ENFORCED (the v5 discharge — the verdict-driving change).**
  v4 made the set explicit by **name** (`.striatum`, `.git`, `.gemini`, `.claude`, `.codex`,
  configured credential caches) but assumed every forbidden path is a **SIBLING** of a source
  top-level. That assumption is **FALSE** for a **configured** credential cache that nests
  **UNDER** an allowlisted source top-level (e.g. `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`,
  the ledger's exact case). v5 replaces "explicit-by-name siblings" with a single coherent,
  enforced **ancestry invariant** — *no provider credential cache is ever reachable by a
  `g:striatum-lanes` ACL* — guaranteed at **three coordinated chokepoints**, each holding
  information the others cannot:

  1. **Launch-time resolution-domain ban (OQ4.1.1 — load-bearing for the per-lane case).** A
     configured provider credential/cache directory MUST resolve, after real-path evaluation,
     **OUTSIDE** `<repoRoot>`. The supervised launch **refuses fail-closed** otherwise. This
     is the only chokepoint that knows the per-lane `command_env`-supplied cache path (the
     group source ACL is provisioned repo-globally, with no per-lane knowledge — see the
     timing argument below), so it is the **load-bearing** fix for the ledger's exact attack.
  2. **Planner/A23 ancestry rejection (the static backstop).** The pure ACL planner rejects
     any `g:striatum-lanes` op whose target is **equal-to, a descendant-of, OR an
     ANCESTOR-of** any *statically-configured* credential-cache path — the **same** ancestor
     semantics A23 already applies to `.striatum/`. This catches host/runbook-configured
     caches (paths known at provision time) and gives a **CI-checkable** enforcement surface.
  3. **Physical (no-symlink-follow) recursive apply (OQ4.1(c)).** The applier recurses with
     physical semantics so a symlink planted inside an allowlisted source tree can never be
     followed into a credential cache (or any out-of-tree path).

  The remaining named top-levels (`.striatum`, `.git`, `.gemini`, `.claude`, `.codex`) stay
  pruned as **siblings** exactly as in v4 (they are never descendants of a source entry). The
  **ancestry** machinery is what newly closes the *nested configured cache*. New top-level
  entries default to **forbidden unless explicitly allowlisted** (the safe direction, carried).

  > **OQ4 invariant (ANCESTRY-ENFORCED end-state, load-bearing — UPDATED in v5):** *No path
  > that is **equal-to, a descendant-of, OR an ancestor-of** any provider credential/cache
  > directory — and no path under `<repoRoot>/.striatum/` or `<repoRoot>/.git/`, nor any
  > provider top-level (`.gemini`/`.claude`/`.codex`) — carries a `g:striatum-lanes` access
  > **or** default ACL entry — **before, during, OR after** provisioning, for existing or
  > future files. This is guaranteed because (i) every configured provider credential cache
  > resolves OUTSIDE the group-ACL domain (`<repoRoot>`) — the launch refuses fail-closed
  > otherwise; (ii) the pure ACL planner rejects any `g:striatum-lanes` op equal-to /
  > descendant-of / ancestor-of any statically-configured credential-cache path; and (iii)
  > the applier recurses physically (no symlink-follow) so the grant never escapes its
  > planned in-repo source targets.*

  **Why the launch-time ban (1) is the load-bearing chokepoint (source-anchored timing
  argument).** The group source ACL is provisioned by `provisionCommitteeACLs`
  (`repo_acl.go:97-140`) at **`repo.init`/`repo.add`** time — a **repo-global, one-time**
  provisioning whose comment explicitly notes "repo.init must not fail registration"
  (`:112-114`); it has **no** visibility into any future lane's `command_env`. The cache path
  `CLAUDE_CONFIG_DIR` is supplied **per-lane** at **`supervise.start`** via `command_env`
  (`supervision_lane_config.go:440-451` admits it; `supervision_env.go:110-118` merges it;
  `:303-318` passes it through the run-as filter; `resolver.go:78-85` resolves
  `.credentials.json` under it). Because these are at **different times and scopes**, a
  planner-only guard (ledger option b alone) **cannot** see the per-lane cache path — so the
  per-lane attack is closed only by refusing the launch when the resolved cache lands inside
  `<repoRoot>` (1). The planner guard (2) and physical apply (3) close the *statically-known*
  and *symlink* residuals respectively. This is why v5 enforces the **one** invariant at
  **all three** points rather than picking a single insufficient one.

### OQ4.1.1 — The launch-time credential-cache resolution-domain ban (NEW — the ancestry-discharge primary mechanism)

A NEW guard runs on the supervised-launch path, **after** the lane launch env is assembled
(`applyLaneLaunchEnv`, `supervision_env.go:110-118`) and **before** the lane is launched. For
**every** provider credential/cache directory the resolver would resolve from the launch env
— concretely the directory parents of `ResolveCredential`'s result for each in-scope provider
(claude `CLAUDE_CONFIG_DIR`, codex `CODEX_HOME`, and any future provider config-dir env key
the resolver consults, `resolver.go:24-32,:57-98`) — the guard:

1. resolves the **real** directory: `cleaned := filepath.Clean(dir)`, then
   `real, _ := filepath.EvalSymlinks(cleaned)` (use `cleaned` when the path does not yet
   exist), so a symlinked cache is judged by its target;
2. computes `rel, err := filepath.Rel(repoRootReal, real)` against the **real** repo root,
   and treats the cache as **inside the group-ACL domain** iff `err == nil && rel != ".." &&
   !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)` (i.e. it
   is `<repoRoot>` itself or strictly beneath it);
3. when the cache is inside the domain, **REFUSES the launch fail-closed** with a typed
   `lane_credential_cache_inside_repo` floor and leaves the job **queued/recoverable** (the
   same fail-closed posture as `lane_uid_pool_exhausted`, OQ1, and `lane_uid_scrub_failed`,
   OQ2). The lane never launches, so the cache is **never created/used inside the source
   tree** and the group ACL never has anything to sweep.

The legitimate placement — the **per-uid HOME** credential store (`$HOME/.claude/`,
`$CODEX_HOME`, the RFC 0165 hydrator target, OQ6) — resolves **outside** `<repoRoot>` (the
leased uid's `0700` HOME is not a repo path), so it passes the guard and launches normally.
The ban is therefore a pure **narrowing**: it forbids only the previously-leaky case (a
provider credential cache inside the shared repo), never a legitimate per-uid store.

### OQ4.1(b) — Control-plane / private / worktree → per LEASED uid only, removed on scrub (CARRIED VERBATIM from v4, RE-KEYED for the real bearer path)

No group ACL touches any of these; every grant is keyed to the **currently leased** uid,
applied at lease/launch, removed at scrub. The re-keyed `scratchACLTargets`
(`scratch_acl.go:42-49`) returns **three** targets (vs the current two), applied by
`prepareScratchACLsForLaneUser` (`scratch_acl.go:58-79`, called at `supervision_control.go:125`
after the `:115` `MkdirAll`), keyed to the **leased uid**:

- `.striatum/` → `u:<leased-uid>:--x` (traverse only) — re-keys `scratch_acl.go:46` from the
  fixed `laneUser` to the **leased** uid. An **unleased** pool uid gets **no entry** ⟹ cannot
  even traverse `.striatum/`.
- `.striatum/scratch/` → `u:<leased-uid>:--x` (traverse only — **pushed down** from the
  current `rwx`+default at `scratch_acl.go:47`). SAFE *because the bearer no longer lives here*
  (OQ4.0).
- `.striatum/scratch/<supervisor_id>/` → `u:<leased-uid>:rwx` + default ACL — the lane's
  **own** ephemeral private dir, where the **re-rooted** `writeEphemeralMCPConfig` (OQ4.0)
  now `CreateTemp`s the `0600` bearer and `loop.go:145` writes `pty.log`. Per-supervisor.
- `.striatum/worktrees/<id>/` → **chowned to the leased uid** at worktree creation; **no group
  ACL, no group default** on the worktrees root (replacing the `repo_acl.go:130-138`
  default-ACL-on-worktrees-root grant for the pool case).
- **PG isolation** covers every pool uid via a group reject rule (`local all %striatum-lanes
  reject` + loopback forms).

### OQ4.1(c) — Physical (no-symlink-follow) recursive apply (NEW — closes the symlink-into-cache residual)

The applier of the group source grant recurses with **physical** semantics — `setfacl -RP`
(or an explicit tree walk that does **not** descend into symlinked directories and does not
dereference symlinks when applying) — over each allowlisted source top-level. A symlink
planted inside a source tree (e.g. `docs/cred -> <outside>/cache`) is therefore **never
followed**, so the grant cannot reach a credential cache (or any out-of-tree path) by symlink
traversal. Combined with the launch-time ban (OQ4.1.1, no cache inside the domain) and the
planner ancestry rule (OQ4.3, no plan targeting an ancestor/descendant/equal of a known
cache), the apply step provably stays within its planned in-repo source targets.

### OQ4.2 — Why the exact failing cases are closed (steady-state + transition + the ancestry case)

**Steady-state (carried).** An **unleased** pool uid `U_x` has **no** ACL entry on
`.striatum/` ⟹ cannot traverse into `.striatum/scratch/<S1>/` — `open(2)` of S1's bearer
fails `EACCES` at the `.striatum` traverse. A **different leased** uid `U_2`:
`.striatum/scratch` is now `--x` (traverse-only) and `.striatum/scratch/<S1>/` carries an
entry only for S1's leased uid ⟹ `EACCES`. No group ACL ever touches `.striatum/`.

**Launch-consistency (the v3 break, closed in v4, carried).** S1's own leased uid `U1`
`CreateTemp`s its re-rooted bearer into `.striatum/scratch/S1/` — a dir the daemon `MkdirAll`'d
and granted `U1` `rwx` on — so `os.CreateTemp` **succeeds**; the `EACCES`/`#279` break cannot
arise.

**The provider/credential-cache ancestry case (the v5 closure).** A lane that tries to nest
its credential cache under an allowlisted source top-level
(`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`) is **refused at launch** (OQ4.1.1) —
the cache is never created inside the domain, so the group `setfacl -RP docs/` has nothing to
sweep. A statically-configured cache path inside the domain is **rejected by the planner**
(OQ4.3 ancestor rule) — no plan that group-grants an ancestor (`docs/`), the cache, or a
descendant is ever applied. A symlink inside a source tree pointing at an out-of-tree cache is
**never followed** by the physical apply (OQ4.1(c)). A cache **created after** provisioning
cannot be inside the domain at all (OQ4.1.1 refuses any launch that would resolve it there),
and the planner refuses any **default** `g:striatum-lanes` ACL on an ancestor of a
statically-known cache (OQ4.3) — so no future file inherits the grant. There is **no instant**
— before, during, OR after — at which any `g:striatum-lanes` access-or-default entry exists on
a provider credential cache (A25).

### OQ4.3 — The ACL-planner guard (A23 EXTENDED with ancestry) + the real bearer transition test (A22/A24 CARRIED) + the real ancestry test (A25 NEW)

**A23 (EXTENDED — necessary, not sufficient).** The pool ACL provisioner is split into a
**pure planner** (emits the ordered `{target_path, specs}` list) and an **applier**. A
deterministic guard over the **planner output** **fails** the plan (refuse-to-provision, typed
error) if any `g:striatum-lanes` operation: targets `<repoRoot>` (or any **ancestor of
`.striatum/`**) as a **raw recursive (`-R`) root** while `.striatum/` exists; targets
`.striatum/`, `.git/`, `.gemini`/`.claude`/`.codex` (recursively or not); **or — NEW in v5 —
is equal-to, a descendant-of, OR an ANCESTOR-of any statically-configured credential-cache
path** (the same ancestor semantics the guard already applies to `.striatum/`); or sets a
**default** (`d:`) `g:striatum-lanes` entry on a directory with `.striatum/` **or a configured
credential cache** as a descendant. The guard is **pure** (planned paths/specs, no root, no
`setfacl`), so A23 runs in CI. **A23 alone is NOT sufficient** — it cannot see a per-lane
`command_env` cache path (closed by OQ4.1.1) and does not constrain the applier's symlink
behavior (closed by OQ4.1(c)).

**A22/A24 (CARRIED VERBATIM from v4 — the bearer-path discharge).** **A22**
(`TestPoolACLProvisioningNeverTransientlyExposesScratch`) **derives** the bearer path from the
**live writer** (set `STRIATUM_SUPERVISOR_ID=S1`, call/share `writeEphemeralMCPConfig`/its
resolver — not a hand-planted fixture), asserts it resolves under `.striatum/scratch/S1/`,
asserts **no residual root-level `.striatum/scratch/lane-mcp-config-*` bearer** after the
transition, runs the before/during/after cross-uid `open(2)` exposure loop, and includes the
launch-positive control (S1's own uid `CreateTemp`s successfully). **A24**
(`TestEphemeralMCPConfigResolvesUnderSupervisorScratchSubdir`) asserts the writer's
supervisor-scoped resolution + tmp fallback + never-root. Both unchanged in v5.

**A25 (NEW — the provider/credential-cache ANCESTRY discharge).** A25
(`TestProviderCredentialCacheNeverInGroupACLDomain`) exercises the **live env chain** and the
real ACL planner/applier over the ledger's exact case and the F1 probes:

1. **Resolution-ban (OQ4.1.1), the ledger's exact case.** Drive a lane launch with
   `command_env CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` through the **live**
   chain (`validateLaneCommandEnvKey` → `applyLaneLaunchEnv` → `sensitiveRunAsEnvKey` filter →
   `ResolveCredential`). Assert the resolved credential dir is detected **inside** `<repoRoot>`
   and the launch is **REFUSED** with typed `lane_credential_cache_inside_repo` (job
   queued/recoverable); assert **no** `g:striatum-lanes` access OR default ACL is ever applied
   under `docs/` for that lane. **Refuter:** the launch proceeds with a cache resolved inside
   `<repoRoot>`.
2. **Positive control.** A lane whose `CLAUDE_CONFIG_DIR` resolves **outside** `<repoRoot>`
   (or is unset → per-uid `$HOME/.claude`) launches **without** spurious refusal. **Refuter:**
   a legitimate out-of-repo cache is refused.
3. **Planner ancestry (OQ4.3).** Feed the pure planner a source allowlist containing `docs/`
   and a statically-configured credential-cache path `<repoRoot>/docs/.lane-auth/claude`.
   Assert the planner **REJECTS** the plan because a `g:striatum-lanes` op targets an
   **ancestor** (`docs/`) — and likewise rejects ops targeting the cache itself and a
   descendant. **Refuter:** the planner emits a passing plan that group-grants `docs/`.
4. **Symlink-follow (OQ4.1(c)).** Seed `<outside>/cache/.credentials.json` and a symlink
   `docs/cred -> <outside>/cache`; apply the group source ACL over `docs/` with physical
   recursion. Assert **no** `g:striatum-lanes` access OR default ACL on `<outside>/cache` or
   its `.credentials.json`, **before / during / after**. **Refuter:** the recursive apply
   follows `docs/cred` and ACLs the target.
5. **Created-after-provisioning.** Provision the group source ACL over source first; then
   create `docs/.lane-auth/claude/.credentials.json`. Assert **no** inherited default
   `g:striatum-lanes` ACL on it (held because OQ4.1.1 refuses any launch that would resolve a
   per-lane cache there, and OQ4.3 refuses any default ACL on an ancestor of a statically-known
   cache). **Refuter:** the late-created credential inherits a default `g:` ACL.

Together A22/A24 (bearer path), A23 (planner, raw-root + ancestry), A25 (live-chain ancestry +
symlink + timing), and the OQ4.1(c) physical apply discharge `OQ4-ACL-PROVISIONING-TRANSITION`
in full: the procedure can never emit a control-plane- or credential-cache-touching op (A23),
the per-uid scratch ACL is provably compatible with the real bearer writer (A22/A24), and a
provider credential cache can never be reachable by a group ACL — before, during, or after —
whether configured per-lane, statically, via a nested path, via a symlink, or created late
(A25).

## OQ5 — Attestation + recycle-confusion generation token (CARRIED UNREGRESSED)

Attestation records the **leased uid** (from the `lane_uid_leases` row); the PID start-token
(`ProcessStartToken`, `process_identity_linux.go:13`, driven by the liveness probe region
`tmux_liveness.go:387-452`) discriminates the **process**, the leased uid the **principal**.
The `lane_uid_leases.generation` (monotonic per uid, minted in `tx_alloc`) is folded into
the lane's capability material exactly as `MCPBootEpoch` is (`mutations.go:41-48`); the
daemon **refuses to attest, and refuses a control frame, when the presented generation ≠ the
live generation for that uid** — on **every** attestation **and** control-frame path. **Test
A14** (`TestRecycledUIDGenerationPreventsCrossLeaseConfusion`) carries forward.

## OQ6 — Per-uid credential store (CARRIED; the carry-forward sentence now RE-BACKED by the ancestry mechanism)

The RFC 0165 spawn-time hydrator (#583) targets the **leased** uid's HOME via
temp-file+rename. Hydration is **per-spawn** and the **scrub deletes** the per-uid store on
return (OQ2 S2, proven absent by P3) — fresh in, deleted out; `0600` owned by the leased uid
inside its `0700` HOME, unreadable by any other pool uid (HC-A2). A failed P3 ⟹
`quarantined`; with the C1-RESIDUAL fix, a stopped/traced survivor that could otherwise
re-open the store post-return is **also** caught by P1 ⟹ `quarantined`. **The resolved
credential cache is kept OUTSIDE the group-ACL domain by the OQ4 ancestry invariant** (the
launch refuses a cache resolving inside `<repoRoot>`, OQ4.1.1; the planner rejects any group
op ancestor-of/descendant-of/equal-to a configured cache, OQ4.3; the apply never follows
symlinks, OQ4.1(c)) — so **no** re-provision, **no** nested `CLAUDE_CONFIG_DIR` under an
allowlisted source top-level, and **no** late-created credential can ever group-grant it
(A25). *(This replaces the v4 sentence "the resolved credential cache path is in the OQ4.1(a)
forbidden top-level set" — the very claim the v4 ledger showed was not ancestry-enforceable;
it is now enforced, not merely asserted.)* **Test A15**
(`TestPerUIDCredentialHydrateNoStaleNoLeak`) carries forward.

---

# Part 3 — THE P0 SLICE (updated for the discharged C1-RESIDUAL / C2 bearer-path / provider-ancestry)

P0 is the minimum for a lane to run as its own pooled uid and safely own a `0600` reseal
token:

1. **Static pool, host-provisioned** (OQ3): N pool uids, `striatum-lanes` group, widened
   runas-group sudoers, per-uid PG reject, **and the revised OQ4 ACL** (group `rX` on shared
   source via the mandatory allowlist that prunes the named forbidden siblings AND is
   **ancestry-enforced** for provider credential caches via the launch-time resolution-domain
   ban + the planner ancestor rule + physical apply; per-leased-uid `.striatum/` +
   `.striatum/scratch` traverse + per-supervisor-subdir `rwx` + chowned worktree); daemon
   holds no uid-lifecycle authority.
2. **The MCP bearer-path migration** (OQ4.0 — the C2 bearer-path fix, a REQUIRED build step,
   CARRIED from v4): re-root `writeEphemeralMCPConfig` (`mcpconfig.go:550-565`) to
   `<repoRoot>/.striatum/scratch/<supervisor_id>/` **before** re-keying the scratch ACLs; tmp
   fallback preserved, scratch-root never a target; update the focused agentloop tests.
3. **The credential-cache resolution-domain ban + ancestry-aware ACL planner/applier**
   (OQ4.1.1/OQ4.1(c)/OQ4.3 — the provider-ancestry fix, NEW): refuse a launch whose resolved
   provider credential cache lands inside `<repoRoot>` (typed `lane_credential_cache_inside_repo`);
   extend the pure planner/A23 with ancestor-of-credential-cache rejection; pin the group apply
   to physical (no-symlink-follow) recursion. **No new schema.**
4. **Daemon-owned `lane_uid_leases` table** (OQ2): the **four-state** machine, the partial
   **held-unique** index, the generation token, **persisted** (restart-survival).
5. **Allocation + host-global admission ceiling** (OQ1): lease a free uid at
   `supervise.start`; refuse `lane_uid_pool_exhausted` when none is free.
6. **Return + scrub + PROOF + reaper** (OQ2): the allocate/scrub-begin/scrub-finalize
   boundary; S1–S3 scrub + P1–P5 postcondition proof on `session.close`, **with P1 rejecting
   every non-zombie `pool_uid`-owned task** (the C1-RESIDUAL fix, wired to
   `classifyPoolUIDTaskState`); the sweep reaps leaked-active and re-drives stuck-scrubbing;
   quarantine-on-failed-proof with a doctor surface and operator retry.
7. **Attestation binds uid + generation** (OQ5); **per-uid credential hydration** (OQ6),
   scrub deletes on return (proven by P3).

With (1)–(7), RFC 0143 Slice B reduces to *"write a session-scoped reseal token owned by the
leased uid, `0600`"* — safe by HC-A2.

**Build-run target (per the SEED).** The new table lands in the **next FREE runtime-migration
slot** — runtime migrations currently end at `0044_deploy_cursor.sql`, and **`0045` is
reserved by the concurrent RFC 0170 P0 work**, so `lane_uid_leases` takes **`0046+` (do NOT
hardcode `0045`; the build picks the next free slot at implementation time)** — plus an
**owner-bundle bump** for the daemon-owned `striatumd.lane_uid_leases` table (owner migrations
currently end at `owner/0022_operator_identity_run_attribution.sql`, so the bump is
`owner/0023+`, granting `striatumd` ownership/grants — the same additive mechanism as the
prior owner-bundle versions). **The provider-ancestry fix (step 3) is Go-only — no migration.**

**Seams deferred to later slices (named, not dropped):** daemon-managed dynamic uid
create/destroy; automated pool autogrow/resizing; the reseal-token write itself (RFC 0143
Slice B); cross-host/multi-host pooling; non-tmux adapter parity for the per-uid kill domain;
non-claude adapter bearer-path parity (the agy/gemini settings writer already supervisor-scopes
its markers, `mcpconfig.go:266`; codex carries the bearer in `STRIATUM_MCP_TOKEN` env, not a
scratch file). The credential-cache resolution-domain ban (OQ4.1.1) covers **every** provider
the resolver resolves a config-dir env for (claude `CLAUDE_CONFIG_DIR`, codex `CODEX_HOME`,
`resolver.go:24-32`) — extensible by adding the provider's config-dir env key to the resolver
roster.

**Local-first boundary preserved.** One host, one PostgreSQL (the single writer, D094 / RFC
0043), one daemon; every scrub/kill/probe is local `sudo -n -u <pool_uid>`; the pool is OS
users; no hosted service, cloud API, telemetry, or external persistence. The provider-ancestry
fix is a **narrowing** — it forbids a provider credential cache from living inside the shared
repo and forbids the group grant from escaping its planned in-repo source targets; no new
authority is granted.

---

# Source re-verification (every load-bearing site CONFIRMED against current run-branch worktree HEAD `621312c4`; the v4 bearer anchor + the v5 provider-auth chain pinned to the LIVE code)

| Claim | Site | Status |
| --- | --- | --- |
| Run-as launch = `sudo -n -u <runAsUser> -- env -i …`; bare tmux; minimal env; deterministic session name | `pty.go:98-112,:120-155,:310-314,:620-633`; `tmux_liveness.go:125-149` | **CONFIRMED** (carried Part 1) |
| `processZombie` classifies `/proc` state as BINARY `Z`-or-not; returns `false` on read error (the C1-RESIDUAL trap) | `go/pkg/supervisor/tmux_liveness.go:599-614` | **CONFIRMED (LIVE)** — carried; P0 adds the NEW `classifyPoolUIDTaskState` 3-way classifier |
| **The live `0600` MCP bearer is `writeEphemeralMCPConfig` under `<repoRoot>/.striatum/scratch` ROOT, no `STRIATUM_SUPERVISOR_ID`** (the v4 bearer-path discharge target) | `go/pkg/agentloop/mcpconfig.go:550-580` (`dir:=Join(repoRoot,".striatum","scratch")` `:555`; `os.TempDir()` fallback `:556-558`; `CreateTemp` `:559`; chmod `0600` `:565`) | **CONFIRMED (LIVE)** — carried from v4; re-rooted by OQ4.0 |
| `mcpconfig.go:241/266` are the gemini `settings.json` write + backup/created markers, NOT the bearer | `:241` `os.WriteFile(...,0o600)` (`"write gemini settings"`); `:266` `Join(repoRoot,".striatum","scratch",supervisorID)` | **CONFIRMED (LIVE)** — the v4 citation correction holds |
| Scratch ACL: `.striatum`:`u:<lane>:--x`; `.striatum/scratch`:`u:<lane>:rwx`+default — granted because the bearer `CreateTemp`s in scratch root (#279) | `scratch_acl.go:31-49` (targets `:42-49`); `prepareScratchACLsForLaneUser:58-79` | **CONFIRMED (LIVE)** — carried; P0 re-keys to leased uid + pushes `--x` down |
| **`command_env CLAUDE_CONFIG_DIR` is ADMITTED — `validateLaneCommandEnvKey` rejects only empty / `PATH` / `STRIATUM_*`** | `go/pkg/mutations/supervision_lane_config.go:440-451` | **CONFIRMED (LIVE)** — the v5 chain start; OQ4.1.1 adds the resolution-domain refuse |
| **`command_env` merged into the live lane env via `applyLaneLaunchEnv`→`mergeEnvReplacing`** | `go/pkg/mutations/supervision_env.go:110-118` | **CONFIRMED (LIVE)** — OQ4.1.1 guard runs after this |
| **`CLAUDE_CONFIG_DIR` SURVIVES the run-as filter — `sensitiveRunAsEnvKey` drops only `STRIATUM_MCP_TOKEN`/`STRIATUM_MCP_TOKEN_FILE`/`DATABASE_URL`/`PGPASSWORD` + substrings `TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN`; `CLAUDE_CONFIG_DIR` matches none** | `go/pkg/mutations/supervision_env.go:303-318` | **CONFIRMED (LIVE)** — the v5 leak vector |
| **The Claude branch of `ResolveCredential` takes `CLAUDE_CONFIG_DIR` AUTHORITATIVELY (before HOME) and resolves `.credentials.json` under it** | `go/pkg/laneproviderauth/resolver.go:78-85` (`dir:=values[EnvClaudeConfigDir]` `:79`; `Join(Clean(dir),".credentials.json")` `:82`); `EnvClaudeConfigDir`/`EnvCodexHome` `:24-32` | **CONFIRMED (LIVE)** — the cache the ancestry invariant keeps out of the domain |
| **The group source ACL is provisioned REPO-GLOBALLY at repo.init/repo.add — `provisionCommitteeACLs` applies `setRepoACL` (= `setfacl -R`) over `repoRoot` AND `.striatum/worktrees`; "repo.init must not fail registration"** (so a planner-only guard cannot see a per-lane `command_env` cache — the timing argument for OQ4.1.1) | `go/pkg/admin/repo_acl.go:97-140` (`setRepoACL` `:21-31`; recursion `:130-138`; comment `:112-114`) | **CONFIRMED (LIVE)** — replaced by the allowlist/exclude-at-traversal + physical-apply + ancestry-aware planner |
| Live teardown closes the session when it holds no active lease — NO per-uid kill/cred/HOME scrub today | `supervision_control.go:557-637` | **CONFIRMED** (carried) — P0 hooks `tx_scrub_begin` + the scrub here |
| Recovery sweep is the reaper host (default 60s); #198 moves probes OUT of the sweep tx | `recovery_auto.go:12,:22-38`; `main.go:869` | **CONFIRMED (LIVE)** (carried) |
| Boot epoch fresh-per-process, NOT persisted; `daemonInstanceID` restart-stable ⟹ derive free set from the DB | `main.go:782`, `:728` | **CONFIRMED (LIVE)** (carried) |
| `MCPBootEpoch` folded into capability material + rejected on mismatch (the model the OQ5 generation token reuses) | `mutations.go:41-48` | **CONFIRMED** (carried OQ5) |
| Pre-launch per-supervisor scratch dir create + #279 scratch ACL prep | `supervision_control.go:112-115,:121,:125` | **CONFIRMED (LIVE)** (carried) |
| `RewriteEphemeralMCPConfig` (#323 rotation) is path-relative (`dir:=filepath.Dir(path)`) — follows the bearer | `mcpconfig.go:518-548`; `loop.go:670` | **CONFIRMED (LIVE)** (carried) |
| Per-uid HOME + per-uid Claude credential store; RFC 0165 hydrator contract (resolves OUTSIDE `<repoRoot>` ⟹ passes OQ4.1.1) | `supervision_env.go:205-226`; `laneproviderauth/resolver.go:86-92`; RFC 0165 | **CONFIRMED** (carried OQ6) |
| No host-global concurrent-lane ceiling today (`max_active_jobs` per-workflow, default unlimited) | `runreconcile_test.go:395` | **CONFIRMED** (carried OQ1) |
| Runtime migrations end at `0044`; `0045` reserved by RFC 0170 P0; owner migrations end at `0022` | `go/pkg/db/sql/0044_deploy_cursor.sql`; `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql` | **CONFIRMED (LIVE)** — `lane_uid_leases` → runtime `0046+`, owner bump `owner/0023+` (not hardcoded) |
| D261 ratifies the per-lane-uid direction; rejects namespace-inode/AppArmor-hat/private-socket-alone | `docs/decisions/decision-log.md` (D261); RFC 0168 | **CONFIRMED** |

---

# Falsifiable-assertion index (the claims the v5 falsifiers re-attack)

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
| **A21** | **(C1-RESIDUAL — DISCHARGED v3, CARRIED)** a `pool_uid`-owned task left **stopped (`T`)** or **tracing-stopped (`t`)** **blocks** `returned`: P1 (wired to `classifyPoolUIDTaskState`, NOT `processZombie`) classifies it `Live`, finalizes `quarantined` with `lane_uid_scrub_failed`, the uid is **not** allocated, the quarantine survives a boot-epoch rotation, and only the same P1–P5 proof clears it; the blocking PID + `/proc` state is recorded. **Refuter:** a `T`/`t` (or unknown/unreadable-still-present) survivor passes P1 and the uid reaches `returned`. | `TestStoppedOrTracedUIDProcessBlocksReturn` |
| **A22** | **(C2 bearer-path — DISCHARGED v4, CARRIED)** the pool ACL provisioner **never transiently** exposes `.striatum/`, AND the bearer lives in the lane's own per-supervisor subdir: A22 **derives** the bearer path from the **live writer**, asserts it resolves under `.striatum/scratch/S1/`, asserts **no residual root-level `.striatum/scratch/lane-mcp-config-*` bearer**, runs the unleased-`U_x` + different-leased-`U_2` before/during/after `open(2)` loop, AND S1's **own** leased uid `U1` `CreateTemp`s successfully. **Refuter:** any cross-uid read succeeds; OR a group entry transiently appears under `.striatum/`; OR a bearer is created/rewritten directly under `.striatum/scratch` root; OR S1's own launch `CreateTemp` fails `EACCES`. | `TestPoolACLProvisioningNeverTransientlyExposesScratch` |
| **A23** | **(C2 procedure — KEPT + EXTENDED with ancestry; necessary, not sufficient)** the pure ACL planner's output is **rejected** if any `g:striatum-lanes` op targets `<repoRoot>`/an ancestor of `.striatum/` as a raw recursive `-R` root while `.striatum/` exists, targets `.striatum/`/`.git/`/`.gemini`/`.claude`/`.codex`, **is equal-to / a descendant-of / an ANCESTOR-of any statically-configured credential-cache path**, or sets a `d:g:striatum-lanes` default on a dir with `.striatum/` **or a configured credential cache** as a descendant. **Refuter:** the planner emits — and the guard passes — a raw `setfacl -R …:rX <repoRoot>` plan, a plan touching `.gemini`/`.claude`/`.codex`, or a plan group-granting an ancestor/descendant/equal of a configured credential cache. | `TestACLPlannerRejectsRawRecursiveRootWhileScratchExists` (extended with the ancestor-of-cache cases) |
| **A24** | **(C2 bearer-writer realness — DISCHARGED v4, CARRIED)** the live `writeEphemeralMCPConfig` resolves to `<repoRoot>/.striatum/scratch/<STRIATUM_SUPERVISOR_ID>/`, falls back to `os.TempDir()` when the supervisor id is empty/uncreatable, and **never** targets the `.striatum/scratch` **root**; the focused agentloop tests assert the supervisor-subdir shape. **Refuter:** the writer (or its updated focused test) places/accepts a bearer directly under `.striatum/scratch` root, or ignores `STRIATUM_SUPERVISOR_ID`. | `TestEphemeralMCPConfigResolvesUnderSupervisorScratchSubdir` |
| **A25** | **(provider/credential-cache ANCESTRY — DISCHARGED v5, the verdict-driving fix)** no provider credential cache is ever reachable by a `g:striatum-lanes` ACL: **(1)** a lane launched with `command_env CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` is **REFUSED** fail-closed at launch (typed `lane_credential_cache_inside_repo`) via the live env chain (`validateLaneCommandEnvKey`→`applyLaneLaunchEnv`→`sensitiveRunAsEnvKey`→`ResolveCredential`), so no ACL is ever applied under `docs/` for it; **(2)** a cache resolving OUTSIDE `<repoRoot>` (per-uid HOME) launches fine; **(3)** the pure planner **REJECTS** any `g:striatum-lanes` op equal-to/descendant-of/**ancestor-of** a statically-configured cache (incl. `docs/`); **(4)** a symlink `docs/cred -> <outside>/cache` is **never followed** by the physical (`-RP`) apply — no ACL on the target or its `.credentials.json` before/during/after; **(5)** a credential created AFTER provisioning inherits **no** default `g:striatum-lanes` ACL. **Refuter:** any of — the inside-repo launch proceeds; a legitimate outside-repo cache is refused; the planner passes a group grant on `docs/`; the symlink target gets an ACL; a late-created credential inherits a default `g:` ACL. | `TestProviderCredentialCacheNeverInGroupACLDomain` |

**Negative control:** the BC1-W1-ORACLE replay itself (A1). **C1 controls:** A17/A18/A19/A20
**+ the C1-RESIDUAL control A21**. **C2 controls:** A16 (end-state non-exposure) **+ the
C2 bearer-path controls A22/A24** (live bearer writer supervisor-scoped, real resolved-path
transition + launch-positive control) **+ the C2 procedure control A23** (planner can never
emit a control-plane- or credential-cache-touching op) **+ the provider/credential-cache
ANCESTRY control A25** (no group ACL ever reaches a credential cache — per-lane, static,
nested, symlinked, or late-created). **Restart controls:** A8′/A11′/A19. A clearing verdict
requires the hard core proven (A1–A5, carried), the lease/scrub/reaper complete with the
tightened postcondition proof (A8′–A11′, A17–A21, carried-DISCHARGED), the ACL **exact across
the full transition over the real bearer AND ancestry-enforced for credential caches** (A13,
A16, A22, A23, A24, A25), and no standing falsifier challenge.

---
<sub>Holder leading proposal — RFC 0168 P0 `falsification_gate` design run, **REVISION v5**.
Discharges the SINGLE standing v4 residual — the **provider/credential-cache ANCESTRY**
sub-part of `OQ4-ACL-PROVISIONING-TRANSITION` — by making the forbidden set
**ANCESTRY-ENFORCED** via ONE coherent invariant (*no provider credential cache is ever
reachable by a `g:striatum-lanes` ACL*) enforced at **three coordinated chokepoints**: **(a)**
a NEW launch-time **resolution-domain ban** (OQ4.1.1) refusing any lane whose configured
provider credential cache resolves inside `<repoRoot>` (typed `lane_credential_cache_inside_repo`)
— load-bearing because the per-lane `command_env CLAUDE_CONFIG_DIR` is known only at launch
while the group source ACL is provisioned repo-globally (`repo_acl.go:97-140`); **(b)** the
pure ACL planner/**A23** extended to reject any `g:striatum-lanes` op equal-to/descendant-of/
**ancestor-of** any statically-configured credential-cache path (the same ancestor semantics
A23 already gives `.striatum/`); **(c)** a **physical (no-symlink-follow) recursive apply**
(`setfacl -RP`, OQ4.1(c)) so the grant never escapes its planned in-repo source targets. Adds
**A25** `TestProviderCredentialCacheNeverInGroupACLDomain` exercising the live env chain over
the ledger's exact `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` case plus symlink and
created-after probes, and updates the OQ4 invariant + OQ6 carry-forward sentence to ancestry
enforcement. Carries the v1-proven hard core (HC-A1..A5), the **v3-DISCHARGED C1-RESIDUAL**
(fail-closed three-way P1 → `classifyPoolUIDTaskState`, NOT binary `processZombie`
`tmux_liveness.go:599-614`; `/proc` evidence; A21), the **v4-DISCHARGED C2 bearer-path**
(re-rooted `writeEphemeralMCPConfig` `mcpconfig.go:550-559` under `.striatum/scratch/<supervisor_id>/`;
real A22; A24), the C1 durable four-state lease machine, the C2 procedure fix + the A23
raw-recursive-root prohibition + the `.striatum/`/`.git/`-excluding GROUP-ACL end-state
invariant, and OQ1/OQ3/OQ5/OQ6 + the narrowing invariant **unregressed**. Build-run target:
runtime migration `0046+` (`0045` reserved by RFC 0170 P0; not hardcoded) + owner-bundle bump
`owner/0023+` for `striatumd.lane_uid_leases`; the provider-ancestry fix is Go-only (no
schema). Local-first boundary intact: one host, one PostgreSQL, one daemon as single writer;
no hosted services. The fix is a narrowing — a provider credential cache may no longer live
inside the shared repo and the group grant may no longer escape its planned in-repo source
targets; no new authority. Re-verified against run-branch worktree HEAD `621312c4`. This is
the published claim the v5 falsifiers re-attack.</sub>
