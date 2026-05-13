author: designer-gemini-pro-001

# Track B Design: Engram Phase 1 (RFC 0044)

This design specifies Phase 1 of the Engram-Striatum integration (RFC 0044). It focuses on providing a read-only memory layer for Striatum operators by exposing a software-building corpus through an Engram MCP server.

## Grounding and Reference

This proposal is grounded in the Engram documentation (README.md, SPEC.md, docs/ingestion.md, docs/segmentation.md, docs/claims_beliefs.md) and RFC 0041. It respects Engram's existing architecture and vocabulary, specifically the lifecycle of **raw evidence**, **segments**, **claims**, and **beliefs**.

## Core Principles

1. **Augmentation, Not Dependency**: Striatum must remain fully functional if Engram is unavailable or slow.
2. **Read-Only V1**: This phase implements retrieval only. No write-side claims or beliefs are derived from Striatum artifacts in V1.
3. **Corpus Isolation**: The `striatum` (software-building) corpus is strictly separated from the `personal-life` corpus.
4. **Local-First**: All operations remain local to the user's machine, following the D083 single-user/single-machine posture.

## 1. Ingestion Path: Operator-Triggered Pull Mode

To maintain a clean boundary and avoid coupling Striatum's run lifecycle to Engram availability in V1, ingestion is implemented as a **pull-mode indexer** owned by Engram.

- **Indexer**: A new Engram-side CLI verb or internal process (e.g., `engram index-striatum --repo <path>`) scans a registered Striatum repository.
- **Evidence Sources**:
    - **Markdown Artifacts**: RFCs, decision logs, operator reports, run summaries, and findings.
    - **Runner Metadata**: Snapshots of the `.striatum/state.sqlite3` database (runs, jobs, verdicts, audit rows), subject to Striatum's redaction policy (no transcripts).
- **Redaction**: The indexer must respect Striatum's `evidence export` redaction rules to ensure no private prose or transcripts leak into the memory layer.
- **Idempotency**: Ingestion is based on content hashes (blob SHA-256) and stable external IDs (e.g., `run:<id>`, `rfc:<num>`).

## 2. Cross-Corpus Separation

Engram's storage substrate is augmented to support multiple corpuses.

- **Corpus ID**: Every piece of **raw evidence** and every derived **segment** carries a `corpus_id` (e.g., `striatum` or `personal-life`).
- **Isolation by Default**: Retrieval tools default to the `striatum` corpus for Striatum operators.
- **Explicit Opt-In**: Cross-corpus queries (retrieving from both personal and software corpuses) are refused unless explicitly requested by the operator and authorized by a high-tier capability.

## 3. Scoped Capability Tokens

Access to Engram's retrieval surface is gated by **Engram-local capability tokens**, decoupled from Striatum's daemon RPC capabilities.

- `memory.read_striatum`: Allows retrieval from the `striatum` software-building corpus only.
- `memory.read_personal`: Allows retrieval from the original `personal-life` corpus.
- `memory.read_cross_corpus`: Allows mixed queries across multiple corpuses.
- `memory.admin`: Allows triggering re-indexing and manifest management.

For V1, the Striatum operator's MCP configuration is granted only `memory.read_striatum`.

## 4. Engram MCP Server Topology

A standalone Engram MCP server (stdio or loopback) is implemented within the Engram repository (`~/git/engram/agent-runner/` or similar).

- **Tools**:
    - `engram.search(query, corpus="striatum", filters?)`: Returns ranked snippets of raw evidence or segments with provenance (commit hash, file path, decision ID).
    - `engram.get_reference(reference_id)`: Retrieves the full allowed content of a specific evidence item.
- **Separation**: By running as a standalone server, Engram preserves its own security boundary and avoids bloating the Striatum daemon.

## 5. Striatum-Side Wiring and Fallback

Striatum's integration is advisory and fail-soft.

- **Bootstrap**: The operator adds the Engram MCP server to their CLI config (e.g., `~/.claude/settings.json` or `striatum` config).
- **Usage**: When a session starts, the operator is prompted (via workflow instructions or lane prompts) to retrieve relevant context if Engram is available.
- **Graceful Degradation**:
    - If the MCP server is not registered, the operator proceeds with standard session-start context.
    - If a retrieval call times out (e.g., >2 seconds), the operator proceeds without the memory layer.
    - Errors in Engram MUST NOT block Striatum transitions (`ack`, `complete`, `verdict`).

## 6. Vocabulary Alignment

This design preserves Engram's ubiquitous language:
- **Raw Evidence**: The base layer of Striatum artifacts and rows.
- **Segments**: The indexed/processed units of evidence used for retrieval.
- **Provenance**: Mandatory metadata (git commit, artifact hash, run ID) attached to every result.
- **Privacy Tiers**: Each corpus and evidence item respects Engram's existing privacy tiering model.

## Acceptance Criteria (RFC 0044 V1)

1. **Pull-Mode Indexer**: Engram can index a local Striatum repository path and record a manifest of observed artifacts.
2. **Corpus Separation**: Results from `striatum` and `personal-life` corpuses are never mixed unless `memory.read_cross_corpus` is present.
3. **Read-Only Retrieval**: The MCP server provides `search` and `get_reference` tools; no mutation of Engram or Striatum state is exposed.
4. **Fallback Integrity**: `striatum` commands remain fully functional without a running or reachable Engram MCP server.
5. **No Network Egress**: The Engram indexing and retrieval process makes no outbound network calls, preserving the local-first security posture.
6. **Provenance**: Every search result includes a stable reference to its source (e.g., file path + commit hash).

## Non-Goals

- Writing Engram **claims** or **beliefs** from Striatum data (Phase 3).
- Automatic "push" ingestion on `run.completed`.
- Merging the Striatum and Engram databases.
- Hosted or multi-tenant retrieval.
