---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/escalations.py", "src/striatum/daemon_pg/handlers/workflow_loop/block_job.py", "go/pkg/mutations/lifecycle.go", "tests/daemon_pg/handlers/workflow_loop/test_block_job.py", "go/pkg/mutations/lifecycle_test.go", "docs/SPEC.md", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Item 4 Escalation Payload Hardening Result
author: operator [self-declared: current-todo-item4]

Result: complete.

`work.block` now validates non-empty string fields, bounded blocker kinds, the
`blocked|human_checkpoint` severity set, and bounded descriptions before
mutating state. The Python and Go paths both persist
`striatum.blocker_payload.v1` metadata to `blockers.payload_json`; escalation
class blockers mirror the same payload into `escalation_inbox.payload_json`
and include payload schema metadata on `job.blocked` events.

This preserves D130 link-only behavior: escalation artifacts still link to
existing blockers and do not synthesize live blocker or inbox rows.

Validation:

```bash
.venv/bin/python -m pytest -q tests/daemon_pg/handlers/workflow_loop/test_block_job.py tests/test_artifact_schemas.py::test_escalation_front_matter_validates_human_principal_request tests/test_artifact_schemas.py::test_escalation_front_matter_rejects_unbounded_blocker_kind
cd go && go test ./pkg/mutations
PYTHONPATH=src python3 -m compileall -q src/striatum/escalations.py src/striatum/daemon_pg/handlers/workflow_loop/block_job.py
.venv/bin/python -m mypy src/striatum/escalations.py src/striatum/daemon_pg/handlers/workflow_loop/block_job.py
```

Results: `3 passed, 2 skipped`; Go mutations package passed; compileall and
mypy clean. A direct mypy run including the PG test file was skipped because
that test imports the `_harness.pg` fixture through pytest path setup.
