# P002: Validate agent_runner With RFC 0014

Status: ready
Date: 2026-05-06
Scope: `agent_runner` dogfood validation
Target RFC: `docs/rfcs/0014-operational-artifact-home.md`
Primary outcome: a completed or honestly blocked `agent_runner` validation run
against a bounded Engram RFC review workflow.

## Mission

You are the `agent_runner` validation coordinator.

Your job is to use the `agent_runner` MVP itself to coordinate review,
findings-ledger, synthesis, and final-review work for RFC 0014. This is a
dogfood run: the output is not just the RFC disposition, but evidence about
whether the runner can coordinate a real Engram-style workflow.

Do not fall back to an ordinary single-agent review unless the runner blocks. If
the runner blocks, record the blocker through `agent_runner` where possible and
write a durable validation note explaining what failed.

## Read First

Read these files in order:

1. `agent-runner/README.md`
2. `agent-runner/docs/SPEC.md`
3. `agent-runner/docs/DECISION_LOG.md`
4. `agent-runner/docs/UBIQUITOUS_LANGUAGE.md`
5. `agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`
6. `docs/rfcs/0014-operational-artifact-home.md`
7. `docs/rfcs/0013-development-operational-issue-loop.md`
8. `docs/process/multi-agent-review-loop.md`
9. this prompt

Treat `agent-runner/docs/DECISION_LOG.md` as binding for runner behavior.
Treat Engram RFCs as proposals unless already promoted by Engram's canonical
docs.

## Target RFC

Use RFC 0014 because it is small, process-oriented, and directly connected to
the runner's own operational-artifact pain:

- it touches committed operational artifacts, markers, and review artifacts;
- it depends on RFC 0013 but should not require private corpus data;
- it can exercise parallel reviews, findings normalization, synthesis,
  re-review behavior, artifact publishing, and status introspection;
- it is safe to stop at a proposed disposition if final human approval is still
  needed.

Do not switch to RFC 0011, RFC 0015, or Phase 3 build work during this run.

## Branch Setup

From the Engram repository root:

1. Run `git status --short`.
2. If the worktree has uncommitted changes you did not make, stop and report
   the dirty files.
3. If already on `agent-runner/rfc-0014-validation`, continue.
4. Otherwise create or switch to `agent-runner/rfc-0014-validation`.

The MVP's `branch confirm` command records branch confirmation; it does not
perform the actual `git switch`. For this validation, do the actual git branch
operation yourself, then record the branch with `agent_runner branch confirm`.

## Runner Setup

Use the Engram repo root as the runner repo and `agent-runner/` as the Python
project directory.

Recommended command shape from `agent-runner/`:

```bash
PYTHONPATH=src python -m agent_runner.cli --repo .. init --json
PYTHONPATH=src python -m agent_runner.cli --repo .. workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
PYTHONPATH=src python -m agent_runner.cli --repo .. run prepare --workflow examples/rfc-0014-operational-artifact-home/workflow.json --json
PYTHONPATH=src python -m agent_runner.cli --repo .. branch confirm --run-id <RUN_ID> --branch agent-runner/rfc-0014-validation --use-current --json
PYTHONPATH=src python -m agent_runner.cli --repo .. run start --run-id <RUN_ID> --json
```

Register only the sessions needed by the workflow. Use fresh sessions for jobs
whose work packet says `fresh_session_required: true`.

Expected lanes:

- `reviewer` / `claude`
- `reviewer` / `codex`
- `reviewer` / `gemini`
- `ledger` / `codex`
- `synthesizer` / `claude`
- `reviewer` / `codex` for final review, as a fresh session

Use `claim-next`, `ack`, `publish-artifact`, `complete`, and `verdict` exactly
as the work packet commands specify. Use `status --json`, `why`, and `doctor
--json` when diagnosing state.

## Workflow Discipline

The workflow fixture is:

```text
agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json
```

It should produce durable artifacts under:

```text
docs/reviews/rfc-0014-operational-artifact-home/
```

The root review jobs may run in parallel. They must write only their assigned
review artifact. The ledger job normalizes findings but does not decide them.
The synthesis job decides accepted, modified, deferred, and rejected findings.
The final review checks the synthesis and issues the final gate verdict.

If any review verdict is `needs_revision`, follow the declared runner cycle if
one exists. If a verdict is `reject`, stop expansion and report the failed run
state. Do not manually advance downstream jobs after a rejected or blocked gate.

## Validation Notes

Keep a separate validation note if the runner itself is awkward, incomplete, or
blocks the run:

```text
docs/reviews/rfc-0014-operational-artifact-home/AGENT_RUNNER_VALIDATION_NOTES.md
```

Record runner findings separately from RFC 0014 findings. Examples:

- unclear packet path resolution;
- too much manual command plumbing;
- status output missing the next useful action;
- missing process/tmux adapter behavior;
- difficult re-review cycle handling;
- artifact publication friction;
- confusing branch confirmation behavior.

Do not record private corpus content. RFC 0014 is process documentation; no
private evidence should be needed.

## Completion Criteria

A successful validation run has:

- all three independent RFC 0014 review artifacts;
- a findings ledger;
- a synthesis;
- a final review verdict;
- runner status showing the run completed or honestly blocked;
- `doctor --json` output checked;
- validation notes if any runner friction was observed.

The run may end as blocked if a model lane is unavailable or the MVP cannot
represent a required transition. A blocked run is still useful if the blocker is
recorded durably and the final response is honest.

## Verification

At minimum, run from `agent-runner/`:

```bash
PYTHONPATH=src python -m pytest -q
PYTHONPATH=src python -m agent_runner.cli --repo .. status --json
PYTHONPATH=src python -m agent_runner.cli --repo .. doctor --json
```

If tests fail, stop and record the failure. If tests pass but the workflow
blocks, report both facts.

## Final Response Requirements

Report:

- run id;
- branch name;
- review artifacts written;
- findings ledger and synthesis paths;
- final review verdict;
- tests and runner checks run;
- runner validation findings;
- whether RFC 0014 is ready for human disposition, needs revision, or should
  remain only a proposal.

Do not claim RFC 0014 is accepted unless the synthesis and final review support
that disposition and the human has approved any required canonical doc updates.
