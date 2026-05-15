---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
---
author: reviewer-unknown-model-001

# Build Review Codex Threat Model

## Verdict

needs_revision

## Trust Boundaries Reviewed

- Daemon RPC caller to PG handler: every handler must authorize the capability token before reading workflow state.
- Registered target repository to daemon-owned Postgres: every read must be scoped by `repository_id`, not only by caller-supplied `run_id`, `session_id`, or `job_id`.
- Handler SQL to Postgres: read handlers must use parameterized `SELECT` statements only.
- Cross-repository workflow state: union or aggregation queries must not combine rows across repositories unless the API is explicitly daemon-admin/global.
- Router fallback boundary: a PG-backed method must fail closed instead of falling through to historical SQLite-backed dispatch.

## Finding

HIGH: The referenced build evidence does not demonstrate the packet's read-handler threat-model requirements for the dogfood-060 implementation.

The reviewed documents establish the product rule that daemon-owned Postgres is authoritative and that RFC 0048 ports handlers away from SQLite-backed CLI dispatch. They also record prior write-handler and recovery/evidence work, but they do not provide handler-level evidence for the specific dogfood-060 read handlers under review. In particular, the packet asks for proof that every read handler:

- filters by `repository_id` before returning rows;
- short-circuits on capability denial;
- performs no writes;
- uses parameterized SQL for attacker-controlled identifiers such as `run_id`, `session_id`, and `job_id`;
- avoids cross-repository leakage in union or aggregation queries.

The referenced materials do not enumerate the dogfood-060 read handlers, their SQL, or their tests. Because this is a threat-model review, absence of that evidence is a blocking gap: a handler could satisfy the broad RFC 0048 direction while still leaking data across repositories through an unscoped join or a union branch missing `repository_id`.

## Required Remediation

Add or update the build handoff/test evidence so it names each read handler and maps it to tests that prove:

- repo A calls cannot return repo B rows, including for joins and unions;
- monkeypatched authorization denial prevents any SQL execution or result assembly;
- the handler executes `SELECT` only in read mode;
- SQL parameters are bound through the driver, with no f-string, `%`, or `.format()` interpolation for ids;
- multi-repo fixtures include colliding `run_id`, `session_id`, or `job_id` values where the schema allows them, or otherwise assert that globally unique ids are still paired with `repository_id` filters.

Until that evidence is present, acceptance would rely on intent rather than demonstrated boundary enforcement.
