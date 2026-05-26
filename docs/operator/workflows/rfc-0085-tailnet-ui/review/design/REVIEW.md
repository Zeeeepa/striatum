---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
---

# Threat-model finding

The design is close, but needs one revision before build: make the
tailnet-identity read-only route invariant explicit and testable.

Interrogation `intg_4b69c5624c4345699d3606cb95d07d7b` resolved the socket
spoofing boundary: answer turn 1 accepts same-uid/root header forgery as equal
to the existing runtime-dir bearer-token boundary, while non-owner local users
are blocked by `0600 web-ui.sock` and forged access remains read-only.

Answer turn 1 also resolved Host handling: MagicDNS Host and identity headers
are dedicated UI-socket properties; the main loopback listener stays
identity-blind, bearer-authenticated, and MagicDNS-Host-rejecting.

Answer turn 1 resolved fail-closed config: unset, empty, whitespace-only, and
empty-after-parse allowlists all normalize to an empty set and deny every
identity request; tests should cover those cases.

The unresolved design flaw is GET-only enforcement. In answer turn 3, the
designer clarified that the minimal slice should add a normative route-audit
test proving identity-socket GET routes are an audited read-only set and no
mutating route is reachable. The RFC/design defense should say this before
implementation, because the current "existing web handler + reject non-GET"
wording relies too much on "GET means safe."

SSE resource limits for `/v1/runs/{id}/events` can remain a documented follow-up
risk for allowlisted tailnet users; they are availability hardening, not this
verdict's blocker.
