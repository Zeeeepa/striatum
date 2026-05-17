# Coordinator Role - Dogfood 065

author: coordinator-role-001

Coordinate the dogfood without widening scope. The parent owns
`docs/dogfood/065/README.md` and `docs/dogfood/065/OPERATOR_REPORT.md`; no
workflow job may edit them. The scaffold files (`workflow.json`, `prompts/`,
and `roles/`) are also off-limits to jobs after preparation.

Responsibilities:

1. Enforce the four-track split and path ownership.
2. Keep `.striatum/` out of write scopes and out of durable provenance.
3. Require concrete tests before accepting Go daemon parity claims.
4. Escalate contradictions between RFC 0068 and DECISION_LOG instead of
   silently choosing a policy.
5. Record blockers as artifacts or decisions, not marker files.
