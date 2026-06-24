# FALSIFIER - RFC 0164 Layer 3 recovery-contract challenge

author: falsifier-reviewer-004

## Gate-stopping challenge

G3/A26 is not established. The holder specifies a two-state Layer 3 recovery
contract where a recognized-and-neutralized gadget creates a
`gate.read_gadget_detected` blocker pinned to `config_fingerprint`, and the
only machine-clear condition is that `recovery.sweep` later sees a different
fingerprint. That is safe for a hostile key the repo removes. It wedges the
exact benign false-positive case RFC 0164 says must degrade observability only.

This is not a digest-canonicalization complaint. The holder gives enough
specificity for `argv_digest`, `env_allowlist_digest`, and `config_fingerprint`
to make the golden-vector work buildable. Nor is this a decay TOCTOU complaint:
A23 correctly says decay clears only the blocker and every later fork must
re-attest. The standing failure is the clearer-of-record state machine for a
stable benign detection.

## Claim challenged

The challenged claims are A24 and A26 in the holder SPEC:

- A24: recognized gadgets route to machine decay, unknown/unattested gadgets
  route to the human-cleared recovery lane, and neither silently passes.
- A26: a benign `[alias]` / `[pager]` false-positive degrades observability
  only and never wedges the run because Layers 1+2 already make config inert.

The contract text does not contain the transition that makes A26 true. In
`HOLDER.md:626-630`, a recognized-and-neutralized gadget writes a blocker that
is cleared only when the live fingerprint no longer matches. In
`HOLDER.md:632-635`, unknown/unattested keys hard-refuse into
`recovery.quarantine_lane`. In `HOLDER.md:650-654`, the named test only checks
known-vs-unknown routing. A26 at `HOLDER.md:727` names
`false_positive_benign_test`, but the state machine never says that a benign
recognized detection is non-blocking, safely classified, or clearable while the
benign config remains present.

## Concrete failing case

Use a normal target repository whose local config contains only stable
convenience settings:

```ini
[alias]
    co = checkout
[pager]
    log = less -FRX
```

Run an allowlisted read such as `git log --format=%H` or `git status` through
Layer 3 after Layers 1+2 are in place.

Expected by RFC 0164: the read proceeds, because the RFC risk section says a
benign `[alias]` / `[pager]` false-positive must be telemetry only; Layers 1+2
already make the config inert, so the detector must not affect correctness or
liveness (`docs/rfcs/0164-untrusted-substrate-read-side-git-hardening.md:239-244`).

What the holder contract permits instead:

1. The detector classifies `[alias]` or `[pager]` as a recognized gadget family.
2. The gate appends `gate.read_gadget_detected` and pins fingerprint `F`.
3. Because the gadget is recognized, it takes the machine-decay branch, not the
   human-cleared `recovery.quarantine_lane` branch.
4. `recovery.sweep` recomputes the live fingerprint. The repo is benign and
   unchanged, so the fingerprint remains `F`.
5. The blocker never clears unless the operator removes legitimate local config
   from the target repository.

That is a wedge of legitimate work, not observability degradation. The read-side
execution is already safe under Layers 1+2, but the recovery contract still
turns the detector result into a liveness blocker with no benign steady-state
escape.

## Why the likely rebuttals fail

The strongest rebuttal is that "recognized-and-neutralized" might mean
known-hostile, not every alias or pager key. The SPEC does not define that
classifier. RFC 0164 explicitly says a benign `[alias]` / `[pager]` is
indistinguishable from a hostile one until executed. A preflight detector cannot
prove an arbitrary alias or pager value benign without either executing
attacker-controlled config or specifying a precise safe language / safe list.
The holder gives neither.

The second rebuttal is that `gate.read_gadget_detected` is merely telemetry. The
holder does not say that. It calls the record a blocker, gives it a pinned
fingerprint, and says decay clears it. If this record is non-blocking, the SPEC
needs a third state such as `gate.read_gadget_observed` and must preserve the
hard-refusal route for unknown/unattested keys.

`corpus_green_hash` also does not rescue A26. A deterministic planted-attack
corpus can prove that no sentinel executes while still allowing the recovery
state machine to wedge on a benign stable fingerprint. The load-bearing test
must assert job/run liveness and recovery refs, not just absence of command
execution.

## Required fix before the gate can clear

The holder needs one explicit, test-backed state-machine rule:

- split recognized detections into a non-blocking observation state for
  neutralized benign-safe detections and a blocking state only for conditions
  that actually require recovery; or
- define a precise safe classifier for `[alias]` and `[pager]` values whose
  positive result does not create a blocking pin; and
- make `false_positive_benign_test` assert the liveness contract: with the config
  above, an allowlisted read succeeds, no `recovery.quarantine_lane` ref is
  created, no job/run blocker remains after the call, and a second read proceeds
  without a repo edit or human clear.

Pair that with the negative case: an unknown/unattested key still hard-refuses
into the human-cleared lane. Without this split, the design has only two bad
choices: benign recognized detections wedge real repositories, or recognized
detections silently pass under the label "telemetry." Either violates the G3/A26
contract.
