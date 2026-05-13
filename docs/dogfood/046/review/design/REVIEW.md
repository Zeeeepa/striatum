---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0044", "v1", "design"]
---

author: reviewer-unknown-model-001

# RFC 0044 V1 Corpus Export Design Review (ergonomics_dx)

Posture: ergonomics_dx. The review treats the synthesis as a first-time-user
surface: a Striatum operator who has never seen RFC 0044 should be able to
discover, run, and recover from errors via `--help` and the JSON envelope
alone. The synthesis at `docs/dogfood/046/DESIGN_SYNTHESIS.md` is broadly
discoverable and consistent with existing CLI verbs. A small number of
ergonomics gaps remain and are listed below as actionable findings.

## Verdict

`accept_with_findings`. The design is implementable as-is. The findings below
are scoped, pinpoint, and resolvable during implementation; none of them
require re-synthesis.

## What lands well (ergonomics_dx)

- Module layout is one chosen shape with eight files and one named single
  responsibility each (synthesis §Module Layout, lines 80–108). A reader
  scanning `src/striatum/corpus/` will not have to choose between
  alternatives.
- Enumeration sources are named with concrete CLI or function entrypoints,
  not vague "the dispatcher" prose. Run-summary enumeration explicitly
  routes through `striatum.cli.run_summary.run_summary_snapshot(...)` and
  `render_run_summary_markdown(...)`, the same helper path used by
  `run_summary_export(...)` (synthesis §Enumeration Sources, `run_summary`
  bullet, line 133). The implementation-regression grep at lines 282–284 is
  a strong ergonomics cue that "use the run-summary surface" is enforced,
  not aspirational.
- Redaction policy is a concrete denylist with named per-field rules
  (synthesis §Redaction Policy, lines 142–161). `.env`, transcripts,
  `.striatum/state.sqlite3` blobs, terminal output, patch text, and
  Markdown outside the closed source list are all explicitly excluded.
  Event/audit emission uses an *allowlist* of structural ids/enums/
  timestamps/hashes (line 158), which is the ergonomically safer default:
  a future event-table column added by a migration is invisible to the
  exporter by construction, instead of leaking.
- JSONL emission shape locks to RFC 0044 §3 row shape and the `external_id`
  table; the ordering rule is named ("fixed file order; within each file
  sort by `(external_id, provenance.path, observed_at)`"; compact UTF-8
  JSON with `ensure_ascii=False, separators=(",", ":"), sort_keys=False`;
  final newline; zero-row files still emitted) (synthesis §JSONL Shape And
  Ordering, lines 163–209). Re-export stability is therefore named, not
  hand-waved.
- Manifest fields are enumerated with a complete sample object including
  `bundle_sha256` computed from the canonical manifest bytes with
  `sort_keys=True` (synthesis §Manifest, lines 212–254). Verification
  rules are listed.
- Augmentation-not-dependency is enforced by a named regression grep
  (`^(import|from) engram` and `memory\.`) across named directories
  (synthesis §Augmentation-Not-Dependency Checks, lines 256–267) and a
  named pyproject assertion. Striatum workflow commands (`ack`,
  `publish-artifact`, `complete`, `verdict`, recovery, run prepare/start)
  are explicitly named as forbidden importers. The exporter must succeed
  with `/home/halbritt/git/engram` missing and `engram-mcp-stdio` absent
  from PATH.
- Tests have exact file paths and the integration test
  (`tests/test_corpus_export_integration.py`, line 278) operates against
  a real temporary Striatum run that exercises the existing
  `run_summary_export` path, not a synthetic fixture.

## Findings (low severity)

### F1. `--help` text is not drafted

Cited section: synthesis §CLI Verb Wiring, lines 28–36. The parser code
shows:

```python
corpus = sub.add_parser("corpus")
corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
corpus_export = corpus_sub.add_parser("export")
corpus_export.add_argument("--since", required=True)
corpus_export.add_argument("--out", required=True)
corpus_export.add_argument("--json", action="store_true")
```

None of `add_parser(...)`, `add_subparsers(...)`, or `add_argument(...)`
calls include `help=` / `description=` strings. A first-time operator
running `striatum corpus --help` or `striatum corpus export --help` will
see only argument names with no semantic content. The prompt's stated
ergonomics check ("`striatum corpus export --since <ref> --out <path>`
operator-discoverable from `--help`") is therefore not yet satisfied.

Implementation should pin help strings now so the reviewer of the
implementation PR can match against drafted text. Suggested minimum:

- `corpus`: `help="Export Striatum corpus bundles for downstream ingestion."`
- `corpus_export`: `help="Write a deterministic, redacted JSONL bundle to <out>."`
- `--since`: `help="Git ref (tag, branch, or commit) marking the inclusive lower bound. Files changed in <since>..HEAD and worktree-dirty files are included."`
- `--out`: `help="Output directory. Must be inside the repository and outside .striatum/. Created if missing."` — note that the synthesis does not pin whether `--out` is created on demand or required to pre-exist; that decision belongs in `--help` too (see F4).
- `--json`: `help="Emit a {\"ok\": ...} JSON envelope on stdout instead of human-readable status."`

### F2. JSON error envelope examples are not given for `--since` parse failure or `--out` permission failure

Cited section: synthesis §CLI Verb Wiring, lines 58–64. The synthesis lists
exit codes (0/1/6/8) and says "With `--json`, success prints
`{"ok": true, "data": ...}` and `StriatumError` prints
`{"ok": false, "error": {"message": "...", "code": <exit_code>}}`." That
re-uses the existing envelope and is consistent with `run summary` and
`recovery` verbs.

What is missing is concrete envelope text for the two failure shapes the
prompt names. Without drafted examples, the implementer has to invent
`message` strings, and the integration test
(`test_cli_corpus_export.py`, line 277) cannot pin expected strings.

Suggested drafts to add to the design:

```json
{"ok": false, "error": {"message": "unknown --since ref: v9.99.9", "code": 8}}
{"ok": false, "error": {"message": "--out is not writable: /repo/docs/corpus-export/v1.34.0 (permission denied)", "code": 1}}
```

Also clarify whether `--out` pointing at a path outside the repository or
under `.striatum/` produces code 8 with `message` distinguishable from the
"unknown --since" case. The synthesis groups all four under code 8 (line
63) but does not pin distinct messages, which matters for a first-time
user reading the error.

### F3. `dispatch.py` dispatcher function is named only by structural reference

Cited section: synthesis §CLI Verb Wiring, lines 40–56. The synthesis
names `build_parser()` in `parser.py` (good) and says "the existing
`dispatch.main()` result and error envelope" (line 58, good). However the
location of the new dispatch branch is described as "inside the `with
connect(repo) as conn:` block, near `evidence export` and `run summary`"
(line 47). That is a structural reference rather than a function name.

For a first-time implementer, "near `evidence export` and `run summary`"
is discoverable in practice because `dispatch.main()` is short enough to
read end-to-end, but the prompt's stated check ("CLI verb wiring names the
exact `parser.py` and `dispatch.py` functions, not 'the CLI dispatcher'")
asks for a named function. Pin it explicitly: the new branch lives inside
`striatum.cli.dispatch.main(...)`, alongside the existing `args.command ==
"evidence"` and `args.command == "run"` branches.

### F4. `--out` creation semantics are not pinned

Cited section: synthesis §CLI Verb Wiring, lines 60–64 and §Manifest,
line 222 (`"out": "docs/corpus-export/v1.34.0"`). The exit-code list says
code 8 covers "a non-directory output target" but does not say what
happens when `--out` is a path that does not yet exist. The sample result
payload (lines 67–76) writes to `docs/corpus-export/v1.34.0`, which on a
fresh checkout will not pre-exist.

Pin one of: (a) the exporter creates `--out` with `mkdir(parents=True,
exist_ok=True)`; (b) the exporter requires `--out` to exist and refuses
with code 8 otherwise. Either is defensible; the unspecified behavior is
not. This is the kind of decision a first-time operator will discover by
trial and error, which is the failure mode ergonomics_dx is trying to
prevent.

### F5. Non-JSON (human-readable) output shape is not described

Cited section: synthesis §CLI Verb Wiring, lines 58–76. The result payload
example is shown only as the `--json` envelope. The synthesis is silent on
what `striatum corpus export --since v1.34.0 --out docs/corpus-export/v1.34.0`
(without `--json`) writes to stdout on success or failure.

`run summary` and `evidence export` both have human-readable defaults in
the existing CLI; a first-time user invoking `corpus export` without
`--json` should see something useful (a one-line "wrote N rows across K
files to <out>; manifest sha256 <short>"), not silence and not the raw
envelope.

### F6. Redaction "fail loud on unknown field" contract is asymmetric

Cited section: synthesis §Redaction Policy, lines 142–161. For repo-local
event/audit rows the policy is allowlist-only ("Keep only structural
ids/enums/timestamps/hashes…"; line 158), which fails loud on unknown
fields by construction — good.

For live run-summary snapshots the policy says they "pass through
`redact_evidence_payload(...)` before rendering. Verdict rationales,
blocker descriptions, and unknown fields therefore become
`<redacted-free-text>`" (line 157). The "unknown fields therefore" clause
is a claim about `redact_evidence_payload(...)`'s behavior, not a contract
pinned in this design. The implementation should either (a) restate the
contract as a redaction-test assertion (e.g., add to
`tests/test_corpus_redaction.py`: "a run-summary snapshot with an
unrecognized top-level key emits `<redacted-free-text>` for that key, not
a pass-through"), or (b) cite the existing test that already pins this in
the evidence-export code path. Otherwise a future change to
`redact_evidence_payload(...)` could silently weaken the corpus exporter.

### F7. `--since` accepts what, exactly?

Cited section: synthesis §Enumeration Sources, line 112. "Resolve `--since`
with `git rev-parse --verify <ref>^{commit}` before writing anything." That
sentence implies any git rev-parseable ref (tag, branch, commit, `HEAD~3`,
etc.) is accepted. From an ergonomics_dx standpoint that is the right
behavior, but it should be stated in the `--help` text and the design
prose, not implied via the verification command. A first-time operator
seeing `--since <ref>` may assume "tag only" or "version-string only".

## Operator-discoverability summary

After F1–F7 are addressed in the implementation PR, a first-time operator
should be able to:

1. Run `striatum corpus --help` and `striatum corpus export --help` and
   learn what the command does, what `--since` and `--out` accept, and
   whether `--out` is auto-created.
2. Trigger the two named failure modes (`--since` parse failure, `--out`
   permission failure) and recognise the JSON envelope.
3. Run without `--json` and see a useful human-readable line.
4. Trust that any field added to events or run-summary snapshots later
   does not silently appear in the corpus bundle.

None of the above blocks acceptance of the synthesis; they are
implementation-PR-scope clarifications that the design should pin before
the first `make test` round.

## Out-of-scope notes (non-findings)

- The synthesis chooses `src/striatum/corpus/` over alternative layouts
  explicitly (lines 22–23); this is the stricter Striatum-local shape and
  matches the augmentation-not-dependency posture. No finding.
- The `audit:<store>:<row-id>` namespace refinement (line 134, line 203)
  resolves the RFC 0044 §3 table ambiguity without breaking the documented
  `audit:` family. No finding.
- Implementation sequence (lines 290–296) lists a sensible order. No
  finding.
