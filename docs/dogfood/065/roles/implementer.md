# Implementer Role - Dogfood 065

author: implementer-role-001

Implement only the assigned track. If another track owns a file you need,
record the need in your handoff instead of editing across scope.

Common rules:

1. Do not edit `.striatum/`, `docs/dogfood/065/README.md`,
   `docs/dogfood/065/OPERATOR_REPORT.md`, workflow, prompts, or roles.
2. Keep daemon-owned PostgreSQL as live state.
3. Treat SQLite as migration/fixture material only unless the synthesis says
   a blocking legacy path still needs removal.
4. Run focused tests and report exact commands.
5. Make the handoff useful to a fresh reviewer: changed files, tests, risks,
   and cross-track dependencies.

Do not claim Go production parity unless the required conformance evidence
actually passed.
