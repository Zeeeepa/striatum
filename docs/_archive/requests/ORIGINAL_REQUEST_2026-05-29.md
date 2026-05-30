# Original User Request

## Initial Request — 2026-05-29T03:38:19Z

An expert systems architecture review of the Striatum codebase, producing a highly detailed, grounded, and actionable markdown report for the sole maintainer.

Working directory: `~/git/striatum`

Integrity mode: development

## Requirements

### R1. Repository Inventory & Deep Source-Code Audit
The agent team must read and analyze the Striatum repository codebase (located at `~/git/striatum`), identifying the project structure, domain model, daemon/MCP/CLI boundaries, and storage transition state. The report must explicitly list every source file actually read during the review.

### R2. Comprehensive Architecture Review Report
The agent team must write a markdown file in the root of the working directory named `STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md` (matching the `<PROJECT_NAME>_ARCHITECTURE_REVIEW_<MODEL_NAME>_YYYY-MM-DD.md` pattern). The report must be between 3,000 and 5,000 words, dense, highly technical, and completely free of filler or generic SaaS-ops advice.

The report must contain the following 11 sections in order:
- **0. Files reviewed**: A flat list of absolute or repository-relative paths of all files read.
- **1. Executive summary**: 5–10 honest, fluff-free bullets.
- **2. What the project is trying to be**: Goals, principles, domain model, and operating model, citing repository docs.
- **3. Current architecture**: Components, runtime, state/storage, interfaces (CLI, API, daemon, web, MCP), test posture, and release posture. Note any discrepancies between documentation and actual implementation.
- **4. Strengths**: Specific architectural decisions and abstractions worth preserving, with justifications.
- **5. Concerns**: Critical architectural concerns ranked as Blocker, Serious, or Smell, each backed by specific code evidence.
- **6. North-star architecture**: A greenfield architecture design tailored specifically to the project's real constraints (single operator, laptop/homelab runtime, no Kubernetes, no managed cloud, demo-stage maturity).
- **7. Recommended changes**: A table mapping priority, change, rationale, benefit, risk, and rough effort (hours/days/weeks).
- **8. Functionality I'd add**: A table mapping proposed feature, priority, rationale, benefit, risk, and rough effort.
- **9. Execution roadmap**: Concrete first step (startable today), near-term (month), medium-term (quarter), and long-term milestones.
- **10. Open questions**: Crucial architectural or design questions that could not be determined solely from the codebase.

### R3. Tri-Voice Grounding & Disagreement
All non-trivial claims in the report must be grounded in specific file paths, functions, or line ranges. The review must maintain three clearly labeled, distinct voices:
- **Stated**: What the documentation or READMEs claim.
- **Actual**: What the codebase actually implements.
- **Mine**: The agent team's expert judgment.
The agent team must actively disagree and argue against principles or designs they believe are wrong, avoiding describe-then-defer patterns.

## Acceptance Criteria

### Report Completeness & Structure
- [ ] The file `STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md` is successfully generated in `~/git/striatum`.
- [ ] The generated report contains all 11 required sections (numbered 0 to 10) in the exact specified order.
- [ ] The report length is between 3,000 and 5,000 words.

### Technical Grounding & Tone
- [ ] Every non-trivial architectural claim in sections 3, 4, and 5 is grounded in a specific file path and/or line range (e.g. `docs/reference/spec.md` or `go/pkg/daemon/server.go:120-150`).
- [ ] The report maintains the three voices (`stated`, `actual`, `mine`) throughout and does not blur them.
- [ ] The report contains no vague verbs like "improve", "enhance", "consider", or "explore" in the recommendation tables, instead naming precise changes.
- [ ] The report contains no generic SaaS or cloud-ops advice (e.g. no "use Kubernetes", "add AWS RDS", or "use a third-party feature flag service").

## Follow-up — 2026-05-29T07:45:46Z

Resolve remaining Github issues, work through the Striatum codebase todo list, and scaffold, implement, and run RFCs 0090 and 0091, along with other active follow-up items.

Working directory: ~/git/striatum
Integrity mode: development

## Requirements

### R1. Resolve Tracked Issues & TODOs
- **MCP Settings Cleanup (Issue #51):** Ensure the token-bearing, gitignored `.gemini/settings.json` file created during `agy` MCP bootstrap is cleanly removed on session completion, supervisor stop, or unexpected daemon recovery termination.
- **Supervised Exit Terminal Persistence (D146):** Ensure unexpected PTY supervised child process exits are authoritative daemon-supervision state transitions and are durably persisted in PostgreSQL, rather than just being computed as read-only projections.
- **Conversation UI Rendering (F43):** Support querying and rendering multi-party conversation trajectories in the web/chat UI at `/v1/runs/{runID}/conversations[/{id}]` as a server-side read-only view, similar to the existing interrogation UI (D142).

### R2. Scaffold and Implement RFC 0090 (Workspace Security & Attestation Parity)
- Fully transition RFC 0090 from `proposed` to `accepted` in its header, land the specification, ubiquitous language, and command authority matrix updates, and implement its core security mechanisms.
- Implement process attestation checks to structurally verify that a supervised process produces its published artifacts, hardening attestation against client-side fabrication.
- Support robust directory/workspace isolation for concurrent supervised lanes, ensuring lanes cannot access out-of-scope files or clobber sibling workspaces.

### R3. Align with RFC 0091 (Lane Health Module)
- Fully integrate the newly-landed `go/pkg/lanehealth` module by migrating any remaining ad-hoc liveness, attestation, and delivery checks in the mutation (`pkg/mutations`) and read (`pkg/reads`) paths onto this unified checker.
- Eradicate duplicate `start_token` and `delivery_liveness` parsing logic across the codebase, ensuring `lanehealth` remains the sole classification authority.

## Acceptance Criteria

### Security & Correctness
- [ ] Ephemeral Settings File (`.gemini/settings.json`) is cleanly deleted on supervisor stop, kill, and graceful completing.
- [ ] Unexpected supervisor exits are permanently recorded in Postgres as terminal states.
- [ ] Workspace attestation forgery checks correctly deny unattested lanes from masquerading bylines.
- [ ] Unit tests for workspace security, attestation parities, and unified lane health pass successfully.

### System Verification & Integration
- [ ] The entire Go test suite (`go test -race ./...`) compiles and passes cleanly with zero race conditions or lints.
- [ ] The automated retired vocabulary grep gate remains fully operational and passes without warnings.
- [ ] Command authority matrix and spec updates are successfully documented.

## Follow-up — 2026-05-29T12:00:25Z

Triage and resolve all six outstanding GitHub issues in the Striatum repository (including #49, #54, #57, #58, #59, and #60), ensuring all fixes are fully verified, robustly integrated, and all tests pass.

Working directory: ~/git/striatum
Integrity mode: development

## Requirements

### R1. Resolve Issue #57 (Write-Scope Strictness)
- Relax the git-based write-scope checker to ensure it only flags new files or mutated files outside `allowed_paths` as violations. Clean or stashed files (transitioning from dirty to clean compared to baseline) must not trigger a violation.

### R2. Resolve Issue #58 (Duplicate Artifact Publication in `submit-review`)
- Update review submission handlers so they do not crash with a raw unique key database constraint error if a finding artifact has already been published. Log a helpful, user-friendly message and proceed with recording the verdict.

### R3. Resolve Issue #59 (Strict Front-Matter List Formatting)
- Enhance front matter parsing in synthesis and finding artifacts to support standard multi-line YAML formatting for lists (such as `inputs`). Return precise syntax errors with line numbers rather than a silent exit-code 6.

### R4. Resolve Issue #60 (Rigid Session Lifetime Enforcement)
- Support a parameter or automated logic to replace duplicate active sessions on the same lane for a run, avoiding manual unregister blocks.

### R5. Resolve Issue #49 & #54 (PTY Supervision, Rebridge, & Re-queueing)
- Triage and resolve Issue #49 (re-queued packet after checkpoint resolution does not resume) and Issue #54 (RFC 0089 Phase 2 supervision rebridge and status details).

## Acceptance Criteria

### Correctness & Verifications
- [ ] All six open issues (#49, #54, #57, #58, #59, #60) are resolved with corresponding regression tests in the codebase.
- [ ] Running `striatum complete` or `striatum submit-review` does not fail due to files transitioning from dirty to clean compared to baseline.
- [ ] Running review submission after manual publication successfully records the verdict without database unique key constraint errors.
- [ ] Markdown artifacts with multi-line list front matter parse successfully and report detailed line-number syntax errors on failure.
- [ ] The entire Go test suite (`go test -race ./...`) compiles and passes cleanly with zero race conditions.
- [ ] The automated retired vocabulary grep gate remains fully operational and passes.
