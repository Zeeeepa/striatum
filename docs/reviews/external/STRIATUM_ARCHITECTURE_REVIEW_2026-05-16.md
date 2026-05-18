# Striatum Architecture Review

Date: 2026-05-16
Reviewer: Codex, systems architecture review
Scope reviewed: `<striatum-repo>` on `main` at recent HEAD `a2fbb7f`, with existing staged/untracked dogfood material under `docs/dogfood/061` through `063` left untouched.
Status: historical snapshot as of 2026-05-16. Current remediation status
lives in `CHANGELOG.md`, `docs/TODO.md`, and `docs/ROADMAP.md`; D105
supersedes any Go-primary direction in this review.

## Executive summary

Striatum is not just "a workflow runner for agents." Its strongest idea is that it treats multi-agent software work as a local, auditable domain model: runs, sessions, leases, jobs, artifacts, verdicts, blockers, reviews, and decisions are not incidental data structures, they are the vocabulary of the product. That is the right axis. Most competing systems either over-index on model invocation or under-model the workflow itself. Striatum's durable contribution is the opposite: it models the workflow first and keeps model providers behind lanes/adapters.

The architecture is directionally strong, but it is carrying a large amount of transition debt. The docs and decision log now say the product is daemon-required, Postgres-backed, and daemon-MCP-first. The implementation still contains three partially overlapping control planes:

1. Legacy repo-local SQLite business logic in `src/striatum/db.py`, `workflow.py`, and many CLI paths.
2. Python daemon RPC plus native Postgres handlers in `src/striatum/daemon_rpc/` and `src/striatum/daemon_pg/handlers/`.
3. A parallel Go daemon and Go Postgres handler port under `go/pkg/`.

That dual or triple implementation is the main architectural risk. It makes every invariant expensive to preserve, every new verb require multiple mappings, and every bug fix prone to landing in one path but not the others. The current codebase mitigates this with extensive tests and dogfood reports, but the better long-term answer is to finish the authority-boundary migration and delete the fallback spine.

If I were building greenfield, I would still keep the core product thesis: local-first, no provider SDKs in the runner, explicit workflow graph, structured leases and verdicts, durable artifacts, metadata-only audit, and model-portable lanes. I would change the implementation shape substantially: daemon-first from day zero, one authoritative state transition engine, generated CLI/MCP/UI clients, typed schema definitions as the source of truth, and an embedded-or-managed Postgres story that does not make day-zero setup feel like a database administration exercise.

## What Striatum is trying to be

The project goals are clear across `README.md`, `docs/PRD.md`, `docs/SPEC.md`, `docs/DDD.md`, and `docs/DECISION_LOG.md`:

- Local-first orchestration for terminal-based AI coding agents.
- Provider portability through lanes and process adapters, not model SDK imports.
- Deterministic coordination around workflow state, with AI operators using the same command surface as humans.
- Structured multi-lane review and repair loops to reduce reviewer co-blindness.
- Durable repo artifacts for decisions, findings, syntheses, reports, and evidence.
- Metadata-only audit chains, no broad transcript capture.
- A daemon-owned Postgres substrate as authoritative live state.
- A daemon MCP/control surface as the preferred operator/agent interface.

The product boundary is unusually well stated. The DDD framing is also valuable: the vocabulary is the model, and the legal mutation verbs are the enforcement mechanism. That framing explains why the CLI/RPC surface matters more than the process launcher.

The principles I see as load-bearing:

- **Vocabulary over orchestration scripts.** Jobs, leases, verdicts, blockers, and artifacts are first-class. Ad hoc marker files are explicitly not state.
- **Adapter boundary.** Core scheduling should not parse terminal output or infer provider behavior from model names.
- **Audit without surveillance.** Audit hashes and metadata matter; transcripts are intentionally excluded.
- **Local authority.** The runner coordinates local repositories and local agent processes; hosted coordination is out of scope.
- **Explicit workflow graphs.** Parallelism, cycles, review policies, and write scopes are authored, not inferred at runtime.
- **Artifacts as provenance, not message bus.** Repo files are durable evidence. Live state lives in the control plane.

Those principles are strong. I would preserve them.

## Current architecture as implemented

At a high level, the current system has these layers:

```text
Agent CLI / human operator
        |
        v
striatum CLI / daemon MCP / local web service
        |
        v
Daemon RPC envelope and method registry
        |
        v
Python PG handlers or Go PG handlers
        |
        v
Postgres striatumd schema
        |
        v
Durable repo artifacts + .striatum scratch
```

But the actual code path is more complicated:

```text
CLI dispatch
  -> daemon-required preflight
  -> daemon RPC route if mapped
  -> native PG handler if registered
  -> otherwise CLI_ROUTES fallback to striatum.api.invoke
  -> legacy SQLite-backed dispatch path
```

Concrete examples:

- `src/striatum/cli/dispatch.py:188-210` enforces daemon-required preflight, then attempts daemon RPC routing.
- `src/striatum/cli/dispatch.py:327-380` still handles workflow authoring and dashboard paths in-process.
- `src/striatum/daemon_rpc/server.py:230-264` resolves a native PG handler, then falls back to `CLI_ROUTES` and `striatum.api.invoke`.
- `src/striatum/api.py:14-32` is an in-process parser/dispatcher wrapper and still catches `sqlite3.Error`.
- `src/striatum/daemon_pg/handlers/context.py` carries the shared PG handler context, event append, packet construction, artifact validation, and many domain helpers.
- `go/cmd/striatumd/main.go` starts a Go daemon with the same wire protocol and registers Go read/mutation handlers plus not-implemented placeholders.

The data model is Postgres-first now. The schema under `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` defines the per-repository workflow tables in the daemon schema: runs, sessions, jobs, queue messages, leases, work packets, artifacts, verdicts, blockers, process executions, supervisors, worktrees, and events. Migration `0006_events_chain_anchors.sql` adds dedicated event hash-chain columns and a per-repo chain head.

The codebase is substantial:

- About 42k lines across Python and Go, excluding tests and docs from the rough `wc` sample.
- `src/striatum/service.py` is 3,870 lines.
- `src/striatum/workflow.py` is 2,187 lines.
- `src/striatum/db.py` is 2,165 lines and still the legacy SQLite domain implementation.
- `src/striatum/cli/dispatch.py` is 1,418 lines.
- The test suite has about 140 Python test files and roughly 1,050 `def test_...` functions.
- The daemon RPC registry has 93 method entries.
- The native Python PG handler registry currently has about 31 registered handler decorators.

The repository has a serious testing culture. That is one of the best signs in the project. There are targeted tests for migration, capability denial, audit chain concurrency, append-only role grants, web UI behavior, cross-repo harnesses, Go daemon smoke/audit behavior, recovery, process adapters, and corpus export.

## What is working well

### 1. The domain model is coherent

The strongest architectural asset is the shared language. `docs/DDD.md` is not decorative; it describes the real model. The code and docs consistently orbit around runs, jobs, sessions, leases, artifacts, verdicts, blockers, and decisions.

This gives Striatum a real product spine. You can add surfaces without changing the model: CLI, MCP, local web, daemon RPC, and future UI features are adapters over the same verbs.

### 2. The provider boundary is correct

Keeping model providers behind lanes and process commands is the right choice. The runner should not import OpenAI, Anthropic, Gemini, or any model SDK. It should coordinate processes and structured state. That preserves optionality and keeps the workflow model portable.

### 3. Audit scope is disciplined

The metadata-only audit approach is pragmatic. The project explicitly rejects transcript capture while still recording command metadata, hashes, decisions, and event chains. That is a good local-first trust model: enough to reconstruct control-plane history without quietly becoming a surveillance log.

### 4. The daemon/Postgres direction is right

Moving authoritative live state out of repo-local SQLite was the correct strategic decision. The target features already require it: multi-repo coordination, daemon-owned supervision, MCP mutation capabilities, stronger apply boundaries, cross-language daemon parity, and eventually better packaging.

### 5. Dogfood history is real evidence

The `docs/dogfood/` corpus, decision log, RFCs, and operator reports make the project easier to audit than most agent tooling. They also expose operational failures honestly: same-model reviewer co-blindness, wrapper stalls, missing front matter, operator-on-behalf publication, and migration edge cases.

This matters because Striatum's whole thesis is workflow reliability under messy agent behavior. The project is testing that thesis on itself.

## Main architectural concerns

### 1. There are too many authoritative-looking paths

The intended architecture says "daemon/Postgres is authoritative." The implementation still has a large legacy SQLite state engine and multiple fallback paths.

Examples:

- `src/striatum/db.py` still contains the old SQLite state machine and many transition helpers.
- `src/striatum/cli/dispatch.py` falls through to `ensure_initialized(repo)` and `connect(repo)` after the daemon routing block.
- `src/striatum/daemon_rpc/server.py` falls back from native PG handlers to `CLI_ROUTES` and `striatum.api.invoke`.
- `src/striatum/service.py` still depends heavily on `sqlite3.Connection` helper shapes and has a pre-bind SQLite health check at `service.py:3640`.
- The docs still contain stale or transitional SQLite claims in places such as `docs/MCP.md` and `docs/WRITING_WORKFLOWS.md`.

This creates an authority ambiguity. The code tries to fence it with daemon-required preflight and test-harness env vars, but the mental model remains expensive:

- Is this verb daemon-routed?
- Does it have a native PG handler?
- Does it fall back to CLI?
- Does the fallback re-enter dispatch?
- Is this path test-only or production-allowed?
- Does the web service read SQLite-shaped tables or daemon data?
- Does the Go daemon implement this route or return a not-implemented placeholder?

That is the primary risk I would address first.

### 2. The state transition engine is duplicated across substrates and languages

Striatum has the same domain behavior expressed in multiple places:

- SQLite legacy Python path.
- Python PG handler path.
- Go PG handler path.
- Read model shaping in CLI and web service.
- Mapping tables in CLI route translation, RPC registry, daemon server, Go server, MCP tools, chat tools, docs, and tests.

Duplication is not inherently bad during migration. But this codebase has many invariants that must match exactly: lease ownership, job states, event emission, audit chains, artifact validation, expected artifacts, review gates, branch state, run completion, and recovery behavior. Duplicating those rules multiplies risk.

The project recognizes this with parity tests, but parity tests are a compensating control. The architectural goal should be one transition engine with generated/adapted clients.

### 3. The CLI dispatcher is doing too much

`src/striatum/cli/dispatch.py` is a large integration hub. It owns bootstrap, installer behavior, workflow authoring, daemon routing, legacy SQLite dispatch, local service start, dashboard behavior, publish defaults, special override flows, byline, inbox, recovery, and more.

That makes every new surface tempting to add as another branch. It also makes it harder to enforce D104's "daemon MCP mandatory for operator-driven runs" because the CLI remains both a client and a fallback application service.

I would reduce the CLI to:

- Bootstrap/admin commands that genuinely cannot call the daemon.
- A generated thin client for daemon RPC methods.
- Local file authoring helpers that are clearly non-control-plane.

Everything else should be daemon RPC first.

### 4. The local web service is not yet aligned with daemon-first

The web UI has grown into a substantial product surface. The server-rendered Jinja plus React-island approach is reasonable for a local operator UI. But `src/striatum/service.py` is too large and too coupled to the old state shape.

It should become a daemon API client, not a parallel web application service over repo-local state. The current code still contains direct SQLite assumptions and a local `api.invoke` model. That conflicts with D104's daemon MCP/control-plane direction.

A daemon-first web UI would:

- Read from daemon RPC read methods or a daemon HTTP endpoint.
- Mutate only through daemon RPC/MCP-equivalent methods.
- Avoid direct SQLite health checks.
- Share DTO schemas with CLI and MCP.
- Treat the browser UI as a client, not a privileged control plane.

### 5. The Go daemon is strategically sensible but operationally incomplete

The Go core makes sense for long-running supervision, signals, PTY work, packaging, and single-binary distribution. It already has Postgres connection/migrations/audit and some read/mutation parity.

The risk is that the project now has two daemon cores before the authority-boundary migration is fully complete. The Go daemon registers many methods, but not all are implemented. `go/cmd/striatumd/main.go` explicitly fills gaps with not-implemented handlers.

I would avoid expanding product surface until the daemon core strategy is settled:

- Either finish Go as the primary daemon and freeze Python daemon growth.
- Or keep Python as primary and use Go only for a narrow supervisor sub-process.

Running both as peers is expensive.

### 6. Postgres is the right substrate but a hard adoption tax

System Postgres is architecturally appropriate for multi-repo, transactions, audit chains, and concurrency. But for day-zero users, "install Postgres, provision roles, run migrations, start daemon, register repo" is a lot.

The current runbook is careful, but the product wants AI operators to drive routine work. The install story should be as boring as possible.

I would invest in:

- `striatum daemon doctor --provision-local-dev` for common local setups.
- A managed local dev profile using a bundled Postgres, Docker Compose, or embedded distribution.
- A clear production profile that keeps system Postgres.
- Better error recovery when role grants or append-only permissions are wrong.

Without that, Striatum risks being admired by power users but bounced by first-time users.

### 7. Workflow schema and method registry need one source of truth

The project has many structured contracts:

- Workflow JSON.
- Work packet JSON.
- Artifact front matter.
- Daemon RPC envelope.
- Method registry.
- CLI parser arguments.
- MCP tools.
- Web DTOs.
- Go and Python handler params.

Some of these are hand-coded in multiple locations. Over time, this creates drift. For example, the method registry says many methods exist, the CLI route translator maps many of them, Python PG implements a subset, Go implements a different subset, and docs present a curated version.

I would define schemas once and generate:

- Python dataclasses or Pydantic-like validators.
- Go structs.
- CLI argument translators.
- MCP tool definitions.
- OpenAPI/JSON Schema for web clients.
- Docs tables.
- Compatibility/parity fixtures.

The project does not need a huge framework, but it does need contract generation.

### 8. The process/supervisor contract is still brittle

The supervised process model is practical, but the line-delimited packet-over-stdin contract only works for agents that cooperate. Many agent CLIs want a TTY, interactive permission flow, or a vendor-specific session protocol.

The docs already identify the lack of a PTY path. I agree this is a major operational gap. The current wrappers work, but wrappers are brittle integration points.

I would make process supervision a first-class subsystem:

- PTY support by default for interactive CLIs.
- Stable packet delivery with explicit ack from wrapper to daemon.
- Wrapper health protocol separate from agent output.
- Reattach/recover semantics.
- Capability attestation tied to daemon-owned supervisor identity.
- Clean separation between "agent stdout" and "wrapper control channel."

### 9. Security posture is good for single-user local, but some names overclaim

The docs mostly avoid overclaiming. But features like sealed apply, lane attestation, and audit chains can sound stronger than they are. The code and docs correctly say local filesystem writers and operators are not adversaries. Keep that honesty.

I would keep using terms like "guardrail", "metadata audit", and "lane-liveness attestation." I would avoid any language that sounds like model authorship proof, cryptographic non-repudiation, or malicious-operator resistance unless the system later gets real containment.

### 10. The UI build artifact story needs cleanup

The committed `src/striatum/web/static/build/` directory currently contains 285 tracked files and about 9.9 MB of built assets, including many hashed `island-shared-*` chunks. The manifest has 284 lines.

Shipping bundled UI assets in the wheel is reasonable. Accumulating stale hashed chunks is not. The build output should be cleaned before each build, or the bundler should emit a stable small set of chunks. Otherwise package size, review noise, and supply-chain audit surface grow unnecessarily.

## What I would do differently greenfield

### Greenfield architecture shape

I would build Striatum around a single daemon-owned domain kernel from day one:

```text
striatumd
  domain/
    run lifecycle
    job queue and leases
    artifact publication
    review gates and verdicts
    recovery policies
    event/audit append
  adapters/
    postgres
    process/pty supervisor
    git/worktree
    artifact filesystem
    MCP
    CLI client
    web API
  generated contracts/
    JSON Schema
    Go/Python types
    MCP tool descriptors
    CLI help tables
```

Key greenfield decisions:

- **Daemon first, no SQLite transition path.** SQLite can exist only as a fixture format or import source.
- **One implementation language for the domain kernel.** I would pick Go if daemon supervision, PTY, packaging, and long-running operation are primary. I would pick Python only if rapid domain iteration outweighs operational daemon concerns. I would not run Python and Go as equal primary cores long term.
- **Postgres from day one, with a local easy mode.** Use system Postgres for serious installs, but ship a low-friction local profile.
- **Append-only event log plus relational projections.** Keep relational tables for operational queries, but make transitions append domain events consistently and generate read models from those events where practical.
- **Generated client surfaces.** CLI, MCP, and web should be clients of the same method registry and schemas, not separate hand-maintained interpretations.
- **MCP as a first-class agent interface.** Given D104, make daemon MCP the normal operator/agent tool surface. Keep CLI as human/debug/admin shell and a generated RPC client.
- **PTY-first supervision.** Treat plain stdin as a fast path, not the only process contract.
- **Artifacts as immutable content-addressed records.** Store artifact metadata in Postgres and content in repo paths, but consider optional content-addressed copies for audit/replay where privacy policy allows.

### Greenfield state model

I would retain most aggregate roots:

- Repository
- Workflow snapshot
- Run
- Session
- Job
- Queue message
- Lease
- Work packet
- Artifact
- Verdict
- Blocker/escalation
- Decision
- Supervisor
- Apply receipt
- Cross-repo run

I would make a few changes:

- Rename `human_checkpoint` and `waiting_human` now, before more code depends on them. Use `escalation_checkpoint` and `waiting_escalation` or similar.
- Separate `Blocker` from `Escalation`. A blocker is any impediment; an escalation is a request for human-principal authority.
- Separate `ArtifactDeclared` from `ArtifactPublished`. Expected artifacts are workflow contract; published artifacts are observed facts.
- Treat `Review` as a first-class aggregate or sub-aggregate, not just a job plus verdict. Review has posture, independence policy, inspected scope, finding artifact, verdict, supersession, and override history.
- Make `Override` explicit. Operator overrides should be their own records linked to verdicts/jobs, not just newer verdicts with rationale.
- Move `Decision` out of "artifact kind plus event" into first-class state with artifact body attached.

### Greenfield API surface

I would make the method registry the contract:

```text
daemon.hello
daemon.describe
repo.add / repo.remove / repo.list
workflow.validate / workflow.generate / workflow.snapshot
run.prepare / run.start / run.pause / run.resume / run.cancel
session.register / session.close
work.claim / work.ack / work.heartbeat / work.release / work.complete / work.block
artifact.publish / artifact.show
review.submit / review.override
decision.record
recovery.inspect / recovery.apply
supervisor.start / supervisor.send / supervisor.stop / supervisor.status
```

Then generate:

- CLI subcommands.
- MCP tool definitions.
- Web forms/actions.
- Docs reference pages.
- Go/Python client stubs.

This would reduce the current mapping sprawl.

### Greenfield repository layout

I would keep Striatum's recommended consumer layout, but for Striatum itself I would separate product source from dogfood history more aggressively:

```text
src/striatum/       # product
go/                 # daemon if Go remains
contracts/          # schemas, method registry, generated fixtures
web/                # frontend source, build output generated into package area
tests/
docs/
  product/          # user docs
  architecture/     # DDD, SPEC, decisions
  rfcs/
  dogfood/          # archived run records
```

The current repo is rich but cognitively heavy. A clearer docs taxonomy would help new contributors distinguish current contract from historical dogfood evidence.

## What I would change in this codebase now

### Priority 1: Collapse the authority boundary

Finish the daemon-required migration in code, not just docs.

Specific work:

- Delete or quarantine the legacy SQLite transition engine once all production verbs have native PG handlers.
- Remove `CLI_ROUTES` fallback from daemon RPC for production routes.
- Keep SQLite code only under `legacy_import/`, `fixtures/`, or `migration/`.
- Make `STRIATUM_TEST_HARNESS=1` paths impossible to use accidentally outside tests.
- Convert `striatum.api.invoke` into a generated RPC client or mark it test/bootstrap-only.
- Make the local web service use daemon RPC for all state reads and writes.

Exit criterion: a production command never reaches `src/striatum/db.py`.

### Priority 2: Generate the method/API contracts

Create a small `contracts/` source of truth for:

- Method name.
- Required capability.
- Repository scope.
- Request schema.
- Response schema.
- CLI mapping.
- MCP exposure.
- Handler implementation status per core.

Use it to generate:

- `daemon_rpc/registry.py`
- Go method registry
- CLI translator lookup
- MCP tool descriptors
- docs/CLI_REFERENCE excerpts
- parity fixtures

This will pay for itself quickly.

### Priority 3: Break up the large modules

Targets:

- `src/striatum/service.py` into routing, auth/security, DTO shaping, templates, chat, file views, SSE, and workflow editor modules.
- `src/striatum/cli/dispatch.py` into generated RPC client dispatch plus bootstrap/admin modules.
- `src/striatum/workflow.py` into parser, validator, graph, planner, phase validation, lane validation, and generator contracts.
- `src/striatum/daemon_pg/handlers/context.py` into transaction/event append, packet building, artifact validation, lease helpers, and run completion.

The goal is not abstraction for its own sake. The goal is to make invariants easy to find and make ownership boundaries real.

### Priority 4: Decide the daemon core strategy

Pick one:

- **Go-primary daemon.** Freeze Python daemon feature growth, finish Go handlers, move supervision to Go, make Python CLI a client.
- **Python-primary daemon with Go supervisor helper.** Keep domain behavior in Python and use Go only where Python is weak: PTY/process supervision and packaged helpers.

The current "both cores implement the product" direction is expensive.

My bias: make Go the primary daemon if the product is serious about daemon-owned supervision, PTY, packaging, and cross-repo long-running operation. Keep Python for CLI, workflow generation, migration tooling, and tests if desired.

### Priority 5: Fix the operator onboarding tax

Add first-class local setup commands:

- `striatum daemon doctor --provision-local-role`
- `striatum daemon doctor --repair-grants`
- `striatum daemon install-service`
- `striatum daemon start --dev-postgres` or a documented Docker/local bundled option
- `striatum init --register --migrate` as one guided path

The current docs are careful, but a new operator should not have to understand role grants and append-only table privileges before the first run.

### Priority 6: Make supervision robust

Build the PTY path and wrapper protocol:

- PTY-backed sessions.
- Daemon-owned supervisor lifecycle.
- Packet delivery ack from wrapper.
- Health checks independent of model output.
- Supervisor event stream.
- Reattach after daemon restart.
- Stronger lane-liveness attestation.

The current process adapter is enough for a controlled harness, but robust agent orchestration will depend on this layer.

### Priority 7: Clean the UI bundle pipeline

Make `ui-build` clear `src/striatum/web/static/build` before writing new assets. Keep stable entrypoint names or a manifest-driven loader, but do not retain old hashed chunks.

Also move `@vitejs/plugin-react` to `devDependencies`; it is currently under `dependencies` in `src/striatum/web/frontend/package.json`.

## Functionality I would add

### 1. First-class escalation inbox

Docs now tell the human principal to check `striatum inbox`, but the current CLI `inbox` helper is session-scoped and packet-focused. That is useful for operator-on-behalf publication, not a principal escalation inbox.

Add a real escalation inbox:

```text
striatum inbox --run-id <id>
striatum inbox --all
striatum escalation list
striatum escalation resolve --id <id> --decision <...>
```

Back it with structured escalation records and artifacts. This aligns with RFC 0053.

### 2. Review quorum and committee workflow

The project has lived through same-model reviewer co-blindness. The next step should be first-class quorum semantics:

- Required reviewer diversity rules.
- Same-model pairing rejection by default.
- Quorum policies: unanimous, majority, weighted, arbitrated.
- Committee stalemate escalation.
- Supersession and compromised-run propagation.

This is a natural extension of the current review-posture work.

### 3. Auto-finalize from front matter

RFC 0051 is worth implementing. If an agent writes the expected artifact with valid front matter and byline, the runner should be able to publish, record verdict, and complete safely after a grace period.

This directly reduces operator-on-behalf toil.

### 4. Run replay and simulation

Add a replay/simulator mode:

- Feed an event log or exported corpus bundle.
- Rebuild run state.
- Verify audit/event chains.
- Show what transition would happen for a proposed command.
- Test workflows without live agent processes.

This would strengthen migration confidence and make demos easier.

### 5. Workflow lint with operational risk scoring

The validator checks structural correctness. Add a higher-level linter:

- Same-model author/reviewer pairing.
- Missing fresh review policy.
- Unattested review jobs.
- Repo-write jobs without worktree isolation.
- Broad write scopes.
- Missing support ledgers for high-risk artifacts.
- Review cycles without resolution policy.
- No escalation path.

Return warnings and a score before `run prepare`.

### 6. Artifact manifest and archive

At run close, produce a deterministic artifact manifest:

- Every published artifact path, kind, hash, author line, job, session, verdict.
- Event chain head.
- Audit request ids.
- Redaction policy version.
- Optional tar/zip archive with artifacts only, no transcripts.

This complements `corpus export` and makes external audit easier.

### 7. Service-manager integration

Add supported install/start/stop/status commands for systemd user units and macOS launchd. The docs already mention both. Make them product features.

### 8. Target-repo adoption wizard

Use the workflow generator and consumer layout recommendations to implement:

```text
striatum adopt
```

It would inspect a repo, suggest workflow shapes, scaffold docs/workflow directories, register/migrate the repo, install skills/plugins, and produce a "next command" checklist.

### 9. Optional Git/PR integration

Keep automatic commits out of the core default, but add explicit gated verbs:

- `git.status.snapshot`
- `git.commit.request`
- `git.commit.apply --confirmed`
- `github.pr.prepare` as optional adapter/plugin

For code-change workflows, the handoff from artifact provenance to source diff needs a clean endgame.

### 10. Better corpus/memory boundary tooling

The augmentation-not-dependency boundary is right. Add tooling around it:

- Corpus contract v2 validation.
- Incremental export watermarks.
- Multi-corpus identity.
- Redaction tier metadata.
- Optional context injection policy that is explicitly workflow-authored.

Keep retrieval outside Striatum's transition path.

## Documentation issues to fix

The docs are strong but some are stale or inconsistent because the architecture moved quickly.

Examples:

- `docs/MCP.md` still describes `.striatum/state.sqlite3` as state authority for the local wrapper.
- `docs/WRITING_WORKFLOWS.md` still labels `.striatum/` as runtime state including SQLite.
- `docs/USING_STRIATUM.md` says `striatum inbox --json`, but the CLI parser requires `--session-id`.
- `docs/SPEC.md` has sections that still refer to dashboard and service views over SQLite, even though the product boundary says Postgres is authoritative.

I would do a docs sweep with this rule:

- Current contract docs must describe daemon/Postgres/MCP as normal.
- Historical SQLite content must be explicitly marked historical or migration-only.
- Bootstrap/test-harness paths must not appear as normal operator paths.
- CLI examples must be executable as written.

## Risk register

### High: transition/fallback split-brain

The most serious risk is that a production path accidentally falls through to legacy behavior or that two cores diverge. Mitigation: remove fallback paths, generate contracts, and test that production commands cannot open SQLite.

### High: duplicated state transitions

Python SQLite, Python PG, and Go PG implementations of the same behavior will drift. Mitigation: one domain kernel or generated transition specs; parity tests only as backstop.

### Medium: onboarding friction

Postgres role/grant setup is a barrier. Mitigation: provisioning tools and bundled/dev substrate option.

### Medium: supervision fragility

Wrappers and stdin pipes are brittle for real interactive agent CLIs. Mitigation: PTY, wrapper control protocol, daemon-owned supervision.

### Medium: UI/service coupling

The web service can become a second app server rather than a client of the daemon. Mitigation: route through daemon RPC and shrink `service.py`.

### Medium: artifact/provenance overclaim

Lane liveness and audit chains can be misunderstood as authorship proof. Mitigation: keep terminology honest and expose provenance modes clearly.

### Low-to-medium: package bloat

Tracked UI build artifacts have accumulated. Mitigation: clean build output and manifest check.

## Suggested 90-day architecture plan

### Month 1: authority boundary cleanup

- Inventory every CLI verb and classify it: daemon-native, bootstrap/admin, workflow-authoring local file helper, test-only, deprecated.
- Add a test that production-mode commands cannot call `striatum.db.connect`.
- Port or remove remaining fallback routes.
- Convert local service reads/writes to daemon RPC for core run state.
- Fix docs that still advertise SQLite as live state.

### Month 2: contract generation and core split decision

- Introduce a single method contract source file.
- Generate Python and Go registry tables and CLI/MCP docs from it.
- Decide Go-primary vs Python-primary daemon.
- If Go-primary, create a deprecation plan for Python daemon routes.
- If Python-primary, restrict Go to supervisor helper and stop duplicating domain transitions.

### Month 3: operator experience and supervision

- Add local Postgres provisioning/repair commands.
- Add service-manager install/start helpers.
- Implement real escalation inbox.
- Build PTY-backed supervision prototype.
- Implement same-model reviewer warning/rejection and auto-finalize from front matter.

## Final judgment

Striatum is architecturally promising because it models the hard part: coordinated, auditable work across unreliable agents and reviewers. The current system is much more rigorous than most agent orchestration projects. Its weakness is not lack of ideas; it is that the implementation now contains too many eras at once.

The product should narrow its runtime spine:

```text
daemon RPC/MCP -> one transition engine -> Postgres -> durable artifacts
```

Everything else should be a client, fixture, migration source, or historical artifact. If the project makes that cut, the rest of the roadmap becomes easier: review quorum, stronger supervision, auto-finalize, corpus v2, and UI improvements all fit cleanly. If it keeps carrying SQLite, Python daemon, Go daemon, CLI fallback, local service fallback, and hand-maintained route maps as peer architecture, correctness work will increasingly become parity work.

My highest-confidence recommendation: finish the daemon authority migration before adding major new product surface.
