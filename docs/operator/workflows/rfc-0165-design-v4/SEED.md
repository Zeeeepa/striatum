# Design-Run Seed — RFC 0165 (REVISION v4)

> This is the **v4 revision** of the RFC 0165 design run (Claude provider-credential
> freshness + spawn-time hydration, #583). v3 ("daemon-brokered access-token
> projection for Claude lanes") cleared the original v1 findings F1/F2 and
> preserved the carry-forwards, but the v3 adjudicator found **three binding
> constraints** (C1/C2/C3) and, its single cycle exhausted, routed them to the
> operator. This run discharges C1/C2/C3 while carrying forward everything
> cleared through v1+v3. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md` — the v3 SPEC you are revising (the base; do not rewrite from scratch).
> - `docs/operator/artifacts/rfc-0165-design-v3/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v3 verdict; its `constraints:` (C1/C2/C3) are the exact prescribed fixes.
> - `docs/rfcs/0165-claude-provider-credential-freshness.md` — the RFC.

## Relationship to RFC 0169 (kept SEPARATE by operator decision)

RFC 0169 (provider-agnostic lane credential readiness) is the **cross-provider
spine** and runs as its own design. **RFC 0165 v4 stays CLAUDE-specific** — the
Claude assurance class / daemon-broker custody story. 0169 defines the uniform
refuse-closed readiness contract every provider adapter must satisfy; 0165 v4
hardens the Claude adapter's deep custody (no raw refresh-token to the lane).
Note the seam so the two coexist without fully duplicating, but v4 must stand on
its own — do not defer C1/C2/C3 to 0169.

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0165** the downstream `rfc-0165-build` `code_change`
run executes. It must **resolve C1, C2, and C3** and **carry forward, unregressed,
everything cleared through v1+v3**. A revision that leaves any of C1/C2/C3 open —
or regresses a carry-forward — has NOT cleared the gate.

## Carried forward — CLEARED through v1+v3 (do NOT reopen, do NOT regress)

- **The access-token-only projector** in `laneproviderauth`: the daemon projects
  a short-lived **access token** to the lane (B1 lane-owned `0600` file or B2
  `SO_PEERCRED` broker socket) that **never carries a refresh token**.
- **Path / ownership rules**; the **spawn-time projection gate** (placement
  unchanged); the **F1 runtime-expiry circuit breaker** + decay signal.
- **Durable state + receipts** — the access token is **never persisted**.
- **Redaction contract** — no raw OAuth material in DB rows, repo artifacts,
  metrics, events, or doctor output.
- The **RFC 0096 trust boundary** (lanes authenticate to Striatum with their own
  session-bound capability token; never daemon/admin tokens) and
  `refresh_token_absent`.

## The three binding constraints to DISCHARGE

### C1 — daemon-owned state is the ONLY positive freshness authority in recovery

v3's recovery freshness check was not race-free / positively daemon-owned: a
stale daemon row could be upgraded to ready (a lane writes a future-dated value
with no refresh token), or a missing/inconsistent row could be inferred green.
The revised SPEC must make a **stale / missing / internally inconsistent**
`provider_auth_dependencies` (or broker) row **fail CLOSED** to a typed
**reseed-required** classification — never a generic MCP-discovery retry, never a
green/ready inference. Specify the **race-free** read: the daemon row is
authoritative; the lane-side file/socket is never the freshness oracle.

### C2 — explicit same-user decision for Claude OAuth self-driving lanes

v3's F2 proof holds only for a **distinct-UID** lane. Under
`config.RunAsUser == ""` (or a value resolving to the daemon/operator identity)
the lane shares the operator's home and can read the raw credential. The revised
SPEC must **fail CLOSED** when `RunAsUser` is empty or resolves to the
daemon/operator identity, with a **typed launch-precondition error** — no silent
same-user credential sharing.

### C3 — no raw refresh-token custody by ANY route

Even via the same-user shape, the lane must never reach the rotating refresh
token. The revised SPEC must either declare Claude OAuth **same-user
self-driving lanes UNSUPPORTED** (fail closed), OR have the daemon **broker**
mint/serve only short-lived access (the projection carries NO refresh token;
refresh authority held in exactly one place). Required test: a **same-user
fixture** asserting no refresh token, scanning **EVERY lane-readable credential
surface named by the launch environment** (not only the newly written projection
file).

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1 + C2 lens):** Can a stale/missing/inconsistent daemon row
  ever go green or route to generic retry instead of reseed-required (construct
  the race)? Is daemon-owned state the only positive authority? Does the
  same-user shape fail closed at the right gate with a typed error?
- **Falsifier 2 (C3 + carry-forward lens):** Can the lane reach the rotating
  refresh token by ANY route (distinct-UID, same-user, a lane-readable file/env/
  fd, a fallback)? Is the same-user no-refresh-token test exhaustive over every
  lane-readable surface? Did any carry-forward (access-token-only projector,
  spawn-time gate, F1 breaker, durable-state/receipts, redaction, RFC 0096
  boundary) regress?

The adjudicator gates on whether C1, C2, and C3 are each **genuinely discharged**
(mechanisms anchored to real source; named tests + controls specified) and
whether any carry-forward regressed or a **new** material challenge lands. A
clearing verdict (`accept` / `accept_with_findings`) requires all three
discharged with their tests and no standing regression. This is the single
allowed v4 revision cycle; a second `needs_revision` ends the gate uncleared and
routes to the operator (a fresh `-v5` run with a revising holder).
