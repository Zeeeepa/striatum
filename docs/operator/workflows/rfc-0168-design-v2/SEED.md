# Design-Run Seed — RFC 0168 P0 (REVISION v2)

> This is the **v2 revision** of the RFC 0168 P0 design run. v1 **proved the
> structural hard core** (a per-lane uid dissolves `BC1-W1-ORACLE`) and resolved
> OQ1/OQ3/OQ5/OQ6, but the adjudicator found **two source-anchored gate holes**
> (C1, C2) and, its single revision cycle exhausted, routed them to the operator.
> This run discharges C1 and C2 while carrying forward, unregressed, everything
> v1 cleared. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0168-design/dialogue/holder/HOLDER.md` — the v1 SPEC you are revising (the base; do NOT rewrite from scratch).
> - `docs/operator/artifacts/rfc-0168-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v1 verdict; its C1/C2 detail is the exact prescribed fix.
> - `docs/rfcs/0168-per-lane-security-principal.md` — the RFC (direction ratified D261).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. It must **resolve C1 and C2** and **carry forward,
unregressed, everything v1 cleared**. A revision that leaves C1 or C2 open — or
regresses a carry-forward — has NOT cleared the gate.

## Carried forward — CLEARED in v1 (do NOT reopen, do NOT regress)

- **The structural hard core HC-A1..A5 — PROVEN.** A per-lane uid dissolves
  `BC1-W1-ORACLE` on this host under Yama `ptrace_scope=1`: distinct run-as uids
  land on tmux's per-uid `0700` socket (HC-A1); a different uid fails
  `ptrace_may_access` for `ptrace`/`setns`/`/proc` (HC-A3 — the axis
  namespace-inode failed under D261); a `0600`/`0700` lane-owned file is
  cross-uid-unreadable (HC-A2); the `SO_PEERCRED` uid discriminator (HC-A4) and
  residual-surface closure (HC-A5) hold. Both falsifiers credited these.
- **OQ1** — host-global pool sizing across all runs + a typed fail-closed
  refuse-and-requeue default (sound; the only caveat is exhaustion accounting
  must read the C1 quarantine state — see below).
- **OQ3** — a static host-runbook pool; the daemon holds only the widened
  launch-as `(%striatum-lanes)` grant and NO `useradd`/`userdel` authority
  (privilege boundary preserved).
- **OQ5** — leased-uid + monotonic generation token as the anti-recycle primitive.
- **OQ6** — per-spawn per-uid hydration with scrub-on-return (contingent on C1's
  scrub closing).
- **The NARROWING invariant** — no admin-token widening, no lane-readable shared
  reseal bearer; the change grants no new authority.

## The two binding constraints to DISCHARGE

### C1 — the lease lifecycle must represent and safely handle a returned-but-dirty uid (OQ2, verdict-driving)

v1's durable lease state machine cannot represent a returned-but-dirty uid. The
schema is only `state active|returned`; the prose relies on an unrepresented
`quarantined`; the free predicate is "no active lease row"; and "mark returned
only after scrub steps 1–3 succeed" rests on command exit codes — `kill -KILL
-1` returning `0` does **not** prove an empty per-uid kill domain (a process can
survive, reappear, or recreate HOME files after the command returns). A scrub
failure forces the daemon into a state the table cannot express: mark `returned`
and the free-set rule re-leases the dirty uid (a same-uid cross-lease residue
leak — the exact class RFC 0168 exists to eliminate); leave it `active` and it is
a zombie-active lease for a dead session with no recovery path, no doctor
surface, and false exhaustion accounting.

The revised SPEC must specify:
- A durable `scrubbing` / `quarantined` lease state in the schema.
- An allocation/scrub **transaction boundary** (when a uid moves
  active→scrubbing→returned, atomically and crash-safely).
- A scrub **postcondition proof** — prove the per-uid kill domain is empty and
  HOME / credential-store / per-uid tmux server are scrubbed, NOT merely that the
  scrub command exited `0`.
- The leaked / failed-scrub **reaper** recovery path.
- A **doctor surface** for a quarantined uid.
- **Exhaustion accounting (OQ1)** that excludes quarantined uids from the free
  set, so a dirty uid is never re-leased and the ceiling is honest.

### C2 — the per-uid repo ACL must not over-grant `.striatum/` control-plane state (OQ4, verdict-driving)

v1's recursive DEFAULT group ACL `setfacl -R -m g:striatum-lanes:rX <repo>` over
the repo ROOT reaches `.striatum/` — daemon/operator-private control-plane: the
`0600` MCP bearer at `.striatum/scratch` (`mcpconfig.go:550-571`), PTY logs, the
token cache, and foreign worktrees. `scratch_acl.go:31-48` grants `.striatum`
only `u:<lane>:--x` traverse under an explicit "never broaden read access to
private operator state" invariant; the whole-group `g:striatum-lanes:rX -R`
breaks it — a pool uid not leased to this repo gets group read on another lane's
session-bound bearer, and `-R …:rX` adds group `r` even to existing `0600` files
(so "mode is 0600" is not a rebuttal). That re-introduces exactly the cross-lane
control-plane replay surface RFC 0168 exists to remove.

The revised SPEC must specify:
- An ACL grant that **EXCLUDES `.striatum/`** (preserve the `u:<lane>:--x`
  traverse-only carve-out).
- A grant scoped to the **leased** uid that grants exactly-enough repo-tree
  access **without** over-granting unleased pool uids.
- A load-bearing **non-exposure test**: plant another lane's `0600`
  control-plane file (MCP bearer / PTY log / foreign worktree) and assert an
  unleased pool uid cannot read it.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1 / lease-lifecycle + scrub-postcondition lens):** Is the
  quarantine state real and the scrub postcondition a genuine proof (not exit
  codes)? On scrub failure, is the dirty uid provably excluded from the free set
  (no re-lease), with a reaper + doctor surface + honest exhaustion accounting?
  Construct the survive-the-scrub failing case. Did the fix regress the hard core
  or OQ1/3/5/6?
- **Falsifier 2 (C2 / ACL-exactness lens):** Does the ACL exclude `.striatum/`
  and keep the `u:<lane>:--x` carve-out, granting only the leased uid? Is the
  non-exposure test real? Construct the unleased-uid-reads-another-lane's-bearer
  failing case. Did the fix regress the hard core / runbook carve-out / OQs?

The adjudicator gates on whether C1 and C2 are each **genuinely discharged**
(mechanisms anchored to real source; named tests + controls specified) and
whether any carry-forward regressed or any new material challenge lands. A
clearing verdict (`accept` / `accept_with_findings`) requires both discharged
with their tests and no standing regression. This is the single allowed v2
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator (a fresh `-v3` run with a revising holder). Keep the local-first
boundary (one host, one PostgreSQL, one daemon as single writer; no hosted
services).
