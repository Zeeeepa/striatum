# Design task — RFC 0053 Phase B schema rename

Produce `docs/dogfood/063/design/<lane>/DESIGN.md`
(`handoff.v1`, byline `author: designer-<lane>-<model>-001`)
answering the seven questions in `roles/designer.md`. One concrete
choice each.

**Deliverable sections** (mirror the workflow.json design objective):

1. Dual-name read discipline (validator + handlers + CLI).
2. `workflow upgrade <path>` rule with cited existing
   infrastructure.
3. PG runtime migration discipline (code-level dual-read, no SQL).
4. CLI prompt-string sweep target list with rename-vs-stay
   justification.
5. `escalation` artifact-kind scope decision; historical resolution was
   deferral from this rename slice and landing in follow-up Phase 5 work.
6. Back-compat test plan (4-5 named tests).
7. Deprecation-warning audit-row format.

**Bouncing conditions:** menu instead of locked choice; missing
back-compat coverage of existing v1 workflows under
`docs/dogfood/*`; SQL migration proposed (must stay in code).

**Write scope:** `docs/dogfood/063/design/<lane>/DESIGN.md`.
