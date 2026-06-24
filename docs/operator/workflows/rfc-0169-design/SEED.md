# Design-Run Seed — RFC 0169 P0/P1 (FRESH v1)

> Fresh v1 `falsification_gate` design run for RFC 0169 (provider-agnostic lane
> credential readiness). RFC 0169 reframes the #583 stale-credential class: a
> code audit found the three providers do NOT share a credential model — **claude**
> copies rotating OAuth it cannot self-refresh (the #583 wedge, the odd one out);
> **codex** owns + self-refreshes its token (RFC 0121, fine); **agy/gemini** has no
> provider OAuth — the daemon mints an ephemeral credential per spawn (already the
> target architecture). The genuine gap is not "more hydrators" but a missing
> provider-agnostic spine and convergence onto agy's mint-fresh-per-spawn model.
> **Required context docs** (read in full first):
> - `docs/rfcs/0169-provider-agnostic-lane-credential-readiness.md` — the proposal (the 3-layer design, acceptance criteria, P0–P4, 6 open questions).
> - `docs/rfcs/0165-claude-provider-credential-freshness.md` — the Claude-specific predecessor 0169 generalizes (the `OAUTH_COPIED` class).
> - `docs/operator/artifacts/rfc-0165-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the 0165 v1 findings (runtime-expiry-after-hydration; refresh-token-rotation desync) — the same failure modes 0169's spine must close generally.

## Relationship to RFC 0165 (kept SEPARATE per operator decision)

The operator decided 0165 and 0169 run as **separate** designs: 0165 (v4) hardens
the concrete Claude assurance class now; **0169 is the provider-agnostic spine**.
This design must therefore stay at the **spine + structural-prevention** altitude
(the contract, the registry, the fresh-per-spawn placement, the tamper-proof
custody/breaker) and treat the Claude specifics as the `OAUTH_COPIED` class that
0165 details — per 0169 Open Question 6, 0165 remains the `OAUTH_COPIED` detail
doc, 0169 the umbrella. Do not re-derive 0165's Claude-only mechanics here; cite
them.

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the falsifiable
implementation spec for **RFC 0169 P0–P1 (the spine) + P2 (ephemeral placement)**
the downstream `rfc-0169-build` `code_change` run executes. It must turn the
3-layer design into build-bearing, falsifiable constraints + the test that
refutes each, and prove the two load-bearing claims below.

## The two hard claims to PROVE

1. **The contract spine is a behavior-preserving refactor.** A single
   `ProviderReadinessAdapter` registry replaces the ~5 hardcoded switch sites
   (`agentloop/loop.go`, `supervision_lane_config.go`, `laneproviderauth/resolver.go`,
   `laneproviderauth/expiry.go`, `laneproviderauth/lane_provider_auth.go`); the
   `supervise.start` gate (`go/pkg/mutations/supervision_provider_auth.go`)
   collapses to one registry lookup + `ValidateReadiness`, refuse-closed on any
   error/timeout/panic; an unregistered provider is a typed refusal
   (`provider_readiness_unregistered`), not a fall-through. PROVE the codex
   `OAUTH_LANE_OWNED_SELF_REFRESH` predicate is RFC 0121's existing offline
   `Check` **verbatim** (zero behavior change) and agy's `EPHEMERAL_DAEMON_MINTED`
   is a no-op receipt (zero behavior change). The seam must be proven before any
   new logic lands (P1 pure refactor).
2. **Spawn-fresh placement makes #583 structurally impossible WITHOUT modifying
   the provider CLI.** The third-party CLIs (claude/codex/gemini) read credentials
   from FILES and call vendor APIs directly — no fd-injection, no custom-auth
   endpoint, no in-process callback. So the SPEC must keep a real file the CLI can
   read, but make it spawn-fresh, lane-private, mode `0600`, re-minted every launch
   via a `CredentialPlacer` (agy's `writeEphemeralGeminiSettings` is the first impl,
   zero change; `ClaudeCredentialPlacer` added). For Claude, "fresh source" = copy
   the operator's already-refreshed file at spawn with a rotation-race guard (read
   source before AND after copy; refuse `provider_credential_source_unstable` on
   mid-copy rotation). PROVE a `HOME`/`XDG_CONFIG_HOME` lane-private prefix actually
   makes the claude CLI read the placed file.

## The six open questions to DISCHARGE (each → a build-bearing constraint + test)

1. **OQ1 — Runtime-freshness closure** (admission-freshness ≠ runtime-freshness;
   the mid-run-expiry half of #583): re-admission heartbeat, cleanup-as-lease-
   boundary, or both? What re-check interval for `OAUTH_COPIED`? This is a GATE
   property — a spawn-only gate does NOT close #583.
2. **OQ2 — Placement strategy default**: atomic chown-write (universal, no kernel
   cap) vs per-lane tmpfs + setuid helper (stronger). Per-deploy flag or pick one?
3. **OQ3 — Generation identity for opaque providers (codex)**: monotonic-dispatch-
   seq + advisory-STALE-only (lane may report itself stale, never authoritative-
   fresh), or a daemon-side secondary probe for FRESH?
4. **OQ4 — Minimum expiry lead time** for `OAUTH_COPIED` placement: fixed (10/30
   min) or a fraction of observed TTL?
5. **OQ5 — Run dependency?**: should provider-auth readiness become a first-class
   `run.start` dependency so the RFC 0122 scheduler parks dependent jobs behind one
   reseed gate (turning "8 lanes wedge on one dead token" into "the run never
   starts until fresh"), or stay launch-time? (Surface; don't over-commit P0.)
6. **OQ6 — Subsumption**: does 0169 mark 0165 superseded, or does 0165 stay the
   `OAUTH_COPIED` detail? (Per operator: KEPT SEPARATE → 0165 stays the detail.)

## Tamper-resistance (a lane is sandboxed but UNTRUSTED)

Layer 3 (custody receipts + bad-generation immune memory + circuit breaker) must
be hardened against a compromised lane OS user: custody fingerprints + breaker
counters live ONLY in daemon state, never in a lane-readable file (no
spoof-via-touch/mtime, no exfil side channel); the placer holds a short-lived,
job-scoped, single-use daemon-minted capability (closes the privilege-bridge /
replay); destination owned by exactly the lane user, `0600`, written by
fd/atomic-rename or per-lane namespace (defeats symlink/TOCTOU); no credential
bytes / OAuth tokens / full operator path / provider stdout in DB rows, repo
artifacts, metrics, events, or doctor output. Preserve the RFC 0096/#135/#296
trust boundary (lanes get only their session-bound token; no daemon/admin token,
no minting authority, no other provider's credential).

## Falsifier guidance (attack the v1 proposal)

- **Falsifier 1 (structural-prevention + CLI-compatibility + runtime-closure
  lens):** Does spawn-fresh placement ACTUALLY close #583 given the CLIs read
  files and Striatum cannot modify them? Does a lane-private `HOME`/`XDG_CONFIG_HOME`
  override actually make claude read the placed file (not a global `~/.claude`)?
  Does the source-rotation-during-placement race truly close (before/after read)?
  Does the runtime-freshness closure (OQ1) actually catch a mid-run operator-source
  expiry, or is there a window where the lane 401s with no recovery? Is the
  cleanup-as-lease-boundary signal race-free?
- **Falsifier 2 (refactor-soundness + tamper-resistance + carry-forward lens):**
  Is the registry refactor genuinely behavior-preserving for codex (RFC 0121
  `Check` verbatim) and agy (no-op) — or does collapsing 5 switch sites change a
  predicate? Is Layer 3 genuinely tamper-proof against a compromised lane (can a
  lane read/reset/decrement the breaker, spoof a custody receipt, or replay the
  placer capability)? Does the generation key work for opaque codex without
  trusting a lane's FRESH claim? Does the placer's privilege bridge over-grant
  (arbitrary-path write, symlink/TOCTOU, cross-provider read)? Does an unknown
  provider truly refuse-closed?

The adjudicator gates on whether the two hard claims are PROVEN (the refactor is
behavior-preserving and source-anchored; spawn-fresh placement structurally
closes #583 without CLI modification), each open question is genuinely discharged
with a named test, and Layer 3 is tamper-proof against an untrusted lane. A
clearing verdict (`accept` / `accept_with_findings`) requires both claims proven,
the runtime-freshness closure real, and no standing falsifier challenge. This is
the single allowed v1 revision cycle; a second `needs_revision` ends the gate
uncleared and routes to the operator (a fresh `-v2` run with a revising holder).
