# RFC 0057: Striatum Corpus Contract V2

Status: proposed (scaffold)
Date: 2026-05-14
author: implementer-unknown-model-001
Context:
[`docs/SPEC.md`](../SPEC.md),
[`docs/ROADMAP.md § 5.7`](../ROADMAP.md),
[`RFC 0041`](0041-engram-memory-layer-for-striatum-operators.md),
[`RFC 0044`](0044-engram-phase-1-implementation-spec.md),
`~/git/engram/STRIATUM_MEMORY_ROADMAP.md` (external roadmap, read-only reference).

RFC 0044 V1 shipped the first Striatum corpus export. RFC 0057 names the V2
decisions Engram needs before it can stand up retrieval over multiple Striatum
instances on one machine and before Striatum can opt workflows into Engram-
backed augmentation. This RFC is **scaffold only**: it bounds the decision
surface and lists the open questions. It does not commit Striatum to import
Engram or to call Engram during a run.

## Problem

The V1 contract (`striatum corpus export --since <ref> --out <dir>` plus nine
JSONL files plus `manifest.json`) is single-corpus. The whole machine emits
one bundle under `tenant_id='striatum'` and `corpus_id='striatum'`. That is
fine for one Striatum project; it stops being fine the moment one operator
runs Striatum against two unrelated target repositories on the same machine.
Engram cannot keep their evidence separated, retrieval cannot scope to one
project, and the "Striatum runs without Engram" invariant from RFC 0044 §8 has
no V2 regression coverage for new entry points.

The V2 surface must also clarify how (or whether) Striatum injects retrieved
context back into workflow packets, what budget and policy the runner is
willing to commit to, and which workflows opt in. That decision belongs in an
RFC rather than in unscoped operator improvisation, because injection touches
work-packet shape — a SPEC-level surface.

## Goals

- Define a versioned corpus bundle that downstream consumers (Engram first,
  later potentially others) can validate against without reading Striatum
  source. The bundle remains a redacted JSONL export with a verifying
  `manifest.json`; V2 adds explicit identity, watermark, and validation fields.
- Make multi-corpus identity explicit. A single Striatum installation can
  emit bundles for multiple `corpus_id` values without mixing rows across
  separate target repositories or separate Striatum daemon registrations.
- Preserve the augmentation-not-dependency invariant: Striatum continues to
  run with Engram missing, slow, unreachable, or unconfigured. No new
  `import engram`, no new `memory.*` capability, no daemon RPC method that
  reaches outside the Striatum daemon vocabulary.
- Specify an optional context-injection *policy* surface so workflow authors
  can opt a job into retrieval-backed augmentation under a per-packet budget,
  without making augmentation a packet prerequisite.
- Keep the V1 export consumable. V1 bundles must remain parseable by V2-aware
  consumers; V2 manifests must declare the contract version they advertise so
  Engram can refuse incompatible mixes without inferring shape.

## Non-Goals

- Implementing the Engram side. RFC 0057 is a Striatum-side contract. The
  Engram-side ingester, retrieval tools, MCP server, schema migrations, and
  capability vocabulary remain Engram's responsibility under their own RFC
  numbering.
- Hosted-mode export, cloud sync, remote retrieval, telemetry, transcript
  capture, or background ingestion. Bundles remain operator-triggered and
  local. Striatum must not stream events to Engram or any other consumer at
  run time.
- Replacing daemon-owned PostgreSQL, migration-only `.striatum/state.sqlite3`
  sources, the decision log, RFCs, operator reports, run summaries, or git
  history as live state or authoritative history. The corpus is durable
  provenance reshaped for retrieval, not live workflow state.
- Adding personal-memory functionality to Striatum. Personal-memory data
  lives entirely in Engram under `tenant_id='personal'`; Striatum has no
  per-user memory concept and does not export to one.
- Promoting Engram from an optional augmentation consumer to a runtime
  dependency. RFC 0057 cannot land in a shape that breaks the augmentation
  boundary regression test
  (`tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`).
- Defining how Engram embeds, segments, or scores the corpus. Striatum owns
  the bundle and redaction; Engram owns retrieval quality.

## Proposed Decision Surface

This section enumerates the V2 decisions that RFC 0057 must answer before it
moves from `scaffold` to `proposed (v2)` to `accepted`. Each decision should
be made deliberately by the design phase of a future dogfood; this scaffold
only frames the question.

### 1. Bundle manifest shape

What does `manifest.json` look like in V2? At minimum, it must carry:

- `corpus_contract_version` — closed integer, V1 implied, V2 explicit.
- `striatum_version`, `striatum_daemon_substrate` (`sqlite` or `postgres`),
  and `striatum_schema_version`.
- `repository.identity` — see Decision 5.
- `corpus_id` — see Decision 2.
- `since` ref, `git_head`, `dirty_tree` boolean, `generated_at`.
- `bundle_sha256` derived from canonical per-file hashes.
- Per-file row counts and SHA-256, exactly matching the emitted JSONL files.
- `redaction_tier` — see Decision 6.
- `incremental_export_watermark` — see Decision 7.

The decision is which of these are required vs. optional, and which
additional fields a V2-aware consumer is allowed to assume. The V1 manifest
must remain parseable by a V2-aware reader through an explicit fallback.

### 2. Multi-corpus identity (`corpus_id` naming)

V1 hardcodes `corpus_id='striatum'`. V2 must support one Striatum
installation emitting separate bundles per target repository or per logical
project without mixing rows. Open: human-readable string
(`striatum:<repo-slug>`), UUID, both, or a structured object. The exporter
must derive the value deterministically — re-running the export over the
same target repository must produce the same `corpus_id`.

The decision must also name the migration path for V1 consumers that already
ingested `corpus_id='striatum'`. The proposed default is to map the V1
identity onto a V2 identity using the operator's local Striatum daemon
registry repo entry, with a `legacy_corpus_alias` field in the manifest so
Engram can dedupe.

### 3. Source kinds and `sub_kind` extensibility

V1 ships a closed nine-kind set
(`rfc`, `decision_log_row`, `operator_report`, `run_summary`,
`audit_chain_entry`, `changelog_entry`, `ubiquitous_language_term`,
`harness_friction_pattern`, `commit`). V2 must decide:

- Whether to add `dogfood_review`, `dogfood_finding`, `blocker_row`, or
  similar new kinds, and at what granularity.
- Whether the V2 contract treats `sub_kind` as a closed enum or an open
  string with reserved prefixes. The closed-enum option preserves consumer
  validation; the open-string option lowers the bar for one-off operator
  scripts.
- Which kinds are required vs. optional in a valid V2 bundle. Mandatory
  kinds power Engram's retrieval guarantees; optional kinds keep small or
  young Striatum instances exportable without contortion.

### 4. Stable item IDs and content hashing

V1 derives `external_id` per kind (e.g., `decision:<D###>`, `rfc:<NNNN>#<slug>`,
`commit:<sha>`). V2 must decide:

- Whether the per-kind ID rules become part of the contract (so Engram can
  rely on them) or remain Striatum-private. The proposed default is to
  freeze V1 shapes into the contract.
- How to handle ID drift when a Markdown heading slug changes between
  exports; Engram needs either a stable identity or an explicit "deprecated
  alias" record so it can update retrievable references without dropping
  evidence.
- Whether each JSONL row carries `content_sha256` directly, in addition to
  the manifest's per-file hash. Per-row hashes let Engram detect partial
  reuse without re-reading the file.

### 5. Instance and repository identity

V1 records repository path and git HEAD. V2 should decide:

- Whether to expose a stable Striatum *instance* identity (per-machine, per-
  daemon, or per-target-repo) and how it interacts with `corpus_id`.
- Whether to record the daemon registry repository row identity so Engram
  can attribute retrieved evidence to a specific registered repository in
  multi-repo daemon deployments.
- How to represent identity when a target repository is renamed, moved on
  disk, or re-registered. The proposed default is "identity follows the
  daemon registry row id; path is best-effort".

### 6. Redaction tier and privacy metadata

V1 ships a denylist-based redactor and labels nothing on the row level. V2
should decide:

- A small closed set of privacy tiers (e.g., `tier_1_public`,
  `tier_2_operator_prose`, `tier_3_restricted`) and which kinds are exported
  in each tier by default.
- Whether `manifest.json` carries the redaction tier the bundle was emitted
  under, so Engram can refuse to mix tiers in retrieval results without
  capability evidence.
- Whether per-row redaction hints (e.g., "this row's free-text fields were
  scrubbed", "this row was already public") are part of the contract.

### 7. Incremental-export watermark

V1 exports a full bundle on every invocation since `--since <ref>`. V2 must
decide:

- Where the incremental watermark lives (manifest only; manifest plus a
  side file under `.striatum/`; daemon DB column). The chosen storage must
  survive bundle deletion and a clean re-export.
- The semantics of "incremental" — append-only since last bundle, plus
  retraction rows for deleted/renamed items, vs. full re-export with a
  short-circuit hash check.
- Whether the watermark is per-`corpus_id` (so each target repository
  advances independently).

### 8. Validation rules and consumer obligations

V2 must say which validation a conforming consumer MUST perform:

- Manifest schema version recognized.
- Per-file row counts and hashes match.
- `bundle_sha256` matches.
- Redaction tier acceptable for the consumer's intended use.
- Reject unknown required fields by default; warn-and-skip on unknown
  optional fields.
- Refuse to ingest a bundle whose `corpus_contract_version` is newer than
  the consumer supports.

These rules form Striatum's side of the contract: if Engram complies,
Striatum guarantees bundle stability across V2 minor versions.

### 9. Backward compatibility

V2-aware consumers must accept V1 bundles. The decision is whether:

- V1 bundles auto-promote to a synthesized V2 manifest (`corpus_contract_version: 1`,
  default `redaction_tier`, single-corpus identity).
- Or V1 bundles flow through a Striatum-side migration verb
  (`striatum corpus migrate-bundle --in <v1-dir> --out <v2-dir>`).

V1 bundles already in Engram's data store must be addressable under their
V2 identity without re-ingestion.

### 10. Augmentation boundary regression

V2 must extend the V1 boundary test
(`tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`)
to cover any new entry points the V2 surface introduces, including the
optional injection policy in Decision 11. The decision is which file paths
and substrings the test enforces. The non-negotiable invariants are:

- No `import engram`, no `from engram`, no `memory.*` capability in
  Striatum source.
- No daemon RPC method whose handler reaches outside the Striatum daemon
  vocabulary.
- No state transition (`ack`, `publish-artifact`, `complete`, `verdict`,
  recovery, `run prepare`, `run start`) that fails because Engram is
  missing, unreachable, or misconfigured.

### 11. Context-injection policy (optional V2 surface)

The most contentious V2 decision. Three shapes are on the table:

- **No injection.** V2 only specifies the bundle. Operators install
  Engram's MCP server out of band; agents query it through their own MCP
  client when they choose to. Striatum does not see the queries.
- **Explicit workflow-level opt-in.** A new `augmentation` block in
  `workflow.json` names a retrieval source, a per-packet token budget, and
  the workflow jobs that opt in. The runner prepares packet `context`
  references to retrieval results but does not call Engram itself; the
  agent (or a sidecar) performs the fetch using the standard MCP path.
- **Runner-side fetch.** The runner shells out to a configured retrieval
  command at packet-build time, captures the result, and embeds it in the
  packet. This breaks the augmentation boundary unless the call is
  fail-open and bounded.

The proposed default for V2 is the **explicit workflow-level opt-in** shape,
because it preserves the runner-never-imports-Engram invariant while still
giving operators a documented place to wire retrieval. Default per-packet
budgets and per-workflow caps belong in the decision.

### 12. Engram availability without a runtime dependency

V2 should decide how (and whether) Striatum records that an operator has
Engram installed and configured. The default is "no recording" — Striatum
remains unaware of Engram, the operator manages MCP configuration, and the
runner never reads `~/.local/share/engram/` or similar paths. If V2
introduces an optional record (e.g., a `striatum operator memory check`
verb), it must exit `0` with a clear message when Engram is unreachable
and must never gate a workflow command.

## Acceptance Criteria (deferred)

This is a scaffold. Acceptance criteria will be authored once the decisions
above are made. At minimum, the final RFC must require:

- A V2 bundle round-trips through a fresh export with byte-identical hashes
  for unchanged inputs (only `generated_at` changes).
- Multi-corpus exports under one Striatum installation produce disjoint
  per-`corpus_id` bundles.
- The V1 fixture exports remain readable by a V2-aware consumer.
- The augmentation-boundary regression covers every V2 surface added by
  this RFC.
- Every new operator-facing verb introduced by V2 documents an Engram-
  unavailable success path.

## Open Questions

1. Does V2 ship as a single contract bump, or as `V1.5` (multi-corpus + ID
   rules) followed by `V2.0` (injection policy)? Engram's roadmap is gated
   on identity and incremental export; the injection policy could be sliced
   off into a separate `RFC 0053` if it slows the rest down.
2. Should the contract live in `docs/rfcs/0057-corpus-contract-v2.md` plus
   a machine-readable schema under `src/striatum/corpus/`? If so, which
   file extension and which validator owns it?
3. Should Striatum expose a corpus *describe* verb (`striatum corpus
   describe --since <ref>`) that emits the manifest without writing the
   JSONL files, so Engram can negotiate before paying the export cost?
4. How should corpus export's `since <ref>` semantics evolve now that
   RFC 0043/RFC 0048 moved `runs`/`events`/`artifacts` into daemon-owned
   PostgreSQL? Current export reads from daemon state plus repository
   provenance; V2 should make that contract explicit.
5. What is the long-term home for the augmentation-boundary regression
   test? `tests/test_cli_corpus_export.py` is the right place for V1; a
   broader `tests/test_augmentation_boundary.py` may be warranted for V2.

## Domain Modeling

V2 reuses RFC 0044's terms with explicit version annotation:

| Term | V2 Notes |
|---|---|
| memory augmentation | Unchanged. Engram is one possible consumer; the augmentation contract is not Engram-specific. |
| Striatum corpus | V2 closed set of source kinds, possibly larger than V1's nine; documented per-bundle in `manifest.json`. |
| Striatum corpus export | V2 manifest carries `corpus_contract_version`; multi-corpus identity per Decision 2; per-bundle redaction tier per Decision 6. |
| augmentation-not-dependency | V2 must extend the boundary test (Decision 10) to cover any new entry point introduced by the optional injection policy (Decision 11). |
| corpus contract version | New value object. Closed integer set; V1 implied, V2 explicit. Encodes which fields a conforming consumer is allowed to assume. |
| corpus identity | New value object. Stable per-Striatum-installation, per-target-repository identifier. Replaces V1's hard-coded `corpus_id='striatum'`. |
| redaction tier | New value object. Closed set; carried per bundle in the manifest and (per Decision 6) optionally per row. |
| incremental-export watermark | New value object. Stable across exports for a given `corpus_id`. Storage location per Decision 7. |
| context-injection policy | New workflow-level surface (Decision 11). Workflow config opt-in only; not a runtime dependency on Engram. |

The contract is a value object the Striatum runner owns and Engram (and any
future consumer) consumes. The boundary test is a code-owned invariant that
this RFC must not regress.
