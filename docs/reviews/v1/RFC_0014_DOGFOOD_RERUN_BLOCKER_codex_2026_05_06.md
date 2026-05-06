# RFC 0014 Dogfood Rerun Blocker - Codex

Date: 2026-05-06
Coordinator: Codex GPT-5.5
Branch checked: `agent-runner/rfc-0014-validation`
Prompt: `agent-runner/prompts/P004_rerun_rfc_0014_dogfood.md`
Verdict: blocked

## Scope

Ran the P004 preflight gate for a fresh RFC 0014 dogfood validation rerun.
The rerun was not started because the evidence-export redaction gate is not
fully satisfied.

Verification run during preflight:

```bash
cd agent-runner
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Results:

- `28 passed in 19.75s`
- workflow validation returned `valid: true` for
  `rfc-0014-operational-artifact-home`

## Blocker

`agent_runner evidence export` still includes workflow job titles in the
default snapshot path. The P004 preflight explicitly requires default evidence
export to omit free-text blocker descriptions, verdict rationales, and workflow
job titles before the dogfood rerun starts.

Observed implementation state:

- `agent-runner/src/agent_runner/cli.py` defines
  `EVIDENCE_FREE_TEXT_KEYS = {"description", "rationale"}`.
- `evidence_job_summaries` still adds `"title": job["title"]` to the exported
  job summary.
- `agent-runner/tests/test_cli_mvp.py` asserts redaction of private verdict
  rationale text and state/transcript markers, but it does not include the P004
  sentinel assertion proving that a private-looking job title is absent from
  exported evidence.

Because this is a validation rerun prompt rather than a code-build prompt, no
runner implementation changes were made. Per P004, the RFC 0014 dogfood
workflow was not prepared, started, or advanced.

## Required Fix Before Rerun

Add default title redaction or title omission to evidence export, and add a
sentinel regression test where a private-looking workflow job title is absent
from the exported evidence. Then rerun the P004 preflight and only start the
dogfood workflow if the redaction gate passes.

## Follow-Up

The human owner directed the concrete fix on 2026-05-06: evidence should use
role id, lane id, declared model display name, and workflow job id instead of
workflow job title prose; durable artifacts should include an `Author:` line
with that same identity. Those owner decisions are recorded as D039 and D040 in
`agent-runner/docs/DECISION_LOG.md`.
