---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/DECISION_LOG.md#D124", "docs/TODO.md#55", "docs/ROADMAP.md#4.11", "docs/rfcs/0064-review-diversity-enforcement.md", "docs/operator/BRIEF.md"]
---

# TODO 55 Accepted-Risk Implementation Plan
author: accepted-risk-planner-codex-001

## Decision Boundary

D124 settles the product question: accepted workflow-lint risk is live daemon
state, not workflow-file metadata. The implementation should move strict lint
acceptance from a CLI-local result shape into daemon-owned PostgreSQL, with
CLI, MCP, and UI acting only as clients. Each accepted-risk record must cite a
durable decision artifact and bind to either an immutable workflow snapshot or
a deterministic workflow fingerprint before it can suppress or annotate
daemon-core lint findings.

This plan does not change the lint rules themselves. It moves the authority
surface for accepting those rules into the daemon and preserves existing
authoring helpers as advisory projections until they are wired as clients.

## Data Model And Migration

Add migration `0013_workflow_accepted_risks.sql` in both migration trees:

- `go/pkg/db/sql/0013_workflow_accepted_risks.sql`
- `src/striatum/daemon_pg/sql/0013_workflow_accepted_risks.sql`
- migration registries in `go/pkg/db/migrations.go` and
  `src/striatum/daemon_pg/migrations.py`

Create `striatumd.workflow_accepted_risks`:

| Column | Purpose |
|---|---|
| `repository_id` | Repository scope. |
| `accepted_risk_id` | Opaque id, e.g. `war_<token>`. |
| `workflow_snapshot_id` | Nullable FK to `workflow_snapshots`; required for prepared/running workflows. |
| `workflow_fingerprint_sha256` | Deterministic hash for file-authoring previews before a snapshot exists. |
| `workflow_id` / `workflow_version` | Query/display convenience copied from the snapshot or submitted workflow body. |
| `source_path` | Repo-relative workflow path when the acceptance came from authoring. |
| `lint_rule` | Stable lint rule id, e.g. `same_model_review_pair`. |
| `finding_fingerprint_sha256` | Hash over rule id plus normalized lint finding payload. |
| `finding_json` | The normalized lint finding that was accepted. |
| `decision_artifact_ref` | Required repo-relative artifact path or decision id reference. |
| `accepted_by` | Operator/session/client identity from the daemon request context where available. |
| `accepted_at` | Insert timestamp. |
| `expires_at` | Nullable future expiry for short-lived exceptions. |
| `revoked_at`, `revoked_by`, `revocation_reason` | Nullable revocation metadata. |
| `rationale` | Required short operator rationale. |

Constraints:

- exactly one binding mode must be present: `workflow_snapshot_id` or
  `workflow_fingerprint_sha256`;
- `decision_artifact_ref`, `lint_rule`, `finding_fingerprint_sha256`, and
  `rationale` must be non-empty;
- `(repository_id, workflow_snapshot_id, lint_rule, finding_fingerprint_sha256)`
  is unique when `workflow_snapshot_id` is not null and `revoked_at is null`;
- `(repository_id, workflow_fingerprint_sha256, lint_rule,
  finding_fingerprint_sha256)` is unique when no snapshot exists and
  `revoked_at is null`.

Append-only posture should match existing event/artifact discipline. Do not
update or delete accepted rows to revise history; revocation is a metadata
state transition. If the existing append-only grant model cannot allow
`revoked_at` updates cleanly, split revocations into
`workflow_accepted_risk_revocations` and keep the base table insert-only.

## Daemon Methods And Audit

Register these production daemon methods:

| Method | Capability | Scope | Behavior |
|---|---|---|---|
| `workflow.lint` | `read` | `single_repo` | Evaluate daemon-core lint for a workflow path, workflow JSON body, run id, or snapshot id. Include matching active accepted-risk records in the response. |
| `workflow.accept_risk` | `admin` | `single_repo` | Re-evaluate lint against the bound workflow identity, validate the requested finding fingerprints are present, require `decision_artifact_ref` and `rationale`, insert accepted-risk rows, and emit events. |
| `workflow.accepted_risks.list` | `read` | `single_repo` | List active and optionally revoked accepted risks by workflow snapshot, fingerprint, rule, run, or source path. |
| `workflow.accepted_risk.revoke` | `admin` | `single_repo` | Revoke one accepted-risk id with a required reason and emit a revocation event. |

Audit behavior:

- all methods use metadata audit class;
- audit params/response hashes must not include workflow file contents,
  transcript text, or artifact bodies;
- successful acceptance emits `workflow.accepted_risk_recorded`;
- revocation emits `workflow.accepted_risk_revoked`;
- `workflow.lint` remains read-shaped and emits only normal daemon request
  audit rows, not workflow events.

Contract updates are required in `contracts/daemon_methods.json`, generated Go
registry/table outputs, MCP tool discovery, and
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`. New methods must also be
covered by the authority guardrail tests so the matrix cannot drift from the
contract.

## Snapshot And Fingerprint Binding

Use snapshot binding whenever the workflow already has daemon identity:

- `run.prepare` should evaluate daemon-core lint after creating or loading the
  workflow snapshot and before allowing strict-risk findings to proceed.
- Any accepted-risk lookup for a run uses `runs.workflow_snapshot_id`, never
  the mutable workflow file path.
- Snapshot-bound acceptance records remain valid even if the workflow file is
  edited later.

Use fingerprint binding only for authoring-time workflows that have not yet
been prepared:

- compute the fingerprint from the canonical normalized workflow JSON, using
  the same content shape as `run.prepare` uses for `workflow_snapshots.content_sha256`;
- return the fingerprint from `workflow.lint`;
- require `workflow.accept_risk` to submit the fingerprint and the normalized
  finding fingerprints it is accepting;
- when a later `run.prepare` creates a workflow snapshot with the same content
  hash, it may match active fingerprint-bound acceptances and report them as
  applicable to the new snapshot. A later migration can materialize that link
  if needed, but matching by immutable hash is enough for the first slice.

Accepted-risk records should cite decision artifacts by stable path when
available. Decision ids like `D124` may appear as supporting references, but a
record that accepts a concrete workflow risk should point to the specific
decision artifact or operator decision that explains why that workflow shape is
acceptable.

## Client Surfaces

CLI:

- keep `workflow lint` as the compatibility entry point, but route strict
  acceptance through daemon `workflow.accept_risk` when the daemon is
  reachable;
- retain non-strict local lint as an advisory authoring helper during the
  transition;
- deprecate the current behavior where `--accepted-risk-decision-id` only
  appears in the local JSON result and does not persist state.

MCP:

- expose `workflow.lint`, `workflow.accept_risk`,
  `workflow.accepted_risks.list`, and `workflow.accepted_risk.revoke` through
  `tools/list` according to token capability;
- keep mutation methods hidden from read-only tokens and re-authorized on
  `tools/call`;
- prefer MCP for new agent/operator flows per the RFC 0050 cutover direction.

UI:

- workflow detail/generator preview pages should show lint findings with
  "accepted by daemon record" state when a matching record exists;
- strict acceptance requires an operator confirmation gesture, decision
  artifact reference, and rationale;
- revoked or expired accepted risks should be visible but not counted as
  suppressing current findings.

No client may write accepted-risk metadata into `workflow.json` and treat it as
live authority. Workflow-file annotations may be displayed later as comments or
authoring hints only.

## Tests And Guardrails

Migration and model:

- migration creates the table, constraints, indexes, and append/revocation
  posture in both Go and Python migration paths;
- fixture tests prove duplicate active acceptances are refused and revoked
  acceptances no longer suppress findings;
- grant tests preserve append-only expectations or explicitly cover the
  revocation table alternative.

Daemon contract:

- contract, generated method tables, MCP visibility, and command authority
  matrix tests cover all new methods;
- capability-denial tests cover read token vs admin token behavior, wrong
  repository scope, and missing token;
- stale or unknown method calls still audit as `method_unknown`.

Lint behavior:

- daemon `workflow.lint` returns the same warning rule ids and coverage summary
  as the existing authoring lint for representative workflows;
- `workflow.accept_risk` refuses a finding fingerprint that is not present in
  the current lint payload;
- snapshot-bound records continue to apply after the source workflow file
  changes;
- fingerprint-bound records apply only to identical normalized workflow
  content;
- `run.prepare` refuses strict-risk workflows unless matching active
  accepted-risk records exist or the existing explicit CLI override path is
  intentionally still supported for compatibility.

Client/UI:

- CLI strict mode persists accepted-risk records through the daemon and reports
  accepted-risk ids;
- MCP fake-client tests prove read/mutation visibility and acceptance flow;
- UI tests verify confirmation/rationale/decision-reference requirements
  without making workflow metadata authoritative.

## First Implementation Slice

The smallest useful slice should avoid UI churn and touch a coherent daemon
surface:

1. Add the migration/table, migration registration, and migration tests.
2. Add Go daemon handlers for `workflow.lint`,
   `workflow.accepted_risks.list`, and `workflow.accept_risk` for
   fingerprint-bound and snapshot-bound records. Defer revocation to slice 2
   if it would expand scope.
3. Add contract/registry/MCP visibility entries and update the generated
   daemon method tables plus `COMMAND_AUTHORITY_MATRIX.md`.
4. Add daemon contract, capability, duplicate-record, and fingerprint/snapshot
   binding tests.
5. Add a thin CLI route for `workflow lint --strict --override-rationale
   --accepted-risk-decision-id` that calls `workflow.accept_risk` and returns
   the inserted accepted-risk ids when warnings are accepted.

Suggested disjoint write scope for that implementation packet:

- `contracts/daemon_methods.json`
- `go/pkg/db/**`
- `go/pkg/mutations/workflow_accepted_risk.go`
- `go/pkg/reads/workflow_authoring.go`
- `go/pkg/mcp/**`
- `src/striatum/daemon_pg/sql/**`
- `src/striatum/daemon_pg/migrations.py`
- `src/striatum/cli/{parser.py,dispatch.py,daemon_rpc_route.py}`
- `docs/architecture/{COMMAND_AUTHORITY_MATRIX.md,DAEMON_METHOD_TABLES.md}`
- `tests/architecture/**`
- `tests/daemon_pg/**`
- focused Go tests under `go/pkg/**`

Leave UI presentation, revocation UX, and run-prepare strict gating to follow
up slices unless the first implementer finds they are trivial once the daemon
record exists. The key first deliverable is durable, decision-linked,
daemon-owned accepted-risk state that can be observed through daemon lint.
