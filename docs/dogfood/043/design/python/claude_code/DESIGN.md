author: designer-unknown-model-002

# RFC 0045 Python-Core Design (Track A)

Scope: V1 acceptance criteria for `striatum.workflow.v1.1`, implemented inside
the Python core (`src/striatum/workflow.py`, `src/striatum/dashboard.py`,
`src/striatum/service.py`, `src/striatum/workflow_generator/`,
`src/striatum/cli/workflow.py`). Frontend (RFC 0045 §5) is Track B and not
covered here. Future-work items called out as out-of-scope in RFC 0045 §3
(conditional phase routing, phase-level retry, phase parallelism, multi-repo
phase fan-out) stay deferred.

The validator must accept every existing `striatum.workflow.v1` workflow
unchanged. Phases are an additive, opt-in layer that materialises into the
same job/edge graph the runner already executes.

## 1. Schema bump (`striatum.workflow.v1.1`)

Today the schema-version gate is the single-line check at
`src/striatum/workflow.py:495`:

```python
if workflow.get("schema_version") != "striatum.workflow.v1":
    raise WorkflowError(...)
```

Replace with a small allow-set so both versions parse:

```python
ACCEPTED_SCHEMA_VERSIONS = frozenset({
    "striatum.workflow.v1",
    "striatum.workflow.v1.1",
})

schema_version = workflow.get("schema_version")
if schema_version not in ACCEPTED_SCHEMA_VERSIONS:
    raise WorkflowError(
        f"workflow schema_version must be one of "
        f"{sorted(ACCEPTED_SCHEMA_VERSIONS)!r}",
        field_path="schema_version",
    )
```

`REQUIRED_TOP_LEVEL` (`workflow.py:29`) stays unchanged — `phases` is optional.
Add `phases` and `phase_id` to a new `OPTIONAL_TOP_LEVEL` /
`OPTIONAL_JOB_FIELDS` lint allowlist if/when we tighten unknown-key checks, but
that is not required for V1; the current validator silently ignores unknown
top-level keys.

### Schema shape (V1.1 deltas only)

Top level:

```jsonc
{
  "schema_version": "striatum.workflow.v1.1",
  "phases": [
    {
      "id": "phase_1_design",        // required, unique, [a-z][a-z0-9_]*
      "name": "Design",              // required, human label
      "color": "#abcdef",            // optional, frontend rendering hint
      "description": "...",          // optional
      "synthesis_job_id": "synth_1"  // required; must resolve to a
                                     // type=phase_synthesis job with
                                     // phase_id == this phase id
    }
  ]
}
```

> Naming note: RFC 0045 §3 uses `phase` as the per-job field name and the
> Track A prompt uses `phase_id`. We adopt `phase_id` on jobs and
> `phases[].id` at the top level — it is unambiguous in logs/diagnostics
> and matches the existing `role_id` / `lane_id` / `workflow_job_id`
> naming. The validator still accepts the bare key `phase` as a v1.1
> alias and rewrites it to `phase_id` in `plan_workflow` output so RFC
> 0045 example JSON does not need to be retyped; the alias emits a soft
> warning via the `warnings` channel that `validate_workflow` already
> threads (see the `warnings: list[str] | None` param at
> `workflow.py:478`).

Per job:

```jsonc
{
  "id": "design_python_claude",
  "type": "generic",
  "phase_id": "phase_1_design",   // optional in v1.1; forbidden in v1
  ...
}
```

`phase_synthesis` job type:

```jsonc
{
  "id": "synth_1",
  "type": "phase_synthesis",
  "phase_id": "phase_1_design",
  "role_id": "reviewer",
  // inherits review-job semantics: verdict-bearing
  "on_verdict": { ... }
}
```

## 2. New job type: `phase_synthesis`

Contract:

- **Fan-in**: every job in the same phase whose `phase_id == this.phase_id`
  AND whose id is not the synthesis job is an implicit upstream. The validator
  injects these edges into the in-memory `edges` list before downstream
  validation runs (see §3); authors do not write the dependencies by hand.
- **Fan-out**: every job in phase N+1 that has no explicit
  `depends_on`/`needs` entry pointing back at a phase-N job receives an
  injected edge from `phases[N].synthesis_job_id`. Phase N+1 entry jobs are
  defined as jobs whose `phase_id == phases[N+1].id` and whose only cross-
  phase dependency targets are `phases[N].synthesis_job_id`.
- **Verdict lifecycle**: identical to a `review` job. The publisher and the
  `submit-review` CLI verb already accept review jobs (the work-packet
  `verdict` command shape we see in this packet's `commands.verdict`). No new
  state machine. `requires_verdict: ["accept", "accept_with_findings"]` on the
  outbound edges is added the same way `create_run` already adds it for
  `type == "review"` jobs at `workflow.py:696-697`:

  ```python
  if upstream_job.get("type") == "review":
      gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
  ```

  Extend that conditional to:

  ```python
  if upstream_job.get("type") in {"review", "phase_synthesis"}:
      gate_json["requires_verdict"] = ["accept", "accept_with_findings"]
  ```

- **Exactly one per phase**: the validator enforces a 1:1 mapping between
  `phases[i]` and `phase_synthesis` jobs with `phase_id == phases[i].id`. Two
  synthesis jobs in the same phase is a validation error.
- **Empty phase guard** (RFC 0045 open question, resolved "refuse"): a phase
  whose synthesis job has zero implicit upstreams is a validation error. The
  synthesis is the gate; gating on nothing is a bug.

## 3. Validator rules — `src/striatum/workflow.py`

The current `validate_workflow` body runs in this order (annotated for the
extensions below):

1. `workflow.py:492` `missing = sorted(REQUIRED_TOP_LEVEL.difference(workflow))`
2. `workflow.py:495` schema-version gate (extended in §1).
3. `workflow.py:500-512` cross-repo, provenance, branch, recovery-policy blocks.
4. `workflow.py:521-573` per-job loop populating `job_map` and checking
   `role_id`, `lane_id`, `write_scope`, `expected_artifacts`, postures, apply
   gates.
5. `workflow.py:574-607` artifact-path uniqueness, posture reachability, edges
   (`edge_dependency_pairs` and `validate_needs_match_edges`), cycles, and
   `_validate_parallelism` / `_validate_parallelism_config` /
   `_validate_revision_policy` / `_warn_same_lane_review_implement_cycles`.

Insert a new helper `validate_phases(workflow, *, job_map, edges, warnings)`
between step 4 and step 5 — after every job has been validated standalone but
before edge-level invariants run, because the helper needs `job_map` and must
also synthesise extra edges that the cycle/parallelism checks then see.

```python
def validate_phases(
    workflow: JsonObject,
    *,
    job_map: dict[str, JsonValue],
    edges: list[tuple[str, str, JsonObject]],
    warnings: list[str] | None,
) -> list[tuple[str, str, JsonObject]]:
    """Validate v1.1 phases and return the edge list with implicit
    fan-in / fan-out edges materialised. Returns the original list
    unchanged for v1 workflows."""
```

Rules enforced (each maps to a stable error string for tests):

1. **`phases` absent** (or empty): every job MUST NOT carry `phase_id`. If any
   job has `phase_id` set, raise `WorkflowError("job <id> declares phase_id "
   "but workflow has no phases")` with `field_path="jobs[<i>].phase_id"`.
   `phase_synthesis` job type is likewise rejected.
2. **`phases` non-empty**:
   - Every `phases[i].id` is a unique non-empty string. Duplicate ids → error.
   - Every `phases[i].synthesis_job_id` resolves to a job in `job_map` whose
     `type == "phase_synthesis"` and whose `phase_id == phases[i].id`.
     Mismatch → error.
   - Exactly one `phase_synthesis` job per phase (count by `phase_id`).
     Reference test: a v1.1 fixture with two synthesis jobs in one phase must
     raise `phase has multiple synthesis jobs`.
   - Every job's `phase_id` (when set) resolves to a known phase id.
   - Every `phase_synthesis` job has `phase_id` set and it matches the phase
     that names it as `synthesis_job_id`.
   - **Cross-phase edges**: walk every (from, to) in `edges` plus every
     declared `needs`/`depends_on` upstream. If both endpoints carry
     `phase_id` and `from.phase_id != to.phase_id`:
       - The from-job MUST be `phases[from.phase_id].synthesis_job_id`. If
         the from-job's `phase_id` index is `>=` the to-job's `phase_id`
         index, raise `cross-phase edge runs backwards`.
       - Otherwise raise `cross-phase edge <from> -> <to> must originate at
         the source phase's synthesis_job_id`. This is the dogfood-042 quirk
         RFC 0045 §3 calls out — `cancel-job --cascade` followed
         `blocked_by` chains across the bespoke consolidate; refusing
         non-synthesis cross-phase edges makes the cascade boundaries
         explicit.
   - **Intra-phase edges**: no restriction beyond what
     `validate_needs_match_edges` (`workflow.py:749`) and
     `_validate_cycle_targets_feed_sources` (`workflow.py:1811`) already
     enforce.
   - **Implicit synthesis fan-in**: synthesise edges
     `(intra_phase_job, synthesis_job, {"on": "completed"})` for every job in
     the phase whose id is not the synthesis job. These edges go into the
     same in-memory `edges` list the rest of the validator already iterates,
     so cycle detection, parallelism overlap, and gate insertion all keep
     working without further changes.
   - **Empty phase**: after fan-in injection, a synthesis job with zero
     declared upstreams from authors AND zero injected upstreams (i.e. the
     phase contains only the synthesis job) raises `phase <id> is empty`.

3. **v1 schema_version with `phases`**: raise `workflow uses 'phases' but
   declares schema_version striatum.workflow.v1; bump to
   striatum.workflow.v1.1`. This is a soft fence — once we surface phases at
   all, the schema version must be honest.

`validate_phases` is called from `validate_workflow` at the position
indicated above. Its return value (the materialised edge list) is passed
forward; the existing `edge_dependency_pairs` call at `workflow.py:576` is
adapted to either accept the materialised list as a parameter or be invoked
indirectly through a `_materialise_edges` helper that knows whether phases
applied. The cleanest refactor is:

- Hoist the body of `edge_dependency_pairs` into a private
  `_explicit_edge_dependency_pairs` that reads only `workflow["edges"]`.
- `edge_dependency_pairs(workflow)` becomes:
  `explicit = _explicit_edge_dependency_pairs(workflow); return _materialise_phase_edges(workflow, explicit)`
  where `_materialise_phase_edges` is the read-only twin of
  `validate_phases` that re-injects fan-in/fan-out without raising; the
  validating pass remains the only mutating consumer.
- `create_run` at `workflow.py:693` already iterates
  `edge_dependency_pairs(workflow)` to insert `job_dependencies` rows. Once
  the function returns the phase-materialised edges, runtime materialisation
  comes for free with no changes to the SQL writer.

## 4. Runtime materialisation

The job graph is built by `create_run` (`workflow.py:610-718`). Two relevant
loops:

- `workflow.py:654-692` iterates `_list(workflow, "jobs")` and INSERTs each
  job into the `jobs` table, threading `job_type`, `role_id`, `write_scope`,
  `expected_artifacts`, etc. Phase metadata is materialised here:

  ```python
  conn.execute(
      "INSERT INTO jobs (...) VALUES (?, ...)",
      (..., job.get("phase_id"), ...),
  )
  ```

  We add a new nullable column `phase_id TEXT` to the `jobs` table via a
  schema migration (next migration ordinal in
  `src/striatum/db/migrations/`). For v1 workflows the column is `NULL`; no
  downstream code paths read it unconditionally.

- `workflow.py:693-704` iterates `edge_dependency_pairs(workflow)` and
  INSERTs `job_dependencies` rows. Because step 3 makes
  `edge_dependency_pairs` return phase-materialised edges, the implicit
  fan-in/fan-out edges become real `job_dependencies` rows, which is what
  `claim-next` already uses to decide readiness. `phase_synthesis` jobs
  block on every intra-phase upstream and gate every next-phase entry job
  the same way a `review` job already gates its downstreams.

- Add a single new event type, `phase.entered` / `phase.exited`, written via
  the existing `insert_event` call. The trigger lives in the verdict path
  (whichever module owns `submit-review` / `verdict`; the work-packet
  shape shows `verdict` as a CLI verb): when a `phase_synthesis` job
  records an accepting verdict, fire `phase.entered` for phase N+1. When
  every job in a phase reaches `state == "completed"`, fire
  `phase.exited`. These events are advisory — the runner does not depend
  on them — but they let the dashboard render phase transitions and let
  the audit chain explain "why did phase 2 start when it did".

- `plan_workflow` (`workflow.py:190-247`) needs no functional change: the
  Kahn-style readiness walk operates on the materialised edge list and will
  naturally place phase_synthesis jobs at the right depth.

- `workflow_graph_data` (`workflow.py:250`) and `workflow_graph_mermaid`
  (`workflow.py:266`) gain optional `phase_id` per node and surface phase
  bands in the rendered output. This is purely additive — existing v1 graph
  outputs are byte-identical for v1 workflows.

`compute_node_states(conn, run_id=...)` (`workflow.py:445`) is the read path
used by the dashboard graph panel; it reads from `jobs` and per-job
provisional state, and gains no new logic. Once `phase_id` is on the job
row, it flows through this function untouched.

The executor (`claim-next`, `submit-review`, lease/heartbeat) needs zero
changes. Phase synthesis is a verdict-bearing job; the runner already knows
how to materialise, claim, and gate review jobs.

## 5. Status reporting

Two surfaces extend, both reading from the same SQLite source of truth.

### 5.1 `striatum status --json` (`src/striatum/cli/introspect.py:161`)

The `status` function at `cli/introspect.py:161-209` builds the status
envelope. The relevant aggregation is:

```python
jobs = conn.execute(
    "SELECT state, COUNT(*) AS count FROM jobs "
    "WHERE (? IS NULL OR run_id = ?) GROUP BY state ORDER BY state",
    (run_id, run_id),
).fetchall()
...
"jobs": {str(row["state"]): int(row["count"]) for row in jobs},
```

Add a second aggregation:

```python
phases = conn.execute(
    """
    SELECT j.phase_id AS phase_id, j.state AS state, COUNT(*) AS count
    FROM jobs j
    WHERE (? IS NULL OR j.run_id = ?)
      AND j.phase_id IS NOT NULL
    GROUP BY j.phase_id, j.state
    ORDER BY j.phase_id, j.state
    """,
    (run_id, run_id),
).fetchall()
```

Fold into a per-phase block keyed by phase id, joined against the workflow
snapshot's `phases` array to attach `name`, `synthesis_job_id`, current
phase index, and the gate's verdict state. When no run in the slice carries
`phase_id` on any job, the field is omitted to preserve the v1 status
envelope byte-for-byte. Sketch of the appended field:

```jsonc
"phases": {
  "<run_id>": [
    {
      "phase_id": "phase_1_design",
      "name": "Design",
      "index": 0,
      "synthesis_job_id": "synth_1",
      "synthesis_state": "ready",
      "jobs": { "blocked": 0, "ready": 1, "running": 2, "completed": 3 },
      "totals": { "completed": 3, "total": 6 },
      "gate": "open"  // open | accepting | accepted | needs_revision | rejected
    }
  ]
}
```

`status` already projects per-run snapshots in `_provenance_mode_for_status`
(`cli/introspect.py:212-239`); the same `workflow_snapshots.workflow_json`
load lets the helper join phase metadata in.

### 5.2 Dashboard

`gather_payload` (`dashboard.py:57-125`) already reads the workflow snapshot
into `workflow_payload`, computes `node_states`, and returns a `status_payload`
that flows into `render_frame`. The dashboard panel additions:

- Add `_render_phases_panel` (sibling to `_render_left_column` at
  `dashboard.py:324` and `_render_right_column` at `dashboard.py:339`) that
  reads `payload["status"]["phases"][run_id]` and emits one line per phase:

  ```
  Phase 1 Design        ████████░░  3/6   gate ready
  Phase 2 Build         ░░░░░░░░░░  0/4   gate blocked
  ```

- `_graph_topology` (`dashboard.py:750-792`) gains a `phase_id` field on each
  node payload (already builds a dict per job). The graph-panel renderers
  (`_render_graph_layered`, `_render_graph_fancy`, `_render_graph_lr`) get a
  small extension: when every node has `phase_id`, group rows by phase id
  and emit a phase header line between groups. Outside that, the graph
  layout is unchanged.

- `_render_phases_panel` is rendered after the verdicts panel and before the
  events panel when at least one phase exists; otherwise the dashboard
  output is byte-identical to today's.

### 5.3 Service surface (`src/striatum/service.py`)

`service.py` exposes the HTTP / Unix-socket adapter; the request handlers
return JSON envelopes that wrap the same `status` function above (or read
the SQL directly). The relevant entry points are `StriatumServiceHandler`
(`service.py:497`) and the wider request-routing surface. The change is
mechanical: the handler that backs `GET /runs/<id>/status` already returns
whatever `status(conn, run_id=run_id)` produces, so adding phases to the
status payload automatically surfaces them over the service. Any
service-local templated views (the chat-briefing builder at
`_build_chat_briefing` at `service.py:229`) gain a phase summary block when
the active run has phases — a 2–3 line addition that consumes the same
status envelope.

## 6. Generator catalog — `multi_phase` shape

`workflow_generator/core.py:265` (`default_spec`) and the
`_compile_shape` dispatcher at `workflow_generator/core.py:352` define
shape-specific compilation. Today the catalog ships
`design_implement_review`, `build_review`, etc. through
`_compile_shape`; we add `multi_phase`:

```python
def _compile_multi_phase(spec: WorkflowGenerationSpec) -> tuple[
    list[JsonObject],  # jobs
    list[JsonObject],  # edges
    list[JsonObject],  # cycles
    list[JsonObject],  # phases  (new return slot)
]:
    ...
```

The shape parameters mirror RFC 0045 §4:

```jsonc
{
  "shape": "multi_phase",
  "phases": [
    {
      "id": "phase_1_design",
      "name": "Design",
      "tracks": [
        {"id": "track_a", "shape": "design_implement_review",
         "lanes": {...}, "review_postures": ["devils_advocate"]}
      ],
      "synthesis_lane": "claude_code",
      "synthesis_postures": ["neutral"]
    },
    {"id": "phase_2_build", ...}
  ]
}
```

Per-phase the generator:

1. For each track, dispatches to the existing single-phase compiler
   (`_compile_shape` with the track's shape), then renames the generated job
   ids with a phase-scoped prefix (`{phase_id}__{track_id}__...`) to avoid
   collisions across phases.
2. Emits exactly one `phase_synthesis` job per phase
   (`{phase_id}__synthesis`), with `phase_id` set and `lane_id` /
   `role_id` derived from `synthesis_lane` / a reviewer role.
3. Wires no per-phase explicit edges for synthesis fan-in — the validator
   injects those (§3). Cross-phase wiring: emits an explicit edge from
   `phases[N].synthesis_job_id` to each phase-N+1 track's first job.
4. Appends a `phases` array at the workflow root and bumps
   `schema_version` to `striatum.workflow.v1.1`.

`generate_workflow` (`core.py:211`) calls the new compiler when
`spec.shape == "multi_phase"` and stitches the phases array into the
output workflow alongside the existing `jobs` / `edges` / `cycles`.

`catalog.py:18` (`load_catalog`) registers the catalog entry; the bundled
JSON catalog file gains a `multi_phase` template documenting the shape,
its parameters, and a worked example with two phases × two tracks.

The CLI entry `striatum workflow generate --shape multi_phase --params
<json>` already routes through `generate_workflow`; the new shape lights
up automatically.

## 7. New CLI verb: `striatum workflow upgrade --add-phases <path>`

`src/striatum/cli/workflow.py:35` already defines `workflow_upgrade(path,
*, repo, force, dry_run)` — the existing verb that backports RFC 0040
harness-profile fragments. The `--add-phases` flag is a new mode of the
same verb (or a peer function `workflow_upgrade_add_phases`; layout is a
small naming question, behaviour identical).

Inputs:

- `path` — target `workflow.json`.
- `--phase-map <file>` (optional) — explicit JSON mapping of parallel_group
  → `{phase_id, phase_name}`. When omitted, the verb infers the mapping
  heuristically.
- `--apply` (required to write) — without it, the verb prints the proposed
  upgrade diff and exits with status `would_update`; with it, the verb
  rewrites the file (and refuses on non-terminal runs the same way
  `_running_runs_for_workflow` at `cli/workflow.py:80` already does).

Inference algorithm when `--phase-map` is absent:

1. Load the workflow (`workflow.py:172`) and reject if it already declares
   `phases` (`refused_already_v1.1`).
2. Build `groups: dict[str, list[job]]` from `parallel_group` (mirroring
   `_validate_parallelism` at `workflow.py:893`).
3. Cluster groups by lexical prefix up to the first underscore (`design_a` →
   `design`, `synth_a` → `synth`, `build_review_a` → `build`). Each cluster
   becomes a candidate phase, ordered by the lowest job ordinal in the
   cluster (using job order in the source `jobs` array as the tiebreaker).
4. For each phase candidate, synthesise a `phase_synthesis` job named
   `{phase_id}_synthesis` placed at the end of the phase, with role
   `reviewer` and lane chosen from the phase's most common reviewer lane
   (fallback: the workflow's `coordinator.lane_id`).
5. Rewrite per-job `phase_id` from the inferred cluster membership.
6. Rewrite cross-cluster `edges` to terminate at the upstream phase's
   synthesis job. Edges already terminating at a consolidate-style job are
   left intact (the verb prefers the existing gate over inventing a new
   one); when the existing gate is reused, the synthesise step is skipped
   for that phase and the existing job is annotated with
   `type: "phase_synthesis"` instead.
7. Bump `schema_version` to `striatum.workflow.v1.1`.
8. Re-validate the rewritten workflow with `validate_workflow`. Any error
   aborts the upgrade and reports the validation message; the input file
   is left untouched.

Return envelope reuses the `workflow_upgrade` shape:

```jsonc
{
  "workflow_path": "...",
  "status": "would_update" | "updated" | "refused_already_v1.1"
          | "refused_running" | "refused_conflict",
  "phases_added": [{"id": "phase_1_design", "synthesis_job_id": "..."}],
  "jobs_relabelled": [{"job_id": "design_a_python", "phase_id": "phase_1_design"}],
  "edges_rewritten": [{"from": "design_a_python", "to": "build_a_python",
                       "rewritten_via": "phase_1_design_synthesis"}],
  "warnings": ["..."]
}
```

The verb writes the rewrite via the same path-atomic rename
`tempfile`/`os.replace` pattern `workflow_upgrade` already uses
(`cli/workflow.py` body, not re-quoted here) so a write failure leaves the
input untouched. Front matter / schemas are not touched; `--add-phases`
operates purely on the JSON workflow document.

## 8. Backwards compatibility & test matrix

Every existing `striatum.workflow.v1` workflow MUST continue to validate,
plan, run, status, and dashboard-render byte-identically. Concretely:

| Scenario | Expected behaviour | Where verified |
| --- | --- | --- |
| v1 fixture, no phases | `validate_workflow` raises nothing; `plan_workflow` output identical to pre-change snapshot. | `tests/test_workflow_validate.py` and the existing plan-snapshot tests. Add a `striatum.workflow.v1` regression fixture iff one is not already present. |
| v1 fixture + `phase_id` on a job | `validate_phases` rejects with `job <id> declares phase_id but workflow has no phases`. | New `tests/test_workflow_phases.py::test_v1_job_phase_id_rejected`. |
| v1.1 valid, single phase | accepts; status payload exposes one-phase progress; dashboard renders one band. | `tests/test_workflow_phases.py::test_v1_1_single_phase_accepts`. |
| v1.1 valid, two phases × two tracks | accepts; implicit synthesis fan-in materialises; cross-phase edges through synthesis; status shows two phases. | `tests/test_workflow_phases.py::test_v1_1_multi_phase_accepts` plus an integration test that drives a run end-to-end (matches RFC 0045 acceptance criterion 8). |
| v1.1 cross-phase edge bypassing synthesis | rejected with named error. | `tests/test_workflow_phases.py::test_cross_phase_edge_bypassing_synthesis_rejected`. |
| v1.1 cross-phase edge running backwards | rejected. | `...::test_cross_phase_edge_runs_backwards_rejected`. |
| v1.1 with two `phase_synthesis` jobs in one phase | rejected. | `...::test_phase_with_two_synthesis_jobs_rejected`. |
| v1.1 with empty phase (synthesis only) | rejected. | `...::test_empty_phase_rejected`. |
| v1.1 with `phase_synthesis` job whose `phase_id` does not match its parent phase | rejected. | `...::test_synthesis_phase_mismatch_rejected`. |
| `generate_workflow` shape `multi_phase` | emits a valid v1.1 workflow that passes `validate_workflow`. | `tests/test_workflow_generator.py::test_multi_phase_shape`. |
| `workflow upgrade --add-phases` on a v1 fixture with `parallel_group` clusters | dry-run prints proposed change set; `--apply` rewrites and re-validates. | `tests/test_workflow_upgrade.py::test_add_phases_infers_from_groups` plus a `--phase-map` variant. |
| `striatum status --json` on a v1 run | envelope does not contain a `phases` field. | `tests/test_status_json.py::test_v1_status_omits_phases`. |
| `striatum status --json` on a v1.1 run | envelope contains the per-phase block defined in §5.1. | `tests/test_status_json.py::test_v1_1_status_includes_phases`. |
| `dashboard --once --run-id <v1>` | byte-identical to pre-change snapshot. | Snapshot test. |
| `dashboard --once --run-id <v1.1>` | adds phases panel; graph panel groups by phase header. | New snapshot test in `tests/test_dashboard.py`. |

The runtime executor (`claim-next`, `submit-review`, lease/heartbeat) does
not need new tests beyond what already exists for review jobs — the
phase_synthesis job type reuses the review lifecycle wholesale. The CI
fixture called out in RFC 0045 acceptance criterion 8
(`tests/fixtures/multi_phase_workflow.json`) is the integration anchor; it
drives a 2-phase × 2-track-per-phase run from `run prepare` through to
phase-2 completion using the existing harness.

## 9. Out of scope (V1)

- Phase parallelism (two phases running concurrently). V1 phases run
  sequentially; the executor's existing readiness walk already enforces
  this once synthesis-gated edges are in the dependency table.
- Conditional phase routing (`if verdict X then phase Y else phase Z`).
- Phase-level retry budgets. V1 reuses per-job `max_attempts`.
- Multi-machine / cross-repo phase fan-out (RFC 0032 territory).
- Frontend rendering of phase bands and cross-phase edge styling (RFC 0045
  §5, Track B).
- Sealed-patch / provenance interactions with phase_synthesis jobs beyond
  inheriting review-job behaviour. `_validate_provenance_mode`
  (`workflow.py:1001`) is untouched; review-policy validators
  (`_validate_reviewer_policy` at `workflow.py:1451`,
  `_validate_review_posture` at `workflow.py:1494`) apply to
  `phase_synthesis` jobs without modification.

## 10. Risk register

- **Edge materialisation order**: `validate_phases` must run before the
  edge invariants in step 5 of `validate_workflow`. Misordering produces
  spurious "needs disagree with workflow edges" errors from
  `validate_needs_match_edges` (`workflow.py:749`). Mitigation: the
  injected edges are not author-declared, so `validate_needs_match_edges`
  must skip injected edges; the cleanest approach is to keep the
  `needs`-vs-edges check on the *explicit* edge list and feed the
  materialised list only to downstream consumers
  (`_validate_cycle_targets_feed_sources`, `create_run`,
  `compute_node_states`).
- **Migration ordinal**: adding the `phase_id` column to `jobs` requires a
  new migration. The migration is forward-only and additive (nullable
  column), so existing DBs stay valid; rollback is a no-op because old
  code ignores the column.
- **Generator id collisions**: phase-scoped prefixes for track-generated
  job ids must be deterministic to keep `idempotency_key` stable
  (`workflow.py:689` builds `f"{run_id}:{workflow_job_id}:1"` from the
  generated id). The `{phase_id}__{track_id}__{base_id}` convention is
  stable across runs.
- **Upgrade verb misinference**: heuristic clustering may misread a
  workflow's `parallel_group` taxonomy. Mitigation: refuse to write
  without `--apply`, always print the diff, and document `--phase-map`
  as the authoritative escape hatch. The verb also refuses on
  non-terminal runs (already enforced by
  `_running_runs_for_workflow`).

## 11. Implementation order

Suggested commit sequence for the Python core (Tracks B/C land
afterwards):

1. Migration adding nullable `jobs.phase_id`. Tests: schema sanity.
2. Schema-version allow-set in `validate_workflow` + a soft warning when a
   v1 workflow declares `phase_id`. Tests: v1 rejection and v1.1 trivial
   acceptance (no phases yet).
3. `validate_phases` helper + per-job `phase_id` materialisation into
   `create_run`. Tests: the table in §8.
4. `edge_dependency_pairs` returns phase-materialised edges; gate insertion
   handles `phase_synthesis` the same as `review`. Tests: `plan_workflow`
   shape on a v1.1 fixture matches the snapshot.
5. Status payload `phases` block in `cli/introspect.py::status`. Tests:
   v1 omission, v1.1 inclusion.
6. Dashboard panel + service surface. Tests: snapshot rendering.
7. Generator `multi_phase` shape. Tests: shape compiles to a valid v1.1
   workflow.
8. `workflow upgrade --add-phases`. Tests: dry-run, apply, refuse-running,
   refuse-already-v1.1.
9. CI integration fixture and end-to-end test exercising a two-phase run.

Each step is independently shippable behind the existing v1 acceptance
criterion (everything stays unchanged for v1 workflows) and the v1.1
opt-in.
