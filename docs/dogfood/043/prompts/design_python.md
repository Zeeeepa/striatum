# Track A Design Prompt: RFC 0045 Python Core

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/043/design/python/<lane>/`).

Design **RFC 0045 V1 acceptance criteria** for the Python-core multi-phase workflow schema. Read the RFC first: `docs/rfcs/0045-multi-phase-workflow-editor-and-schema.md`.

Cover concretely:

- **Schema bump**: `schema_version` accepts both `striatum.workflow.v1` and `striatum.workflow.v1.1`. The latter unlocks an optional `phases: [{id, name, color?, description?}]` array at the workflow root, and jobs gain an optional `phase_id` field.
- **`phase_synthesis` job type**: new job type whose contract is "fan-in from prior phase, fan-out is the next phase's entry jobs". Validator must enforce that any edge crossing phases originates from or terminates at a `phase_synthesis` job.
- **Validator rules** (`src/striatum/workflow.py`): cross-phase dependencies must route through synthesis; intra-phase edges flow freely; absent `phases` array means v1 single-phase behavior. Cite existing validator functions you will extend.
- **Runtime materialization**: how jobs and edges are loaded into runtime state. Identify the exact functions in `src/striatum/workflow.py` that build the job graph. Phases are materialized as metadata; the executor still acts on jobs and edges.
- **Status reporting**: `src/striatum/dashboard.py` and `src/striatum/service.py` must surface per-phase progress (jobs done / total per phase). Identify the existing status aggregation code you will extend.
- **Generator catalog**: `src/striatum/workflow_generator/` gains a `multi_phase` shape that emits a `phases` array plus `phase_synthesis` jobs at boundaries.
- **New CLI verb**: `striatum workflow upgrade --add-phases <workflow.json>` rewrites a v1 workflow to v1.1 by inferring phases from `parallel_group` clusters (or accepting an explicit phase map flag).
- **Backwards compatibility**: every existing v1 workflow MUST continue to validate and execute unchanged. Lay out the test matrix.

Designers MUST cite existing code in `src/striatum/workflow.py` (function names, line refs). Hand-waving the validator extension is grounds for design review to bounce.

Out of scope: frontend (Track B), RFC 0045 §6 future work items not in V1 acceptance.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
