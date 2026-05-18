# Striatum Architecture Remediation Plan

Date: 2026-05-16
Companion review: `STRIATUM_ARCHITECTURE_REVIEW_2026-05-16.md`
Status: superseded historical input with shipped follow-through. Current
completion status lives in `CHANGELOG.md`, `docs/TODO.md`, and
`docs/ROADMAP.md`.

## Goal

Collapse Striatum onto the intended runtime spine:

```text
daemon RPC/MCP -> one transition engine -> Postgres -> durable artifacts
```

Everything else should become one of:

- a generated client,
- a bootstrap/admin helper,
- a migration/import path,
- a test fixture,
- or historical dogfood provenance.

This plan is ordered to reduce correctness risk before expanding product surface.

## Operating rules

1. Do not add major new product surface until production workflow mutations no longer depend on the legacy SQLite path.
2. Preserve the local-first, no-transcripts, no-provider-SDK product boundary.
3. Keep every phase dogfoodable and shippable independently.
4. Prefer deleting fallback paths over documenting them.
5. Treat parity tests as transition scaffolding, not permanent architecture.
6. Current operator-visible behavior should fail closed when daemon routing is unavailable.

## Phase 0: Baseline inventory and guardrails

Purpose: make the current split-brain surface measurable before changing it.

Steps:

1. Build a command inventory table.
   - Source inputs: `src/striatum/cli/parser.py`, `src/striatum/daemon_rpc/registry.py`, `src/striatum/cli/daemon_rpc_route.py`, `src/striatum/daemon_rpc/server.py`, `src/striatum/daemon_pg/handlers/`, `go/pkg/`.
   - Columns: CLI command, RPC method, required capability, repo scope, Python PG handler, Go handler, CLI fallback, SQLite dependency, docs status.
   - Output: `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`.

2. Add a production-mode SQLite tripwire.
   - Instrument `striatum.db.connect()` so tests can assert it is not called by production-mode commands.
   - Add a test helper that runs representative CLI commands with daemon-required mode active and fails if the legacy SQLite connect path is reached.
   - Keep explicit migration tests exempt.

3. Add a fallback-route coverage test.
   - Assert every non-bootstrap method in the daemon registry is either native PG implemented or intentionally marked `migration_only`, `bootstrap_only`, or `not_implemented`.
   - Fail if a production method reaches `CLI_ROUTES` fallback.

4. Freeze new hand-written route maps.
   - Add a short contributor rule to `AGENTS.md` or equivalent: new RPC methods must update the command authority matrix until generated contracts land.

Acceptance criteria:

- A committed matrix names every command/method and its current authority path.
- CI has a failing test if a production command opens repo-local SQLite.
- CI has a failing test if a production daemon method silently falls back to `striatum.api.invoke`.

Suggested verification:

```bash
make lint typecheck test
make test-multi-repo CORE=python
make test-multi-repo CORE=go
```

## Phase 1: Close the production SQLite fallback

Purpose: make D094/D104 true in code, not only in docs.

Steps:

1. Classify commands into four buckets.
   - `daemon_native`: normal production read/write route.
   - `bootstrap_admin`: daemon lifecycle, init, repo registration, local install, skills/plugins.
   - `local_file_authoring`: workflow validate/graph/generate if deliberately allowed without live state.
   - `legacy_migration`: SQLite import and golden fixtures only.

2. Port remaining production CLI fallback commands to native PG handlers.
   - Use the matrix from Phase 0.
   - Prioritize commands used by ordinary runs: run lifecycle, session loop, artifact publish, review, recovery, supervisor, status/why/doctor/dashboard, list, run summary, evidence export.

3. Remove daemon-side `CLI_ROUTES` fallback for production methods.
   - In `src/striatum/daemon_rpc/server.py`, keep fallback only for methods explicitly tagged `legacy_migration` or test-only.
   - Prefer `method_not_implemented` over fallback when a method is registered but not ported.

4. Convert `striatum.api.invoke` to a compatibility shim.
   - Mark it bootstrap/test/local-service legacy only.
   - Do not let daemon RPC use it for production transitions.

5. Move legacy SQLite domain code behind a migration namespace.
   - Target shape: `src/striatum/legacy_sqlite/` or `src/striatum/migration/sqlite_import.py`.
   - Keep the old code importable for migration and golden fixtures.
   - Remove it from normal command dispatch.

6. Update docs.
   - Sweep `docs/MCP.md`, `docs/WRITING_WORKFLOWS.md`, `docs/SPEC.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_AGENT.md`, and `docs/HOW_TO_HUMAN.md`.
   - Current docs should say daemon/Postgres/MCP is normal.
   - Historical SQLite text should be labeled migration-only or historical.

Acceptance criteria:

- In production-mode tests, ordinary commands cannot open `.striatum/state.sqlite3`.
- Daemon RPC does not call `striatum.api.invoke` for production transitions.
- Docs no longer present SQLite or direct mode as ordinary operator flow.
- `striatum inbox --json` docs are corrected or a real global inbox is implemented.

Suggested dogfood:

- A small code-change workflow using daemon MCP only.
- No operator CLI fallback except documented bootstrap/admin commands.

## Phase 2: Create a single method-contract source

Purpose: stop hand-maintaining the same command vocabulary in many files.

Steps:

1. Define a method contract file.
   - Suggested path: `contracts/daemon_methods.json` or `contracts/daemon_methods.toml`.
   - Fields: method, lifecycle status, required capability, repo scope mode, request schema, response schema, CLI command mapping, MCP exposure, Python handler, Go handler.

2. Generate Python registry artifacts.
   - Generate `src/striatum/daemon_rpc/registry.py` or a data module it imports.
   - Generate method etag from the contract.

3. Generate Go registry artifacts.
   - Generate `go/pkg/rpc/registry_gen.go`.
   - Ensure etag parity with Python.

4. Generate CLI route translation.
   - Replace most of `src/striatum/cli/daemon_rpc_route.py` lookup boilerplate with generated mapping.
   - Keep custom param adapters only where command syntax truly differs from RPC params.

5. Generate MCP tool descriptors.
   - Use the same request schema and capability metadata.
   - Tools/list should derive from the contract and authorization scope.

6. Generate docs tables.
   - At minimum: CLI reference method table and daemon method registry table.
   - Include lifecycle status: stable, deprecated, migration-only, not implemented.

7. Add contract parity tests.
   - Python registry etag equals Go registry etag.
   - CLI mappings reference known methods.
   - MCP exposed tools reference known methods.
   - Every implemented handler is declared in the contract.

Acceptance criteria:

- One contract source can regenerate Python registry, Go registry, CLI mapping, MCP descriptors, and docs tables.
- Manual registry drift becomes a CI failure.
- Deprecated aliases are visible in the contract, not scattered comments.

Suggested verification:

```bash
make lint typecheck test
make daemon-go-test
make test-multi-repo CORE=python
make test-multi-repo CORE=go
```

## Phase 3: Decide the daemon core strategy

Purpose: avoid maintaining two full daemon products indefinitely.

Decision point:

Choose one of these two paths.

### Option A: Go-primary daemon

Use Go for the long-running daemon, process supervision, PTY, signal handling, and distribution. Keep Python for CLI client, workflow generation, migration tooling, docs tooling, and tests.

Steps:

1. Mark Python daemon as transition core.
2. Finish Go implementation of all stable daemon-native methods.
3. Make `striatum daemon start --core go` a first-class supported path.
4. Package Go daemon binaries into the Python wheel.
5. Route Python CLI to the Go daemon by default when available.
6. Retire Python daemon method growth.

### Option B: Python-primary daemon plus Go supervisor helper

Use Python as the domain daemon and use Go only for process/PTY supervision where Python is weak.

Steps:

1. Stop expanding Go domain mutation handlers.
2. Remove or demote Go not-implemented method registrations.
3. Build a narrow Go supervisor process with a small protocol.
4. Keep daemon RPC and domain transitions in Python.
5. Use Go helper only through daemon-owned supervisor methods.

Recommendation:

Choose Option A if packaged daemon operation and PTY supervision are core to the product. Choose Option B if rapid domain iteration matters more than daemon operations for the next two quarters.

Acceptance criteria:

- One core is named primary in `docs/DECISION_LOG.md`.
- The roadmap stops treating both cores as equal owners of all behavior.
- CI reflects the decision: either full Go parity as release-blocking, or Go helper tests only.

## Phase 4: Make the web service daemon-first

Purpose: prevent the local web UI from becoming a second control plane.

Steps:

1. Split `src/striatum/service.py`.
   - Suggested modules:
     - `service/routing.py`
     - `service/security.py`
     - `service/daemon_client.py`
     - `service/dto.py`
     - `service/templates.py`
     - `service/chat.py`
     - `service/sse.py`
     - `service/repo_view.py`

2. Replace SQLite-shaped reads with daemon RPC reads.
   - Run list, run detail, job detail, artifacts, doctor, status, why, recovery panel.

3. Replace mutation POSTs with daemon RPC calls.
   - Keep browser-side confirmation and CSRF/context-token checks.
   - The server should not have a separate mutation whitelist that diverges from the daemon method registry.

4. Make SSE read from daemon events or daemon event stream.
   - Avoid direct database reads from web code unless using an explicit daemon read API.

5. Remove SQLite health check from web startup.
   - Replace with daemon health and repository registration checks.

6. Add web/daemon parity tests.
   - Same payload shape from CLI read method and web route.
   - Mutations fail closed without daemon authorization.

Acceptance criteria:

- `service.py` shrinks substantially or becomes a package entrypoint.
- Web UI opens no repo-local SQLite state in production.
- Web routes use daemon RPC DTOs.
- The mutation gate derives from daemon method capabilities.

## Phase 5: Build real escalation inbox

Purpose: align RFC 0053 docs with executable behavior.

Steps:

1. Model escalation explicitly.
   - Add `escalations` table or a typed subset of blockers with stricter schema.
   - Fields: escalation_id, run_id, job_id, source, class, state, created_at, resolved_at, decision_id, artifact_id.

2. Add an escalation artifact kind.
   - Schema: `striatum.escalation.v1`.
   - Link AI-authored escalation artifacts to escalation records.

3. Add daemon methods.
   - `escalation.list`
   - `escalation.show`
   - `escalation.resolve`

4. Redesign `inbox`.
   - `striatum inbox --all`
   - `striatum inbox --run-id <id>`
   - `striatum inbox --session-id <id>` remains a packet helper or is renamed `packet inbox`.

5. Update docs and skill bundles.
   - Human principal instructions should point to the real escalation inbox.
   - Operator-on-behalf packet helper should have a distinct name to avoid role confusion.

Acceptance criteria:

- The example in `docs/USING_STRIATUM.md` runs as written.
- Principal inbox shows escalations/open blockers without requiring a session id.
- Packet-helper behavior remains available for operator-on-behalf workflows.

## Phase 6: Harden process supervision

Purpose: make supervised agent lanes reliable enough to be normal workflow infrastructure.

Steps:

1. Add daemon-owned PTY supervision.
   - Implement PTY start/send/stop/status in the chosen daemon core.
   - Preserve no-transcript default.

2. Define a wrapper control protocol.
   - Packet delivered.
   - Packet accepted by wrapper.
   - Agent subprocess started.
   - Artifact observed.
   - Heartbeat/progress.
   - Agent process exited.

3. Separate control channel from model output.
   - Never parse model stdout as state.
   - Let wrapper control messages go to daemon over a pipe/socket/file descriptor.

4. Add reattach.
   - After daemon restart, recover supervisor pointers and process identity.
   - Mark uncertain sessions lost rather than silently reattaching.

5. Strengthen lane-liveness attestation.
   - Tie attestation to daemon-owned supervisor identity.
   - Preserve honest "not authorship proof" terminology.

6. Add supported wrappers.
   - Codex, Claude Code, Gemini CLI, generic.
   - Test wrappers with stub commands and PTY behavior.

Acceptance criteria:

- PTY-dependent CLIs can be supervised.
- Packet delivery has an explicit wrapper ack.
- Daemon restart does not leave invisible active sessions.
- Doctor surfaces stale/lost/uncertain supervisor states.

## Phase 7: Add workflow risk lint and review diversity enforcement

Purpose: make repeated dogfood lessons executable.

Steps:

1. Add workflow lint command.
   - `striatum workflow lint <workflow.json> --json`
   - Do not overload structural validation with all advisory guidance.

2. Add same-model review warnings.
   - Detect implementer/reviewer pairs with same declared model family.
   - Start as warning if needed.

3. Add refuse-by-default option.
   - Reject same-model implementer/reviewer pairs unless workflow sets an explicit override with rationale.

4. Add broader lint rules.
   - Broad write scope.
   - Repo-write without worktree isolation.
   - Review without fresh context.
   - Missing escalation path.
   - Required postures with weak coverage.
   - Attested-byline mode without supervised lanes.

5. Surface lint in generator and web workflow editor.
   - Show risk warnings before `run prepare`.

Acceptance criteria:

- Codex/codex or same-model co-blindness pattern is caught before run start.
- Workflow generator emits safer default lane/review topology.
- Overrides are auditable.

## Phase 8: Implement auto-finalize from front matter

Purpose: reduce operator-on-behalf toil when agents wrote valid artifacts but failed to publish/complete.

Steps:

1. Implement expected-artifact watcher logic.
   - Check expected path exists.
   - Validate artifact kind and front matter.
   - Validate byline.
   - Require stable mtime grace period.
   - Confirm active lease/session ownership.

2. Add daemon method.
   - `recovery.auto_finalize`
   - Dry-run and live modes.

3. Add recovery policy controls.
   - Workflow opt-in first.
   - Later default-on after dogfood confidence.

4. Record explicit events.
   - `artifact.auto_finalized`
   - `job.auto_finalized`
   - Include rationale and validation facts, not model prose.

5. Add web and dashboard surfaces.
   - Show "eligible for auto-finalize."
   - Show exact refusal reason when not eligible.

Acceptance criteria:

- A stalled session with valid expected artifact can complete without manual publish-on-behalf.
- Malformed front matter, missing byline, wrong author line, and racey file writes all refuse.
- Evidence export clearly distinguishes agent-published and auto-finalized artifacts.

## Phase 9: Clean packaging and UI assets

Purpose: keep wheel and review surface manageable.

Steps:

1. Make `ui-build` clear old build output.
   - Delete `src/striatum/web/static/build/*` before Vite emits assets.
   - Regenerate manifest after build.

2. Ensure stable loader behavior.
   - Templates should load entries by manifest or stable filenames.
   - Avoid accumulating hashed chunks.

3. Move build-only dependencies.
   - Move `@vitejs/plugin-react` to `devDependencies`.

4. Add package size check.
   - Fail if bundled web assets exceed a chosen threshold without explicit override.

5. Add bundle drift check.
   - Existing `ui-check-bundle` should remain, but against cleaned output.

Acceptance criteria:

- Build directory contains only current assets.
- Wheel size is tracked.
- UI build drift is deterministic.

## Phase 10: Improve day-zero setup

Purpose: reduce adoption friction from Postgres and daemon setup.

Steps:

1. Add local role provisioning.
   - `striatum daemon doctor --provision-rw-role`
   - `striatum daemon doctor --repair-grants`

2. Add service manager helpers.
   - `striatum daemon service install`
   - `striatum daemon service start`
   - `striatum daemon service status`
   - Support systemd user and macOS launchd.

3. Add guided adoption.
   - `striatum adopt`
   - Inspect repo, initialize `.striatum/`, register repo, install skills, scaffold DDD docs, suggest workflow location.

4. Add optional dev substrate.
   - Docker Compose or bundled local Postgres profile.
   - Keep production system Postgres path documented separately.

5. Add first-run smoke.
   - `striatum doctor --first-run`
   - Verifies daemon, DB, token, repo registration, MCP capability, and sample read route.

Acceptance criteria:

- New user can get from install to first registered repo with one guided flow.
- Role/grant mistakes produce repair commands.
- Docs distinguish dev-local and production-local setup.

## Phase 11: Add replay, archive, and corpus v2 foundations

Purpose: make audit and memory boundaries stronger after the runtime spine is clean.

Steps:

1. Add deterministic run archive.
   - `striatum run archive --run-id <id> --out <dir>`
   - Include artifact manifest, hashes, event chain head, audit refs, redaction policy version.

2. Add replay verifier.
   - Verify event chain and materialized run state.
   - Detect missing artifacts, hash mismatch, or broken supersession.

3. Implement Corpus Contract V2.
   - Multi-corpus identity.
   - Redaction tiers.
   - Incremental watermarks.
   - Validation command.

4. Keep augmentation optional.
   - No runtime dependency on Engram or any memory service.
   - Context injection only when workflow-authored and failure-tolerant.

Acceptance criteria:

- A run can be archived and verified from a fresh checkout plus daemon export.
- Corpus v2 bundles validate independently.
- Striatum still runs unchanged without any memory consumer.

## Phase 12: Optional Git/PR integration

Purpose: complete code-change workflows without making autonomous commits the default.

Steps:

1. Add read-only git snapshot methods.
   - Branch, status, diff summary, untracked files.

2. Add commit request artifact.
   - Agent asks for commit with structured message/body.
   - Principal/operator confirms.

3. Add explicit commit apply verb.
   - `git.commit.apply --request-id <id> --confirmed`
   - Audited and gated.

4. Add optional GitHub plugin later.
   - Prepare PR body from run archive and artifacts.
   - Do not make hosted GitHub a core dependency.

Acceptance criteria:

- Source changes can move from reviewed artifact to explicit commit request.
- No autonomous commit occurs without a confirmation method.
- Hosted integrations remain optional plugins.

## Recommended release sequence

1. `v1.56`: Phase 0 guardrails and command authority matrix.
2. `v1.57`: Phase 1 first slice: remove production fallback for highest-use workflow-loop verbs.
3. `v1.58`: Phase 1 complete: no production command opens SQLite.
4. `v1.59`: Phase 2 contract source and generated registries.
5. `v1.60`: Phase 3 daemon core decision and roadmap adjustment.
6. `v1.61`: Phase 4 daemon-first web service slice.
7. `v1.62`: Phase 5 real escalation inbox.
8. `v1.63`: Phase 6 PTY supervision prototype.
9. `v1.64`: Phase 7 workflow risk lint and same-model enforcement.
10. `v1.65`: Phase 8 auto-finalize.
11. `v1.66`: Phase 9 UI packaging cleanup.
12. `v1.67`: Phase 10 first-run setup improvements.
13. `v1.68+`: Replay/archive/corpus v2 and optional Git/PR integration.

The exact version numbers are placeholders. The dependency order matters more than the numbering.

## Definition of done for the architectural remediation

The remediation is done when all of the following are true:

- Production workflow commands do not open repo-local SQLite.
- The daemon has no production `CLI_ROUTES` fallback.
- CLI, MCP, Go registry, Python registry, and docs derive from one method contract.
- One daemon core is named primary.
- The web UI is a daemon client, not a direct state-store peer.
- The human principal has a real escalation inbox.
- Supervised lanes support PTY and daemon-owned reattach/lost-state handling.
- Workflow lint catches known dogfood anti-patterns before run start.
- Auto-finalize handles valid written artifacts without manual publish-on-behalf.
- Day-zero setup can provision or repair the common local Postgres/daemon path.

## First concrete task to start

Start with Phase 0, task 1:

```text
Create docs/architecture/COMMAND_AUTHORITY_MATRIX.md.
```

Do not guess. Generate it from the current parser, RPC registry, route translator, Python PG handler decorators, and Go registered handlers. Then add a failing test for any stable production method whose authority path is ambiguous.

That matrix will make the rest of the remediation schedulable instead of impressionistic.
