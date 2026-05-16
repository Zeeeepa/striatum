# Implementer Role (Dogfood 063 — RFC 0053 Phase B schema rename)

author: implementer-role-001

Implement per synthesis. Sub-agents per layer:

1. **Validator** — `src/striatum/workflow.py`: accept both old and
   new field names; emit deprecation warning on old; new name is
   canonical.
2. **Generator** — `src/striatum/workflow_generator.py`: emit only
   new names.
3. **Upgrade** — `src/striatum/cli/workflow.py`: `workflow upgrade
   <path>` rewrites old→new. Idempotent.
4. **CLI prompt-string sweep** — `src/striatum/cli/{recovery,
   parser, list_commands, introspect, evidence}.py`: stderr / help
   text only. Flag NAMES stay.
5. **PG state acceptance** — `src/striatum/daemon_pg/handlers/`:
   handlers accept both `waiting_human` and `waiting_escalation` on
   read; writes use the new name.
6. **Docs** — `docs/UBIQUITOUS_LANGUAGE.md`, `docs/SPEC.md`,
   `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`: update to new
   vocabulary with one-line back-compat note.
7. **Tests** — `tests/test_workflow_rename_v1_2.py` covering: v1
   workflow with old names validates + warns; same after upgrade
   validates without warns; runtime behavior identical; PG-state
   dual-read; flag NAMES unchanged.

**Forbidden:** `.striatum/`, `src/striatum/daemon_pg/sql/` (no
schema migration — the rename is in code, not in SQL), and
`src/striatum/daemon.py`.

**Deliverable:** `docs/dogfood/063/build/HANDOFF.md`.
