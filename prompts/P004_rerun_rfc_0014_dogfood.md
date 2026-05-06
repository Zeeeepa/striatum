# P004: Re-run RFC 0014 Dogfood Validation

Status: ready
Date: 2026-05-06
Scope: `agent_runner` dogfood validation rerun
Target package:
- `docs/rfcs/0014-operational-artifact-home.md`
- `docs/process/operational-artifact-home-spec.md`
Primary workflow fixture:
`agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`

## Mission

You are the `agent_runner` validation coordinator.

Your job is to run a fresh RFC 0014 dogfood validation after the RFC 0014
runner-recovery fixes and the RFC 0014 spec-handoff revision. The rerun should
prove whether the runner can coordinate the RFC-plus-spec handoff review
workflow with durable, redacted, commit-ready evidence from a fresh run.

This is not a code-build prompt. Do not implement runner changes unless a
preflight check proves the runner is unsafe to use and the human explicitly asks
you to fix it. If the runner is not ready, stop and record the blocker.

## Read First

Read these files in order:

1. `agent-runner/README.md`
2. `agent-runner/docs/SPEC.md`
3. `agent-runner/docs/RFC_0014_DOGFOOD_FIX_SPEC.md`
4. `agent-runner/docs/DECISION_LOG.md`
5. `agent-runner/docs/UBIQUITOUS_LANGUAGE.md`
6. `agent-runner/prompts/P002_validate_agent_runner_with_rfc_0014.md`
7. `agent-runner/docs/reviews/v1/RFC_0014_DOGFOOD_FIX_REVIEW_codex_2026_05_06.md`
8. `agent-runner/docs/reviews/v1/RFC_0014_DOGFOOD_FIX_REREVIEW_codex_2026_05_06.md`
9. `docs/reviews/rfc-0014-operational-artifact-home/AGENT_RUNNER_VALIDATION_NOTES.md`
10. `agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`
11. `docs/rfcs/0014-operational-artifact-home.md`
12. `docs/process/operational-artifact-home-spec.md`
13. this prompt

Treat `agent-runner/docs/DECISION_LOG.md` as binding for runner behavior.
Treat Engram RFCs as proposals unless they have already been promoted by
Engram's canonical docs.

## Preflight Gate

Before starting a new dogfood run, verify the known evidence-redaction issue is
resolved:

- `agent_runner evidence export` must not emit free-text blocker descriptions,
  verdict rationales, or workflow job titles by default.
- The test suite should include or be updated with a sentinel assertion proving
  that a private-looking job title does not appear in exported evidence.
- If this is still unfixed, do not run the RFC 0014 dogfood workflow. Record a
  blocker under `agent-runner/docs/reviews/v1/` and report that the rerun is
  blocked by unsafe evidence export.

Run from `agent-runner/`:

```bash
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Only continue if both commands pass and the redaction issue is resolved.

## Branch Discipline

Work on the current `agent-runner/rfc-0014-validation` branch unless the human
explicitly redirects you.

Before editing or running the workflow:

1. Run `git status --short --branch`.
2. If the worktree has uncommitted changes you did not make, inspect them and
   preserve them.
3. Do not delete or rewrite `.agent_runner/` state unless the human explicitly
   asks for that. Use a new run id and pass `--run-id` to runner commands.
4. Do not overwrite the first RFC 0014 validation artifacts. This rerun must use
   a distinct artifact directory.

## Rerun Artifact Directory

Create a distinct rerun directory:

```text
docs/reviews/rfc-0014-operational-artifact-home/reruns/<RUN_SLUG>/
```

Use a stable `RUN_SLUG` such as:

```text
2026-05-06-redaction-rerun
```

If that directory already exists, add a short suffix rather than overwriting it.

Create a rerun workflow copy under the same directory:

```text
docs/reviews/rfc-0014-operational-artifact-home/reruns/<RUN_SLUG>/workflow.json
```

Base it on:

```text
agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json
```

In the rerun copy:

- keep the same workflow id unless the runner requires otherwise;
- set `workflow_version` to include the rerun slug;
- preserve the handoff spec as a required context doc and as a review,
  synthesis, and final-review input;
- rewrite every expected review, ledger, synthesis, final-review, and evidence
  artifact path into the rerun directory;
- rewrite ledger/synthesis/final-review input paths to the rerun artifacts;
- set each job write scope `allowed_paths` to the rerun directory;
- keep forbidden paths including `.agent_runner/`;
- keep the root-review `needs_revision` human-checkpoint policy unless the
  human explicitly asks to test a declared root-review revision cycle.

Commit-ready outputs from this run should live under the rerun directory, not
beside the original validation files.

## Runner Setup

Use the Engram repo root as the runner repo and `agent-runner/` as the Python
project directory.

Recommended command shape from `agent-runner/`:

```bash
PYTHONPATH=src python3 -m agent_runner.cli --repo .. init --json
PYTHONPATH=src python3 -m agent_runner.cli --repo .. workflow validate ../docs/reviews/rfc-0014-operational-artifact-home/reruns/<RUN_SLUG>/workflow.json --json
PYTHONPATH=src python3 -m agent_runner.cli --repo .. run prepare --workflow ../docs/reviews/rfc-0014-operational-artifact-home/reruns/<RUN_SLUG>/workflow.json --json
PYTHONPATH=src python3 -m agent_runner.cli --repo .. branch confirm --run-id <RUN_ID> --branch agent-runner/rfc-0014-validation --use-current --json
PYTHONPATH=src python3 -m agent_runner.cli --repo .. run start --run-id <RUN_ID> --json
```

Use the returned `RUN_ID` for every subsequent `status`, `doctor`, `why`, and
`evidence export` command.

## Model Lanes

Use the three independent model lanes defined by the workflow:

- Claude Opus lane for Claude review and synthesis jobs;
- Codex GPT-5.5 lane for Codex review, ledger, and final-review jobs;
- Gemini 3.1 Pro Preview lane for Gemini review jobs.

Register only the sessions needed by the workflow. Use fresh sessions for jobs
whose work packet says `fresh_session_required: true`.

Recommended session slugs:

- `reviewer-claude-001`
- `reviewer-codex-001`
- `reviewer-gemini-001`
- `ledger-codex-001`
- `synthesizer-claude-001`
- `reviewer-codex-002`

If a model lane is unavailable, stop and record a runner validation blocker.
Do not silently replace one model with another.

## Execution Discipline

For each job:

1. Claim the job with `claim-next`.
2. Read the work packet.
3. Use the packet's command strings wherever possible.
4. Write only the expected artifact path for that job.
5. For review jobs, prefer `submit-review`.
6. For non-review jobs, use `ack`, `publish-artifact`, and `complete`.

Root review jobs may run in parallel. They must remain independent and must not
read each other's draft review artifacts before submission.

The review target is the RFC-plus-spec handoff package. The RFC is the
proposal/history record; the spec is the implementation handoff. Do not treat
"explicit choices live in the spec" as a defect unless the RFC and spec
contradict each other or the spec is missing from the work packet.

The ledger job normalizes review findings and does not decide package
readiness. The synthesis job decides accepted, modified, deferred, and rejected
findings and recommends whether the package is ready for implementation
handoff or still needs package revision. The final review checks whether the
synthesis is internally supported and whether the dogfood evidence is adequate.

## Blocked Or Rejected Gates

If any root review returns `needs_revision`, treat the configured human
checkpoint as the expected stop state. Do not manually advance ledger,
synthesis, or final-review jobs.

If the final review returns `needs_revision`, follow the declared cycle back to
synthesis once. Do not exceed the workflow's `max_iterations`.

If any review returns `reject`, stop expansion and report the failed run state.

In every blocked or rejected case:

- run `status --json --run-id <RUN_ID>`;
- run `why <BLOCKER_OR_VERDICT_ID> --json` when available;
- run `doctor --json --run-id <RUN_ID>`;
- export evidence into the rerun directory;
- record a short validation note in the rerun directory.

## Evidence Export

At the end of the run, and also when blocked, export evidence:

```bash
PYTHONPATH=src python3 -m agent_runner.cli --repo .. evidence export \
  --run-id <RUN_ID> \
  --path docs/reviews/rfc-0014-operational-artifact-home/reruns/<RUN_SLUG>/RUN_EVIDENCE.md \
  --json
```

Then inspect `RUN_EVIDENCE.md` before committing it:

- it must not contain `.agent_runner/state.sqlite3`;
- it must not contain transcript text;
- it must not contain private-looking sentinel strings used in redaction tests;
- it must not contain workflow job titles if title redaction is the selected
  fix;
- it should contain enough status, doctor, blocker, verdict, artifact, and
  dependency evidence for a fresh checkout to understand the run.

## Required Rerun Artifacts

The rerun directory should contain, as applicable:

- `workflow.json`
- `RFC_0014_REVIEW_claude.md`
- `RFC_0014_REVIEW_codex.md`
- `RFC_0014_REVIEW_gemini.md`
- `RFC_0014_FINDINGS_LEDGER.md`
- `RFC_0014_SYNTHESIS.md`
- `RFC_0014_FINAL_REVIEW.md`
- `RUN_EVIDENCE.md`
- `VALIDATION_NOTES.md`

If the run blocks before some artifacts are reachable, do not fabricate them.
Instead, explain why they are absent in `VALIDATION_NOTES.md`.

## Verification

Before final response, run from `agent-runner/`:

```bash
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli --repo .. status --run-id <RUN_ID> --json
PYTHONPATH=src python3 -m agent_runner.cli --repo .. doctor --run-id <RUN_ID> --json
git diff --check
```

If tests fail, stop and record the failure. If tests pass but the workflow
blocks honestly, report both facts.

## Final Response Requirements

Report:

- branch name;
- run id;
- rerun slug and rerun directory;
- sessions registered;
- artifacts written;
- review verdicts;
- final run state;
- `status`, `doctor`, test, workflow-validation, and whitespace-check results;
- evidence export path and redaction inspection result;
- runner validation findings;
- whether the RFC-plus-spec handoff package is ready for a later
  implementation prompt, needs package revision, or remains only a proposal.

Do not claim RFC 0014 is accepted unless the synthesis and final review support
that disposition and the human has approved any required canonical doc updates.
