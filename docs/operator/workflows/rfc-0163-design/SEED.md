# Design-Run Seed — RFC 0163 (FRESH v1)

> Fresh v1 `falsification_gate` design run for RFC 0163 (staged-not-adopted
> offline consolidation + attested noise-robust gate). **Required context docs**
> (read in full first):
> - `docs/rfcs/0163-staged-not-adopted-offline-consolidation.md` — the RFC (the two coupled mechanisms, the SkillOpt-Sleep genotype, the go/pkg integration seam).

## ⚠️ This carries an UNRESOLVED PRODUCT DECISION — surface it, do not assume it

The maintainer has **not** decided that striatum should do offline
self-improvement at all. This design run hardens the *safety-by-construction
mechanism* **conditional on** a future "yes" — it does **not** adopt the
capability. The holder's SPEC must put the product question at the TOP as the
gating precondition ("IF the product decision is yes, here is the safe
mechanism"), and must land **no default and no code**. A clearing verdict
ratifies the mechanism's safety, not the product direction.

## Charter

The deliverable (committed `PROPOSAL.md`) is the falsifiable spec the
`rfc-0163-build` run would execute **only if the product decision lands yes**.
The goal: make "offline self-improvement is safe to run unattended" a
**structural** property, not a policy.

## The two coupled mechanisms to harden

1. **Staged-not-adopted by construction (self-adopt is UNREPRESENTABLE).** A
   `MergeToken` Go type whose only constructor is an unexported
   `mintMergeToken(...)` callable solely from the human-cleared lane's keyring
   package; a nightly-optimizer packet carries `Capability = ProposeOnly` and has
   **no `MergeToken` value** to pass to `Adopt(tok, sig, replay)`; adoption needs
   **two keys** (operator signature + a *fresh adopt-time* deterministic
   `--assert-improves` replay that agrees with the staged verdict). The new
   capability/merge-token types must **map onto striatum's real
   machine-cleared/human-cleared gate** (these literal terms are NOT in `go/pkg`
   today — confirm the real type; anchor to `go/pkg/{lanehealth,supervisor,agentloop,verifier,mutations}`).
2. **Attested noise-robust gate.** Accept a candidate only when it beats the
   incumbent **beyond measured noise** on a freshly-resampled check-set; chain a
   **signed verdict-receipt** (partition-hash + incumbent re-score + noise-margin)
   into the append-only provenance ledger so a faked or rotted gate is detectable
   after the fact.

## HARD product-boundary constraint

Local-first only: **no** hosted services, cloud APIs, telemetry, durable
transcript capture/export, or external persistence (AGENTS.md). The consolidation
runs on-box against local provenance. Note that mining/replaying past run
transcripts must NOT require durable transcript capture the product deliberately
does not keep (D028 — stdout/stderr off by default; PTY logs are private
diagnostics, not workflow state). If the design needs durable transcripts, that
is a blocking finding, not an assumption.

## Falsifier guidance

- **Falsifier 1 (staged-not-adopted structural):** find any route to a
  `MergeToken` for a propose-only packet (reflection, test export, zero-value,
  rehydration, keyring importable by a non-human-cleared lane); show the two-key
  adoption doesn't bind or the replay isn't independent; show the type model
  doesn't map onto the real gate.
- **Falsifier 2 (gate-soundness / product-boundary):** show the noise gate can
  pass a no-op/regression (sample size, multiple-comparisons, p-hacking); show
  the receipt is forgeable/replayable; show any product-boundary crossing
  (esp. a durable-transcript requirement); or that the design assumes the product
  decision.

The adjudicator gates clearing on both mechanisms sound + inside the product
boundary + the product question honestly surfaced. Single v1 revision cycle; a
second `needs_revision` routes to the operator.
