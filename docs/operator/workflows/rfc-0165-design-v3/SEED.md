# Design-Run Seed — RFC 0165 (REVISION v3)

> This is the **v3 revision** of the RFC 0165 design run (Claude provider
> credential freshness + spawn-time hydration; #583). **v1** ran a full
> `falsification_gate` cycle and ended `needs_revision`: both falsifiers landed
> **unrebutted** and the adjudicator issued two binding constraints. **v2**
> attempted the revision but was **quarantined** by a runner defect (its
> official cycle-2 ledger body is missing on disk — `artifact body file does
> not exist`), so v2 produced **no authoritative review trail** and is treated
> as non-evidence. v3 carries forward v1's authoritative findings and
> discharges them. **Required context docs** (read in full first):
> - `docs/rfcs/0165-claude-provider-credential-freshness.md` — the proposed RFC.
> - `docs/operator/artifacts/rfc-0165-design/dialogue/holder/HOLDER.md` — the v1 SPEC (the base; revise it, do not rewrite from scratch).
> - `docs/operator/artifacts/rfc-0165-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v1 verdict; its `findings:` (F1, F2) + `constraints:` are the exact prescribed fixes.
>
> Do **not** rely on any `rfc-0165-design-v2` artifact — that run is quarantined
> (`docs/operator/artifacts/rfc-0165-design-v2/QUARANTINE.md`).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised
falsifiable implementation spec for RFC 0165** the downstream `rfc-0165-build`
`code_change` run executes. It must **discharge F1 and F2** and **carry forward,
unregressed, everything v1 got right**. A revision that leaves F1 or F2 open —
or regresses a carry-forward — has NOT cleared the gate.

The operational stopgap (the host cred-resync timer, #583, already live on the
host) is NOT the structural fix; this RFC is the structural fix. Do not propose
"the host timer handles it" — that is the status quo this RFC replaces.

## Carried forward — what v1 got right (do NOT regress)

v1's holder resolved the launch-time decisions at a useful level of concreteness;
keep these and integrate them with the F1/F2 fixes:
- A **spawn-time freshness gate**: a Claude lane cannot launch with a stale,
  expired, unparseable, or generation-drifted provider credential — on every
  spawn, respawn, and recovery requeue path.
- **Provider OAuth credentials stay separate from Striatum control-plane
  (session-bound capability) tokens** (RFC 0096 / #135 / #296 trust boundary):
  lanes still authenticate to Striatum with their own session token and never
  receive daemon/admin tokens or token-minting authority.
- **`provider_auth_gate=off` cannot bypass hydration/freshness.**
- **Redacted, private-safe custody receipts** (no raw OAuth material in DB rows,
  repo artifacts, metrics, events, or doctor output).

## The two binding findings to DISCHARGE

### F2 — no lane raw-refresh-token custody (CRITICAL)

The v1 JIT file-copy model (`~striatum-lane/.claude/.credentials.json`) gives
each lane **raw refresh-token custody**. With OAuth refresh-token rotation, a
lane-side refresh rotates the token in the lane file while the operator source
file stays stale — **invalidating the operator source** for future lanes and
possibly the operator's own CLI session; concurrent lanes also share stale
copied refresh-token state. The revised SPEC must:
- Move to a **daemon-managed credential broker / access-token model** (or an
  equally concrete, equally explicit OAuth-safe design) where **lanes do not
  receive raw refresh tokens** and **cannot independently rotate the operator
  credential family**. The lane should receive only what it needs to make
  Claude calls (e.g. a short-lived access token, or a brokered read), with the
  refresh authority held in exactly one place.
- Prove it does not corrupt or invalidate the operator source login, and that
  concurrent + subsequent lane launches cannot desynchronize.
- Add **concurrent-lane and subsequent-lane refresh-token-rotation (RTR) tests**
  (the named falsifiable controls).

### F1 — runtime-expiry circuit breaker (HIGH)

Spawn-time hydration can pass, then the credential can **expire mid-session**
before the lane first needs Claude. The stored dependency/receipt still say
ready/passed, so recovery treats the resulting `agent_mcp_discovery_stall` as
**generic** and burns requeue/transfer budget before `reseed_required` is
discovered on a later launch. The revised SPEC must:
- Make **recovery perform a current freshness check** (against the lane
  credential or an equivalent daemon-owned provider-auth state) **before**
  incrementing generic requeue/transfer counters.
- Make the **supervisor/heartbeat path report provider credential expiry or
  near-expiry as provider-auth readiness debt** before ordinary stall recovery
  fires — so stale runtime provider auth becomes typed readiness debt, never
  generic retry budget burn.
- State each as a falsifiable assertion + its test.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (F1 / runtime-expiry + recovery-classification lens):** Does
  recovery now classify runtime credential expiry as provider-auth debt BEFORE
  consuming generic requeue/transfer budget? Does the heartbeat/supervisor path
  surface near-expiry as readiness debt? Construct the launch-fresh-then-expire-
  mid-session case: does it still leak into generic MCP-discovery recovery, or is
  it caught? Is the freshness check itself cheap and race-free?
- **Falsifier 2 (F2 / credential-custody + OAuth-safety lens):** Can a lane
  still obtain raw refresh-token custody by ANY route? Can a lane-side refresh
  still rotate/invalidate the operator source credential family? Build the
  concurrent-lane and subsequent-lane RTR cases — do they desynchronize? Does
  the broker/access-token model keep provider OAuth separate from Striatum
  control-plane tokens (no daemon/admin authority leaked to lanes)? Is any raw
  OAuth material exposed in DB rows / artifacts / metrics / events / doctor?
  Then verify NO carry-forward regressed (the spawn-time gate, the trust
  boundary, `provider_auth_gate=off` cannot bypass, redacted custody receipts).

The adjudicator gates on whether F1 and F2 are each **genuinely discharged**
(mechanisms anchored to real source — the lane-launch/hydration path, the
recovery decision tree, the heartbeat/supervisor path; the named RTR +
runtime-expiry tests + controls specified) and whether any carry-forward
regressed or any **new** material challenge lands. A clearing verdict
(`accept` / `accept_with_findings`) requires both F1 and F2 discharged with
their controls and no standing regression. This is the single allowed v3
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator (who spins a fresh `-v4` run with a revising holder). Keep the
local-first boundary (one host, no hosted services, no external persistence).
