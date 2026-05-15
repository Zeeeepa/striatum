---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0048", "phase-c", "design-review", "ergonomics_dx", "read-handlers"]
---
author: reviewer-unknown-model-001

# RFC 0048 Phase C Read-Surface Synthesis Review

Fresh-context, ergonomics-dx review of `docs/dogfood/060/DESIGN_SYNTHESIS.md`. All seven mandatory checks pass. Three ergonomics_dx findings are recorded as follow-ups; none rises to a mandatory bounce.

## Mandatory checks

### 1. All read methods enumerated (PASS)

Synthesis L10 declares the inventory: 8 CLI surfaces / 12 RPC methods. Every method block has legacy-source + new-handler + test triples:

| Method | Legacy citation | Handler path | Test path | Synthesis lines |
|---|---|---|---|---|
| `status` | `src/striatum/cli/introspect.py:170-225` `status()` | `daemon_pg/handlers/reads/status.py` | `tests/daemon_pg/handlers/reads/test_status.py` | L38-43 |
| `dashboard` | `src/striatum/dashboard.py:84-211` `gather_payload()` | `reads/dashboard.py` | `test_dashboard.py` | L48-55 |
| `list.runs` | `src/striatum/cli/list_commands.py:93-119` `list_runs` | `reads/list_runs.py` | `test_list_runs.py` | L63 |
| `list.sessions` | `list_commands.py:122-168` `list_sessions` | `reads/list_sessions.py` | `test_list_sessions.py` | L64 |
| `list.jobs` | `list_commands.py:171-223` `list_jobs` | `reads/list_jobs.py` | `test_list_jobs.py` | L65 |
| `list.artifacts` | `list_commands.py:226-266` `list_artifacts` | `reads/list_artifacts.py` | `test_list_artifacts.py` | L66 |
| `list.workflows` | `list_commands.py:269-288` `list_workflows` | `reads/list_workflows.py` | `test_list_workflows.py` | L67 |
| `run.summary` | `cli/run_summary.py:23-38` + `:41-110` | `reads/run_summary.py` | `test_run_summary.py` | L73-79 |
| `why` | `cli/introspect.py:564-681` `why()` | `reads/why.py` | `test_why.py` | L85-91 |
| `doctor` | `cli/introspect.py:1204-1808` `doctor()` | `reads/doctor.py` | `test_doctor.py` | L97-103 |
| `evidence.export` | `cli/evidence.py:356-383` + `:386-426` | `reads/evidence_export.py` | `test_evidence_export.py` | L109-115 |
| `corpus.export` | `corpus/export.py:16-48` `export_corpus_bundle()` | `reads/corpus_export.py` | `test_corpus_export.py` | L121-127 |

Spot-checked every legacy citation against the cited code:
- `introspect.py:170` opens `def status(conn, *, run_id)` and the function body runs to L225 (return).
- `dashboard.py:84` opens `def gather_payload(repo, *, run_id)` ending at L211.
- `list_commands.py:93,122,171,226,269` open `list_runs/list_sessions/list_jobs/list_artifacts/list_workflows` at the cited line ranges.
- `run_summary.py:23` and `:41` open `run_summary_export` and `run_summary_snapshot`.
- `introspect.py:564` opens `def why(conn, *, target_id)` ending at L681 with the `NotFoundError`.
- `introspect.py:1204` opens `def doctor(conn, *, repo, run_id, verbose)` returning at L1808.
- `evidence.py:356` and `:386` open `evidence_export` and `evidence_snapshot`.
- `corpus/export.py:16` opens `def export_corpus_bundle(conn, *, repo, since, out_text)` returning at L48.

All citations match. No method omitted.

### 2. Return-shape parity contract per method (PASS)

Every method block lists exact top-level keys, not "similar shape to legacy":

- `status` (L41): `runs, provenance_mode, sessions, jobs, open_blockers, human_checkpoints, latest_non_accepting_review_verdicts, verdicts_by_posture, claimable_jobs, blocked_downstream_jobs, process_health, next_actions` plus optional `phases, current_phase_id` — matches the dict literal at `introspect.py:207-224`.
- `dashboard` (L53): `run, status, events, verdict_counts, posture_counts, updated_at, workflow, node_states, override_verdict_counts, override_verdicts` — matches `dashboard.py:200-211`.
- All `list.*` (L59): `items, count` via the legacy `_envelope()` shape (`list_commands.py:83-84`).
- `run.summary` (L77): `status, run_id, path, sha256` — matches `run_summary.py:38`.
- `why` (L89): seven `target_type` branches with explicit per-branch key lists matching `introspect.py:569-677`.
- `doctor` (L101): `ok, schema_version, problems`, plus `problem_records` when verbose — matches `introspect.py:1801-1807`.
- `evidence.export` (L113): `status, run_id, path, sha256` — matches `evidence.py:383`.
- `corpus.export` (L125): `status, since, out, manifest_path, row_counts, bundle_sha256` — close to `CorpusBundleResult.to_json()` at L40-48.

No "TODO" / "see synthesis section X" placeholders. Contract is explicit.

### 3. repository_id scoping mechanism (PASS)

Synthesis L15 specifies the discipline directly: every SQL statement touching a workflow table must include an explicit `WHERE <alias>.repository_id = %(repository_id)s` predicate, and every join must add `joined.repository_id = base.repository_id`. The shared `reads/_sql.py` may inject `repository_id` into params but **must not** hide the predicate behind a wrapper "because reviewers need grep-visible scope discipline." This is a hard "no wrapper" stance, which is what the check asks for.

### 4. Single implement track (PASS)

Synthesis L9 ("Dogfood-060 ports the daemon-RPC read surface to native Postgres handlers in one implementation track") and L20 ("Single implementation track: locked. Native sub-agents may split local work by cluster inside the one implementer session, but Striatum gets one owner for `reads/_read_model.py`, exports, list filters, and tests") explicitly lock single-track. The cycle-exhaustion lesson from dogfood-058 is honored.

### 5. Parity test strategy (PASS)

Synthesis L19 commits to per-key diffs: "seed equivalent SQLite and PG rows, run the legacy function and PG handler, and assert per-key diffs after normalizing timestamp formatting and generated export paths. Shape-only smoke is rejected because `status`, `doctor`, and evidence shapes have changed recently enough that smoke would miss real regressions." Fixture path (`tests/daemon_pg/handlers/reads/conftest.py`), helper name (`assert_payload_parity`), and per-method per-key acceptance criteria are concrete (e.g., `dashboard` ignores only `updated_at` at L54; `corpus.export` normalizes timestamps + substrate version at L126). No "we'll decide in implementation."

### 6. Decorator + signature mirrors Phase A (PASS)

Every method block specifies `@register_pg_handler("<method>", read_only=True)` and `def handle(ctx: RepoHandlerContext, params: Mapping[str, Any]) -> dict[str, Any]` (status block at L39; all subsequent methods say "same `(ctx, params)` signature"). Synthesis L16 explicitly forbids a new registry or router mechanism and only requests an optional `read_only: bool = True` kwarg as a test hook. This matches the prompt's Phase A pattern verbatim.

Note for the implementer (informational, not a finding): the current `register_pg_handler` at `src/striatum/daemon_pg/handlers/registry.py:15` takes `*methods: str` only — adding the optional `read_only` kwarg is a one-line signature extension, not a new decorator.

### 7. Registration locked (PASS)

Synthesis L17: "add one line in `src/striatum/daemon_pg/handlers/__init__.py`: `from . import reads as reads`. Add `reads/__init__.py` that imports each method file for decorator side effects." This matches the existing pattern at `daemon_pg/handlers/__init__.py:5-10` (which already does `from . import workflow_loop as workflow_loop` and a guarded `recovery_evidence` import). No alternate registration mechanism.

## Ergonomics_dx findings (recorded as follow-ups; do not block accept)

### F1. Handler error messages do not commit to operator-actionable next-command text

Synthesis names error codes (`schema_invalid`, `not_found`, `repo_not_registered`, `path_outside_scope`) but does not say the error *message* will cite the operator's next command. For example, `why`'s unknown-target error at `introspect.py:679-681` reads "target id is not a known run, job, message, blocker, artifact, verdict, session, or process" — informative, but does not point at `striatum list runs --limit N` or `striatum why <id>` with a fresh id. The synthesis declares "preserves legacy message" (L91) which keeps parity but locks in the legacy ergonomics gap.

**Suggested fix in implementation:** for `repo_not_registered`, append "run `striatum repo register --repo <path>` first"; for `not_found` on `why`/`dashboard`/`doctor`, append "try `striatum list runs` to see known run ids."

### F2. Parity test failure output is not explicitly specified as per-key diff

Synthesis L29 names `assert_payload_parity` as a conftest helper, and L19 commits to "per-key diffs after normalizing." Together these imply the helper will emit a per-key diff on failure, but the synthesis does not say so. A naïve implementation that does `assert pg_payload == sqlite_payload` inside `assert_payload_parity` would still satisfy the literal text of the synthesis while producing the unhelpful "Pyhton dict mismatch" output the ergonomics check warns against.

**Suggested fix in implementation:** `assert_payload_parity` should compute the symmetric per-key diff (e.g., via `deepdiff` or a hand-rolled walker) and format keys with values inline before raising, not pass the two dicts to `assert ==`.

### F3. `status` returning legacy empty/phase-less shape on unknown `run_id` is a parity choice that hides a missing run

Synthesis L43: "unknown `run_id` preserves legacy empty/phase-less status shape." This is faithful to `introspect.py:174-184` (the legacy SQL uses `(? IS NULL OR run_id = ?)` and silently returns an empty `runs` list for unknown ids). The synthesis is correct to preserve parity, but it locks in a legacy ergonomics gap where a typo in `--run-id` returns "everything looks empty" instead of "unknown run_id."

**Suggested fix as a follow-up** (not this dogfood): a future RFC could add a `--strict` flag or a separate `status.run` method that returns `not_found` on unknown run, matching `dashboard`'s and `doctor`'s behavior (synthesis L55 and L103). Out of scope for Phase C; record it as a friction item.

## Verdict

`accept_with_findings`. All seven mandatory checks pass with citation-level evidence. The three ergonomics_dx findings (F1 error-message actionable text, F2 explicit per-key diff output, F3 status-on-unknown-run parity tradeoff) are recorded for the implementer and a future ergonomics RFC; none warrants bouncing the synthesis back to the design cycle.
