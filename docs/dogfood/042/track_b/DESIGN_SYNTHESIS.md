---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/042/track_b/design/codex/DESIGN.md", "docs/dogfood/042/track_b/design/claude_code/DESIGN.md", "docs/dogfood/042/track_b/design/gemini/DESIGN.md"]
---

author: synthesizer-codex-1

# RFC 0044 V1 Design Synthesis: Engram Phase 1 as Striatum's Optional Memory Layer

This synthesis reconciles three Track B designs (codex, claude_code, gemini)
into a single concrete plan for RFC 0044 V1. The implementer for Track B
is codex, authoring `docs/rfcs/0044-engram-phase-1-implementation-spec.md`
against this synthesis. Where the three designs agreed, the synthesis adopts
the consensus. Where they diverged, it picks one path and states why.

## Engram Vocabulary Citations (Ground Truth)

These citations are taken directly from `~/git/engram/` and are load-bearing
for the rest of the synthesis. RFC 0044 must use this vocabulary verbatim.

- **Three-tier separation** (`~/git/engram/README.md`,
  `~/git/engram/docs/UBIQUITOUS_LANGUAGE.md`): raw evidence → claims →
  beliefs. Raw evidence is immutable after insert (DB triggers block
  UPDATE/DELETE on `sources`, `conversations`, `messages`, `notes`,
  `captures`). Every derivation is non-destructive and versioned.
- **`source_kind` enum** (`~/git/engram/migrations/001_raw_evidence.sql`,
  amended by `003_source_kind_claude.sql` and
  `005_source_kind_gemini.sql`): current values are `chatgpt`, `claude`,
  `gemini`, `obsidian`, `capture`, `future`. Adding a new source kind
  requires a numbered SQL migration applied in filename order by
  `engram migrate`.
- **`sources` row shape** (`~/git/engram/docs/ingestion.md` §"What Gets
  Stored", schema in `~/git/engram/docs/schema/README.md`): each ingest
  writes one `sources` row keyed by `(source_kind, external_id)` with a
  SHA-256 manifest hash and the original payload as `raw_payload`.
- **Bounded contexts** (`~/git/engram/docs/UBIQUITOUS_LANGUAGE.md`):
  Ingest, Corpus, Retrieval, Eval, Supervisor, Lifecycle, and Project
  Coordination. The corpus-reading process has no network egress; the
  network-using process has no direct corpus access. This is structural,
  not disciplinary.
- **Subject / consumer split** (same file): there is one subject
  (the biography's owner); consumers are downstream MCP clients that
  receive `context_for` output but never touch the corpus directly.
- **`claims` vs `beliefs`** (`~/git/engram/docs/claims_beliefs.md`):
  claims are insert-only LLM extractions bound to a segment with
  `evidence_message_ids`; beliefs are bitemporal, status-tracked,
  stability-classed rows consolidated from claims by a deterministic
  Python consolidator. Beliefs never derive from other beliefs.
- **Segmentation contract** (`~/git/engram/docs/segmentation.md`): the
  segmenter calls only the local OpenAI-compatible ik-llama endpoint at
  `http://127.0.0.1:8081/v1/chat/completions`. Raw turns are never
  embedded directly; segments are the unit of embedding and claim
  extraction. Embedding uses `nomic-embed-text` via Ollama at 768
  dimensions stored in `segment_embeddings` (HNSW + pgvector).
- **Database** (`~/git/engram/docs/ingestion.md` §"Database",
  `~/git/engram/SPEC.md`): PostgreSQL 16 + pgvector, local socket auth
  at `postgresql:///engram`, bound to 127.0.0.1 only.
- **Server topology direction** (`~/git/engram/docs/rfcs/0022-server-binary-api-mcp.md`):
  a single Python binary `engramd` hosts a shared handler layer with both
  HTTP and MCP transports, binds to 127.0.0.1, and is the substrate for
  external MCP clients (Claude, ChatGPT, Gemini, Cursor). RFC 0022 is
  proposal status, not yet implemented; RFC 0044 V1 implements the
  MCP-stdio slice of that direction for the Striatum corpus.
- **Reclassification vs capture** (`~/git/engram/docs/UBIQUITOUS_LANGUAGE.md`,
  migration `002_capture_reclassification.sql`): `captures` is one
  specific kind of raw evidence (observations, tasks, ideas, references,
  reclassifications); it is not a generic external-corpus slot. Do not
  overload `source_kind='capture'` for the Striatum corpus.

These are the names RFC 0044 must respect. The synthesis derives every
schema and capability choice from them.

## Decision Set

The synthesis decides eight things. Each is binding for the RFC implementer.

### D1. Ingestion path: operator-triggered pull, owned by Engram

All three designs agreed on pull-mode for V1; the synthesis adopts it.

**Engram owns the indexer.** A new Engram CLI verb
`engram ingest-striatum --repo <path> [--since <ref>]` reads the local
Striatum repository through a stable, redacted export contract (see D3).
Striatum does not push, does not call Engram, and does not emit
`run.completed` events to Engram in V1. Push-mode is explicitly deferred
to Phase 3 (RFC 0046+) per RFC 0041 §"Phase 3".

Rationale: pull mode keeps the dependency one-way (Engram knows the
Striatum corpus shape; Striatum does not know Engram exists at runtime),
avoids coupling Striatum's daemon RPC version (RFC 0030/0039) to Engram's
ingest schema, and lets the Engram-side reading process keep its
no-egress property. Cron is documented as an operator convenience, not
a V1 dependency.

### D2. Corpus separation: a new `corpus` discriminator, not a new `source_kind`

The three designs split on this. The synthesis picks a hybrid that respects
Engram's actual schema.

Engram's existing `source_kind` enum is the **ingest** discriminator
(which parser produced this raw row). It is not the **corpus** discriminator
(which biographical scope this row belongs to). Adding `source_kind='striatum'`
without a corpus discriminator would mix Striatum software-building rows
into the same biographical pool as ChatGPT / Claude / Gemini personal-life
rows, which is the exact failure RFC 0041 §"Augmentation-Not-Replacement"
forbids.

V1 therefore:

1. **Adds `source_kind='striatum'`** via a new numbered migration
   (provisionally `013_source_kind_striatum.sql`, final number set at
   implementation time), mirroring the existing Claude and Gemini
   precedent (`003_source_kind_claude.sql`, `005_source_kind_gemini.sql`).
2. **Adds a `corpus_id` column** to `sources` (default `'personal'`,
   non-null after backfill), and propagates it onto every derived row
   (`conversations`, `messages`, `notes`, `captures`, `segments`,
   `claims`, `beliefs`). The migration backfills existing rows with
   `corpus_id='personal'` so the existing personal-life corpus stays
   untouched. Striatum-corpus rows are written with `corpus_id='striatum'`.
3. **Defaults retrieval to `corpus_id='striatum'`** for the Striatum
   operator's MCP token. Cross-corpus retrieval is refused unless the
   request both carries `memory.read_cross_corpus` (see D5) and
   explicitly names both corpora.

Rationale: `source_kind` is preserved as the ingest taxonomy that the
schema and existing reviews already depend on; `corpus_id` is added
because the biography vs software-building split is a different axis.
Overloading `source_kind='capture'` (as one alternative considered) would
collide with the captures-table reclassification flow per
`~/git/engram/docs/UBIQUITOUS_LANGUAGE.md` §"Raw evidence vs captures".
Using `source_kind='future'` with a discriminator inside `raw_payload`
was considered as a faster path but rejected because it would force the
retrieval layer to filter on JSONB rather than a typed column, and would
need a follow-up migration anyway.

The personal-life corpus is **never** modified by the Striatum
ingester. The existing ChatGPT/Claude/Gemini ingest paths are untouched;
they continue to write `corpus_id='personal'` rows by default after the
backfill.

### D3. The Striatum export format: JSONL bundle on disk, produced by Striatum

The codex and claude_code designs put the export verb on the Striatum side;
gemini left ownership ambiguous. The synthesis sides with codex / claude_code:
**Striatum owns its own redaction discipline.**

Striatum adds one new CLI verb:

```text
striatum corpus export --since <ref> [--out <dir>]
```

implemented in a new package `src/striatum/corpus_export/`. The verb is
read-only against the target repository and `.striatum/state.sqlite3`
and emits a directory bundle:

```text
<out>/
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

`manifest.json` carries `striatum_version`, repo path, git HEAD, dirty-tree
flag, `since` ref, per-file SHA-256, SQLite schema version, selected row
counts, source kinds emitted, and `generated_at`. Engram's
`ingest-striatum` reads the manifest, verifies counts, and refuses partial
loads.

Each JSONL line is a `{source_kind: "striatum", external_id, sub_kind,
content, provenance, observed_at}` record where `sub_kind` is one of the
nine values above (`rfc`, `decision_log_row`, `operator_report`,
`run_summary`, `audit_chain_entry`, `changelog_entry`,
`ubiquitous_language_term`, `harness_friction_pattern`, `commit`) and
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

Format properties (consensus across all three designs):

- JSONL only — operator-readable, diff-able, no binary tools required.
- One file per `sub_kind` so re-ingest can be targeted.
- No transcripts, no terminal output, no model output, no SQLite blobs.
  Run summaries are sourced via `striatum run summary --json`, not
  raw SQLite reads, per D028.
- `.striatum/` is never read directly for free-text fields whose
  privacy tier is ambiguous; the export is the redaction boundary.

Engram consumes the bundle by walking the JSONL files in a fixed order,
writing one `sources` row per bundle (keyed `(source_kind='striatum',
external_id=<repo-id>:<since>:<manifest-hash>)`), and one downstream row
per JSONL line. Re-ingesting the same manifest hash is a no-op; an
external-id collision with different content raises `IngestConflict`
exactly as the existing ingesters do
(`~/git/engram/docs/ingestion.md` §"Dedup And Conflicts").

### D4. Engram MCP server topology: standalone `engram-mcp-stdio` binary

All three designs agreed on standalone. The synthesis adopts it with
specifics matched to RFC 0022's direction.

V1 ships **`engram-mcp-stdio`**, a standalone process living in Engram's
repository (recommended path: `~/git/engram/src/engram/api/mcp_stdio.py`
with a console-script entry point), implementing the MCP stdio transport
against the shared handler layer RFC 0022 §"Shape" describes. The same
handler layer can later be reused by `engramd`'s HTTP transport; V1 is
not blocked on `engramd` shipping first.

The MCP server:

- Speaks **MCP stdio**, not a socket, because that is the shape Claude
  Code's `mcp_servers` config and codex/gemini CLI MCP clients expect,
  and it sidesteps port management.
- Runs in-process with read-only Postgres access to the Engram database
  (`postgresql:///engram` per `~/git/engram/docs/ingestion.md`). It does
  not run as a sidecar of the Docker Compose path; that path stays for
  bulk ingest. Phase 1 retrieval does not require the LLM segmenter or
  Ollama embedding service to be running; it only requires Postgres
  with the ingested Striatum corpus.
- Honors Engram's no-egress invariant: the server process never makes
  outbound network calls. This is enforced at the OS level per D018
  (cited in RFC 0022).
- Is **not** implemented as a Striatum chat tool. Wrapping Engram
  retrieval into Striatum's RFC 0036 chat-tool closed set would invert
  the augmentation boundary, balloon the chat-tool count (already 14 in
  RFC 0036 + RFC 0040), and couple Engram retrieval to Striatum's
  daemon RPC version migration (RFC 0039). A standalone server reaches
  codex / gemini / any future frontier-model CLI with zero Striatum
  surface area.

### D5. Capability vocabulary: Engram-local, four tokens, never mixed with Striatum RPC

All three designs agreed that capabilities are Engram-local and must not
extend Striatum's RFC 0030 daemon RPC set
(`read`/`write`/`review`/`claim`/`apply`/`admin`/`recovery`). The synthesis
fixes the exact names:

| Capability | Issued by | Default for Striatum operator | Required for |
|---|---|---|---|
| `memory.read_striatum` | Engram MCP server | yes | `engram.search(corpus="striatum")`, `engram.fetch_reference` |
| `memory.describe` | Engram MCP server | yes | `engram.describe_corpus`, `engram.health` |
| `memory.read_personal` | Engram MCP server | **no** | `engram.search(corpus="personal")` |
| `memory.read_cross_corpus` | Engram MCP server | **no** | any mixed-corpus search |

`memory.write` is reserved for Phase 3 and is **not** issued in V1; the
MCP tool surface for V1 has no write tools at all. `memory.admin_index`
is **not** added in V1; manual indexing is CLI-only (`engram
ingest-striatum`) to keep the operator surface minimal. Promoting the
admin verb to MCP can land in Phase 1.5 if the smoke-test bar exposes a
need.

The Engram capability token format is Engram's choice; the synthesis does
not propose a token shape because that is Engram-internal. Token storage
is in `~/.config/engram/` and is independent of Striatum's
`${XDG_RUNTIME_DIR}/striatum/` runtime token store per RFC 0030 §10.

### D6. MCP tool surface: four read-only tools

The three designs converged on four tools modulo naming. The synthesis
picks names that match Engram's `~/git/engram/docs/UBIQUITOUS_LANGUAGE.md`
§"Retrieval" vocabulary (`context_for`, `lane`, `section`) without
pretending V1 has implemented the full `context_for` compiler.

```text
engram.search(query: str, corpus: "striatum" = "striatum",
              filters?: { sub_kind?: [...], since?: ts, until?: ts,
                          run_id?: str, rfc?: str, decision_id?: str },
              k: int = 10)
  -> { results: [{ reference_id, corpus_id, source_kind, sub_kind,
                   external_id, content, provenance:
                   { commit?, path?, sha256?, audit_id?, run_id?,
                     decision_id?, rfc?, ts },
                   score, privacy_tier }],
       retrieval_id }

engram.fetch_reference(reference_id: str)
  -> { reference_id, corpus_id, source_kind, sub_kind, external_id,
       content, provenance, neighbors? }

engram.describe_corpus(corpus: "striatum" = "striatum")
  -> { document_count, oldest_ts, newest_ts,
       sub_kind_counts: { sub_kind: count },
       last_ingest_ts, last_ingest_manifest_hash,
       schema_version, adapter_version }

engram.health()
  -> { ok: bool,
       corpus_status: { striatum: "ready" | "empty" | "degraded" | "missing" },
       substrate_version, engram_version, mcp_version }
```

All four are read-only. There is no `claim_create`, no `belief_revise`,
no `ingest_run_completed`, no Striatum mutation passthrough, no raw
SQL surface. Privacy-tier and corpus labels appear on every result item
so the operator session can see at a glance what was returned.

The retrieval implementation for V1 reuses Engram's segmentation +
embeddings pipeline against the Striatum corpus when feasible, with a
deterministic lexical fallback (Postgres FTS over the same rows) so
that a missing or stale embedding model does not block retrieval. The
scorer is the same simple weighted scorer Engram already commits to per
`~/git/engram/SPEC.md` §"Key design properties" ("Simple weighted
scorer in the live path: no LLM reranker at serve time"). V1 does not
add a Striatum-tuned ranker; if retrieval quality on the smoke-test bar
is poor, a structural-aware segmenter follow-up lands in Phase 1.5.

### D7. Striatum-side wiring: one skill, one read-only CLI verb, one export verb, zero workflow coupling

The three designs proposed adjacent but slightly different Striatum
surfaces. The synthesis fixes a minimal set:

1. **Skill bundle**: a new RFC 0015 bundled skill `striatum-engram`
   teaches the operator session how Engram retrieval works: when to
   invoke (session start + on demand for unknown D### / RFC ####),
   how to call the four tools, the augmentation-not-dependency rule,
   and the capability vocabulary. Templates land under
   `src/striatum/skills/templates/claude_code/engram.md.tmpl` and the
   generic equivalent, wired into the existing skills tuple. The skill
   body is short and harmless when Engram is offline.
2. **CLI verbs (Striatum side, two total)**:
   - `striatum corpus export --since <ref> [--out <dir>]` (per D3).
   - `striatum operator memory check` — read-only, shells out to
     `engram-mcp-stdio --health-check` (or the equivalent
     `engram.health` call), prints status, always exits 0 even if
     Engram is unreachable. This is the only Striatum surface that
     touches Engram, and it is informational.
3. **No `workflow.json` field, no daemon RPC method, no chat tool.**
   Retrieval is operator-scoped per session, not workflow-scoped, and
   not part of Striatum's mutation surface. A future `memory_provider`
   advisory hint can be considered in Phase 2 if operator UX justifies
   it; it is not in V1.
4. **MCP client configuration is documented, not automated.** The
   operator manually adds the Engram MCP server to
   `~/.claude/settings.json` (or the codex / gemini equivalent) per
   the snippet documented in `docs/HOW_TO_HUMAN.md`. Striatum does not
   write the operator's MCP config.

   ```jsonc
   "mcp_servers": {
     "engram": {
       "command": "engram-mcp-stdio",
       "args": ["--corpus", "striatum"]
     }
   }
   ```

### D8. Augmentation-not-dependency: mechanically enforced, three rules

All three designs agreed on this boundary in principle. The synthesis
gives it three enforceable rules:

1. **No Striatum CLI verb imports an Engram client library.** A grep
   audit of `src/striatum/cli/**/*.py` after RFC 0044 V1 lands must
   return zero hits for Engram client imports. The one verb that
   touches Engram (`striatum operator memory check`) shells out via
   subprocess and returns exit 0 unconditionally — the printed status
   is informational only.
2. **No Striatum daemon RPC method references Engram.** RFC 0030's
   method registry stays at its current seven capabilities. No
   `memory.*` capability is added on the Striatum side. No daemon code
   path waits on Engram.
3. **The harness includes an "Engram off" acceptance test.** A
   dogfood-shaped run with no `engram-mcp-stdio` process running, and
   no Engram Postgres instance reachable, must produce the same
   Striatum artifacts (decision log row, operator report, run summary)
   as an Engram-running run. The session-start `engram.health` call
   times out at 3 seconds and the session proceeds with the pre-Engram
   behavior (read AGENTS.md / DECISION_LOG.md / RFCs directly).

Retrieval-call timeouts inside an active session degrade silently: a 2s
budget for `engram.search`, 5s for `engram.fetch_reference`, after
which the operator proceeds without the result. No Striatum state
transition (`ack`, `publish-artifact`, `complete`, verdict, recovery)
ever blocks on or fails because of an Engram call.

## Phase 1 Boundaries (Non-Goals)

Locked, for the implementer's RFC body:

- **No claim or belief creation from the Striatum corpus in V1.**
  Striatum-corpus rows live as raw evidence and (optionally)
  segment-indexed retrieval rows. Belief consolidation is Phase 3.
- **No personal-life corpus changes.** ChatGPT, Claude, Gemini, and
  Obsidian ingest paths are untouched. The personal-life biography
  remains Engram's long-arc mission.
- **No write-side dogfood ingestion.** `run.completed` does not trigger
  Engram. Phase 3 covers this.
- **No personal-life retrieval in the default Striatum operator
  session.** `memory.read_personal` is never issued by default.
- **No hosted, multi-tenant, or cross-machine memory.** Engram stays
  single-user / single-machine per D083; if the operator works on two
  laptops, they have two Engram instances.
- **No transcript capture, no model-output ingestion, no terminal
  scrape.** The Striatum corpus is the structural corpus only.
- **No outbound network from any Engram corpus-reading process.**
  Per Engram's structural no-egress rule.
- **No Striatum mutation tool exposed through `engram.*`.** The MCP
  surface is read-only.
- **No `memory.admin_index` MCP tool in V1.** Indexing is CLI-only.

## Acceptance Criteria

RFC 0044 V1 lands only if all of the following hold.

### A. Engram-side ingestion (Engram repo work)

1. Migration adds `source_kind='striatum'` to the enum and adds
   `corpus_id` columns where required, applied via `engram migrate` in
   filename order.
2. Existing rows backfill to `corpus_id='personal'`; the personal-life
   corpus is byte-identical for retrieval pre- and post-migration.
3. `engram ingest-striatum --repo <path> [--since <ref>]` reads a
   Striatum export bundle, validates the manifest, writes one
   `sources` row plus per-line `raw_payload`-bearing rows under
   `corpus_id='striatum'`, and is idempotent under unchanged manifest
   hash. Conflicting manifest hashes for the same `(source_kind,
   external_id)` raise `IngestConflict`.
4. The ingester preserves provenance for every row: repo path, git
   HEAD, file path, blob SHA-256, sub_kind, decision_id / rfc / run_id
   / audit_id as applicable, and `observed_at` vs `recorded_at`
   distinct.
5. The ingester never writes to `.striatum/` and never calls Striatum
   mutation commands.
6. Transcript-like data is excluded by construction (the export bundle
   does not contain it).

### B. Engram-side retrieval (Engram repo work)

1. `engram-mcp-stdio` ships as a console-script entry point and speaks
   MCP stdio against the shared handler layer.
2. The four V1 tools (`engram.search`, `engram.fetch_reference`,
   `engram.describe_corpus`, `engram.health`) are read-only and refuse
   unknown corpora.
3. Capability checks refuse `corpus="personal"` without
   `memory.read_personal` and refuse mixed-corpus queries without
   `memory.read_cross_corpus`. Default Striatum operator tokens carry
   only `memory.read_striatum` + `memory.describe`.
4. The server binds stdio or 127.0.0.1 only; the corpus-reading
   process makes no outbound network calls.
5. Every search result carries `corpus_id`, `source_kind`, `sub_kind`,
   `privacy_tier`, and a reconstructable `provenance` block.

### C. Striatum-side export and wiring (Striatum repo work)

1. `striatum corpus export --since <ref> [--out <dir>]` writes the
   JSONL bundle in D3 with a verifying manifest. Re-running with
   unchanged inputs produces identical bundles (within timestamps
   captured in the manifest).
2. The export reads `.striatum/state.sqlite3` only through the
   existing `striatum run summary --json` interface for run-summary
   rows; it does not introduce a new SQLite read path for free-text
   fields.
3. `striatum operator memory check` shells out to
   `engram-mcp-stdio --health-check`, prints status, and exits 0
   regardless of Engram availability.
4. The `striatum-engram` skill is installed in the bundle and visible
   to the operator session at start.
5. Documentation lands in `docs/SPEC.md` (Optional Engram Memory
   Augmentation), `docs/HOW_TO_AGENT.md`, `docs/HOW_TO_HUMAN.md`,
   `docs/UBIQUITOUS_LANGUAGE.md` (new terms in §"Ubiquitous Language
   Additions" below), and `docs/DECISION_LOG.md` (one row recording
   the V1 commitment).

### D. Augmentation-not-dependency

1. A grep audit of `src/striatum/cli/**/*.py` finds zero Engram client
   imports.
2. No `memory.*` capability appears in the Striatum daemon RPC method
   registry.
3. The acceptance harness includes an "Engram off" test where a full
   dogfood-shaped run with `engram-mcp-stdio` not on PATH and the
   Engram Postgres database unreachable produces the same Striatum
   artifacts as a run with Engram available.
4. The RFC 0035 multi-repo harness gains an **optional** Engram
   fixture gated by `ENGRAM_TEST=1`; CI green is unaffected when the
   env var is unset.

### E. Smoke and negative retrieval

1. **Seed corpus**: dogfoods 035–040, decisions D080–D092, RFCs
   0030–0041 (plus RFC 0044 V1 itself once authored), the active
   `CHANGELOG.md` entries, `docs/UBIQUITOUS_LANGUAGE.md` rows, and the
   most recent ~50 commits. Ingest must complete under one minute on a
   developer laptop.
2. **Positive smoke (5/5 required)** — for each of the following,
   `engram.search(query, corpus="striatum", k=5)` returns at least one
   hit whose provenance matches the expected D### / RFC #### /
   dogfood id:
   - "what friction patterns recurred across dogfoods 036-039"
   - "which RFC moved the no-node toolchain rule and why"
   - "has surgical_recovery been invoked before"
   - "what did the build review for dogfood-037 say about test coverage"
   - "which dogfoods touched the daemon RPC capability vocabulary"
3. **Negative smoke (5/5 required)** — for each out-of-corpus query,
   top-5 hits are either empty or below the documented score floor:
   - "what is Jennifer's MBTI"
   - "best pizza in Berlin"
   - "Python f-string formatter rules"
   - "tomorrow's weather"
   - "the JFK files"
4. Documentation states what RFC 0044 V1 **cannot** claim: no
   cross-machine sync, no hosted retrieval, no multi-tenant memory, no
   default personal-life retrieval, no authoritative replacement for
   Striatum's state store, no belief derivation, no retrieval-grounded
   auto-summary at session start.

## Implementation Plan (5 Steps, Independently Landable)

The five steps split cleanly across two repositories. Engram-side work
lands first because Striatum's export format is designed against
Engram's ingest contract.

### Step 1 — Engram: corpus + ingester (Engram repo)

- Add migration `NNN_source_kind_striatum.sql` plus `corpus_id`
  columns and backfill (`corpus_id='personal'` for existing rows).
- Implement `engram ingest-striatum --repo <path> [--since <ref>]`,
  reading the JSONL bundle from D3 and writing
  `corpus_id='striatum'` rows.
- Reuse the existing segmentation pipeline against
  `source_kind='striatum'` rows. No new segmenter strategy in V1; if
  retrieval quality is poor, Phase 1.5 adds a structural-aware
  segmenter behind a flag.
- Add `~/git/engram/docs/striatum_corpus.md` documenting the new
  corpus and ingest verb.

### Step 2 — Engram: MCP server (Engram repo)

- Land `engram-mcp-stdio` console-script entry point with the four
  tools from D6.
- Implement Engram-local capability vocabulary from D5.
- Add `~/git/engram/docs/howto/mcp_server.md` covering setup, the
  stdio config snippet, and the smoke procedure.

### Step 3 — Striatum: corpus export (Striatum repo)

- Add `src/striatum/corpus_export/` package and
  `striatum corpus export` CLI verb.
- Emit the JSONL bundle from D3 with a verifying manifest.
- Unit tests cover each `sub_kind`'s extraction rule against
  fixtures.

### Step 4 — Striatum: operator skill + check verb (Striatum repo)

- Add the `striatum-engram` skill template and wire it into the
  RFC 0015 V1 skill tuple.
- Add `striatum operator memory check` shelling out to
  `engram-mcp-stdio --health-check`.
- Update `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/DECISION_LOG.md` per acceptance criterion C5.

### Step 5 — Integration harness (Striatum repo)

- Extend the RFC 0035 multi-repo harness with the optional Engram
  fixture (skipped unless `ENGRAM_TEST=1`).
- Land the five positive and five negative smoke queries under
  `tests/engram_smoke/`.
- Land the "Engram off" assertion: a dogfood-shaped run with Engram
  intentionally unreachable produces the same artifacts.

Steps 1 and 2 are Engram-repo PRs; Steps 3, 4, and 5 are Striatum-repo
PRs. Step 3 can be drafted in parallel with Steps 1-2 because the
export format is the contract between them; the smoke tests in Step 5
only run once Steps 1-2 land.

## Ubiquitous Language Additions

New terms RFC 0044 V1 adds to `docs/UBIQUITOUS_LANGUAGE.md` on the
Striatum side and (via the Engram-side doc landing in Step 1) to
Engram's `docs/UBIQUITOUS_LANGUAGE.md`:

| Term | Definition |
|------|------------|
| memory augmentation | An optional retrieval layer over a corpus of repository-grounded documents the Striatum operator session may query but never depends on. V1's only provider is Engram. |
| Striatum corpus | The closed set of Striatum software-building artifacts Engram indexes: RFCs, decision-log rows, operator-report interventions, run summaries, audit-chain entries, changelog entries, ubiquitous-language rows, harness-friction-pattern rows, and commits. |
| `corpus_id` | Discriminator on every Engram row separating `striatum` (software-building) from `personal` (Engram's existing biographical scope). Added in V1 alongside `source_kind='striatum'`. |
| Engram-local capability | A capability token issued by the Engram MCP server (`memory.read_striatum`, `memory.describe`, `memory.read_personal`, `memory.read_cross_corpus`), distinct from Striatum's RFC 0030 capability vocabulary and stored separately. |
| Striatum corpus export | The redacted-by-construction JSONL bundle produced by `striatum corpus export --since <ref>` and consumed by `engram ingest-striatum`. Contains no transcripts, no terminal output, no model output. |
| augmentation-not-dependency | The product invariant that no Striatum critical-path operation may block on or require a memory-augmentation call. If Engram is unavailable, Striatum runs unchanged. |

## Open Items the Implementer Resolves at Authoring Time

These are not unresolved design decisions; they are mechanical choices
the RFC author makes during drafting and that do not change the
synthesis:

1. The exact migration filename and numeric prefix for the
   `source_kind='striatum'` + `corpus_id` migration. The prefix is the
   next available number in `~/git/engram/migrations/`.
2. The exact `extraction_prompt_version` / `segmenter_prompt_version`
   strings used when the existing pipeline runs against the Striatum
   corpus. V1 reuses the active versions; no new prompt versions are
   minted unless retrieval quality forces it (Phase 1.5).
3. The default privacy tier applied to Striatum-corpus rows. The
   synthesis recommends Tier 1 for commit-safe rows (RFCs, decisions,
   changelog, ubiquitous language) and a stricter tier for free-text
   operator-report and audit-chain rows until a reclassification pass
   runs. The RFC author proposes a default and `docs/DECISION_LOG.md`
   records it.
4. Final names of the four MCP tools if Engram's internal verb shape
   prefers `claim.search` or similar. The synthesis recommends
   `engram.search` / `engram.fetch_reference` /
   `engram.describe_corpus` / `engram.health`; the implementer may
   align with whatever Engram's `engramd` handler layer ends up
   naming, provided the read-only-four-tool shape is preserved.
5. Whether the smoke-query corpus needs a Phase 1.5
   structural-aware segmenter. Decided at smoke-test-run time, not at
   RFC-authoring time. If the 5/5 positive bar is met with the
   default segmenter, no Phase 1.5 work is needed for V1.

## Followups (Explicitly Out of Phase 1)

- **F1 — Phase 1.5 structural segmenter** for the Striatum corpus, if
  default segmentation underperforms on the smoke-test bar.
- **F2 — Phase 2 auto-retrieval** at session start: the operator's
  session brief auto-invokes `engram.search` with the active dogfood /
  RFC / decision context. Same MCP surface; new operator convention.
- **F3 — Phase 3 write-side**: `run.completed` triggers Engram
  ingestion, dogfood operator reports become Engram claims with
  audit-chain-grounded provenance, belief consolidation runs over the
  Striatum corpus. Requires `memory.write` capability.
- **F4 — Harness improvement**: design-lane work packets gain an
  `extra_read_paths` field granting read-only sibling-repo access
  (e.g., `~/git/engram/`) for the duration of the lane. Filed
  separately as a harness-improvement proposal; tracked outside
  RFC 0044.
- **F5 — Phase 4 personal-life corpus re-attack** with the validated
  pipeline. Engram's long-arc mission.

## Risks

- **Engram pipeline instability.** Engram is mid-build (Phase 1.5
  complete per `~/git/engram/README.md`; Phase 2 segmentation is
  active). Schema changes between V1 design and ship could force
  re-ingest. Mitigation: ingest is idempotent under unchanged
  manifest hash; re-ingest is cheap.
- **Retrieval quality below smoke bar.** If the five positive
  queries don't all return a provenance-correct hit, V1 ships only
  with the structural-segmenter follow-up landed first. F1 is the
  pre-planned fallback.
- **Migration backfill cost.** Adding `corpus_id` to existing tables
  requires touching every derived row. Mitigation: the backfill is a
  single-statement `UPDATE ... SET corpus_id='personal'` per table
  within one migration transaction; existing rows are not large enough
  to make this a real cost on a developer laptop.
- **Capability-vocabulary confusion.** Operators now juggle two token
  surfaces (Engram-local and Striatum-RPC). Mitigation: D5 documents
  them as separate registries in separate filesystem locations; the
  `striatum-engram` skill body teaches the distinction in two
  sentences.
- **Phase 1 → Phase 2 surface stability.** Phase 2 (auto-retrieval)
  must not require renaming the four V1 tools. Mitigation: the V1
  tool surface already covers the Phase 2 retrieval shape; Phase 2
  adds an operator convention, not new tools.

## Summary of the Specific Plan

RFC 0044 V1 ships:

- A new `source_kind='striatum'` plus a `corpus_id` discriminator
  added by one Engram migration; the personal-life corpus stays
  untouched under `corpus_id='personal'`.
- An operator-triggered pull-mode ingest path: `striatum corpus
  export --since <ref>` produces a JSONL bundle on disk; `engram
  ingest-striatum --repo <path>` consumes it.
- A standalone `engram-mcp-stdio` server living in Engram's
  repository, speaking MCP stdio, binding loopback only, with four
  read-only tools (`engram.search`, `engram.fetch_reference`,
  `engram.describe_corpus`, `engram.health`).
- Four Engram-local capabilities (`memory.read_striatum`,
  `memory.describe`, `memory.read_personal`,
  `memory.read_cross_corpus`); Striatum's daemon RPC capability set
  is unchanged.
- Striatum-side wiring limited to one skill (`striatum-engram`), one
  read-only check verb (`striatum operator memory check`), and one
  export verb (`striatum corpus export`). No `workflow.json` field,
  no daemon RPC method, no chat tool.
- Mechanically enforced augmentation-not-dependency: zero Engram
  imports under `src/striatum/cli/`, zero `memory.*` daemon RPC
  capabilities, and an "Engram off" acceptance test producing
  identical Striatum artifacts.

The plan is concrete enough to author RFC 0044 against without
returning to the three design proposals. The implementer is codex.
