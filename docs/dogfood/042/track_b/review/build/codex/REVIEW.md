---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "engram", "build", "track_b"]
---

author: reviewer-codex-gpt-5.5-001

# RFC 0044 Build Review

Verdict: accept.

RFC 0044 is implementable as a Phase 1 spec. It preserves the key
augmentation boundary: Striatum remains authoritative for workflow state,
Engram derives optional read-only references, and Striatum must still run when
Engram is missing. The MCP capability vocabulary is also correctly separated:
Engram owns `memory.*` capabilities, while Striatum's daemon registry keeps
the RFC 0030 / RFC 0032 set unchanged.

The RFC is strongest where it turns RFC 0041's design-shape questions into
testable surfaces. The export bundle has a concrete file list, `sub_kind`
vocabulary, stable `external_id` patterns, manifest validation, and transcript
exclusions. The ingestion side names idempotence, conflict behavior,
provenance fields, and the no-claims/no-beliefs V1 boundary. The retrieval
side fixes the four-tool MCP surface and gives enough result fields for a
future implementation to write meaningful smoke tests.

Two low-severity clarity gaps are worth tightening before implementation, but
neither should block acceptance.

First, the retrieval API leaves `filters?`, `reference_id`, and fetch
neighbors under-specified. The four-tool shape is clear, but implementers still
need a closed V1 filter vocabulary such as `sub_kind`, `rfc`, `decision_id`,
`run_id`, `since`, and `until`; a stable `reference_id` format; and a bounded
neighbor contract for `engram.fetch_reference`. Without that, two compliant
implementations could both expose `engram.search` while producing
incompatible client behavior.

Second, `privacy_tier` is required on every search result, but the V1 tier
vocabulary is left as an open question. That is acceptable for a design RFC,
but this is an implementation spec. A minimal closed set, even if conservative
for operator reports and audit-chain text, would make tests and client display
logic deterministic. Similarly, the idempotent export criterion says
`generated_at` is the only allowed timestamp variation, while each JSONL row
also carries `observed_at`. The RFC should state whether `observed_at` is
derived from source metadata and therefore stable, or whether it is omitted
from hash-stable rows when the source has no recorded timestamp.

Required checks:

- V1 acceptance criteria are concrete enough for a future dogfood to implement
  against.
- The augmentation-not-replacement boundary is preserved; Engram's claims,
  beliefs, ingestion, segmentation, and `context_for(conversation)` model are
  reused or left untouched rather than redesigned.
- Striatum-without-Engram fallback is explicit through the Engram-off artifact
  equivalence test, non-zero-tolerant `operator memory check`, no Striatum
  Engram client import, and no daemon RPC Engram methods.
- Capability vocabulary is documented and separated between Engram-local
  `memory.*` and Striatum daemon capabilities.
- Future open questions are clearly named for Phase 1.5 and later work,
  especially segmentation quality, final MCP tool names, privacy tiers, and
  the future write-side phases.
