# HOLDER — RFC 0168 P0 falsifiable implementation spec (fresh v1: the per-lane OS uid that dissolves BC1-W1-ORACLE)

author: holder-author-001

> This is the **fresh v1** `falsification_gate` proposal for **RFC 0168 P0** — a
> pre-provisioned pool of per-lane OS uids, leased per lane, as the lane security
> principal. The **direction is ratified (D261, 2026-06-24)** and is **not
> relitigated here**: per-lane uid vs namespace-inode / AppArmor-hat /
> private-socket-alone is closed (all three rejected). This spec hardens the
> ratified direction into **build-bearing constraints** — it (1) **proves the hard
> core**, that a per-lane uid actually dissolves the `BC1-W1-ORACLE` same-uid
> replay class on *this* host (Yama `ptrace_scope=1`), and (2) **discharges the six
> design-gate open questions**, each as a FALSIFIABLE assertion + the test/check
> that would refute it, and (3) **defines the P0 slice** that unblocks RFC 0143
> Slice B (a lane-uid-owned `0600` reseal token). Every source citation was
> re-verified against the current worktree HEAD while authoring this; drift is
> flagged inline in §Source re-verification. This is the published claim the two
> falsifiers re-attack.

## Root frame (the wall this dissolves — held, not reopened)

RFC 0143's design gate converged across seven cycles on one irreducible
obstruction, **`BC1-W1-ORACLE`** (the v7 adjudicator ledger,
`docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`):

> the production tmux control surface runs as the **shared** `striatum-lane` uid,
> with a deterministic session name and **no** private `tmux -S` socket /
> `TMUX_TMPDIR` isolation, so a same-uid sibling can `respawn-pane -k` the target
> pane between the daemon's launch and its capture, and the daemon — whose only
> handle on "the wrapper" is a **post-launch tmux query against a same-uid-mutable
> oracle** — authenticates the *replacement*. The identical same-uid model
> rejected a `0600` reseal file (a sibling-readable replay surface).

The gate's own conclusion: *same-uid no-replay is unsolvable while lanes share one
uid.* RFC 0168's move is structural — **stop sharing the uid.** This spec must
prove that move actually closes `BC1-W1-ORACLE` (and the whole same-uid class:
`BC1-W1-CAPTURE`'s residue, the rejected `0600` file, same-uid tmux replay), then
make the provisioning/lease/scrub/attestation/credential machinery that lets a lane
*run as its own pooled uid* build-bearing and falsifiable.

---

# Part 1 — THE HARD CORE: a per-lane uid dissolves BC1-W1-ORACLE on this host

The whole RFC leans on one assertion. I prove it as four structural sub-claims (the
kernel/OS mechanisms that enforce uid isolation against a *different-uid* peer), then
name and close every residual *same-uid* surface. Each sub-claim is anchored to the
real run-as launch path and carries the test that would refute it.

## The exact BC1-W1-ORACLE attack, replayed under a per-lane uid

Setup (today, shared uid): target lane and sibling lane both run as the single
`striatum-lane` uid. The launch path is `sudo -n -u <runAsUser> -- env -i …`
(`commandInvocationWithEnvFile`, `pty.go:98-112`) and tmux is invoked **bare**
through that same run-as path (`tmuxRunnerForSpec` → `RunAsTmuxRunner`,
`pty.go:310-314`; `runAsTmuxRunner.Run` builds `LaunchSpec{RunAsUser, Env}` and runs
`commandContext(..., "tmux", args...)`, `tmux_liveness.go:125-149`) with a
**deterministic** session name (`tmuxSessionName`, `pty.go:620-633`) and **no** `-S`
socket / `TMUX_TMPDIR` override anywhere in `go/pkg/supervisor`. So the sibling
addresses the **same per-uid tmux server**, issues
`tmux respawn-pane -k -t <pane> -- <attacker>`, and the daemon's post-launch
`CaptureTmuxIdentity` (`pty.go:493`) observes the replacement. That is `BC1-W1-ORACLE`.

Under a **per-lane uid** the target runs as uid `U_t`, the sibling as uid `U_s ≠ U_t`.
The launch path is **unchanged** — still `sudo -n -u <U_t> -- env -i … tmux …` — but
the *runAsUser is now different per lane*. The attack requires the sibling's `tmux`
client to reach `U_t`'s tmux server. It cannot, for the four structural reasons below.

## HC-A1 — tmux's control surface is per-uid; a different-uid sibling cannot address it

**Claim.** Each lane's bare `tmux` (run as its own uid) uses tmux's **default per-uid
socket** at `$TMUX_TMPDIR/tmux-<uid>/default` (`TMUX_TMPDIR` unset ⟹ `/tmp`), and
tmux creates `/tmp/tmux-<uid>` **mode `0700` owned by that uid** and the socket
`srwx------`. A process running as `U_s` has **no traverse (`--x`) permission** on
`/tmp/tmux-<U_t>`, so `connect(2)` to `U_t`'s socket fails `EACCES` before any tmux
command is parsed. The same-uid-mutable oracle is gone: there is no per-uid tmux
server `U_s` can address, hence no `respawn-pane` it can issue against `U_t`'s pane.

**Why this is the load-bearing source fact.** Our code runs tmux **bare** (no `-S`,
no `TMUX_TMPDIR` in the run-as env — `sanitizedRunAsEnv`/`nonSensitiveEnv` strip the
env to a minimal allowlist, `pty.go:120-155`), so each distinct run-as uid
**necessarily** lands on tmux's default per-uid socket. The per-uid `0700` socket dir
is a standard tmux/OS property (tmux `server.c` creates `tmux-<uid>` with `mkdir(…,
0700)` and refuses a dir it does not own), but because P0 *depends* on it, it is
**verified, not assumed** (the negative test below).

**Falsifiable — A1.** Real-path negative (host integration test, gated on `sudo` +
`tmux`, build-tagged like `tmux_liveness_integration_test.go`):
`TestSiblingPoolUIDCannotRespawnTargetPane`. Launch a target lane as pool uid `U_t`
through `RunHelper` with `RequireTmux`; from a second process running as a *different*
pool uid `U_s`, attempt `tmux respawn-pane -k -t <U_t pane> -- /bin/true` and assert
it **fails** (`error connecting … permission denied` / `no server running`), the
target pane's `#{pane_pid}` is **unchanged**, and `CaptureTmuxIdentity` still reports
the daemon-launched wrapper. **Refuter:** the sibling's respawn succeeds, or the
target pane pid changes ⟹ the per-uid tmux boundary is not real on this host and the
hard core fails (P0 must then add an explicit private `tmux -S <0700-socket-owned-by-U_t>`).

## HC-A2 — a `0600`/`0700` lane-uid-owned file is unreadable by a different-uid sibling (the reseal token)

**Claim.** A file owned by `U_t`, mode `0600`, in a `0700` dir owned by `U_t`, is
unreadable by `U_s` under ordinary POSIX DAC — `U_s` is not the owner, not in a
granting group, and has no traverse on the `0700` parent. This is exactly the surface
RFC 0143 option 2 needed and could not have under a shared uid; per-lane uid makes it
safe. (This is the *direct* unblock of RFC 0143 Slice B.)

**Falsifiable — A2.** `TestSiblingPoolUIDCannotReadLaneOwnedResealToken`: write a
`0600` file owned by `U_t` under `U_t`'s `0700` HOME; assert `open(2)` as `U_s`
returns `EACCES`. **Refuter:** any read path for `U_s` (owner mismatch ignored, a
group/`other` bit set, a world-traversable parent, or a default ACL granting `U_s`) ⟹
the token is still a cross-uid replay surface (see OQ4 for the ACL discipline that
keeps this true).

## HC-A3 — cross-uid `ptrace`/`setns`/`/proc` secret reads are denied (this is why namespace-inode-alone was rejected, and why uid is the fix)

**Claim.** The rejected alternatives fail precisely where per-lane uid succeeds, and
the difference is the credential check inside `ptrace_may_access`:

- **`ptrace`** of `U_t` by `U_s`: denied. The ordinary DAC ptrace credential check
  requires matching uid (or `CAP_SYS_PTRACE`); `U_s ≠ U_t` fails it **independently
  of** Yama. Yama `ptrace_scope=1` *additionally* restricts even same-uid non-ancestors —
  but the different-uid case is already closed by the credential check.
- **`setns` / `open("/proc/<U_t-pid>/ns/*")`**: entering another process's namespaces
  calls `ptrace_may_access(…, MODE_ATTACH_REALCREDS)`; a different uid without
  `CAP_SYS_PTRACE` is denied. This is **the** reason namespace-inode binding was
  rejected (D261): under a *shared* uid a sibling passes `ptrace_may_access` and can
  `setns` in; under a *different* uid it cannot. **The uid is doing the work the
  namespace inode could not.**
- **`/proc/<U_t-pid>/{environ,mem,fd,maps}`** (the W2 secrecy surface): gated by
  `ptrace_may_access`; `U_s` is denied. The control-channel nonce / any secret in
  `U_t`'s environ stays unreadable by `U_s`.

**Falsifiable — A3.** `TestSiblingPoolUIDCannotPtraceOrSetnsOrReadProcSecrets`
(host integration, `ptrace_scope=1` asserted as a precondition): as `U_s`, assert
`ptrace(PTRACE_ATTACH, <U_t-pid>)` ⟹ `EPERM`, `open("/proc/<U_t-pid>/ns/net")` +
`setns` ⟹ `EACCES/EPERM`, and `open("/proc/<U_t-pid>/environ")` ⟹ `EACCES`.
**Refuter:** any of these succeeds for `U_s` ⟹ the kernel does not enforce uid
isolation as claimed on this host (a misconfiguration the test surfaces loudly).

## HC-A4 — the daemon-owned control socket admits only the launched wrapper's uid+pid (SO_PEERCRED)

**Claim.** RFC 0143 Slice B's authenticated channel (carried, not redesigned here) is
a daemon-held `SO_PASSCRED` `SOCK_SEQPACKET` listener; the daemon reads `SO_PEERCRED`
on the accepted connection — the connecting peer's **kernel-attested** `{pid, uid}`.
With a per-lane uid the accept predicate gains a *uid* discriminator that is now
**meaningful**: `peer.uid == U_t` (the lease's bound uid) AND `peer.pid == launched
pane pid`. A sibling `U_s` is rejected at the uid check **before** any pid/start-token
reasoning is needed — the BC1-W1 capture-oracle problem does not even arise, because
the daemon no longer has to *infer* identity from a same-uid-mutable tmux query; the
kernel attests the connecting uid and it is structurally `U_s ≠ U_t`.

**Falsifiable — A4.** `TestControlFrameAcceptsOnlyLeasedLaneUID`: a frame from a peer
whose `SO_PEERCRED.uid ≠ U_t` (the leased uid) is **refused** even with the correct
pid; only the `U_t`-owned wrapper is accepted. **Refuter:** a `U_s` peer is accepted ⟹
the uid is not a real discriminator (e.g. the listener is world-connectable and the
predicate ignores `peer.uid`).

## HC-A5 — every residual SAME-uid surface, named and closed

A per-lane uid only helps if **no** same-uid bridge remains. The four candidates the
SEED names, each closed:

1. **Shared parent process.** The launch chain is daemon (operator uid) → `sudo`
   (transiently root, then `setuid(U_t)`) → `env -i` → tmux/agentloop. The **only**
   common ancestor of two lanes is the **daemon**, which is the trusted single writer
   (D094 / RFC 0043), *not* a lane. There is **no lane-to-lane** parent: `U_t`'s pane
   is a child of `U_t`'s tmux server, `U_s`'s of `U_s`'s. Closed — provided the daemon
   never copies one uid's secret to a path another uid can read (see OQ6).
2. **Shared tmux server.** *Was* the BC1-W1-ORACLE surface; **now per-uid** (HC-A1).
   Closed.
3. **World/group-readable path.** Closed by `0600`/`0700` on every *private* surface
   (the reseal token, the per-uid HOME, the per-uid credential store). The pool's
   shared **repo-traversal** ACL (OQ4) deliberately covers only the shared work
   product, **never** a lane's private token/HOME — those carry no group/other read
   bit and no default ACL for the lane group. The discipline is: *group ACL for
   shared repo read; per-uid `0600`/`0700` for private secrets; the two never cross.*
4. **The daemon bridging uids.** The daemon runs as the operator and *can* read
   everything (it must, to attest, porter artifacts, and scrub). That is a
   lane-vs-daemon relationship, not lane-to-lane, and the daemon is trusted by
   construction. The one rule it must honor — **never materialize `U_t`'s secret where
   `U_s` can read it** — is enforced by per-uid hydration into `U_t`'s `0700` HOME
   (OQ6) and the scrub-on-return that deletes it (OQ2).

**Falsifiable — A5.** `TestNoSharedSameUIDSurfaceBetweenPoolLanes`: launch two lanes
on distinct pool uids; assert (a) no shared tmux server (HC-A1), (b) their HOMEs and
credential stores are distinct `0700`/`0600` and mutually unreadable (HC-A2/OQ6), (c)
their only common ancestor pid is the daemon, and (d) neither lane's process tree
contains a process owned by the other's uid. **Refuter:** any shared same-uid object
between the two lanes (a shared tmux server, a group-readable secret, a co-owned
process) ⟹ a residual replay surface survives.

**Hard-core conclusion.** `BC1-W1-ORACLE`'s mechanism — *a sibling mutates the
same-uid tmux oracle the daemon queries* — requires the sibling to address `U_t`'s
tmux server (HC-A1: denied), or to `ptrace`/`setns` into `U_t` (HC-A3: denied), or to
read a `U_t`-owned file (HC-A2: denied). All three are structurally closed by the
different uid, and no residual same-uid bridge remains (HC-A5). The daemon no longer
needs to *prove a token belongs to the born wrapper via a mutable oracle*; the kernel
**attests** the connecting uid (HC-A4), and it is `U_t`. This is a **narrowing** — no
new authority is granted; the principal boundary moves from one shared uid to a
per-lane uid.

---

# Part 2 — THE SIX OPEN QUESTIONS, each discharged as a build-bearing constraint + refuting test

## OQ1 — Pool size + exhaustion

**Decision.** Today there is **no host-global concurrent-lane ceiling**: `max_active_jobs`
is a per-workflow field, default `0` = unlimited
(`go/pkg/runreconcile/runreconcile_test.go:395` comment: *"max_active_jobs:0 means
unlimited — every distinct-lane queued job launches"*), and the daemon enforces no
global cap. A finite uid pool therefore **introduces the first host-global
concurrent-lane ceiling** — that is a deliberate, named consequence, not an accident.

- **Sizing.** The pool size `N` is the **host's** maximum concurrent live lanes
  **across all runs**, not one run's `max_active_jobs`. A lane holds a uid for its
  **session** lifetime (OQ2), spanning every job it claims, so the binding count is
  *live sessions with an attached supervisor*, not queued jobs. Default
  `N = ` (operator-chosen, documented) the host's intended concurrency; the runbook
  recommends a conservative starting `N` (e.g. 8) and how to compute it: `N ≥
  Σ_runs(expected concurrent distinct-lane supervisors)`.
- **Exhaustion policy = REFUSE, fail-closed, typed; never share, never auto-grow.**
  When `supervise.start` (or `run drive`'s launch leg) needs a uid and none is free,
  the daemon **refuses the launch with a typed `lane_uid_pool_exhausted` floor** and
  leaves the job **queued** (recoverable; the existing supervise/drive retry relaunches
  when a uid frees). It does **NOT** silently fall back to the shared `striatum-lane`
  uid (that would re-open the whole class), does **NOT** block-and-wait holding a lock
  (deadlock risk), and does **NOT** auto-`useradd` (OQ3 forbids daemon uid authority).
  Refuse-and-requeue is the safe default because lanes are independent (no lane waits
  on another lane's uid), so a bounded refuse cannot deadlock and a freed uid always
  re-enables the queued launch.

**Falsifiable — A6 (sizing) + A7 (exhaustion).**
`TestLaneUIDPoolCeilingIsHostGlobalAcrossRuns`: with pool `N`, start `N` lanes across
**two different runs**; assert exactly `N` distinct uids are leased and no uid is
double-leased (the active-lease uniqueness, mirroring
`uq_active_resource_lease`, `0005_…sql:184-186`). `TestLaneUIDPoolExhaustionRefusesTyped`:
request lane `N+1`; assert it (a) is **refused with `lane_uid_pool_exhausted`** (a
typed floor, not a raw error), (b) the job stays **queued/recoverable**, (c) **no uid
is shared** and the launch did **not** fall back to `striatum-lane`, and (d) after one
lease returns, the queued launch **succeeds** on the freed uid. **Refuter:** an `N+1`th
lane runs (shared uid or silent fallback), or the refusal deadlocks/strands the run, or
the pool grows past `N`.

## OQ2 — Lease/allocation lifecycle (return + scrub + reaper + restart-survival)

**Decision — bind the uid to the SESSION, persist the binding in PostgreSQL.** The
existing `leases` table is a **job** lease (`resource_type CHECK IN ('job')`,
`owner_session_id` FK to `sessions`, `0005_…sql:166-182`); a uid must instead bind to
the **session/supervisor** (one live lane process, one tmux pane, one uid) spanning
**all** jobs that session claims via its receive loop. P0 adds a daemon-owned table
**`striatumd.lane_uid_leases`** (a new owner-bundle version, the same mechanism BC4/BC5
used to add `jobs.recovery_generation` / `leases.reseal_grace_extended_at`):
`{ repository_id, pool_uid, session_id, supervisor_id, generation bigint, state
active|returned, leased_at, returned_at, scrub_status }`, with a **unique active index
on `pool_uid`** (a uid is leased to at most one live session). Because it lives in
daemon-owned PostgreSQL, it **survives a `striatumd` restart** (D094 / RFC 0043) — the
load-bearing property D261 demands — exactly as `sessions`/`leases` do.

- **Allocation.** At `supervise.start`, before the token mint + launch, the daemon
  picks a free pool uid (no active `lane_uid_leases` row), inserts the lease row with a
  monotonically **incremented `generation`** for that uid, and launches as it
  (`RunAsUser = pool_uid`). The generation is the anti-recycling token (OQ5).
- **Return + scrub (the exact steps).** On `session.close`
  (`lifecycle.go:483-627` → `stopSupervisorsForTerminalSession`,
  `mutations.go:1596-1687`), after the existing supervisor stop
  (`stopTmuxBackedLane` issues `tmux kill-session`; pipe-backed lanes
  `terminateProcessWithStartToken`), P0 adds a **scrub** of the returned uid, in order:
  1. **Per-uid process kill domain** — `sudo -n -u <pool_uid> -- kill -KILL -1`
     (signal every process owned by `pool_uid`), reaping the uid-owned tmux **server**
     and any stray/daemonized processes the lane spawned. This is the per-uid kill
     domain the SEED names; it is only safe **because** the uid is private to this
     lease (a shared uid could not be blanket-killed).
  2. **Credential store** — delete `~<pool_uid>/.claude/.credentials.json` (and the
     resolved `CLAUDE_CONFIG_DIR` store), so no provider credential persists into the
     next lease (OQ6).
  3. **Home scratch** — remove the lane's writable HOME contents (provider caches/config
     the CLI wrote). Today **no home scrub exists** (the agent-confirmed teardown
     removes only `.gemini/settings.json` / `.claude/scheduled_tasks.lock` and the
     stdin pipe); P0 adds the full per-uid HOME scrub.
  4. **Mark the lease `returned`** with `scrub_status` only after 1–3 succeed; a
     **failed scrub leaves the uid `quarantined`** (not returned to the free set) and
     records a typed `lane_uid_scrub_failed` for the operator — never silently
     re-leased dirty.
- **Leaked / never-returned uid REAPER.** The daemon-died-mid-lease case (no
  `session.close` ran) is handled by the **recovery sweep** (`HandleRecoveryAuto`,
  `recovery.go:553`, every 60s, `main.go:81`). The sweep already expires stale leases
  (`expireLeases`, `recovery.go:2516`) and reaps idle orphan sessions
  (`reapIdleOrphanSessions`, `recovery.go:856`). P0 extends it: when a session bound to
  a `lane_uid_lease` is reaped/closed/expired and its lane is no longer live (the same
  `buildRunLivenessOracle` tmux/`/proc`/`kill(0)` probe the sweep already builds),
  **scrub-then-return** the uid (the same 4 steps), or **quarantine** on scrub failure.
  No uid leaks: every code path that ends a session also ends its uid lease.
- **RESTART-SURVIVAL (the exact RFC 0143 case).** The boot epoch is **fresh per
  process** and deliberately **not persisted** (`main.go:459-468`, `randomBootEpoch`);
  `daemonInstanceID` **is** restart-stable (`main.go:665-677`). On restart **no
  in-memory binding survives** — but the `lane_uid_leases` rows do (PostgreSQL). The
  daemon reconstructs the pool state by **reading the table**: for each active uid
  lease it re-probes its session's lane liveness (the sweep's existing oracle); a
  **still-live** lane keeps its uid (its session/supervisor/pane identity survive in
  the DB, so attestation re-binds — OQ5), a **dead** lane's uid is scrubbed+returned by
  the first post-restart sweep. The free set is therefore *derived* (uids with no
  active lease row), never held in memory, so a restart cannot double-lease a uid or
  strand one.

**Falsifiable — A8 (binding/persist) + A9 (scrub) + A10 (reaper) + A11 (restart).**
- **A8** `TestUIDLeaseBindsSessionAndPersistsAcrossRestart`: a uid leased to a session
  is recorded in `lane_uid_leases`; simulate a daemon restart (fresh boot epoch, reload
  from DB) and assert the live lane keeps the **same** uid (binding reconstructed from
  PostgreSQL, not memory). **Refuter:** the binding is lost or a second uid is leased to
  the same live session after restart.
- **A9** `TestUIDReturnScrubsProcessesCredsAndHome`: return a uid whose lease left a
  stray daemonized process, a credential file, and HOME scratch; assert the per-uid kill
  domain killed the stray + tmux server, the credential store and HOME scratch are gone,
  and the lease is `returned` only after a clean scrub. **Refuter:** any lane-N residue
  (process, cred, or file) is observable to lane N+1 on the same uid, or the uid returns
  to the free set with `scrub_status` unset.
- **A10** `TestLeakedUIDReapedAfterDaemonDeath`: kill the daemon mid-lease (no
  `session.close`); on the next sweep with the lane dead, assert the uid is
  scrubbed+returned (or quarantined on scrub failure), never left leaked-active.
  **Refuter:** a uid stays `active` with no live session and is never reclaimed.
- **A11** `TestUIDLeaseReconstructedAfterBootEpochRotation`: the exact RFC 0143 case —
  rotate the boot epoch (restart) while a lane is mid-work; assert the uid↔session
  binding is rebuilt from the DB and the lane can still own its `0600` reseal token as
  the same uid. **Refuter:** the binding cannot be reconstructed without an in-memory
  value (i.e. restart-survival fails — the property D261 requires).

## OQ3 — Provisioning ownership

**Decision — host-setup runbook artifact (static pool); the daemon gets NO uid-lifecycle
authority.** The pool is pre-created by the **operator runbook** (an extension of
`docs/how-to/lane-sandbox.md`), exactly as the single lane user is today. The daemon
**leases** from a static set; it never `useradd`/`userdel`s.

**Justification against the daemon privilege boundary.** Daemon-managed create/destroy
would require the daemon to hold **uid-management authority** (`CAP_SETUID`-class power,
or NOPASSWD `useradd`/`usermod`/`visudo`) — a **much larger blast radius**: a
compromised daemon could mint arbitrary system users. Today the daemon holds only
*launch-as* authority: `striatumd ALL=(striatum-lane) NOPASSWD: ALL`
(`lane-sandbox.md:93`) — the power to *run a command as* the lane user, **not** to
create users. P0 **preserves that boundary**: the runbook widens the sudoers grant from
one user to the pool (a runas **group**: `striatumd ALL=(%striatum-lanes) NOPASSWD:
ALL`), still **launch-only**. The daemon's new authority is exactly the *scrub*
(`sudo -n -u <pool_uid> -- kill/rm` within the launch-as grant it already has) — no new
class of privilege. This keeps RFC 0168 a **narrowing**, consistent with D261's framing
and the lane-sandbox non-goal *"not new daemon authority"* (`lane-sandbox.md:415-417`).

**Falsifiable — A12.** `TestDaemonHoldsNoUIDLifecycleAuthority`: assert (a) the pool
sudoers grant is runas-only (`(%striatum-lanes) NOPASSWD: ALL`) with **no**
`useradd`/`usermod`/`visudo` entry for the daemon user, and (b) the daemon code path
contains **no** `useradd`/`userdel` call — pool members are read from config, never
created. `doctor` reports `lane_uid_pool: { provisioned: true, size: N,
daemon_creates_uids: false }`. **Refuter:** any daemon code (or sudoers grant) that
creates/destroys OS users ⟹ the blast radius widened past launch-as.

## OQ4 — ACL interaction (every pool uid covered, without over-grant)

**Decision — DEFAULT group ACL for SHARED repo traversal/read; per-uid `0600`/`0700`
for PRIVATE secrets; per-job-worktree write owned by the leasing uid.** Today the
runbook grants the single lane user repo ACLs: `setfacl -R -m u:<lane-user>:rX <repo>`
+ `--x` on ancestors, plus per-job-worktree **write** under `.striatum/worktrees/<id>`
(`lane-sandbox.md:254-268`). For a pool, replicating *per-uid* `setfacl` grants on the
whole tree is N× churn and drifts; instead:

- **Shared read/traverse** → a **DEFAULT ACL on a lane group** `striatum-lanes`
  (every pool uid is a member): `setfacl -R -m g:striatum-lanes:rX <repo>` and
  `setfacl -R -d -m g:striatum-lanes:rX <repo>` so new files inherit it; `--x` on
  ancestors via the group. One grant covers **every** pool uid and every future file.
- **Per-job-worktree write** → the worktree is **`chown`ed to the leasing uid** at
  creation (the daemon already owns the worktree lifecycle and now knows the lease's
  uid), so only the leasing uid writes it — not a group-write that any pool uid could
  abuse.
- **Private surfaces** (HOME, credential store, reseal token) carry **no** group ACL
  and **no** default ACL for `striatum-lanes` — they are per-uid `0700`/`0600`,
  closing HC-A2/HC-A5(3).

**On the over-grant worry ("a uid not leased to that repo").** A pool uid that is a
group member can **read/traverse** the repo even when not currently leased to a lane
in that repo. This is **not** a meaningful over-grant: the repo tree is shared work
product already readable by the lane *principal class* today (the single shared uid
reads it); the security boundary that matters is (i) the daemon's PG socket and (ii)
each lane's **private** secrets — **neither** is group-granted. **Write** is the
confined capability, and it is confined to the **leasing** uid's own `chown`ed
worktree, so a non-leased pool uid cannot mutate anything: no confused deputy. The PG
isolation (`pg_hba` reject) must cover **every** pool uid — handled by a group reject
rule (`local all %striatum-lanes reject`, the pool analogue of `lane-sandbox.md:75-80`).

**Falsifiable — A13.** `TestPoolACLGrantsSharedReadNotPrivateOrCrossWrite`: assert (a)
every pool uid can traverse+read the repo via the `striatum-lanes` group ACL; (b) a
pool uid **not** leased to a job's worktree **cannot write** it (worktree owned by the
leasing uid); (c) no pool uid can read another lane's `0600` reseal token / HOME
(HC-A2); and (d) `make lane-isolation-check` (extended to the group reject rule) shows
**every** pool uid denied PostgreSQL over **both** the UNIX socket and loopback TCP.
**Refuter:** a non-leased uid writes a worktree, or any pool uid reads another's private
secret, or any pool uid reaches PostgreSQL.

## OQ5 — Attestation (pooled uid + recycle-confusion token)

**Decision — record the leased uid + the uid-lease GENERATION in attestation; refuse a
stale-generation attestation.** Today attestation records `supervisor_id`, `pid`, and
tmux liveness; `RunAsUser` rides in tmux metadata (`tmux_meta.go` `RunAsUser` field)
and selects the probe runner (`lanehealth.go` `runnerForMeta`) but does **not** drive
the attested/unattested decision (`lanehealth.Classify`). P0 changes two things:

- **Bind the principal.** The attestation packet records the **leased pool uid** (from
  the `lane_uid_leases` row), so attestation now answers *"is this lane the one we
  leased uid U_t to?"* — not just *"is some pane alive?"*. The existing PID start-token
  (`ProcessStartToken`, `/proc` field 22, via `PIDLiveWithStartToken`,
  `tmux_liveness.go:392-408`) already discriminates the **process**; the leased uid
  discriminates the **principal**.
- **The recycled-uid generation token (the new anti-confusion primitive).** A uid `U`
  returned and re-leased to a new session must not let a **stale** attestation/process
  from lease `g-1` authenticate lease `g`. P0 stamps the `lane_uid_leases.generation`
  (monotonic per uid) into the attestation and into the lane's capability material (the
  same way `MCPBootEpoch` is folded in, `mutations.go:41-48`); the daemon **refuses to
  attest** (or to accept a control frame) when the presented generation ≠ the live
  generation for that uid. This is the structural defense against cross-lease confusion;
  the scrub-on-return (OQ2) closes the *surface* (kills lease `g-1`'s processes), the
  generation token closes the *window* (a process that somehow survives the scrub
  carries the wrong generation and is refused).

**Falsifiable — A14.** `TestRecycledUIDGenerationPreventsCrossLeaseConfusion`: lease
uid `U` to session `S1` (generation `g`), return it, re-lease to `S2` (generation
`g+1`); assert (a) attestation for `S2` records `{uid: U, generation: g+1}`; (b) a
process/attestation carrying generation `g` (a stale survivor of `S1`) is **refused**
while `g+1` is live; (c) a control frame authenticating with generation `g` is rejected.
**Refuter:** a stale-generation actor on uid `U` authenticates the new lease ⟹ recycling
creates cross-lease confusion.

## OQ6 — Credential store (per-uid hydration, no stale copies, no cross-uid leak)

**Decision — the RFC 0165 spawn-time hydrator targets the LEASED uid's HOME; scrub
deletes it on return.** Each pool uid has its **own** HOME (per OS user record;
`laneUserIdentityEnv` sets `HOME/USER/LOGNAME` from `user.Lookup`,
`supervision_env.go:205-226`) and therefore its **own**
`~/.claude/.credentials.json` (resolved via `CLAUDE_CONFIG_DIR` or
`$HOME/.claude/.credentials.json`, `laneproviderauth/resolver.go:78-92`). The RFC 0165
hydrator (#583, `docs/rfcs/0165-claude-provider-credential-freshness.md`) already
hydrates **at spawn time** into `STRIATUM_LANE_OS_USER`'s HOME via temp-file+rename,
verifying destination **owner == lane user**, mode **`0600`**, parseable OAuth, expiry
lead time, and **source generation stability** (`provider_credential_source_unstable`).
P0 makes one change: the hydrator's destination is the **leased uid** (not a fixed
`striatum-lane`).

- **No N stale copies.** Hydration is **per-spawn** (fresh from the operator source each
  launch) and the **scrub-on-return deletes** the per-uid store (OQ2 step 2). So a uid's
  credential store exists only for the lease's lifetime: fresh in, deleted out. There is
  never a persistent fleet of N stale copies — at most one live per leased uid, each
  re-hydrated from the single operator source at spawn.
- **No cross-uid leak.** Each store is `0600` owned by its leased uid inside that uid's
  `0700` HOME; the hydrator's existing **owner == lane user** check (now == leased uid)
  refuses to write a store the destination uid does not own, and HC-A2 makes it
  unreadable by any other pool uid. The hydrate is the **only** place the operator
  secret enters a pool HOME, and it enters exactly one (`U_t`'s).

**Falsifiable — A15.** `TestPerUIDCredentialHydrateNoStaleNoLeak`: lease `U_t`, hydrate;
assert (a) `~U_t/.claude/.credentials.json` is `0600` owned by `U_t`; (b) `U_s` cannot
read it (HC-A2); (c) on lease return the store is **deleted** (OQ2 scrub); (d) a later
lease of `U_t` starts with **no** pre-existing store and re-hydrates fresh. **Refuter:**
a credential store survives lease return, or is readable by another pool uid, or a stale
copy authenticates a later lease.

---

# Part 3 — THE P0 SLICE (minimum that unblocks RFC 0143 Slice B) + deferred seams

**P0 is exactly the minimum for a lane to run as its own pooled uid and safely own a
`0600` reseal token:**

1. **Static pool, host-provisioned** (OQ3): N pool uids, `striatum-lanes` group,
   widened runas-group sudoers, per-uid PG reject, group repo ACL — all in the runbook;
   daemon holds no uid-lifecycle authority.
2. **Daemon-owned `lane_uid_leases` table** (OQ2), a new owner-bundle version: bind uid
   ↔ session ↔ generation, unique-active-per-uid, **persisted** (restart-survival).
3. **Allocation + host-global admission ceiling** (OQ1): lease a free uid at
   `supervise.start`; **refuse `lane_uid_pool_exhausted`** (typed, queued, no shared-uid
   fallback) when none is free.
4. **Return + scrub + reaper** (OQ2): per-uid kill domain + credential + HOME scrub on
   `session.close`; the recovery sweep reaps leaked uids; quarantine-on-scrub-failure.
5. **Attestation binds uid + generation** (OQ5): the recycle-confusion token.
6. **Per-uid credential hydration** (OQ6): RFC 0165 hydrator targets the leased uid;
   scrub deletes on return.

With (1)–(6) in place, RFC 0143 Slice B reduces to *"write a session-scoped reseal
token owned by the leased uid, `0600`"* — safe by HC-A2 — with **no** further
same-uid-safe-channel machinery (the W1/W2/W3 walls collapse because HC-A4's uid
discriminator replaces the same-uid-mutable oracle reasoning).

**Seams deferred to later slices (named, not silently dropped):**

- **Daemon-managed dynamic uid create/destroy** — explicitly out of P0 (OQ3 blast
  radius); a later slice may add it behind a separate ratification.
- **Automated pool autogrow / dynamic resizing** — P0 is a fixed `N`; growth is an
  operator runbook action.
- **The reseal-token write itself** — that is RFC 0143 Slice B (this RFC only makes the
  uid-owned `0600` surface *safe*; it writes no credential code, per RFC 0168 §Out of
  scope).
- **Cross-host / multi-host pooling** — out of scope; local-first, one host, one pool.
- **Non-tmux adapter parity for the per-uid kill domain** — P0 covers the tmux and
  pipe-backed lanes the supervisor launches today; a future adapter inherits the same
  `lane_uid_leases` + scrub contract.

**Local-first boundary preserved.** No hosted service, no cloud API, no telemetry, no
external persistence: the pool is OS users on one host, the lease lives in the
**daemon-owned PostgreSQL** (the single writer, D094 / RFC 0043), and every scrub/kill
is local `sudo -n -u <pool_uid>` within the launch-as grant the daemon already holds.
The change is a **narrowing** of the lane principal, granting no new authority
(RFC 0168 Blast radius §security_or_authz).

---

# Source re-verification (every load-bearing site CONFIRMED against current worktree HEAD; drift FLAGGED)

| Claim | Site | Status |
| --- | --- | --- |
| Run-as launch = `sudo -n -u <runAsUser> -- env -i …` (env-file shim and sanitized-env forms) | `pty.go:98-112` (`commandInvocationWithEnvFile`) | **CONFIRMED** verbatim (`:104-106` env-file, `:108-112` sanitized) |
| tmux invoked **bare** through the run-as path (no `-S`/`TMUX_TMPDIR`) | `pty.go:310-314` (`tmuxRunnerForSpec` → `RunAsTmuxRunner`); `tmux_liveness.go:125-149` (`runAsTmuxRunner.Run` builds `LaunchSpec{RunAsUser, Env}`, runs `commandContext(..., "tmux", args...)`) | **CONFIRMED** — only `-s`/session-name args; no socket override |
| run-as env stripped to a minimal non-sensitive allowlist (so each uid lands on tmux's default per-uid socket) | `pty.go:120-155` (`sanitizedRunAsEnv`/`nonSensitiveEnv`/`sensitiveEnvKey`) | **CONFIRMED** — drops `*TOKEN*`/`*SECRET*`/`*DSN*`/etc.; PATH defaulted |
| Deterministic tmux session name (no private socket) | `pty.go:620-633` (`tmuxSessionName`: `striatum-<runID>-<laneID>-<supervisorID>` + sha256 suffix) | **CONFIRMED** |
| `leases` is a **job** lease (uid must bind to the session instead) | `0005_repo_local_workflow_state.sql:166-182` (`resource_type CHECK IN ('job')`, `owner_session_id` FK→`sessions`); unique active index `:184-186` | **CONFIRMED** |
| Recovery sweep loop (the reaper host) every 60s | `recovery.go:553` (`HandleRecoveryAuto`); `main.go:81` (`-sweep-interval-seconds` default `60.0`) | **CONFIRMED** |
| Sweep already expires leases + reaps idle orphan sessions (P0 extends these to return+scrub uids) | `recovery.go:2516` (`expireLeases`), `recovery.go:856` (`reapIdleOrphanSessions`) | **CONFIRMED** |
| Boot epoch fresh per process, NOT persisted; `daemonInstanceID` IS restart-stable (⟹ restore the binding from PostgreSQL, not memory) | `main.go:459-468`, `randomBootEpoch`; `main.go:665-677` (`daemonInstanceID`) | **CONFIRMED** |
| Session close → supervisor stop (today: `tmux kill-session` / `terminateProcessWithStartToken`; **no** home/cred scrub beyond `.gemini`/`.claude` lock files) | `lifecycle.go:483-627` (`session.close`); `mutations.go:1596-1687` (`stopSupervisorsForTerminalSession`) | **CONFIRMED** — P0 adds the per-uid kill domain + cred + HOME scrub here |
| Attestation records supervisor_id/pid/liveness; `RunAsUser` selects the probe runner but does NOT drive the attested decision | `lanehealth.go` (`Classify`, `Health`, `runnerForMeta`); `sessionLaneAttestation`, `mutations.go:1689-1704`; PID identity `tmux_liveness.go:392-408` | **CONFIRMED** — P0 adds the leased-uid + generation binding |
| `MCPBootEpoch` folded into lane capability material + rejected on mismatch (the model the generation token reuses) | `mutations.go:41-48` | **CONFIRMED** |
| Per-uid HOME from the OS user record; per-uid Claude credential store resolution | `supervision_env.go:205-226` (`laneUserIdentityEnv`/`laneOSUserHome`); `laneproviderauth/resolver.go:78-92` | **CONFIRMED** |
| RFC 0165 hydrator: spawn-time, temp-file+rename, owner==lane-user, `0600`, source-generation-stable (P0 re-targets it to the leased uid) | `docs/rfcs/0165-claude-provider-credential-freshness.md` (hydrator contract + launch-gate placement) | **CONFIRMED** |
| Lane sandbox runbook: single lane user, `pg_hba` reject, `striatumd ALL=(striatum-lane) NOPASSWD: ALL`, per-repo `setfacl`, per-job-worktree write, HOME/provider-auth; non-goal "not new daemon authority" | `docs/how-to/lane-sandbox.md:49-118,254-268,415-417` | **CONFIRMED** — P0 widens to the pool/group analogues |
| No host-global concurrent-lane ceiling today (`max_active_jobs` per-workflow, default unlimited) | `go/pkg/runreconcile/runreconcile_test.go:395` comment | **CONFIRMED** — P0's pool is the first such ceiling |
| D261 split: RFC 0143 Slice A ships / Slice B blocked on RFC 0168; rejected namespace-inode / AppArmor-hat / private-socket-alone; blocker #585 | `docs/decisions/decision-log.md` (D261) | **CONFIRMED** |

---

# Falsifiable-assertion index (the claims the falsifiers re-attack)

| ID | Claim | Refuting test/check |
| --- | --- | --- |
| **A1** | tmux's control surface is per-uid (`/tmp/tmux-<uid>` `0700`); `U_s` cannot `respawn-pane` `U_t`'s pane | `TestSiblingPoolUIDCannotRespawnTargetPane` (real-path) |
| **A2** | a `U_t`-owned `0600`/`0700` reseal token is unreadable by `U_s` | `TestSiblingPoolUIDCannotReadLaneOwnedResealToken` |
| **A3** | cross-uid `ptrace`/`setns`/`/proc` secret reads denied (the namespace-inode-vs-uid distinction) | `TestSiblingPoolUIDCannotPtraceOrSetnsOrReadProcSecrets` |
| **A4** | the control socket admits only the leased uid (`SO_PEERCRED.uid == U_t`) | `TestControlFrameAcceptsOnlyLeasedLaneUID` |
| **A5** | no residual same-uid surface between two pool lanes | `TestNoSharedSameUIDSurfaceBetweenPoolLanes` |
| **A6** | pool ceiling is host-global across all runs; no double-lease | `TestLaneUIDPoolCeilingIsHostGlobalAcrossRuns` |
| **A7** | exhaustion refuses typed `lane_uid_pool_exhausted`, queues, never shares/grows; frees on return | `TestLaneUIDPoolExhaustionRefusesTyped` |
| **A8** | uid binds to the session and persists across a restart (reconstructed from PostgreSQL) | `TestUIDLeaseBindsSessionAndPersistsAcrossRestart` |
| **A9** | return scrubs the per-uid process kill domain + credential store + HOME; returns only on clean scrub | `TestUIDReturnScrubsProcessesCredsAndHome` |
| **A10** | leaked-mid-lease uid is reaped (scrub+return or quarantine) by the sweep | `TestLeakedUIDReapedAfterDaemonDeath` |
| **A11** | the binding is reconstructed after a boot-epoch rotation (the RFC 0143 case) | `TestUIDLeaseReconstructedAfterBootEpochRotation` |
| **A12** | the daemon holds no uid-lifecycle authority (launch-as only) | `TestDaemonHoldsNoUIDLifecycleAuthority` |
| **A13** | group ACL grants shared repo read only; private secrets + worktree write stay per-uid; all pool uids PG-denied | `TestPoolACLGrantsSharedReadNotPrivateOrCrossWrite` + extended `make lane-isolation-check` |
| **A14** | a recycled uid's generation token refuses a stale-lease actor | `TestRecycledUIDGenerationPreventsCrossLeaseConfusion` |
| **A15** | per-uid credential hydration leaves no stale copy and no cross-uid leak | `TestPerUIDCredentialHydrateNoStaleNoLeak` |

The **negative control** is the BC1-W1-ORACLE replay itself (A1): a sibling pool uid
attempting the exact `respawn-pane -k` attack that broke RFC 0143 v7 must be **refused
at the kernel boundary**. The **restart-survival** control is A8/A11 (the binding
rebuilt from PostgreSQL after a fresh boot epoch). A clearing verdict requires the hard
core proven (HC-A1…A5 + A1–A5 tests), the lease/scrub/reaper complete (A8–A11),
restart-survival established (A8/A11), and no standing falsifier challenge.

---
<sub>Holder leading proposal for the RFC 0168 P0 `falsification_gate` design run (fresh
v1). The direction (per-lane pooled OS uid) is ratified (D261) and not relitigated;
this spec proves the hard core (a per-lane uid dissolves `BC1-W1-ORACLE` on this host
under Yama `ptrace_scope=1`: HC-A1 per-uid tmux socket, HC-A2 `0600` DAC, HC-A3
cross-uid ptrace/setns denial — the exact axis namespace-inode failed and uid
succeeds, HC-A4 `SO_PEERCRED` uid discriminator, HC-A5 every residual same-uid surface
closed) and discharges the six open questions into build-bearing, source-anchored,
falsifiable constraints (OQ1 host-global pool ceiling + typed fail-closed exhaustion;
OQ2 session-bound uid lease in daemon-owned PostgreSQL with a complete scrub + sweep
reaper + restart-survival reconstruction; OQ3 host-runbook provisioning preserving the
daemon's launch-as-only privilege boundary; OQ4 DEFAULT group ACL for shared repo read
with per-uid private secrets and leasing-uid worktree write; OQ5 attestation bound to
the leased uid + a recycle-confusion generation token; OQ6 RFC 0165 spawn-time
hydration into the leased uid's store, scrubbed on return). The P0 slice is the minimum
provisioning+lease+scrub+attestation+credential that lets a lane own a `0600` reseal
token safely and unblock RFC 0143 Slice B; the deferred seams (daemon-managed uid
lifecycle, autogrow, the reseal write itself, cross-host pooling) are named. Local-first
boundary intact: one host, one PostgreSQL, one daemon as single writer; no hosted
services.</sub>
