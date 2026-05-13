---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---
author: designer-unknown-model-001

# Dogfood-046 — RFC 0044 V1 Striatum-side corpus export — Design (claude_code lane)

Scope: Striatum-side only. Add the `striatum corpus export --since <ref>
[--out <dir>]` CLI verb and a new `src/striatum/corpus/` package that
emits a redacted JSONL bundle Engram's future `ingest-striatum` reads.
Engram ingestion, `engram-mcp-stdio`, retrieval tools, capability
vocabulary, and the `striatum-engram` skill bundle are explicitly out
of scope (RFC 0044 §3 / §8). `striatum operator memory check` is
deferred to a follow-up dogfood — V1 ships export only.

Grounding: RFC 0044 §3 "Striatum Export Bundle" defines the bundle
contract (filenames, JSONL shape, `sub_kind` set, `external_id` table,
manifest fields). RFC 0044 §8 "Augmentation-Not-Dependency
Enforcement" defines the boundary the implementation must not cross.
RFC 0041 §"Augmentation-Not-Replacement Boundary" supplies operator
motivation: Striatum must run unchanged when Engram is missing.

## 1. CLI verb wiring

Add a top-level `corpus` subparser in `src/striatum/cli/parser.py`,
mirroring the existing pattern at lines 475-489 for `decision record`
and 508-526 for `recovery <subcommand>`. Construction:

```python
# src/striatum/cli/parser.py — added after the `decision` subparser
corpus = sub.add_parser(
    "corpus",
    help=(
        "RFC 0044 V1: produce a redacted JSONL bundle of the Striatum "
        "software-building corpus (RFCs, decisions, operator reports, "
        "run summaries, audit chain, changelog, ubiquitous-language "
        "terms, harness-friction patterns, commits) for Engram "
        "ingestion. Striatum stays independent of Engram at runtime."
    ),
)
corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
corpus_export = corpus_sub.add_parser("export")
corpus_export.add_argument(
    "--since", required=True,
    help="git ref bounding which commits and which file revisions enter the bundle",
)
corpus_export.add_argument(
    "--out", default=None,
    help="output directory; defaults to .striatum/corpus_export/<since-sha>/",
)
corpus_export.add_argument(
    "--include-active-runs", action="store_true",
    help=(
        "ALSO call run_summary_snapshot for live runs in repo-local SQLite "
        "(or daemon, per D094) on top of committed RUN_SUMMARY.md files. "
        "Off by default to keep the bundle deterministic."
    ),
)
corpus_export.add_argument("--json", action="store_true")
```

Dispatch wiring in `src/striatum/cli/dispatch.py`, mirroring the
`run summary` branch at lines 434-435 and `evidence export` at line
551-552:

```python
# src/striatum/cli/dispatch.py — added inside the connect(repo) block
from striatum.cli.corpus_export import corpus_export as _corpus_export

if args.command == "corpus" and args.corpus_command == "export":
    return _corpus_export(
        conn,
        repo=repo,
        since=args.since,
        out_text=args.out,
        include_active_runs=bool(args.include_active_runs),
    )
```

`_corpus_export` is a thin CLI handler that delegates to
`striatum.corpus.bundle.build_bundle(...)` and returns the JSON
envelope. The same `StriatumError` / `--json` envelope semantics from
`dispatch.main` at lines 86-103 cover error reporting; the verb uses
exit codes `2` (invalid argument), `3` (repo not initialized), `4`
(invalid transition — e.g. unknown `--since` ref), and a new `15` for
"corpus export refused" (e.g. dirty tree without `--allow-dirty`,
deferred for V1.5).

`--out` defaults to `.striatum/corpus_export/<short-sha-of-since>/`.
Per the `path_allowed` check in `db.py` line 275-287 and the
`forbidden_paths=[STATE_DIR]` convention, the corpus export writes
under `.striatum/` legitimately because the bundle is operator-local
scratch, not a publishable artifact — `repo_relative_path` allows this
via the `allow_state=True` shape used by `_repo_relative_path` line
257. The export does NOT pass through the artifact publisher in
`src/striatum/artifacts.py`, because corpus bundles are not workflow
artifacts (no `workflow.json` declares them, no `expected_artifacts`
entry binds them, no lease owns them). The export is operator-shell
output, on the same shape level as `striatum evidence export`.

Exit-code envelope (JSON mode) on success:

```json
{
  "ok": true,
  "data": {
    "status": "exported",
    "since": "v1.34.0",
    "out": ".striatum/corpus_export/a0b2e94/",
    "manifest_path": ".striatum/corpus_export/a0b2e94/manifest.json",
    "row_counts": {"rfc": 44, "decision_log_row": 99, "...": "..."},
    "bundle_sha256": "...manifest-hash..."
  }
}
```

## 2. New module `src/striatum/corpus/`

```text
src/striatum/corpus/
  __init__.py            # public facade: build_bundle(...)
  bundle.py              # top-level orchestration; opens git, iterates
                         # sub_kinds, writes JSONL files, writes manifest
  manifest.py            # ManifestBuilder: collects per-file SHA-256,
                         # row counts, git HEAD, dirty flag, since ref,
                         # striatum_version
  jsonl.py               # writer with deterministic ordering + canonical
                         # JSON encoding (sort_keys=True, no NaN/Inf)
  git_index.py           # thin git wrapper (subprocess; no GitPython
                         # dependency). Resolves --since, lists commits
                         # in <since>..HEAD, last-touching commit per file
  redaction.py           # per-source-kind redaction rules + a single
                         # `redact(content: str, policy: Policy) -> str`
                         # entry point. Mirrors evidence.py's
                         # _apply_evidence_policy (cli/evidence.py:290)
                         # but is content-, not envelope-shaped.
  enumerators/
    __init__.py          # registry: SUB_KIND_ENUMERATORS dict
    rfcs.py              # docs/rfcs/*.md
    decisions.py         # docs/DECISION_LOG.md table rows
    operator_reports.py  # docs/dogfood/<NNN>/OPERATOR_REPORT.md
                         # intervention sections
    run_summaries.py     # docs/dogfood/<NNN>/RUN_SUMMARY.md ∪
                         # run_summary_snapshot() (opt-in)
    audit_chain.py       # repo-local `events` rows ∪ daemon audit_log
                         # (filtered by repository_id)
    changelog.py         # CHANGELOG.md version-block sections
    ubiquitous_language.py  # docs/UBIQUITOUS_LANGUAGE.md table rows
    friction.py          # docs/HARNESS_FRICTION_PATTERNS.md pattern
                         # sections
    commits.py           # git log <since>..HEAD --pretty=...
```

One module per concern, all pure-function except `bundle.py` and
`audit_chain.py` (which read from sqlite + the daemon respectively).
No module imports an Engram client library; verify with the §8
acceptance grep (below).

## 3. Corpus enumeration

Each enumerator returns an iterable of `CorpusRow` dataclass instances
the writer serializes. `CorpusRow` shape mirrors RFC 0044 §3 verbatim:

```python
@dataclass(frozen=True)
class CorpusRow:
    source_kind: Literal["striatum"]  # constant
    sub_kind: SubKind                  # see closed set below
    external_id: str
    content: str                       # redacted body
    provenance: Provenance             # path, sha256, commit
    observed_at: str                   # ISO-8601 UTC, the source's
                                       # natural "observed" timestamp,
                                       # NOT generated_at (which lives
                                       # only on the manifest)
```

`SubKind = Literal["rfc", "decision_log_row", "operator_report",
"run_summary", "audit_chain_entry", "changelog_entry",
"ubiquitous_language_term", "harness_friction_pattern", "commit"]`
exactly matches RFC 0044 §3's closed set. Anything new must extend the
type and add a registered enumerator before passing tests.

Per-enumerator sourcing:

| sub_kind | source | one row per | `external_id` | `observed_at` |
|---|---|---|---|---|
| `rfc` | every file matching `docs/rfcs/*.md` at git HEAD | top-level `##` heading inside the file | `rfc:<NNNN>#<heading-slug>` per RFC 0044 §3 table | committer date of last commit touching the file |
| `decision_log_row` | `docs/DECISION_LOG.md` parsed via `markdown_it.parse_table()` (the project already depends on `markdown-it-py>=4.0` per `pyproject.toml:30`) | one `D###` row | `decision:<D###>` | committer date of last commit touching `docs/DECISION_LOG.md` (per-row date not available without git-blame; defer per-row to V1.5) |
| `operator_report` | `docs/dogfood/<NNN>/OPERATOR_REPORT.md` for every dogfood directory | one `### Intervention <N>:` section, plus one preamble row + one "## Run Outcome" row | `dogfood:<NNN>#intervention-<N>`, `dogfood:<NNN>#preamble`, `dogfood:<NNN>#outcome` | committer date of last commit touching the file |
| `run_summary` | (a) every committed `docs/dogfood/<NNN>/RUN_SUMMARY.md`. (b) when `--include-active-runs` is passed, call `run_summary_snapshot(conn, repo=repo, run_id=...)` from `cli/run_summary.py:41` for each non-terminal run | one run | `run:<run-id>` (from the `Run ID:` line in the markdown or from the snapshot dict) | `run.completed_at` if known, else `run.created_at`, else committer date |
| `audit_chain_entry` | (a) repo-local `events` rows (table inserted by `db.insert_event` at `db.py:205-240`) — filtered to a curated transition set (`run.completed`, `run.failed`, `verdict.recorded`, `decision.recorded`, `recovery.stale_requeued`, `recovery.process_blocker_resolved`, `job.canceled`); (b) daemon-side `audit_log` rows (filtered by `repository_id` for the target repo) when the daemon is reachable per D094/RFC 0043 | one event/audit row | `audit:repo:<event_id>` for repo-local, `audit:daemon:<audit_id>` for daemon-side | `created_at` (repo-local) or `timestamp` (daemon-side) |
| `changelog_entry` | `CHANGELOG.md` parsed into per-version blocks delimited by `## vX.Y.Z` headings | one version block | `changelog:<vX.Y.Z>` | header date on the version line (e.g. `## v1.34.0 — 2026-05-13`) |
| `ubiquitous_language_term` | `docs/UBIQUITOUS_LANGUAGE.md` table rows | one term | `ulang:<slug-of-term>` | committer date of last commit touching the file |
| `harness_friction_pattern` | `docs/HARNESS_FRICTION_PATTERNS.md` parsed into per-pattern sections (`## Pattern N — <slug>`) | one pattern | `friction:<slug>` (slug = lowercased, dash-joined section title minus `Pattern N — ` prefix) | committer date of last commit touching the file |
| `commit` | `git log <since>..HEAD --pretty='%H%n%aI%n%cI%n%an%n%s%n--%n%B%n=='` | one commit | `commit:<full-sha>` per RFC 0044 §3 table | committer date (`%cI`) |

Critical mandate from RFC 0044 §3: **run summaries MUST go through
`striatum run summary --json`, not free-text SQLite reads.** The V1
sourcing for `run_summary` therefore reads (a) committed
`RUN_SUMMARY.md` files (which were themselves written through the
`run_summary_export` path at `cli/run_summary.py:23` and are durable
provenance), and (b) under `--include-active-runs`, invokes
`run_summary_snapshot(conn, repo=repo, run_id=...)` — the same
function `striatum run summary` invokes through the CLI. No
enumerator opens the `runs`, `verdicts`, `artifacts`, `jobs`, or
`sessions` tables directly. Acceptance grep (added to a test):

```bash
rg -n "FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions" \
    src/striatum/corpus/enumerators/run_summaries.py
# must return zero matches
```

`--since <ref>` semantics are intentionally simple:

- For `commit`: filter to `git rev-list <since>..HEAD` (commits that
  exist on HEAD's reachability and not on `<since>`'s).
- For every file-backed sub_kind: include the row when the file's
  last-touching commit (`git log -n1 --pretty=%H -- <path>`) is in the
  `<since>..HEAD` set OR `<since>` is omitted. This makes the bundle
  stable: re-running with the same `<since>` and a clean tree
  produces byte-identical JSONL content (only `manifest.generated_at`
  varies — RFC 0044 §"Re-running the export with unchanged inputs
  produces identical JSONL file content").
- For `audit_chain_entry` rows: filter to rows whose `created_at >=
  committer_date(<since>)`. This is a temporal join, not a git-tree
  join, and it's documented as such in the manifest.

Deterministic ordering inside each `.jsonl` file: lexicographic sort
by `external_id` after enumeration. The writer asserts non-decreasing
order before writing each line so a regression in an enumerator
surfaces at write time instead of producing a non-reproducible
bundle.

## 4. Redaction policy

The export is redacted-by-construction. RFC 0044 §3 says the export
"never includes transcripts, terminal output, raw model output,
SQLite blobs, or ambiguous free-text live-state fields." The V1
denylist is hard and explicit, not heuristic:

### Per-file/path denylist (refused entirely)

- `.env`, `.env.*`, `*.pem`, `*.key`, `*.p12` — refused with an
  explicit error if encountered during enumeration (bug if it
  happens, since none of the V1 enumerators traverse outside
  `docs/`, `CHANGELOG.md`, `git log`, and `events` rows).
- `.striatum/state.sqlite3`, `.striatum/*.sqlite3`, anything under
  `.striatum/` other than `.striatum/corpus_export/<out>/` — never
  read by any enumerator. Validated by a test that monkey-patches
  `Path.open` and asserts no `.striatum/` reads occur during a
  bundle build.
- `tests/`, `transcripts/`, `**/transcript*.{txt,md,log}` — never
  enumerated.
- `~/.claude/`, `/tmp/`, anything outside `repo_root` — never
  enumerated.

### Per-field redaction rules (applied to the `content` of each row)

| Sub-kind | Field source | V1 redaction |
|---|---|---|
| `rfc`, `decision_log_row`, `ubiquitous_language_term`, `harness_friction_pattern`, `changelog_entry`, `operator_report` | committed markdown | pass-through — these files are already public, redacted-at-commit-time provenance |
| `run_summary` (committed) | committed `RUN_SUMMARY.md` | pass-through (already passed through `evidence.redact_evidence_payload` at write time — `cli/evidence.py:257`) |
| `run_summary` (active, opt-in) | `run_summary_snapshot()` JSON | apply `evidence.redact_evidence_payload` (already exists at `cli/evidence.py:257`) before flattening to text. Reuse, do not re-implement. |
| `audit_chain_entry` (repo-local `events`) | `payload_json` column | apply `redact_evidence_payload` on the payload; drop `payload.description`, `payload.rationale`, `payload.summary`, `payload.original_envelope`, `payload.error_message` fields (free-text agent prose). Keep `payload.reason`, `payload.workflow_job_id`, `payload.completed_inline`, `payload.blocker_kind` — structural enums and ids. |
| `audit_chain_entry` (daemon `audit_log`) | already metadata-only per D085 | pass-through, but drop `denial_reason` if it appears (D085 says daemon audit is metadata-only; this is belt-and-suspenders). |
| `commit` | `git log %B` (commit body) | strip lines matching `Co-Authored-By: .*<.*@.*>` (preserves trailing newline) — these may carry email addresses. Also strip lines matching `^\s*[A-Za-z0-9+/]{40,}\s*$` (heuristic detection of pasted keys/tokens in commit messages — rare, but explicit). |

Reuse, not re-implement: `cli/evidence.py:257-355` (`redact_evidence_payload`,
`_evidence_policy_for_top_level`, `_apply_evidence_policy`) is the
existing redaction engine. `corpus/redaction.py` imports those and
adds the per-content-kind rules above. This keeps a single redaction
truth surface — `evidence_export` and `corpus export` agree on what
"free text" means.

Negative test, in `tests/test_corpus_export.py`:

```python
def test_bundle_excludes_transcript_paths(tmp_repo):
    # plant a fake transcript under target repo
    (tmp_repo / "transcripts" / "session1.log").write_text("PROMPT: ...")
    build_bundle(tmp_repo, since="HEAD~5", out=tmp_repo / "out")
    for jsonl in (tmp_repo / "out").glob("*.jsonl"):
        body = jsonl.read_text()
        assert "PROMPT:" not in body
        assert "transcripts/" not in body
```

## 5. JSONL emission

Exact line shape, locked to RFC 0044 §3:

```json
{
  "source_kind": "striatum",
  "sub_kind": "rfc",
  "external_id": "rfc:0044#proposal",
  "content": "...redacted markdown body...",
  "provenance": {
    "path": "docs/rfcs/0044-engram-phase-1-implementation-spec.md",
    "sha256": "5e8f...",
    "commit": "a0b2e94..."
  },
  "observed_at": "2026-05-13T00:00:00Z"
}
```

Encoding rules:

- One JSON object per line, no leading/trailing whitespace inside a
  line, `\n` line endings (Unix), file ends with a final `\n`.
- Keys emitted in fixed insertion order: `source_kind`, `sub_kind`,
  `external_id`, `content`, `provenance`, `observed_at`. The writer
  passes `sort_keys=False` and uses an `OrderedDict` (or a
  `dataclasses.asdict` -> ordered dict translation) because RFC
  ordering matters more than alphabetic ordering for diff
  readability. Inner objects (`provenance`) use the same fixed key
  order: `path`, `sha256`, `commit`.
- `json.dumps(..., ensure_ascii=False, separators=(",", ":"))` —
  match the project's `json_dumps` helper in `db.py` (line 14-ish)
  for canonical compactness.
- `observed_at` is ISO-8601 UTC, second precision, `Z` suffix —
  matches the rest of the codebase per `cli/recovery.py:464-465`.
- Empty `.jsonl` files are valid (zero rows of that sub_kind). The
  writer still emits the file so the manifest's count of 0 is
  verifiable.

Stable hashes acceptance test (the §"Re-running the export … stable
hashes" requirement):

```python
def test_bundle_replay_is_byte_stable(tmp_repo_with_history):
    out_a = tmp_repo_with_history / "a"
    out_b = tmp_repo_with_history / "b"
    build_bundle(tmp_repo_with_history, since="v0.1", out=out_a)
    build_bundle(tmp_repo_with_history, since="v0.1", out=out_b)
    for name in ("rfcs.jsonl", "decision_log_rows.jsonl", "commits.jsonl"):
        assert (out_a / name).read_bytes() == (out_b / name).read_bytes()
    # manifest.generated_at is the only allowed difference
    ma = json.loads((out_a / "manifest.json").read_text())
    mb = json.loads((out_b / "manifest.json").read_text())
    ma.pop("generated_at"); mb.pop("generated_at")
    assert ma == mb
```

## 6. Manifest

`manifest.json` (one file at bundle root) per RFC 0044 §3, fields
locked:

```json
{
  "schema_version": "striatum.corpus_export.v1",
  "striatum_version": "1.34.0",
  "repo_root": "/home/halbritt/git/striatum",
  "repo_relative_root": ".",
  "git_head": "a0b2e94...",
  "git_dirty": false,
  "since_ref": "v1.33.0",
  "since_commit": "5ef1175...",
  "generated_at": "2026-05-13T12:55:00Z",
  "schema": {
    "row_shape_version": "striatum.corpus_row.v1",
    "sub_kinds": ["rfc", "decision_log_row", "operator_report",
                  "run_summary", "audit_chain_entry", "changelog_entry",
                  "ubiquitous_language_term", "harness_friction_pattern",
                  "commit"]
  },
  "source_kinds": ["striatum"],
  "row_counts": {
    "rfc": 44, "decision_log_row": 99, "operator_report": 158,
    "run_summary": 46, "audit_chain_entry": 12034,
    "changelog_entry": 34, "ubiquitous_language_term": 96,
    "harness_friction_pattern": 3, "commit": 821
  },
  "files": {
    "rfcs.jsonl": {"sha256": "...", "rows": 44, "bytes": 184421},
    "decision_log_rows.jsonl": {"sha256": "...", "rows": 99, "bytes": ...},
    "...": "..."
  },
  "repo_local_schema_version": 14,
  "daemon_audit_included": true,
  "options": {"include_active_runs": false}
}
```

Field sources:

- `striatum_version`: read from `pyproject.toml` via
  `importlib.metadata.version("striatum-orchestrator")` (matches the
  pattern used by `cli/__init__.py` and the daemon doctor).
- `repo_root`: `Path(args.repo).resolve()` matching
  `dispatch.dispatch` line 168.
- `git_head`: `git rev-parse HEAD`.
- `git_dirty`: `git status --porcelain` non-empty.
- `since_ref`: argv value verbatim.
- `since_commit`: `git rev-parse <since>^{commit}`.
- `repo_local_schema_version`: read from `migrations.py:CURRENT_SCHEMA_VERSION`
  (or equivalent — see `src/striatum/migrations.py`); cross-check
  with `PRAGMA user_version` from the live `state.sqlite3`.
- `daemon_audit_included`: True only if the daemon was reachable
  AND its `repository_id` for this repo was resolvable AND at least
  one audit row was emitted (else False; bundle still validates).
- `files[*].sha256`: SHA-256 of the per-file JSONL bytes computed
  AFTER the file is finalized (use `db.sha256_bytes` from `db.py`
  which is already used by `run_summary_export` at line 31).
- `files[*].rows`: line count.
- `files[*].bytes`: byte length.

The `bundle_sha256` returned in the CLI envelope is the SHA-256 of
the canonicalized manifest bytes (`json.dumps(manifest, sort_keys=True,
ensure_ascii=False, separators=(',', ':')).encode("utf-8")`). The
ingester re-derives that hash; mismatch fails ingest per RFC 0044
§"Engram refuses partial bundles when counts or hashes do not match."

## 7. Augmentation-not-dependency enforcement

RFC 0044 §8 names six rules. Striatum's V1 implementation must honor
all of them; the design pins the enforcement:

1. **No Engram client import**. `src/striatum/corpus/` and
   `src/striatum/cli/corpus_export.py` may not `import engram`,
   `from engram ...`, or list any `engram-*` package in
   `pyproject.toml` `dependencies` or `optional-dependencies`.
   Acceptance grep:

   ```bash
   rg -n "engram" src/striatum/cli src/striatum/corpus
   # allowed matches: help text, subprocess args, doc literals
   # forbidden matches: `import engram`, `from engram`, dependency declarations
   ```

   This is the exact grep RFC 0044 §8 specifies — the test runs it
   and fails on any import-shaped match (a tightened regex).

2. **No Engram method on the daemon RPC registry**. `corpus export`
   is a plain CLI verb; it does NOT register a method on
   `src/striatum/daemon_rpc/` (`MethodRegistry` lives there per
   D087). Verify with: `rg -n "engram|memory" src/striatum/daemon_rpc/`
   returns zero matches.

3. **No `memory.*` capability in the Striatum daemon method
   registry**. The closed set stays `{read, write, review, claim,
   apply, admin, recovery}` per D087/D088. Corpus export inherits
   `read`. No new capability ships in V1.

4. **Operator-session budget**: corpus export is a foreground
   command, not a session-time retrieval; the V1 budget rule from
   RFC 0044 §8 (2s search / 5s fetch) applies to Engram retrieval,
   not export. V1 explicitly does NOT call into Engram. Export
   wall-time targets: <30s for a 1-year `<since>` window on the
   current repo; pin in a `pytest -m smoke` budget.

5. **Engram unavailability degrades gracefully**. Striatum has no
   runtime dependency on Engram. The V1 implementation only emits
   the bundle; the operator (or a follow-on script) feeds it to
   `engram ingest-striatum`. Striatum does not block on the result.

6. **"Engram off" artifact equivalence test**. RFC 0044 §"Augmentation
   Boundary" requires a dogfood-shaped test proving Striatum's
   required artifacts come out the same with Engram unavailable.
   V1 satisfies this trivially: corpus export is a separate verb;
   no workflow execution path calls it. The "Engram off"
   equivalence test for V1 = `pytest tests/test_engram_off_smoke.py`
   shaped as: start a tiny dogfood-style run that produces the
   normal artifact set, then verify byte-identical artifacts with
   `engram-mcp-stdio` symlinked to `/usr/bin/false` and PG point
   nulled. Acceptable to land this test as a skip-marked stub in
   V1 with the implementation in V1.5 — operator memory check
   lands then.

## 8. Tests

`tests/test_corpus_export.py` (new):

- `test_cli_smoke` — invoke `python -m striatum.cli corpus export
  --since HEAD~5 --out <tmp>` against a fixture repo built by an
  existing test helper (steal the pattern from
  `tests/test_evidence_export.py` if present, else from
  `tests/test_run_summary.py`).
- `test_bundle_files_exist` — every filename in RFC 0044 §3 is
  present in the output directory (`rfcs.jsonl`, `decision_log_rows.jsonl`,
  ... `commits.jsonl`, `manifest.json`).
- `test_jsonl_shape` — for each non-empty .jsonl, every line parses,
  has the locked key order, and every row's `source_kind` ==
  `"striatum"` and `sub_kind` ∈ the closed set.
- `test_external_ids_match_rfc_0044_table` — for each sub_kind,
  random sample 3 rows and assert the `external_id` regex matches
  RFC 0044 §3's table (`commit:<40-hex>`, `decision:D\d+`,
  `rfc:\d{4}#[a-z0-9-]+`, etc.).
- `test_bundle_replay_is_byte_stable` — already shown in §5.
- `test_bundle_excludes_transcript_paths` — already shown in §4.
- `test_no_state_sqlite_reads` — monkey-patch `sqlite3.connect` and
  assert no read of `state.sqlite3` happens during a bundle build
  whose enumerators don't logically need it (i.e. when
  `--include-active-runs` is off and the run_summary enumerator
  only reads committed markdown).
- `test_redaction_drops_free_text` — plant an `events` row with a
  `payload.description: "secret"` field; assert the corresponding
  `audit_chain_entry` row's `content` does not contain `"secret"`.
- `test_manifest_hash_is_stable` — run twice, manifest.json with
  `generated_at` stripped is byte-identical.
- `test_no_engram_import` — `rg -n "^(import|from) engram" src/striatum/`
  returns zero matches. This is the §8.1 grep, asserted in CI.

Per-enumerator unit tests (one per file under
`tests/test_corpus_enumerators/`): each takes a small markdown
fixture and asserts the returned `CorpusRow` list matches the
expected (`external_id`, content snippet, provenance.path, sub_kind)
tuples. These run in milliseconds and lock the parser shape.

Integration test (slow, `pytest -m integration`): run against an
actual dogfood-N branch (e.g. `striatum/dogfood-045-rfc-0038-v1-5`,
which is committed). Build the bundle, assert non-zero row counts
for every sub_kind, and assert that re-running with the same
`--since` produces byte-identical JSONL content. This is the
"integration test against a real run" the prompt requires.

## 9. Out of scope (V1) and open questions for synthesis

Explicitly deferred to a follow-on:

- `striatum operator memory check`. The §"Striatum Operator Wiring"
  verb (RFC 0044 §7) is informational and exits 0 regardless of
  Engram state. Lands in the operator-wiring dogfood that follows
  the Engram-side phase 1 (ingester + MCP server).
- The `striatum-engram` skill bundle for RFC 0015 profiles. Same
  follow-on. V1 carries no skill changes.
- Doc updates beyond a one-line BUILD/HANDOFF note. SPEC.md,
  HOW_TO_AGENT.md, HOW_TO_HUMAN.md, UBIQUITOUS_LANGUAGE.md, and
  DECISION_LOG.md edits called out in RFC 0044 §"Striatum Export
  And Wiring" partially apply to corpus export (adding the
  `striatum corpus export` verb to CLI_REFERENCE.md is in scope) —
  but the full operator-wiring narrative belongs to the follow-on.

Open questions the synthesis should resolve:

1. **Module name discrepancy**. Prompt: `src/striatum/corpus/`.
   RFC 0044 §3 verbatim: `src/striatum/corpus_export/`. This
   design follows the prompt. Synthesis should pick one and update
   the RFC if it goes with `corpus/`.
2. **`audit_chain_entry` source**. RFC 0044 §3's `external_id`
   table says `audit:<row-id>`, ambiguous between repo-local
   `events.event_id` and daemon `audit_log.audit_id`. This design
   namespaces them (`audit:repo:<id>`, `audit:daemon:<id>`). The
   synthesis should bless one shape or accept the namespaced one
   for forward-compatibility.
3. **`--since` semantics for non-`commit` sub_kinds**. Filter by
   "last-touching commit in `<since>..HEAD`" is one defensible
   choice (this design). The alternative — "all rows at HEAD,
   ignore `--since` for non-commit sub_kinds" — is simpler but
   makes bundles grow unboundedly. Pin one in synthesis.
4. **Run-summary live-run inclusion**. `--include-active-runs` is
   off by default in this design (favor deterministic bundles).
   Synthesis should confirm that, or flip the default if the
   intended use case is "always re-snapshot active runs."

## 10. Acceptance hooks (one-line each)

- `python -m striatum.cli corpus export --since v1.33.0 --out
  /tmp/out` exits 0 and writes 10 files (`9 .jsonl + manifest.json`).
- Re-running the same command produces byte-identical `.jsonl`
  files; only `manifest.generated_at` differs.
- `rg -n "^(import|from) engram" src/striatum/` returns zero
  matches (V1 §8 grep).
- `rg -n "FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM
  sessions" src/striatum/corpus/enumerators/run_summaries.py`
  returns zero matches (RFC 0044 §3 mandate).
- `rg -n "engram|memory" src/striatum/daemon_rpc/` returns zero
  matches (RFC 0044 §8 daemon-registry rule).
- `pytest tests/test_corpus_export.py tests/test_corpus_enumerators/`
  passes.

## 11. File-citation index (for the reviewer)

- CLI parser pattern to mirror: `src/striatum/cli/parser.py:475-489`
  (decision subparser), `:508-526` (recovery subcommands).
- CLI dispatch pattern to mirror: `src/striatum/cli/dispatch.py:551-552`
  (`evidence export` thin handler), `:434-435` (`run summary`
  delegation).
- Existing JSON envelope shape: `src/striatum/cli/dispatch.py:86-115`.
- Run-summary live-snapshot helper: `src/striatum/cli/run_summary.py:23-39`
  (`run_summary_export`), `:41-110` (`run_summary_snapshot`).
- Redaction engine to reuse: `src/striatum/cli/evidence.py:257-355`
  (`redact_evidence_payload`, `_evidence_policy_for_top_level`,
  `_apply_evidence_policy`).
- Event insertion (audit source for repo-local): `src/striatum/db.py:205-240`
  (`insert_event`); the `events` table.
- Daemon hash-chained audit (audit source when daemon-mediated per
  D094): `src/striatum/daemon.py:1041-1080` (`_audit_chain_records`);
  table created at `src/striatum/daemon_pg/sql/0001_baseline.sql:103`
  (`striatumd.audit_chain_head`).
- SHA-256 helper to reuse: `striatum.db.sha256_bytes` (used by
  `cli/run_summary.py:31`).
- Path-safety / state-dir gating: `src/striatum/db.py:252-287`
  (`repo_relative_path`, `_repo_relative_path`).
- Engram migration precedent for `source_kind` enum (do not modify):
  `~/git/engram/migrations/003_source_kind_claude.sql`,
  `~/git/engram/migrations/004_source_kind_gemini.sql`,
  `~/git/engram/migrations/005_source_kind_gemini.sql`. The
  Engram-side migration that adds `source_kind='striatum'` lands
  separately under `~/git/engram/` per RFC 0044 §"Implementation
  Plan" Step 1.
- Engram-side ingester precedent (do not modify):
  `~/git/engram/src/engram/claude_export.py`,
  `~/git/engram/src/engram/gemini_export.py` show the per-source
  export module shape Engram already uses; the Striatum export
  module name mirrors that convention from the Engram side.

## 12. Verdict-handle for review

The minimal V1 surface this design pins:

- `striatum corpus export --since <ref> [--out <dir>]
  [--include-active-runs]` CLI verb.
- `src/striatum/corpus/` package, eight enumerator files, one
  redaction module reusing `evidence.py:257-355`, one writer, one
  manifest builder, one bundle orchestrator.
- A locked JSONL shape and manifest schema that match RFC 0044 §3
  verbatim.
- A redaction posture that reuses the evidence redaction engine
  (single truth surface).
- An augmentation-not-dependency posture verified by three CI
  greps (Engram import, daemon-RPC method, run-summary
  free-text-SQL).
- A test suite that covers per-enumerator parsing, replay
  stability, redaction sufficiency, and the `--no-engram-import`
  invariant.

If the reviewer wants tighter scope: drop `--include-active-runs`
entirely from V1 (the committed `RUN_SUMMARY.md` source is enough
to validate the bundle). If the reviewer wants wider scope: add
`striatum operator memory check` — but that pulls in the
`engram-mcp-stdio` dependency, which RFC 0044 §"Implementation
Plan" Steps 1-2 explicitly puts in a different phase.
