---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "v1", "build"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0044 V1 (claude / ergonomics_dx)

Posture: developer-ergonomics, first-time-operator perspective.
Scope: `docs/dogfood/046/build/HANDOFF.md`, RFC 0044, RFC 0041, and the
implementation surface the handoff names (`src/striatum/cli/parser.py`,
`src/striatum/cli/dispatch.py`, `src/striatum/corpus/`,
`tests/test_cli_corpus_export.py`, `tests/test_corpus_export_integration.py`,
`tests/test_corpus_redaction.py`).

## Verdict

**accept_with_findings.** The CLI verb shape matches RFC 0044 §3, the
result envelope is operator-readable, the augmentation boundary is
test-pinned, and redaction has named denylist coverage. The verb is
usable today by an operator who has read the RFC. Findings below are
discoverability gaps that bite a first-time user who has not.

## What Works

- **CLI shape matches the RFC contract.** `striatum corpus export
  --since <ref> --out <dir>` is wired at
  `src/striatum/cli/parser.py:475-480` and dispatched at
  `src/striatum/cli/dispatch.py:553-561`. Bundle layout (nine JSONL
  files + `manifest.json`) is enforced from the closed `SUB_KINDS` /
  `JSONL_FILES` mapping at `src/striatum/corpus/types.py:13-35`, which
  matches RFC 0044 §3 line-for-line.
- **JSON envelope is the standard shape.** `StriatumError` is mapped
  through the existing dispatch handler
  (`src/striatum/cli/dispatch.py:88-100`) so `--out` violations and
  unresolvable `--since` refs surface as
  `{ok: false, error: {message, code}}` with exit 8. Confirmed by
  `tests/test_cli_corpus_export.py::test_corpus_export_invalid_since_returns_json_error_code_8`
  (line 55) and
  `tests/test_cli_corpus_export.py::test_corpus_export_rejects_bad_output_targets`
  (line 64), which together cover `.striatum/`, outside-repo, and
  file-target rejection.
- **Success envelope is operator-readable.** `CorpusBundleResult.to_json`
  (`src/striatum/corpus/types.py:82-90`) emits `status="exported"`,
  repo-relative `manifest_path` / `out`, `since` (both ref and resolved
  commit), `row_counts`, and `bundle_sha256` — exactly the pointers a
  human or scripted next-step needs.
- **Replay-stability is asserted on the operator path, not the
  internals.** `tests/test_corpus_export_integration.py::test_corpus_export_replays_with_stable_jsonl_hashes`
  (line 46) re-invokes the CLI twice into different `--out` dirs and
  byte-compares the JSONLs, then compares manifests with only
  `generated_at` stripped — matches RFC 0044 §3 acceptance criteria
  verbatim.
- **Augmentation boundary is pinned by a regression test, not a
  one-shot grep.** `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  (line 78) walks `corpus`, `cli`, `daemon_rpc`, `daemon_pg`, `mcp.py`,
  `service.py` and asserts `import engram`, `from engram`, `memory.`
  absent, and `pyproject.toml` Engram-free. This is exactly the
  "augmentation-not-dependency" guarantee in RFC 0044 §8 and it stays
  green on subsequent edits, which is the operator-trust win.
- **Redaction denylist is real.** `tests/test_corpus_redaction.py::test_source_path_denylist`
  (line 15) parametrizes over `.env`, `.env.local`, `keys/private.pem`,
  `.striatum/state.sqlite3`, `transcripts/session.md`,
  `raw_model_output/out.log`, `docs/transcript.txt` — the exact
  categories RFC 0044 §3 forbids. Commit-message scrubbing of
  co-author emails and 64-char tokens is also covered
  (`tests/test_corpus_redaction.py:48`).
- **Lazy import.** `from striatum.corpus import export_corpus_bundle`
  is local to the dispatch branch (`dispatch.py:554`) so unrelated
  verbs never pay the corpus surface's startup cost. Good DX even for
  operators who never run `corpus export`.

## Findings

### F1 — RFC 0044 §3 marks `--out` optional; implementation requires it

Severity: low (blocks discoverability not function).

RFC 0044 §3 documents the verb as
`striatum corpus export --since <ref> [--out <dir>]`. The bracketed
form is repeated in the §3 "Striatum Export Bundle" header, in the
acceptance-criteria block ("Striatum Export And Wiring"), and in
RFC 0041's downstream references. `src/striatum/cli/parser.py:479`
hard-requires it (`required=True`).

A first-time operator following the RFC will run
`striatum corpus export --since <ref>`, hit argparse's
`error: the following arguments are required: --out`, and *not* get a
JSON envelope (argparse exits before dispatch's `StriatumError`
handler runs). This violates the ergonomics_dx requirement that
"`--since` and `--out` errors produce a useful JSON envelope."

Two acceptable resolutions, either is fine:

- Honor the RFC: drop `required=True`, default to a path like
  `exports/corpus/<since-sha>/` (must stay outside `.striatum/` per
  existing redaction policy).
- Update RFC 0044 §3 + Acceptance Criteria to make `--out` required
  and add a `help=` string explaining why.

Today the operator-facing failure mode is non-fatal (the verb works
when the operator reads the argparse error), but it breaks the
contract a first-time user reads first.

### F2 — `corpus` and `corpus export` parsers carry no `help=` / `description=`

Severity: low.

`src/striatum/cli/parser.py:475-480` registers the subcommand with no
description, no help, and no per-argument help strings. Compare to
peers in the same file that *do* surface RFC pointers and example
invocations:

- `workflow generate` (`parser.py:263-272`) describes the verb,
  references `workflow templates list`, and shows a concrete
  invocation in `--help`.
- `workflow upgrade` (`parser.py:220-229`) explains per-model
  harness-profile semantics.
- `recovery auto` (`parser.py:576-584`) and `recovery watch`
  (`parser.py:617-624`) cite their RFCs and describe the loop.

`striatum corpus --help` and `striatum corpus export --help` are
currently bare. A first-time operator cannot discover from
`--help` alone:

- That `--since` accepts any git revision resolved through
  `git rev-parse --verify <ref>^{commit}` (HANDOFF.md line 16;
  `src/striatum/corpus/git.py:41-42`).
- That `--out` must be inside the repo and outside `.striatum/`
  (HANDOFF.md line 15; `src/striatum/corpus/export.py:51-62`).
- That the next-step operator command is
  `engram ingest-striatum --repo <path>` (RFC 0044 §3 / §4; not
  surfaced in CLI text at all).

Recommend: add `description=` to `corpus_export` with the verb shape
and the next-step command, and `help=` strings on `--since` and
`--out` naming the constraints. Purely additive; no behavioral change.

### F3 — Bare-`--since` (omitted, not invalid) bypasses the JSON envelope

Severity: low.

`tests/test_cli_corpus_export.py:55` proves that an *unresolvable*
`--since` (e.g. `missing-ref`) returns the structured envelope. But
omitting `--since` entirely is caught by argparse before the dispatch
handler — the operator gets a stderr line and exit 2, not a JSON
envelope. This is consistent with the rest of the CLI, but the
ergonomics_dx checklist specifically calls out "no foot-guns when
`--since` is omitted." Recommend: either accept the asymmetry (it is
the project-wide pattern) and document the constraint in the verb's
`help=`, or attach a custom argparse `error()` hook for this verb.
Lowest-cost fix is documentation in `help=` (folded into F2).

### F4 — No breadcrumb to `engram ingest-striatum` in the success envelope

Severity: low (ergonomics).

`CorpusBundleResult.to_json` (`types.py:82-90`) returns
`manifest_path` and `out` but not the next operator step. RFC 0044 §3
"Phase 1 Data Flow" makes the handoff explicit:
`striatum corpus export ... -> engram ingest-striatum --repo <path>
[--since <ref>]`. A friendly envelope would include a
`next_step.command` field, e.g.
`engram ingest-striatum --repo <repo> --since <ref>`. This is a small
addition (one string field, deterministic, redaction-safe) that
makes the augmentation-not-dependency path discoverable to a
first-time user who only reads the runner's output. Engram absence
remains harmless because the field is a printed string, not an
execution.

### F5 — `--since` resolution failures surface raw git stderr

Severity: low (polish).

`src/striatum/corpus/git.py:36-37` raises `StriatumError` with
`result.stderr.strip()` as the message, so an invalid `--since` like
a typo'd branch name lands in the JSON envelope as
`{"error": {"message": "fatal: Needed a single revision", ...}}`.
Operators who use git daily will recognize this; first-time users
may not. `StriatumError` supports optional `hint` and `field_path`
attributes — see `src/striatum/cli/dispatch.py:91-99` for the
envelope fields that are already passed through. Recommend: when
re-raising from `resolve_commit`, pass `field_path="--since"` and a
`hint` such as `"Provide a git ref (tag, sha, or branch). Try \`git
log --oneline\`."`. Low-cost, high-discoverability.

## Required-Check Results

- **CLI verb works**: shape and behavior verified through
  `tests/test_cli_corpus_export.py::test_corpus_export_cli_success_and_manifest`
  (line 41), which invokes the real CLI subprocess and asserts
  the manifest path, JSONL presence, and `status="exported"`.
- **Replay-stability**: covered by
  `tests/test_corpus_export_integration.py::test_corpus_export_replays_with_stable_jsonl_hashes`
  (line 46); byte-equality on JSONLs, manifest equality after
  stripping `generated_at`. Matches RFC 0044 acceptance criteria.
- **Augmentation boundary**: pinned by
  `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  (line 78), and HANDOFF.md lines 44-46 record the matching live
  ripgrep results. Daemon RPC registry has no `memory.*` capability
  (asserted in the same test through the daemon_rpc / daemon_pg
  globs).
- **Redaction enforced**:
  `tests/test_corpus_redaction.py::test_source_path_denylist`
  asserts `.env`, `.striatum/state.sqlite3`, transcripts, and raw
  model output are refused;
  `test_audit_payload_keeps_metadata_only` scrubs free-text from
  audit rows;
  `test_run_summary_redaction_preserves_renderer_shape_and_redacts_unknowns`
  defaults unknown fields to the evidence placeholder.
- **Tests pass**: HANDOFF.md line 43 records 31/31 corpus-targeted
  tests green; lines 50-52 record the full suite at 739 passed /
  33 skipped with one pre-existing documentation-budget failure
  (`tests/test_doc_links.py::test_decision_log_rows_under_word_budget`)
  outside this packet's write scope. Not a corpus-export regression.

## Operator Walkthrough (first-time-user simulation)

1. `striatum corpus --help` → no help text (F2). Operator falls back
   to RFC 0044.
2. Operator copies `striatum corpus export --since <ref>` from
   RFC 0044 §3. Hits argparse `--out` required error (F1). Adds an
   `--out` of their own choosing.
3. Picks `.striatum/exports/`. Gets exit-8 JSON envelope
   (`--out must not be under .striatum`). Re-runs with
   `exports/corpus`. Success envelope returns `manifest_path`,
   `bundle_sha256`. Good.
4. Operator now has a bundle but no in-runner hint to run
   `engram ingest-striatum` (F4). They re-read RFC 0044 §4 to find
   the next command.

Net: the verb works, the failure paths are mostly informative, and
the success envelope is concise. Findings are all on the
discoverability axis, not correctness.

## Citations

- RFC: `docs/rfcs/0044-engram-phase-1-implementation-spec.md` §3
  (lines 127-203), §8 (lines 308-321), Acceptance Criteria
  (lines 354-378).
- RFC 0041:
  `docs/rfcs/0041-engram-memory-layer-for-striatum-operators.md`
  (problem framing, augmentation-not-dependency invariant).
- Handoff: `docs/dogfood/046/build/HANDOFF.md` (shipped scope and
  verification).
- Parser: `src/striatum/cli/parser.py:475-480`.
- Dispatch: `src/striatum/cli/dispatch.py:553-561`; envelope
  handling at lines 88-100.
- Export orchestration:
  `src/striatum/corpus/export.py:16-62`,
  `src/striatum/corpus/types.py:13-90`,
  `src/striatum/corpus/git.py:24-42`.
- Tests cited inline above.
