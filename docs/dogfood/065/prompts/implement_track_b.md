# Implement Track B prompt - SQLite eradication

Produce `docs/dogfood/065/build/track_b/HANDOFF.md` as a handoff artifact.
Use a title block with `author: implementer-claude_code-claude_code-001`.

Track B owns PostgreSQL-only daemon-global surfaces and legacy SQLite
quarantine. Stay inside the workflow write scope. Do not edit Go code,
contracts, client/service/MCP boundary code, docs decisions, or parent-owned
dogfood files.

Implement per synthesis. Required work items:

1. Gate `connect_registry()` so production callers cannot open the legacy
   SQLite registry outside migration/test compatibility.
2. Port daemon startup bootstrap, health, audit/doctor probes, `dashboard.all`,
   daemon sweep, and daemon MCP resource reads to PostgreSQL-backed DTOs or
   fail closed until the DTO exists.
3. Keep repo-local `.striatum/` as operational scratch only. Do not create or
   open `.striatum/state.sqlite3` from production paths.
4. Quarantine remaining SQLite helpers under migration or fixture namespaces
   with guardrail tests.
5. Add regression coverage with the SQLite tripwire enabled for daemon-global
   commands.

Handoff must include:

- Files changed.
- Tests run and results.
- Remaining SQLite exceptions, each labeled migration-only, fixture-only, or
  still-blocking.
- Any Track C dependency for daemon method exposure.
- Confirmation that `.striatum/`, README.md, OPERATOR_REPORT.md, workflow,
  prompts, and roles were not edited.
