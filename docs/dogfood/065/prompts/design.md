# Design prompt - dogfood 065 Go daemon port

Produce `docs/dogfood/065/design/<lane>/DESIGN.md` as a handoff artifact.
Use a title block with `author: designer-<lane>-<model>-001`.

Read the dogfood README/operator report, RFC 0068-0071, SPEC, DECISION_LOG,
TODO, UBIQUITOUS_LANGUAGE, the authority matrix, and the daemon method
contract before proposing the plan.

Required sections:

1. Current direction and contradictions: state what RFC 0068 asks for, what
   D105 still says, and what Track D must reconcile without over-claiming Go
   parity.
2. Four-track split: Track A Go daemon core parity/schema, Track B SQLite
   eradication, Track C client/service/MCP boundary, Track D docs/decision
   consolidation.
3. Path ownership: list allowed and forbidden path groups for each track. Any
   parallel track overlap is a design bug.
4. Track A acceptance: Go supported schema version, migrations hash/source
   freshness, method contract freshness, audit-chain parity, capability
   denial parity, and a CORE=go conformance target that cannot silently skip.
5. Track B acceptance: production `connect_registry()` is impossible, daemon
   global surfaces are PostgreSQL-only, migration/test fixture exceptions are
   explicitly named, and no `.striatum/` path is live state.
6. Track C acceptance: repository resolution is daemon-owned, `/v1/invoke`
   production mutations route through daemon RPC, LocalRpcServer/API invoke are
   local-authoring or legacy/test surfaces, and dogfood composites are PG-native
   or hidden.
7. Track D acceptance: D107/D105 status is coherent, RFCs and docs line up with
   operator direction, and docs do not imply Go production parity has already
   landed.
8. Test gate: exact test or make target names for each track.
9. Risk cuts: list what is deliberately deferred.

Bouncing conditions:

- Any proposed write scope includes `docs/dogfood/065/README.md`,
  `docs/dogfood/065/OPERATOR_REPORT.md`, `docs/dogfood/065/workflow.json`,
  `docs/dogfood/065/prompts/`, `docs/dogfood/065/roles/`, or `.striatum/`.
- Any track depends on direct SQLite as production live state.
- Go is described as production-complete before conformance evidence exists.
- The output is a menu instead of a locked plan.
