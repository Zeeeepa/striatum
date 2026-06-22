# Design-Run Seed — RFC 0143 Lane credential survival across a daemon boot-epoch rotation

> This document is the **required input** for the RFC 0143 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed
> at `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`
> (status `proposed`) — read it in full as your primary source; this SEED carries
> the charter, restates the four Open Questions, and pins an operator
> anchor-verification table you must build on. Read this whole file and the RFC
> before producing any artifact.

## Charter — what this run must produce

This is a **design run**, not an implementation run. The deliverable is a
**falsifiable implementation spec** for RFC 0143: a concrete, buildable
specification that the `rfc-0143-build` run can execute contract-first (TDD),
produced by hardening the RFC against adversarial falsification.

This is a **security/authz-hot** decision (the RFC's `security_or_authz` blast
dimension). The committed `PROPOSAL.md` MUST:

1. **Resolve all four Open Questions** (below) with a concrete, defensible
   decision each — *which option / which mechanism / why*. A design run that
   leaves an Open Question unresolved has not cleared the gate.
2. **Hold the security invariant as the spine:** preserve the RFC 0096/#135/#296
   session-bound trust model. No lane ever reads the daemon's full-authority
   bootstrap admin client-token; no new lane-readable credential carries any of
   `{admin, apply, recovery, surgical_recovery}`.
3. **Name the exact surfaces** (files + functions + the new credential's
   ownership/mode/TTL) from the anchor table below.
4. **State every load-bearing claim as a falsifiable assertion** paired with the
   named test / game-day step that would prove it false.
5. **Flag maintainer ratification:** the cleared spec is a RECOMMENDATION the
   maintainer ratifies before any build slice touches credential code.

## Root reframe (do not lose this)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin token and exiting silently unsealed.**
Today, when a lane loses its live RPC connection across a daemon restart, its
credential-resolution chain falls through to the admin `client-token` (which a
`striatum-lane` lane cannot read), so a complete-on-disk deliverable exits
unsealed with a misleading permission error. Either give the lane a *narrow,
session-scoped, lane-readable* credential that survives the rotation, or make the
failure *loud and routed* — never silent, never via the admin token.

## The four options (from the RFC §"The unresolved decision")

1. **Status quo** — orphan the in-flight output as complete-but-unsealed; operator
   requeue (`supervise stop` → `session close` → `recovery auto`) is the supported
   recovery. Accept the interruption.
2. **Durable lane-owned reseal token** — mint a `striatum-lane`-owned `0600` token
   carrying only the caps needed to seal the in-flight job, invalidated on session
   close, bounded by the session TTL. Preserves the trust model; adds a new
   credential file + lifecycle. *(Handoff-recommended trust-preserving direction.)*
3. **Re-mint + re-inject on rotation** — the boot-epoch rotation handshake re-mints
   and re-injects the session-bound token into the live lane env/token file, so the
   chain stops at step 1/2 and the lane never needs the admin token.
4. **Narrow the fallback (legible failure)** — make the resolution chain refuse to
   reach the admin `client-token` for a non-owner lane and surface a typed,
   self-escalating "session unrecoverable across rotation" signal so the run routes
   it. Does not reseal, but removes the silent/misleading dead-end. Combinable
   with 1.

## Open Questions to resolve (design calls)

1. **OQ1 — Which option (the trust-model shape).** Pick the primary survival
   mechanism (1/2/3) AND whether to also land option 4 as the immediate safety
   net. This is THE security/authz call.
2. **OQ2 — Surviving authority + lifecycle.** If a new credential class: which
   exact caps survive a rotation (just `write` to seal? the lane's normal
   `{claim,write,read,review}`?), TTL/window, file ownership/mode, and the exact
   invalidation triggers (session close / TTL / new boot epoch).
3. **OQ3 — Where the mechanism lives.** Name the exact code site(s) for the mint,
   the injection, the rotation handshake (option 3), and the resolution-chain
   refusal (option 4).
4. **OQ4 — Legible-failure fallback.** Define the self-escalating failure when the
   lane genuinely cannot reseal, and how the run's recovery routes it.

## Load-bearing risks (attack these)

- **R1 trust-model widening:** any option that exposes the admin token, or mints a
  credential carrying elevated caps, is categorically out of bounds. Test: present
  the new credential for `admin`/`apply`/`recovery` → REFUSED.
- **R2 durable-file replay:** a lane-readable token file is a leak surface (the
  reason today's token is env-only/in-memory). Must be invalidated on session
  close, TTL-bound, epoch-bound, and not readable by a different lane user.
- **R3 split-brain across rotation:** a reseal must only seal the in-flight job the
  daemon still recognizes; it cannot write into a session the daemon retired.
- **R4 silent-failure regression:** option 4 must make the failure LOUD
  (self-escalating recovery class wired into routing), not swap one silent exit for
  another.

## Anchor verification against current `main` (operator pre-flight, 2026-06-22)

Verified against `~/git/striatum` @ `main` (all claims **ACCURATE**, no drift).
Treat as ground truth; re-anchor the spec to these file:line references.

| RFC claim / area | Status | Anchor (current source) |
| --- | --- | --- |
| Session-bound token grants `{claim,write,read,review}`, 24h TTL, bound to `session_id` | **ACCURATE** | `go/pkg/mutations/session_token.go:21` `sessionBoundTokenTTL = 24*time.Hour`; `:46` `sessionBoundCapabilities = {Claim,Write,Read,Review}`; grants bound to `sessionID` at `:77-89`. |
| Injected as `STRIATUM_MCP_TOKEN` into lane env at supervise start | **ACCURATE** | `go/pkg/mutations/supervision_env.go:342` `entries = append(entries, "STRIATUM_MCP_TOKEN="+boundToken)`; minted in `HandleSuperviseStart` tx (`supervision_control.go:158`). |
| Token does NOT rotate on a normal restart (only the endpoint does) | **ACCURATE (load-bearing)** | `go/pkg/agentloop/endpoint.go:111-112`: "The token does NOT rotate on a normal daemon restart (only the endpoint does)…". |
| 4-step credential-resolution order (env literal → env file → runtime client-token → repo `.striatum/capability_token`) | **ACCURATE (load-bearing)** | `go/pkg/agentloop/token.go:18-20` (1, `STRIATUM_MCP_TOKEN`), `:23-28` (2, `STRIATUM_MCP_TOKEN_FILE`), `:31-41` (3, runtime `client-token` via `admin.RuntimeTokenPath()` / `STRIATUM_DAEMON_RUNTIME_DIR`), `:44-51` (4, repo token). The bug fires at step 3. |
| Runtime `client-token` is the full-authority bootstrap admin token, `0600` owner-only | **ACCURATE (load-bearing)** | `go/pkg/admin/bootstrap.go:18-27` `bootstrapCapabilities = {admin,read,write,claim,review,apply,recovery,surgical_recovery}`; `writeRuntimeToken` `:139-142` mkdir `0700`, `:153/:164` chmod `0600`. |
| `ReadTokenFile` refuses any token file not owner-only | **ACCURATE (load-bearing)** | `go/pkg/agentloop/token.go:81-84` `if mode&0077 != 0 { return "", errors.New("daemon token file is not owner-only") }`. |
| Daemon publishes a boot-epoch file + rotates the mcp-http-endpoint on a boot-epoch change | **ACCURATE (load-bearing)** | `go/cmd/striatumd/main.go:700` `bootEpochFileName = "mcp-boot-epoch"`, `:748-749` writes atomically; `:465-486` computes `daemonBootEpoch()` and injects `BootEpoch` into lane env (`supervision_env.go:352-354`). |
| #323 endpoint-rotation recovery re-reads token via `ResolveTokenMaterialFresh` and reconnects | **ACCURATE (load-bearing)** | `go/pkg/agentloop/loop.go:600-605` re-reads token on rotation; `:619-658` `applyMCPEndpointRotation` rewrites ephemeral config + reconnects ("#323 rotation recovery"). |
| A durable, lane-readable session-token file does NOT exist today | **NOT-FOUND (ACCURATE)** | No `striatum-lane`-owned token-file write anywhere in `go/pkg/`; the survival credential is genuinely new. |
| Lanes run as OS user `striatum-lane` via `sudo -n -u striatum-lane` | **ACCURATE** | `docs/how-to/lane-sandbox.md:34,52,99`; daemon launches lane commands via `sudo -n -u <lane-user> -- env -i …`. |

**Net design implication.** The RFC's premise is fully current: the bug is the
step-3 fall-through to an unreadable admin token across an endpoint rotation,
while the session-bound token (env-only, never on disk) is correct-by-design but
unreachable once the live RPC connection drops. The single most important
decision is OQ1 — and the falsifiers must press hardest on the security invariant
(no widening, no replay, no split-brain). Lean: option 4 (legible failure) as the
immediate, zero-trust-change safety net, plus a narrow option-2/3 survival
mechanism — but the gate decides.

---
<sub>Operator scaffold for the RFC 0143 falsification-gate design run. Lanes:
author=claude (holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>
