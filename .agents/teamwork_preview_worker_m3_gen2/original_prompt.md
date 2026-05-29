## 2026-05-29T07:58:26Z

### Objective:
Implement and align the codebase with RFC 0091 (Lane Health Module). You must carefully follow the detailed architectural design and step-by-step caller migration strategy formulated by the Architect in their handoff report:
`~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen2/handoff.md`

### Tasks to Execute:
1. **Implement `supervisor.TmuxMeta` Metadata Struct**:
   - In `go/pkg/supervisor/tmux_meta.go` (create if not exists), define the structured single metadata codec `TmuxMeta` matching the fields in the Architect's report: `AgentLoopMode`, `Tmux`, `DeliveryLiveness`, etc.
   - Include validation methods like `IsValidTmux() bool`.

2. **Implement `go/pkg/lanehealth` Module**:
   - Create package directory `go/pkg/lanehealth` and source files.
   - Implement enum type `LaneReason` with target constant wire-compatible string values (`no_attached_supervisor`, `start_token_unverified`, `pid_gone`, `tmux_metadata_corrupt`, etc.).
   - Define type `Health` and predicates `LiveTarget()`, `CanDeliver()`, and `IsStalled()`.
   - Implement the pure classifier `Classify(Facts, time.Time) Health` representing the exact precedence rules of the state machine (no database or system access dependencies).
   - Define structure `Facts` containing join states of supervisors, processes, and session activity.
   - Implement `LegacyMap` helper to format `Health` into compatible legacy `map[string]any` structures for existing CLI/RPC wire compatibility.
   - Implement the database-wired `Checker` and standard process probe adapter interface `Probe` / `ProdProbe`.

3. **Migrate Callers & Delete Duplication**:
   - In `go/pkg/mutations/mutations.go`: Migrate `sessionLaneAttestation` to run in terms of `Checker.Check` and `LegacyMap`, removing the custom 3-table SQL join and nested logic blocks.
   - In `go/pkg/reads/supervision.go`:
     - Delete `applySupervisorLaneAttestation` and `tmuxStartTokenUnverified`.
     - Update status, list, and show views (`HandleSuperviseStatus`, `HandleSuperviseShow`) to leverage the unified `lanehealth` checker or its fields, eradicating parallel status derivations.
   - In `go/pkg/mutations/supervision_control.go` (`reconcileSupervisorForDelivery`): Use `lanehealth.Checker` to verify liveness and delivery-state health, while keeping the transactional `FOR UPDATE` lock inside the mutation block.
   - In `go/pkg/mutations/interrogation.go` (`requireLiveTarget`): Use `Checker.Check().LiveTarget()` instead of ad-hoc map checks.
   - In `go/pkg/reads/status.go` (`statusActiveSessions`): Update active sessions status queries to read `lanehealth` checker axes, deleting duplicate parsing logic.

4. **Unit and Integration Testing**:
   - Implement `TestClassify` pure table-tests covering all attestation/reason scenarios.
   - Implement integration test `TestLoad` (using pgtest + a mock Probe implementation) to verify DB facts loading.
   - Implement compatibility tests to verify that `LegacyMap` exactly matches previous wire JSON maps for all reasons.
