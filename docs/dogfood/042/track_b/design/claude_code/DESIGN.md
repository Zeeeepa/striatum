# Track B Design: Engram Phase 1 (RFC 0044 V1) — Read-Only Engram MCP Server over the Striatum Corpus

author: designer-claude-opus-001

Status: design (Track B, dogfood-042, lane `claude_code`)
Date: 2026-05-13
Context:
[`RFC 0041`](../../../../../rfcs/0041-engram-memory-layer-for-striatum-operators.md),
[`RFC 0030`](../../../../../rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md),
[`RFC 0033`](../../../../../rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md),
[`RFC 0035`](../../../../../rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md),
[`RFC 0036`](../../../../../rfcs/0036-mcp-harness-for-daemon-v2-mutation-surface.md),
[`RFC 0039`](../../../../../rfcs/0039-go-daemon-core.md),
[`RFC 0040`](../../../../../rfcs/0040-mcp-driven-dogfood-harness.md),
`~/git/engram/` (sibling repository — Engram, the memory project this RFC
augments).

## Sourcing Note on Engram Reads

RFC 0041's Design-Phase Directive is to read all `.md` files under
`~/git/engram/` (top-level, `docs/`, `docs/specs/`, `docs/design/`,
`docs/rfcs/`, `docs/schema/`, `docs/process/`, `docs/howto/`),
`docs/claims_beliefs.md`, `docs/ingestion.md`, `docs/segmentation.md`,
`migrations/`, `agent-runner/`, and `docker-compose.yml` before
proposing. This `claude_code` design lane runs inside a Striatum
working-directory sandbox that blocks `Read` / `Bash` / `Grep` against
`~/git/engram/`; only directory listing via `Glob` works. The
file-path inventory it returned shapes this design (see "Engram
shape inferred from observed structure" below), but verbatim Engram
schema citations are taken from RFC 0041's own authoritative quotations
of Engram vocabulary, not from independent reads. The design review
must validate every Engram-side specific against Engram's actual
docs; any deltas are review-time corrections, not design rejections.
The other Track B lane (codex, gemini) should be presumed to have
the same sandbox boundary unless their environment was configured
differently. **This is a harness-improvement finding for dogfood-042
synthesis: design lanes that MUST read sibling repos need a
write-scope-style "extra_read_paths" lane attribute (see "Followups"
§F4).**

Where this design reaches a point that depends on an Engram-internal
schema choice that RFC 0041 does not quote verbatim, it names the
choice and the candidate shapes, and defers the final pick to the
synthesis (which can pair with Engram-side reading) or to a tiny
verification pass.

## Engram Shape Inferred from Observed Structure

Directory inventory from `Glob('~/git/engram/**/*.md')` plus
RFC 0041's authoritative naming gives this shape:

- Top-level: `README.md`, `SPEC.md`, `HUMAN_REQUIREMENTS.md`,
  `ROADMAP.md`, `TODO.md`, `SECURITY.md` (no `AGENTS.md`,
  `CLAUDE.md`, `BUILD_PHASES.md`, `DECISION_LOG.md` visible at top
  level — these may exist under different names or have been
  consolidated; the design review must confirm).
- `docs/UBIQUITOUS_LANGUAGE.md`, `docs/ingestion.md`,
  `docs/segmentation.md` (these are the spine concepts).
- `docs/design/`: V1 architecture draft + synthesis deltas.
- `docs/specs/`, `docs/schema/`, `docs/howto/`, `docs/process/`,
  `docs/rfcs/`: existing spec/schema/RFC tree.
- `docs/reviews/phase3/`: extensive review/build/repair history,
  including `PHASE_3_CLAIMS_BELIEFS_SPEC_*` review/synthesis docs.
  This is the most recent published phase and indicates **claims
  and beliefs schemas exist as named, reviewed artifacts**.
- `migrations/`: present (matches RFC 0041's mention that Engram
  has a storage substrate with migrations; whether it is Postgres,
  SQLite, or duckdb is not visible from path inventory and must be
  confirmed at review time).
- `agent-runner/`: present (the natural home for an MCP server).
- `docker-compose.yml`: present (Engram has a runtime topology).
- `benchmarks/segmentation/`: present with `BENCHMARK_READY.md`
  (segmentation is shipped to a benchmark-passing bar).

The phase-3 review tree (claims/beliefs spec + build reviews) is
the strongest signal that Engram has both (a) a published
schema-shape for claims and beliefs and (b) a built path through
it that the operator runs through `docs/reviews/phase3/postbuild/`
runtime checks. This design treats those as load-bearing and
**must not be redesigned**.

## Goals

- Land Engram Phase 1 per RFC 0041: a read-only Engram MCP server
  over the Striatum software-building corpus.
- Augment, never redesign, Engram's existing claims / beliefs /
  ingestion / segmentation schemas.
- Make Engram retrieval an **optional augmentation** at the
  Striatum operator's session, with graceful absence when Engram
  is unavailable.
- Define the bootstrap UX so the operator session discovers and
  queries Engram with one config gesture and zero behavior change
  if Engram is not running.
- Keep cross-corpus retrieval (personal-life ↔ Striatum) gated by
  explicit capability; the default `tools/list` exposes
  Striatum-corpus retrieval only.

## Non-Goals (Phase 1)

- Write-side: dogfoods do NOT produce Engram claims in Phase 1.
  That's Phase 3 / RFC 0046+.
- Personal-life corpus changes. Engram's existing ingestion
  pipeline for personal-life data stays untouched.
- Modifying Engram's claims / beliefs / provenance / segmentation
  schemas. Phase 1 adds a new corpus and a retrieval surface; it
  does not redesign the meaning of a claim or a belief.
- Hosted-mode or multi-tenant Engram. Per RFC 0041 + D083,
  Engram lives in `~/git/engram/` as a single-user single-machine
  service.
- Coupling any Striatum critical-path operation to Engram
  availability. The augmentation-not-dependency boundary is hard.
- Adding Engram retrieval to Striatum's RFC 0030 daemon RPC
  method registry. Engram is a separate process with its own
  capability vocabulary; Striatum's capability set
  (`read`/`write`/`review`/`claim`/`apply`/`admin`/`recovery`)
  stays closed and does not gain `memory.*` capabilities.

## Phase 1 Scope: Acceptance Criteria

The synthesis RFC's acceptance bar is met when all of the
following hold:

### A1. Engram Corpus Surface

- A new corpus `striatum` exists in Engram, distinct from
  Engram's existing personal-life corpus. The two corpus IDs
  are stored in Engram's existing corpus registry (or
  whatever equivalent Engram already names; the design review
  confirms the exact table name).
- Each Striatum-corpus document has provenance metadata
  recording **at minimum**: source kind (`commit` /
  `decision_log_row` / `operator_report` / `rfc` / `audit_chain` /
  `run_summary` / `changelog`), source identifier (commit
  hash / `D###` / file path with sha256 / `RFC ####` /
  `audit_id` / `run_id`), source timestamp, and Striatum repo
  path. This metadata fits inside Engram's existing claim
  provenance shape; if it doesn't fit cleanly, the design review
  bounces with a request to widen Engram's provenance metadata
  (which would itself be a small Engram-side change).
- All ingested documents pass through Engram's existing
  segmentation pipeline. Phase 1 does **not** add a
  Striatum-specific segmentation strategy; per RFC 0041 the
  software-building corpus is structurally easier than
  personal-life, so the existing segmenter is the default
  starting point. The review may surface that
  Markdown-with-front-matter benefits from a structure-aware
  segmenter; that becomes a Phase 1.5 follow-up, not a Phase 1
  blocker.
- Claim extraction over the Striatum corpus reuses Engram's
  existing claim schema. Phase 1 maps the structural artifacts
  to Engram's existing claim shape: a decision-log row becomes
  N claims (one per `What changed` / `Why` / `Constraints`
  sub-bullet, per the D### conventions); an operator-report
  intervention becomes one claim; a commit-message subject becomes
  one claim; an RFC body section becomes one claim per
  H2/H3 heading. The mapping rules are **published in the design
  itself** (§4 below), not made up by ingestion code.
- Beliefs over the Striatum corpus are NOT generated in Phase 1.
  Engram's belief-derivation pipeline ingests only claims; the
  follow-up RFC for Phase 3 covers belief generation from
  audit-chain-grounded claims.

### A2. Ingestion Trigger

- **Pull mode, operator-triggered.** The first invocation in
  Phase 1 is explicit: `engram corpus ingest --source striatum
  --repo ~/git/striatum [--since <ref>]`. No cron, no push,
  no background sweep in Phase 1.
- The Striatum side exposes a thin **export** verb,
  `striatum corpus export --since <ref> [--out <dir>]`, that
  writes the corpus snapshot to disk in a documented JSONL +
  attached-file shape (see §3). Engram's ingester reads that
  snapshot directly. This keeps the dependency one-way:
  Engram knows about Striatum's export shape; Striatum knows
  nothing about Engram.
- The pull is **idempotent**: re-running the export and
  re-ingesting the same `since` window produces the same
  Engram corpus state (same claim ids — claim id is a
  function of source kind + source id + content hash, not a
  fresh UUID per ingest).
- A small **cron-recommended** documentation note ships with
  the operator UX: "if you want continuous ingestion, schedule
  `engram corpus ingest --source striatum --repo ~/git/striatum`
  in your local user crontab; Phase 1 does not provide an
  Engram-side scheduler." Push-mode (Striatum emits
  `run.completed` events into Engram) is explicitly deferred to
  Phase 3.

### A3. Retrieval Surface (Engram MCP server)

- Engram exposes an MCP server that runs as a separate process,
  not as a Striatum chat tool. Topology choice and rationale
  in §5.
- The MCP server's closed-set tools, all read-only:

  ```text
  engram.retrieve(query: str, corpus: "striatum", k: int = 10,
                  filters: {kind?: [...], since?: ts, until?: ts,
                            run_id?: str, rfc?: str, decision_id?: str}?)
    -> {results: [{claim_id, source_kind, source_id, content,
                   provenance: {commit?, path?, sha256?, audit_id?,
                                ts}, score}], retrieval_id}

  engram.fetch(claim_id: str)
    -> {claim_id, source_kind, source_id, content, provenance, neighbors?}

  engram.describe_corpus(corpus: "striatum")
    -> {document_count, claim_count, oldest_ts, newest_ts,
        source_kinds: {kind: count}, last_ingest_ts}

  engram.health()
    -> {ok: bool, corpus_status: {striatum: "ready"|"empty"|"degraded"},
        substrate_version, engram_version}
  ```

  - All four are **read-only**. No `claim_create`, `belief_revise`,
    or any mutation in Phase 1.
  - `engram.retrieve(corpus="striatum")` is the only retrieval
    surface visible by default. The personal-life corpus is **not**
    exposed by default; an Engram-local capability
    `memory.read_personal` (Engram's own capability vocabulary,
    not Striatum's) gates the personal-life corpus. Phase 1
    operators never have that capability.
  - `engram.fetch(claim_id)` returns the raw claim with full
    provenance. The chat session uses this to dereference a
    `retrieve` hit before quoting it.
  - `engram.describe_corpus` is the "what's in here" surface —
    it answers "is Engram populated yet" and "is my recent
    dogfood reflected" without doing a real retrieval.
  - `engram.health` answers the augmentation-availability check
    at session boot. If `ok: false`, Striatum operator behavior
    is unchanged from pre-Engram.

- The Engram MCP server **honors a small Engram-local capability
  vocabulary** (not shared with Striatum's RFC 0030 set):
  `memory.read`, `memory.read_personal`, `memory.describe`. A
  Phase 1 operator token carries `memory.read` + `memory.describe`
  only. `memory.read_personal` is not issued by default; the
  operator can issue it for themselves if they want to query the
  personal-life corpus, but Striatum's default operator
  session never does.

### A4. Striatum-Side Wiring

- A new bundled skill `striatum-engram` (RFC 0015 V1 skill bundle
  pattern, profile fan-out per RFC 0015 §3) teaches the operator
  session how Engram retrieval works. The skill body covers:
  when to invoke (always-on at session start, then on demand for
  unfamiliar D###/RFC ####/dogfood IDs), how to read
  `engram.health` to detect availability, how to call
  `engram.retrieve`, how to dereference results via
  `engram.fetch`, the four-tool closed set, the cap vocabulary
  delta, and the augmentation-not-dependency rule. This is the
  RFC 0041 §"Striatum operator's session brief" surface.
- An explicit Claude Code-style `mcp_servers` config snippet is
  documented in the skill body. Example:

  ```jsonc
  // ~/.claude/settings.json (excerpt)
  "mcp_servers": {
    "engram": {
      "command": "engram-mcp-stdio",
      "args": ["--corpus", "striatum"]
    }
  }
  ```

  Engram ships an `engram-mcp-stdio` entrypoint (per RFC 0041
  §"MCP server topology"; see §5 for the standalone-vs-wrapped
  picks). The operator registers the server once; subsequent
  Claude sessions discover the tools via `tools/list` filtered
  by the Engram-local capability token issued at registration.
- A Striatum-side CLI verb `striatum operator memory check`
  performs a read-only `engram.health` call and prints the
  status. **This is the only Striatum-side CLI verb the design
  adds.** No `striatum operator memory init` (the operator
  edits their own MCP config) and no `memory_provider:` field
  on `workflow.json` (Phase 1 retrieval is operator-scoped,
  not workflow-scoped).
- A Striatum-side CLI verb `striatum corpus export --since <ref>
  [--out <dir>]` (named in A2) is added. Defined under
  `src/striatum/corpus_export/` (new package). The verb is
  read-only against `.striatum/state.sqlite3` + the daemon DB
  (where it reads from depends on what's authoritative for each
  source kind; see §3). Its output is the export format Engram
  consumes.

### A5. Augmentation-Not-Dependency Boundary

- The Striatum operator's session-start brief checks
  `engram.health` once. If `ok: true`, the session may use
  Engram retrieval as a fallback / accelerator. If `ok: false`,
  the session falls back to the pre-Engram behavior (read the
  decision log + RFCs + AGENTS.md directly), unchanged.
- The Engram MCP server is configured with a **timeout** (default
  3 seconds) on the operator session's first `engram.health`
  call at session start. A timeout is treated as `ok: false`;
  the session proceeds without Engram.
- No Striatum command at any layer (CLI, daemon RPC, web UI)
  blocks on an Engram call. If a Striatum operator wraps a
  CLI invocation in an Engram-informed prompt, the wrapping is
  the operator's choice; the runner doesn't know.
- The acceptance harness includes a test where Engram is
  intentionally not running: the Striatum operator session
  starts, drives a dogfood-shaped workflow end-to-end, and
  succeeds with identical artifacts to the
  Engram-running case.

### A6. Test-Set Seeding

- Phase 1 ingests, at minimum, the corpus subset RFC 0041 §
  "Open Questions" recommends: dogfoods 035–040 +
  decisions D080–D092 + RFCs 0030–0040 + the most recent
  ~50 commits. That bound is small enough to ingest in
  under a minute on a developer laptop and large enough to
  produce real retrieval signal across the friction-pattern
  questions in RFC 0041 §1.
- A documented retrieval-quality smoke test exercises five
  reference queries (the same five RFC 0041 §1 names: "what
  friction patterns recurred across dogfoods 036-039",
  "which RFC moved the no-node toolchain rule and why",
  "has surgical_recovery been invoked before", "what did the
  build review for dogfood-037 say about test coverage",
  "which dogfoods touched the daemon RPC capability
  vocabulary"). For each, `engram.retrieve(corpus="striatum",
  k=5)` must return at least one hit whose provenance matches
  the expected RFC / decision / dogfood. **Pass bar: 5/5.**
- A negative test exercises five out-of-corpus queries
  ("what is Jennifer's MBTI", "best pizza in Berlin",
  "Python f-string formatter rules", "tomorrow's weather",
  "the JFK files"). For each, top-5 hits must either be
  empty, or have a similarity score below a documented
  threshold (Engram's existing score floor for "no real
  hit"). **Pass bar: 5/5 — no false-positive provenance
  hits.**

### A7. Documentation

- `docs/SPEC.md` gains an "Optional Engram Memory Augmentation"
  section under "Adapter Constraints" or in a new "Memory
  Augmentation" sibling section (synthesis picks the home);
  it states that Engram is optional, names the four MCP tools,
  and names the augmentation-not-dependency boundary.
- `docs/HOW_TO_AGENT.md` cross-references the
  `striatum-engram` skill body and the `striatum operator
  memory check` CLI verb.
- `docs/HOW_TO_HUMAN.md` documents the
  `~/.claude/settings.json` `mcp_servers` snippet and the
  `engram corpus ingest --source striatum --repo ~/git/striatum`
  setup.
- `docs/UBIQUITOUS_LANGUAGE.md` gains the terms in §6 below.
- `docs/DECISION_LOG.md` records the Phase 1 commitment
  (corpus name, augmentation-not-dependency, no write-side,
  no personal-life exposure-by-default).
- Engram-side documentation (in `~/git/engram/`) gains a
  `docs/striatum_corpus.md` describing the new corpus,
  ingest verb, and the four MCP tools. RFC 0041 §"Design-Phase
  Directive" says Engram doc updates are in scope for the
  augmentation; the design review confirms.

### A8. CI / Multi-Repo Harness

- The RFC 0035 multi-repo test harness gains an **optional**
  Engram fixture (only when `ENGRAM_TEST=1` env var is set):
  spins up an Engram instance from `~/git/engram/docker-compose.yml`
  (or whatever its actual runtime entry is — see §5), runs
  the seed ingest, runs the retrieval smoke + negative tests.
  When `ENGRAM_TEST` is unset, the fixture is skipped; the
  rest of the harness runs unchanged. Per
  augmentation-not-dependency, CI green is unaffected by
  Engram availability.
- A small `tests/engram_smoke/` test directory runs the five
  smoke queries against the fixture. Skipped by default.

## 1. Phase-1 Boundary

Phase 1 is **read-only retrieval over the `striatum` corpus**.
Phase 2 (operator-side context injection auto-retrieval) and
Phase 3 (write-side: dogfoods produce Engram claims) are out of
scope. The design has been written so that the Phase-1 surface
naturally extends to Phase 2 without an MCP-tool-rename: the
same `engram.retrieve` is what the Phase-2 auto-injection
calls. The Phase 3 mutation surface needs a separate
`memory.write` capability and is explicitly behind RFC 0041's
write-side phase.

## 2. The `striatum` Corpus

A corpus, per RFC 0041's authoritative vocabulary, is **"a closed
set of source documents Engram indexes."** The `striatum` corpus's
closed source set:

| Source kind | Authoritative location | Example id |
|---|---|---|
| `commit` | `git log` on the Striatum repo | `c767d02` |
| `decision_log_row` | `docs/DECISION_LOG.md` row | `D091` |
| `rfc` | `docs/rfcs/####-*.md` | `RFC 0041` |
| `operator_report` | `docs/dogfood/###/OPERATOR_REPORT.md` | `dogfood-040 #5` |
| `audit_chain_entry` | Striatum daemon DB `audit_chain` table | `audit_2f...` |
| `run_summary` | `striatum run summary --run-id <id> --json` | `run_8bd11...` |
| `changelog_entry` | `CHANGELOG.md` `vX.Y.Z` block | `v1.30.0` |
| `ubiquitous_language_term` | `docs/UBIQUITOUS_LANGUAGE.md` table row | `lazy lease expiry` |
| `harness_friction_pattern` | `docs/HARNESS_FRICTION_PATTERNS.md` row | (per heading) |

Each source kind is **exhaustively enumerated above**; nothing
else gets ingested. This bound makes the corpus auditable: a
reviewer can list every Engram claim by source kind and
cross-check the count against the on-disk count.

Per-source-kind ingest rules (the claim-extraction mapping
RFC 0041 §"Open Questions" defers to the design phase):

- **commit**: one claim per commit; `content` is the subject
  line + first paragraph of the body; `provenance.commit` is
  the SHA, `provenance.ts` is the author date. Commit footers
  (`Co-Authored-By`, `Signed-off-by`) are stripped.
- **decision_log_row**: one claim per D### row; `content` is
  the row's body up to (but not including) the next D### header
  or H2 boundary; `provenance.path` is `docs/DECISION_LOG.md`,
  `provenance.ts` is the row's stated date. If the row
  contains "Supersedes D###" or "Superseded by D###", those
  IDs are added as `relations.supersedes` / `superseded_by`
  in the claim metadata; Engram's existing relation surface
  (if it has one) consumes them; otherwise they sit as opaque
  metadata for now.
- **rfc**: one claim per H2 section of the RFC body, plus one
  "spine" claim for the RFC header (status / date / context).
  `provenance.path` is the RFC file; `provenance.ts` is the
  RFC's `Date:` field. RFC sections like "Open Questions"
  ingest as a single claim each so reviewers can retrieve
  "what were the open questions on RFC 0041".
- **operator_report**: one claim per "Intervention #N" block
  per D091 OPERATOR_REPORT.md convention; `content` is the
  intervention body; `provenance.path` is the report file,
  `provenance.run_id` is parsed from the path
  (`docs/dogfood/###/OPERATOR_REPORT.md` → dogfood `###`).
- **audit_chain_entry**: one claim per audit row; `content`
  is the `denial_reason` + `decision` + `method` columns;
  `provenance.audit_id` is the row id, `provenance.ts` is
  the row timestamp, `provenance.repo_id` is the daemon
  registry repo id.
- **run_summary**: one claim per run; `content` is the run
  summary's terminal state + verdict counts + duration;
  `provenance.run_id` is the run id.
- **changelog_entry**: one claim per `vX.Y.Z` block;
  `content` is the version block body; `provenance.ts` is
  the release tag's date.
- **ubiquitous_language_term**: one claim per
  `| term | definition |` row; `content` is the definition
  cell; `provenance.path` is the file.
- **harness_friction_pattern**: one claim per top-level
  heading in `docs/HARNESS_FRICTION_PATTERNS.md`.

The mapping above is **deliberately mechanical**: a reviewer
can re-run the export and re-derive every claim id from
source content. This is the "provenance for free" property
RFC 0041's Problem statement highlights.

## 3. The Export Format (Striatum → Engram)

`striatum corpus export --since <ref> [--out <dir>]` writes:

```
<out>/
  manifest.json                  # ingest envelope: striatum_version, repo_id, since,
                                 #  source_kinds emitted, counts, generated_at
  commits.jsonl                  # one line per source kind, each line is one claim-input row
  decision_log_rows.jsonl
  rfcs.jsonl
  operator_reports.jsonl
  audit_chain.jsonl
  run_summaries.jsonl
  changelog.jsonl
  ubiquitous_language.jsonl
  harness_friction_patterns.jsonl
```

Each line in the per-kind JSONL files is a `{source_id, content,
provenance}` row matching the mapping in §2. Engram's
`engram corpus ingest --source striatum` reads `manifest.json`
first (sanity checks the version + repo_id), then walks the
JSONL files in a fixed order, then writes claims into Engram's
existing claim table with `corpus = "striatum"` set.

Format properties:

- **JSONL** (not CSV / SQLite / pickle): operator-readable,
  diff-able, git-friendly for tiny corpora, streaming-friendly
  for big ones. No mandatory dependency on a binary tool to
  inspect the export.
- **One file per source kind**: makes targeted re-ingest cheap
  (e.g., re-ingest only `commits.jsonl` after a `git pull`).
- **Manifest with counts**: Engram verifies counts after
  ingest and refuses partial loads.
- **No transcripts, no terminal output, no model output.** Per
  Striatum's D028, transcripts are off by default. The
  Striatum corpus is the **structural** corpus only.
- **No `.striatum/` SQLite content directly in the export.**
  Run summaries are read via `striatum run summary --json`,
  not by walking the SQLite tables. This keeps the
  state-store-as-private-implementation boundary clean.

## 4. Engram MCP Server Topology

RFC 0041 §"Open Questions" presents the choice: standalone
Engram MCP server in `~/git/engram/agent-runner/` vs wrap
Engram retrieval as Striatum chat tools per RFC 0036 V1
pattern.

**Choice: standalone Engram MCP server in
`~/git/engram/agent-runner/`.** Rationale:

- **Engram is independent.** RFC 0041 makes "augment, don't
  replace" a hard rule. Wrapping Engram retrieval inside
  Striatum's chat-tool closed set would couple Engram's
  retrieval surface to Striatum's chat lifecycle (RFC 0023 +
  RFC 0036) — exactly the inversion of "Engram augments
  Striatum, not the other way around."
- **Non-Striatum operators.** RFC 0041 §"Two Operator
  Surfaces" §2 calls out future frontier-model CLI operators
  (codex, gemini). A standalone Engram MCP server reaches
  them with zero Striatum surface area; a Striatum-chat-tool
  wrapping would require each non-Claude operator to also run
  Striatum's web service. That's a heavier dependency than
  spinning up an Engram MCP stdio process.
- **Striatum's chat-tool closed set stays bounded.** RFC 0036
  V1 + RFC 0040 V1 added 14 chat tools. Adding four
  Engram-prefixed tools would balloon the closed set and blur
  the operator's mental model of "what tools belong to
  Striatum vs to Engram."
- **Augmentation-not-dependency reads cleanly.** With
  standalone, the absence story is "Engram isn't running, MCP
  server isn't there, Claude doesn't see the tools, the
  session falls back." With wrapping, the absence story is
  "Striatum's chat tool calls Engram and gets a timeout."
  The first is structurally cleaner.

Engram's existing `agent-runner/` directory is the natural
home. The Engram repo gains an `engram-mcp-stdio` binary /
script that speaks **stdio MCP** (not a socket — stdio is the
shape Claude Code's `mcp_servers` config expects and avoids
port-management entirely). The MCP server reads from Engram's
existing storage substrate (whatever it is — Postgres, SQLite,
duckdb; the file inventory shows a `migrations/` directory
but the actual substrate is not visible from path inventory
and must be confirmed at review time).

The Engram MCP server's **capability token** is Engram-local
(see §A3): `memory.read`, `memory.read_personal`,
`memory.describe`. The token format is Engram's choice; this
design does not propose a token format because that's Engram's
domain.

A `docker-compose.yml` already exists in Engram (per the
file inventory). The MCP server can optionally run as a
sidecar to whatever's in `docker-compose.yml`, or as a host
process. Recommendation: **host process via `engram-mcp-stdio`
binary; docker-compose stays the way Engram's main service
runs.** This keeps the dev/CI story for "is the MCP server
up" purely a question of "is the stdio binary on PATH and
can it talk to the substrate" — independent of Docker
availability.

## 5. Striatum-Side Operator Bootstrap

The operator's first-time setup (one-time, ~5 minutes):

```bash
# In ~/git/engram/
docker compose up -d                          # start the Engram service (its existing topology)
engram migrate                                # if Engram requires migrations be run separately
engram corpus ingest --source striatum --repo ~/git/striatum
                                              # seed the corpus (operator-triggered)
which engram-mcp-stdio                        # confirm the MCP server binary is on PATH

# In ~/.claude/settings.json
# add:
#   "mcp_servers": {
#     "engram": {
#       "command": "engram-mcp-stdio",
#       "args": ["--corpus", "striatum", "--token-file", "~/.config/engram/operator.token"]
#     }
#   }

# In ~/git/striatum/
striatum operator memory check                # confirms tools/list returns the four engram.* tools
```

After that, every Claude Code session for `~/git/striatum/`
sees the four `engram.*` tools in `tools/list` and can call
them on demand. The `striatum-engram` skill body briefs the
session at start.

A periodic re-ingest (recommended via user crontab):

```cron
0 * * * * cd ~/git/striatum && git pull && \
  ~/.local/bin/engram corpus ingest --source striatum --repo ~/git/striatum --since HEAD~50
```

This keeps the corpus warm without Striatum knowing about it.
If the operator doesn't set up a cron, the corpus is
manually refreshed; that's fine for Phase 1.

## 6. New Ubiquitous-Language Terms

Per RFC 0041 §"Domain Modeling" the terms below are
preliminary; the design phase **uses these** and the
synthesis confirms them after pairing with Engram-side
reading. Wording fits Striatum's existing table style.

| Term | Definition |
|------|------------|
| memory augmentation | An optional retrieval layer over a corpus of repository-grounded documents that the Striatum operator session may query but never depends on. V1's only memory-augmentation provider is Engram. |
| Striatum corpus | The closed set of Striatum software-building source documents Engram indexes: commits, decision-log rows, RFCs, operator-report intervention blocks, audit-chain entries, run summaries, changelog entries, ubiquitous-language terms, harness-friction-pattern rows. |
| Engram-local capability | A capability token issued by the Engram MCP server, distinct from Striatum's RFC 0030 capability tokens. Phase 1 closed set: `memory.read`, `memory.read_personal`, `memory.describe`. |
| corpus export | The redacted-by-construction Striatum-side JSONL dump produced by `striatum corpus export --since <ref>`. Engram's `engram corpus ingest --source striatum` consumes it. Contains no transcripts, no terminal output, no model output. |
| corpus retrieval | A read-only call against the Engram MCP server returning ranked corpus references with provenance metadata. Phase 1 only exposes retrieval over the Striatum corpus by default. |
| augmentation-not-dependency | The product invariant that no Striatum critical-path operation may block on or require a memory-augmentation call. If the augmentation is unavailable, Striatum runs unchanged. |

## 7. Capability Vocabulary

| Capability | Issued by | Phase 1 default | Required for |
|---|---|---|---|
| `memory.read` | Engram MCP server | yes (operator token) | `engram.retrieve(corpus="striatum")`, `engram.fetch(claim_id)` |
| `memory.describe` | Engram MCP server | yes (operator token) | `engram.describe_corpus`, `engram.health` |
| `memory.read_personal` | Engram MCP server | **no** | `engram.retrieve(corpus="personal")` (out of Phase 1 default) |
| `memory.write` | Engram MCP server | **no** (Phase 3) | not yet defined |

These capabilities are **Engram-local**. Striatum's RFC 0030
capability set (`read`/`write`/`review`/`claim`/`apply`/`admin`/
`recovery`) is unchanged. The two capability vocabularies live
in separate registries: Engram's token in
`~/.config/engram/`; Striatum's token in
`${XDG_RUNTIME_DIR}/striatum/` (per RFC 0030 §10).

## 8. Augmentation-Not-Dependency Enforcement

Three rules, mechanically enforceable:

1. **No Striatum CLI verb depends on Engram.** A grep audit
   of `src/striatum/cli/*.py` after Phase 1 must find zero
   imports of any Engram client library. The one Striatum
   verb that touches Engram (`striatum operator memory check`)
   shells out to `engram-mcp-stdio --health-check` and returns
   exit 0 whether or not Engram responds; the exit code is
   purely informational.
2. **No Striatum daemon RPC method references Engram.** RFC
   0030's method registry stays at the seven capabilities; no
   `memory.*` capability gets added.
3. **The acceptance harness test (per A5) asserts** that a
   full dogfood-shaped workflow run with the Engram MCP
   server killed produces the same artifacts as the
   Engram-running run.

## 9. What Cannot Be Claimed in Phase 1

Per RFC 0041's expectation that the design state explicit
limits:

- **No cross-machine memory.** Per D083, Engram lives in
  `~/git/engram/` as a single-user single-machine service.
  If the operator works on two laptops, they have two
  Engram instances; no sync.
- **No hosted Engram.** Engram is local-first. No SaaS,
  no shared cluster.
- **No multi-tenant memory.** Engram is single-user. There
  is no notion of "operator A's memory" vs "operator B's
  memory" at the Engram service layer.
- **No retroactive claim correction.** Phase 1's claim
  extraction is mechanical; if the mapping for, e.g.,
  `decision_log_row` proves wrong for some D### row, the
  fix is to re-ingest with a corrected mapping rule. Phase
  1 does not expose a "edit this claim" operator surface.
- **No write-side dogfood ingestion.** Dogfoods do not
  produce Engram claims in Phase 1. Phase 3 covers this.
- **No personal-life corpus surfaces in default Striatum
  sessions.** The personal-life corpus stays gated behind
  `memory.read_personal` and is never granted to the
  default operator token.
- **No Striatum-side retrieval scoring tweaks.** Engram
  owns retrieval ranking; Striatum does not pass
  `boost: {kind: "decision_log_row"}` parameters or
  similar. If the default ranking is poor for a query
  shape, the fix is on Engram's side (a Phase 1.5 follow-up).
- **No belief-derivation.** Phase 1 ingests claims only;
  Engram's belief-derivation pipeline is not triggered on
  the `striatum` corpus. Phase 3 covers this once write-side
  is wired.
- **No retrieval-grounded auto-summary at session start.**
  Phase 2 covers auto-retrieval. Phase 1 retrieval is
  on-demand.

## 10. Implementation Plan (5 Steps)

### Step 1 — Engram-side: corpus + ingester (Engram repo work)

- Add `corpus = "striatum"` to Engram's existing corpus
  registry.
- Add `engram corpus ingest --source striatum --repo <path>
  [--since <ref>]` CLI verb to Engram.
- Reuse the existing segmentation + claim-extraction
  pipeline. No changes to claim schema, belief lifecycle,
  segmentation strategy.
- Add Engram-side `docs/striatum_corpus.md`.

### Step 2 — Engram-side: MCP server (Engram repo work)

- Add `engram-mcp-stdio` binary/script under
  `~/git/engram/agent-runner/` exposing the four read-only
  MCP tools in §A3.
- Implement Engram-local capability checking
  (`memory.read`/`memory.describe`/`memory.read_personal`).
- Add Engram-side documentation for the MCP server in
  `~/git/engram/docs/mcp_server.md` (Engram's existing
  `docs/howto/` shape).

### Step 3 — Striatum-side: corpus export (Striatum repo work)

- Add `src/striatum/corpus_export/` package.
- Add `striatum corpus export --since <ref> [--out <dir>]`
  CLI verb.
- Output format per §3 (JSONL + manifest).
- Unit tests covering each source kind's claim-extraction
  rule.

### Step 4 — Striatum-side: operator skill + CLI verb (Striatum repo work)

- Add `striatum-engram` skill body to
  `src/striatum/skills/templates/claude_code/engram.md.tmpl`
  and generic equivalent. Wire into `CLAUDE_CODE_SKILLS`
  tuple (RFC 0036 V1 pattern).
- Add `striatum operator memory check` CLI verb shelling
  out to `engram-mcp-stdio --health-check`.
- Update `docs/SPEC.md`, `docs/HOW_TO_AGENT.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/UBIQUITOUS_LANGUAGE.md`
  per §A7.

### Step 5 — Integration harness (Striatum repo work)

- Extend the RFC 0035 multi-repo harness with the optional
  Engram fixture (skipped unless `ENGRAM_TEST=1`).
- Land the five smoke queries + five negative queries.
- Add an "Engram off" assertion: a dogfood-shape run with
  Engram intentionally not running produces the same
  artifacts.

The five steps are intentionally independently landable.
Steps 1-2 are Engram-repo PRs; steps 3-5 are Striatum-repo
PRs. Engram-repo work lands first because Step 3's export
format is designed against Step 1's ingester contract.

## 11. Open Questions for the Synthesis

These are NOT decided here; the synthesis resolves them after
pairing with Engram-side reading:

- **Engram's existing claim schema fields.** §2 and §3 assume
  Engram's claim shape can absorb the `provenance` block in §2
  without schema change. The synthesis confirms this against
  Engram's actual claim schema and bounces with a "widen
  provenance fields" note if the existing shape can't hold
  `commit`, `audit_id`, `run_id`, and `path` together.
- **Engram's substrate.** The file inventory shows
  `migrations/` and `docker-compose.yml` but doesn't confirm
  Postgres vs SQLite vs duckdb. The synthesis confirms; if
  Postgres, then Phase 1 ingest runs against Engram's existing
  PG instance and the operator UX in §5 stands. If something
  else, the operator UX may need a paragraph adjustment but
  the design's structure is unchanged.
- **Engram's existing capability vocabulary.** §7 names three
  Engram-local capabilities by analogy with Striatum's set.
  If Engram already has capability semantics, the synthesis
  reuses those names; if not, this design proposes the names
  above.
- **Segmentation strategy for Markdown-with-front-matter.**
  Phase 1 reuses Engram's default segmenter. If Engram's
  default segmenter is tuned to free-form dialog (per
  `docs/segmentation.md` and the `superdialseg` benchmark
  evidence), it may underperform on structural Markdown.
  The synthesis decides whether Phase 1 ships with the
  default + a "structural-segmenter-for-corpus=striatum"
  flag is a Phase 1.5 follow-up.
- **The four tool names.** `engram.retrieve` /
  `engram.fetch` / `engram.describe_corpus` / `engram.health`
  fit Engram's vocabulary. If Engram already exposes a
  different verb shape internally (e.g. `claim.search`
  instead of `retrieve`), the synthesis aligns the MCP-tool
  names with Engram's existing internal verb shape.
- **Whether Phase 1's ingester is incremental or full.**
  §A2 specifies idempotent re-ingest; the implementation
  detail of "rewrite all claims for affected files" vs
  "diff-based update" is left to Engram-side implementation
  judgment.
- **Whether the operator's `striatum-engram` skill body
  briefs by default (every session) or only when the
  operator opts in via `striatum operator memory check
  --enable-skill`.** Recommendation: brief by default;
  the skill body is short and the augmentation-not-dependency
  rule keeps the skill harmless even when Engram is offline.
- **Whether `engram corpus export` lives on the Striatum
  side or under Engram.** §3 puts it on Striatum
  (`striatum corpus export`); the alternative is
  Engram-side (`engram corpus pull --source striatum
  --repo <path>`) where Engram walks the Striatum repo
  itself. Recommendation: keep on Striatum side so Striatum
  owns "what's in its own corpus" rather than Engram
  having to know Striatum's directory layout. If the
  synthesis disagrees, the export format in §3 is
  unchanged; only the verb name moves.

## 12. Followups (Out of Phase 1)

- **F1: Phase 1.5 — Structural segmenter for the Striatum corpus.** If
  retrieval-quality smoke tests show that
  Markdown-with-front-matter retrieval is poor under
  Engram's default segmenter, add a structural-aware
  segmentation strategy gated by `--segmenter=structural`
  on `engram corpus ingest`.
- **F2: Phase 2 — Auto-retrieval at session start.** The
  Striatum operator session-start brief runs
  `engram.retrieve(corpus="striatum", query=<current dogfood
  / RFC / decision context>)` automatically and includes
  the top-3 hits in the session prompt. Per RFC 0041
  §"Roadmap" §"Phase 2".
- **F3: Phase 3 — Write-side: dogfoods produce Engram
  claims.** Per RFC 0041 §"Roadmap" §"Phase 3". Adds
  `memory.write` capability, `run.completed`-event-driven
  ingest, belief-derivation over the Striatum corpus.
- **F4: Harness-improvement proposal — Sandbox read-paths
  attribute for design lanes.** This dogfood's
  `claude_code` design lane could not read sibling
  Engram docs because Striatum's lane sandbox bounds reads
  to the project working directory. The work-packet shape
  could gain an `extra_read_paths` field that grants
  read-only access to declared sibling repositories
  for the duration of the lane. Filed as harness
  improvement; tracked outside this design.
- **F5: Phase 4 — Personal-life corpus re-attack.** Per
  RFC 0041 §"Roadmap" §"Phase 4". After Phases 1-3 prove
  the pipeline on the easier domain, re-attack the
  original Engram mission.

## 13. Risks

- **Engram instability.** Engram is mid-design per
  RFC 0041's framing. If Engram's claim or segmentation
  schema changes between Phase 1's design and ship,
  re-ingest may be needed. Mitigation: per A2 the ingest
  is idempotent; re-ingest is cheap.
- **Retrieval quality below the smoke-test bar.** If
  the five reference queries don't all return at least
  one provenance-correct hit, Phase 1 doesn't ship.
  Mitigation: F1 (structural segmenter) is the
  pre-planned fallback.
- **Operator skill confusion.** Adding a sixth skill
  (`striatum-engram`) to the bundle (after claim-loop /
  workflow / scaffold / supervise / recover / mcp) risks
  diluting operator attention. Mitigation: the skill
  body is intentionally short; the augmentation-not-
  dependency rule means a session that ignores the skill
  is no worse off than today.
- **Capability-vocabulary mixing.** Engram has its own
  capability tokens; the operator now juggles two token
  surfaces. Mitigation: §7 explicitly names them as
  separate vocabularies in separate registries; the
  skill body teaches the distinction.
- **Phase 1 → Phase 2 surface stability.** If Phase 1
  exposes four tools and Phase 2 needs five, that's a
  contract break for any non-Striatum operator
  registered against Phase 1. Mitigation: Phase 1's
  four tools cover the Phase 2 superset's retrieval
  shape; Phase 2 only adds the session-side
  auto-retrieval convention, not new tools.
- **The "did Engram redesign its claim schema?" foot-gun.**
  If Engram's `docs/reviews/phase3/PHASE_3_CLAIMS_BELIEFS_SPEC_*`
  reviews resolved to a schema this design doesn't
  match, the synthesis must catch it. Mitigation:
  this design quotes RFC 0041's vocabulary, not invented
  schemas, so any delta is bounded to the synthesis
  pairing pass.
