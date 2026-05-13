author: designer-unknown-model-002

# Design: RFC 0045 Python Core Implementation

This design specifies the Python-core changes required to support multi-phase workflows as defined in RFC 0045. The implementation enables coarser-grained units of work (phases) that gate transitions through mandatory synthesis jobs.

## 1. Schema Extensions (`striatum.workflow.v1.1`)

The workflow schema is bumped to `striatum.workflow.v1.1`. This version is backwards-compatible with `v1` but unlocks phase-aware features.

### 1.1 Top-level `phases` array
The workflow root gains an optional `phases` array. Each entry defines a phase:
- `id`: unique string identifier.
- `name`: human-readable title.
- `color`: (optional) Hex color for UI banding.
- `description`: (optional) Narrative summary.

### 1.2 Job `phase_id` field
Jobs gain an optional `phase_id` field. In a `v1.1` workflow with a non-empty `phases` array, this field is required for all jobs to ensure unambiguous phase assignment.

### 1.3 `phase_synthesis` job type
A new job type `phase_synthesis` is introduced. It acts as the terminal gate for a phase.

## 2. Validator Extensions (`src/striatum/workflow.py`)

The validator is the primary enforcer of phase integrity.

### 2.1 Schema Version Validation
`validate_workflow` (L475) is updated to accept both `striatum.workflow.v1` and `striatum.workflow.v1.1`. It will branch logic based on the version:
- `v1`: Existing single-phase behavior; `phases` and `phase_id` are forbidden.
- `v1.1`: Multi-phase behavior; `phases` is optional (defaults to single implicit phase if absent).

### 2.2 Phase Definition Validation
A new helper `_validate_phases(workflow)` will:
- Verify `phases` is a list of valid phase objects.
- Ensure all phase IDs are unique.
- Ensure every job in a `v1.1` workflow (with phases) declares a `phase_id` matching an entry in `phases`.
- Ensure each phase contains exactly one `phase_synthesis` job.

### 2.3 Cross-Phase Edge Enforcement
The validator will iterate over edges produced by `edge_dependency_pairs` (L731).
- **Rule**: Any edge where `source.phase_id != target.phase_id` MUST involve a `phase_synthesis` job as the source.
- **Rule**: A `phase_synthesis` job in Phase N cannot depend on jobs in Phase M > N.
- **Rule**: `phase_synthesis` jobs gain implicit dependencies on all other jobs in their own phase. `edge_dependency_pairs` will be updated to inject these virtual edges so that the synthesis job truly gates the phase exit.

## 3. Runtime Materialization (`src/striatum/workflow.py`)

### 3.1 DB Migration (v15)
A new migration in `src/striatum/migrations.py` will:
- Add `phase_id TEXT` to the `jobs` table.
- Rebuild the `jobs` table to update the `CHECK` constraint on `job_type`, adding `phase_synthesis`.

### 3.2 Run Creation
`create_run` (L606) is the authoritative site for job graph materialization.
- **Job Insertion** (L644): Captures `phase_id` from the workflow JSON and persists it to the DB.
- **Dependency Insertion** (L693): `edge_dependency_pairs` will now return the expanded set of edges (including implicit phase synthesis edges).
- **Gate Logic** (L696): The condition `if upstream_job.get("type") == "review":` will be extended to `if upstream_job.get("type") in ("review", "phase_synthesis"):` to ensure phase gates require an `accept` verdict.

## 4. Status and Reporting

### 4.1 CLI Introspect (`src/striatum/cli/introspect.py`)
`status` (L161) will include a `phases` block in its return payload:
- Aggregates job states grouped by `phase_id`.
- Calculates `percent_complete` per phase (completed jobs / total jobs).
- Identifies the `active_phase_id`.

### 4.2 Dashboard (`src/striatum/dashboard.py`)
`gather_payload` (L101) consumes the enhanced status. The terminal dashboard will render a "Phases" progress panel when phase metadata is present.

### 4.3 Service (`src/striatum/service.py`)
`_render_run_detail_page` (L993) passes the phase progress data to the frontend context. This allows the web UI to render phase bands and progress bars.

## 5. Generator and CLI Ergonomics

### 5.1 Multi-Phase Shape (`src/striatum/workflow_generator/`)
`SHAPES` in `core.py` (L23) gains `multi_phase`. The generator will support a `phases` parameter where each phase defines its own tracks and lanes. It will automatically emit the `phase_synthesis` jobs and the appropriate `phase_id` mappings.

### 5.2 Workflow Upgrade (`src/striatum/cli/workflow.py`)
`workflow_upgrade` gains `--add-phases`.
- **Heuristic**: Clusters jobs by the prefix of their `parallel_group` field.
- **Action**: Wraps clusters in phase definitions, injects `phase_synthesis` jobs, and updates `schema_version` to `v1.1`.

## 6. Compatibility and Testing

- **Backwards Compatibility**: Existing `v1` workflows continue to validate and execute without change.
- **Test Matrix**:
  - `tests/test_workflow_v11.py`: Verifies `v1.1` validation (positive and negative cases for phase boundaries).
  - `tests/test_multi_phase_lifecycle.py`: End-to-end run from `run prepare` through phase gates.
  - `tests/test_workflow_upgrade_phases.py`: Verifies the `--add-phases` heuristic on legacy dogfood workflows.
