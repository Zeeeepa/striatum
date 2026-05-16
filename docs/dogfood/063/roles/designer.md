# Designer Role (Dogfood 063 — RFC 0053 Phase B schema rename)

author: designer-role-001

Answer the 7 questions in workflow.json's design objective. Key
constraint: every existing v1 workflow in `docs/dogfood/*` and
`examples/*` MUST continue to validate after the rename (with
deprecation warning OK), and `workflow upgrade <path>` MUST be
idempotent.

**Must read first:**

- `docs/rfcs/0053-human-principal-and-terminology-truing.md`
- `src/striatum/workflow.py` (the validator — search for
  `human_checkpoint`, `waiting_human`)
- `src/striatum/cli/workflow.py` (the upgrade infrastructure)
- `src/striatum/workflow_generator.py` (the generator that produces
  new workflows — emits the new names)
- `src/striatum/cli/parser.py` line ~546 (--severity choices)
- `src/striatum/cli/recovery.py`, `src/striatum/cli/list_commands.py`,
  `src/striatum/cli/introspect.py`, `src/striatum/cli/evidence.py`
  for current `human_checkpoint` / `waiting_human` literal references

**Scope tension:** the `escalation` artifact-kind work (front-matter
schema + RPC methods) is borderline scope creep. Argue for inclusion
(one coherent dogfood) or deferral (smaller dogfood, faster ship).

**Write scope:** `docs/dogfood/063/design/<lane>/DESIGN.md`.
