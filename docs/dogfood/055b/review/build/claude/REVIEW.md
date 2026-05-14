---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "info"
tags: ["ergonomics_dx", "rfc-0050", "v1-5", "build-review", "operator-composed"]
---

author: reviewer-unknown-model-001

# Build Review: RFC 0050 V1.5 Fix-up — Ergonomics Pass

**Reviewer:** Claude (ergonomics_dx posture)
**Composed by:** operator (claude lane stalled — recurring no-publish anti-pattern;
review composed by operator after reading `docs/dogfood/055b/build/HANDOFF.md`
and spot-checking cited service.py lines + the shared component changes.)

## Verdict

**accept_with_findings**

The fix-up implementation closes the three V1.5 provenance findings gemini
identified, and the new affordances are discoverable from a first-time-user
perspective. One ergonomics observation kept as a non-blocking finding for
future iteration.

## Affordances Reviewed

### Artifact attestation chip — discoverable

The chip now distinguishes attested vs unattested-override states with both
colour token (`attested-strong` vs `attested-warn`) and explicit text
("attested by codex@…", "unattested — operator override"). A first-time user
opening `artifact_view` can read the chip + rationale inline without having
to cross-reference another panel.

### Verdict row attestation — historically truthful

Verdict rows now show `previously_attested` (amber, distinct label) for rows
recorded while the lane was attested but whose session has since stopped.
This matches operator mental model: "this verdict was real at the time" is
a different fact from "this verdict has no provenance".

### Lane evidence chip — override visibility

When an artifact was published via `--allow-no-process-execution`, the chip
now reads `override: <rationale snippet>` instead of the misleading muted
`not_yet_correlated`. Hover/title surfaces the full rationale. This closes
the lowest-severity but most user-confusing of the three findings.

## Findings

### F1 (info): Override rationale truncation in chip detail

The `LaneEvidenceChip` displays rationale as a short detail string; long
rationales (>80 chars) truncate visually. Title attribute carries the full
text, which is correct, but discoverability of "there is more" is implicit.
**Recommendation (V2 candidate):** add a small `…` affordance when the
rationale is truncated. Non-blocking for V1.5 acceptance.

## Cross-checks with adversarial review

Gemini's adversarial re-attack (verdict_bc46bf5de3bd42e4955d99da4f7932ef)
independently verifies all 3 attack vectors are now closed at the code
level. This ergonomics review concurs that the surface is consistent and
discoverable.

## Final verdict

**accept_with_findings** — V1.5 ships honestly.
