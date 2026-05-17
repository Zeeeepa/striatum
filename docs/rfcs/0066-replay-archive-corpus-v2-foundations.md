# RFC 0066: Replay, Archive, and Corpus V2 Foundations

## Status
Partially implemented

## Summary
Local replay/archive foundations have landed: corpus verification, run archive
creation/verification, and deterministic archive/replay guardrails are present.
Corpus Contract V2 remains a separate product decision because identity,
redaction tier, watermark, and context-injection semantics are not yet locked.

## Motivation
The architecture review identified evidence/replay reliability as a product
boundary issue: local-first archives should be inspectable and deterministic
without implying hosted persistence or unapproved memory augmentation.

## Proposed Implementation
Completed work covers local archive and verify commands plus corpus verification
foundations. Remaining work belongs to the Corpus Contract V2 decision track:
multi-corpus identity, redaction tiers, watermarking, and whether any future
context injection remains strictly optional and local.
