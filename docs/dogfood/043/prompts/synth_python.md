# Track A Synthesis Prompt: RFC 0045 Python Core

Produce `docs/dogfood/043/DESIGN_SYNTHESIS_python.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/043/design/python/codex/DESIGN.md", "docs/dogfood/043/design/python/claude_code/DESIGN.md", "docs/dogfood/043/design/python/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 Python designs into ONE concrete plan for RFC 0045 V1:

- Exact schema accepted by the validator: `striatum.workflow.v1` and `striatum.workflow.v1.1` strings, optional `phases[]` shape, optional `phase_id` on jobs.
- `phase_synthesis` job type contract — fan-in/fan-out enforcement.
- Exact validator rule for cross-phase edges (allowed paths through `phase_synthesis` only).
- Runtime materialization site: which function(s) in `src/striatum/workflow.py` get the new code, and what data structures hold phase metadata at runtime.
- Status reporting site: which functions in `src/striatum/dashboard.py` and `src/striatum/service.py` are extended; field shape for per-phase progress.
- Generator catalog `multi_phase` shape — exact files under `src/striatum/workflow_generator/`.
- `striatum workflow upgrade --add-phases` exact CLI shape and inference rules.
- Backwards-compatibility test matrix: which existing v1 fixtures must continue to pass unchanged.
- Test plan: new fixtures, e2e test that exercises a v1.1 multi-phase workflow lifecycle.

Choose; do not enumerate. Output is a SPECIFIC plan ready to implement against. If the three designs disagree on a point, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
