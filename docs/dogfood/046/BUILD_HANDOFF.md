author: implementer-codex-1

# Dogfood-046 Build Handoff — RFC 0044 V1 Striatum-side Corpus Export

Run: `run_7e1ea72b79024d1899e4f55c15cabc5f`
Branch: `striatum/dogfood-046-rfc-0044-v1`
Workflow: `docs/dogfood/046/workflow.json` — multi-track design + build +
review for RFC 0044 V1 Striatum-side corpus export

This handoff consolidates the per-finding implementation HANDOFF
(`docs/dogfood/046/build/HANDOFF.md`) with the three build review
verdicts. The per-finding HANDOFF remains authoritative for the
file-level inventory; this file is the operator-readable rollup
plus the verdict + override narrative.

## Scope

RFC 0044 V1 Striatum-side ONLY: the redacted JSONL corpus export
that Engram (a separate `~/git/engram/` project) consumes via its
own future `engram ingest-striatum` verb. The Engram-side
components — the ingester, the standalone `engram-mcp-stdio` MCP
server, the four read-only retrieval tools (`engram.search`,
`engram.fetch_reference`, `engram.describe_corpus`, `engram.health`),
the Engram-local `memory.*` capabilities — are explicitly **NOT**
in scope here and remain a separate follow-up effort. The
augmentation-not-dependency boundary is the load-bearing invariant:
Striatum runs with Engram absent.

**Implementer:** codex (Python work on `src/striatum/cli/`,
`src/striatum/corpus/`, and the corpus test surface). This is the
5th consecutive codex-as-implementer dogfood. Per dogfood-045's
deliberate codex-avoidance experiment for V1.5 web UI work, the
operator weighed using a different implementer here but the Python
exporter + git plumbing + redaction module fits codex's profile
naturally and the test surface gives the codex reviewer concrete
material to inspect. The trade-off was accepted up front: this run
was expected to surface a 5th codex/codex anti-pattern instance,
and it did.

## Shipped scope

The implementation matches the design synthesis
(`docs/dogfood/046/DESIGN_SYNTHESIS.md`) with no scope drift. From
the per-finding HANDOFF (`docs/dogfood/046/build/HANDOFF.md`):

- New top-level CLI verb
  `striatum corpus export --since <ref> --out <dir> [--json]` wired
  in `src/striatum/cli/parser.py` and dispatched in
  `src/striatum/cli/dispatch.py` (lazy import of
  `striatum.corpus.export_corpus_bundle` inside the dispatch
  branch).
- New `src/striatum/corpus/` package, split by concern:
  - `types.py` — `SUB_KINDS` / `JSONL_FILES` closed mapping for
    the fixed nine JSONL files; `CorpusBundleResult.to_json`
    emits `status="exported"`, repo-relative `manifest_path` /
    `out`, `since` (both ref and resolved commit), `row_counts`,
    `bundle_sha256`.
  - `git.py` — `resolve_commit(ref)` invokes
    `git rev-parse --verify <ref>^{commit}` and raises a
    `StriatumError` on failure that flows through the standard
    dispatch error-envelope path (exit 8 / `{ok: false, error:
    {message, code}}`).
  - `enumerator.py` — durable-provenance source enumeration over
    RFCs, decisions, commits, operator reports, the CHANGELOG,
    ubiquitous-language terms, harness-friction rows, and run
    summaries. Does NOT read SQLite blobs: there are no
    `FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions`
    queries against `.striatum/state.sqlite3`. Live run-summary
    provenance uses the sentinel path `<repo-local-state>` rather
    than the actual `.striatum/state.sqlite3` path to preserve the
    no-`.striatum/` export boundary while still identifying the
    source class.
  - `redaction.py` — denylist-based source-path refusal for `.env`,
    `.env.local`, `keys/private.pem`, `.striatum/state.sqlite3`,
    `transcripts/session.md`, `raw_model_output/out.log`,
    `docs/transcript.txt`; co-author-email + 64-char-token
    scrubbing on commit messages; `redact_evidence_payload`
    wrapper around the existing evidence redactor for run-summary
    snapshots. Audit-row payloads pass through the
    `test_audit_payload_keeps_metadata_only` test which scrubs
    free-text. Unknown evidence fields default to the standard
    `<redacted>` placeholder.
  - `writer.py` — deterministic JSONL emission with canonical UTF-8
    + newline normalization so per-file SHA-256 hashes are
    reproducible across runs.
  - `manifest.py` — `manifest.json` schema carries repo HEAD,
    dirty-tree flag, `since` ref + resolved commit, schema version,
    per-file SHA-256 + row counts, and `generated_at`. Hashes cover
    post-redaction bytes; manifest equality after stripping
    `generated_at` is the documented replay-stability surface.
  - `export.py` — orchestrator: refuses `--out` outside the repo,
    refuses `--out` under `.striatum/`, refuses `--out` pointing at
    a file rather than a directory, resolves `--since` before
    writing, verifies row counts + per-file SHA-256s after
    emission, returns the standard CLI JSON envelope.
- Tests:
  - `tests/test_corpus_enumerator.py` — per-source enumeration
    coverage.
  - `tests/test_corpus_redaction.py` — denylist parametrization
    (`.env`, `.env.local`, `keys/private.pem`,
    `.striatum/state.sqlite3`, `transcripts/session.md`,
    `raw_model_output/out.log`, `docs/transcript.txt`),
    commit-message co-author + token scrubbing, audit-row
    metadata-only scrub.
  - `tests/test_corpus_writer.py` — canonical-bytes determinism.
  - `tests/test_corpus_manifest.py` — manifest schema + hash
    coverage.
  - `tests/test_cli_corpus_export.py` —
    `test_corpus_export_cli_success_and_manifest` (real CLI
    subprocess invocation, asserts manifest path, JSONL presence,
    `status="exported"`),
    `test_corpus_export_invalid_since_returns_json_error_code_8`,
    `test_corpus_export_rejects_bad_output_targets` (covers
    `.striatum/`, outside-repo, file-target rejection paths), and
    the augmentation-boundary regression
    `test_no_engram_imports_or_memory_capabilities_in_striatum`
    (walks `src/striatum/corpus/`, `src/striatum/cli/`,
    `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/`,
    `src/striatum/mcp.py`, `src/striatum/service.py`, and
    `pyproject.toml`; asserts `import engram`, `from engram`, and
    `memory.` absent).
  - `tests/test_corpus_export_integration.py` —
    `test_corpus_export_replays_with_stable_jsonl_hashes` is the
    RFC 0044 §3 acceptance test: two CLI invocations into different
    `--out` directories produce byte-equal JSONLs and manifests
    that differ only on `generated_at`.

## Verification

Passing in the implementer run:

- `python -m pytest tests/test_corpus_*.py tests/test_cli_corpus_export.py -q`
  → 31/31 passed.
- `rg -n "^(import|from) engram" src/striatum/corpus src/striatum/cli
  src/striatum/daemon_rpc src/striatum/daemon_pg src/striatum/mcp.py
  src/striatum/service.py` → no matches.
- `rg -n "memory\\." src/striatum/corpus src/striatum/cli
  src/striatum/daemon_rpc src/striatum/daemon_pg src/striatum/mcp.py
  src/striatum/service.py` → no matches.
- `rg -n "FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions"
  src/striatum/corpus/enumerator.py` → no matches.
- `make lint` → passed.
- `make typecheck` → passed (`tests/test_web_ui.py` adjusted from
  `Traversable.read_text(errors=...)` to `read_bytes().decode(...,
  errors=...)` — test-only `importlib.resources` typing-surface
  compatibility, not a behavior change).

Full suite: `make test` → 739 passed / 33 skipped with one
pre-existing documentation-budget failure
(`tests/test_doc_links.py::test_decision_log_rows_under_word_budget`
reports `docs/DECISION_LOG.md` row D094 at 439 words). That row is
outside this packet's `write_scope.allowed_paths`, and the failure
is not a corpus-export regression.

## Build review verdicts

Three-way build review with distinct postures:

| Reviewer | Verdict | Severity | Posture | Scope |
|----------|---------|----------|---------|-------|
| codex | needs_revision | high | threat_model | redaction completeness + JSONL secret leakage |
| claude | accept_with_findings | low | ergonomics_dx | Striatum-side CLI + corpus surface |
| gemini | needs_revision | medium | threat_model | Engram-side attack surface (OUT OF SCOPE) |

**D100 cycle-exhaustion override applied**
(`dec_b3b26d4c86df408ab75f4cf515a82d1e`,
`accepted_with_follow_up`). Single accepting verdict (claude
`accept_with_findings` low) + 2 out-of-scope/anti-pattern
`needs_revision`s; impl meets V1 scope acceptance criteria.

### Codex review — 5th codex/codex anti-pattern instance

Codex `needs_revision severity=high` under threat_model posture
called out four redaction gaps:

- F1: redaction contract for JSONL `content` is unspecified at the
  RFC level; accidental `.env` or token-shaped content in otherwise
  allowed docs could become durable JSONL.
- F2: manifest provenance may leak absolute paths / private context;
  RFC says "repository path" without constraining the value.
- F3: tamper detection is hash-coverage-only; the spec does not say
  whether hashes cover pre- or post-redaction bytes, nor whether
  the future Engram ingester validates per-record redaction.
- F4: future MCP `engram.search` / `engram.fetch_reference` returns
  raw `content` without a stated output-side redaction gate.

Each finding is a fair adversarial probe of the RFC text but only
F1 + F2 land on the Striatum-side surface this dogfood actually
implements. F3 and F4 describe future Engram-side concerns.
Concretely, the implementation does enforce:

- F1: the redaction module refuses every denylisted source path
  (`tests/test_corpus_redaction.py::test_source_path_denylist`
  parametrizes over `.env`, `.env.local`, `keys/private.pem`,
  `.striatum/state.sqlite3`, `transcripts/`, `raw_model_output/`,
  `docs/transcript.txt`); commit messages get co-author-email +
  64-char-token scrubbing; audit-row payloads are
  metadata-only-scrubbed; unknown evidence fields default to the
  `<redacted>` placeholder. The "free-text content can carry a
  pasted token" risk is real but the denylist + commit-message
  scrubber + default-deny evidence path cover the documented
  attack vector for V1.
- F2: JSONL records carry repo-relative paths only; live
  run-summary provenance uses the sentinel `<repo-local-state>`
  instead of `.striatum/state.sqlite3`; the manifest carries repo
  HEAD + dirty-tree flag + `since` ref, none of which are
  absolute-path values.

This is the **5th consecutive codex/codex implementer+reviewer
anti-pattern instance** after D095 (dogfood-042 Track A), D096
(dogfood-042 Track C), D097 (dogfood-043), D098 (dogfood-044). The
empirical case is now overwhelming: when codex implements and codex
reviews, the codex reviewer's findings cluster around the codex
implementer's same blind spots, producing apparent `needs_revision`
verdicts that cross-lane review consistently overrides. The
refuse-by-default validator rule remains the most-overdue harness
improvement (TODO item 26).

The codex findings on Engram-side concerns (F3, F4) are forwarded
to the RFC 0044 threat-model section and to the Engram-side
follow-up at `~/git/engram/`.

### Gemini review — focused on out-of-scope Engram-side

Gemini `needs_revision severity=medium` under threat_model posture
returned a strong adversarial review of the **Engram side** of
RFC 0044 — the MCP server, the ingester, the capability model.
The five findings (A1 contradictory capability spec on
`memory.read_personal` default token, A2 lack of explicit
authorization checks in `engram.fetch_reference`, A3
cross-repository context leakage via shared `corpus_id`, A4
redaction bypass via memory poisoning in curated artifacts, A5
`describe_corpus` metadata leakage) are real and important —
**but none of them apply to what dogfood-046 actually shipped**.
The MCP server, the ingester, and the capability model all live in
`~/git/engram/` and are not in this packet's `write_scope`.

The override is **not** disagreement with the gemini findings; it
is a scope match. Gemini's findings are forwarded verbatim to the
Engram-side follow-up effort. The gemini review file
(`docs/dogfood/046/review/build/gemini/REVIEW.md`) was
operator-rewritten to preserve the substantive analysis while
adding the required `striatum.finding.v1` front-matter YAML block
and a plain `author:` byline — see PHASE_1_OPERATOR_NOTES for the
gemini-byline-prefix bug recurrence.

### Claude review — operator-composed minimal acceptance

The claude reviewer session **did not produce an on-disk review
artifact**. Only a 3.8 KB packet log was emitted; no
`docs/dogfood/046/review/build/claude/REVIEW.md` was written
through the publish path. The operator composed a minimal
`accept_with_findings severity=low ergonomics_dx` review from the
packet-log content covering five low-severity discoverability
findings on the in-scope Striatum-side CLI surface:

- F1: RFC 0044 §3 documents `--out` as optional (bracketed); the
  implementation hard-requires it.
- F2: `corpus` and `corpus export` parsers carry no
  `description=` / `help=` / per-argument `help=` strings.
- F3: bare-`--since` (omitted, not invalid) is caught by argparse
  before the dispatch handler, so the operator gets a stderr line
  + exit 2, not a JSON envelope.
- F4: success envelope returns `manifest_path` + `bundle_sha256`
  but no breadcrumb to the next operator step
  (`engram ingest-striatum --repo <path>`).
- F5: `--since` resolution failures surface raw `git rev-parse`
  stderr instead of using `StriatumError`'s `field_path=` + `hint=`
  fields.

None of the claude findings block V1 acceptance; all five are
ergonomics polish appropriate for a V1.5 sweep. The reviewer-emits-
no-artifact pattern is a **6th distinct anti-pattern instance**
distinct from the codex/codex co-blindness (D095-D098, D100) and
the codex-threat_model-reviewer harshness (D099) — see
PHASE_1_OPERATOR_NOTES.

## Out of scope (lives in `~/git/engram/`)

The following RFC 0044 components are **NOT** part of dogfood-046
and remain a separate Engram-side effort:

- `engram ingest-striatum --repo <path> [--since <ref>]` CLI verb on
  the Engram side that consumes the JSONL bundle this dogfood
  produces.
- Standalone `engram-mcp-stdio` MCP server binary.
- Four read-only retrieval tools: `engram.search`,
  `engram.fetch_reference`, `engram.describe_corpus`,
  `engram.health`.
- Engram-local `memory.*` capabilities (`memory.read_striatum`,
  `memory.read_personal`, `memory.describe`).
- The cryptographic-signing / manifest-trust trail discussed in
  codex F3 + gemini A1-A5 — those gates belong to the ingester's
  validation surface, not to the exporter.

The hard augmentation-not-dependency invariant says Striatum must
run with Engram absent; the regression test
`test_no_engram_imports_or_memory_capabilities_in_striatum` enforces
that invariant in the Striatum codebase. The Engram-side work has
no compile-time or runtime dependency from Striatum onto it.

## Test status

The codex implementer run completed `make lint`, `make typecheck`,
the 31 corpus-targeted tests, and `make test` (modulo the
pre-existing D094 word-budget failure documented above). No
TypeScript / Vite path was exercised this run (no frontend
changes).

## Backward compatibility

- No public CLI verb removed; `corpus export` is purely additive.
- No MCP tool name changes; the new verb is not exposed as an MCP
  tool in V1.
- No daemon RPC envelope changes; the corpus exporter is direct CLI
  only.
- No workflow schema changes; the exporter does not participate in
  runner state transitions.
- No `pyproject.toml` package-data changes.
- `tests/test_web_ui.py` adjustment is test-only compatibility
  plumbing.

## Known V1 follow-up gaps

- **Engram-side V1** (separate `~/git/engram/` effort): the
  ingester, the standalone MCP server, the four read-only retrieval
  tools, the Engram-local `memory.*` capabilities. Codex F3/F4 and
  gemini A1-A5 findings are forwarded to this effort.
- **Claude F1-F5 ergonomics polish** (Striatum-side V1.5 sweep
  candidate): align CLI behavior with RFC 0044 §3 on `--out`
  optionality (either drop `required=True` and default to
  `exports/corpus/<since-sha>/`, or update the RFC to say `--out` is
  required); add `description=` / `help=` strings; include a
  `next_step.command` breadcrumb in the success envelope; pass
  `field_path="--since"` + a `hint=` on the `git rev-parse` raise.
- **Codex F1/F2 surfacing of RFC-level redaction gaps**: forwarded
  to the RFC 0044 threat-model section. The implementation already
  enforces a denylist + commit-message scrubber + default-deny
  evidence path, but the RFC text could be more prescriptive about
  the redaction contract.

## Pointers

- Per-finding implementation HANDOFF:
  `docs/dogfood/046/build/HANDOFF.md`.
- Build review verdicts:
  `docs/dogfood/046/review/build/codex/REVIEW.md`,
  `docs/dogfood/046/review/build/claude/REVIEW.md`,
  `docs/dogfood/046/review/build/gemini/REVIEW.md`,
  `docs/dogfood/046/review/build/gemini/FINALIZATION.md`.
- Decision:
  `docs/dogfood/046/decisions/D100_cycle_exhaustion.md`.
- Design synthesis:
  `docs/dogfood/046/DESIGN_SYNTHESIS.md`.
- Operator notes:
  `docs/dogfood/046/PHASE_1_OPERATOR_NOTES.md`.
- Operator report (per-intervention narrative):
  `docs/dogfood/046/OPERATOR_REPORT.md`.
- `CHANGELOG.md` v1.35.0 — promotion entry.
- `docs/TODO.md` item 23 (✅ done, Striatum-side only) and new F47
  row.
- `docs/rfcs/README.md` RFC 0044 row — status bumped to
  `proposed (+ Striatum-side V1 landed under dogfood-046)`.
