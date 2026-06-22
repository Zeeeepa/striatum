# RFC 0163: Staged-not-adopted offline consolidation + attested noise-robust gate

Status: proposal (promoted from raw input; not yet adjudicated by a recorded decision)
Date: 2026-06-22
author: proposer-claude-opus-4-8 — promoted from `showerthoughts/prior-art-skillopt.md`; design diverged via `/adhd` (frames: regulator, logistics, 3am-on-call) + a 2-branch focus pass

## Summary

We want striatum to be able to **improve itself / its driven artifacts offline**
(a nightly "consolidation" run: harvest past run transcripts → mine recurring tasks
→ replay → propose edits) **without ever being able to ship an unreviewed change**.
The genotype is Microsoft SkillOpt's deployment companion *SkillOpt-Sleep*
(`harvest → mine → replay → GATE → stage proposal → human adopts`), re-expressed for
striatum's governance model. SkillOpt is MIT; no code copied. Raw-input provenance:
`CONSOLIDATED-FROM: showerthoughts/prior-art-skillopt.md`.

striatum already has the right primitives — the machine-cleared vs human-cleared gate
distinction, the **"never fake a gate"** rule, isolated lanes, self-contained
work-packets, autonomy levels, shadow mode, and an append-only provenance ledger.
This RFC asks for a product decision on **two coupled mechanisms** that make
"offline self-improvement is safe to run unattended" a *structural* property, not a
policy:

1. **Staged-not-adopted by construction.** A nightly-optimizer packet is minted
   `propose-only` and **physically lacks the merge capability**; adoption needs two
   keys (operator signature + a *fresh, adopt-time* deterministic `--assert-improves`
   replay that must agree with the staged verdict). Self-adopt is *unrepresentable*,
   not merely denied.
2. **Attested noise-robust gate.** Accept a candidate only when it beats the
   incumbent **beyond measured noise** on a freshly-resampled check-set, and chain a
   **signed verdict-receipt** (partition-hash + incumbent re-score + noise-margin)
   into the provenance ledger so a faked or rotted gate is detectable after the fact.

No code, contract, or default change lands with this proposal.

## Context

- The integration seam exists: `go/pkg/lanehealth`, `go/pkg/supervisor`,
  `go/pkg/agentloop`, `go/pkg/verifier`, `go/pkg/mutations` (verified present
  2026-06-22). The capability/merge-token *types* below are **new** and must be
  mapped onto striatum's actual machine-cleared/human-cleared gate representation
  (the literal terms aren't in `go/pkg` today — confirm the real type).
- Genotype witness: SkillOpt-Sleep (`SkillOpt/docs/sleep/README.md:21-39`) and its
  validation gate (`skillopt/evaluation/gate.py`).

## Design

### 1. Staged-not-adopted by construction

- **Capability boundary as a Go type.** `MergeToken` is a struct whose only
  constructor is an *unexported* `mintMergeToken(...)` callable solely from the
  human-cleared lane's keyring package. A nightly-optimizer packet carries
  `Capability = ProposeOnly` and the merge path is
  `func (p *WorkPacket) Adopt(tok MergeToken, sig OperatorSignature, replay AssertImprovesReceipt) error`
  — a `propose-only` packet has **no `MergeToken` value to pass**, so self-adopt won't
  compile, let alone run. This single type + constructor-visibility decision is the
  load-bearing boundary; everything else hangs off it.
- **Two-key adopt.** Key 1 = operator signature in the human-cleared lane. Key 2 = a
  **fresh deterministic `--assert-improves` replay run at adopt-time** (not
  stage-time). If the adopt-time score vector diverges from the staged one, adopt
  **aborts, pins last-known-good, and logs both vectors** — killing stale-staging rot.
- **Signed verdict-receipts.** Every gate decision writes a receipt (partition-hash +
  incumbent re-score + noise-margin) chained into the append-only provenance ledger;
  a missing or out-of-order receipt is a **hard refusal**, so faking the gate means
  forging an unbroken hash chain.
- **`gate-was-real` challenge.** Any reviewer/critic can deterministically re-derive a
  past verdict from the committed bundle; a non-reproducing re-derivation
  **auto-quarantines the entire downstream dependency cone** (consider a Merkle-DAG
  ledger so the quarantined cone is *minimal*, not "everything after").

### 2. Attested noise-robust gate

Accept work only if it beats the incumbent by **more than measured run-to-run
variance** on a freshly-resampled check-set, with a **power-precondition lane** that
refuses to even run the accept-gate until the check-set is large enough — emitting
`inconclusive` (distinct from `rejected`) when it isn't. (This is the striatum-side
port of engram RFC 0084's `paired_gate_decision`; share the statistic.)

### 3. Safe-unattended envelope (worst case = a no-op night, never a page)

- **Canary self-test (validator-of-the-validator).** Each run **prepends** a
  synthetic known-good/known-bad packet pair with hard-coded expected verdicts; if the
  gate doesn't pass the good and reject the bad, the **whole run aborts** before any
  real packet is touched. This catches the one failure that makes every other
  safeguard a lie: a silently-degraded, drifting-to-always-accept gate. **Sign the
  canary contract** (hash-pin packets + expected verdicts) so a prior bad night can't
  have edited the canary to always pass.
- **Blast-radius token bucket per lane.** Fixed budget of mutated-file-tokens /
  packet-count / wall-minutes; exhausting any bucket halts the lane and stages a
  `budget-exhausted` marker.
- **Reverse-diff reversibility as an adopt precondition.** Before LKG is replaced,
  compute + commit the exact **inverse patch + input-snapshot hash**; adopt **fails
  closed** if the inverse can't re-apply cleanly against current HEAD. You can't adopt
  what you can't prove you can un-adopt.
- **Two-clock staleness lease on "live."** If no operator key touches the lane within
  the lease window, **pin LKG and refuse to stage new candidates** until a human
  re-arms — so an unattended weekend can't pile up un-audited edits that pressure a
  tired operator into a bulk adopt.
- **Check-set drift tripwire.** Snapshot-hash the resampled check-set each run; if its
  *own* baseline scores move beyond the measured noise band, **freeze adoption
  lane-wide and alarm** — a moving ruler is a first-class incident.

### 4. Doc-as-costed-resource (work-packet leanness)

Compaction of a work-packet is itself a **gated packet that must re-pass all
governance gates at strictly-smaller byte size**, emitting a compaction-receipt
(pre/post bytes + gate verdicts). Preserves KV-cache append-only discipline; bloat
can't smuggle in behavior change because it re-runs the gates.

## Alternatives considered

- **Capability as an unforgeable *witness*** (`*MergeWitness` returned only by
  `keyring.Authorize(receiptChain)`) — fuses the type boundary with receipt
  validation (can't get the type without the proof), at the cost of a heavier
  authorize call.
- **OS/process sandbox** instead of a Go type boundary — the optimizer lane physically
  lacks IPC to the keyring socket; stronger against `unsafe`/reflection escapes, but
  moves the invariant out of the type system where it's harder to test.
- **Tolerance-band + k-of-n replay quorum** instead of bit-identical adopt-time replay
  — robust to genuine eval noise, weakens the crisp "divergence aborts" guarantee.
- **Earned-trust credit ledger** instead of a per-run reset bucket — a clean lane earns
  autonomy, a flaky lane is throttled automatically.
- **Logistics framings** (worth keeping as ops design): cross-dock replay (no packet
  sits in staging un-replayed), a returns/defect-bin keyed by failure-mode with
  diff-hash dedupe (never re-propose a known-bad edit), consignment byte-budget with
  TTL revert, a milk-run shared check-set partition per scheduling tick, an express
  lane for repeatedly-low-variance packets.

## Risks (load-bearing)

- **Determinism of the re-score path.** Both two-key adopt and the `gate-was-real`
  challenge assume `--assert-improves` replays bit-identically. Any non-determinism
  (model sampling, float reduction order, dataset ordering, RNG/wall-clock seeding,
  GPU variance) makes legitimate adopts spuriously abort **and** makes honest
  historical gates fail re-derivation and auto-quarantine healthy cones. The verdict
  must be a pure, seeded, environment-pinned function of the committed bundle — this
  is the hardest prerequisite.
- **A silently-degraded gate** is the root threat; the signed canary is the only thing
  validating the validator, so its correctness and tamper-evidence are the true root
  of trust.

## Rollout

1. `go/pkg/lanehealth.PreflightGate` + the **signed canary contract**; red test:
   inject a stub always-accept gate, assert `RunCanary` returns abort and the
   supervisor stages a `canary-failed` marker. (Validator-of-the-validator first.)
2. The `MergeToken` type + constructor-visibility boundary; compile-time proof that a
   `propose-only` packet can't call `Adopt`.
3. Verdict-receipt chain + `gate-was-real` challenge command.
4. Two-key adopt (adopt-time replay) + reverse-diff precondition + staleness lease +
   drift tripwire, behind shadow mode until the determinism risk is closed.

**Kill-gate:** if the re-score path can't be made deterministic enough that an
incumbent re-derives its own historical verdict without spurious quarantine, **stop**
— the attestation half is unsound on this eval, and only the staged-not-adopted
capability boundary (which doesn't need determinism) should ship.
