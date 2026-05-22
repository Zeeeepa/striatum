# RFC 0066: Replay, Archive, and Corpus V2 Foundations

## Status
Partially implemented

## Summary
Local replay/archive foundations have landed: corpus verification, run archive
creation/verification, deterministic archive/replay guardrails, deep replay by
default, and read-only archive inspection are present. Corpus Contract V2
identity and privacy metadata now land in new corpus export manifests while V1
bundle verification remains compatible. Incremental watermarks and any
augmentation fetch surface remain follow-up implementation.

## Motivation
The architecture review identified evidence/replay reliability as a product
boundary issue: local-first archives should be inspectable and deterministic
without implying hosted persistence or unapproved memory augmentation.

## Proposed Implementation
Completed work covers local archive and verify commands plus corpus verification
foundations. The V2 manifest slice adds explicit
`corpus_contract_version=2`, composite `corpus_id`, graduated redaction tier,
reference-only workflow-opt-in augmentation policy, `deep_chain` verification
metadata, hybrid archive defaults, optional `git_snapshot_hash`, and verifier
support for both implied-V1 and V2 manifests. The archive follow-up adds
`archive_contract_version=2`, advertised `deep_chain` verification, enforced
hybrid archive defaults, default local semantic replay for archive verify, and
`archive inspect` as a read-only semantic inspection surface. Remaining work is
bounded to watermarking and any future augmentation-reference fetch surface,
which must remain optional and local.
