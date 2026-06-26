# Design-Run Seed — RFC 0168 P0 (REVISION v3)

> This is the **v3 revision** of the RFC 0168 P0 design run. v1 proved the
> structural hard core (a per-lane uid dissolves `BC1-W1-ORACLE`) and resolved
> OQ1/OQ3/OQ5/OQ6; **v2 credited the structural halves of both binding
> constraints** (the C1 durable lease state machine, the C2 final `.striatum/`-
> excluding ACL end-state) but the v2 cycle-1 adjudicator returned
> **`needs_revision`** on **two narrow, source-anchored residual holes**
> (`OQ2-SCRUB-POSTCONDITION`, `OQ4-ACL-PROVISIONING-TRANSITION`) and, its single
> revision cycle exhausted, routed them to the operator. This is exactly the
> `-v3` that the v2 ledger prescribes. This run **discharges the two residual
> holes** while carrying forward, unregressed, everything v1 AND v2 cleared.
> This is a **tightening of two predicates, not a rewrite**.
>
> **Required context docs** (read in full first):
> - `docs/operator/workflows/rfc-0168-design-v3/context/v2_HOLDER.md` — the **v2
>   SPEC you are revising** (the base; do NOT rewrite from scratch; only the two
>   residual predicates change).
> - `docs/operator/workflows/rfc-0168-design-v3/context/v2_COLLABORATION_LEDGER_cycle_1.md`
>   — the v2 verdict; its `OQ2-SCRUB-POSTCONDITION` and
>   `OQ4-ACL-PROVISIONING-TRANSITION` findings are the exact prescribed fixes.
> - `docs/operator/workflows/rfc-0168-design-v3/context/v2_FALSIFIER_1.md` /
>   `…/context/v2_FALSIFIER_2.md` — the prior attacks that landed the two holes.
> - `docs/rfcs/0168-per-lane-security-principal.md` — the RFC (direction ratified D261).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. It must **resolve C1-RESIDUAL and C2-RESIDUAL** and
**carry forward, unregressed, everything v1 AND v2 cleared**. A revision that
leaves either residual hole open — or regresses a carry-forward — has NOT cleared
the gate.

## Carried forward — CLEARED in v1 + CREDITED in v2 (do NOT reopen, do NOT regress)

- **The structural hard core HC-A1..A5 — PROVEN.** A per-lane uid dissolves
  `BC1-W1-ORACLE` on this host under Yama `ptrace_scope=1`: distinct run-as uids
  land on tmux's per-uid `0700` socket (HC-A1); a `0600`/`0700` lane-owned file
  is cross-uid-unreadable (HC-A2 — the surface that makes RFC 0143 Slice B's
  lane-uid-owned reseal token safe); a different uid fails `ptrace_may_access`
  for `ptrace`/`setns`/`/proc` (HC-A3); the `SO_PEERCRED` uid discriminator
  (HC-A4) and residual-surface closure (HC-A5) hold.
- **C1 structural half (CREDITED in v2).** The durable 4-state lease machine
  (`active`/`scrubbing`/`quarantined`/`returned`), the partial held-unique index
  `uq_lane_uid_held`, the allocate / scrub-begin / scrub-finalize 3-transaction
  boundary, the leaked-active and stuck-scrubbing reaper, the doctor surface, the
  quarantine-survives-restart property, and dirty-excluding exhaustion accounting.
- **C2 structural half (CREDITED in v2).** The final `.striatum/`-excluding ACL
  end-state, the per-leased-uid traverse-only entry, the per-supervisor scratch
  leg, the chowned worktree, per-lease ACLs removed on scrub, and the A16
  after-state test.
- **OQ1 / OQ3 / OQ5 / OQ6** and the **NARROWING invariant** (no admin-token
  widening, no lane-readable shared reseal bearer; surface is only ever removed).

## The two residual constraints to DISCHARGE

### C1-RESIDUAL — the scrub-postcondition proof must reject a non-zombie survivor (`OQ2-SCRUB-POSTCONDITION`, verdict-driving)

The v2 proof predicate P1 blocks `returned` only on a `pool_uid`-owned process in
`R`/`S`/`D`, and records zombies without blocking. But Linux `/proc/<pid>/status`
also reports non-zombie **`T`** (stopped) and **`t`** (tracing-stop): such a task
has **not** exited — it still holds the uid, memory, FDs, and HOME/credential
reachability, and is `SIGCONT`-resumable. A survivor in `T`/`t` passes P1, the
proof finalizes `returned`, and the dirty uid is re-leased = the exact same-uid
cross-lease residue leak RFC 0168 exists to close. This is buildable-critical:
the existing `processZombie` helper (`go/pkg/supervisor/tmux_liveness.go:599-614`)
classifies `/proc` state as **binary `Z`-or-not**, so an implementer wiring P1
off it inherits the leak.

The revised SPEC must:
- Tighten P1 to **zero `pool_uid`-owned tasks except true zombies / dead tasks
  that cannot execute and hold no resources** — any observed non-zombie state
  (`T`, `t`, or unknown) MUST block `returned` and finalize `quarantined` with a
  typed `lane_uid_scrub_failed`.
- Wire the proof to a state classifier that is **not** the binary `Z`-only
  `processZombie`.
- Record observed PIDs + `/proc` states in `scrub_proof` so `doctor`
  distinguishes tolerated `Z` residue from a non-zombie quarantine cause.
- Add `TestStoppedOrTracedUIDProcessBlocksReturn` (inject a stopped/traced
  uid-owned survivor; assert quarantine + non-allocation + restart-preserved
  quarantine + proof-gated clear).

### C2-RESIDUAL — ACL provisioning must never transiently group-expose `.striatum/` (`OQ4-ACL-PROVISIONING-TRANSITION`, verdict-driving)

The v2 SPEC gates only on the auditable **end-state** and blesses
`setfacl -R -m g:striatum-lanes:rX <repoRoot>` followed by a strip over
`.striatum/`. But the first `-R` necessarily adds group `r` to existing
`.striatum/` `0600` control-plane files (the MCP bearer, `mcpconfig.go:241/266`;
PTY logs, `loop.go`) **before** the strip. On a live repo `.striatum/` is
**transiently group-readable**, and a bearer/PTY-log read in that window is
irreversible cross-lane exfiltration replayable as another lane. The A16
after-state test misses it entirely.

The revised SPEC must:
- Make the **allowlist / exclude-at-traversal** provisioning form **MANDATORY**:
  the group grant NEVER applies `g:striatum-lanes:rX` (or its default) to
  `.striatum/`, `.git/`, or provider/token-cache paths, even transiently.
- Explicitly **FORBID** `setfacl -R …:rX <repoRoot>`-then-strip as a live-repo
  path.
- Extend A16 into a transition test
  `TestPoolACLProvisioningNeverTransientlyExposesScratch` (an unleased AND a
  different-leased pool uid attempt `open(2)`/traversal **before / during /
  after** provisioning; no read ever succeeds).
- Add an **ACL-planner guard / unit test** that fails if any `g:striatum-lanes`
  op targets `<repoRoot>` as a raw recursive root while `.striatum/` exists.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (scrub-postcondition predicate lens):** Does P1 now reject every
  non-zombie `pool_uid`-owned task (`T`/`t`/unknown → `quarantined`), wired off a
  non-binary classifier? Construct the stopped/traced/reparented/re-forking
  survivor case. Is the test real? Did the tightening regress the C1 structural
  machine or the hard core / OQs?
- **Falsifier 2 (ACL provisioning-transition lens):** Is the allowlist form
  mandatory and raw-recursive-root forbidden, so `.striatum/` is never transiently
  group-readable? Construct the read-during-provisioning case and probe the
  allowlist for enumeration gaps. Are the transition test + planner guard real?
  Did the procedure change regress the C2 final end-state or the hard core / OQs?

The adjudicator gates on whether **C1-RESIDUAL and C2-RESIDUAL are each genuinely
discharged** (mechanisms anchored to real source; named tests + guards specified)
and whether any carry-forward regressed or any new material challenge lands. A
clearing verdict (`accept` / `accept_with_findings`) requires both discharged
with their tests and no standing regression. This is the single allowed v3
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator. On a clearing verdict the operator ratifies **D272** (D271 is
reserved by the concurrent RFC 0170 P0 design). The build run targets the next
FREE runtime-migration slot — **0045 is reserved by RFC 0170 P0
`cullable_entity`; use 0046+** — plus an owner-bundle bump for the daemon-owned
`striatumd.lane_uid_leases` table. Keep the local-first boundary (one host, one
PostgreSQL, one daemon as single writer; no hosted services).
