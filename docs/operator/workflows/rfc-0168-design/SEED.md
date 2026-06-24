# Design-Run Seed — RFC 0168 P0 (FRESH v1)

> This is the **fresh v1** `falsification_gate` design run for RFC 0168
> (per-lane OS user as the lane security principal). The **direction** is
> already maintainer-ratified (**D261**, 2026-06-24): a pre-provisioned pool of
> per-lane OS uids, leased per lane, is the structural fix for the shared-uid
> `BC1-W1-ORACLE` wall that made RFC 0143's authenticated reseal channel
> unsolvable across seven cycles. This run hardens the **spec** — the six
> design-gate open questions — into falsifiable, build-bearing constraints.
> **Required context docs** (read in full first):
> - `docs/rfcs/0168-per-lane-security-principal.md` — the proposed RFC (the direction + blast radius + the six open questions).
> - `docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the `BC1-W1-ORACLE` finding that forced this RFC (the same-uid replay surface that cannot be authenticated).
> - `docs/decisions/decision-log.md` row **D261** — the split decision (0143 Slice A ships / Slice B blocked on this RFC) and the `/adhd` rejected alternatives (namespace-inode, AppArmor-hat, private-socket-alone).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. The direction is settled; do **not** re-litigate
"per-lane uid vs alternatives" (D261 closed that). The SPEC must turn the
ratified direction into **build-bearing constraints**: it answers each of the
six open questions with a concrete, source-anchored, falsifiable decision +
the test that would refute it, and defines the **P0 slice** precisely — the
minimum provisioning + lease + attestation that unblocks **RFC 0143 Slice B**
(a lane-uid-owned `0600` session-scoped reseal token). A SPEC that leaves the
structural claim unproven, the scrub/lease lifecycle incomplete, or
restart-survival unestablished has NOT cleared the gate.

## The settled direction (do NOT reopen — D261)

- **Per-lane OS uid, pre-provisioned pool, leased per lane** is the principal.
  It is the only structural, host-independent fix that survives a daemon
  restart and dissolves the whole same-uid class (`BC1-W1-ORACLE`,
  `BC1-W1-CAPTURE`, the rejected `0600` reseal file, same-uid tmux replay).
- **Rejected (closed):** namespace-inode binding (not structural under Yama
  `ptrace_scope=1`); AppArmor-hat + magic token (host-dependent); private tmux
  socket alone (insufficient under a shared uid).
- The change is a **narrowing** — it grants no new authority; it redraws the
  principal boundary from one shared `striatum-lane` uid to a per-lane uid.

## The hard core — PROVE the structural claim

The whole RFC leans on one assertion: **a per-lane uid actually dissolves the
same-uid replay class on this host.** The SPEC must prove, source-anchored,
that a sibling lane (a *different* uid) cannot `respawn-pane -k`, `open`,
`setns`, `ptrace`, or read the target lane's tmux pane / control socket /
`0600` file — under Yama `ptrace_scope=1`, with the current run-as launch path
(`commandInvocationWithEnvFile` wrapping `sudo -n -u <runAsUser> -- env -i …`,
`go/pkg/supervisor/pty.go`; `RunAsTmuxRunner` via the same path,
`tmux_liveness.go:125-133`). Name any residual same-uid surface (shared parent
process, shared tmux server, world/group-readable path, the daemon bridging
uids) and close it, or the gate does not clear.

## The six open questions to DISCHARGE (each → a build-bearing constraint + test)

1. **OQ1 — Pool size + exhaustion.** Concrete sizing relative to
   `max_active_jobs` / concurrent-lane ceilings **across all runs** (not one);
   exhaustion policy (block / grow / refuse new lanes) and the safe default.
2. **OQ2 — Lease/allocation lifecycle.** Daemon-leased per session (like the
   existing session lease); the exact return + **scrub** steps (home dir,
   credential store, the uid-owned tmux server, stray/daemonized processes via
   the per-uid kill domain); the leaked / never-returned uid **reaper**;
   **restart-survival** — how lease↔uid bindings are reconstructed after a
   daemon boot-epoch rotation (the exact case RFC 0143 targets), given no
   in-memory binding survives.
3. **OQ3 — Provisioning ownership.** Host-setup runbook artifact (like today's
   single lane user) **vs** daemon-managed create/destroy. Pick one and justify
   against the daemon privilege boundary (daemon-managed = the daemon holds
   uid-management authority — a larger blast radius / possible over-grant).
4. **OQ4 — ACL interaction.** How per-target-repo ACLs (#537/#539,
   `docs/how-to/lane-sandbox.md` `setfacl`) cover **every** pool uid: a
   `DEFAULT` ACL on a lane group vs per-uid grants — without over-granting a uid
   not currently leased to that repo.
5. **OQ5 — Attestation.** How `lane_attestation` / `lane_attestation_reason`
   change when the principal is a pooled uid not fixed `striatum-lane`; whether
   a **recycled** uid needs a generation/epoch token to prevent cross-lease
   confusion (a stale attestation authenticating a new lease).
6. **OQ6 — Credential store.** Each pool uid's own provider-credential store
   (`~/.claude/.credentials.json` etc.); how the RFC 0165 spawn-time hydrator
   (#583) populates per-uid stores **without** N stale copies or cross-uid leak.

## P0 slice boundary

Define P0 as the minimum that unblocks RFC 0143 Slice B: the provisioning +
lease + scrub + attestation needed for a lane to run as its own pooled uid and
own a `0600` reseal token safely. Name the seams deferred to later slices.
Keep the local-first boundary (one host, one PostgreSQL, one daemon as single
writer; no hosted services / external persistence).

## Falsifier guidance (attack the v1 proposal)

- **Falsifier 1 (provisioning / lease-lifecycle / exhaustion / restart-survival
  lens):** Does the lease lifecycle **scrub** a returned uid completely (home,
  `~/.claude` creds, the uid-owned tmux server, stray processes) — or can lane
  N's residue leak to the next lease of the same uid? Is the leaked-uid reaper
  real for the daemon-died-mid-lease case? Does the design **survive a daemon
  restart** (D261's load-bearing property) given the pool/lease state must be
  reconstructed? Is exhaustion safe under concurrent-lane pressure (no deadlock,
  no silent refusal, no unbounded growth)? Does pool size correctly relate to
  `max_active_jobs` across **all** concurrent runs? Is the provisioning-ownership
  choice coherent with the daemon privilege boundary (no uid-admin over-grant)?
- **Falsifier 2 (structural-security / attestation / credential-store / ACL
  lens):** Is the core claim GENUINELY true — does a per-lane uid actually
  dissolve `BC1-W1-ORACLE` on this host (Yama `ptrace_scope=1`), with **no**
  residual same-uid surface? Does a recycled uid create cross-lease confusion
  (need a generation/epoch)? Can a stale attestation from a prior lease
  authenticate a new one? Does per-uid credential hydration (#583) race or leak
  creds across uids? Do per-uid/DEFAULT-group ACLs grant exactly-enough without
  over-grant (a confused deputy)? Confirm the change is a **narrowing**, not a
  widening.

The adjudicator gates on whether each open question is **genuinely discharged**
(mechanisms anchored to real source; named tests + the `BC1-W1-ORACLE` negative
control + the restart-survival test specified) and the structural claim proven,
not merely asserted. A clearing verdict (`accept` / `accept_with_findings`)
requires the structural claim proven, the lease/scrub/reaper complete,
restart-survival established, and no standing falsifier challenge. This is the
single allowed v1 revision cycle; a second `needs_revision` ends the gate
uncleared and routes to the operator (who spins a fresh `-v2` run with a
revising holder).
