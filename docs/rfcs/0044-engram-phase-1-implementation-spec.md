# RFC 0044: Engram Phase 1 Implementation Spec

Status: proposed
Date: 2026-05-13
author: implementer-codex-gpt-5.5-001
Context:
[`RFC 0041`](0041-engram-memory-layer-for-striatum-operators.md),
[`RFC 0036`](0036-mcp-harness-for-daemon-v2-mutation-surface.md),
[`RFC 0030`](0030-daemon-rpc-server-and-version-skew-protocol.md),
[`RFC 0033`](0033-storage-substrate-rewrite-for-daemon-v2.md),
[`RFC 0035`](0035-multi-repo-test-harness-for-cross-repo-workflows.md),
`~/git/engram/` (authoritative external dependency for Engram vocabulary,
schema, ingestion, segmentation, claims, beliefs, and serving boundaries).

RFC 0041's follow-up-numbering section tentatively assigned RFC 0044 to a
later write-side phase. Dogfood 042 reassigns this number to the Phase 1
implementation spec. This RFC is Phase 1 only: read-only Engram retrieval over
the first Striatum corpus. It also establishes the local isolation vocabulary
Engram needs for later local application-memory systems on the same machine.
It does not include write-side dogfood-to-claim flow.

## Problem

RFC 0041 names the operating problem: Striatum already produces a rich,
repository-grounded operational corpus, but operator sessions rediscover that
history instead of retrieving it as memory. RFCs, decision log rows, operator
reports, run summaries, audit-chain metadata, harness friction notes, and
commits answer recurring questions such as which dogfoods touched daemon RPC,
which decision superseded a prior rule, and which friction patterns repeated
across recent runs. Fresh sessions still pay the re-reading cost.

Engram has the complementary problem. Its original personal-life memory mission
is valuable but harder to ground because many personal-life beliefs have
squishy truth conditions. The Striatum corpus is the first easier validation
surface: the relevant evidence is already in git, artifact hashes, decision
rows, and audit transitions. Engram can validate a serving layer over this
corpus without redirecting its long-term personal-life mission or closing the
door on later local memory systems for other applications.

The Phase 1 problem is therefore narrow: add a local, optional, read-only
Engram memory layer for Striatum operators, while preserving Striatum's
authoritative state, Engram's existing architecture, and the operator's root
authority over the machine. Striatum's daemon-owned PostgreSQL remains live
workflow state; `.striatum/retired-local-state` is pre-D094 migration
input/tombstone or fixture material only. Repository artifacts remain durable
provenance.
Engram derives retrievable references from those sources; it does not become
the message bus, source of truth, hosted service, or critical path.

## Goals

- Provide read-only retrieval over a separated Striatum application memory.
  Acceptance: `engram-mcp-stdio` exposes a small MCP stdio surface whose
  default Striatum operator token can search and fetch only
  `tenant_id='striatum'` and allowed `corpus_id` rows.
- Preserve Engram's personal-life mission and default isolation. Acceptance:
  existing rows backfill to `tenant_id='personal'` and
  `corpus_id='personal'`; personal-life retrieval is unchanged after
  migration; default Striatum operator tokens do not carry
  `memory.read_personal`, `memory.read_cross_tenant`, or
  `memory.read_cross_corpus`.
- Add a local, provenance-preserving ingestion path. Acceptance:
  `striatum corpus export --since <ref> [--out <dir>]` emits a verifying JSONL
  bundle, and `engram ingest-striatum --bundle <dir> [--repo <name>]`
  validates and idempotently ingests that bundle.
- Keep the Phase 1 schema usable beyond Striatum. Acceptance: RFC 0044 names
  `tenant_id` as the local application-memory boundary and `corpus_id` as the
  workload/dataset boundary inside a tenant; Striatum is the first tenant and
  corpus, not the only future one.
- Keep Striatum independent of Engram at runtime. Acceptance: no Striatum state
  transition (`ack`, `publish-artifact`, `complete`, `verdict`, recovery, run
  prepare/start) imports, waits on, or fails because of Engram.
- Keep capability vocabularies separate. Acceptance: Engram defines
  `memory.*` capabilities locally; RFC 0030's Striatum daemon RPC capabilities
  remain unchanged.
- Make retrieval quality testable. Acceptance: the seed corpus and smoke query
  set in this RFC pass before V1 is accepted, with positive provenance hits and
  negative out-of-corpus misses.

## Non-Goals

- Hosted retrieval, remote service mode, telemetry, cloud sync, remote
  persistence, or cross-machine sync. RFC 0044 allows local tenant isolation
  only: multiple application-memory systems under the same machine owner/root
  authority, not hosted or cloud multi-tenancy.
- Transcript capture, model-output ingestion, terminal scraping, or broad log
  ingestion. The Striatum corpus is structural and curated.
- Replacing daemon-owned PostgreSQL, migration-only `.striatum/retired-local-state`
  sources, the decision log, RFCs, operator reports, run summaries, audit
  chain, or git history as authority.
- Adding a Striatum `workflow.json` memory field, daemon RPC method, chat tool,
  Striatum-side `memory.*` capability, or Engram client import.
- Write-side dogfood ingestion. V1 does not emit `run.completed` to Engram and
  does not create Engram claims or beliefs from Striatum artifacts.
- Personal-life retrieval by default. Access to `corpus_id='personal'` requires
  explicit Engram-local capability; access to `tenant_id='personal'` is
  isolated by default.
- Raw SQL exposure, Striatum mutation passthrough, `memory.write`, or
  `memory.admin_index` through MCP.
- Redesigning Engram's raw evidence, segmentation, claims, beliefs,
  `predicate_vocabulary`, or `context_for(conversation)` model.

## Proposal

### 1. Phase 1 Data Flow

Phase 1 is operator-triggered pull:

```text
Striatum repository artifacts and summaries
  -> striatum corpus export --since <ref> [--out <dir>]
  -> JSONL bundle with manifest
  -> engram ingest-striatum --bundle <dir> [--repo <name>]
  -> Engram rows under source_kind='striatum',
     tenant_id='striatum', and corpus_id='striatum'
  -> engram-mcp-stdio read-only MCP tools
  -> operator session retrieval calls
```

Striatum does not call Engram when a run completes. Engram owns the ingester.
Striatum owns the export format and redaction discipline.

### 2. Engram Local Isolation Model

Engram's current docs define raw evidence as `sources`, `conversations`,
`messages`, `notes`, and `captures`; derived layers include `segments`,
`segment_embeddings`, `claims`, `claim_extractions`, `beliefs`,
`belief_audit`, and related review/eval tables. A `capture` is one raw
evidence table, not a generic external-corpus slot.

V1 adds three fields needed for Striatum and for later local memory systems:

- `source_kind='striatum'`, added by the next Engram numbered migration,
  following the existing `source_kind` enum precedent.
- `tenant_id`, propagated through `sources` and derived rows needed for
  retrieval. It is the local application-memory boundary. In Phase 1,
  existing personal rows backfill to `tenant_id='personal'`; Striatum rows
  ingest with `tenant_id='striatum'`.
- `corpus_id`, propagated through `sources` and derived rows needed for
  retrieval. It is the workload or dataset boundary inside a tenant. Existing
  personal rows backfill to `corpus_id='personal'`; the first Striatum bundle
  ingests with `corpus_id='striatum'`.

The Striatum corpus must not be hidden inside `source_kind='capture'` or
`source_kind='future'`. `source_kind` remains the ingest/parser discriminator;
`tenant_id` is the app-memory boundary; `corpus_id` is the corpus boundary.

This is not hosted multi-tenancy. All tenants in RFC 0044 are local namespaces
under the same machine owner/root authority. The purpose is blast-radius
control between application memories on one local Engram instance: a Striatum
operator token must not read personal memory or another future application's
memory unless Engram grants an explicit local capability for that boundary.

### 3. Striatum Export Bundle

Striatum provides a read-only corpus exporter and a CLI verb:

```text
striatum corpus export --since <ref> [--out <dir>]
```

The command emits a directory bundle:

```text
manifest.json
rfcs.jsonl
decision_log_rows.jsonl
operator_reports.jsonl
run_summaries.jsonl
audit_chain.jsonl
changelog.jsonl
ubiquitous_language.jsonl
harness_friction_patterns.jsonl
commits.jsonl
```

Each JSONL line has this shape:

```json
{
  "source_kind": "striatum",
  "external_id": "rfc:0040#proposal",
  "sub_kind": "rfc",
  "content": "...",
  "provenance": {
    "path": "docs/rfcs/0040-mcp-driven-dogfood-harness.md",
    "sha256": "...",
    "commit": "..."
  },
  "observed_at": "2026-05-13T00:00:00Z"
}
```

The Striatum bundle does not need to carry Engram-local `tenant_id` or
`corpus_id` fields. Engram assigns those values during ingestion so the
already-shipped Striatum export remains stable.

`sub_kind` is a closed V1 set:

- `rfc`
- `decision_log_row`
- `operator_report`
- `run_summary`
- `audit_chain_entry`
- `changelog_entry`
- `ubiquitous_language_term`
- `harness_friction_pattern`
- `commit`

`external_id` is content-stable:

| `sub_kind` | `external_id` |
|---|---|
| `commit` | `commit:<sha>` |
| `decision_log_row` | `decision:<D###>` |
| `rfc` | `rfc:<####>#<heading-slug>` |
| `operator_report` | `dogfood:<###>#intervention-<N>` |
| `audit_chain_entry` | `audit:<row-id>` |
| `run_summary` | `run:<run-id>` |
| `changelog_entry` | `changelog:<vX.Y.Z>` |
| `ubiquitous_language_term` | `ulang:<slug>` |
| `harness_friction_pattern` | `friction:<slug>` |

`manifest.json` carries `striatum_version`, repository path, git HEAD,
dirty-tree flag, `since` ref, per-file SHA-256, explicit `state_authority`
metadata for the daemon/PostgreSQL authority, selected row counts, emitted
`source_kind` values, `generated_at`, and `bundle_sha256`. The bundle digest is
derived from the canonical manifest after excluding `bundle_sha256` and
`generated_at`, so the timestamp can vary without changing the content hash.
Engram refuses partial bundles when manifest row counts or file hashes do not
match the emitted JSONL files.

The export never includes transcripts, terminal output, raw model output,
SQLite blobs, or ambiguous free-text live-state fields. Run summaries are
sourced through the existing `striatum run summary --json` interface rather
than ad hoc free-text SQLite reads.

### 4. Engram Ingestion

Engram adds:

```text
engram ingest-striatum --bundle <dir> [--repo <name>]
```

The command reads the Striatum export bundle, validates the manifest, and
writes one `sources` row for the bundle plus downstream rows for JSONL records
under `tenant_id='striatum'` and `corpus_id='striatum'`. The source key is
`(tenant_id, corpus_id, source_kind, external_id)`, where the bundle-level
`external_id` includes repository identity, `since`, and manifest hash.
`--repo <name>` is a human repository label such as `striatum`; it is not a
request for Engram to call Striatum. Re-ingesting the same canonical bundle
hash is a no-op. A same-key collision with different content raises Engram's
existing `IngestConflict`.

Every ingested row preserves provenance: repository path, git HEAD, source
file path, blob SHA-256, `sub_kind`, applicable `decision_id`, `rfc`, `run_id`,
`audit_id`, and separate `observed_at` / `recorded_at` timestamps where the
source supplies both.

The ingester may reuse Engram's existing segmentation and embedding pipeline.
Raw messages are not embedded directly in Engram's model; segments remain the
embedding and claim-extraction unit. In Phase 1, Striatum rows are indexed for
retrieval only. No claims or beliefs are created from the Striatum corpus.

Later local application-memory ingesters may use other `tenant_id` and
`corpus_id` values, but the Phase 1 Striatum ingester must not accept an
operator-supplied tenant override. Striatum's first validation corpus is fixed
to `tenant_id='striatum'` and `corpus_id='striatum'` unless a later Engram RFC
extends the mapping deliberately.

### 5. Engram MCP Stdio Server

Engram ships a standalone console script:

```text
engram-mcp-stdio
```

The server speaks MCP stdio against Engram's handler layer and uses read-only
access to the local Engram PostgreSQL database. It is not a Striatum chat tool
and does not route through Striatum daemon RPC.

V1 exposes four read-only tools:

```text
engram.search(query, tenant="striatum", corpus="striatum", filters?, k=10)
engram.fetch_reference(reference_id)
engram.describe_corpus(tenant="striatum", corpus="striatum")
engram.health()
```

`engram.search` returns ranked references with `reference_id`, `tenant_id`,
`corpus_id`, `source_kind`, `sub_kind`, `external_id`, `content`, `score`,
`privacy_tier`, and reconstructable provenance. `engram.fetch_reference`
returns the referenced record and optional neighbors only if the caller has
read capability for the referenced row's tenant and corpus.
`engram.describe_corpus` reports counts, time bounds, schema/adapter versions,
and last ingest metadata for the requested local tenant/corpus.
`engram.health` reports substrate readiness and the set of visible local
tenant/corpus pairs, not hidden corpus names.

The server does not expose raw SQL, write tools, claim creation, belief
revision, indexing/admin operations, or Striatum mutation passthrough.

### 6. Capability Boundary

Engram defines its own memory capabilities:

| Capability | Default Striatum operator token | Required for |
|---|---:|---|
| `memory.read_striatum` | yes | Read `tenant_id='striatum'` and allowed Striatum `corpus_id` values |
| `memory.describe` | yes | `engram.describe_corpus`, `engram.health` |
| `memory.read_personal` | no | Read `tenant_id='personal'` |
| `memory.read_cross_tenant` | no | Read more than one `tenant_id` in one request or token |
| `memory.read_cross_corpus` | no | Mixed-corpus retrieval inside an allowed tenant |

These are not Striatum daemon capabilities. RFC 0030's Striatum capability
vocabulary remains `read`, `write`, `review`, `claim`, `apply`, `admin`, and
`recovery`. No `memory.*` capability is added to the Striatum daemon method
registry.

Default Striatum operator access is read-only and scoped to Striatum's allowed
local tenant/corpus pair. Cross-tenant or cross-corpus retrieval requires an
explicit Engram-local capability and explicit requested tenant/corpus names.
Existing personal memory stays isolated by default, including from reference
fetches where the `reference_id` was discovered outside the current token's
visible scope.

### 7. Striatum Operator Wiring

Striatum Phase 1 requires only the shipped export verb:

- `striatum corpus export --since <ref> [--out <dir>]`

Additional operator-facing helpers are optional follow-ups, not dependencies.
If Striatum later adds `striatum operator memory check`, it must shell out to
`engram-mcp-stdio --health-check` or the equivalent `engram.health` call,
print status, and exit `0` even when Engram is missing, misconfigured, or
unreachable. If Striatum later adds a `striatum-engram` skill, the skill must
only explain when to query Engram, the four MCP tools, the Engram-local
capability names, and the augmentation-not-dependency rule.

MCP client configuration is documented, not automated. Operators manually add
the Engram MCP server to their agent CLI configuration, for example:

```json
{
  "mcp_servers": {
    "engram": {
      "command": "engram-mcp-stdio",
      "args": ["--tenant", "striatum", "--corpus", "striatum"]
    }
  }
}
```

### 8. Augmentation-Not-Dependency Enforcement

Phase 1 is accepted only if all of the following are true:

- No Striatum CLI module imports an Engram client library.
- No Striatum daemon RPC method references Engram.
- No `memory.*` capability appears in Striatum's daemon method registry.
- Engram retrieval calls have short operator-session budgets: 2 seconds for
  search and 5 seconds for fetch.
- Engram unavailability degrades to the pre-Engram operator path: read the
  repository docs and explicit work packet context directly.
- A dogfood-shaped "Engram off" test proves Striatum produces the same required
  artifacts with `engram-mcp-stdio` absent from `PATH` and Engram PostgreSQL
  unreachable.

## Acceptance Criteria

### Engram Ingestion

- A numbered Engram migration adds `source_kind='striatum'`, `tenant_id`, and
  `corpus_id`; backfills existing rows to `tenant_id='personal'` and
  `corpus_id='personal'`; and preserves retrieval behavior for the
  personal-life corpus.
- `engram ingest-striatum --bundle <dir> [--repo <name>]` validates the
  Striatum bundle manifest, verifies per-file counts and hashes, ingests rows
  under `tenant_id='striatum'` and `corpus_id='striatum'`, and is idempotent
  for unchanged canonical bundle hashes.
- Conflicting content for the same
  `(tenant_id, corpus_id, source_kind, external_id)` raises `IngestConflict`.
- Ingested rows preserve file, hash, commit, run, RFC, decision, and audit
  provenance where applicable.
- The ingester never calls Striatum mutation commands and never writes into a
  target repository's `.striatum/` directory.
- The ingester rejects any operator-supplied tenant override for the Striatum
  bundle in Phase 1.

### Engram Retrieval

- `engram-mcp-stdio` ships as a console-script entry point.
- The only V1 MCP tools are `engram.search`, `engram.fetch_reference`,
  `engram.describe_corpus`, and `engram.health`.
- Default Striatum operator tokens carry only `memory.read_striatum` scoped to
  `tenant_id='striatum'` and allowed Striatum `corpus_id` values, plus
  `memory.describe`.
- Personal-life retrieval is refused without `memory.read_personal`, including
  both search and direct `fetch_reference` attempts.
- Cross-tenant retrieval is refused without `memory.read_cross_tenant` and
  explicit tenant names.
- Mixed-corpus retrieval is refused without `memory.read_cross_corpus` and
  explicit corpus names.
- Every search result carries `tenant_id`, `corpus_id`, `source_kind`,
  `sub_kind`, `privacy_tier`, `external_id`, and provenance.
- The corpus-reading process has no outbound network egress.

### Boundary Tests

- A default Striatum operator token can search
  `tenant_id='striatum', corpus_id='striatum'`.
- The same token is refused for `tenant_id='personal'`, for an unknown future
  application tenant, and for any second corpus inside `tenant_id='striatum'`
  that is not explicitly allowed.
- `engram.fetch_reference` rechecks the referenced row's `tenant_id` and
  `corpus_id`; it does not trust opaque reference IDs as authorization.
- A token with `memory.read_cross_corpus` but not `memory.read_cross_tenant`
  can read multiple allowed corpora inside one tenant but cannot read another
  tenant.
- A token with `memory.describe` sees only visible tenant/corpus metadata by
  default; hidden personal memory does not leak through corpus counts or names.

### Striatum Export And Wiring

- `striatum corpus export --since <ref> [--out <dir>]` writes the current
  Striatum bundle: nine JSONL files plus a verifying `manifest.json`.
- Re-running the export with unchanged inputs produces identical JSONL file
  content and stable hashes; `generated_at` is the only allowed timestamp
  variation.
- Run-summary rows are produced through `striatum run summary --json`, not a
  new ad hoc database read path.
- Striatum-side export remains augmentation-only: Striatum does not import
  Engram, call Engram, or acquire `memory.*` capabilities.
- Any future Striatum-side operator wiring, such as an informational health
  check or skill text, must exit successfully when Engram is unavailable and
  must remain outside Striatum's workflow critical path.

### Augmentation Boundary

- `rg -n "engram" src/striatum/cli` shows no Engram client imports; allowed
  matches are help text, subprocess command strings, or documentation-facing
  literals for `operator memory check`.
- Striatum's daemon RPC registry has no `memory.*` capability or Engram method.
- A dogfood-shaped run with Engram unavailable produces the same required
  Striatum artifacts as the same run with Engram available.
- The RFC 0035 multi-repo harness gains an optional Engram fixture gated by
  `ENGRAM_TEST=1`; normal CI stays green when the variable is unset.

### Smoke Retrieval

Seed corpus:

- dogfoods 035-040
- decisions D080-D092
- RFCs 0030-0041 plus this RFC after it lands
- active `CHANGELOG.md` entries
- `docs/UBIQUITOUS_LANGUAGE.md` terms
- `docs/HARNESS_FRICTION_PATTERNS.md` rows
- run summaries and audit-chain rows for landed dogfood runs
- the most recent approximately 50 commits

Positive smoke requires 5/5 provenance-correct top-5 hits:

- "what friction patterns recurred across dogfoods 036-039"
- "which RFC moved the no-node toolchain rule and why"
- "has surgical_recovery been invoked before"
- "what did the build review for dogfood-037 say about test coverage"
- "which dogfoods touched the daemon RPC capability vocabulary"

Negative smoke requires 5/5 below the documented score floor or empty top-5:

- "what is Jennifer's MBTI"
- "best pizza in Berlin"
- "Python f-string formatter rules"
- "tomorrow's weather"
- "the JFK files"

## Implementation Plan

### Step 1: Engram Corpus And Ingester

- Add the next Engram migration for `source_kind='striatum'`, `tenant_id`,
  and `corpus_id`.
- Backfill existing rows to `tenant_id='personal'` and
  `corpus_id='personal'`.
- Implement `engram ingest-striatum --bundle <dir> [--repo <name>]`.
- Document Striatum as the first local application-memory tenant/corpus in
  Engram docs.

### Step 2: Engram MCP Server

- Add `engram-mcp-stdio`.
- Implement the four read-only tools.
- Implement Engram-local tenant/corpus capability checks.
- Add Engram setup and smoke-test docs for MCP stdio.

### Step 3: Striatum Export Contract

Striatum-side V1 export is already shipped. RFC 0044 now treats the bundle as
the contract Engram must consume, not as new Striatum implementation work.
Future Striatum changes to the bundle require a versioned manifest change and
matching Engram-side compatibility decision.

### Step 4: Optional Striatum Operator Wiring

No additional Striatum wiring is required for the Engram Phase 1 ingester and
MCP server. If later queued, an informational health check or skill template
must remain optional and harmless when Engram is missing.

### Step 5: Integration Harness

- Add an optional RFC 0035-style Engram fixture gated by `ENGRAM_TEST=1`.
- Add the positive and negative smoke retrieval set.
- Add the "Engram off" artifact-equivalence test.

Steps 1 and 2 land in the Engram repository and are queued outside Striatum.
Step 3 is already complete on the Striatum side. Steps 4 and 5 are optional
follow-ups and must not make Striatum depend on Engram.

## Open Questions

- What is the exact Engram migration filename? Recommendation: use the next
  available prefix after the current Engram migrations, e.g.
  `013_source_kind_striatum_tenant_corpus.sql` if no newer migration exists
  when implementation starts.
- What is the Engram schema shape for propagating `tenant_id` and `corpus_id`
  through existing derived tables without broad refactors? Recommendation:
  touch only the rows needed for Phase 1 retrieval, then widen in later
  Engram phases.
- Which `segmenter_prompt_version` and extraction version strings are recorded
  for Striatum corpus derivations? Recommendation: reuse active Engram versions
  unless smoke quality forces a Phase 1.5 structural segmenter.
- What privacy tier applies by default per Striatum row class? Recommendation:
  Tier 1 for commit-safe public-repo-style rows; stricter effective treatment
  for operator-report and audit-chain free text until reclassification rules
  are explicit.
- Do the final MCP tool names remain `engram.search`,
  `engram.fetch_reference`, `engram.describe_corpus`, and `engram.health`?
  The names may align with Engram's handler conventions, but the four-tool
  read-only shape is fixed.
- Does default Engram segmentation meet the 5/5 positive and 5/5 negative
  smoke bar? If not, Phase 1.5 lands a Striatum-aware segmenter before V1 is
  accepted.

## Domain Modeling

RFC 0044 is a boundary clarification plus a new optional adapter surface.

Striatum-side terms:

| Term | Definition |
|---|---|
| memory augmentation | Optional retrieval layer an operator may query for repository-grounded context. It never owns workflow state. |
| Striatum corpus | Closed set of Striatum software-building artifacts exported for Engram: RFCs, decision-log rows, operator reports, run summaries, audit-chain entries, changelog entries, ubiquitous-language terms, harness-friction patterns, and commits. |
| Striatum corpus export | Redacted-by-construction JSONL bundle produced by `striatum corpus export --since <ref>`. |
| augmentation-not-dependency | Invariant that Striatum runs unchanged when Engram is missing, slow, or unavailable. |

Engram-side terms reused without redesign:

| Term | RFC 0044 usage |
|---|---|
| raw evidence | Engram source-of-truth layer: `sources`, `conversations`, `messages`, `notes`, and `captures`. |
| `source_kind` | Ingest/parser discriminator. RFC 0044 adds `source_kind='striatum'`. |
| `tenant_id` | Local application-memory boundary under the same machine owner/root authority. It separates `personal`, `striatum`, and future local application memories without implying hosted/cloud multi-tenancy. |
| `corpus_id` | Workload or dataset discriminator inside a tenant. RFC 0044 uses `corpus_id='personal'` for existing personal memory and `corpus_id='striatum'` for the first Striatum bundle. |
| segment | Topic-coherent slice and embedding unit. Striatum corpus retrieval reuses this concept. |
| claim | Insert-only LLM extraction grounded by `claims.evidence_message_ids`; not created from Striatum corpus in V1. |
| belief | Consolidated bitemporal assertion with `beliefs.evidence_ids` and `beliefs.claim_ids`; not created from Striatum corpus in V1. |
| `context_for(conversation)` | Engram's eventual sectioned context compiler. RFC 0044's four MCP tools are a Phase 1 retrieval surface, not the full `context_for` product. |

Engram-local capabilities are value objects in Engram's serving boundary, not
Striatum daemon capabilities. The Striatum export bundle is durable provenance,
not live state. The MCP result reference is a retrievable citation into the
Engram corpus, not a Striatum artifact.
