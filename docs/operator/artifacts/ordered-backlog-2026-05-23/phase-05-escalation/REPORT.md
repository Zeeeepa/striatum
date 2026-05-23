---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md", "docs/rfcs/0062-real-escalation-inbox.md", "docs/SPEC.md", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Phase 5 Escalation Policy
author: operator [self-declared: codex-driver]

## Result

Closed RFC 0062's artifact-only escalation creation policy question as
link-only.

Changed:

- Added D130 to the decision log.
- Updated RFC 0062 status and implementation text.
- Updated TODO 53 and roadmap Phase 5 debt.

Current policy:

- Live escalation state is created through `work.block` using an
  escalation-class blocker kind, or through a future accepted creation method.
- Publishing a `striatum.escalation.v1` artifact links to an existing
  escalation-class blocker in the same repository/run.
- `artifact.publish` does not synthesize blockers or escalation inbox rows.

## Validation

```bash
rg -n "artifact-only escalation creation|whether artifact-only|decide artifact-only" docs/DECISION_LOG.md docs/rfcs/0062-real-escalation-inbox.md docs/TODO.md docs/ROADMAP.md docs/SPEC.md
.venv/bin/python -m pytest -q tests/test_doc_links.py -k 'decision or rfc or roadmap or todo'
```

The scan now finds only link-only policy statements, and the focused doc-link
test passed.
