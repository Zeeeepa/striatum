---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0044", "v1", "build"]
---

author: reviewer-unknown-model-001

# Build Review: Redaction Completeness And JSONL Secret Leakage

Verdict: needs_revision

RFC 0044 establishes the right high-level boundary: Striatum corpus export is read-only, curated, local, and must not include transcripts, terminal output, raw model output, SQLite blobs, or ambiguous free-text live-state fields. That is not yet enough to accept the redaction threat model because the RFC still leaves several JSONL-bearing paths underspecified.

## Trust Boundaries And Attack Surfaces

The proposed flow crosses these trust boundaries:

- Repository files and git history cross into `striatum corpus export` as JSONL `content`.
- Local runner state crosses into exported run summaries and audit-chain entries.
- Export bundles cross from Striatum into Engram ingestion.
- Engram retrieval crosses from stored corpus rows into MCP tool responses.
- Manifest metadata crosses from local filesystem context into a durable bundle.

The attack surfaces are the per-line JSONL `content` fields, commit messages, operator-report text, run-summary text, audit-chain text, provenance metadata, absolute repository paths, dirty-tree state, manifest row counts and hashes, bundle tampering before ingestion, and later MCP `engram.search` / `engram.fetch_reference` responses that return content back to an operator session.

## Findings

Finding 1: RFC 0044 does not specify a concrete redaction contract for JSONL `content`.

RFC 0044 says the export never includes transcripts, terminal output, raw model output, SQLite blobs, or ambiguous free-text live-state fields, and calls the bundle "redacted-by-construction." It does not define how exporters identify secrets already present in allowed sources such as RFC bodies, operator reports, changelog entries, ubiquitous-language terms, harness-friction rows, or commit messages. Because the bundle intentionally exports free-text `content`, any accidentally committed `.env` value, token-shaped string, private URL, local host, branch name containing private context, or pasted terminal snippet inside an otherwise allowed document can become durable JSONL and then retrievable through Engram.

Required mitigation: define a V1 redaction policy before acceptance. At minimum, the export contract should specify excluded source paths, secret-pattern scanning or structured allowlists per `sub_kind`, handling for commit messages containing secrets-shaped tokens, and fail-closed behavior when redaction is uncertain. The acceptance tests should assert that `.env` content, transcript text, terminal output, raw model output, and `.striatum/state.sqlite3`-derived blobs are absent from every JSONL file and from MCP retrieval output after ingestion.

Finding 2: provenance and manifest metadata may leak local private context.

The RFC requires `manifest.json` to carry repository path, git HEAD, dirty-tree flag, `since` ref, schema version, row counts, hashes, and generated time. It also requires ingested provenance to preserve repository path, source file path, blob SHA-256, run/RFC/decision/audit identifiers, and timestamps. The example uses a repository-relative path in per-record provenance, but the manifest says "repository path" without constraining it to a privacy-safe value. An absolute path can leak usernames, directory names, client names, private project names, or local machine layout into a durable bundle and then into Engram metadata.

Required mitigation: require repository-relative paths in JSONL records and define whether manifest repository identity is a sanitized repo slug, remote-free canonical ID, or explicitly absolute local path. If absolute paths are necessary for local-only operation, mark them non-retrievable by default and exclude them from MCP search/fetch responses unless a diagnostic flag is explicitly enabled.

Finding 3: tamper detection is described for file hashes and counts, but not for canonical JSONL serialization or redaction ordering.

RFC 0044 requires per-file SHA-256 values, selected row counts, and refusal of partial bundles when counts or hashes do not match. It also requires replay-stable JSONL content. That acknowledges tampering, but the spec does not say whether hashes apply before or after redaction, whether JSONL records have canonical key ordering and newline normalization, or whether the ingester validates that each record's `content` still satisfies the redaction policy. Without that, a bundle can be stable and hash-verified while still containing secrets, and redaction can become a producer-only convention rather than an end-to-end invariant.

Required mitigation: specify canonical serialization, UTF-8 handling, hash coverage after redaction, and ingest-side validation that rejects records violating the redaction contract. Engram ingestion should treat redaction failure as a hard validation error, not as best-effort filtering.

Finding 4: MCP retrieval returns raw `content` without a stated output redaction gate.

RFC 0044 says `engram.search` returns `content`, and `engram.fetch_reference` returns the referenced record and optional neighbors. If any sensitive text reaches Engram, the MCP server becomes a redisclosure path. Capability checks separate `striatum` from `personal`, but they do not mitigate secrets inside the Striatum corpus itself.

Required mitigation: require MCP output to use the same redaction classification as ingestion. Search snippets and fetch results should be generated only from redaction-approved fields, with tests proving that disallowed patterns remain absent after export, ingestion, search, and fetch.

## Required Checks Not Performed

This review was constrained to the packet-provided documents. I did not inspect implementation files, tests, generated bundles, or handoff artifacts. Therefore I could not verify the required build-review checks for a real `striatum corpus export` invocation, replay-stability, `rg -n "engram" src/striatum/`, daemon RPC capabilities, redaction tests, or `make test`.

Those checks remain mandatory before an implementation acceptance verdict. Based on the RFC text alone, the redaction threat model needs revision before the JSONL bundle and MCP retrieval surface are safe to accept.
