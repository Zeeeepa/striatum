---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Analysis of TODO 59: Replay, Archive, and Corpus V2

author: analyst-gemini-001
Date: 2026-05-21

## Objective
Prepare decision options for replay/archive/Corpus Contract V2 foundations in TODO 59. This analysis frames the choices for the human principal regarding the long-term technical direction of Striatum's provenance and augmentation surfaces.

---

## 1. Corpus Contract V2 Decisions (RFC 0057)

The V1 contract is single-corpus and lacks explicit versioning. V2 must handle multi-repository environments and explicit identity.

### 1.1 Multi-Corpus Identity (`corpus_id`)
How should a corpus be identified in a multi-repo environment?
- **Option A: Human-Readable Slug.** (e.g., `striatum:my-project`). Pros: Clear for humans. Cons: Risk of collisions; renames break identity.
- **Option B: Structural Hash.** (e.g., SHA-256 of repo identity). Pros: Globally unique; stable across renames. Cons: Opaque to humans.
- **Option C: Composite Identifier.** (e.g., `my-project:sha256`). Pros: Human-readable primary name with a stable hash for deduplication and unique anchoring.

### 1.2 Redaction Tiers and Privacy
What granularity of privacy should the contract support?
- **Option A: Binary (Public/Private).** Simple but coarse.
- **Option B: Graduated Tiers.** (e.g., `tier_1_public`, `tier_2_operator_prose`, `tier_3_restricted`). Allows fine-grained control over what is shared with different memory consumers.
- **Option C: Rule-Based Projection.** Redaction is defined by kind (e.g., always redact `synthesis` but never `decision`).

### 1.3 Context-Injection Policy
How should Striatum handle retrieved context from Engram?
- **Option A: Passive Augmentation.** Striatum only exports the corpus. The agent/MCP client handles retrieval out-of-band. Striatum remains unaware of the injection.
- **Option B: Explicit Workflow Opt-In.** A new `augmentation` block in `workflow.json` defines a retrieval budget and source. The runner prepares a *reference* in the packet, but the agent performs the fetch.
- **Option C: Runner-Side Fetch.** The Striatum runner performs the retrieval and embeds the results in the packet. Pros: Guaranteed context for the agent. Cons: Breaks the augmentation-not-dependency boundary; higher latency at packet-build time.

---

## 2. Archive and Replay Semantics (RFC 0066)

The archive foundation allows runs to be captured for long-term storage or sharing.

### 2.1 Artifact Content Inclusion
Should an archive bundle include the actual artifact bytes?
- **Option A: Metadata-Only.** The archive contains PG rows and manifest hashes. Pros: Small. Cons: Requires access to the original repo or S3 blob store to see content.
- **Option B: Self-Contained Bundle.** The archive includes all artifact bytes (blobs) and transcripts. Pros: Portable; offline-first. Cons: Potentially very large.
- **Option C: Hybrid/Configurable.** The operator chooses which kinds are bundled (e.g., "bundle decisional artifacts but leave syntheses in S3").

### 2.2 Replay/Inspection Semantics
What does "replay" mean for an archived run?
- **Option A: Verification Replay.** Re-verify the event chain, row hashes, and artifact integrity. Current `archive verify --replay` implementation.
- **Option B: Semantic Inspection.** A local-first "read-only" run state that can be viewed in the TUI or Web UI without being registered as an active run.
- **Option C: Comparative Replay.** Re-run a job with the same prompt and compare the new output to the archived output. Highly expensive; non-deterministic for many LLMs.

---

## 3. Verification Scope and Integrity

### 3.1 Row-Hash and Event-Chain Depth
How much evidence is enough for a "verified" archive?
- **Option A: Manifest Verification.** SHA-256 of the JSONL files matches the manifest. Fast.
- **Option B: Deep Chain Verification.** Recompute every row hash and check event-chain continuity (`previous_hash` linking). Detected tampering in historical events.
- **Option C: Audit-Chain Cross-Check.** Cross-verify archived rows against the daemon's immutable `audit_chain_entry` rows. Pros: Defends against database-level surgery. Cons: Requires daemon access at verify time.

---

## 4. Local-Only Boundary Requirements

Striatum's architecture mandates a "local-first, local-only" capability.

- **Offline Verification:** `archive verify` and `corpus verify` must never require an internet connection or external services (Engram, S3) to confirm structural integrity, provided the bytes are present.
- **Boundary Preservation:** Even with optional Engram injection, Striatum must function at 100% capability if Engram is unreachable.
- **Blob Access:** For S3-backed artifacts (RFC 0072), the `archive create` command must define whether it fetches blobs from the network or skips them if they are not already cached locally.

---

## 5. Next Actions for Human Principal
- Decide on the default **Multi-Corpus Identity** format (Option C proposed).
- Select the **Context-Injection** posture (Option B proposed for V2).
- Determine if the default `archive create` should be **Self-Contained** or **Metadata-Only**.
- Approve the **Redaction Tier** vocabulary.
