# Gate F — Rewrite current-guidance docs to Go-only

You are the implementer for RFC 0078 Gate F. Runs AFTER Gate D. Read first:
`docs/operator/plans/rfc-0078-remaining-work.md` (Gate F) and
`scripts/python_trace_guardrail.sh` (the `is_current_guidance_path` allowlist
and the `guidance_pattern` regex define exactly which lines count).

## Goal

Drive `active_python_runtime_guidance` to **0** in
`make python-trace-report`, for the doc surfaces in YOUR write scope
(README, AGENTS.md, and `docs/*` current-guidance files — NOT Makefile,
`.github`, or `scripts`, which are Gate E's).

## Steps

1. Run `make python-trace-report --format tsv` (via the script:
   `scripts/python_trace_guardrail.sh --report --format tsv`) and filter rows
   where `classification == blocked` and `class == active_python_runtime_guidance`
   for paths in your scope.
2. Rewrite each flagged line to Go-only install/run/test language:
   `pip install` / `python -m striatum` / `pytest` / `venv` →
   `striatum` binary / `make test` (Go) / release-archive install. Use the
   Go getting-started path already in CI and the Makefile.
3. **Preserve** target-repository Python examples — a workflow may orchestrate
   a target project that uses Python. Only Striatum's own runtime guidance
   changes. (Those examples live under `examples/`, which is not your scope.)
4. Add supersession notes rather than deleting history: mark RFC 0068 and
   RFC 0070's "keep Python CLI/web" carve-outs and the superseded
   Python-for-V1 decision rows in `docs/DECISION_LOG.md` as superseded by
   RFC 0078 (do not erase the provenance).
5. Do NOT touch historical/provenance docs (the guardrail already allowlists
   `docs/rfcs/`, `docs/reviews/`, `docs/operator/artifacts/`, `prompts/`, etc.).

## Validate

```bash
scripts/python_trace_guardrail.sh --report \
  | grep -E 'active_python_runtime_guidance'
# Goal: the count attributable to your scoped paths is 0.
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/docs/SUMMARY.md`
(`artifact_kind: synthesis`): the files rewritten, the supersession notes
added, and the before/after `active_python_runtime_guidance` count. Use your
byline.
