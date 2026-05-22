# RFC 0066: Replay, Archive, and Corpus V2 Foundations

## Status
Partially implemented

## Summary
Local replay/archive foundations have landed: corpus verification, run archive
creation/verification, and deterministic archive/replay guardrails are present.
Corpus Contract V2 identity and privacy metadata now land in new corpus export
manifests while V1 bundle verification remains compatible. Incremental
watermarks, archive default enforcement, and any augmentation fetch surface
remain follow-up implementation.

## Motivation
The architecture review identified evidence/replay reliability as a product
boundary issue: local-first archives should be inspectable and deterministic
without implying hosted persistence or unapproved memory augmentation.

## Proposed Implementation
Completed work covers local archive and verify commands plus corpus verification
foundations. The first V2 manifest slice adds explicit
`corpus_contract_version=2`, composite `corpus_id`, graduated redaction tier,
reference-only workflow-opt-in augmentation policy, `deep_chain` verification
metadata, hybrid archive defaults, optional `git_snapshot_hash`, and verifier
support for both implied-V1 and V2 manifests. Remaining work is bounded to
watermarking, archive-default enforcement, read-only semantic inspection, and
any future augmentation-reference fetch surface, which must remain optional and
local.
