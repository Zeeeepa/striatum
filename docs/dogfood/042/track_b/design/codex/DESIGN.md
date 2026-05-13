# Track B Design: Engram Phase 1 RFC Body

author: designer-codex-gpt-5.5-001
date: 2026-05-13
status: handoff
target: RFC 0044 V1, Engram Phase 1 read-only MCP over Striatum corpus

## Reading Attestation

This design is based on the required Striatum context docs and the Engram
Markdown corpus under `~/git/engram/`. The Engram pass covered the current
canonical docs (`README.md`, `SPEC.md`, `HUMAN_REQUIREMENTS.md`,
`BUILD_PHASES.md`, `DECISION_LOG.md`, `docs/ingestion.md`,
`docs/segmentation.md`, `docs/claims_beliefs.md`,
`docs/schema/README.md`), Engram design/RFC docs including RFC 0022, and the
historical agent-runner and Striatum fixture material.

## Design Position

RFC 0044 V1 should ship Engram as an optional, local-only, read-only memory
surface for Striatum operators. It should not make Striatum depend on Engram,
and it should not turn Engram into a project-log product. The correct framing
is narrower: Striatum's software-building corpus becomes a new local evidence
source that Engram can retrieve from for the subject's AI-assisted work.

The key product boundary is:

- Striatum remains the workflow runner. Its authoritative live state stays in
  `.striatum/state.sqlite3`, and durable repository artifacts remain
  provenance rather than a live message bus.
- Engram remains the local-first memory system. Its existing flow remains raw
  evidence -> segments -> claims -> beliefs -> `context_for(conversation)`.
- RFC 0044 V1 exposes retrieval over a Striatum corpus snapshot through an
  Engram MCP server. It does not write Striatum state, publish Striatum
  artifacts, mutate Engram claims/beliefs, or merge personal-life and
  software-building corpora by default.

One inconsistency should be resolved in the RFC body: RFC 0041 describes the
read-only Phase 1 RFC as "tentatively RFC 0042", while this dogfood names it
RFC 0044. The RFC should explicitly state that this document is the Phase 1
read-only slice regardless of the final number, and should not drift into the
later write-side dogfood-to-claim phase.

## Problem

Striatum produces exactly the sort of high-provenance operational history that
Engram needs for a first useful serving layer: RFCs, decisions, operator
reports, run summaries, audit metadata, verdicts, artifacts, and commits. The
operator currently pays the cost of rediscovering this history across fresh
sessions. Engram can reduce that cost by retrieving prior Striatum context,
but only if the integration preserves both projects' boundaries.

The risk is overreach. If Phase 1 tries to convert Striatum artifacts directly
into Engram beliefs, wrap Striatum mutation tools, expose personal-life memory
by default, or let a corpus-reading process use network tools, it violates
Engram's current architecture principles and Striatum's local-first control
plane.

## Goals

V1 should:

- Add a Striatum corpus source to Engram as local raw evidence or a read-only
  retrieval adapter, with stable provenance for each observed item.
- Serve retrieval over that Striatum corpus through an Engram-local MCP server.
- Keep the Striatum corpus separate from Engram's personal-life corpus by
  default.
- Require an explicit capability for cross-corpus retrieval.
- Return provenance-bearing results: repository path, artifact path, RFC or
  decision id when present, run/job/artifact ids when present, git commit hash
  when available, and source hash or manifest hash where available.
- Preserve Engram's no-egress property for any corpus-reading process.
- Make Striatum-side use optional and fail-soft.

## Non-Goals

V1 should not:

- Redesign Engram's `claims`, `beliefs`, `segments`, `predicate_vocabulary`,
  bitemporal belief lifecycle, or privacy-tier model.
- Derive Engram beliefs from Striatum artifacts.
- Add write-side ingestion triggered by `run.completed`; that belongs to a
  later phase.
- Add Striatum daemon RPC capabilities for memory access.
- Add Striatum MCP mutation tools.
- Make Engram the source of truth for Striatum decisions, artifacts, audit
  rows, or live state.
- Support hosted, cross-machine, multi-tenant, or cloud retrieval.
- Capture or publish transcripts.
- Let the Engram-reading process make outbound network calls.

## Proposal

### 1. Corpus Ingestion Path

Use operator-triggered pull mode for V1.

The RFC should specify a command or server action in Engram, not Striatum, that
indexes a local Striatum repository path:

```text
engram striatum index --repo /path/to/striatum --corpus-id striatum
```

The exact command name can change, but the ownership should not: Engram reads;
Striatum does not push. Pull mode is safer for Phase 1 because it avoids
adding Striatum event hooks, avoids daemon version coupling, and keeps the
integration an optional memory adjunct. Cron/sweep can be a follow-up once the
manual path is proven. Push from Striatum on `run.completed` belongs to the
write-side phase.

The source adapter should observe two classes of Striatum evidence:

- Durable repository artifacts: `docs/rfcs/`, `docs/DECISION_LOG.md`,
  `docs/dogfood/**/{OPERATOR_REPORT.md,RUN_SUMMARY.md,BUILD_HANDOFF.md,
  EVIDENCE.md,DESIGN*.md,*SYNTHESIS*.md,*REVIEW*.md}`, and committed workflow
  fixtures.
- Read-only runner metadata from `.striatum/state.sqlite3`: runs, jobs,
  sessions, artifacts, verdicts, blockers, and events, with transcript-like
  fields excluded or redacted according to Striatum's evidence policy.

The adapter should record a manifest for each indexing run: repository path,
git HEAD, dirty-tree flag, selected file paths and hashes, SQLite schema
version, selected row counts, and adapter version. This is the reproducibility
boundary for later derivations.

### 2. Engram Data Boundary

The RFC should use Engram's existing raw-evidence vocabulary. A Striatum
artifact or runner row is evidence about the subject's software-building work,
not a belief.

For V1, the safest implementation path is a Striatum-specific raw evidence
source and retrieval index:

- A `sources` row identifies the Striatum repository or snapshot.
- `external_id` is stable: for files, a repo-relative path plus blob hash; for
  runner metadata, a typed id such as `run:<run_id>`, `job:<job_id>`,
  `artifact:<artifact_id>`, `decision:<decision_id>`, or `rfc:<rfc_id>`.
- `raw_payload` preserves original metadata needed to reconstruct the evidence
  reference, subject to redaction rules.
- `observed_at` / `recorded_at` should distinguish when Striatum recorded an
  event from when Engram indexed it.
- `privacy_tier` defaults fail-closed. V1 can default Striatum corpus rows to
  Tier 1 only when the selected artifacts are already commit-safe and
  transcript-free; otherwise the adapter must either skip them or mark them
  above the MCP exposure ceiling.

Engram's current `source_kind` values are `chatgpt`, `claude`, `gemini`,
`obsidian`, `capture`, and `future`. The RFC should choose one of two explicit
paths:

1. Add `source_kind='striatum'` through a small Engram migration.
2. Use `source_kind='future'` only with a documented `raw_payload.kind =
   "striatum"` discriminator and a follow-up migration to promote it.

The first option is cleaner and reviewable. The RFC must not overload
`capture`, because Engram uses `captures` for manual/raw correction events and
privacy reclassification, not for every external corpus source.

### 3. Retrieval Model

V1 retrieval should be boring and inspectable. It can use Postgres FTS and a
small deterministic scorer over indexed Striatum evidence before trying to
reuse Engram's Phase 2 segment/embedding path.

Recommended V1 result scoring:

```text
score =
    lexical_match
  + explicit_id_match
  + recency_weight
  + artifact_kind_weight
  + provenance_strength
  - privacy_penalty
```

This is deliberately less ambitious than Engram's eventual
`context_for(conversation)` lane compiler. It gets the operator a useful MCP
tool without pretending that software-building artifacts have already passed
through Engram segmentation, claim extraction, consolidation, entity
canonicalization, or review.

If the implementation reuses embeddings, it must do so as a derived,
versioned retrieval projection. It must not create `claims` or `beliefs` in
V1, and it must carry `model_version` / `prompt_version` or equivalent
derivation metadata following Engram's model-portability rule.

### 4. MCP Server Topology

Use a standalone Engram MCP server.

The server should live in Engram's repository and follow Engram's planned
serving direction from RFC 0022: shared handler layer, local server binary,
MCP transport, loopback or stdio transport, and no outbound network from the
corpus-reading process. It should not be implemented as Striatum chat tools
in V1. A Striatum wrapper would blur the product boundary and would couple
Engram retrieval to Striatum's daemon RPC transition while RFC 0039 is moving
the daemon core toward Go.

The Striatum side should only document optional operator wiring:

- Add the Engram MCP server to the operator CLI's MCP configuration when
  available.
- Session-start guidance may say "retrieve from Engram if available and
  relevant."
- No workflow should require Engram.
- No Striatum run should fail because Engram is missing, stale, or slow.

### 5. MCP Tool Surface

Keep the V1 MCP surface small and read-only:

```text
engram.search_striatum_corpus(query, filters?, limit?)
engram.read_striatum_reference(reference_id)
engram.striatum_corpus_status()
```

`search_striatum_corpus` returns ranked, redacted snippets with provenance
references. `read_striatum_reference` returns the full allowed text for a
single reference when the caller has the same capability and privacy tier.
`striatum_corpus_status` reports the indexed repo path, HEAD, manifest hash,
row counts, last indexed time, and any skipped paths.

Do not expose:

- `write_claim`
- `derive_belief`
- `ingest_run_completed`
- `striatum.*` mutation wrappers
- personal-life retrieval through the Striatum tool
- raw SQLite query tools

### 6. Capability Vocabulary

Use Engram-local capabilities, not Striatum daemon RPC capabilities.

Striatum's RFC 0030/0032 vocabulary (`read`, `write`, `review`, `claim`,
`apply`, `admin`, `recovery`) is a control-plane authorization set for runner
operations. It is not the right place to authorize access to a personal memory
corpus.

Recommended Engram capabilities:

```text
memory.read_striatum
memory.read_personal
memory.read_cross_corpus
memory.admin_index
```

V1 MCP tools need only `memory.read_striatum` for retrieval and
`memory.admin_index` for manual indexing if indexing is exposed through MCP at
all. `memory.read_cross_corpus` is required for any query that can retrieve
from both Striatum and personal-life corpora in one response. The default MCP
configuration for Striatum operators should grant only `memory.read_striatum`.

### 7. Corpus Separation

Engram's personal-life corpus and Striatum software-building corpus must be
separate by default. The RFC should make this concrete:

- Each indexed evidence row carries `corpus_id`.
- MCP search defaults to `corpus_id='striatum'`.
- Personal-life rows are excluded unless `memory.read_personal` is present.
- Cross-corpus responses are refused unless `memory.read_cross_corpus` is
  present and the query explicitly requests cross-corpus retrieval.
- Search results label corpus on every item.

This preserves Engram's long-arc mission while allowing the Striatum corpus to
be the cleaner validation surface.

### 8. Striatum Bootstrap

The Striatum-side bootstrap should be documentation and optional local config,
not a new hard runtime dependency.

The RFC should specify:

- A human/operator step to start or register the Engram MCP server.
- An optional operator-session instruction: before dogfood work, query Engram
  for prior RFCs, decisions, operator reports, and recurring friction patterns
  related to the active RFC.
- A timeout budget, for example 2 seconds for search and 5 seconds for a
  reference read, after which the operator proceeds without Engram.
- A clear degradation rule: failed Engram retrieval produces a local warning
  only; it never blocks `striatum ack`, `publish-artifact`, `complete`,
  workflow validation, or daemon operations.

No Striatum workflow schema field is required in V1. A future phase can add an
advisory `memory_provider` hint if the operator experience justifies it.

## Acceptance Criteria

RFC 0044 V1 should be accepted only if the implementation satisfies all of the
following.

### A. Local-Only And Optional

1. Engram retrieval runs without outbound network access from any
   corpus-reading process.
2. The MCP server binds only through stdio or loopback.
3. Striatum commands and workflows run unchanged when Engram is absent.
4. Engram retrieval timeouts degrade to "memory unavailable" without failing
   Striatum state transitions.
5. No hosted service, telemetry, remote persistence, or cloud model is added.

### B. Striatum Corpus Indexing

1. An operator can index a local Striatum repository in pull mode.
2. The indexer records a manifest containing repo path, HEAD, dirty flag,
   file hashes, selected SQLite schema version, selected row counts, adapter
   version, and index timestamp.
3. Re-running the indexer with unchanged inputs is idempotent.
4. Re-running against the same external id with changed content records a new
   observed version or fails with a conflict; it must not silently overwrite
   raw evidence.
5. The indexer never writes to `.striatum/` and never calls Striatum mutation
   commands.
6. Transcript-like data is skipped or redacted by default.

### C. Engram Boundary

1. V1 does not insert or update `claims`, `beliefs`, `belief_audit`,
   `contradictions`, `entities`, or `entity_edges` from Striatum data.
2. V1 preserves Engram's raw evidence, provenance, privacy-tier, prompt/model
   versioning, and non-destructive derivation principles.
3. If a new `source_kind='striatum'` is added, it is introduced through an
   explicit migration and documented in schema docs.
4. If `source_kind='future'` is used temporarily, the RFC records why and
   includes a follow-up to promote the source kind.
5. Raw evidence references preserve enough data to reconstruct which Striatum
   file, row, artifact, run, job, verdict, or commit produced the result.

### D. MCP Retrieval

1. `engram.search_striatum_corpus` returns ranked results with snippets,
   corpus labels, privacy tier, source type, reference id, and provenance.
2. `engram.read_striatum_reference` returns a single allowed reference by id
   and refuses inaccessible or skipped references.
3. `engram.striatum_corpus_status` returns index freshness and manifest
   metadata without exposing private raw content.
4. The V1 MCP surface is read-only. No tool mutates Striatum state, Engram
   claims/beliefs, or personal-life corpus rows.
5. Responses make uncertainty explicit: stale index, skipped paths, redacted
   content, and no-result states are visible to the caller.

### E. Capability And Corpus Separation

1. `memory.read_striatum` is sufficient only for Striatum corpus retrieval.
2. `memory.read_personal` is required for personal-life corpus retrieval.
3. `memory.read_cross_corpus` is required for any mixed response.
4. Cross-corpus retrieval is opt-in per request and never the default.
5. Striatum's daemon RPC capability vocabulary is not extended for Engram V1.

### F. Test Seed And Verification

1. A seed corpus covers at least dogfoods 035-040, decisions D080-D092, RFCs
   0030-0041, current RFC 0044 design artifacts, and a bounded recent commit
   window.
2. Tests cover indexing, idempotent re-index, conflict/change detection,
   redaction/skipping, search, reference read, status, capability refusal,
   cross-corpus refusal, and Engram-unavailable fallback.
3. A smoke test demonstrates a query such as "which RFC moved the no-node
   toolchain rule?" returning D092/RFC 0038 provenance without needing a
   transcript.
4. A smoke test demonstrates a query such as "what recurring dogfood harness
   friction led to RFC 0040?" returning RFC 0040, TODO item 20, and relevant
   operator-report/run-summary references where indexed.
5. Documentation states what cannot be claimed: no cross-machine memory sync,
   no hosted retrieval, no multi-tenant access, no personal-life retrieval by
   default, and no authoritative replacement for Striatum's state store.

## Open Questions

1. Should V1 add `source_kind='striatum'`, or use `future` plus a
   discriminator until the raw-evidence source taxonomy is revisited?
2. Does the retrieval index live in existing Engram raw tables plus an FTS
   projection, or in a dedicated Striatum corpus projection table? Either is
   acceptable if raw evidence remains reconstructable and derived indexes are
   rebuildable.
3. Should indexing read `.striatum/state.sqlite3` directly, or consume a
   Striatum evidence export command? Direct read is simpler; evidence export
   is better aligned with Striatum's redaction discipline. V1 can start with
   durable artifacts plus evidence export and defer direct SQLite reads.
4. What exact privacy tier should Striatum artifacts receive by default?
   Commit-safe artifacts can likely be Tier 1; live SQLite rows and
   free-text blocker/verdict fields may need a stricter default.
5. Should manual indexing be a CLI command only, or also an MCP admin tool
   gated by `memory.admin_index`? CLI-only is safer for V1.

## Domain Modeling

The new concept is a boundary adapter, not a new Engram memory primitive.
In DDD terms:

- Engram's existing raw evidence aggregate remains the entry point.
- Striatum corpus items are evidence records with provenance, not beliefs.
- The MCP server is an adapter over Engram retrieval, not a Striatum
  coordinator and not a Striatum daemon method.
- Capabilities are Engram-local value objects because the protected resource
  is memory access, not workflow mutation.

This preserves Engram's ubiquitous language (`source`, `raw evidence`,
`segment`, `claim`, `belief`, `privacy_tier`, `provenance`) while adding only
the minimum vocabulary needed for the software-building corpus:
`striatum corpus`, `striatum reference`, `index manifest`, and
`memory.read_striatum`.
