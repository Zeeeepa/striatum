# Changelog

## Unreleased — 2026-05-18

### Architecture remediation follow-through

The remediation plan from the 2026-05-16 architecture review is now tracked
in the roadmap/TODO and has several production slices landed:

Current behavior is summarized in this section. Older release entries below
remain historical notes for the behavior that shipped at that tag.

Recent checkpoints:

- D107 supersedes D105: Go is now the production/default daemon, active
  contract-method parity is landed, D111 retires the Python daemon selector,
  and the retired Python daemon module is deleted. Python CLI/web clients
  remain useful, while SQLite is retired from production and operator
  compatibility paths. RFC 0068 records the port; RFC 0069-0071 cover
  daemon-global PG, client-boundary, and diagnostic follow-ups.
- Stale decision/RFC wording now reflects the Go/PostgreSQL runtime boundary:
  durable artifact provenance, evidence identity, worktree state, dogfood
  composite tooling, and packaging notes no longer imply a current Python
  daemon or repo-local SQLite authority.
- `import striatum.cli` no longer eagerly imports SQLite-backed legacy
  evidence/introspection/list/mutation/recovery/run-summary/worktree modules;
  historical package-level re-exports now resolve lazily when callers request
  a specific compatibility symbol.
- Evidence redaction policy and Markdown rendering now live in a
  substrate-neutral presenter module. PostgreSQL evidence handlers and corpus
  redaction use that shared code directly instead of importing the legacy
  SQLite-backed CLI evidence reader.
- Run-summary Markdown formatting, duration formatting, and verdict grouping
  now live in a substrate-neutral formatter used by PostgreSQL handlers and
  corpus exports; the SQLite-backed CLI module keeps only its legacy snapshot
  and export wrapper.
- The remaining direct SQLite CLI dispatch block now runs only under the
  paired legacy test-harness escape; production commands that are not
  daemon-routed fail closed before opening repo-local state.
- Importing `striatum.cli.dispatch` no longer eagerly imports `sqlite3`,
  `striatum.db`, legacy workflow/artifact helpers, or SQLite-backed CLI
  reader/mutation modules; fixture-only imports are loaded only after the
  paired legacy test-harness gate.
- The deterministic `next_actions` projection moved into a substrate-neutral
  module. PostgreSQL read-model status no longer imports the SQLite-backed
  CLI introspection module for that helper.
- `current_git_branch` moved into a substrate-neutral Git helper so
  PostgreSQL run-summary and branch-confirm handlers no longer import the
  SQLite-backed CLI mutation module for Git branch inspection.
- Artifact-kind constants, front-matter validation, and Markdown byline
  parsing moved into `striatum.artifact_contracts`; PostgreSQL artifact
  publish/recovery handlers no longer import the SQLite-backed legacy
  `striatum.artifacts` module for neutral contract helpers.
- Daemon PostgreSQL handler registration no longer eagerly imports
  `striatum.artifacts` or `striatum.workflow`; architecture guardrails now
  cover those legacy module boundaries in addition to `sqlite3`,
  `striatum.db`, and SQLite-backed CLI readers.
- The legacy repo-local SQLite artifact publisher moved to
  `striatum.legacy_sqlite.artifacts`; `striatum.artifacts` is now a neutral
  compatibility facade that imports the legacy publisher only when callers
  invoke legacy publish/byline helpers.
- Legacy repo-local SQLite workflow live-state helpers (`create_run` and
  `compute_node_states`) moved to `striatum.legacy_sqlite.workflow`;
  `striatum.workflow` now keeps validation, graph, and planning helpers
  separate from the fixture-only run-state implementation.
- The legacy repo-local SQLite autonomous recovery sweep moved to
  `striatum.legacy_sqlite.recovery_auto`; `striatum.recovery.auto` now
  exposes only lazy compatibility wrappers for that retired path.
- Process-adapter diagnostic envelope and recovery-command helpers remain in
  neutral `striatum.process_completion`; SQLite output validation and blocker
  insertion moved to `striatum.legacy_sqlite.process_completion` and now load
  lazily for legacy adapter paths.
- The legacy repo-local SQLite process adapter moved to
  `striatum.legacy_sqlite.process_adapter`; `striatum.process_adapter` now
  keeps neutral env expansion/schema constants and lazy wrappers for legacy
  adapter calls.
- The legacy repo-local SQLite supervisor helper moved to
  `striatum.legacy_sqlite.supervisor`; `striatum.supervisor` now exposes only
  the active-state constant and lazy wrappers for legacy supervise calls.
- The SQLite-bound dogfood operator composites moved to
  `striatum.legacy_sqlite.dogfood_operator_tools`; `striatum.dogfood` and
  `striatum.dogfood.operator_tools` now import without loading SQLite.
- The legacy SQLite worktree CLI helpers moved to
  `striatum.legacy_sqlite.cli_worktree`; `striatum.cli.worktree` now exposes
  lazy wrappers for compatibility callers.
- The legacy SQLite evidence CLI reader/exporter moved to
  `striatum.legacy_sqlite.cli_evidence`; `striatum.cli.evidence` now exposes
  lazy wrappers for compatibility callers.
- The legacy SQLite run-summary CLI reader/exporter moved to
  `striatum.legacy_sqlite.cli_run_summary`; `striatum.cli.run_summary` now
  exposes lazy wrappers for compatibility callers.
- The legacy SQLite list CLI readers moved to
  `striatum.legacy_sqlite.cli_list_commands`; `striatum.cli.list_commands`
  now keeps neutral filter constants and lazy wrappers for compatibility
  callers.
- Product docs now describe the Python daemon module/selector as deleted,
  with remaining cleanup limited to legacy SQLite fixture/import conversion
  or deletion.
- Service/web legacy SQLite fallback now requires the explicit
  `STRIATUM_LEGACY_SERVICE_FIXTURE=1` marker in addition to the paired
  test-harness daemon opt-out; broad pytest daemon opt-out alone no longer
  disables daemon RPC routing for service calls.
- Legacy SQLite status/why/doctor introspection helpers moved to
  `striatum.legacy_sqlite.cli_introspect`; `striatum.cli.introspect` now
  exposes neutral constants and lazy compatibility accessors.
- Legacy SQLite recovery mutation helpers moved to
  `striatum.legacy_sqlite.cli_recovery`; `striatum.cli.recovery` now keeps
  parity constants and lazy compatibility accessors.
- Legacy SQLite workflow-loop mutation helpers moved to
  `striatum.legacy_sqlite.cli_mutations`; `striatum.cli.mutations` now keeps
  the neutral verdict-job constant and lazy compatibility accessors.
- Legacy SQLite DB imports used by the paired test-harness CLI dispatch path
  moved behind `striatum.legacy_sqlite.cli_dispatch_db`; importing
  `striatum.cli.dispatch` no longer imports `sqlite3` or `striatum.db`.
- The retired `src/striatum/daemon.py` Python daemon / daemon-global SQLite
  registry module was deleted. Architecture guardrails now assert the module
  remains absent and keep daemon-global refusal coverage on the PostgreSQL
  admin helper surface.
- The Go daemon launch contract now reports supported daemon PostgreSQL schema
  and migration count from `--describe`; the Python launcher refuses stale Go
  daemon binaries before socket bind when their schema, migration count, or
  method contract does not match the source tree.
- Go daemon builds now stamp `striatumd --describe` with the Python package
  version, git SHA, and dirty/clean state. The Python launcher also rejects
  unstamped `go-dev` binaries and binaries that omit git provenance before
  they can bind a socket.
- `doctor --first-run` now returns a single V1 diagnostic report that combines
  day-zero smoke checks, Go daemon binary provenance, and the daemon authority
  report so operators can validate the local stack with one command.
- The local stdio MCP compatibility wrapper no longer advertises or executes
  CLI-shaped aliases through `tools/list` / `tools/call`. Production MCP tool
  discovery stays on the daemon registry surface, while `striatum/invoke` and
  read resources remain available only as compatibility/manual paths.
- Active operator docs now frame legacy SQLite handling as archive/remove plus
  repository registration, not a current per-repo migration workflow; smoke
  scripts also stopped exporting the legacy daemon SQLite registry path.
- The retired daemon-global SQLite registry cutover implementation
  (`striatum.daemon_pg.cutover`) was deleted; compatibility tests now assert
  the old `daemon migrate --from sqlite --to pg` spelling still refuses before
  importing any cutover code.
- The command authority matrix now names the bounded direct-PostgreSQL
  bootstrap/admin plane, and an architecture guardrail scans Python client/CLI
  sources so new direct daemon-PG helper imports must be explicitly listed.
- The Go cross-repo runner boundary was narrowed from one speculative
  `LocalRunner` interface to per-operation prepare/start/cancel interfaces,
  removing the unused `ParticipantIntact` and `HumanCheckpoint` hooks and the
  Go daemon's placeholder `Prepare` method.
- Go migration SHA-source verification now rejects extra newer Python-source
  migrations, closing the stale-binary gap where an old Go binary could pass
  hash checks until it hit a migrated database.
- Fresh Go daemon startup now bootstraps the first PostgreSQL admin client
  and writes the private runtime `client-token`, matching the legacy Python
  daemon's first-start auth contract without requiring that daemon core.
- The Go daemon now starts a resident recovery scheduler after socket bind.
  It runs an immediate PostgreSQL active-run sweep, calls the Go
  `recovery.sweep` path, records `daemon.recovery_sweep`, updates
  `striatumd.scheduler_cursors`, and accepts `--sweep-interval-seconds` plus
  bounded-test `--max-sweeps` flags through the Python launcher.
- `make daemon-go-conformance` is now the Go production-daemon CI gate. It
  builds and tests the Go daemon, then runs the multi-repo harness with
  `CORE=go`, including Go daemon smoke, audit, mutation-registry, and
  supervisor smoke coverage.
- Go `daemon.shutdown` now wires through the daemon process cancellation path
  and returns an accepted shutdown response instead of the previous
  fail-closed `shutdown_unavailable` placeholder.
- Go `doctor` now reads `striatumd.schema_meta['substrate_version']` instead
  of querying a nonexistent `schema_meta.version` column.
- Daemon RPC handshakes from the CLI and day-zero first-run smoke now use
  `striatum.__version__` instead of hardcoded client versions.
- The Go daemon now has an executable handler-coverage ledger for missing
  and placeholder methods, and `recovery.sweep` is registered on the Go
  mutation surface instead of only the deprecated `recovery.auto` alias.
- D110 removes the SQLite-bound `daemon.migrate_repo_local`,
  `dogfood.publish_on_behalf`, and `dogfood.surgical_recovery` RPC methods
  from the production daemon contract and MCP discovery. Unknown calls now
  audit as `method_unknown`.
- D112 removes `apply.reviewed_patch` from the production daemon RPC contract
  instead of carrying it as a fail-closed RFC 0068 retirement blocker. Stale
  direct calls now return and audit as `method_unknown`; apply receipt reads
  and daemon signing-key rotation remain supported.
- SQLite import windows are now closed for production/operator paths.
  `striatum daemon migrate` and `striatum daemon migrate-repo-local` remain
  parser-compatible compatibility spellings, but they refuse with exit code
  12 before importing or opening SQLite migration code. Direct
  `migrate_repo_local()` use is guarded behind the explicit
  `STRIATUM_LEGACY_SQLITE_IMPORT=1` fixture escape; `adopt`, repo
  registration, and repo-not-migrated hints now point operators to archive or
  remove legacy SQLite files and register with `adopt` / `repo add --init`.
- Daemon MCP `resources/list` and `resources/read` now require an explicit
  daemon PostgreSQL connection. The no-`pg_conn` legacy SQLite registry
  fallback is retired, and the corresponding Python-daemon MCP resource
  helpers were removed.
- `striatum.api` no longer imports `sqlite3` or `striatum.db`; it uses the
  shared primitives types and leaves SQLite-era failures outside the local API
  compatibility wrapper.
- `workflow upgrade` and `workflow upgrade --add-phases` now use only the
  daemon PostgreSQL running-run guard. The repo-local SQLite fallback and its
  paired test-harness escape were removed from the workflow-upgrade path.
- Corpus manifest construction no longer accepts or fakes a SQLite
  connection. Manifests now carry explicit `state_authority` metadata, and the
  PostgreSQL `corpus.export` handler reads daemon/repository schema metadata
  directly instead of emulating `PRAGMA user_version`.
- Production daemon CLI/admin dispatch now imports the PostgreSQL-only
  `striatum.daemon_pg.client_admin` surface instead of the legacy Python daemon
  module. The CLI-side legacy daemon registry wrapper and its direct
  `--daemon`/`dashboard --all` SQLite fallback paths are removed; remaining
  `striatum.daemon` imports are explicit legacy migration/test fixtures
  (D117).
- Legacy daemon security fixture coverage was narrowed again: runtime token and
  daemon MCP denial checks now exercise `daemon_runtime`, `daemon_pg.client_admin`,
  and daemon RPC capability helpers directly, leaving the Python daemon import
  to cutover/quarantine fixtures.
- The multi-repo Go daemon test harness no longer imports the legacy Python
  daemon module for runtime environment constant names; it uses
  `daemon_runtime` and the PostgreSQL admin client surface directly.
- CLI daemon-route parsing, workflow scaffolding, skill/plugin installers,
  daemon supervisor helper imports, and several tests now import neutral
  primitives/path-policy helpers directly instead of loading them through
  `striatum.db`; the SQLite quarantine allowlist shrank accordingly.
- Legacy SQLite cutover guardrail tests now live with the legacy quarantine
  tripwires, so `tests/test_daemon_pg.py` no longer imports the retired Python
  daemon module.
- Mixed legacy modules now import neutral JSON/id/time/path helpers directly
  from `striatum.primitives` and `striatum.repo_policy`; an architecture
  guardrail prevents new `striatum.db` imports of substrate-neutral helpers.
- Runtime path and token-file helpers now live in `striatum.daemon_runtime`,
  and PostgreSQL repository registration helpers used by day-zero setup and
  daemon RPC routing now live in `striatum.daemon_pg.repositories`, reducing
  Python CLI/client imports of the legacy Python daemon module.
- The unused repo-local SQLite supervisor pointer helper
  (`striatum.daemon_supervisor.pointer`) was deleted; current supervisor
  pointer writes live under the daemon/PostgreSQL handlers.
- `striatum.daemon_supervisor.progress_watcher` no longer imports `sqlite3`;
  its optional connection is typed generically while the caller owns the
  legacy repo-local connection.
- Legacy corpus export helpers no longer import `sqlite3` or `striatum.db`;
  their caller supplies the connection while corpus-specific row lookup stays
  local to the compatibility exporter.
- Shared identity helpers no longer import `sqlite3`; the legacy
  session-lane attestation path accepts a generic row-capable connection while
  PostgreSQL code keeps using the substrate-neutral author/process helpers.
- The legacy `striatum.process_progress` SQLite wrapper was deleted; the
  retired Python-daemon sweep path no longer invokes repo-local supervised
  progress reconciliation, while shared progress-watcher coverage remains.
- The legacy SQLite `recovery watch` loop was deleted. The CLI-local watcher
  always runs the daemon-backed scheduler over `recovery.sweep`, including
  paired test-harness invocations.
- The dead legacy SQLite view-file breadcrumb reader was removed from the
  service fallback quarantine; `/view/...` no longer has a repo-local
  SQLite breadcrumb escape.
- Unused legacy Python-daemon `read_status` and `read_why` registry readers
  were deleted; status/why reads are owned by daemon RPC and PostgreSQL paths.
- The unused legacy Python-daemon `dashboard_all` repo-local fallback was
  deleted; daemon-global dashboard reads stay on the PostgreSQL client/admin
  and Go daemon paths.
- The legacy service artifact-row wrapper was removed; the remaining SQLite
  service fallback uses the shared web artifact row shaper directly.
- The web doctor page no longer has a legacy SQLite fallback; daemon doctor
  DTO errors fail closed as HTTP-shaped doctor page errors.
- The unused SQLite registry audit-segment rotation test helper was deleted
  from the legacy Python daemon module.
- Duplicate repo add/list/remove helpers and their legacy SQLite registry
  fallbacks were deleted from the legacy Python daemon module; repo
  registration now lives only on the PostgreSQL admin/repository helpers.
- Legacy Python-daemon global entry points (`status`, `stop`, `health`,
  `audit`, `sweep`, `doctor`, and foreground startup) no longer open the
  SQLite daemon registry; without a PostgreSQL daemon URL they fail closed
  before touching registry files.
- The now-unreachable SQLite daemon auth/audit/doctor helper island was
  removed from the legacy Python daemon module after the global fallbacks
  moved to PostgreSQL-only behavior.
- The obsolete standalone legacy-registry opt-in environment variable is no
  longer exported by daemon helper modules or surfaced in authority doctor
  diagnostics.
- The standalone `striatum.daemon_pg.sqlite_compat` helper was removed. Its
  last repository-identity calculation now lives beside the one-way
  repo-local migration fixture, and the unused daemon audit-chain validators
  are gone.
- D111 retires the operator-facing Python daemon selector. `striatum daemon
  start` always launches the Go daemon; `--core go` remains a deprecated
  no-op compatibility flag, while `--core python` and
  `STRIATUM_DAEMON_CORE=python` no longer select a Python daemon.
- The `striatumd` console script now targets a small Go-daemon launcher shim
  instead of importing the legacy Python daemon module; the old
  `striatumd --foreground` spelling is accepted as a compatibility alias.
- The multi-repo test harness no longer initializes participant repositories
  with repo-local SQLite. Participant prepare/start/cancel/checkpoint
  assertions now use daemon-owned PostgreSQL rows under `striatumd.*`.
- Packaged wheels now stage the Go daemon binary before build, and fresh-clone
  smoke builds `go/bin/striatumd` before the default daemon start path.
- Go PostgreSQL mutation paths now encode structured JSONB arguments through
  a shared pgx-safe helper, covering workflow snapshots, job definitions,
  queue messages, work packets, session capabilities, supervisor metadata,
  blockers, recovery cursors, and event payloads.
- The Go RPC envelope validator now matches the published daemon contract by
  accepting non-empty method strings, including contracted undotted reads such
  as `status` and `dashboard`.
- Release metadata checks now source both package name and version from
  `pyproject.toml`, avoiding false failures when an unrelated `striatum`
  distribution is installed beside `striatum-orchestrator`.
- Go now registers the canonical `recovery.auto_publish_stale_artifacts`
  method, keeps the deprecated `recovery.auto` alias on the same handler, and
  requires every auto-published file to match the expected byline.
- Go now owns the `recovery.auto_finalize` RPC handler as a dry-run-by-default
  projection with workflow-opt-in or forced live mode over stable expected
  artifact files.
- The first Go read-detail cluster is registered for `run.detail`,
  `job.detail`, `run.events`, `run.posture_verdicts`, `artifact.show`,
  `escalation.list`, `escalation.show`, and `escalation.resolve`, reducing
  missing contract handlers while keeping remaining web-context parity gaps
  visible.
- Go now owns `archive.create` for the V1 run archive bundle format, including
  safe repo-relative output paths, PostgreSQL run-scoped row export, and
  deterministic manifest/file hashes.
- Go `evidence.export` now writes the Markdown evidence file under the target
  repository and uses current PostgreSQL artifact/verdict column aliases; the
  same alias fix covers Go `run.summary` and `corpus.export`.
- Go now owns the read-only `worktree.list` handler over PostgreSQL
  `job_worktrees`, returning the Python-compatible `worktrees` row list with
  optional run filtering.
- Go now owns `worktree.create` and `worktree.release` over PostgreSQL
  worktree state, with repo-scope/lease/workflow validation, safe
  `.striatum/worktrees/` path confinement, and Git worktree add/remove calls
  performed directly by the Go daemon.
- Go now owns `work.send_message`, inserting completed agent messages and
  appending `message.sent` through the hash-chained PostgreSQL event helper.
- Go now owns `workflow.templates.list` and `workflow.templates.show` from an
  embedded copy of the workflow template catalog, with a drift test against
  the Python package-data catalog.
- Go now owns workflow file-authoring handlers: `workflow.validate`,
  `workflow.plan`, and `workflow.graph` validate repo-local workflow JSON and
  return plan/graph projections without mutating daemon state or opening
  SQLite.
- Go now owns workflow generation handlers: `workflow.generate.preview`
  produces read-only planned writes; `workflow.generate` and `workflow.init`
  write safe repo-relative scaffold files; `workflow.upgrade` uses
  PostgreSQL running-run checks and fails closed when PostgreSQL state is
  unknown, including `--add-phases` rewrites and
  `workflow.generate --shape multi_phase` V1.1 phase graph generation.
- Go `workflow.upgrade --add-phases` now matches the Python V1-to-V1.1
  phase-inference path for preview/apply, synthesis-job insertion,
  cross-phase edge rewriting, and non-terminal-run refusal.
- Web and chat workflow-generation preview now call
  `workflow.generate.preview` through daemon RPC in production, preserving
  the local in-process generator only for the explicit test-harness fallback.
- Production `cross-repo` CLI dispatch now refuses the remaining direct
  PostgreSQL fallback path if daemon RPC routing did not handle the command;
  the direct path is limited to the explicit legacy test-harness escape.
- Go cross-repo lifecycle reads now return typed `not_found` RPC errors for
  missing cross-repo run ids instead of leaking plain internal errors.
- Go daemon socket-level conformance now covers `cross_repo.cancel` against
  a live CORE=go Unix RPC daemon and PostgreSQL state, including mixed
  canceled/blocked participants, audit evidence, and JSONB-safe event payload
  insertion for pgx-backed mutation handlers.
- `striatum init --with-striatum-layout` now scaffolds the RFC 0056
  consumer-repo directories `striatum/workflows/` and
  `striatum/<workflow-slug>/` without writing workflow files or `.gitignore`
  policy. Day-zero docs, agent skill examples, and `adopt`'s suggested
  starter path now use the generated-tree form
  `striatum/workflows/<name>/workflow.json`.
- Go `daemon.key.rotate` now rotates a local Ed25519 sealed-apply signing
  key into the `0600` fallback key file, returns the new key id/public key
  metadata, and `daemon.hello` advertises the current public key when the
  fallback key is loadable. Malformed private fallback files are preserved as
  `.invalid.<timestamp>` backups during rotation; over-permissive key files
  still fail closed. Full apply-gate mutation and OS keyring custody remain
  deferred.
- Go now owns `supervise.status`, `supervise.list`, and
  `supervise.reattach_status` as read-only PostgreSQL projections. The status
  handler reports liveness, lane attestation, and stalled-supervisor fields
  without mutating pointer rows or draining helper events.
- Go now owns `supervise.start`, `supervise.send`, and `supervise.stop` over
  PostgreSQL supervisor rows and FIFO/helper transport. Sends preserve the
  delivered-unacknowledged contract, and stops update terminal supervisor state
  before signaling/removing control paths.
- Go now owns `daemon.migrate`, applying the embedded daemon PostgreSQL
  migrations without Python or SQLite.
- Go now owns daemon token lifecycle handlers:
  `daemon.token.create/revoke/rotate` write only daemon PostgreSQL client and
  capability rows, store HMAC-SHA256 token hashes, and return cleartext bearer
  tokens only at creation/rotation time.
- `apply.reviewed_patch` is no longer a production daemon RPC. The supported
  apply-adjacent surface is receipt read/verify plus daemon key rotation until
  a future sealed-apply decision reintroduces a mutation.
- Go now owns `repo.init` as PostgreSQL-backed repository initialization that
  creates only operational scratch and refuses repo-local SQLite state.
- The Go daemon handler-coverage ledger now reports zero generic
  `not_implemented` handlers for active contract methods; removed unsupported
  method names are expected to audit as `method_unknown`.
- Go now owns `run.graph` for JSON, Mermaid, DOT, and ASCII run graph
  projections from PostgreSQL workflow snapshots, materialized dependencies,
  latest job attempts, and review verdicts.
- Go `cross_repo.cancel` now calls the Go cross-repo lifecycle service and
  local run-cancel mutation instead of returning `not_implemented`, and now
  matches Python participant-cancel parity for terminal skips, preparing
  participants without local runs, inactive participant repositories, and
  persisted `blocked_errors`.
- Go now owns `repo.add`, `repo.list`, and `repo.remove` handlers over
  daemon-owned PostgreSQL, including SQLite-source refusal, operational
  scratch initialization, active-path conflict checks, and repo-scoped
  capability revocation on removal.
- Go now owns daemon-global `repo.resolve`, a read-capability bootstrap method
  that normalizes a repository path and returns active repository metadata
  without requiring CLI/web clients to open daemon PostgreSQL directly.
- The retired Python-daemon compatibility path also handles `repo.resolve`
  through PostgreSQL for legacy fixture coverage; production deployments use
  the Go daemon.
- Python CLI and service repository-scoped RPC routing now resolve repository
  ids through daemon RPC instead of importing daemon PostgreSQL connection
  helpers. Resolution errors fail closed rather than falling back to local
  state.
- Production Python daemon startup no longer opens the legacy SQLite daemon
  registry when PostgreSQL is configured. `connect_registry()` is explicitly
  gated to migration/test compatibility escapes, and startup uses PostgreSQL
  daemon metadata plus PostgreSQL sweep plumbing.
- `/v1/invoke` now sends daemon-mapped production reads and mutations through
  daemon RPC. The local `striatum.api.invoke` path remains available for
  explicit local/test surfaces and workflow authoring, not production run
  authority.
- Local MCP and web chat tools now use the same daemon-routing policy for
  mapped status, why, run lifecycle, artifact, review, and recovery commands;
  `striatum.api.invoke` remains only for unmapped local authoring and explicit
  fixture compatibility.
- Go `run.prepare` now loads workflow files through the Go workflow-authoring
  loader before writing rows, so repo-bound path checks and JSON-only workflow
  source validation are enforced in the Go daemon path.
- The SQLite-bound `dogfood.publish_on_behalf` and
  `dogfood.surgical_recovery` composites are removed from the production
  daemon contract in favor of primitive daemon methods until a
  PostgreSQL-native composite is designed.
- Production daemon MCP `tools/list` now hides local workflow-file authoring
  methods in both Python and Go; direct calls to removed dogfood composites
  now audit as `method_unknown`.
- SQLite registry-probe guardrails now classify every remaining direct
  `striatum.daemon.connect_registry()` caller and runtime-tripwire daemon MCP
  resource reads, so newly introduced daemon-global SQLite probes fail the
  architecture tests before they can become production fallback paths.
- `striatum daemon doctor --authority --json` now emits a cutover authority
  report covering PostgreSQL live-state authority, disabled legacy SQLite
  registry status, daemon method fallback counts, allowed migration/test-only
  SQLite exceptions, and remediation recommendations.
- The repository `/view/<path>` page no longer consults the legacy
  SQLite-backed run breadcrumb helper; file viewing stays a pure repository
  file read with no production SQLite touchpoint.
- Go now owns a read-only `dashboard.all` handler over daemon-owned
  PostgreSQL repositories. It reports per-repository status and stale-lease
  projections without opening SQLite; follow-up parity now also exposes
  per-active-run `run_progress` with phase progress, auto-finalize dry-run
  summary, and stalled-supervisor detail in both Go and Python/PostgreSQL
  dashboard-all projections.
- The compact terminal dashboard now renders single-run text frames from the
  daemon/PostgreSQL `dashboard` DTO in production. The old repo-local SQLite
  payload reader has been deleted, and paired test-harness assertions now use
  renderer fixtures; JSON `dashboard --run-id` and daemon-global
  `dashboard --all` remain RPC DTO surfaces.
- Go `status` now uses the PostgreSQL/Python read-model shape instead of raw
  row dumps: job counts by state, nested verdict counts by posture/verdict,
  queue-based claimable jobs, blocker/checkpoint payloads, run-scoped process
  health, supervisor stalls, phase/provenance fields, auto-finalize dry-run
  visibility, and deterministic `next_actions`.
- RFC 0058 V1.5 landed: `striatum operator current-brief` reads and validates
  the current operator brief without daemon RPC, and `operator_brief`
  `context_budget_lines` overruns are schema errors instead of warnings.
- Daemon diagnostics now fail closed without traceback leakage when the
  runtime PostgreSQL role cannot apply pending migrations, returning a
  structured `daemon status --json` error with the owner/admin repair hint.
  `daemon doctor --postgres-url` also threads that explicit URL into
  secondary daemon diagnostics instead of relying on env/config and risking an
  implicit legacy-registry probe.
- Daemon MCP `resources/list` and `resources/read` now use PostgreSQL-backed
  repository visibility, status, doctor, blocker, run, why, dashboard, and
  stale-lease projections whenever a daemon PostgreSQL connection is present;
  regression coverage runs those paths with the SQLite registry tripwire on.
  If the daemon MCP server is constructed without a PostgreSQL connection,
  resource list/read now fail closed before opening the legacy SQLite registry
  unless the paired legacy test-harness escape is active.
- `striatum daemon audit` now reads and authorizes against PostgreSQL when a
  daemon DB is configured, keeps the legacy audit output field names for CLI
  compatibility, and has SQLite-registry tripwire coverage for direct and
  dispatcher paths.
- `striatum daemon health` now uses PostgreSQL and appends to the PostgreSQL
  audit chain when a daemon DB is configured, avoiding the legacy registry
  probe while preserving the existing health JSON shape.
- `daemon doctor` no longer probes the legacy SQLite registry after a
  successful PostgreSQL doctor check. It reports the SQLite registry as
  post-cutover/unused, carries PostgreSQL-backed daemon diagnostics separately,
  and `read_doctor` uses PostgreSQL for global and repo-scoped diagnostics when
  a daemon DB is configured.
- `striatum daemon status` and `striatum daemon stop` now authorize and audit
  through PostgreSQL when a daemon DB is configured, preserving pidfile/runtime
  lifecycle behavior without opening the legacy registry.
- `supervise.status`, `doctor`, and `status` now surface stalled attached
  supervisors, and recovery sweep opens
  `heartbeat_stall_lease_expired` blockers when stalled leases expire.
- The historical three-lane design/build/review workflow fixture is now
  indexed under `examples/three-lane-design-build-review/` with graph and
  referenced-file regression coverage.
- The `/doctor` HTML page renders from daemon `doctor` in production while
  keeping the old fixture payload path quarantined for the subprocess test
  harness.
- PostgreSQL lane-liveness attestation now verifies the session/run binding,
  live PID identity, PID start-time token, and workflow snapshot lane command.
- The Postgres supervision handler suite now launches the real
  `go/bin/striatum-supervisor-helper` in a focused integration test and
  verifies start, send, packet acknowledgement, status drain, and agent-exit
  event ingestion across the Python/Go boundary. CI now promotes that check
  through a Linux/Postgres `daemon-go-helper-integration` target instead of
  relying on full-suite discovery.
- Go now owns the `supervise.report` mutation for direct wrapper control
  events and helper JSONL batches, recording supervisor heartbeat/exit state
  and hash-chained `supervisor.*` events without SQLite fallback.
- Existing supervisor paths now reconcile restart state before trusting an
  attached process: `supervise.status`, `supervise.send`, and claim-next
  auto-delivery record `supervisor.reattached` for surviving PID identity,
  fail closed for unverifiable repair states, and mark stale PID identity as
  `lost` before any packet write.
- The supervised-wrapper fixture suite now covers Claude, Codex, and Gemini
  wrappers, verifying multi-packet loops, inner-command failure isolation,
  clean EOF exits, temp scratch logging, and the non-interactive tool-approval
  flags that keep lanes from stalling on prompts.
- Chat transcript, briefing, session listing, display projection, and
  workflow-write confirmation helpers now live in `striatum.web.chat_session`
  with focused regression coverage.
- Web-only legacy SQLite fallbacks moved from the root service namespace into
  `striatum.legacy_sqlite.service`; the root `service_legacy.py` module is
  gone, and quarantine tests now assert that the primary service only loads
  the explicit legacy package through a lazy fallback boundary.
- The local service no longer eagerly imports the legacy `striatum.api`
  wrapper at module load; `/v1/invoke` keeps the compatibility wrapper but
  lazy-loads the legacy API only when that test-harness path is called.
- D108 resolves RFC 0071's authority-matrix generation question: the command
  authority matrix stays curated for authority/status classification, while
  architecture tests enforce generated CLI route labels and runtime CLI
  fallback cells.
- `striatum daemon doctor --repo <path> --authority --json` now mirrors the
  verify-only `striatum.repo_cutover_report.v1` inside doctor output and
  summarizes repository cutover health in `striatum.authority_report.v1`
  without opening SQLite.
- Static asset lookup and content-type mapping moved from `service.py` into
  `striatum.web.static_assets`, keeping HTTP response writing in the service
  handler while making the non-SQLite web split independently testable.
- Workflow editor file resolution, scaffold payloads, validation, atomic
  writes, and If-Match checks moved from `service.py` into
  `striatum.web.workflows`; the service handler now keeps only HTTP request
  parsing, template rendering, and JSON response mapping for those routes.
- Run-list presentation helpers for GitHub remote parsing, source-path
  normalization, workflow tree links, and state chips moved from `service.py`
  into `striatum.web.run_list`.
- Artifact web helpers for safe repo-relative path resolution, raw download
  content-type selection, and inline Markdown rendering moved from
  `service.py` into `striatum.web.artifacts`.
- The `/v1/invoke` read/mutation classifier moved from `service.py` into
  `striatum.service_command_policy`, keeping the legacy
  `striatum.service.is_read_command` import surface stable.
- Phase 7, Phase 8, and Phase 12 policy blockers are now explicit in the
  backlog/RFCs: accepted lint-risk persistence waits on a durable authority
  decision, global/default auto-finalize waits on live dogfood confidence plus
  a product decision, and Git/PR integration remains read-only-local-only until
  commit authority and hosted-provider boundaries are accepted.
- The repository file-view helpers for safe path validation, binary detection,
  text/Markdown payload shaping, and inline Markdown rendering moved from
  `service.py` into `striatum.web.view_file`; the service keeps the route,
  template rendering, and legacy breadcrumb injection.
- SSE replay offset parsing and event framing moved from `service.py` into
  `striatum.service_sse`, keeping the stream loop and daemon polling in the
  service handler.
- Local service process state, GitHub remote/default-branch caching,
  shutdown signaling, web-context secret generation, and per-run SSE slot
  accounting moved from `service.py` into `striatum.service_state`.
- Local service runtime helpers for version/mode reporting, loopback binding
  validation, PID-file single-instance checks, startup exceptions, and idle
  shutdown waiting moved from `service.py` into `striatum.service_runtime`.
- Web template environment construction and HTML escaping helpers moved from
  `service.py` into `striatum.web.template_env`, keeping the existing
  `striatum.service` private aliases stable for tests and route methods.
- Request authentication, bearer-token checks, same-origin mutation policy,
  and override-verdict web-context validation moved from `service.py` into
  `striatum.service_request_security` with pure decision helpers and focused
  CSRF/context-token tests.
- Workflow template listing/show and workflow generation preview/write response
  shaping moved from `service.py` into `striatum.web.workflow_generation`.
- Request-body parsing and JSON/HTML response helpers moved from `service.py`
  into `striatum.service_request_io`, keeping the handler wrappers stable.
- Daemon-backed run-event SSE streaming moved from `service.py` into
  `striatum.service_sse`, keeping the handler responsible for slot accounting
  and legacy fixture fallback selection.
- `recovery watch` is now a daemon-backed foreground scheduler over the
  canonical `recovery.sweep` RPC, with the broken `recovery.watch` method and
  CLI route removed from the shared contract, generated docs, Python registry,
  and Go registry.
- Documentation stale-state cleanup now records the shipped status for the
  workflow chooser, chat-assisted workflow scaffolding, escalation artifact
  schema/inbox, current-scope process supervision, and the RFC 0039 blocker.
- Doctor page DTO loading, legacy fallback selection, record recipe shaping,
  and problem grouping moved from `service.py` into `striatum.web.doctor`.
- Workflow browser index/detail page DTO shaping moved from `service.py` into
  `striatum.web.workflows`, keeping the handler responsible only for template
  rendering and HTTP error mapping.
- Chat index/session rendering, chat creation, provider send/tool loop,
  workflow-write confirmation, stop redirects, and transcript SSE tailing
  moved from `service.py` into `striatum.web.chat_routes`; service-private
  briefing and git-helper aliases remain stable.
- Run list/detail, job detail, artifact view, and posture-verdict page
  rendering moved from `service.py` into `striatum.web.run_pages`, leaving
  stable private handler wrappers for existing route tests and callers.
- Artifact raw download orchestration moved from `service.py` into
  `striatum.web.artifacts`, with the service wrapper still owning the HTTP
  handler entry point and response writer callbacks.
- Workflow run-now, branch-confirm, run cancel/pause/resume, and job
  cancel/retry route handling moved from `service.py` into
  `striatum.web.run_actions`, preserving the private service wrappers and
  legacy fixture fallback/error-mapping boundaries.
- Workflow browser and visual-editor route rendering/saving moved from
  `service.py` into `striatum.web.workflows`; the service now keeps only
  stable private wrappers and passes the existing template factory seam.
- Repository `/view` page rendering moved from `service.py` into
  `striatum.web.view_file`, with the legacy dogfood run-breadcrumb lookup
  injected through the service wrapper.
- JSON read helpers, repo-tree reads, daemon-read fallback handling, and
  run-event SSE route control moved from `service.py` into
  `striatum.service_api_routes`, preserving handler wrappers for direct tests.
- `cross-repo cancel` now routes through the daemon RPC contract to
  `cross_repo.cancel`, uses the PG-native participant-cancel runner, delegates
  each non-terminal participant to the daemon `run.cancel` handler, skips
  terminal or not-yet-local participants, and records blocked participant
  diagnostics in `last_reconcile_error`.
- PostgreSQL `recovery.sweep` now executes configured checkpoint-timeout
  escalation hooks (`marker_file`, `webhook`, `shell`) through the shared
  recovery hook dispatcher, keeps dry-runs side-effect-free, and folds hook
  failures into `escalations[]` instead of raising or reporting the old
  deferred placeholder.
- Local service GET/POST route selection moved from `service.py` into
  `striatum.service_routes`, keeping the handler's stable wrapper methods
  while continuing the daemon-first web-service split.
- Local service TCP/Unix binding, PID-file handling, signal shutdown, and
  serve loop orchestration moved from `service.py` into
  `striatum.service_server`; private compatibility wrappers remain in place.
- Workflow validation now rejects `needs_revision` cycles whose `from`/`to`
  jobs cross phase boundaries, closing the RFC 0045 V1.5 cycle phase-jump
  validator gap.
- Workflow validation now accepts canonical job `phase` fields from the React
  workflow editor, keeps `phase_id` as a compatibility alias, and rejects
  conflicting aliases.
- Explicit v1.1 phase arrays now require `phases[].synthesis_job_id` to
  point at the same phase's unique `phase_synthesis` job; generator, upgrade,
  fixtures, and phase-progress tests now emit the field.
- The React workflow editor now keeps missing/unknown phase jobs visible in an
  invalid phase bucket, removes the explicit-phase `(unset)` dropdown bypass,
  and defaults newly dropped jobs to the first declared phase.
- `dogfood.publish_on_behalf` mid-composite failures now report the failed
  step, partial composition steps, and nested specific error details through
  the helper result, rollback event, daemon RPC error, and MCP
  `structuredContent`.
- Archive create/verify replay now covers archived command request,
  process-supervisor, and process-supervisor-pointer rows, and replay
  verification rejects duplicate or missing ids for those rows plus archived
  verdict, blocker, process-execution, and job-worktree rows.
- Roadmap kickoff status and remediation sequencing notes were refreshed to
  match the post-v1.55.0 daemon-first architecture work and the current
  blocked-policy boundaries.
- Current docs, RFC status notes, reusable prompts, and root reference
  artifacts were swept for stale substrate/runtime guidance: daemon-owned
  PostgreSQL is now the live-state authority, `.striatum/` is operational
  scratch, RFC 0048 is marked completed, and Engram is framed only as optional
  external augmentation.
- Daemon-routed CLI commands now fail closed on unexpected route-layer
  exceptions instead of falling through to the legacy dispatch body; an
  architecture guardrail keeps the SQLite-connect tripwire armed for that
  path.
- Backlog records for the same-model validator rule and real UI bundle /
  supply-chain polish were closed after verifying the current validator,
  package, bundle, and guardrail tests cover the formerly open work.
- Job-detail page DTO shaping moved from `service.py` into
  `striatum.web.job_detail`, leaving the route handler responsible for daemon
  RPC/fallback, template selection, and HTTP error mapping.
- `repo.add`, `repo.list`, and `repo.remove` now route through daemon RPC and
  operate directly on daemon-owned Postgres registration rows. `repo add
  --init` creates only `.striatum/` operational scratch and no
  `.striatum/state.sqlite3`; existing repo-local SQLite sources must be
  archived/removed before registration. The legacy importer is fixture-only.
- Production `striatum init` and `striatum adopt` now use the same
  scratch-only bootstrap and no longer create repo-local SQLite, including
  `init --with-skills` in paired test-harness mode.
- `adapter run` is retired outside that same legacy fixture escape, closing
  another production path to the repo-local SQLite process-adapter tables.
- The legacy `byline` helper and `inbox --session-id` packet helper are also
  retired outside fixtures; production clients should use daemon read
  surfaces.
- The legacy SQLite daemon registry compatibility escape now requires the
  paired test-harness markers; setting only
  `STRIATUM_ALLOW_LEGACY_SQLITE_REGISTRY=1` no longer reopens production
  registry access.
- `workflow upgrade` now fails closed instead of falling back to repo-local
  SQLite running-run checks; unknown PostgreSQL state is a refusal even when
  legacy SQLite files are present.
- RFC 0058 V1 now has publisher-visible operator artifact kinds
  (`operator_brief`, `work_plan`, `progress_note`, `operator_report`),
  corpus metadata columns for operator docs, and a seeded
  `docs/operator/` current-state surface that supersedes ad-hoc handoffs.
- `daemon migrate-repo-local --verify-cutover --json` now emits
  `striatum.repo_cutover_report.v1` using PostgreSQL queries plus raw
  source/tombstone/sentinel file checks, without opening SQLite as a database.
- Fresh-clone and package smoke scripts now exercise only the daemon/Postgres
  repo registration path. If PostgreSQL setup is unavailable they skip with a
  clear prerequisite message instead of falling back to repo-local SQLite
  test-harness mode; the scripts still keep their smoke workflow inside the
  target repository for `run prepare`, install the packaged RPC method
  contract into wheels, and use the current `striatum-orchestrator`
  distribution artifact names.
- Artifact view template-context shaping, byline display, recorded
  attestation chips, lane-evidence chips, and expected-artifact row shaping
  moved into `striatum.web.artifacts`; the daemon-backed artifact page no
  longer reaches into the legacy SQLite fallback module for pure
  presentation shaping.
- Run posture-verdict template-context shaping moved into
  `striatum.web.run_posture_verdicts`; the service route keeps daemon
  RPC/fallback and HTTP error mapping while posture DTO validation and
  verdict-row filtering live in web presentation code.
- Current docs were swept again for stale routing/runtime language after
  PG-native repo registration: quick-start docs now favor `adopt` or
  `repo add --init`, `dashboard --all` is described as daemon/Postgres-backed,
  Pattern 5 in the harness friction notes is marked historical/resolved for
  daemon-routed command and post-tombstone init slices, and evidence exports
  no longer imply `.striatum/` SQLite is live state.

- **Command authority and fallback guardrails.**
  `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` now names the authority
  owner for daemon RPC, CLI translation, Python PG handlers, Go helper
  registrations, and remaining SQLite quarantine paths. Guardrail tests
  keep daemon registry methods classified, prevent new CLI fallback routes
  from appearing silently, and tripwire representative production commands
  against direct SQLite opens.
- **Single daemon method contract source.**
  `contracts/daemon_methods.json` drives the Python compatibility registry,
  `daemon.describe`, generated Go registry metadata, MCP tool descriptors,
  generated architecture reference tables for daemon methods and CLI route
  translation, runtime CLI route lookup through the declarative `cli_routes`
  map, and contract parity tests. Workflow authoring remains explicitly
  CLI-local.
- **Go production-daemon strategy.**
  D107 supersedes D105 and restores the Go production-daemon port as the
  active architecture target. D109 made Go the default daemon core, and D111
  retires the Python daemon selector while leaving the Python CLI/web layers
  as daemon clients.
- **Daemon-first web service.**
  The local web service now uses daemon RPC for run cancel/pause/resume, job
  cancel/retry, branch confirm, workflow run-now lifecycles, run listing,
  chat briefing active-run
  summaries, the JSON read endpoints for status/doctor/why/dashboard/run
  artifacts, the artifact detail page, and the posture-verdict drill-down
  page. The run detail page now renders from daemon `run.detail` in
  production, keeping SVG/HTML rendering local while moving page state to a
  read DTO. The job detail page now renders from daemon `job.detail`,
  including expected artifacts, process evidence, and verdict override
  context. Run-now now calls daemon `run.prepare`, `branch.confirm`, and
  `run.start` in production, preserving the historical 422 field-level
  workflow validation response through daemon RPC error details. The new
  `run.posture_verdicts` daemon DTO backs the posture page,
  while `artifact.show` can now include run, expected-author, and provenance
  context for the artifact page. Legacy CLI/SQLite fallbacks are retained
  only for the subprocess test-harness escape. The `/v1/invoke`
  mutation gate now classifies daemon-routed commands from
  `METHOD_REGISTRY.required_capability`, with only CLI-local workflow
  authoring reads kept in an explicit service list. Production service
  startup now checks daemon/repository health through daemon `doctor` before
  binding; the old SQLite integrity check is limited to the subprocess
  compatibility harness. The `/doctor` HTML page now renders from daemon
  `doctor` in production while retaining per-record recovery recipes and a
  test-harness-only legacy fixture fallback. The web SSE event stream now
  polls daemon `run.events` in production, with the old SQLite event tail
  kept only for the same subprocess harness. As the first behavior-preserving split,
  pure HTTP/security helpers moved from `service.py` into
  `service_http.py` while keeping the existing `striatum.service` imports
  stable. Chat transcript projection, briefing, JSONL append, timestamp,
  stable-hash, safe-git, multipart, session path/listing, display-message,
  and workflow-write confirmation helpers now live in
  `striatum.web.chat_session`, leaving `service.py` focused on HTTP routing,
  provider streaming, and response handling.
  The gated subprocess-fixture mutation fallbacks and legacy error mappers
  now live in `striatum.legacy_sqlite.service`. The remaining legacy
  page-read payload builders, view-file breadcrumb lookup, doctor-page
  fixture payload, SSE event tail, and legacy startup integrity check are now
  quarantined there as well. `service.py` no longer imports or opens
  repo-local SQLite directly, and importing the primary service no longer
  eagerly imports the legacy SQLite fallback module. Static asset lookup and
  MIME selection now live in `striatum.web.static_assets`, with service-level
  response writing kept unchanged. Workflow editor file resolution,
  new-workflow scaffolding, validation, atomic writes, and If-Match handling
  now live in `striatum.web.workflows`, while service-level route methods keep
  the HTTP request/response boundary. Run-list presentation helpers for
  GitHub remote parsing, workflow source-path normalization, tree-link
  construction, and state chips now live in `striatum.web.run_list`. Artifact
  path validation, raw download content-type selection, and inline Markdown
  rendering now live in `striatum.web.artifacts`. The `/v1/invoke`
  read/mutation classifier now lives in `striatum.service_command_policy`,
  keeping the service route focused on request validation and dispatch.
  Repository file-view path validation and content payload shaping now live in
  `striatum.web.view_file`; `service.py` keeps route-level rendering and the
  legacy run-breadcrumb fallback injection. SSE replay offset parsing and event
  framing now live in `striatum.service_sse`. Local service process state and
  per-run SSE slot accounting now live in `striatum.service_state`. Service
  runtime helpers now live in `striatum.service_runtime`, and template
  environment helpers now live in `striatum.web.template_env`. Request
  security policy now lives in `striatum.service_request_security`. Workflow
  generation endpoint response helpers now live in
  `striatum.web.workflow_generation`. Request-body parsing plus JSON/HTML
  response helpers now live in `striatum.service_request_io`. Doctor page
  response shaping now lives in `striatum.web.doctor`.
- **Escalation inbox foundation.**
  `escalation.list`, `escalation.show`, and `escalation.resolve` project
  human-principal escalations from blocker state. The `escalation` artifact
  kind, `striatum.escalation.v1` front matter schema, CLI routes, daemon
  contract entries, and artifact-to-blocker linkage are in place. Escalation
  projections now suppress stale artifact links unless they match a real
  artifact row by id, path, and content hash; idempotent artifact publish
  retries repair missing links and reject conflicting blocker metadata.
- **Supervisor control channel.**
  Supervision now records structured control events through
  `supervise.report`, reports delivered-unacknowledged sends explicitly, and
  includes a standalone Go `striatum-supervisor-helper` that launches agents
  under PTY while emitting JSONL control events without importing domain DB
  or RPC code. Lanes can now opt in to `supervision.transport: "pty_helper"`,
  letting `supervise.start` launch the helper, persist pointer metadata, and
  ingest helper JSONL acknowledgements through the existing control-event
  path. Pipe transport also has an explicit
  `supervision.stdin_delivery: "one_shot_eof"` opt-in for single-prompt
  commands such as `cmd -`; default supervised lanes continue to use the
  persistent FIFO contract. The daemon now implements
  `supervise.reattach_status` as a read-only supervisor health DTO, and
  `doctor` surfaces non-healthy reattach states for stale supervisors without
  mutating runner state. Recovery sweep now owns attached-supervisor
  heartbeat-stall detection: `supervise.status` can report
  `liveness: "stalled"` with `last_progress_age_seconds`, `doctor` and
  `status` surface stale attached supervisors, and expired stalled leases
  become open `heartbeat_stall_lease_expired` blockers without auto-killing
  the OS process. PostgreSQL lane-liveness attestation now matches the
  stricter legacy semantics: attached supervisor rows attest only when the
  session/run binding, live PID, PID start-time token, and workflow snapshot
  lane command all match. The Postgres supervision handler tests now include
  a focused integration case that launches the built Go helper and verifies
  helper event ingestion across `supervise.start`, `supervise.send`, and
  `supervise.status`; CI now runs that case explicitly through
  `make daemon-go-helper-integration` on Linux runners with Postgres.
  Reattach/lost-state reconciliation now runs on existing status, send, and
  claim auto-delivery paths, updating daemon-instance metadata for surviving
  supervisors and marking stale PID identity lost before delivery. The
  supervised-wrapper fixture suite now exercises
  `.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh` with provider
  stubs, pinning the persistent FIFO loop contract and the auth-bypass flags
  required for non-interactive lane operation.
- **Workflow risk lint.**
  `striatum workflow lint` supports structured warnings, opt-in strict mode,
  accepted-risk rationale and decision references, advisory coverage scoring,
  service/API surfacing, workflow browser warnings, and generator preview
  summaries. `workflow validate` now refuses same-model
  implementer/reviewer pairings by default when the existing lint rules find
  them, with `--allow-same-model-pairing` as the explicit override.
- **Runner-owned workflow fixture cleanup.**
  The historical P001 three-lane design/build/review shape is now indexed as
  `examples/three-lane-design-build-review/`, covered by a regression test
  for its graph and referenced files, and marked complete in the TODO and
  roadmap trackers.
- **Auto-finalize, archive, replay, packaging, and setup slices.**
  `recovery.auto_finalize` landed as a daemon/Postgres recovery method with
  dry-run and opt-in live modes, status/dashboard preview surfacing, and
  auto-from-artifact provenance. Recovery sweep acceptance coverage now pins
  a dogfood-shaped run where three valid written review findings auto-finalize
  without operator-on-behalf or override provenance. Run archive and corpus
  verification foundations, archive replay event-row hash recomputation,
  frontend bundle integrity checks, and day-zero setup docs were advanced as
  part of the same remediation sequence.
- **Redaction hardening.**
  Evidence redaction now treats `safe` policy entries as scalar-only, so
  injected objects/lists in otherwise safe fields are replaced with the
  redaction placeholder. Corpus source-path deny checks are
  case-insensitive for transcript/output/private path shapes, with synthetic
  injection coverage for workflow/job prompts, verdict rationales, blocker
  text, transcript-like fields, nested payloads, and path hygiene.
- **Operator terminology cleanup.**
  Reader-facing docs, CLI help, scaffold text, workflow templates, and
  recovery skill templates now use principal/operator vocabulary where the
  product means a user decision or operational action, while leaving durable
  schema and event identifiers unchanged.
- **CI portability.**
  The multi-repo harness CI install now includes the `daemon-pg` extra before
  running Postgres-backed tests, and Go supervisor process-launch tests resolve
  `true`/`cat` from `PATH` instead of assuming Linux-style absolute paths.
  The Makefile's Postgres-backed test targets now install the same extra into
  the project `.venv` before invoking the harness.

## v1.55.0 — 2026-05-15

### RFC 0048 V1.5 hardening + Schema v6

The substrate flip from RFC 0043 V1.6 → RFC 0048 Phase A/B/C is now
hardened end-to-end:

- **F2 — capability-denial test matrix.**
  `tests/daemon_pg/test_capability_denial_matrix.py` parametrizes every
  PG-backed RPC method × five denial reasons (token_missing,
  capability_missing, token_revoked, token_expired,
  capability_scope_mismatch). 70 deny cases lock the fail-closed
  routing-rule for the ported handler set. Plus an audit-row append
  assertion for the deny path.
- **F3 — audit-chain row-lock.**
  `src/striatum/daemon_rpc/request_log.py::append_audit_row` now
  `SELECT … FROM striatumd.audit_chain_head … FOR UPDATE` inside an
  explicit `conn.transaction()` so concurrent appenders serialize on
  the singleton head row. Without it, two transactions could compute
  `row_hash` over the same `previous_hash` and fork the chain.
  `tests/daemon_pg/test_audit_chain_concurrency.py` verifies a contiguous
  chain across 12 simultaneous denied requests.
- **F4 — append-only role-grant tests.**
  `tests/daemon_pg/test_append_only_role_grants.py` asserts the
  `striatumd_rw` role lacks UPDATE/DELETE on `striatumd.events` and
  `striatumd.artifacts` (migration 0005 REVOKE) while retaining
  UPDATE/DELETE on transient state tables. End-to-end SQLSTATE 42501
  checks gated on TCP auth (peer-auth setups skip).
- **HIGH#1 — parity rig.**
  `tests/daemon_pg/handlers/_parity.py` provides `assert_payload_parity`
  (recursive dict/list diff with ignore-keys for timestamps/UUIDs).
  Removed the historical `_stub_missing_workflow_loop_modules` workaround
  from `tests/daemon_pg/handlers/recovery_evidence/conftest.py` and
  `_helpers.py` (Track A landed in v1.49.0, stubs are dead). Removed
  the `RFC0048_PARITY` env-var skipif from `test_stale_leases` and
  `test_requeue_stale` so PG-handler invocations run by default. Full
  per-handler byte-equivalent fixture seeding (16 handlers) tracked
  as follow-up.
- **HIGH#2 — inline-helper wiring.**
  `src/striatum/daemon_pg/handlers/workflow_loop/complete_job.py`
  exports `complete_inline(...)` and `ack_work.py` exports
  `ack_inline(...)`. `recovery.resume --complete` (`resume_blocker`) and
  `recovery.auto` live mode (`auto_publish_stale_artifacts`) no longer
  raise ImportError on the inline-helper imports.
- **Schema v6 — migration 0006.**
  `striatumd.events` gains dedicated `previous_hash` (nullable) and
  `row_hash` columns plus a `striatumd.repo_event_chain_heads`
  singleton-per-repository pointer. The migration backfills both
  columns from `payload_json._event_chain` and strips that key from
  payload_json on existing rows; refuses to migrate if any row lacks
  anchor metadata. `RepoHandlerContext.append_event` (Python) and
  `pkg/mutations.insertEvent` (Go) read the chain head with `FOR
  UPDATE`, write the columns directly, and upsert the head pointer —
  serializing concurrent appenders per-repository on the parent
  `striatumd.repositories` row.
- **CLI dispatch fail-closed.**
  Earlier commits also flipped the CLI dispatch hook so mapped daemon
  RPC verbs no longer fall back to SQLite when the daemon is
  unreachable or the target repository is not registered (`src/striatum/
  cli/daemon_rpc_route.py`). The `daemon doctor`'s legacy SQLite
  registry probe is surfaced as
  `{"status": "post_pg_cutover_unused", …}` rather than a scary
  `token_invalid` error.

### Pre-flight cleanup (aaf5d3c)

- `daemon doctor`'s SQLite-registry probe now reports
  `post_pg_cutover_unused` when PG is the authoritative auth surface.
- `.gitignore`: `build/` un-commented (the tracked dogfood-043
  HANDOFFs remain un-ignored via existing `!…/HANDOFF.md` exceptions).
- `docs/handoffs/2026-05-15-rfc-0048-postgres-transition.md`: `daemon
  doctor --explain --json` jq path corrected to `.data.explain`.

## v1.54.0 — 2026-05-15

### RFC 0048 Phase B (read surface) — Go-core parity for the 12 read CLI verbs

The Go daemon previously registered every single-repo handler as
`notImplementedHandler` (codex F2 finding from dogfood-049). Phase B
ports the read-surface handlers — same shape as Python's
`src/striatum/daemon_pg/handlers/reads/`, same return-shape parity
contract so CLI + operator UI don't detect the substrate-language flip.

New `go/pkg/reads/` package:

- `reads.go` — shared helpers: `Queryer` interface (narrowed
  `pgx.Rows` access), `collectRows` for generic `map[string]any`
  result sets, `requireRepositoryID`, parameter helpers.
- `status.go` — `HandleStatus`: runs/jobs/sessions/verdicts/blockers
  scoped by repository_id + optional run_id; computes claimable +
  blocked_downstream + next_actions; returns the legacy JSON shape.
- `dashboard.go` — `HandleDashboard`: jobs_by_state / verdicts_by_state /
  blockers / sessions / last-10 events. Defaults to the most recent run
  when no run_id supplied (parity with Python).
- `doctor.go` — `HandleDoctor`: schema_version + stale-lease +
  waiting-human counts + problems list.
- `why.go` — `HandleWhy`: events touching a target_id across job/session/
  run/message/lease/payload-json columns.
- `listings.go` — `HandleListRuns` / `HandleListSessions` /
  `HandleListJobs` / `HandleListArtifacts` / `HandleListWorkflows`. Each
  accepts state/role/lane/workflow_job_id/kind filters (matches the
  Python translator's parameter propagation) with bounded `limit`
  (max 1000, default 200-500 per-method).
- `exports.go` — `HandleRunSummary` (run row + jobs + artifacts +
  verdicts + doctor block via `HandleDoctor` for parity), `HandleEvidenceExport`
  (scoped artifacts + verdicts + doctor), `HandleCorpusExport`
  (corpus_contract_version=1 manifest + paged artifact rows).

`go/cmd/striatumd/main.go` calls `reads.Register(server, runner)` before
the not-implemented stub loop. The for-loop's
`if _, exists := server.Handlers[method]; exists { continue }` then
skips these methods, so existing fallbacks remain for unported
mutations. Snapshot: ~12 fewer "not_implemented" methods.

`go build ./... && go vet ./...` clean. Read handlers integrate with
the existing `PostgresAuthorizer` + `AuditRecorder` so capability
checks + audit chain semantics are unchanged from cross_repo handlers
(no per-handler auth shim required).

### Companion: Python GH #19 PG-side message parity

`src/striatum/daemon_pg/handlers/recovery_evidence/requeue_stale.py`
and `tests/test_cli_mvp.py` updated to point operators to the new
`--force --justification "<reason>"` flag that shipped in v1.53.0. The
SQLite-backed path's message was already updated in v1.53.0; this
brings the PG-backed handler's message + the integration test in line.

### Outstanding Phase B (still deferred)

- Write-surface Go ports — 16 mutation handlers (session.register,
  claim_next, ack_work, complete_job, release_lease, block_job,
  record_verdict, submit_review, override_review_verdict +
  recovery.\* + evidence.\*). Each requires transaction + audit-chain
  append, materially more complex than reads. Tracked as next
  Phase B milestone.
- Cross-implementation parity tests (`make test-multi-repo CORE=go`
  byte-identical state assertion). Land after writes port.
- RFC 0048 Phase C SQLite-removal default flip — gated on
  Phase B mutations + the V1.5 fix-up items (codex F2-F4 + claude
  HIGH#1/#2 + schema migration 0006).

## v1.53.0 — 2026-05-15

### GH #19 — recovery requeue-stale --force --justification

`striatum recovery requeue-stale` now accepts `--force --justification
"<reason>"` to override the `repo-write stale jobs require manual
inspection` refusal after the operator has inspected the on-disk
artifact and decided requeue is appropriate. The override is
audit-chained: the resulting `recovery.stale_requeued` event payload
gets `operator_override=true` and `justification=<reason>` fields so
future audits can replay the decision.

Without `--force --justification`, the original refusal still fires
(regression guard).

### GH #21 — serve refuses to start over a corrupted state.sqlite3

Adds `_verify_state_health(repo)` to the `striatum serve` startup path
(both TCP and Unix transports). Before binding any socket, the function:

- Refuses to open if `state.sqlite3` exists but cannot be opened by
  `sqlite3.connect`.
- Runs `PRAGMA integrity_check`; if the result isn't `ok`, raises
  `ServiceConfigError` naming the file + remediation (quarantine to
  `.corrupt`, run `striatum init`, retry).
- Runs `PRAGMA wal_checkpoint(TRUNCATE)` on the existing DB so any
  pending WAL is flushed to the main file before the new serve takes
  the write lock. This closes the failure mode observed 3 times in one
  session: SIGKILL on the previous serve left WAL in an inconsistent
  state; SQLite recovery on the new serve truncated to the last
  checkpoint, losing MB-scale active-run rows down to KB-scale.

### RFC 0048 V1.5 — daemon doctor --explain

New `--explain` flag on `striatum daemon doctor` adds a per-method
table to the doctor output. Each row reports:

- `method` — RPC method name (from `striatum.daemon_rpc.registry`).
- `pg_backed` — whether `resolve_pg_handler(method)` returns a handler
  (true = ported in Phase A or Phase C; false = still falls through).
- `sqlite_fallback_route` — the legacy CLI route in `CLI_ROUTES`, if
  any. Methods with no fallback route are PG-only.
- `required_capability` / `repository_scope` / `deprecated` — registry
  metadata.

Plus summary: `method_count`, `pg_backed_count`.

Current snapshot post-v1.52.0: 93 methods / 34 PG-backed / 68
SQLite-fallback-routed.

### Outstanding RFC 0048 V1.5 items (still deferred)

- codex F2 capability-denial test matrix (16 handlers × 6 denial cases).
- codex F3 audit-chain SERIALIZABLE/row-lock per write handler.
- codex F4 append-only role-grant tests at the daemon-pg layer.
- claude HIGH#1 byte-equivalence parity rig wired into all 16+ tests
  (the `ReadSeed` / `pg_ctx` / `sqlite_conn` / `assert_payload_parity`
  helpers).
- claude HIGH#2 dead code paths (`complete_inline`, `ack_inline`,
  `recovery.resume --complete`, `recovery.auto` live mode).
- Schema migration 0006 (`events.previous_hash` / `row_hash` columns).
- RFC 0048 Phase B (Go core parity).
- RFC 0048 Phase C SQLite-removal default flip.

## v1.52.0 — 2026-05-15

### RFC 0048 Phase C complete — read-surface PG handlers (dogfood-060)

Closes the substrate flip. All 12 read-surface CLI verbs now have
native PG-backed handlers under `src/striatum/daemon_pg/handlers/reads/`
and route through the daemon RPC instead of falling through
`CLI_ROUTES` → `invoke()` → repo-local SQLite. After
`daemon migrate-repo-local`, the `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` escape is no longer required for the read
verbs.

Ported handlers (12):

- `status`, `dashboard`, `why`, `doctor` — core operator reads.
- `list.runs`, `list.sessions`, `list.jobs`, `list.artifacts`,
  `list.workflows` — listing reads.
- `run.summary`, `evidence.export`, `corpus.export` — reporting /
  export reads.

Each handler:

- Registered via `@register_pg_handler("<method>", read_only=True)`
  decorator from the Phase A registry.
- Scopes by `ctx.repository_id` on every SELECT (no cross-repo leakage).
- Returns the same top-level JSON shape as the legacy SQLite-backed
  function (parity contract — CLI and operator UI don't detect the
  substrate flip).

Implementation supports + plumbing:

- New `_read_model.py`, `_registry.py`, `_sql.py` shared infrastructure
  under `daemon_pg/handlers/reads/`.
- `daemon_pg/handlers/__init__.py` imports the `reads` subpackage so
  decorator registrations fire on `import striatum.daemon_pg.handlers`.
- `cli/daemon_rpc_route.py` translator updates: `list.*` filters
  (state/role/lane/workflow_job_id/kind/limit) now propagate to RPC
  params; added missing `("corpus", "export")` lookup entry.
- `corpus/redaction.py` extended to redact artifact/session prose
  before rendering corpus run-summary rows (closes a build-review
  finding about evidence leak).
- `status` handler uses the legacy operator action vocabulary from
  `cli/introspect.py:857` (closes a build-review finding about
  dashboard/web-UI parity).
- `run.summary` + `evidence.export` call the real PG `doctor` handler
  instead of hardcoding `{"ok": true, "schema_version": 5}` (closes a
  build-review finding about always-green doctor in post-migration
  exports).

Tests:

- 12 handler test files under `tests/daemon_pg/handlers/reads/` plus
  shared `read_handler_fixtures.py` for cross-suite reuse.
- `tests/test_cli_daemon_rpc_route.py` covers the translator
  parameter-propagation and corpus-export wiring.
- `tests/test_corpus_redaction.py` covers the redaction additions.
- Full target test sweep: 83 passed, 5 skipped (gated multi-repo PG
  fixtures).

### Operator-driven completion note

The dogfood-060 workflow ran the design + synth + review_design phases
and the first build review. The build review verdicts (codex
threat_model: needs_revision on missing handler-level threat-model
evidence; claude ergonomics_dx: needs_revision on parity-rig absence +
CLI translator drops + next_actions divergence + hardcoded doctor +
missing corpus-export route) named the revision punch list precisely.
The operator addressed all findings directly rather than restarting the
workflow loop, because the build-review report itself was the
implementer spec.

The structural gaps that made the workflow loop expensive — GH #19
(stale-lease recovery for repo_write jobs) and GH #21 (serve restart
clobbers state.sqlite3) — are tracked separately and remain V1.6
follow-up scope.

### Outstanding follow-ups (deferred)

- GH #19 stale-lease operator recovery path.
- GH #21 serve startup must not clobber active state.
- RFC 0048 V1.5 fix-up items: codex F2 capability-denial test matrix,
  F3 audit-chain SERIALIZABLE/row-lock per handler, F4 append-only
  role-grant tests, claude HIGH#1 byte-equivalence parity rig (the
  one named in dogfood-057's reviews — still not wired), HIGH#2 dead
  code cleanup, schema migration 0006 (events.previous_hash /
  row_hash columns), `daemon doctor --explain`.
- RFC 0048 Phase B (Go core parity) — multi-week.
- RFC 0048 Phase C SQLite-removal flip (the actual default switch
  away from CLI_ROUTES fallback) — pending V1.6 fix-up landing.

## v1.51.0 — 2026-05-14

### RFC 0048 Phase C (partial) — CLI dispatch routes through daemon RPC

Lands the substrate-flip plumbing for CLI verbs. The dispatch hook now
checks the daemon socket and routes any verb mapped in the new
``daemon_rpc_route`` lookup through ``DaemonRpcRouter`` over Unix
socket instead of running in-process against SQLite. Falls through to
legacy SQLite when the daemon is offline, the verb is bootstrap-only
(``init``, ``skills``, ``plugin``, ``daemon``, ``repo``, ``cross-repo``,
``serve``, ``byline``, ``inbox``), or ``STRIATUM_TEST_HARNESS=1``.

New module ``src/striatum/cli/daemon_rpc_route.py`` with translators
for status / why / doctor / dashboard / list / run.\* / register-session /
claim-next / ack / heartbeat / release / block / complete /
publish-artifact / verdict / submit-review / override-verdict /
recovery.\* / evidence.export / decision.record / checkpoint.resolve /
branch.confirm. Each translator builds the RPC envelope (with capability
token loaded from ``read_runtime_token()``) and the dispatch hook calls
``daemon_rpc.client.call_unix`` with the daemon's Unix-socket handshake.

Plumbing fixes:

- ``run_daemon_foreground`` always resolves ``daemon.toml`` via
  ``daemon_pg.config.resolve_config`` (the v1.50 implementation only
  fired the PG path when the env var was set — systemd-launched daemons
  silently came up SQLite-only).
- ``run_daemon_foreground`` now bootstraps an admin client into
  ``striatumd.clients`` on first start and writes the runtime token to
  ``runtime_dir() / 'client-token'``. Mirrors the SQLite ``clients``
  bootstrap but targets the Postgres-side table that ``authorize()`` reads.
- Daemon PG connection sets ``row_factory = psycopg.rows.dict_row`` so
  ``authorize()._row_dict`` works on per-cursor results.
- ``daemon_rpc.request_log.append_audit_row`` made compatible with both
  ``tuple_row`` and ``dict_row`` factories (the codebase mixed both).
- ``DaemonRpcRouter._repo_root_for`` no longer rejects requests whose
  registered repo_root differs from the router's startup CWD — the
  daemon serves every registered repository per RFC 0043 §3, not just
  the one it was launched from.
- ``daemon_rpc.envelope.RpcEnvelope.from_mapping`` no longer requires
  dotted method names (matches the in-process ``mcp_dispatch`` behavior;
  the registry has both dotted and undotted methods).
- ``DaemonRpcRouter._route``'s CLI_ROUTES fallback sets
  ``STRIATUM_IN_DAEMON_HANDLER=1`` around ``invoke()`` so the CLI's
  Phase C hook short-circuits and doesn't re-route through the daemon
  recursively.

### systemd user unit

``~/.config/systemd/user/striatumd.service`` ships as the supported
launch path. ``systemctl --user enable --now striatumd.service`` brings
the daemon up; daemon.toml + ~/.local/bin/striatum on PATH supply the
rest. Restart on failure with a 5-second backoff.

### Operator-mode update CLI

``pip install -e . --force-reinstall --user --break-system-packages``
brings the locally-installed ``striatum`` console script forward
between minor bumps when the editable install metadata lags. RFC 0048
V1.5 follow-up will add ``striatum self-update`` as the documented
operator wrapper.

### Phase C remaining (deferred to V1.6 / dogfood-060)

The mutation surface (16 PG handlers from RFC 0048 V1 Phase A) routes
end-to-end via the new Phase C hook + daemon RPC. The read surface
(status, dashboard, list.\*, run.summary, why, doctor, evidence.export,
corpus.export) still falls through ``CLI_ROUTES`` in the daemon to
``invoke()`` which uses repo-local SQLite. After
``daemon migrate-repo-local`` finalizes the SQLite as a tombstone,
those read verbs return exit 3 (``state is not initialized``). To make
the substrate flip complete, RFC 0048 needs PG handlers for the read
verbs too. Captured in OPERATOR_REPORT.md for the next dogfood.

For now: operators run with ``STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1`` for un-migrated repos; migrated repos can
use the mutation verbs through daemon RPC but cannot inspect state
until read handlers land.

## v1.50.0 — 2026-05-14

### RFC 0048 V1.5 — Daemon Unix-socket accept loop + role-provisioning runbook

Closes the V1.5 migration-blocking gap from dogfood-057's V1 Phase A. The
RFC's V1 Phase A landed PG-backed handlers and a router with PG-vs-CLI
delegation; what made the daemon-required CLI non-functional was that
`run_daemon_foreground` bound a Unix socket and listened, but never
called `accept()`. So `striatum status` (and every other daemon-required
verb) refused with exit 11 even though the daemon process was alive.

Adds the missing accept loop to `src/striatum/daemon.py::run_daemon_foreground`
(synthesis pattern from dogfood-058):

- One accept thread polls `sock.accept()` with a 0.5s timeout against a
  `threading.Event` stop flag.
- Each accepted connection gets a daemon thread that wraps
  `conn.makefile("rwb")`, iterates NDJSON envelopes via
  `striatum.daemon_rpc.framing.read_envelopes(stream)`, and dispatches
  through `DaemonRpcRouter.handle(envelope, connection_id=<uuid>,
  transport="unix", require_handshake=True)` — writing each response
  back via `striatum.daemon_rpc.framing.write_response`.
- Router constructed once at startup with the daemon's PG connection
  (from `daemon_pg.connection.connect` after `doctor(..., apply=True)`
  succeeds) and `substrate_schema` from the doctor's reported schema
  version.
- Graceful shutdown: SIGTERM/SIGINT sets the stop event → closes the
  listener (breaks `accept()`) → joins accept thread with 2s timeout →
  joins per-connection threads with 0.5s each → closes daemon PG
  connection → unlinks socket + pid files.

Smoke-tested end-to-end:
- `daemon.hello` via `daemon_rpc.client.call_unix` returns
  `daemon_version` + `methods_etag` (the bound socket now actually
  serves RPC, not just probes).
- `striatum status` without `STRIATUM_DAEMON_REQUIRED=0
  STRIATUM_TEST_HARNESS=1` exits 12 (`repo_not_migrated`) instead of
  exit 11 (`daemon_unreachable`) — the daemon-required path is alive;
  the next step (`daemon migrate-repo-local`) is now reachable through
  the supported CLI flow.

### `POSTGRES_TRANSITION.md` — daemon-role provisioning runbook

Adds the "Provision the daemon-required role" section (operator
friction identified in dogfood-057's setup phase). Copy-pasteable SQL
block creates `striatumd_rw` with the right grants and revokes
(`REVOKE UPDATE, DELETE ON striatumd.{audit_log,events,artifacts}`).
Fresh installs that previously used the database owner as the
connecting role would trip the `unsafe_privileges` doctor refusal;
this section is the documented remediation.

### V1.5 follow-up still outstanding (deferred to V1.6 / dogfood-059)

dogfood-058 was scaffolded as a full 10-job V1.5 fix-up but the
cycle-exhaustion hit on `review_design` (Track-A/Track-B boundary
clarifications that codex couldn't fix in two synth revisions). After
operator override + cascade-cancel the run terminated without an
implementer phase. The accept loop + role runbook above are the
operator-driven subset that unblocks migration; the rest of V1.5
(codex F2 capability-denial test matrix, F3 audit-chain
SERIALIZABLE/row-lock per handler, F4 append-only role-grant test,
claude HIGH#1 actual byte-equivalence parity rig, claude HIGH#2 dead
code cleanup, schema migration 0006 for `striatumd.events.previous_hash`/
`row_hash`, `daemon doctor --explain`) is captured as a V1.6 / dogfood-059
follow-up in `docs/dogfood/058/OPERATOR_REPORT.md`.

Also outstanding: the migration-retry-after-rollback path (clean
re-migration when `repo_migrations` checkpoint mismatches the source
SQLite sha256) requires a `--reset-checkpoint` flag or manual
superuser cleanup; tracked in OPERATOR_REPORT.md.

## v1.49.0 — 2026-05-14

### RFC 0048 V1 Phase A — Python handler port (dogfood-057)

Land the Python side of the substrate-facade fix. All 16 single-repo
mutation handlers move from `striatum.cli` SQLite-backed dispatch into
native PG-backed handlers under `src/striatum/daemon_pg/handlers/`:

- **`workflow_loop/`** (9 methods, Track A codex implementer):
  `register_session`, `claim_next`, `ack_work`, `complete_job`,
  `release_lease`, `block_job`, `record_verdict`, `submit_review`,
  `override_review_verdict`.
- **`recovery_evidence/`** (7 methods, Track B claude implementer):
  `stale_leases`, `requeue_stale`, `cancel_job`, `process_reconcile`,
  `resume_blocker`, `auto_publish_stale_artifacts`, `evidence_export`.
- Shared infra: `handlers/__init__.py`, `handlers/registry.py`,
  `handlers/context.py`. `DaemonRpcRouter._route` resolves the PG
  handler before falling back to legacy `CLI_ROUTES`. Track B
  registers via decorator self-registration so its write scope can
  stay disjoint from Track A's server/registry/__init__ write scope.
- Tests for all 16 methods under `tests/daemon_pg/handlers/`.

**V1.5 follow-up risks** (accepted in this V1 landing — see
`docs/rfcs/0048-daemon-side-substrate-migration.md#v15-follow-up`):
codex F1-F4 (fail-closed routing, capability-denial tests, audit-chain
concurrency, append-only role enforcement) and claude HIGH#1/#2
(byte-equivalence parity tests advertised but unused; dead code paths
in `recovery.resume --complete` / `recovery.auto`). RFC 0048 V1.5
fix-up dogfood will scope these.

### Operator playbook — substrate friction observed during the run

`docs/POSTGRES_TRANSITION.md` still lacks a fresh-install
role-provisioning runbook; an operator who installs Postgres locally
and uses the database owner as the connecting role will trip the
`unsafe_privileges` doctor check (owner has implicit UPDATE/DELETE on
`striatumd.audit_log`). Workaround used during dogfood-057:
`CREATE ROLE striatumd_rw WITH LOGIN PASSWORD '...' ;
GRANT CONNECT/USAGE/SELECT/INSERT/UPDATE/DELETE on schema + tables;
REVOKE UPDATE, DELETE ON striatumd.{audit_log,events,artifacts};
GRANT CREATE ON DATABASE + SCHEMA (for migrations)`. Worth either
documenting or having `daemon doctor --apply-migrations` provision
the role on first run. Tracked as RFC 0048 V1.5 ergonomics.

## v1.48.2 — 2026-05-14

### Fixed — CI green again after 6 days of red

`gh run list --workflow CI` showed 298 consecutive failures since
`2c7237d` (2026-05-08T17:14:49Z). Two root causes:

- **Python typecheck (all 4 Python matrix cells, 16 mypy errors)** —
  missing third-party stubs (`keyring`, `psycopg`), one stale
  `# type: ignore`, one real `str.isoformat()` double-format bug in
  `daemon_pg/repo_local_migration.py::_write_sentinel`, three real
  `object`-not-iterable narrowing gaps in `test_dashboard_web_parity`,
  and untyped test functions in `test_daemon_go_supervisor` +
  `test_registry_rfc0043_coverage`. Fixed in-place; `python -m mypy`
  reports `Success: no issues found in 212 source files`.

- **Go matrix (4 cells, build step fails)** —
  `.github/workflows/ci.yml:27` pinned Go to `1.22` but `go/go.mod`
  requires `1.23` since RFC 0039 V1.5's pgx adoption. CI's setup-go
  installed 1.22; `go build` refused the toolchain mismatch. Also
  added `cache-dependency-path: go/go.sum` so setup-go can warm its
  module cache. (TODO item 30 / RFC 0039 V1.6 F1 covered the
  unchecksummed `go.sum` angle; the actual CI break was the version
  pin, not the sum file.)

No source behavior changes; the wrappers / dogfood artifacts / RFCs
shipped in v1.46.0-v1.48.1 are unaffected.

## v1.48.1 — 2026-05-14

### Fixed — claude / gemini lane wrappers exit cleanly without producing artifacts

Root cause for the 10+ instance claude permission-prompt no-publish stall and
many "gemini wrote artifact but didn't publish" failure modes was identified
by inspecting
`$STRIATUM_SCRATCH_DIR/{claude,gemini}-logs/packet-NNNN.log` after
dogfood-056: each agent CLI's permission system was prompting interactively
on the striatum CLI shell calls the packet required, and since stdin was
already consumed by the packet payload there was no one to answer the
prompt — the agent exited cleanly with the prompt as its last stdout line
and no artifact written / no CLI verb invoked.

- `.striatum/bin/claude-supervised-wrapper.sh` — `claude --print` now
  invoked with `--permission-mode acceptEdits --allowedTools "Bash"`.
  Auto-approves the striatum CLI verbs the agent must call; filesystem
  boundaries are still enforced by the packet's write_scope.
- `.striatum/bin/gemini-supervised-wrapper.sh` — `gemini --prompt -`
  approval mode changed from `auto_edit` to `yolo`. `auto_edit` approved
  file edits but not `run_shell_command`, which is why gemini wrote
  artifacts but couldn't invoke striatum to finalize.
- `.striatum/bin/codex-supervised-wrapper.sh` — no functional change; the
  existing `--dangerously-bypass-approvals-and-sandbox -c approval_policy=never`
  already cleared the same surface. Added a clarifying comment so the
  three wrappers document the same auth contract.

This is the operational complement to RFC 0051 (auto-finalize from
frontmatter): once the wrappers stop stalling on permission prompts,
the agent itself calls the closing CLI verbs, and the auto-finalize
path becomes the fallback for genuinely-crashed agents rather than the
default for every claude review.

## v1.48.0 — 2026-05-14

### Added — RFC 0050 V2: interactive layer (recovery panel island, override modal, copy-on-click, graph-editor data binding)

Lands RFC 0050 V2 via dogfood-056. Closes RFC 0050 across V1 (v1.46.0),
V1.5 (v1.47.0), and V2 (this release).

**dogfood-056 (V2 interactive layer):**
- **Recovery panel island** (`src/striatum/web/frontend/src/islands/recovery-panel/`)
  — React island enhances the server-rendered recovery panel with a dry-run
  preview of `striatum recovery auto-publish` via `/v1/invoke`. No-JS fallback
  preserved per UI_REWORK.md §8.3.
- **Override verdict modal** (`src/striatum/web/static/override_verdict.js`)
  — ARIA `<dialog>` with focus trap, Escape close, focus return.
  Posts only allowed override fields to `/v1/invoke`; identifiers come
  from server-rendered `data-*` attributes per UI_REWORK.md §8.6.
- **Copy-on-click** (`src/striatum/web/static/copy_on_click.js` + `base.js`
  wiring) — `[data-copy]` targets initialize globally on `DOMContentLoaded`,
  Enter/click copy, 1.2s toast. Identifier regex
  `^(run|job|sess|art|proc|super|lease)_[0-9a-f]+$` per UI_REWORK.md §7.7.
- **Workflow graph editor `require_attested_lane`** — per-node data binding
  in `WorkflowGraphEditor.tsx`. Stored in state, rendered in node body +
  textual summary, round-trips through serializer. **Data-binding only**;
  no viewport overlay (deferred to React Flow v12 per GH #6).
- 7 regression tests:
  `test_recovery_panel_dry_run`, `test_override_modal_payload`,
  `test_copy_on_click`, `test_run_detail_recovery_panel` (updated),
  `recovery-panel.test.tsx`, `workflow-graph-editor.test.ts`, and
  bundle-hash discipline in `test_web_ui.py`.

**Known follow-up findings (recorded in dogfood-056 review/build/):**
- **HIGH (gemini F1)**: `/v1/invoke` lacks CSRF protection +
  Content-Type validation; cross-site command execution risk on local
  runner. **Security-hardening pass deferred to v1.48.x.**
- **MEDIUM (gemini F2/F3)**: Override modal DOM tampering + recovery
  dry-run side-effect surface. Deferred to v1.48.x.
- **LOW (gemini F4/F5, claude F1-F3)**: Clipboard hijack via arbitrary
  `data-copy`, graph-editor ghost field on job type change, recovery
  panel error-state copy affordance, modal submit feedback.



### Added — RFC 0050 V1.5: template extensions + provenance-honesty fixes

Lands RFC 0050 V1.5 across dogfood-055 (template extensions) +
dogfood-055b (provenance honesty fix-up). Honest V1.5 acceptance —
gemini's 3 V1.5 provenance findings on 055 were closed in 055b before
the V1.5 override on 055's gemini verdict.

**dogfood-055 (template extensions):**
- New partials: `_recovery_panel.html`, `_expected_artifacts_table.html`,
  `_session_chip.html`.
- Templates extended to consume V1 primitives + new partials:
  `run_detail.html`, `job_detail.html`, `artifact_view.html`,
  `run_posture_verdicts.html`, `doctor.html`, `view_file.html`.
- 6 regression tests:
  `test_run_detail_recovery_panel`, `test_job_detail_expected_artifacts`,
  `test_artifact_view_provenance_trail`, `test_posture_verdicts_override_provenance`,
  `test_doctor_per_record_recipes`, `test_view_file_breadcrumb_heuristic`.

**dogfood-055b (provenance honesty fix-up):**
- `service.py::_recorded_artifact_attestation_chip` now requires both an
  exact `expected_author_line` match AND `attestation_override_rationale
  IS NULL` to render `attested`. Closes byline-forgery vector against
  operator-on-behalf publishes whose recorded byline looks model-shaped.
- `service.py::_shape_verdict_rows` distinguishes `previously_attested`
  (closed/lost supervised session) from `unattested` (never attested) —
  attestation drift over time no longer collapses into the same warning.
- `service.py::_lane_evidence_chip` + `LaneEvidenceChip.tsx` surface
  `override: <rationale>` when `attestation_override_rationale` is
  present, instead of muted `not_yet_correlated`. Closes
  override-rationale visibility gap.
- Updated regression tests: `test_byline_regression`,
  `test_override_rationale_regression`, `test_lane_evidence_guard`.



### Added — RFC 0050 V1: operator UI primitives + dashboard parity + provenance honesty

Lands RFC 0050 V1 across dogfood-054 (primitives) + dogfood-054b
(provenance honesty fix-up). Honest V1 acceptance — gemini's
adversarial findings on 054 were closed in 054b before the V1
override on 054's gemini verdict.

**dogfood-054 (primitives):**
- New shared TypeScript components under
  `src/striatum/web/frontend/src/shared/components/`:
  `RunStatePill`, `JobStatePill`, `VerdictChip` (with override
  provenance slot), `LaneAttestationChip` (with reason
  sub-text), `PostureChip`, `BylineLine`, `LaneEvidenceChip`
  (always `not_yet_correlated` muted per RFC 0050 — never
  green pre-correlation), `ExpectedArtifactsTable`.
- `templates/_components.html` — Jinja2 macros mirroring the
  TypeScript components so server-rendered and island surfaces
  speak the same vocabulary.
- `service.py` page-payload shaping for `run_list` /
  `run_detail` / `job_detail`.
- `dashboard.py` text-mode parity: same chip vocabulary as
  ASCII glyphs, consumes V1.45.0 `next_actions` verbatim.
- `static/base.css` semantic tokens
  (`--status-*`, `--attestation-*`, `--override-marker`,
  `--evidence-not-yet-correlated`). Reserved
  `--status-compromised` for V1.7.
- 3 regression tests: `test_byline_regression.py`,
  `test_dashboard_web_parity.py`,
  `test_override_rationale_regression.py`.

**dogfood-054b (V1 provenance honesty fix-up):**
Closes 4 V1 non-negotiable violations gemini caught in 054:
- **F1 byline forgery loophole closed.** `_components.html:72`
  + `BylineLine.tsx:13` force `author: operator` (or
  self-declared form) when `attested=false`. The forged disk
  byline is not rendered, not just CSS-decorated.
  `service.py:316` + `dashboard.py:473` apply the same
  substitution. Pinned by `tests/test_byline_regression.py:70`
  + `byline-line.test.tsx:7`.
- **F2 inferred-override removed.** `service.py` no longer
  guesses `operator_override` from accepting-after-non-accepting
  patterns. Missing `verdicts.source` → `natural`. Real
  overrides still render via the `verdict.overridden` event
  trail. Pinned by `test_override_rationale_regression.py:26+82`.
- **F3 attestation recording-time.** Lane attestation chips
  read from `artifacts.author_line` + recording-time supervisor
  state, not live recompute. Live recompute only on
  intrinsically-current surfaces.
- **F4 dashboard rationale.** `_verdict_chip` accepts and
  renders truncated rationale for override verdicts.

**V1.45.0 inbox SQL bug fix (incidental):**
`src/striatum/cli/dispatch.py::_cli_inbox` was selecting
`leases.job_id` but the column is named `resource_id`. The
correct subquery is `SELECT resource_id FROM leases WHERE
owner_session_id = ? AND state = 'active' AND resource_type =
'job'`. Without the fix the helper returned a random
session's packet, not the queried session's. Caught during
dogfood-054b reviewer drive.

**Provenance discipline:** every operator-on-behalf publish on
both 054 and 054b used the RFC 0046 V1
`--allow-no-process-execution --override-rationale` path. No
silent operator publishes; audit-chain records every override.

### Backlog queued for v1.47.0 / v1.48.0

- **dogfood-055** (RFC 0050 V1.5) scaffolded + validated:
  template extensions for `run_detail` (recovery panel +
  next-actions banner + sessions strip), `job_detail`
  (expected-artifacts + process-evidence), `artifact_view`
  (provenance trail), `run_posture_verdicts` (override
  visual distinction), `doctor` (per-record recipes),
  `view_file` (breadcrumb). New partials.
- **dogfood-056** (RFC 0050 V2) scaffolded + validated:
  `recovery-panel` island, `override_verdict.js` modal,
  `copy_on_click.js`, `workflow-graph-editor`
  `require_attested_lane` data binding (no viewport overlay
  pending reactflow v12).

Both ready to kick off the moment their predecessor lands.

## v1.45.0 — 2026-05-14

### Added — RFC 0050 V1 prerequisites

Unblocks the `dogfood-054` UI rework run (RFC 0050 V1). The
implementation work happens in a follow-up dogfood; this release
ships only the prerequisites the design handoff (`docs/design/UI_REWORK.md`)
calls out as blocking-for-acceptance.

- **Version drift fix.** `src/striatum/__init__.py::__version__`
  was hardcoded `"1.37.0"` and never bumped with `pyproject.toml`,
  so `striatum --version` reported 1.37.0 while pip showed v1.44.1.
  Now derived from `importlib.metadata.version("striatum-orchestrator")`
  — single source of truth, drift eliminated.
- **OQ-4 — V1.41 burn-down verbs in `next_actions`.**
  `src/striatum/cli/introspect.py::next_actions` emits three new
  deterministic action names so the `dashboard --once` ↔ web
  parity tests (UI_REWORK.md §9.9 + §9.10) can read a single
  source of truth:
  - `inspect_packet_with_inbox` — surfaces whenever a packet is
    claimable; signals the operator should run `striatum inbox`.
  - `derive_expected_byline` — surfaces alongside any verdict
    override or checkpoint resolution; signals `striatum byline`.
  - `recovery_auto_publish` — surfaces when `has_stale_leases=True`;
    signals the V1.41 stale-lease auto-publish sweep would
    self-heal at least one job.
  - New `_has_stale_leases_with_on_disk_artifacts` helper does
    the cheap precheck (existence of `expected_artifacts[].path`
    on disk; the auto-publish call itself enforces full byline
    conformance).
- **RFC 0050.** New RFC adopting `docs/design/UI_REWORK.md` as
  the canonical UI spec; three-phase landing plan (V1 / V1.5 /
  V2). Skips the standard design-ceremony triple because the
  handoff IS the design output.

### Regression tests

- `tests/test_next_actions_v141_burndown.py` — 6/6 pass pinning
  the new action names, conditions, ordering, and dedup behavior.

## v1.44.1 — 2026-05-13

### Fixed — GH #8: v16 runs rebuild leaves runs_new residue

Engram operator runs on 2026-05-13 hit a real bug in the v1.44.0 v16
migration: the SQLite rebuild ran with `PRAGMA foreign_keys = ON`,
the `DROP TABLE runs` step failed because other tables reference
`runs`, and `runs_new` was left behind. Every subsequent CLI command
then failed with `table runs_new already exists`.

Fix:
- `_apply_v16_decision_propagation` now routes through the existing
  `rebuild_table` helper, which toggles `PRAGMA foreign_keys` around
  the rebuild and `DROP TABLE IF EXISTS` any prior temp-table
  residue. Operator-side checkouts hit the GH #8 wedge had to apply
  the same patch locally before commands recovered.
- `tests/test_gh8_v16_rebuild_idempotent.py` pins both halves:
  (1) a clean v16 leaves no `runs_new` behind; (2) a DB with the
  post-failure residue migrates cleanly on the second attempt.

Affected production runs (per GH #8):
- RFC0038 UI rework run_468b22aff5e54a9280a867d3c81314e6
- RFC0044 tenant isolation run_322110269dfb4ec98fc6f7ea818448c0

## v1.44.0 — 2026-05-13

### Added — RFC 0047 V1: decision-record propagation (closes GH #3)

`striatum decision record --outcome rejected` now propagates the
rejection to first-class surfaces. Downstream consumers no longer
have to walk the events table looking for `decision.recorded` —
status, why, dashboard, and evidence export all read the projection.

- **Schema migration v16** (`src/striatum/migrations.py`):
  - `runs.state` CHECK widened to include `compromised`. Table
    rebuilt in place via the standard SQLite drop-and-recreate idiom.
  - `verdicts.superseded_by_decision_id` + `superseded_at` columns
    added. NULL = not superseded; non-null = superseded by the named
    decision at the named time.
- **Propagation** (`src/striatum/cli/mutations.py::_propagate_decision_outcome`):
  - `outcome=rejected` against a non-compromised run → flips
    `runs.state` to `compromised`, marks every accepting verdict
    (`accept`, `accept_with_findings`) as superseded by the
    decision id, emits a `run.compromised` event with the
    superseded-verdict count.
  - `outcome=accepted` against a compromised run → reopens to
    `completed`, emits `run.reopened_after_compromised`. Existing
    verdict supersession trail is preserved (the rejection
    history stays in the audit chain).
  - `outcome=rejected` against an already-compromised run is a
    no-op (no extra event emitted).
  - `outcome=accepted_with_follow_up` and `outcome=accepted` against
    a non-compromised run do not change run state — the follow-up
    is tracked through the existing decision artifact + event.
- **Idempotency:** re-running the same outcome against a run already
  in that state is a no-op.
- **Audit chain:** the existing `decision.recorded` event stays
  authoritative; the new `run.compromised` /
  `run.reopened_after_compromised` events extend the audit-chain
  payload shape but not its hashing strategy.

### Regression tests

- `tests/test_decision_propagation.py` — 7/7 pass.
  - Migration v16 admits `compromised` in CHECK + adds supersession
    columns.
  - Rejected propagates + supersedes accepting verdicts + emits
    event.
  - Rejected against compromised is a no-op.
  - Accepted reopens compromised → completed; supersession trail
    preserved.
  - Accepted against completed run is a no-op.
  - Accepted_with_follow_up does not change state.

### Backlog after v1.44.0

- **GH #2** (operator-asserted lane attestation): broader trust-model
  framing concern. The V1 lane evidence guard (RFC 0046, v1.43.0)
  significantly reduces the practical attack surface; full closure
  needs RFC 0046 V1.7 (path-specific check) + RFC 0048 Phase B
  (Go-core attestation).
- **RFC 0046 V1.7 polish:** add `observed_output_paths_json` to the
  `process_executions` schema; tighten `_lane_evidence_present` to
  path-specific. Web UI `LaneEvidenceChip` + dashboard `evid:`
  column.
- **RFC 0048 V2.0 phase:** the substrate flip — port single-repo
  business logic to PG-backed daemon-internal handlers + Go core
  parity + remove the TEST_HARNESS escape.

## v1.43.0 — 2026-05-13

### Added — V1.7 backlog batch

Three RFCs drafted (0046, 0047, 0048) and one V1 implementation
landed (RFC 0046). Two surgical V1.7 fixes shipped alongside (RFC
0039 V1.7 macOS reader + PointerStore boot wire-up, GH #6 reactflow
ViewportPortal removal). Dogfood-053 ran the RFC 0046 V1 ceremony
and the new lane evidence guard self-validated by refusing the
operator-on-behalf publish until the override flag was supplied.

#### RFC 0046 V1 — Lane evidence guard at publish-artifact (closes GH #2 + #5)

- `src/striatum/migrations.py` v15: new
  `attestation_override_rationale TEXT` column on `artifacts`.
- `src/striatum/artifacts.py::publish_artifact`: if the resolved
  byline is a model byline (not `author: operator [...]`), refuse
  publish when the session has no completed exit-0
  `process_executions` row. New helpers `_is_operator_byline` and
  `_lane_evidence_present`.
- `src/striatum/cli/parser.py` + `dispatch.py`:
  `publish-artifact --allow-no-process-execution
  --override-rationale "<text>"` operator opt-in. Empty rationale
  refuses with exit code 2. `submit-review` gets the same pair of
  flags so operator-composed reviews can also flow.
- New `provenance.publish_without_process_execution` event emitted
  on every override, carrying byline + path + rationale.
- Self-validated by dogfood-053: the operator's publish-on-behalf
  of the implementer HANDOFF was refused with
  `lane_evidence_missing` until `--allow-no-process-execution
  --override-rationale "..."` was supplied.

#### RFC 0039 V1.7 — macOS pid reader + PointerStore wire-up

- `go/pkg/supervisor/start_time_{linux,darwin,other}.go` split the
  per-OS readers via build tags. darwin uses
  `/bin/ps -o lstart= -p <pid>`; non-Linux/darwin returns
  `(_, false)` so the caller falls back to signal-0 only.
- `go/pkg/db/connection.go::Pool.RawPool *pgxpool.Pool` exposes the
  underlying pool to consumers needing typed access (e.g. the
  supervisor pointer store).
- `go/cmd/striatumd/main.go` constructs `SupervisorPointerStore` at
  boot with a `supervisor.PointerStore`-conformant adapter
  (`db.PointerRow ↔ supervisor.PointerRow`). The not_implemented
  handlers stay; RFC 0048 Phase B will wire the actual handler
  ports.

#### GH #6 — reactflow ViewportPortal fix

- `WorkflowGraphEditor.tsx` removes the v12-only `ViewportPortal`
  import and returns `null` from `PhaseBands` with a comment
  pointing to the V1.5 polish backlog. `make ui-build` now produces
  real Vite output (6KB–622KB bundles, not 50–75 byte placeholders).
  `make ui-verify-bundle` passes.

#### RFCs drafted for the rest of the backlog

- **RFC 0046** (V1.7) Lane evidence guard at publish-artifact —
  V1 landed in this release; V1.7 follow-up scope tracked.
- **RFC 0047** (V1.8) Decision-record propagation +
  `runs.state = compromised` (GH #3). Schema migration, byline
  rewrite path, status/why surface changes scoped for next.
- **RFC 0048** (V2.0 phase) Daemon-side substrate migration. Three
  phases: A) PG-backed Python handlers; B) Go core parity;
  C) remove the `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1`
  escape entirely.

### Tests

- `tests/test_lane_evidence_guard.py` — 6/6 pass
  (`_is_operator_byline`, `_lane_evidence_present`, migration v15
  shape).
- 77/77 pass in the broader unit-test slice.
- `make lint` + `make typecheck` clean across the touched files.

## v1.42.0 — 2026-05-13

### Fixed — GH #7: process-adapter post-completion blocker

Closes the recurring "adapter session naturally completed, then exited
nonzero, blocker stuck on terminal job" pattern.

- `evaluate_and_block_inline` in `src/striatum/process_completion.py`
  now short-circuits when the job state is already terminal
  (`completed`, `failed`, `canceled`, `skipped`). The nonzero exit is
  treated as a benign trailing signal from the supervised process,
  not a workflow failure.
- `recovery resume --force` in `src/striatum/cli/recovery.py` now
  dismisses a legacy open process-adapter blocker against a terminal
  job as a no-op (resolves the blocker, preserves terminal state,
  emits `recovery.blocker_dismissed_terminal` event). Closes the
  "no current lease" recovery dead-end on already-affected runs.
- Regression: `tests/test_gh7_terminal_blocker.py` pins the guard
  for all four terminal states.

### Backlog (triaged from open GH issues)

The remaining open issues need design work beyond this surgical
release. Each is documented for the next session:

- **GH #2** (operator-asserted lane treated as attested): the byline
  path already differentiates via `attestation.attested` →
  `operator_author_line`, so unattested sessions DO get `author:
  operator`. Investigation: confirm whether attestation drift
  mid-flight can re-introduce model bylines. If so, treat as a
  regression here. If not, the issue is downstream consumer
  documentation, not byline forgery.
- **GH #3** (decision record propagation): needs a `run.state =
  'compromised'` enum value, byline-rewrite path, and status/why
  surface changes. Multi-file design. V1.42 documents the gap;
  V1.7 implements.
- **GH #5** (publish-artifact without process_execution): related
  to GH #2; add a publish-time guard event
  `provenance.publish_without_process_execution` and an
  `--allow-no-process-execution` opt-in. V1.7.
- **GH #6** (web UI placeholder bundles): mechanical — `make
  ui-build`, commit real Vite output. Blocked by node/npm
  availability in the operator environment. V1.7.

## v1.41.0 — 2026-05-13

### Added — harness friction burn-down

Closes the recurring operator-on-behalf frictions observed across
dogfoods 048-052 (claude-no-explicit-publish 6+ instances, gemini
byline-drift 4 instances, override-fresh-session dance, etc.). No
new dogfood ceremony — this *is* the burn-down.

- **A1 — `striatum recovery auto-publish --run-id`** (`src/striatum/cli/recovery.py`).
  Walks stale leases. For each, if the work-packet's `expected_artifacts[].path`
  is present on disk and the on-disk byline canonicalises exactly to the
  `expected_author_line`, auto-runs ack + publish-artifact + complete on
  behalf of the dead session. Two-condition gate (byline + path) prevents
  misfiring. Dry-run mode reports without writing.
- **A2 — front-matter author wins** (`src/striatum/artifacts.py`).
  `markdown_title_block_author_lines` returns front-matter author lines
  exclusively when present; in title block, only the *first* canonical
  byline counts. Closes the gemini `Author: <real-name>` body-mention
  competing pattern.
- **A3 — `publish-artifact` defaults from `expected_artifacts`**
  (`src/striatum/cli/dispatch.py::_resolve_publish_defaults`). When
  `--path` matches a declared `expected_artifacts[].path` and only one
  declared artifact matches, `--kind` and `--logical-name` default from
  the workflow. Ambiguity errors list declared paths.
- **A4 — `striatum byline --session-id --job-id`**. Prints the exact
  `expected_author_line`; replaces the manual python -c spelunking.
- **C1 — `striatum inbox --session-id`**. Prints the current packet's
  ids + expected artifacts + byline; replaces the multi-step `striatum
  why <sid> --json` parsing operators were doing.
- **B1 — `override-verdict --auto-fresh-session`**. When the supplied
  session already has a verdict for the job (so override-verdict would
  refuse), the flag registers a fresh operator reviewer session on the
  same lane and uses it. Removes the manual two-step dance.

### Regression tests

- `tests/test_harness_friction_burndown.py` — front-matter-wins scanner
  + canonical byline form.
- `tests/exit_codes/test_rfc0043_split_brain.py` — `db.connect` refuses
  fresh SQLite when sentinel/tombstone present.
- `tests/daemon_pg/test_repo_local_migration_locking.py` — concurrent
  `migrate-repo-local` refuses with exit code 8.
- `tests/cli/test_parser_help.py` — per-flag help on
  `daemon migrate-repo-local`.

### Out of scope (still backlog)

- Default workflow-artifact-output path (TODO #30).
- `striatum self-update` (separate feature).
- Operator sub-agent workflows as first-class skill (memory item).
- Daemon-side substrate migration (RFC 0043 V2.0).

## v1.40.0 — 2026-05-13

### Added — RFC 0039 V1.6 Go daemon hardening (dogfood-051)

Closes the V1.6 follow-ups recorded in v1.39.0 across F-pty,
F-pid-recycling, F-perms, F-store, F-ci. Implementer slot was
operator-driven (recurring 5+-instance claude-no-publish anti-pattern;
harness backlog item).

- **F-pty** — `github.com/creack/pty v1.1.24` integrated into
  `go/go.mod` + `go/go.sum`. `go/pkg/supervisor/pty.go::launchPTY`
  uses `pty.Start(cmd)` returning the master fd as `StdinWriter`. The
  not-wired sentinel is removed; the supervisor test now asserts
  functional PTY launch against `/bin/true`.
- **F-pid-recycling** — `go/pkg/supervisor/liveness.go` adds
  `processAliveAtStartTime` + `readProcessStartTime` reading
  `/proc/<pid>/stat` field 22 plus `/proc/stat`'s `btime` with 2s
  tolerance. Liveness goroutine passes `row.StartedAt` on each tick.
  Non-Linux falls back to signal-0 only (V1.7 macOS path with
  `proc_pidinfo` / sysctl).
- **F-perms** — `go/pkg/supervisor/pointer.go` + `pty.go` scratch dir
  `0o700`, pidfile `0o600`, stdout/stderr fallback `0o600`.
- **F-store** — new `go/pkg/db/supervisor_pointers.go`:
  `SupervisorPointerStore{pool *pgxpool.Pool}` implementing
  `supervisor.PointerStore` (`Upsert` / `MarkLost` / `Get`) via UPSERT
  on `striatumd.process_supervisor_pointers`. Typed
  `ErrSupervisorNotFound` returned from `Get` and `MarkLost` when
  rows-affected is zero.
- **F-ci** — `.github/workflows/ci.yml` adds a "Verify Go binary
  present" step under `daemon-core == 'go'` that fails fast with
  `::error::` annotation if `go/bin/striatumd` is missing after
  `make daemon-go-build`. Closes dogfood-049 gemini F6 (CI matrix
  bypass risk).

### Added — RFC 0043 V1.6 substrate hardening (dogfood-052)

Closes the V1.6 follow-ups recorded in v1.38.0 across F-escape,
F-split-brain, F-lock, F-help. Gemini A1 (daemon-side substrate
migration) **stays deferred to V2.0** as a separate phase RFC.

- **F-escape** — `src/striatum/cli/daemon_required.py`:
  `resolve_requirement` opt-out now requires
  `STRIATUM_DAEMON_REQUIRED == "0"` **and**
  `STRIATUM_TEST_HARNESS == "1"`. The bare env var no longer
  bypasses production enforcement. `tests/conftest.py` exports both.
  Closes codex dogfood-050 threat-model finding.
- **F-split-brain** — `src/striatum/db.connect`: before creating a
  fresh SQLite (file absent), checks for sentinel
  `.striatum/state.sqlite3.migrated` OR tombstone
  `.striatum/state.sqlite3.tombstone`. Raises `StriatumError(exit_code=12)`
  with `repo_not_migrated` remediation text. Closes gemini A2.
- **F-lock** —
  `src/striatum/daemon_pg/repo_local_migration.py`: new
  `MigrationInProgressError(StriatumError, exit_code=8)` and
  `_exclusive_migrate_lock(repo)` context manager taking a non-blocking
  exclusive `fcntl.flock` on `.striatum/state.sqlite3.migrate.lock`
  (sidecar — survives the source-file rename during finalization and
  does not fight SQLite's own POSIX byte-range locks). Refusal message
  names the source SQLite path. Exit code reuses the V1.5
  ``migrate-repo-local`` refusal code per the V1.6 design synthesis
  ("avoid introducing a new exit code for this narrow V1.6 slice").
  Closes gemini A3.
- **F-help** — `src/striatum/cli/parser.py` registers
  `description=` + `help=` on every `migrate-repo-local` flag
  (`--from`, `--to`, `--repo`, `--postgres-url`, `--dry-run`,
  `--confirm-delete`, `--keep-sqlite-readonly`,
  `--no-keep-sqlite-readonly`, `--json`). Closes claude
  dogfood-050 F-dx-1.

### Known follow-ups

- **V1.7 (RFC 0039):** macOS process start-time reader (proc_pidinfo
  / sysctl); wire Postgres-backed `SupervisorPointerStore` into
  `cmd/striatumd/main.go` boot path.
- **V2.0 (RFC 0043):** daemon-side single-repo business logic on
  Postgres (gemini A1) — full substrate flip at the daemon RPC
  business-logic layer.

## v1.39.0 — 2026-05-13

### Added

- RFC 0039 Phase 2 — Go daemon completion (Steps 3-6) landed under
  dogfood-049 as a two-track split (Track A codex / Track B claude).
  The Go daemon now ships in the wheel, is selectable via
  `striatum daemon start --core go`, exposes the RFC 0043 method
  vocabulary, and has a Go-side supervisor lifecycle scaffold.

  Track A (codex implementer, 90% natural):
  - `go/pkg/rpc/registry.go` expanded to the RFC 0043 canonical dotted
    method vocabulary (`session.register`, `work.*`, `artifact.publish`,
    `review.*`, `decision.record`, `checkpoint.resolve`, `recovery.*`,
    `worktree.*`, `branch.confirm`, `run.*`, `workflow.*`). Legacy
    undotted aliases registered + deprecated.
  - `go/pkg/apply/{receipt.go,service.go}` — apply receipt lookup +
    fail-closed sealed-apply skeleton (cryptographic verification is
    V1.6 follow-up per gemini F2).
  - `go/pkg/mcp/{capabilities.go,tools.go}` — capability-filtered tool
    visibility + `tools/call` dispatch through Go RPC server.
  - `go/pkg/crossrepo/{prepare.go,lifecycle.go}` — cross-repo lifecycle
    helpers over Postgres.
  - `go/cmd/striatumd/main.go` wired to register apply + cross-repo
    handlers + stable fail-closed handlers for the broader mutation
    surface (deterministic `not_implemented` instead of
    `method_unknown`).
  - `src/striatum/cli/daemon.py` — `daemon start --core {python,go}`
    with `STRIATUM_DAEMON_CORE` env default. Go binary resolver order:
    packaged `_daemongo` → `STRIATUMD_GO_BIN` → `go/bin/striatumd` →
    PATH.
  - `src/striatum/cli/parser.py` — `--core` flag on `daemon start`.

  Track B (claude implementer stalled, operator-driven):
  - `go/pkg/supervisor/pointer.go` — `PointerStore` interface +
    `PointerRow` mirroring `striatumd.process_supervisor_pointers`;
    atomic pidfile write under `<scratch>/<supervisor_id>/pid`.
  - `go/pkg/supervisor/liveness.go` — heartbeat goroutine + dead-PID
    detection via signal-0 probe + SIGTERM-with-grace cleanup. Defaults
    5s heartbeat / 30s lost-after / 5s grace-on-term match the Python
    supervisor.
  - `go/pkg/supervisor/pty.go` — `LaunchSpec` + non-PTY (pipe) launch
    path. **PTY branch returns "not wired" sentinel error** — the
    `creack/pty` integration is V1.6 follow-up.
  - `go/pkg/supervisor/supervisor_test.go` — table-driven tests
    (pidfile round-trip, dead-pid lost-detection, empty-command
    rejection, pipe-mode `/bin/true` launch).
  - `go/Makefile` — `release-{linux,darwin}-{amd64,arm64}` targets
    with `CGO_ENABLED=0`.
  - Top-level `Makefile` — `daemon-go-install` (host-only) and
    `daemon-go-release` (cross-compile + stage under
    `src/striatum/_daemongo/binaries/`).
  - `src/striatum/_daemongo/__init__.py` — `find_binary()` /
    `platform_slug()` package-data resolver. Returns `None` on sdist
    or missing platforms; CLI falls through to `STRIATUMD_GO_BIN`.
  - `pyproject.toml` — `"striatum._daemongo" = ["binaries/*"]` under
    `[tool.setuptools.package-data]`.
  - `MANIFEST.in` — `recursive-include src/striatum/_daemongo *`.
  - `.github/workflows/ci.yml` — `daemon-core: ["python", "go"]`
    matrix axis as explicit jobs (not in-process parametrization);
    `STRIATUM_MULTI_REPO_REQUIRE_PG=1` sentinel against all-skipped
    pass (closes dogfood-047 F3).
  - `.github/workflows/release.yml` — early `make daemon-go-release`
    step + `striatumd-binaries` upload artifact + wheel ships binaries
    via package-data.
  - `tests/test_daemon_go_supervisor.py` — Python harness scaffold;
    functional FIFO/heartbeat/SIGTERM assertions deferred to V1.6
    pending PTY landing.

  Inline operator fix during review phase:
  - `src/striatum/cli/dispatch.py:888-890` rewired from
    `run_daemon_foreground(...)` direct call to
    `launch_daemon_start(args)`. Closes F1 from both codex and claude
    build reviews (`--core go` was silently inert pre-fix).

### Known follow-ups (V1.6)

- **Full PTY integration on Go supervisor** — fold `creack/pty` into
  `go.mod`, wire the PTY branch, replace harness scaffold with
  functional assertions.
- **Full Go mutation handler suite** — implement every registered RPC
  method against Postgres-backed repo-local schema (currently most
  return `not_implemented`).
- **Apply-receipt cryptographic verification** — replace lookup-only
  `apply.VerifyReceipt` with signature check (gemini F2).
- **PID-recycling protection** — pair signal-0 probe with
  `/proc/<pid>/stat` start-time check (gemini F1).
- **Tighten scratch-dir perms** to 0700 / 0600 (gemini F3).
- **`STRIATUM_DAEMON_CORE` operator-clarity** — warn/refuse when env
  disagrees with explicit `--core` flag (gemini F5).
- **CI hard-fail on missing Go binary** when `daemon-core=go`
  (gemini F6).
- **Concrete Postgres-backed `PointerStore`** under
  `go/pkg/db/supervisor_pointers.go`.

## v1.38.0 — 2026-05-13

### Added

- RFC 0043 V1.5 — D102 follow-up findings closure under dogfood-050.
  Single-track claude implementer (deliberately not codex per D102
  anti-pattern note). Four named findings closed:
  - **F-crash:** Transactional rollback + checkpointed resume of
    `striatum daemon migrate-repo-local` after a kill-9 between
    Postgres commit and SQLite finalization. Adds atomic `.migrated`
    sentinel write after commit and before tombstone/delete; resume
    helper re-enters from the early-return path on rerun.
    (`src/striatum/daemon_pg/repo_local_migration.py`,
    `tests/daemon_pg/test_repo_local_migration_crash_resume.py`.)
  - **F-escape:** `STRIATUM_DAEMON_REQUIRED` default flip — unset env
    now enforces daemon-required; only `STRIATUM_DAEMON_REQUIRED == "0"`
    opts out. `resolve_requirement` in
    `src/striatum/cli/daemon_required.py` returns enforcement by
    default; per-command optional list and explicit-zero remain the
    only bypass surfaces.
  - **F-parser:** `striatum daemon migrate-repo-local` subcommand
    wired into argparse + dispatch end-to-end
    (`src/striatum/cli/parser.py:167-199`,
    `src/striatum/cli/dispatch.py:881-887`,
    `src/striatum/cli/daemon.py:24-44`).
  - **F-test:** Exit-code-12 (`repo_not_migrated`) e2e regression
    against real dispatch
    (`tests/exit_codes/test_rfc0043_refusals.py:207-243`) — runs
    `dispatch.main(["--repo", str(tmp), "status"])` against a tmp
    repo with a `.striatum/state.sqlite3` plus listening daemon
    socket, asserts rc == 12 and that the remediation line names
    `striatum daemon migrate-repo-local --from sqlite --to pg --repo`.

### Known follow-ups (V1.6)

- Codex threat-model finding: `STRIATUM_DAEMON_REQUIRED=0` is still
  documented as an operator migration path. V1.6 will remove the
  runtime escape entirely (test-only gating or removal).
- Gemini adversarial findings A1/A2/A3:
  - A1 (critical): server-side substrate mismatch — daemon RPC
    delegates single-repo verbs back to SQLite-backed CLI logic;
    actual substrate flip is incomplete at the daemon business-logic
    layer. V1.6 will port daemon-internal single-repo logic onto
    Postgres directly.
  - A2 (high): split-brain — `striatum.db.connect` creates a fresh
    SQLite when the file is missing post-migration. V1.6 will refuse
    to create when a migration checkpoint exists.
  - A3 (medium): no exclusive lock on the source SQLite during
    migrate-repo-local. V1.6 will add explicit locking.
- Claude ergonomics finding F-dx-1: per-flag help text on
  `migrate-repo-local` is sparse (only two flags carry `help=`).

## v1.37.0 — 2026-05-13

### Added

- RFC 0043 V1 — Postgres as Sole Substrate + Daemon-Required Runtime
  landed under dogfood-048. Per D094, supersedes the local-SQLite
  assumption in D006/D007/D036 and the SQLite half of D009. The
  substrate flip lands on a two-track split so schema and CLI surface
  could proceed in parallel once the shared design synthesis fixed the
  schema name and method vocabulary.
  - **15 repo-local workflow tables in daemon-owned Postgres.**
    `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`
    creates the full repo-local workflow surface under the existing
    `striatumd.*` schema with `repository_id text NOT NULL REFERENCES
    striatumd.repositories(repository_id)` on every repo-scoped table:
    `workflow_snapshots`, `runs`, `sessions`, `jobs`,
    `job_dependencies`, `queue_messages`, `leases`, `work_packets`,
    `artifacts`, `verdicts`, `blockers`, `command_requests`,
    `process_executions`, `events`, `job_worktrees`,
    `process_supervisors`, `process_supervisor_pointers`. (The prompt
    named 15 tables; `workflow_snapshots` and `job_dependencies` are
    required structural tables in `src/striatum/schema.py` and were
    added by the synthesis to avoid breaking
    `runs.workflow_snapshot_id` and job gating.) Index strategy is
    repository-prefixed versions of the current SQLite access paths
    plus the partial-unique constraints from prior migrations
    (`leases(repository_id, resource_type, resource_id) WHERE state =
    'active'`, `queue_messages` partial unique on
    `(repository_id, job_id) WHERE kind = 'work' AND state IN
    ('pending','claimed','acked')`, `process_supervisor*` partial
    unique on `(repository_id, session_id) WHERE state IN
    ('starting','attached','detached')`, etc). Same SQL file creates
    `striatumd.repo_migrations` checkpoint table
    (`repository_id`, `source_substrate`, `target_substrate`,
    `source_user_version`, `source_event_manifest_sha256`,
    `source_artifact_manifest_sha256`, `source_state_db_sha256`,
    `migrated_at`, `tombstone_path`, `row_counts jsonb`) and installs
    append-only trigger functions on `events` and `artifacts`,
    revoking `UPDATE` / `DELETE` on those tables from the daemon
    runtime role. `src/striatum/daemon_pg/migrations.py` bumps
    `LATEST_DAEMON_DB_VERSION` from 4 to 5 and registers the migration
    as `PgMigration(5, "repo-local workflow state",
    "0005_repo_local_workflow_state.sql")`.
  - **`striatum daemon migrate-repo-local` migration verb** with
    `--from sqlite --to pg --repo <path> --postgres-url <url>
    [--dry-run] [--keep-sqlite-readonly] [--confirm-delete] [--json]`.
    Body in `src/striatum/daemon_pg/repo_local_migration.py`
    (separate from `cutover.py` so daemon-registry cutover and
    repo-local workflow cutover stay distinct):
    `RepoLocalMigrationOptions`, `migrate_repo_local()`, and
    `compute_repo_local_reanchor()`. Algorithm: authorize daemon
    admin → resolve or implicitly register the repository → refuse if
    a `repo_migrations` row already exists (returns
    `already_migrated: true`) → open `.striatum/state.sqlite3`
    read-only → verify `PRAGMA user_version ==
    striatum.migrations.LATEST_VERSION` → for full runs, copy every
    repo-scoped row in dependency order inside one `SERIALIZABLE`
    Postgres transaction → write the `repo_migrations` checkpoint
    inside the same transaction → commit → rename
    `.striatum/state.sqlite3 → state.sqlite3.tombstone` with mode
    `0444` (default `--keep-sqlite-readonly`). If
    `--no-keep-sqlite-readonly` is supplied, deletion still requires
    `--confirm-delete`; otherwise the command refuses with exit code
    8. Dry-run path applies pending daemon migrations if needed, then
    reports source counts and manifest hashes without inserting
    repo-local rows. `compute_repo_local_reanchor` defines the
    byte-equivalence check: canonical JSON arrays of source rows
    ordered by stable primary key for `events` and `artifacts`,
    projected to source-column names and compact UTF-8 JSON, SHA-256
    must match between SQLite and Postgres. Daemon-command helper at
    `src/striatum/cli/daemon.py` (Track A) — full parser wiring of
    the subparser deferred to V1.5.
  - **Exit code 11 `daemon_unreachable` + exit code 12
    `repo_not_migrated` with named remediation.** `src/striatum/errors.py`
    introduces `DaemonUnreachableError` and `RepoNotMigratedError`
    plus an `EXIT_*` integer constant table for codes 1–15;
    `src/striatum/cli/daemon_required.py` (new) defines
    `enforce_daemon_required(command, repo)` and the canonical
    stderr / JSON-envelope refusal shapes. Exit 11 stderr lists four
    remediation channels (Linux systemd: `systemctl --user start
    striatumd`; macOS launchd: `launchctl bootstrap gui/$UID
    ~/Library/LaunchAgents/io.striatum.striatumd.plist`; foreground:
    `striatumd --foreground`; Postgres: `striatum daemon doctor
    --postgres-url <url>` or `STRIATUM_DAEMON_DB_URL`). Exit 12
    stderr names the single fix (`striatum daemon migrate-repo-local
    --from sqlite --to pg --repo <path>`). JSON envelope under
    `--json` carries `{"ok": false, "error": {"message": "...",
    "code": 11|12, "hint": "..."}}`. Activation is currently
    env-gated on `STRIATUM_DAEMON_REQUIRED=1`; flipping the default
    to enforced is part of the V1.5 follow-up (closes the CLI escape
    path). `DAEMON_OPTIONAL_COMMANDS` allowlist (`daemon`, `init`,
    `skills`, `plugin`) keeps doctor and lifecycle commands reachable
    without a daemon (RFC 0043 §3 acceptance criterion). Legacy V1
    RFC 0028 daemon errors renumbered to free codes 11 and 12
    (`DaemonAuthError → 14`, `DaemonCapabilityError → 15`); the older
    `DaemonUnreachableError` from `src/striatum/daemon.py` stays at
    code 10 with a docstring pointing at the new entry-layer error.
    Tests assert daemon errors by class name, not numeric exit code,
    so no test fixture broke on renumbering.
  - **`--no-daemon` retired.** Removed from
    `src/striatum/cli/parser.py`'s daemon mutual-exclusion group; no
    hidden alias. Argparse now exits 2 with `unrecognized arguments:
    --no-daemon` for the retired flag. `--daemon` remains as the V1
    RFC 0028 read-mode opt-in until daemon-mediated CLI dispatch
    absorbs it. New `tests/cli/test_no_daemon_retired.py` covers the
    rejection plus `--help` absence assertion.
  - **`.striatum/state.sqlite3` retained read-only when
    `--keep-sqlite-readonly` is set** (mode `0444` tombstone at
    `.striatum/state.sqlite3.tombstone`); otherwise the
    `--confirm-delete` flag deletes the source DB after the
    checkpoint commits. Post-migration `.striatum/` survives as
    operational scratch only — FIFOs, pidfiles, supervisor stdout,
    token cache, marker files — never as the live message bus.
  - **RFC 0030 method registry expanded for repo-local mutations.**
    `src/striatum/daemon_rpc/registry.py::_ENTRIES` and
    `src/striatum/daemon_rpc/server.py::CLI_ROUTES` widened to cover
    every mutation in `src/striatum/cli/mutations.py` per RFC 0043
    §5. New dotted vocabulary: `session.register`, `session.close`,
    `work.claim_next`, `work.ack`, `work.heartbeat`, `work.complete`,
    `work.block`, `work.release`, `work.send_message`,
    `artifact.publish`, `review.submit`, `review.verdict`,
    `review.override`, `decision.record`, `checkpoint.resolve`,
    `branch.confirm`, `run.prepare`, `run.start`, `run.pause`,
    `run.resume`, `run.cancel`, `run.retry_job`, `worktree.create`,
    `worktree.release`, `worktree.list`, `recovery.stale_leases`,
    `recovery.requeue_stale`, `recovery.cancel_job`,
    `recovery.process_reconcile`, `recovery.resume`, `recovery.auto`,
    `recovery.watch`, `supervise.start`, `supervise.send`,
    `supervise.stop`, `supervise.status`, `supervise.list`,
    `supervise.reattach_status`, plus the `workflow.*` and read-side
    surface (`status`, `why`, `doctor`, `dashboard`, `dashboard.all`,
    `evidence.export`, `corpus.export`, `run.summary`, `run.graph`,
    `list.*`). Daemon-global additions: `repo.list`,
    `daemon.migrate_repo_local`. Legacy undotted names (`ack`,
    `heartbeat`, `release`, `block`, `complete`, `publish_artifact`,
    `claim_next`, `verdict`, `submit_review`) kept as
    `deprecated=True` entries so in-flight clients keep resolving
    while callers migrate. New `tests/daemon_rpc/test_registry_rfc0043_coverage.py`
    is the exhaustiveness test: static map of mutation function names
    → RFC 0043 §5 method names, asserts every mutation has a
    registered method, every method's required capability matches §5,
    every canonical method either routes via `CLI_ROUTES` or sits in
    the inline allowlist, legacy aliases are flagged
    `deprecated=True`, and repo-scope modes (single_repo /
    cross_repo / daemon_global) match the synthesis.
  - **D094 supersession of D006 / D007 / D036 / SQLite half of D009
    is now executable.** The local-SQLite assumption baked into those
    earlier decisions no longer holds for repo-local workflow state;
    `.striatum/state.sqlite3` is migration source or read-only
    tombstone only. RFC 0039's Go-core scope can now drop SQLite
    entirely (TODO item 25 marked unblocked).
  - **Files (uncommitted in this branch at merge time):** Track A —
    `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`
    (new), `src/striatum/daemon_pg/repo_local_migration.py` (new),
    `src/striatum/daemon_pg/migrations.py` (modified for v5
    registration), `src/striatum/cli/daemon.py` (new daemon-command
    helper), `tests/daemon_pg/test_repo_local_migration.py` (new),
    `tests/fixtures/v1_repo_local_sqlite/` (new SQLite fixture).
    Track B — `src/striatum/cli/dispatch.py` (modified for
    `enforce_daemon_required` hook + dedicated `DaemonUnreachableError /
    RepoNotMigratedError` except arm + retired `args.no_daemon`),
    `src/striatum/cli/parser.py` (modified for `--no-daemon` removal),
    `src/striatum/cli/daemon_required.py` (new),
    `src/striatum/daemon.py` (modified for V1 daemon error
    renumbering 11/12 → 14/15), `src/striatum/daemon_rpc/registry.py`
    (modified for §5 vocabulary expansion),
    `src/striatum/daemon_rpc/server.py` (modified for `CLI_ROUTES`
    expansion), `src/striatum/errors.py` (modified for new error
    classes + `EXIT_*` constants), `tests/cli/__init__.py`,
    `tests/cli/test_no_daemon_retired.py`,
    `tests/cli/test_daemon_doctor_without_daemon.py`,
    `tests/exit_codes/__init__.py`,
    `tests/exit_codes/test_rfc0043_refusals.py`,
    `tests/daemon_rpc/__init__.py`,
    `tests/daemon_rpc/test_registry_rfc0043_coverage.py` (all new).
    Handoffs at `docs/dogfood/048/build/track_a/HANDOFF.md` and
    `docs/dogfood/048/build/track_b/HANDOFF.md`; combined handoff at
    `docs/dogfood/048/BUILD_HANDOFF.md`; operator narrative at
    `docs/dogfood/048/PHASE_1_OPERATOR_NOTES.md`.

### Decided

- D102 (`dec_0b953435368e40109e793378e1a75054`,
  `accepted_with_follow_up`): cycle-exhaustion override for the
  dogfood-048 build review. Codex `review_build_codex` returned
  `needs_revision severity=high` and gemini `review_build_gemini`
  returned `needs_revision severity=medium` — both with real findings
  (crash-recovery persistence gap between Postgres commit and SQLite
  tombstone rename; CLI escape path remains under the env-gated
  enforcement default; `daemon migrate-repo-local` subcommand body
  exists in `daemon_pg/repo_local_migration.py` but the parser
  subparser is not yet wired). Single accepting verdict claude
  `accept_with_findings` low (cross-lane scope-met envelope).
  **D102 is distinct from D095-D101 in finding character.** Prior
  cycle-exhaustion overrides have fallen into two anti-pattern
  families: (a) codex/codex implementer+reviewer co-blindness
  (D095 dogfood-042 Track A, D096 dogfood-042 Track C, D097
  dogfood-043, D098 dogfood-044, D100 dogfood-046) where the
  reviewer's findings cluster around the implementer's same blind
  spots; (b) codex-reviewer-of-claude-implementer baseline
  conservatism (D099 dogfood-045 reject critical, D101 dogfood-047
  needs_revision high) where codex applies threat_model-posture
  conservatism to a different model's work. D102 belongs to neither —
  the codex/codex pairing in Track A and the gemini reviewer both
  produced real findings on real scope gaps that the operator
  acknowledged and folded into V1.5 (TODO item 31). Codex+gemini
  findings absorbed into RFC 0043 V1.5; ships at V1 because the
  in-scope substrate-flip correctness contract is met and the
  remaining deltas are operator-side wiring + crash-recovery
  hardening, not architectural defects. Two run-quality regressions
  surfaced and were operator-recovered: (1) **3rd
  `claude-no-artifact` instance** — claude reviewer's session
  composed no REVIEW.md artifact in `docs/dogfood/048/review/build/
  claude/`; operator composed the verdict on-behalf with attribution
  preserved. (2) **3rd `gemini-no-frontmatter` instance** — gemini
  reviewer's REVIEW.md was missing `striatum.finding.v1` front
  matter; operator-fixed inline. Operator also performed SQL surgery
  on `artifacts.logical_name` in the live `.striatum/state.sqlite3`
  because the on-behalf publish call had passed the wrong logical
  name during recovery (the artifact's underlying file path was
  correct but the `logical_name` column needed a one-row UPDATE to
  align with the workflow's `expected_artifacts[]` entry). All three
  recurrences are now well-characterized enough that the operator
  recovery scripts under `striatum recovery resume / surgical_recovery`
  should grow targeted helpers for them in a future pass.



### Added

- RFC 0039 V1.5 — Go daemon correctness slice F1-F5 landed under
  dogfood-047. Implementation order respected the synthesis lock
  **F5 → F4 → F1 → F2 → F3** because F4 and F1 needed F5's
  parameter-binding and transaction support before they could land.
  - **F5 — Pure-Go PostgreSQL driver.** `go/pkg/db/connection.go`
    rewritten on top of `github.com/jackc/pgx/v5 v5.7.2` — the Go
    daemon's first third-party runtime dependency. New `db.Runner`
    and `db.TxRunner` interfaces expose parameterized `Exec`,
    `QueryRow`, `QueryScalar`, and (Runner-only) `BeginTx`;
    `PgxRunner` and `PgxTxRunner` are the concrete adapters; `db.Row`
    is a type alias for `pgx.Row` so `rpc` can reference the row
    type without an import cycle. Pool configured with
    `application_name = "striatumd-go/<daemon_version>"`, default
    `statement_timeout = 60000`, and
    `DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol` (simple
    protocol is required because existing migration files contain
    multi-statement DDL; pgx still binds parameters with safe
    client-side quoting under simple protocol, so the SQL-injection
    surface is unchanged). `PsqlRunner`, `exec.Command("psql", ...)`,
    and `fmt.Sprintf` literal interpolation are removed from
    production code paths. `RedactURL` and `ResolveConfig` keep their
    existing contracts; no code path logs raw Postgres URLs.
  - **F4 — Transactional audit append.**
    `go/pkg/db/audit.go::AuditRecorder.RecordRPC` opens one
    `READ COMMITTED` transaction via the F5 runner, locks the
    singleton `striatumd.audit_chain_head` row with
    `SELECT ... FOR UPDATE`, derives the open audit segment
    (creating one only if absent — `0001_baseline.sql` bootstraps an
    open segment so the create branch is dead in practice but
    defends against operator-side cleanup that closes the open
    segment without opening a new one), computes the v2 row hash
    from the locked `previous_hash`, inserts the audit row with
    `INSERT ... RETURNING audit_id`, updates
    `striatumd.audit_chain_head` to the new id and hash, commits,
    and returns the audit id as `strconv.FormatInt`. Rollback fires
    from a deferred function whenever Commit was not reached.
    Public API of `RecordRPC` is unchanged so
    `go/pkg/rpc/server.go` keeps calling it after response
    construction, and the returned `audit_id` flows into the RFC
    0030 response envelope — closing the V1 envelope-shape
    regression where the Go core returned empty `audit_id` to
    clients. Row-hash payload matches the Python `v2_row_hash`
    byte-for-byte: nullable strings encode as JSON `null`,
    `exit_code` is an int when present, `segment_id` is an int64,
    `ts` is RFC3339 truncated to the second.
  - **F1 — Postgres-backed RPC authorization (replaces
    `AllowAllAuthorizer` in production).**
    `go/pkg/rpc/auth_pg.go` introduces `PostgresAuthorizer`. Token
    secrets are HMAC-SHA256(`token_salt`, supplied secret) compared
    with `subtle.ConstantTimeCompare` against the stored
    `token_hash`; capability lookup mirrors
    `src/striatum/daemon_rpc/capability.py` exactly (same WHERE
    clause, same wildcard ordering, same scope-mismatch fallback
    query); revocation and expiry take effect on the next request;
    no positive or negative cache ships in V1.5. The denial-reason
    vocabulary is identical to the Python authorizer so clients
    cannot tell the two cores apart from the refusal envelope.
    `go/cmd/striatumd/main.go` wires
    `&rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}`
    whenever a Postgres URL is configured. `AllowAllAuthorizer{}`
    is now strictly the test default. Implementation deviates from
    the synthesis field type to keep `rpc → db → rpc` from becoming
    a cycle: `auth_pg.go` declares a local `rpc.AuthQuerier`
    interface using `pgx.Row`; `db.Runner` satisfies it
    structurally, so `main.go` still passes `pool.Runner` directly.
  - **F2 — Go harness launch contract.**
    `go/cmd/striatumd/main.go` accepts the synthesis-locked flag
    surface: `--socket`, `--postgres-url`, `--migrate`,
    `--describe`, and the new optional
    `--migrations-sha-source` which compares embedded migration
    file hashes against the SQL files at the supplied path before
    serving and exits non-zero on drift (replaces V1's
    `--migrations-dir` reloader without giving up the drift
    signal). `go/Makefile` writes the binary to `go/bin/striatumd`
    (V1 emitted `go/striatumd`, which the harness probed at
    `go/bin/striatumd` — fixed). `tests/_harness/daemon.py` builds
    via `make -C go build` when the binary is missing and honors
    the `STRIATUMD_GO_BIN` developer override;
    `_start_go` launches with the locked argv
    `--socket <sock> --postgres-url <url> --migrations-sha-source
    src/striatum/daemon_pg/sql` (no `--db-url`, no
    `--migrations-dir`). The narrow launch regression is
    `tests/test_daemon_go_smoke.py`: constructs
    `MultiRepoHarness(daemon_core="go")`, asserts the socket
    exists, runs `daemon.hello` and `daemon.describe`, and verifies
    the audit chain head moved.
  - **F3 — `make test-multi-repo CORE=go` wired + pytest
    parametrization.** Top-level `Makefile` exposes
    `CORE ?= python` and forwards it as
    `STRIATUM_MULTI_REPO_DAEMON_CORE` into pytest;
    `tests/conftest.py` adds a class-scoped `daemon_core` fixture
    that reads `STRIATUM_MULTI_REPO_DAEMON_CORE` (raising
    `pytest.UsageError` on unknown values) and threads it through
    `MultiRepoHarness`. New tests
    `tests/test_daemon_go_smoke.py` and
    `tests/test_daemon_go_audit.py` join the `test-multi-repo`
    target list; both skip when
    `STRIATUM_MULTI_REPO_DAEMON_CORE != "go"` so they do not break
    `CORE=python` runs. CI shape is the synthesis-locked **two
    explicit jobs** (`make test-multi-repo CORE=python` and
    `make test-multi-repo CORE=go`) rather than in-process pytest
    parametrization — Go-core failures surface as separately-named
    jobs rather than as parametrized subtests.
  - Files: `go/cmd/striatumd/main.go`, `go/pkg/db/audit.go`,
    `go/pkg/db/connection.go`, `go/pkg/db/migrations.go`,
    `go/pkg/db/migrations_test.go`, `go/pkg/db/audit_race_test.go`
    (new, opt-in on `STRIATUM_PG_TEST_URL`),
    `go/pkg/rpc/auth_pg.go` (new), `go/Makefile`, `go/go.mod`,
    `tests/_harness/daemon.py`, `tests/conftest.py`, `Makefile`,
    `tests/test_daemon_go_smoke.py` (new),
    `tests/test_daemon_go_audit.py` (new),
    `docs/rfcs/0039-go-daemon-core.md` (V1.5 deltas section).
- Operator-side ergonomics: `striatum --version` flag — prints
  `striatum <version>` and exits zero; wired in
  `src/striatum/cli/parser.py`. Separate from the V1.5 packet but
  rides along on the `striatum/dogfood-047-rfc-0039-v1-5` branch.
- Item 63 (TODO sweep results): items 3, 14, 18 promoted to
  ✅ done after the snapshot table review; items 1, 2, 13 retain
  🟡 most done status with named gaps captured in the per-item
  bodies (item 1 PTY path; item 2 sandbox/worktree adapter for
  mechanical `network`/`repo_scope` enforcement promotion; item 13
  runner-owned design+build+review fixture under `examples/`).
- Item 13 partial: `examples/three-lane-design-build-review/`
  runner-owned workflow fixture scaffolded (workflow.json, roles,
  prompts, README) reproducing the historical P001 three-lane
  shape against the standalone product surface — last operator
  step before the tmux harness fully retires from active workflow
  guidance.
- Pre-scaffold: `docs/dogfood/048/` (workflow.json, roles,
  prompts, OPERATOR_REPORT.md skeleton) staged for RFC 0043 V1
  (2-track: codex schema/migration + claude CLI/RPC). Not started
  in this packet; rides along on the branch so the next dogfood
  has the directory structure ready.

### Decided

- D101 (`dec_f8d268f392ca44dd8a9bccb634249979`,
  `accepted_with_follow_up`): override for the dogfood-047 build
  review. Codex `review_build_codex` returned `needs_revision
  severity=high` under the threat_model posture on five findings
  (F1 `go.sum` not regenerated for the new `pgx/v5` runtime
  dependency, F2 unauthenticated/no-audit production fallback when
  no `--postgres-url` is configured, F3 `make test-multi-repo
  CORE=go` can pass with all tests skipped, F4 smoke-test asserts
  no denial reason on unauthenticated `daemon.describe`, F5
  audit-append race regression not executable without
  `STRIATUM_PG_TEST_URL`). Cross-lane majority disagreed (claude
  `accept_with_findings` low ergonomics_dx, gemini
  `accept_with_findings` medium threat_model); 2-of-3 cross-lane
  consensus said scope was met. **D101 is distinct from D095-D100
  codex/codex co-blindness anti-pattern** — this dogfood
  deliberately routed implementation to **claude** (Go + Python
  harness mix), so the reviewer was scrutinizing a different model's
  work. This is the **codex-reviewer-of-claude-implementer pattern**
  first surfaced under D099 (dogfood-045, RFC 0038 V1.5): codex-as-
  reviewer baseline conservatism appears to be independent of the
  codex/codex convergent-blind-spot anti-pattern, and now has two
  instances on the books (D099 reject critical, D101
  needs_revision high). Codex findings F1-F5 are real but the
  V1.5 slice meets the in-scope correctness contract and ships;
  findings absorbed into RFC 0039 V1.6 follow-up (TODO item 30).

### Notes

- Dogfood-047 ran the multi-track design + build + review workflow
  for RFC 0039 V1.5 with the codex/codex anti-pattern explicitly
  avoided by routing implementation to claude. As with
  dogfood-044/045/046, the `consolidate` job was not part of the
  workflow; the operator authored this changelog entry,
  `docs/rfcs/README.md` status update, `docs/TODO.md` item-24
  promotion + new item 30 follow-up + F48 snapshot row,
  `docs/dogfood/047/BUILD_HANDOFF.md` (combined handoff per the
  consolidate-job-absent pattern), and
  `docs/dogfood/047/PHASE_1_OPERATOR_NOTES.md` out-of-band after
  the run.
- The codex/codex anti-pattern is now well-characterized across
  five instances (D095, D096, D097, D098, D100), and the
  codex-reviewer-of-claude-implementer pattern across two
  instances (D099, D101). The refuse-by-default validator rule for
  same-model implementer↔reviewer pairing (TODO item 26) remains
  deferred. For dogfood-047 the operator-side mitigation (route
  implementation to claude when the reviewer set includes codex
  with threat_model posture) is the same one used in dogfood-045,
  and produced the same outcome shape: codex reviewer comes back
  harsh on a different model's work; cross-lane majority overrides.
- The HANDOFF documents a verification gap on the implementer side:
  `striatum ack` and other Bash commands were denied by the harness
  permission gate, so no `make lint` / `make typecheck` / `make
  test` / `go test ./...` / `go mod tidy` / `make test-multi-repo`
  / binary smoke ran during the implementer session. The
  implement-prompt escape hatch ("If `striatum ack` is denied,
  write the HANDOFF and exit normally") governed the rest of the
  run. The codex review's F1 finding (`go.sum` not regenerated)
  follows directly from this gap: the `go.mod` was hand-edited
  with the canonical `pgx v5.7.2` line, but `go.sum` cryptographic
  hashes were not generated. Operator-side or CI follow-up: run
  `(cd go && go mod tidy)` and commit the resulting `go.sum`
  before merge so `make daemon-go-build` succeeds (folded into RFC
  0039 V1.6, TODO item 30).

## v1.35.0 — 2026-05-13

### Added

- RFC 0044 V1 — Striatum-side corpus export landed under dogfood-046.
  New `striatum corpus export --since <ref> --out <dir> [--json]` CLI
  verb wired in `src/striatum/cli/parser.py` and dispatched through
  `src/striatum/cli/dispatch.py`. New `src/striatum/corpus/` package
  splits the export into focused modules: `types.py`
  (`SUB_KINDS` / `JSONL_FILES` closed mapping for the nine JSONL
  bundle files; `CorpusBundleResult.to_json` shape with
  `status="exported"`, repo-relative `manifest_path`, `out`, `since`
  ref + resolved commit, `row_counts`, `bundle_sha256`), `git.py`
  (`resolve_commit` via `git rev-parse --verify <ref>^{commit}`),
  `enumerator.py` (durable-provenance source enumeration over RFCs,
  decisions, commits, operator reports, changelog, ubiquitous-language
  terms, harness-friction rows, run summaries; no SQLite blobs, no
  `FROM runs|FROM verdicts|FROM artifacts|FROM jobs|FROM sessions`
  queries from the live state DB), `redaction.py` (denylist-based
  source-path refusal for `.env`, `.env.local`, `keys/private.pem`,
  `.striatum/state.sqlite3`, `transcripts/`, `raw_model_output/`,
  `docs/transcript.txt`; co-author-email + 64-char-token scrubbing on
  commit messages; redaction policy enforces no-secrets/no-PII so the
  JSONL bundle is Engram-compatible), `writer.py` (deterministic
  JSONL emission with canonical UTF-8 newline normalization),
  `manifest.py` (per-file SHA-256 + row counts + repo HEAD +
  dirty-tree flag + `since` ref + schema version + `generated_at` —
  manifest hashes cover post-redaction bytes), and `export.py`
  (orchestrator that refuses `--out` outside the repo, under
  `.striatum/`, or pointing at a file; resolves `--since` before
  writing; verifies row counts and SHA-256s after emission; returns
  the standard CLI JSON envelope). Tests:
  `tests/test_corpus_enumerator.py`, `tests/test_corpus_redaction.py`,
  `tests/test_corpus_writer.py`, `tests/test_corpus_manifest.py`,
  `tests/test_cli_corpus_export.py` (incl.
  `test_corpus_export_cli_success_and_manifest`,
  `test_corpus_export_invalid_since_returns_json_error_code_8`,
  `test_corpus_export_rejects_bad_output_targets`,
  `test_no_engram_imports_or_memory_capabilities_in_striatum`),
  `tests/test_corpus_export_integration.py` (incl.
  `test_corpus_export_replays_with_stable_jsonl_hashes` — the
  RFC 0044 §3 acceptance test: byte-equality on JSONLs across two
  CLI invocations into different `--out` dirs, manifest equality
  after stripping `generated_at`). 31/31 corpus-targeted tests
  green; full suite 739 passed / 33 skipped with one pre-existing
  documentation-budget failure in `tests/test_doc_links.py` outside
  this packet's write scope. The corpus package is imported lazily
  from the dispatch branch so unrelated verbs do not pay its
  startup cost. Augmentation-not-dependency boundary is pinned by
  `test_no_engram_imports_or_memory_capabilities_in_striatum` —
  asserts `import engram` / `from engram` / `memory.` absent across
  `src/striatum/corpus/`, `src/striatum/cli/`,
  `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/`,
  `src/striatum/mcp.py`, `src/striatum/service.py`, and
  `pyproject.toml`. Scope was **Striatum-side ONLY**; the
  Engram-side ingester (`engram ingest-striatum`), the standalone
  `engram-mcp-stdio` MCP server, the four read-only retrieval tools
  (`engram.search`, `engram.fetch_reference`, `engram.describe_corpus`,
  `engram.health`), and the Engram-local `memory.*` capabilities are
  explicitly out of scope and live in `~/git/engram/` as a separate
  effort. Implementer was **codex** (Python) — 5th consecutive
  codex-as-implementer dogfood where the codex/codex reviewer
  pairing converged on its own findings (precedents D095 dogfood-042
  Track A, D096 dogfood-042 Track C, D097 dogfood-043, D098
  dogfood-044). Files:
  `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`,
  `src/striatum/corpus/__init__.py`, `src/striatum/corpus/types.py`,
  `src/striatum/corpus/git.py`, `src/striatum/corpus/enumerator.py`,
  `src/striatum/corpus/redaction.py`, `src/striatum/corpus/writer.py`,
  `src/striatum/corpus/manifest.py`, `src/striatum/corpus/export.py`,
  `tests/test_corpus_enumerator.py`, `tests/test_corpus_redaction.py`,
  `tests/test_corpus_writer.py`, `tests/test_corpus_manifest.py`,
  `tests/test_cli_corpus_export.py`,
  `tests/test_corpus_export_integration.py`, `tests/test_web_ui.py`
  (test-only `Traversable.read_text(errors=...)` → `read_bytes().
  decode(..., errors=...)` compatibility adjustment so `make
  typecheck` passes under the current `importlib.resources` typing
  surface).

### Decided

- D100 (`dec_b3b26d4c86df408ab75f4cf515a82d1e`,
  `accepted_with_follow_up`): cycle-exhaustion override for
  dogfood-046 build review. **5th codex/codex anti-pattern
  instance.** Codex `review_build_codex` returned
  `needs_revision severity=high` under the threat_model posture on
  redaction completeness + JSONL secret leakage. Gemini
  `review_build_gemini` returned `needs_revision severity=medium`
  under threat_model posture — but every gemini finding (A1
  contradictory capability spec, A2 lack of authorization in
  `fetch_reference`, A3 cross-repository context leakage via shared
  `corpus_id`, A4 redaction bypass in curated artifacts via memory
  poisoning, A5 `describe_corpus` metadata leakage) targeted the
  Engram-side surface (MCP server, ingester, capability model)
  which is **OUT OF SCOPE** for this dogfood — none of those
  components ship in `src/striatum/` this run. Claude
  `review_build_claude` returned `accept_with_findings severity=low`
  on the in-scope Striatum-side surface (ergonomics_dx posture:
  five discoverability findings F1-F5, all low, none blocking
  function). Single accepting verdict + 2 out-of-scope/anti-pattern
  needs_revisions; impl meets V1 scope acceptance criteria. Codex
  findings (redaction policy specification, manifest privacy-safe
  paths, canonical JSONL serialization + hash coverage, MCP output
  redaction) are absorbed back into RFC 0044's threat model and
  forwarded to the Engram-side follow-up. Gemini findings are
  forwarded to `~/git/engram/` since they describe the Engram-side
  threat surface Striatum is not building.

### Notes

- Dogfood-046 ran the multi-track design + build + review workflow
  for RFC 0044 V1 with the Striatum-side scope only. As with
  dogfood-044/045, the `consolidate` job was not part of the
  workflow; the operator wrote this changelog entry, the
  `docs/rfcs/README.md` status update, the `docs/TODO.md` item-23
  promotion + new F47 row, `docs/dogfood/046/BUILD_HANDOFF.md`, and
  `docs/dogfood/046/PHASE_1_OPERATOR_NOTES.md` out-of-band after
  the run.
- **Claude reviewer produced no on-disk artifact** — only a 3.8 KB
  packet log was emitted. The operator composed a minimal
  `accept_with_findings` review at
  `docs/dogfood/046/review/build/claude/REVIEW.md` from the
  packet-log content to unblock the workflow. This is the **6th
  distinct anti-pattern instance** the dogfood loop has
  surfaced — distinct from both the codex/codex co-blindness
  (D095-D098, D100) and the codex-threat_model-reviewer harshness
  (D099). The reviewer-emits-no-artifact pattern is a new harness
  failure mode: the run cannot proceed without a published review
  artifact, and there is no current operator-recovery surface short
  of writing the artifact by hand. Forwarded to the harness
  improvement RFC backlog along with the codex/codex anti-pattern
  (TODO item 26).
- **Gemini byline-prefix bug surfaced AGAIN.** This is a recurrence
  of the dogfood-044 gemini reviewer profile bug: gemini emitted
  no front-matter YAML block at all, and used the non-conformant
  byline `**Author:** Gemini (Reviewer)` (markdown bold form)
  instead of the required plain `author: <slug>` byline.
  `docs/dogfood/046/review/build/gemini/REVIEW.md` was therefore
  operator-rewritten to preserve gemini's substantive review
  content while adding the required `striatum.finding.v1` front
  matter + a plain `author: reviewer-unknown-model-001` byline.
  The dogfood-044 gemini reviewer profile fragment update did not
  fully fix this — gemini still drops the front matter and still
  reaches for markdown-bold author lines. Forwarded to the
  reviewer-profile audit follow-up alongside the codex
  threat_model harshness pattern from D099.

## v1.34.0 — 2026-05-13

### Added

- RFC 0038 V1.5 — web UI integration gaps landed under dogfood-045.
  (F1) `placeholderIslandPlugin` removed from
  `src/striatum/web/frontend/vite.config.ts`; `plugins` is now
  `[react()]`. `manifest` flipped to `false` so the build no longer
  emits `.vite/manifest.json`; the existing `manifest.sha256` remains
  the single committed manifest. A new `make ui-verify-bundle` target
  rejects (a) any stable island entry whose body contains the V1
  sentinel `Striatum frontend island placeholder loaded`, (b) any
  `island-shared-*.js` chunk containing the same sentinel, and
  (c) any stable island entry under 1024 bytes (unless a sibling
  `island-shared-*.js` chunk ≥ 1024 bytes covers the legitimate
  factored-chunk case). `make ui-check-bundle` now depends on both
  `ui-build` and `ui-verify-bundle`. Python sentinel guard
  `tests/test_web_ui.py::test_island_bundles_have_no_placeholder_sentinel`
  reads each stable island bundle through `importlib.resources` and
  asserts the sentinel is absent so the guard survives `pip install`.
  (F2) `/workflows/new` chooser prop-contract fix: the
  `/workflow-templates` route is unchanged (it already returns
  `{"ok": true, "data": {"templates": list_templates(kind=kind)}}`);
  `src/striatum/web/frontend/src/shared/types.ts` adds
  `WorkflowTemplate` + `WorkflowTemplateListResponse` mirroring the
  server fields and removes the dead `WorkflowShape` /
  `WorkflowLaneSet` / `WorkflowTemplateCatalog` types.
  `WorkflowChooser.tsx` reads `res.data.templates`, partitions by
  `kind`, derives `shape` from the picked `kind: "shape"` row's
  `template_id`, pre-fills `lane_set` from the first overlapping
  `default_lane_sets` entry, and drops the V1 modifier UI (the server
  never returned `catalog.modifiers`). Wizard is now four steps:
  Template → Details → Preview → Save. `__testing` exports `buildSpec`
  + `recommendedForText`; the V1 `isModifierEnabled` export is gone.
  (F3) Island-shared double-mount fix: new
  `src/striatum/web/frontend/src/shared/island-shared-entry.ts`
  (`import "./theme.css"; export {};`) is the new Rollup input for the
  `island-shared` bundle. `src/main.ts` still exists for the Vite dev
  server (`make ui-dev`) but is no longer a production Rollup input,
  so it cannot mount islands twice. Vitest regression
  `src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts`
  mocks `react-dom/client.createRoot`, imports the shared entry plus
  the chooser entry into a JSDOM page with only
  `#island-workflow-chooser`, and asserts `createRoot` is called
  exactly once. (F4) Vite output semantics aligned with the
  package-data layout: output stays at `src/striatum/web/static/build/`,
  public URLs unchanged, and `pyproject.toml`
  `[tool.setuptools.package-data]` already matches the `manifest: false`
  layout (`"striatum.web.static" = [..., "build/*.js", "build/*.css",
  "build/*.sha256"]` + explicit `"striatum.web.static.build" = ["*.js",
  "*.css", "*.sha256"]` sub-package entry). New
  `tests/test_web_workflows.py::test_workflows_edit_renders_graph_editor_island`
  pins `/workflows/edit/<path>`. Supply-chain hygiene: `ui-install`
  now uses `npm ci` (lockfile-reproducible installs); new
  `ui-update-lock` for intentional dependency bumps; new `ui-audit`
  runs `npm audit --audit-level=high`;
  `src/striatum/web/frontend/npm-audit-baseline.json` ships as the
  accepted-findings tracker.
  Files: `Makefile`, `src/striatum/web/frontend/vite.config.ts`,
  `src/striatum/web/frontend/src/shared/api-client.ts`,
  `src/striatum/web/frontend/src/shared/types.ts`,
  `src/striatum/web/frontend/src/shared/island-shared-entry.ts` (new),
  `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`,
  `src/striatum/web/frontend/src/__tests__/workflow-chooser.test.ts`,
  `src/striatum/web/frontend/src/__tests__/workflow-chooser-fetch.test.tsx` (new),
  `src/striatum/web/frontend/src/__tests__/island-shared-no-mount.test.ts` (new),
  `src/striatum/web/frontend/npm-audit-baseline.json` (new),
  `tests/test_web_ui.py`, `tests/test_web_workflows.py`. Implementer
  was **claude** (TypeScript/Vite work), the first dogfood deliberately
  not using codex as implementer to avoid the codex/codex anti-pattern
  (precedents D095-D098). Real-bundle commit and `make` verification
  remain operator-side follow-up — the new sentinel/size guard + Python
  resource test refuse another placeholder commit from reaching CI.

### Decided

- D099: Reject override for the dogfood-045 build review
  (`dec_ccfa1685878d41d69ccc6496cd6612fd`, `accepted_with_follow_up`).
  Codex `review_build_codex` returned `reject severity=critical` under
  the threat_model posture; cross-lane consensus disagreed (claude
  `accept_with_findings` medium, gemini `accept` low). Codex critical
  rests on (a) committed bundles still being V1 placeholders pending
  operator-side rebuild, (b) build verification gates not executed in
  the implementer run, and (c) source-side mitigations being unproven
  against real output. The HANDOFF explicitly documents the
  real-bundle commit as an operator follow-up and the new sentinel
  guard refuses to ship another placeholder commit, so the
  cross-lane 2-of-3 majority overrides. Codex findings absorbed into
  RFC 0038 V1.6 follow-up (TODO item 29). First dogfood with a
  codex-reviewer-of-claude-implementer pattern (not codex/codex);
  the harsh codex verdict suggests codex-as-reviewer baseline
  conservatism is independent of the codex/codex convergent
  blind-spot anti-pattern. Recovery path on this run was non-trivial:
  the codex reject pushed the run state to `failed`, requiring SQL
  surgery + `striatum verdict --override` to recover.

### Notes

- Dogfood-045 ran the 9-job single-track workflow for RFC 0038 V1.5
  (F1-F4 + supply-chain hygiene findings from dogfood-041 deferred
  under cycle-exhaustion override
  `dec_251e8a5f3d674c409de0dad9eacd5844`). Like dogfood-044, the
  `consolidate` job was not in the workflow; the operator authored
  this changelog entry, `docs/rfcs/README.md` status update,
  `docs/TODO.md` follow-ups, `docs/dogfood/045/BUILD_HANDOFF.md`, and
  `docs/dogfood/045/PHASE_1_OPERATOR_NOTES.md` out-of-band.
- The codex review verdict surfaced an operator-facing harness gap:
  a reviewer `reject` verdict transitions the run state to `failed`
  before the operator can decide whether to override. Recovery here
  required SQL surgery on the verdict + run state followed by
  `striatum verdict --override` (the override-accepting verdict path
  landed in v1.32.x). A future RFC could plumb an explicit
  "operator-pending" run state distinct from `failed` so verdicts
  awaiting override do not require manual SQL recovery.

## v1.33.0 — 2026-05-13

### Added

- RFC 0040 V1.5 — daemon-side dispatch + composite tools + watcher
  invocation landed under dogfood-044. (F1) Daemon MCP `tools/call`
  now dispatches through the RFC 0030 method registry via new
  `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
  which owns lookup, capability authorization, envelope build, and
  routing through `DaemonRpcRouter.handle(...)`; the previous stub
  that returned a fake `ok: true` is gone. `DaemonRpcRouter.handle`
  accepts `transport` (default `"rpc"`; MCP passes `"mcp"`) and
  `require_handshake` (default `True`; MCP passes `False`). Audit
  rows are post-dispatch: unknown methods + authorization denials
  emit one `transport="mcp"` deny row; allowed calls emit exactly
  one row carrying the real handler exit code. MCP response shape
  `{content, structuredContent, isError}` preserved; structuredContent
  carries `ok`, `method`, `audit_id`, and `data` on success.
  (F2/F3) `dogfood.publish_on_behalf` runs ack/publish/verdict inside
  one outer `with transaction(conn):` block via new transaction-free
  helpers `_ack_on_behalf_locked`, `_publish_artifact_locked`,
  `_record_verdict_locked`, and `_complete_locked`; review jobs
  require `verdict` up front (validated against the same enum the
  direct-Python helper accepts); `findings_artifact_id` defaults to
  the published artifact id when kind=`finding`; on success exactly
  one `dogfood.publish_on_behalf` event is inserted in-transaction
  with `composition_steps` covering ack/publish/verdict-or-complete;
  on failure the transaction rolls back and a best-effort
  `dogfood.publish_on_behalf_failed` event is written tagged
  `outcome: "rolled_back"`. (F4) New
  `src/striatum/process_progress.py::progress_loop_once` runs one
  bounded supervised-progress pass per repository joined to the
  `runs` table so only attached supervisors under running/paused
  runs tick; called from `daemon.daemon_sweep_once` inside
  `connect_repo(repo)` immediately before per-run auto-sweep work
  and folded into the sweep return payload as `"progress"`. The
  loop materializes each row as a `SupervisedProgressTarget` and
  ticks `SupervisedProgressWatcher`, whose heartbeat callback calls
  `striatum.cli.mutations.heartbeat` on the same repo connection.
  Metadata-only events:
  `supervisor.progress_watcher_heartbeat`,
  `supervisor.progress_watcher_idle`,
  `supervisor.progress_watcher_lost`. Log contents are never read.
  (F5) `ProcessProgressConfig.startup_grace_seconds` defaults to
  60 s; within grace a missing scratch path returns `waiting_for_log`
  with no warning; watcher catches `FileNotFoundError`/`OSError`
  while scanning `*.log` files so rotated logs follow without
  recreating the target; loop accepts a `should_stop` predicate and
  checks it between supervisors so SIGTERM cannot start a new
  heartbeat after shutdown; `progress_advisory_lock(repo,
  job_id=...)` is shared with `surgical_recovery` (watcher tick
  returns `lock_busy`, surgical recovery returns
  `progress_lock_busy`); PID-reuse guard via `process_start_time(pid)`
  flips the row to `state='lost'` on mismatch versus stored
  `pid_start_time` and emits `supervisor.progress_watcher_lost`.
  (F6) New `tests/test_mcp_dogfood_e2e.py` drives MCP `tools/call`
  round-trips for `dogfood.publish_on_behalf` covering completion
  and review-verdict paths (marked `pytest.mark.multi_repo`);
  `tests/test_supervised_progress_watcher.py` extended with
  `test_progress_loop_once_heartbeats_attached_supervisor` +
  `test_progress_loop_once_refuses_pid_identity_mismatch`. Files:
  `src/striatum/mcp.py`, `src/striatum/process_progress.py`,
  `src/striatum/db.py`, `src/striatum/daemon.py`,
  `src/striatum/daemon_pg/mcp_dispatch.py` (new),
  `src/striatum/daemon_rpc/server.py`,
  `src/striatum/dogfood/operator_tools.py`. Tests: 42 passed,
  10 skipped (multi_repo skips without PG harness).

### Decided

- D098: Cycle-exhaustion override for the dogfood-044 build review.
  4th instance of the codex/codex implementer+reviewer
  convergent-blind-spot anti-pattern (precedents D095 dogfood-042
  Track A, D096 dogfood-042 Track C, D097 dogfood-043 Python
  build). Codex needs_revision overridden; cross-lane claude
  accept_with_findings (medium), gemini accept (low). Codex
  findings absorbed into RFC 0040 V1.6 follow-up (TODO item 28).
  Anti-pattern now well-characterized across four independent
  runs; refuse-by-default validator rule (TODO item 26) remains
  the deferred half.

### Notes

- Dogfood-044 ran the 9-job single-track workflow for RFC 0040
  V1.5 (F1-F6 codex findings from dogfood-040). The `consolidate`
  job was not present in the workflow; the operator authored this
  changelog entry, `docs/rfcs/README.md` status update,
  `docs/TODO.md` follow-ups, `docs/dogfood/044/BUILD_HANDOFF.md`,
  and `docs/dogfood/044/PHASE_1_OPERATOR_NOTES.md` out-of-band
  (dogfood-043 lesson applied).
- Stale-lease intervention: codex finished writing code, but the
  supervisor lease expired at ~30 min default before the
  HANDOFF.md was published; operator composed the build HANDOFF
  on behalf of the implementer (per-finding status read from
  source). 30-min default-lease issue noted as a harness gap
  separate from the V1.5 implementation scope.
- Byline-prefix bug observed in 3 of 4 reviewed dogfoods now:
  both gemini and claude reviewers emit
  `(role)-lane-unknown-model-NN` instead of
  `(role)-unknown-model-NN`. Operator hand-edited the bylines
  before publication.

## v1.32.0 — 2026-05-13

### Added

- RFC 0045 V1 multi-phase workflow schema landed
  (`striatum.workflow.v1.1`): new top-level `phases` array, a
  `phase_synthesis` job type that gates phase transitions, validator
  rules refusing cross-phase dependencies that bypass the synthesis
  gate, runtime materialization of phase synthesis fan-in edges,
  `status --json` derives `phases` + `current_phase_id` from the
  workflow snapshot plus latest job attempts, dashboard + service
  run-detail surfaces receive phase progress from the status payload,
  workflow generator gains `shape: "multi_phase"` emitting v1.1
  workflows with phased track jobs + synthesis gates, and
  `striatum workflow upgrade --add-phases` previews by default and
  writes with `--apply`. V1 workflows continue to validate and run
  unchanged. Files: `src/striatum/workflow.py`,
  `src/striatum/cli/{introspect,mutations,parser,dispatch,workflow}.py`,
  `src/striatum/{dashboard,service}.py`,
  `src/striatum/workflow_generator/{core,catalog}.py`,
  plus tests under `tests/test_workflow_phases.py`,
  `tests/test_workflow_generator.py`, `tests/test_workflow_upgrade.py`,
  `tests/test_cli_mvp.py`, `tests/test_dashboard.py`,
  `tests/test_service.py`, and fixture
  `tests/fixtures/multi_phase_workflow.json`.
- RFC 0045 V1 React Flow editor extensions (Track B): phase color
  bands rendered via `<ViewportPortal>` so bands pan/zoom with nodes;
  cross-phase edges receive distinct styling
  (`className: "cross-phase-edge"`, thick black stroke,
  `data: { crossPhase, sourcePhase, targetPhase }`); new
  `PhaseInspector` swaps into the right-hand inspector slot when a
  band header is clicked (edit `title`/`description`, show
  `synthesis_job_id`, list jobs in phase); drag-drop refuses
  cross-band moves with snap-back + inline `role="alert"` error;
  new `phase` selector in the job inspector (gated on
  `workflow.phases?.length > 0`); `syncWorkflowEdges` strips
  derived `crossPhase`/`sourcePhase`/`targetPhase` keys on save;
  `selectedJobId` upgraded to a `GraphSelection` discriminated union.
  V1 workflows keep the original square-grid layout, thin grey edges,
  and job-only inspector with no visual change. Files:
  `src/striatum/web/frontend/src/shared/types.ts`,
  `src/striatum/web/frontend/src/shared/theme.css`,
  `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx`,
  and new unit suites in
  `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts`.

### Decided

- D097: Cycle-exhaustion override for the dogfood-043 Python build
  review. 2-of-3 cross-lane reviewers accept (claude
  accept_with_findings low, gemini accept low); codex needs_revision
  (high) overridden because the codex/codex implementer+reviewer
  pairing produced the third instance of the convergent-blind-spot
  anti-pattern (precedents D095 dogfood-042 Track A, D096 dogfood-042
  Track C). Codex findings (cycle phase-jump validator gap, strict
  phase-skip restriction, phase_id strict-on-v1 check, drag-drop
  dropdown bypass, malformed v1.1 tolerance) absorbed into RFC 0045
  V1.5 follow-up (TODO item 27). Anti-pattern now well-characterized
  across three independent runs; full validator refuse-by-default
  remains the deferred half of TODO item 26 (a soft warning landed in
  the dogfood-043 prep commit).

### Notes

- Dogfood-043 ran with two parallel tracks (Track A Python core
  implemented by codex; Track B React Flow editor implemented by
  claude) and 3-way build review postures (codex threat_model,
  claude ergonomics_dx, gemini adversarial). The `consolidate` job
  was not present in the workflow; the operator authored this
  changelog entry, the `docs/rfcs/README.md` status update, the
  `docs/TODO.md` follow-ups, the `docs/dogfood/043/BUILD_HANDOFF.md`
  cross-track handoff, and the `docs/dogfood/043/PHASE_1_OPERATOR_NOTES.md`
  operator narrative in its place (dogfood-042 lesson applied: the
  in-workflow consolidate job was the wrong locus when the operator
  is already the synthesizing surface).

## v1.31.0 — 2026-05-13

### Added

- Track A (dogfood-042): RFC 0039 V1 Steps 1+2 Go daemon core landed
  under `go/`. New `go/cmd/striatumd` entry point, `go/pkg/rpc`
  (envelope-v1 validation/serialization, RFC 0030 method registry,
  capability vocabulary, in-memory capability helpers, handshake,
  `daemon.describe`, duplicate request detection, RPC server framework
  for read-only routes), `go/pkg/db` (daemon Postgres config
  resolution/redaction, dependency-free `psql` runner, migration
  loading/application, embedded SQL migrations, audit hash/recording),
  `go/go.mod` + `go/go.sum` + `go/Makefile`, and root `Makefile`
  `daemon-go-build` / `daemon-go-test` / `daemon-go-lint` targets.
  Python harness gained `daemon_core: Literal["python","go"]` parameter
  on `DaemonProcess` and `MultiRepoHarness` (default `"python"`,
  backward-compatible); Go invocation resolves the binary via
  `STRIATUMD_GO_BIN` or `<repo>/go/bin/striatumd` and runs
  `make -C go build` on demand. Phase 1 partial — Steps 3-6 (CLI
  integration, mutating verbs, supervised processes, distribution)
  deferred to a Phase 2 dogfood per RFC 0039 §9. Documentation
  updated in `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, and
  `docs/UBIQUITOUS_LANGUAGE.md`.
- Track B (dogfood-042): RFC 0044 drafted as the Engram Phase 1
  implementation spec — Engram as an optional read-only memory
  augmentation for Striatum operators. Pull-mode ingestion with
  Striatum-owned redacted JSONL export, Engram-owned
  `ingest-striatum`, standalone `engram-mcp-stdio` MCP server, four
  read-only retrieval tools, Engram-local `memory.*` capabilities,
  and a hard augmentation-not-dependency boundary. RFC text only;
  implementation lands via a future dogfood.
- Track C (dogfood-042): repo-local-state-to-Postgres design work
  superseded by main's RFC 0043 (Postgres as Sole Substrate and
  Daemon-Required Runtime, accepted via D094) which landed during this
  dogfood from the parallel session. Track C dogfood artifacts (3
  designs + synthesis + 3 reviews + decision) retained under
  `docs/dogfood/042/track_c/` as historical provenance; the draft
  `docs/rfcs/0042-repo-local-state-to-postgres.md` is NOT shipped
  (collides with main's RFC 0042 number, scope absorbed by RFC 0043).

### Decided

- D095: Cycle-exhaustion override for Track A Go daemon build.
  2-of-3 reviewers accept_with_findings (claude, gemini); codex
  needs_revision overridden because the codex/codex
  implementer+reviewer pairing converged on its own findings. Codex
  findings absorbed into RFC 0039 V1.5 follow-up (TODO item 24).
  Follow-up: forbid codex/codex implementer+reviewer pairs in the
  workflow validator (TODO item 26).
- D096: Cycle-exhaustion override for Track C build review.
  2-of-3 reviewers accept-equivalent (claude accept, gemini
  accept_with_findings); codex needs_revision overridden because the
  same codex/codex anti-pattern recurred. Track C's repo-local-PG
  design intent is absorbed by main's RFC 0043; the draft RFC file is
  not shipped.

### Notes

- The dogfood-042 multi-phase workflow with three parallel tracks
  completed with two cycle-exhaustion overrides (D095 Track A, D096
  Track C). The `consolidate_phase_1` job was cascaded into
  cancellation; the operator wrote this changelog entry, the
  `docs/rfcs/README.md` index updates, the `docs/TODO.md` follow-ups,
  and the `docs/dogfood/042/BUILD_HANDOFF.md` cross-track handoff in
  its place.

## 1.30.0 — 2026-05-13

### Added

- RFC 0038 V1 web UI feature additions and frontend toolchain. Vite +
  React + TypeScript contributor-side toolchain bundled into the wheel
  via `src/striatum/web/static/build/` (operators stay pip-only). New
  `make ui-install` / `make ui-build` / `make ui-dev` / `make ui-test`
  targets. Five user-facing additions: workflow detail's Edit
  affordance promoted from a muted text link to a button next to "Run
  this workflow now"; new `/view/` repo file browser with lazy
  expansion via `GET /v1/repo/tree`; new `/workflows/new` chooser
  wizard over the RFC 0034 V1 generator endpoints with a
  `<dialog>`-driven operator confirmation gate; drag-drop React Flow
  workflow graph editor with structured per-node widgets at
  `/workflows/edit/<path>`; Shiki-based syntax-highlighted code viewer
  for non-Markdown files at `/view/<path>` with line numbers, copy,
  raw link, and wrap toggle. New `docs/FRONTEND_DEVELOPMENT.md`
  contributor guide. Dark-mode parity inherited from `base.css`.
- New shared TypeScript prop contract in
  `src/striatum/web/frontend/src/shared/types.ts` mirroring the
  workflow validator's closed vocabularies.

### Decided

- D092 (re-cited): supersede D073's implicit "no node toolchain" rule
  for the contributor-side build path. Operator install remains pip
  only; bundled JavaScript ships in the wheel under
  `src/striatum/web/static/build/`. Bundle drift is detected by a
  committed `manifest.sha256` in CI.

## 1.29.0 — 2026-05-12

### Added

- RFC 0040 V1 operator-side slice of the MCP-driven dogfood harness:
  twelve dogfood-lifecycle chat-tool entries (`run_prepare`,
  `run_start`, `register_session`, `supervise_start`, `claim_next`,
  `ack`, `publish_artifact`, `verdict`, `complete`, `supervise_stop`,
  `run_summary`, `evidence_export`) over `striatum.api.invoke`. Ten
  state-mutating tools are gated behind `serve --allow-mutations`; two
  read-shaped tools (`run_summary`, `evidence_export`) stay available
  unconditionally.
- Per-model harness-profile fragments baked into the bundled workflow
  template catalog (`claude_code_default`, `codex_default`,
  `gemini_default`, `generic_default`). `workflow generate` enriches
  any user-supplied profile body with the catalog defaults when
  `native_delegation.instruction` is missing; existing instructions
  are preserved verbatim.
- `striatum workflow upgrade <path>` CLI verb that backports the
  catalog fragments into existing workflow.json files. Refuse-on-
  conflict default with `--force` to overwrite; `--dry-run` reports
  the change set without writing; refuses when any non-terminal run
  references the workflow.
- New `docs/HARNESS_FRICTION_PATTERNS.md` documenting the four
  observed friction patterns (036 strategy-then-exit, 037 ask-and-
  exit, 038 lease-expiry-under-active-load, 038/039 front-matter
  completeness) and the V1 fixes.
- `docs/MCP.md` "Dogfood-Lifecycle Tools" section listing each new
  tool, its capability requirement, and an example sequence.
- `docs/HOW_TO_HUMAN.md` walkthrough of driving a dogfood through the
  MCP chat tools instead of bash CLI, plus a `workflow upgrade`
  recipe.
- `docs/HOW_TO_AGENT.md` note for operator-AI sessions to prefer the
  MCP chat tools over shelling out to bash; supervised roles still
  use the work-packet `commands` block verbatim.

### Decided

- D093: Accept RFC 0040 V1 as the operator-side slice. Composite
  tools (`dogfood.publish_on_behalf`, `dogfood.surgical_recovery`)
  and the daemon-side supervised-progress heartbeat land in the
  systems half of the RFC. See `docs/HARNESS_FRICTION_PATTERNS.md`
  for the long-form record.

## 1.28.0 — 2026-05-13

### Added

- RFC 0037 V1 web UI ergonomics: run/workflow filters, run duration
  and workflow last-modified columns, grouped doctor problems with a
  terminal-run filter, UTC/local timestamp toggle, graph tooltips,
  keyboard shortcuts, app-specific dark-mode parity, promoted run next
  actions, and empty states for the main triage pages.

## 1.27.0 — 2026-05-13

### Added

- RFC 0035 V1 test infrastructure: `tests/_harness/MultiRepoHarness`
  for ephemeral Postgres + daemon + multi-repository e2e coverage, five
  cross-repo harness-backed test modules, and `make test-multi-repo`.

## 1.26.0 — 2026-05-13

### Added

- RFC 0036 V1: `striatum-mcp` skill coverage for loose skill installs
  and plugin bundles, plus chat tools `generate_workflow_preview` and
  `generate_workflow_write` over the RFC 0034 workflow generator.
- Chat workflow writes are hidden when `serve` lacks
  `--allow-mutations`; crafted write calls fail with
  `mutations_disabled`.
- Chat workflow writes queue a one-shot operator confirmation in the web
  UI before generated workflow files are written.

## 1.25.0 — 2026-05-12

### Added

- RFC 0034 V1 workflow generator: bundled shape/lane-set catalog,
  `workflow templates list/show`, `workflow generate`, local service
  catalog and generation endpoints, custom-plan compilation, and
  `workflow init --style` compatibility over the generator.

### Decided

- D091: OPERATOR_REPORT.md is written incrementally during a dogfood
  run, not only at the end. Refines D089. The operator appends a dated
  entry per intervention (publish-on-behalf, recovery sweep,
  override-verdict, decision recording) at the moment it occurs;
  end-of-run only writes the wrap-up sections.

## 1.24.0 — 2026-05-12

### Added

- RFC 0032 V2 slice: cross-repo workflow schema validation, repo-local
  `runs.cross_repo_run_id`, daemon DB migration v3 for cross-repo run
  metadata, daemon RPC method scope modes, `recovery` capability, daemon
  MCP `tools/list` filtering and `tools/call` re-authorization/audit
  scaffolding, and mocked cross-repo lifecycle helpers.

### Documentation

- Documented the dogfood-035 deferral: real two-repo daemon end-to-end
  integration tests and live scheduler progression wait for TODO Open
  item 19, the multi-repo test harness RFC.

## 1.23.0 — 2026-05-11

### Added

- RFC 0030 daemon RPC foundation: envelope-v1 codec, newline JSON
  framing helpers, owner-local Unix socket and loopback HTTP guards,
  `daemon.hello` / `daemon.welcome`, `daemon.describe`, a
  capability-bound method registry, and PostgreSQL request/audit helper
  wiring.
- RFC 0031 daemon-owned supervision/apply foundation: daemon DB
  migration v2 for method metadata, daemon supervisor ownership, and
  apply receipts; repo-local migration v13 for supervisor pointers; and
  fail-closed apply-key/refusal helpers.

### Documentation

- RFC 0030 and RFC 0031 are now marked accepted for the V2 foundation.
  Docs distinguish the landed RPC/schema boundary from deferred
  cross-repo workflows, MCP mutation expansion, hosted services,
  Windows daemon support, and any claim of third-party cryptographic
  non-repudiation.

## 1.22.1 — 2026-05-11

### Fixed

- Byline tolerance: a Markdown-decorated byline like
  `**Author:** value`, `# Author: value`, or `_author_: value` is now
  recognised by the publisher and stored in `artifacts.author_line`
  as the canonical lowercase `author: value` form. Models seen in
  dogfood-031 and dogfood-033 produced the bold-decorated form, which
  previously caused the publisher to silently drop the byline (stored
  as NULL); the canonicaliser now normalises decoration before
  matching. Mismatched bylines still refuse with the documented error.

### Added

- `publish-artifact` auto-attaches default front matter for the
  `synthesis` artifact kind when the file omits the `---` block. The
  publisher prepends `schema_version: "striatum.synthesis.v1"` and
  `artifact_kind: "synthesis"` (the only required fields, both
  constants the publisher already knows from `--kind synthesis`),
  rewrites the file on disk so the stored SHA agrees with downstream
  reads, and proceeds with the rest of validation. The agent's body
  is preserved verbatim after the prepended block. Other
  schema-bearing kinds (`finding`, `decision`, `findings_ledger`,
  `support_ledger`, `action_item_ledger`,
  `harness_improvement_proposal`) have semantic required fields the
  publisher cannot invent (`verdict_intent`, `outcome`, etc.) and
  continue to silently accept missing front matter — adding an
  explicit refusal there would be a policy break and should land
  behind a workflow-level opt-in.

- Hard byline + front matter discipline section in every dogfood-033
  design prompt (`design_codex.md`, `design_claude_code.md`,
  `design_gemini.md`, `synthesize_design.md`): forbid Markdown bold
  (`**Author:**`), heading prefix (`# author`), italics (`_author_`),
  and quotes around the value; require lowercase `author:` exactly;
  include the JSON-encoded front matter template for schema-bearing
  kinds.

## 1.22.0 — 2026-05-11

### Added

- RFC 0033 daemon PostgreSQL substrate scaffolding: optional
  `striatum-orchestrator[daemon-pg]` driver dependency, packaged
  forward-only daemon DB baseline migration, `daemon doctor`
  PostgreSQL onboarding checks, `daemon start --postgres-url`, and
  `daemon migrate --from sqlite --to pg` cutover wiring with V1 audit
  hash preservation.

### Documentation

- RFC 0033 is now documented as accepted V2: daemon-owned state moves to
  operator-supplied system PostgreSQL, with forward-only daemon DB
  migrations and `striatum daemon migrate --from sqlite --to pg` for the
  V1 registry cutover. The docs keep repo-local
  `.striatum/state.sqlite3` as workflow truth and avoid claiming daemon
  RPC, MCP mutations, daemon-owned supervision, cross-repo mutation, or
  sealed apply before their later RFCs.

## 1.21.1 — 2026-05-11

### Fixed

- Parallel-reviewer cascade-child UNIQUE collision: when two
  reviewer postures fan out from a single cycle target (e.g. three
  parallel design-review postures all routing back to one
  `synthesize_design` via `needs_revision` cycles), the second
  `submit-review` no longer raises
  `UNIQUE constraint failed: jobs.run_id, jobs.idempotency_key`.
  Cycle-target cloning is now idempotent on
  `(run_id, workflow_job_id, attempt)`; parallel reviewers share a
  single revision attempt of the shared cycle target. Surfaced in
  dogfood-031 by `dec_operator_security_cascade_collision_2026_05_11`.

### Added

- `.striatum/bin/codex-supervised-wrapper.sh` mirroring the existing
  claude/gemini supervised wrappers. Codex `exec ... -` hangs on
  empty FIFO stdin in supervised mode in some environments
  (observed during dogfood-031); the wrapper spawns a fresh
  `codex exec` per packet, matching the RFC 0010 V2 one-packet-per-
  invocation model. Updated the bundled `examples/harness-profiles/`
  workflow and `docs/dogfood/031/workflow.json` reference to use
  the wrapper so future runs avoid the FIFO hang.
- `docs/CLI_REFERENCE.md` and `docs/HOW_TO_HUMAN.md` now document
  the RFC 0028 V1 daemon/repo/dashboard verbs (`striatum daemon
  start/status/stop/sweep`, `striatumd` console script, `repo
  add/list/remove`, `--daemon` read routing, `dashboard --all`,
  bootstrap admin token semantics, audit shape, and the V1
  deferrals: no RPC server, no daemon-owned supervision, no
  mutation MCP tools, no Windows daemon support, no
  cross-repository workflows).

## 1.21.0 — 2026-05-11

### Added

- RFC 0026 V1 lane-liveness attestation: work-packet and publish-time
  bylines now downgrade unattested sessions to `author: operator`;
  attached supervised sessions regain lane/model bylines only when the
  pid identity and snapshot command binding match. Added
  `register-session --operator-label`, per-session attestation surfacing,
  and review-job `require_attested_lane: true` gates.
- RFC 0027 provenance-mode guardrails: workflows may declare
  `provenance_mode` (`advisory`, `attested_bylines`, `sealed_patch`).
  `sealed_patch` validates path policy but refuses to start until real
  source containment ships.
- RFC 0028 V1 registry-backed multi-repo acceptance slice: optional
  `striatumd` / `striatum daemon start` foreground sweep process, daemon
  registry, `repo add/list/remove`, explicit `--daemon` read routing,
  `dashboard --all`, resources-only daemon MCP, metadata-only hash-chained
  audit, and recovery sweeping across registered active runs. V1 does not
  ship a daemon RPC server; CLI and MCP clients open the owner-only
  registry SQLite directly under token/capability checks.
- Dogfood-031 revision round 2 hardens the daemon slice: unsupported
  forced-daemon verbs refuse instead of falling back to direct mode,
  `repo add` authorizes before repo-local access and requires `--init`
  for absent state databases, daemon MCP uses explicit tokens with
  repo-scope filtering, audit segment manifests are guarded and checked
  by doctor, and foreground sweeps write repo-local
  `daemon.recovery_sweep` events bylined `striatumd-<instance-id>`.
- Dogfood-031 revision round 3 removes `STRIATUM_DAEMON_TOKEN` plaintext
  env-var support, uses realpath/inode-based repository identity for new
  registrations, admin-gates manual `daemon sweep`, audits denied
  dashboard/MCP aggregate reads with client attribution on allowed reads,
  and documents RPC server, audit retention/rotation, HTTP transport, and
  full underlying recovery-byline propagation as follow-up RFC scope.

## 1.20.1 — 2026-05-10

### Added

- "Verdicts by posture" chips on `/run/<id>` are now clickable.
  New route `GET /run/<id>/posture/<posture>` lists every verdict
  recorded with that posture for the run: verdict value, review
  job, role/lane, session slug, finding artifact link, and
  rationale. Page also shows a one-paragraph "what does this
  posture mean?" explanation per RFC 0018's posture vocabulary
  (`devils_advocate`, `security`, `threat_model`, etc.).

## 1.20.0 — 2026-05-09

### Added

- RFC 0025 V1 Steps 2+3 (dogfood-029): `codex` and `gemini`
  plugin profiles, completing the V1 plugin scope.
  - **`codex` profile**: 14-file Codex plugin bundle under
    `.striatum/plugins/codex/` with `.codex-plugin/plugin.json`, 5
    skills (byte-shared with claude_code), 5 Markdown commands,
    `hooks/hooks.json`, `.mcp.json`, `README.md`, `.manifest.json`.
    User scope: `~/.codex/plugins/<namespace>/`.
  - **`gemini` profile** (promotes from RFC 0015 generic
    fallback): 14-file Gemini extension under
    `.striatum/plugins/gemini/` with `gemini-extension.json`,
    `GEMINI.md` context file, 5 skills (byte-shared), 5 TOML
    commands (bare top-level form per Gemini extension spec),
    `agents/striatum-recover.md` sub-agent definition,
    `README.md`, `.manifest.json`. User scope:
    `~/.gemini/extensions/<namespace>/`.
  - **`--profile all`** aggregates all three profiles into one
    install invocation. Result shape: `{"profile": "all",
    "results": [...]}`.
  - Marketplace fixture continues to be reentrant; gemini
    short-circuits with `{"skipped": True, "reason": "gemini has
    no marketplace concept"}` so JSON callers can detect the skip.
  - F1 byte-match test extended: skill template trees under all
    three profiles must match `skills/templates/claude_code/`
    byte-for-byte.

RFC 0025 status: **accepted (V1)** — three first-class profiles
shipped.

### Deferred to V2

- Cross-target install (one bundle into many target repos).
- Hosted marketplace.
- Codex `apps/` and `assets/`.
- Per-target git-repo extension format for gemini.

## 1.19.0 — 2026-05-09

### Added

- RFC 0025 V1 Step 1 (dogfood-028): `claude_code` plugin profile.
  - `striatum plugin install --profile claude_code` emits a
    14-file Claude Code plugin bundle under
    `.striatum/plugins/claude_code/`. Layout: `.claude-plugin/plugin.json`,
    `skills/striatum-{workflow,scaffold,claim-loop,supervise,recover}/SKILL.md`,
    `commands/{claim-next,status,why,dashboard,doctor}.md`,
    `hooks/hooks.json`, `.mcp.json`, `README.md`, `.manifest.json`.
  - `striatum plugin uninstall --profile claude_code` reads the
    bundle's manifest and deletes only manifest-tracked files;
    refuses to delete operator-edited files without `--force`.
  - `striatum init --with-plugins [profile]` mirrors
    `--with-skills`. Default profile is `claude_code`.
  - `--with-marketplace` (default on) writes
    `.striatum/plugins/marketplace.json` with a `local-striatum`
    fixture entry; reentrant — re-installs update in place.
  - Doctor checks `plugin_missing` and `plugin_outdated` walk every
    installed bundle's `.manifest.json` and surface the exact
    `striatum plugin install --profile <id>` invocation that fixes
    the drift.
  - Skill bodies are byte-shared with `skills/templates/claude_code/`
    via a CI test (`test_skill_templates_match_skills_module`)
    so future skill edits propagate to both surfaces.
  - URL-leak invariant: `test_claude_code_no_external_urls`
    forbids `https?://`, `git://`, `file://`, `ssh://`, `ftp://`
    in any rendered file.

### Deferred to V1 Step 2 / Step 3

- `codex` plugin profile (`.codex-plugin/plugin.json` + Codex
  commands).
- `gemini` profile promotion (split the current single-guide shape
  into the same five-skill structure used by claude_code).
- `--profile all` aggregation.
- Cross-target install (one bundle, many target repos).

## 1.18.0 — 2026-05-09

### Added

- RFC 0024 V4 (dogfood-027): pause/resume + per-job mutations.
  - **Migration v11** adds `runs.paused_at` and `runs.paused_reason`
    columns. Forward-only; idempotent against fresh DB whose
    schema baseline already includes them.
  - **`pause_run(conn, *, run_id, reason)`** sets the columns;
    idempotent on already-paused; refuses terminal states.
  - **`resume_run(conn, *, run_id)`** clears the columns;
    idempotent on not-paused; refuses terminal states (use
    `retry_job` to revive a canceled run).
  - **`claim_next` gate**: runs with `paused_at IS NOT NULL`
    return `{"status": "no_work", "paused": True}`. Active leases
    keep ticking; expire-leases at the top of `claim_next`
    handles paused-with-stale-leases.
  - **`retry_job(conn, *, run_id, job_id)`** resets a
    failed/canceled/blocked job to `queued`, increments
    `attempt`, marks prior `queue_messages` rows as `canceled`
    (preserving the partial unique index), re-enqueues, and
    revives canceled/failed runs to `running` with a loud
    `run.revived` event.
  - CLI: `striatum run pause/resume/retry-job`.
  - HTTP: `POST /run/<id>/pause`, `/run/<id>/resume`,
    `/run/<id>/job/<jid>/cancel`, `/run/<id>/job/<jid>/retry`. All
    mutation-gated.
  - UI: Pause/Resume buttons + paused status pill on the run
    detail page; Cancel/Retry buttons on the job detail page.
    Cancel confirm reads "Cancel this job AND its dependents…".
    All islands CSP-safe.

### Run-revival semantics (D078 follow-up)

Per RFC 0024 V4 design-review F1 (option C): when an operator
retries a job whose run is `canceled` or `failed`, the run
transitions back to `running` and a `run.revived` event is emitted
with `previous_run_state` payload. The terminal-state guarantee
softens for operator-triggered revival but stays loud (event +
documented). `retry_job` refuses to revive a `completed` run.

### Deferred to V5 if needed

- Pause-with-deadline (auto-resume at timestamp).
- Per-lane pause.
- Recovery integration of pause as escalation hook target.
- Consolidate `_read_json_body` / `_read_json_body_strict` helpers.

## 1.17.0 — 2026-05-09

### Added

- RFC 0024 V3 (dogfood-026): cancel-run mutation surface plus the
  dirty-tree visibility V2 deferred.
  - `cancel_run(conn, *, run_id, reason)` in `db.py` — top-down
    cancel that voids active leases, marks in-flight jobs (queued,
    running, blocked, ready, claimed) as canceled, transitions the
    run to `canceled`, emits `run.canceled`, and closes remaining
    sessions. Idempotent on already-canceled; refuses completed /
    failed via `InvalidTransitionError`.
  - `striatum run cancel --run-id <id> [--reason <text>]` — CLI.
  - `POST /run/<id>/cancel` — mutation-gated HTTP endpoint.
    Returns 200 on success (and on idempotent re-cancel); 405 / 404 /
    409 / 415 for the other paths.
  - Cancel button on the run-detail page when state is non-terminal
    (prepared / needs_branch_confirmation / ready / running). CSP-safe
    JS island in `/static/run_cancel.js`.
- Run-now dirty-tree visibility (closes V2 design-review F3):
  `POST /workflows/run/<path>` now returns 409 with
  `error.kind: "dirty_tree"` and `error.git_status` (first ~80
  lines of `git status --short`) when `git_create_or_checkout_branch`
  fails. Operators see the blocker without context-switching to a
  terminal.

### Deferred to V4

- Pause / resume runs.
- Auto-branch suffix (research showed multi-run-per-branch is
  by-design — the friction operators feel is dirty-tree, which V3
  fixes directly).
- Per-job mutation buttons (kill running job, retry).
- Programmatic re-run with parameter overrides.
- Recovery integration: cancel-run as an escalation hook target.

## 1.16.1 — 2026-05-09

### Added

- RFC 0024 V2.1: branch-confirm button on `/run/<id>` for runs in
  `needs_branch_confirmation` state. Operator no longer has to drop
  to the CLI when a `confirm`-mode workflow is started via the
  Run-now button. POST `/run/<id>/branch-confirm` with
  `{branch, create, use_current}` calls `branch_confirm` and
  `run_start` in one transaction; reload reveals the now-running
  run.

### Fixed

- SVG dependency graph rendered with explicit `width`/`height`
  attributes instead of relying solely on `viewBox` + CSS
  `max-width: 100%`. Small graphs no longer scale up to fill the
  full container width — boxes render at their natural pixel size
  and the SVG only shrinks for narrow viewports.

## 1.16.0 — 2026-05-09

### Added

- RFC 0024 V2 (dogfood-025): three editor additions on
  `/workflows/*` — run-now lifecycle, `If-Match` concurrency
  guard, and field-level validation errors.
  - `POST /workflows/run/<path>` — mutation-gated; calls
    `create_run + branch_confirm(create=True) + run_start`;
    returns `{run_id}` on 200; 409 on dirty-tree branch refusal;
    422 on validation failure with structured `errors[]`. When
    `branch.mode == "confirm"`, returns 200 with status
    `needs_branch_confirmation` so the operator can finish out of
    band.
  - `If-Match: <sha256>` precondition on `POST
    /workflows/edit/<path>`. GET stamps the disk sha into a hidden
    `<script id="workflow-sha256">` tag; editor JS echoes it on
    POST; on stale sha the server returns 412 with
    `current_sha256` so the editor can prompt for reload. Missing
    header → V1.5 opt-out (backward compatible).
  - `WorkflowError` extended with optional `field_path`. 8
    high-traffic raise sites tagged: `schema_version`, duplicate
    job id, unknown role, unknown lane, invalid artifact path,
    cycle references unknown job, cycle `max_iterations < 1`. The
    422 body now includes `error.errors: [{field_path, message}]`;
    editor highlights the offending form field via a
    `data-field-path` attribute. Untagged raise sites keep `None`
    and the editor falls back to the V1.5 top-of-form banner.
  - "Run this workflow now" button on `/workflows/<path>` (only
    rendered when the workflow is `valid`). On 200 navigates to
    `/run/<run_id>`. CSP-safe: behavior lives in
    `/static/workflow_run.js`.

### Workflow-trust model

V2 lets any operator with `--allow-mutations` launch a run from any
committed `workflow.json`. This matches the CLI surface (`striatum
run prepare --workflow <path>` from a shell). No new attack surface.

### Deferred to V3

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- AI-assisted scaffolding via chat tool that *writes*
  workflow.json (would require per-tool gating).
- Multi-error reporting (collect all errors, not just first).
- Field-path coverage for the remaining ~22 raise sites.
- `flock()` for hard concurrency guarantees.
- 409 body carrying `git status --short` output.

## 1.15.0 — 2026-05-09

### Added

- RFC 0024 V1.5 (dogfood-024): workflow visual builder.
  - `GET /workflows/edit/<path>` renders a form-driven editor
    for any repo-relative `workflow.json`. Existing files load
    their parsed JSON (even invalid — the editor opens so the
    operator can fix); non-existent paths render an empty
    scaffold with the workflow_id derived from the path stem.
  - `POST /workflows/edit/<path>` saves: validates the body
    via `validate_workflow`; on success atomically writes via
    `<path>.tmp` + rename; on validation failure returns 422
    with the WorkflowError message (file unchanged).
  - Mutation-gated (`--allow-mutations` required for POST).
  - Body capped at 1 MB; non-`application/json` content-types
    rejected with 415 (per design-review F1).
  - Path safety mirrors `/view/<path>`: `..`, leading `/`, null
    bytes, hidden dirs (`.git`, `.striatum`) refused.
  - JS island (`workflow_edit.js`) renders form sections from
    in-memory state: header, roles, lanes, jobs, edges, cycles.
    Add/remove buttons mutate state; save POSTs the full state
    as JSON; on success redirects to the detail page.
  - localStorage backup persists the in-progress draft so a
    browser-crash doesn't lose work; recovered with operator
    confirmation on reload.
  - "Edit" link added to the workflow detail page.

### Deferred to V2

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button.
- Field-level error highlighting (requires `validate_workflow`
  API change).
- `If-Match: <sha256>` precondition for safe concurrent edits.
- AI-assisted scaffolding via chat tool that *writes*
  workflow.json (would require per-tool gating).

## 1.14.0 — 2026-05-09

### Added

- RFC 0024 V1 (dogfood-023): workflow browser (read-only).
  - `GET /workflows` lists every `**/workflow.json` in the
    target repo with validation status, workflow_id,
    job/lane/role counts. Hidden dirs (`.git`, `.striatum`,
    `.venv`, `node_modules`, etc.) excluded from discovery.
  - `GET /workflows/<repo-path>` renders a detail page with
    the SVG dependency graph (reusing RFC 0022 V1's renderer)
    plus tables for jobs, lanes, roles, edges, and cycles.
    Invalid workflows render their `WorkflowError` message
    inline; the page never 500s.
  - Path safety mirrors `/view/<path>`.
  - New chat tool `list_workflows` extends RFC 0023 V1.5's
    closed read-only tool set; the model can answer "which
    workflow produced run X?". Capped at 100 entries.
- `Workflows` link in the top nav (between Runs and Chat).

### Deferred to V1.5

V1.5 (separate dogfood) ships the *visual builder*: form-driven
editor at `/workflows/edit/<path>`, save action with server-side
validation, per-job posture + required_review_postures widgets,
flash banner + redirect-after-save.

## 1.13.0 — 2026-05-09

### Added

- RFC 0023 V1.5 (dogfood-022): chat tool use + system-prompt briefing.
  Closes the V1.5 deferral from RFC 0023 V1.
  - **Six closed-set read-only tools** wired into the chat backend:
    `read_file(path)`, `list_dir(path)`, `striatum_status(run_id?)`,
    `striatum_why(target_id)`, `git_log(limit?)`,
    `git_diff(path?)`. The model decides when to call them; the
    backend executes server-side and feeds results back. Closed-
    set membership enforced in `execute_tool`; unknown tool names
    return error strings rather than executing. No tool that
    mutates state.
  - **Tool-call loop** in `_handle_chat_send`: up to 10 iterations
    of (request → assistant text + tool calls → execute → re-request
    with results). Loop terminates on a no-tool-calls response.
  - **System-prompt briefing** at chat-session creation: repo path,
    current branch, last 10 commits, top-level entries, AGENTS.md
    content (capped at 8 KB), active-run summary, tool-use
    guidance. The chat now has bearings on its first turn.
  - **Per-flavor tool wiring**: Anthropic Messages tool-use shape
    (content blocks with `type: "tool_use"` + `tool_result`) and
    OpenAI Chat tool-use shape (`tool_calls` + `role: "tool"`)
    both supported. Streaming tool calls are accumulated server-
    side and emitted as discrete events.
  - **JSONL transcript extensions**: new role values `tool_use`
    and `tool_result` persist tool calls + their wrapped results.
    Existing user/assistant/system roles unchanged.
  - **Prompt-injection defense**: tool results are wrapped in
    `<tool_result_begin name="..." args="..."> ... <tool_result_end>`
    delimiters. The system briefing instructs the model to treat
    content between the delimiters as data, not instructions
    (defense in depth; closes design-review F1).
- Chat history page now renders `tool_use` and `tool_result`
  entries as collapsed-by-default `<details>` blocks alongside
  user/assistant turns.

### Fixed

- **Graph-node click 404** (RFC 0022 V1 regression): SVG graph
  nodes link by *workflow* job id (e.g., `research_chat`) but
  the `/run/<id>/job/<id>` route handler queried by the *full*
  job id only. The handler now accepts either form.
- **Doctor page rendered no list**: the template referenced
  `doctor.checks` but the `doctor()` function returns
  `doctor.problems` (list[str]) and `doctor.problem_records`
  (list[dict]). Template rewritten to render the actual shape;
  CSS for the problem list added.
- **Chat double-render of user messages**: the JS island
  optimistically appended the user's message on form submit, then
  the SSE round-trip rendered the same message a second time
  (with timestamp). Optimistic append removed; the SSE stream is
  now the single source of truth for message rendering. ~250ms
  perceived latency before the user's own message appears, no
  duplication.

## 1.12.0 — 2026-05-09

### Added

- RFC 0023 V1 (dogfood-021): web chat surface +
  `/view/<path>` endpoint + inline Markdown rendering on
  artifact pages. Provider-neutral chat client streams HTTP
  to an operator-configured endpoint via four env vars
  (`STRIATUM_CHAT_API_BASE_URL`, `STRIATUM_CHAT_API_KEY`,
  `STRIATUM_CHAT_MODEL`, `STRIATUM_CHAT_API_FLAVOR`). Two
  flavors: `anthropic_messages` and `openai_chat` (covers
  OpenAI, OpenRouter, Ollama, vLLM, LiteLLM proxy, etc.).
  No default provider; operators opt in explicitly. URL
  scheme validation refuses non-loopback `http://`. Chat
  startup is `--allow-mutations`-gated.
- `/view/<path>` read-only file viewer: `.md` renders as
  HTML, text as `<pre>`, binaries as a metadata panel.
  Path traversal refused; `.git/` and `.striatum/` hidden
  by default. Directory listings deferred to V1.5.
- `/run/<id>/artifact/<id>` now renders `.md` artifact
  bodies inline (closes RFC 0022 V1.5 deferred).
- Chat transcripts in `.striatum/scratch/chat-<id>/transcript.jsonl`
  (gitignored). SQLite unchanged. No artifacts published.

### Dependency

- **`markdown-it-py` ≥ 4.0** is now a runtime dependency
  (the project's second after Jinja2). `html: False` at
  parse time; no separate sanitizer needed for V1.

### Boundary clarification (D074)

- AGENTS.md "no cloud APIs without explicit product
  decision" gets its first carve-out: outbound HTTP from
  striatum to an operator-configured endpoint is permitted
  for chat (and only chat). No hosted striatum service; no
  default endpoint; no telemetry. D028 (transcripts off)
  gets a parallel narrow carve-out: chat transcripts live
  in scratch JSONL only, never SQLite, never artifacts.

### Dogfood pattern (first 3-lane review)

- dogfood-021 declares three parallel design-review jobs
  (security, devils_advocate, threat_model) and three
  parallel build-review jobs (security, devils_advocate,
  ergonomics_dx) — first run to use RFC 0018 V1's
  `required_review_postures` reachability gate at full
  3-posture coverage.

## 1.11.1 — 2026-05-09

### Changed (docs only)

- Refresh the documentation set against the current state
  (RFCs 0001–0022, v1.11.0 features). Mention
  `--with-ddd-layout` (RFC 0021) + `--ddd-layout-force` /
  `--ddd-layout-dry-run` (V1.5) in `README.md`,
  `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`, and
  `docs/CLI_REFERENCE.md`. Update `README.md` "Status" section
  from `v1.1.0` to `v1.11.0` + add the PyPI `pip install
  striatum-orchestrator` instructions. Rewrite `docs/SPEC.md` §
  "Local Web UI" against the RFC 0022 V1 server-rendered shape
  (Jinja2 multi-page, SVG dependency graph, dark-mode CSS
  custom properties).
- Apply explicit "historical" banners to incubation-era
  documents per `docs/CONTEXT_HYGIENE.md` § "Failure modes" #1
  (mixed live/historical material with no label):
  `docs/INTERVIEW_LOG.md`, `docs/PRIOR_ART.md`,
  `docs/RFC_0014_DOGFOOD_FIX_SPEC.md`, and
  `docs/dogfood/HISTORICAL.md`. The `docs/INDEX.md` table-of-
  contents now lists these in a dedicated "Historical" section
  with a header-level callout, separating them from active
  reference material.
- `docs/dogfood/HISTORICAL.md` extended with a "current
  cadence" subsection listing recent runs (014–020) and what
  each shipped (RFC + tag + highlights), so a reader can find
  a recent canonical run instead of copying patterns from the
  incubation-era 001–013 directories.

No behavior change; no schema change; no new tests.

## 1.11.0 — 2026-05-09

### Added

- RFC 0022 V1 (dogfood-020): web UI redesign. Server-rendered
  Jinja2 multi-page UI replaces the hash-routed SPA. Five pages:
  `/`, `/run/<id>`, `/run/<id>/job/<id>`,
  `/run/<id>/artifact/<id>`, `/doctor`. Each page is real HTML
  that copy/pastes cleanly and works without JS. The JSON API
  (`/v1/*`) and SSE feed (`/events`) are unchanged.
- Refreshed visual palette: CSS custom properties for theme +
  status colors, `prefers-color-scheme: dark` media query for
  dark mode (no toggle button — OS preference wins), system
  font stack, 4px-grid spacing scale. New `base.css` replaces
  `app.css`.
- SVG dependency graph on `run_detail.html`: layered top-down
  layout (longest-path topological depth), state-colored nodes
  via custom-property `fill`, click-to-navigate to job detail,
  SVG `<title>` tooltip on hover for accessibility. Cycles
  (revision loops) are not rendered as edges — only the forward
  DAG from `workflow_graph_data().graph.edges`.
- Legacy hash-route redirect: a small JS island in `base.html`
  reads `window.location.hash` on load and rewrites
  `#/run/<id>` to `/run/<id>` so bookmarked SPA URLs still
  work.

### Dependency

- **Jinja2 ≥ 3.1** is now a runtime dependency (the project's
  first; previously zero-runtime-dep). Adds ~250 KB to the
  install size, pulls in `markupsafe` (~30 KB transitively).
  Trade-off taken for HTML correctness over hand-written
  string-format escaping.

### Removed

- `src/striatum/web/static/app.js`'s hash-router and the
  associated SPA mount. The mutation-button JS is preserved as
  a per-page island. The CSP header is byte-identical
  (`default-src 'self'; …` with no `unsafe-inline` / `unsafe-eval`).

### Deferred to V1.5

- Inline dogfood Markdown rendering on `/run/<id>/artifact/<id>`.
- SVG graph zoom / pan interactivity.

## 1.10.0 — 2026-05-09

### Added

- RFC 0021 V1.5 (dogfood-019): `--ddd-layout-force` and
  `--ddd-layout-dry-run` flags on `striatum init
  --with-ddd-layout`.
  - `--ddd-layout-force` overwrites existing regular-file
    targets with the template body. The envelope reports
    `status: "overwritten"` plus a `prior_sha256` field for
    audit. Non-regular-file targets (directories, broken
    symlinks) still surface as `status: "error"` regardless
    of force — the operator must resolve those manually.
  - `--ddd-layout-dry-run` reports what *would* happen without
    writing any files. The envelope's top-level `dry_run` flag
    is True; per-file statuses use a `would_*` vocabulary
    (`would_create`, `would_skip`, `would_overwrite`,
    `would_error`). Combine with `--ddd-layout-force` to
    preview a destructive overwrite.
  - Both flags without `--with-ddd-layout` are silent no-ops.
- `scaffold_ddd_layout(repo, *, force, dry_run)` public API
  signature is unchanged from V1; V1's `force=False,
  dry_run=False` defaults map to V1's behavior. Callers that
  pass either flag get the new V1.5 branches without
  deprecation work.

RFC 0021 status moves from `accepted (V1)` to
`accepted (V1+V1.5)`. V1.6 candidates (template parameter
substitution, multi-layout, `striatum scaffold sync`, doctor
check) remain deferred until operator evidence shows they're
wanted.

## 1.9.0 — 2026-05-09

### Added

- RFC 0018 step 3 V1.5 (dogfood-018): `verdicts.posture` column
  + introspection surfacing across six paths.
  - Migration v10 ALTERs `verdicts` to add a `posture TEXT NOT
    NULL DEFAULT 'neutral'` column and a covering index
    `idx_verdicts_posture`. Existing rows backfill to
    `'neutral'`. Forward-only; idempotent.
  - `record_review_verdict` reads the review job's posture from
    the workflow snapshot (defaulting to `'neutral'` when
    omitted) and writes it on INSERT. The `verdict.recorded`
    event payload now carries `posture` alongside `verdict`.
  - `status --json` adds a `verdicts_by_posture` dict alongside
    the existing verdict counts. Always emitted (empty dict
    when no verdicts) for stable shape.
  - `run summary` Markdown adds a `[posture: \`<name>\`]` suffix
    on each per-build verdict line *only* when at least one
    non-neutral posture exists in the run. Posture-omitting
    runs render byte-identically to v1.8.1.
  - `evidence export` JSON snapshot includes `posture` on every
    verdict row.
  - `run graph --format json` adds `posture` to each review
    node's `latest_verdict` block (when a verdict exists).
  - Dashboard verdicts panel renders a `Postures: <p1>=<n1>,
    <p2>=<n2>` summary line when at least one non-neutral
    posture exists. Sorted by count descending, then posture
    name ascending for deterministic ties; truncates to the
    top-3 with `+N more` overflow.
  - Web UI verdict list renders a posture chip alongside each
    verdict badge for non-neutral postures. New
    `.posture-chip` CSS class with `max-width: 12em` +
    `text-overflow: ellipsis` for long `custom:<name>` strings;
    full posture name shows on hover via `title` attribute.

### Changed (intentional)

- `evidence export` JSON snapshot's per-verdict block now
  includes a `posture` field. Downstream consumers parsing the
  redacted snapshot by key name (e.g. `verdict`,
  `findings_artifact_id`) tolerate the additive field; consumers
  that rely on a fixed shape may need an update.

### Tests

- `tests/test_review_postures_introspection.py` (15 cases)
  covering migration idempotency, submit-review backfill across
  declared/undeclared/custom postures, and each of the six
  introspection surfaces (including byte-identical zero-
  regression assertions for posture-omitting runs).

## 1.8.1 — 2026-05-09

### Changed

- PyPI distribution renamed from `striatum` (taken on PyPI by an
  unrelated project) to `striatum-orchestrator`. Module imports
  (`import striatum`) and the `striatum` console script are
  unchanged. Operators upgrading from a hypothetical earlier
  install would `pip uninstall striatum && pip install
  striatum-orchestrator`; in practice no one was on PyPI before
  this release.

## 1.8.0 — 2026-05-09

### Added

- RFC 0021 V1 (dogfood-017): `striatum init --with-ddd-layout`
  scaffolds the seven canonical human-facing DDD documents
  (`docs/SPEC.md`, `docs/PRD.md`, `docs/DECISION_LOG.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DDD.md`,
  `docs/rfcs/README.md`, `docs/rfcs/0001-template.md`) into the
  target repo. Mirrors RFC 0015's `--with-skills` for agent-
  facing files: opt-in (default off, plain `striatum init`
  unchanged), idempotent (existing files reported as `skipped`
  with `reason: "exists"`), composable (both flags can be
  combined; scaffold runs after skills install). New
  `src/striatum/scaffold/` package with seven `.md.tmpl`
  templates shipped via setuptools package-data; `scaffold_ddd_
  layout(repo, *, force, dry_run) -> dict` envelope shape:
  `{"layout": "ddd", "files": [...], "dry_run": bool}`.
- Per-file safety: a target that exists but is *not* a regular
  file (directory, broken symlink, etc.) returns
  `{"status": "error", "reason": "target exists but is not a
  regular file"}` rather than silently `skipped`. OSError during
  write surfaces per-file as `status: "error"` without aborting
  the rest of the scaffold.

### Dogfooded

The dogfood-017 workflow itself uses RFC 0018 V1 fields for the
first time end-to-end: both review jobs declare
`review_posture: "devils_advocate"`, the build job declares
`required_review_postures: ["devils_advocate"]`, and the
workflow validator's reachability gate accepts the run.

## 1.7.0 — 2026-05-09

### Added

- RFC 0018 V1 (dogfood-016): focused adversarial review postures.
  Workflow review jobs accept a new `review_posture` field
  (closed set of nine first-class values:
  `neutral | devils_advocate | security | threat_model |
  latency_performance | ergonomics_dx | accessibility |
  compliance_license | supply_chain`, plus a `custom:<name>`
  grammar for off-list flavors). Build jobs accept a new
  `required_review_postures: [...]` list declaring which postures
  must cover the build. The work-packet `review_policy` block
  exposes `posture` when declared and appends a deterministic
  posture-specific instruction sentence to `instruction` for
  first-class postures. The workflow validator walks the directed
  edge graph in both directions from each build with
  `required_review_postures` and refuses (exit code 8) when any
  required posture is not the `review_posture` of a reachable
  review job.

### Design note

The runtime build-completion gate as written in the original RFC
text deadlocks against striatum's lifecycle (a build's `complete`
mutation precedes its downstream review's verdict by
construction); D069 / V1_ACCEPTANCE record the re-cast to a
workflow-validation gate. Today's edge-verdict mechanism plus
existing run-completion semantics preserve runtime enforcement.
RFC 0018's text is patched to match.

### Deferred

RFC 0018 step 3 (`verdicts.posture` column + introspection
surfacing in `status`, `run summary`, `evidence export`,
`run graph --format json`, dashboard, web UI) remains deferred
to V1.5 per the RFC's own implementation path.

## 1.6.0 — 2026-05-09

### Added

- RFC 0020 step 3 (dogfood-015): `striatum recovery watch
  --run-id <id>` long-lived sweeper daemon. Wraps the existing
  `recovery auto` orchestrator in a sleep loop with single-
  instance pidfile (`.striatum/scratch/recovery-watch-<run_id>.pid`),
  `SIGTERM`/`SIGINT` signal-driven shutdown, JSONL emission per
  sweep + a final `watch_exit` envelope, exit-on-terminal default,
  `--max-sweeps` cap, and the same CLI overrides as `recovery
  auto`. Stale pidfiles (dead PIDs) are overwritten cleanly;
  active-PID collisions exit 4 with a clear message. New
  `src/striatum/recovery/watch.py`. Tests at
  `tests/test_recovery_watch.py` (8 cases, including a SIGTERM
  shutdown test that interrupts a long sleep). RFC 0020
  transitions to `accepted (V1)` — the "step 3 deferred"
  qualifier drops.

## 1.5.0 — 2026-05-09

### Added

- RFC 0019 (D067): `docs/DDD.md` documents striatum's domain-
  driven framing — bounded context, ubiquitous language,
  aggregate roots, value objects, domain events, the
  original CLI-only write-boundary invariant, and an "Adding to the
  model" section that gives future RFCs a citation pattern.
  README `## What It Is For` cites it; `docs/INDEX.md` lists
  it; the RFC template gets an optional `## Domain Modeling`
  section. Documentation only.

- RFC 0020 V1 (dogfood-014): autonomous stalled-run recovery
  step 1+2. New `striatum recovery auto --run-id <id>` one-shot
  sweeper composable with cron / systemd timer; runs lazy lease
  expiry, optional process reconciliation, autonomous review-
  only requeue (D036-safe), human_checkpoint timeout escalation,
  and eligible-blocker doctor flagging. Returns a structured
  envelope `{run_id, swept_at, policy_source, dry_run, actions,
  escalations, still_stuck}`. New optional top-level
  `recovery_policy` workflow block with workflow-declared
  thresholds and an `escalation_hook` (kinds: `marker_file`,
  `webhook`, `shell`); validator rejects `.striatum/` marker
  paths, non-http(s) webhook URLs, and negative thresholds.
  Defaults preserve today's flow byte-for-byte
  (`autonomous_*` defaults are `false`; CLI flags
  `--autonomous-review-requeue` and
  `--autonomous-process-reconcile` opt in per sweep).
  Hook runners (`marker_file`, `webhook`, `shell`) emit a status
  dict that folds into the envelope's `escalations[]`; webhook
  failures continue the sweep without raising. New
  `src/striatum/recovery/` package (`auto.py`, `hooks.py`,
  `policy.py`). Tests at `tests/test_recovery_auto.py` (21
  cases). Step 3 (`recovery watch` daemon) deferred per RFC
  0020 § 4.

## 1.4.1 — 2026-05-09

### Added

- Web UI run-level artifact rollup. The run-detail view now
  shows every published artifact for the run as a table (kind,
  logical name, path, source job, byline, timestamp, sha256
  prefix). Clicking the logical name routes to the existing
  artifact viewer; clicking the source job routes to the
  job-detail view. New endpoint `GET /v1/runs/<id>/artifacts`
  wraps the existing read-only `striatum list artifacts
  --run-id <id>` verb. The change is purely additive — it
  closes the discoverability gap from RFC 0013 V1+step 7 where
  per-run Markdown (BUILD_HANDOFF, DESIGN_SYNTHESIS, RUN_SUMMARY,
  decisions, findings) was reachable only by drilling into the
  job that produced it. 3 new tests at `tests/test_web_ui.py`
  (16 total).

## 1.4.0 — 2026-05-08

### Added

- RFC 0013 step 7 (dogfood-013): web UI mutation buttons.
  `POST /v1/invoke` was already gated by `--allow-mutations`
  (RFC 0012); step 7 adds five click-driven buttons to the SPA
  that POST the same argv shapes:
  - **Continue blocker** / **Cancel blocker** on the job-detail
    view (when an open blocker is present); maps to
    `striatum checkpoint resolve --blocker-id <id> --action {continue, cancel}`.
  - **Record verdict** on review-job detail (when state =
    running); collects verdict + rationale + session/lease ids
    and maps to `striatum verdict ...`.
  - **Record decision** on the run-detail view (always
    available; no lease required); maps to
    `striatum decision record ...`.
  - **Requeue stale review** on stale-lease review-only jobs;
    maps to `striatum recovery requeue-stale ...`.
  Each button opens a confirmation modal showing the literal
  argv before firing; destructive actions (cancel job, reject
  verdict) get a red confirm button. `/v1/health` gains an
  `allow_mutations: bool` field the SPA caches once per page
  load to hide buttons when the gate is off; the runner-side
  gate stays authoritative as defence-in-depth. CSP unchanged
  (no external deps, no `eval`, no inline handlers).
  Tests at `tests/test_web_ui.py` (5 new cases, 13 total)
  cover health-flag both states, mutation refusal without the
  flag (HTTP 405 envelope), SPA wiring grep, and the
  no-external-URL invariant.

## 1.3.0 — 2026-05-08

### Added

- RFC 0016 step 3 (dogfood-012): Unicode `fancy` graph style +
  `--graph-orient {tb, lr}`. The dashboard graph panel and
  `striatum run graph --format ascii` now support box-drawn
  rendering with portable BMP characters (`┌`, `┐`, `└`, `┘`, `─`,
  `│`, `╌╌▶` for cycle back-edges) and a left-to-right layout
  that arranges layers as columns instead of rows. Both upgrades
  fall back deterministically: `fancy → layered` when per-slot
  width drops below 14, `lr → tb` when per-column width drops
  below 14. Color path unchanged; `_format_fancy_box` wraps the
  inner content (not the box frame) so the frame stays uniform
  across states. New flags on both `dashboard` and `run graph`:
  `--graph-orient {tb, lr}` (default `tb`) and the existing
  `--graph-style` choices now include `fancy` as a real renderer.
  8 new tests in `tests/test_dashboard.py` (23 total).

## 1.2.0 — 2026-05-08

### Added

- RFC 0015 step 3 (dogfood-011): codex + gemini skill profiles
  + `--profile all`. `striatum skills install --profile codex`
  writes five Markdown files at `.codex/agents/striatum-*.md`
  reusing the Claude Code skill bodies verbatim.
  `--profile gemini` writes a single
  `striatum-STRIATUM_GEMINI_GUIDE.md` (single-guide fallback per
  RFC 0015 § "Profile coverage" until Gemini CLI's skill
  convention stabilizes; the dedicated filename keeps
  `--profile all` collision-free with `generic`).
  `--profile all` fans out across the four first-class profiles
  (`claude_code, codex, gemini, generic`) in deterministic
  order, returning a `{"profile": "all", "results": [...]}`
  envelope. `striatum init --with-skills all` works the same
  way. Doctor's `skills_missing` / `skills_outdated` checks now
  cover every profile. Tests at `tests/test_skills_install.py`
  (10 new cases, 25 total) cover idempotent regeneration,
  manifest shape, edit detection, fan-out, and template-SHA
  parity for the new profiles.

## 1.1.0 — 2026-05-08

### Changed

- RFC 0017 V1 (dogfood-010): documentation reorganization. README
  trimmed from ~1,000 lines to 125 with seven canonical sections
  (Status, Install, Quick Start (Human Operator), Quick Start
  (Coding Agent), What It Is For, Documentation Map, License).
  Behavior model, sequential 1–11 usage walkthrough, dogfood-NNN
  history, per-RFC subsections, and command reference moved out
  of the README into `docs/GETTING_STARTED.md`,
  `docs/HOW_TO_HUMAN.md`, `docs/HOW_TO_AGENT.md`,
  `docs/WRITING_WORKFLOWS.md`, `docs/CLI_REFERENCE.md`,
  `docs/INDEX.md`, and `docs/dogfood/HISTORICAL.md`. AGENTS.md
  slimmed (153 → 104 lines) to point at `docs/HOW_TO_AGENT.md`
  rather than reciting the verbs inline. Three new tests in
  `tests/test_doc_links.py` enforce relative-link integrity, the
  README line budget, and the human/agent quick-start heading
  split. Documentation only — no behavior change, no schema move.

## 1.0.0 — 2026-05-08

First stable release. Every RFC under `docs/rfcs/` is now in an
`accepted` (or `accepted (V1)`) state, and every V1 RFC has shipped
its implementation slice. The `0.x` line tracked individual RFC
landings on top of the V1 MVP baseline; `1.0.0` is the version the
runner exposes once the full V1 surface is on main.

### Highlights since 0.1.0

- **RFC 0006** — forward-only SQLite migration system (`PRAGMA
  user_version`); a database newer than the runner exits with
  code 9.
- **RFC 0007** — workflow visualization (`workflow graph` and
  `run graph` with Mermaid / JSON / Graphviz DOT / state-annotated
  ASCII output).
- **RFC 0008** — opt-in per-job git worktree isolation
  (`worktree create | release | list`) for parallel repo-write
  jobs.
- **RFC 0009** — long-lived process supervision
  (`supervise start | send | stop | status | list`) so an agent
  CLI can be held alive across multiple work packets.
- **RFC 0010 V1+V1.5+V2** — tool harness profiles surfaced on work
  packets, plus the reference Claude Code supervised wrapper at
  `.striatum/bin/claude-supervised-wrapper.sh`.
- **RFC 0011** — explicit session close + run-terminal auto-close
  (`session close`); doctor's `active_session_on_terminal_run`
  warning now clears by construction on clean-finish runs.
- **RFC 0012 V1** — local HTTP / Unix-socket service
  (`striatum serve`) with SSE for events and a mutation gate
  (`--allow-mutations`).
- **RFC 0013 V1** — local web UI: vanilla-JS SPA bundled at
  `src/striatum/web/static/` and served by `striatum serve --web`.
- **RFC 0014 V1** — process adapter completion guarantees
  (post-exit output validation, structured blocker payloads,
  `recovery process-reconcile`, doctor `process_*` checks). Closed
  [issue #1](https://github.com/halbritt/striatum/issues/1).
- **RFC 0015 V1** — self-contained agent skill bundles
  (`striatum skills install`, `init --with-skills`, doctor
  `skills_missing` / `skills_outdated`).
- **RFC 0016 V1** — live dependency graph panel in
  `striatum dashboard`; `run graph --format ascii` reuses the same
  pure renderer for one-shot snapshots.
- **Reviewer policy & artifact contracts** — RFCs 0002/0003/0004/0005
  shipped reviewer access scope + context policy fields, support
  ledgers, action-item ledgers, and harness improvement proposals
  with V1 front-matter schemas under `striatum.artifacts`.

### Tooling

- 50 source modules under `src/striatum/`, 260 tests under
  `tests/`, lint + mypy clean. The Makefile targets `install`,
  `lint`, `typecheck`, `test`, `smoke` are the supported entry
  points.
- `pyproject.toml`'s `[tool.setuptools.package-data]` ships the
  web SPA (`striatum.web.static`) and the agent skill templates
  (`striatum.skills.templates`) with the wheel.

### Notes for upgraders

- The `1.0.0` jump from `0.5.0` is purely a release-naming change;
  every behavior in `1.0.0` already shipped on main as part of the
  `0.2.0`–`0.5.0` line.
- The `striatum.workflow.v1`, `striatum.work-packet.v1`,
  `striatum.skills.manifest.v1`, and the per-kind front-matter
  schema versions remain V1; future schema changes will continue
  to use V1.x suffixes or new V2 schemas behind explicit RFCs.

## 0.5.0 — 2026-05-08

### Added

- RFC 0015 V1 (dogfood-009): self-contained agent skill bundles.
  New `striatum skills install [--profile {claude_code, generic}]
  [--scope {project, user}] [--namespace <prefix>] [--force]
  [--dry-run]` writes a Markdown bundle into the target tree that
  teaches a Striatum-aware agent how to drive the runner without
  reading the source repo. The Claude Code profile produces five
  skills (`striatum-workflow` router plus `striatum-scaffold`,
  `striatum-claim-loop`, `striatum-supervise`, `striatum-recover`)
  under `.claude/skills/<namespace>striatum-*/SKILL.md`; the
  generic profile produces a single
  `<namespace>STRIATUM_AGENT_GUIDE.md` for any agent CLI without a
  skill-discovery convention. Each install records a
  `striatum.skills.manifest.v1` JSON manifest with the rendered
  SHA256, the bundled-template SHA256, and the runner version per
  file. A re-install is byte-identical; an operator-edited file is
  `refused_modified` without `--force`; `--dry-run` writes nothing
  and prints the plan. New `striatum init [--with-skills [profile]]`
  flag runs the same install pipeline immediately after `init`.
  New doctor checks `skills_missing` (recorded file absent on disk)
  and `skills_outdated` (manifest version older than running
  install, or template SHA drift) surface the exact `skills install`
  invocation that would clear the condition; the runner never
  auto-regenerates. The bundle emits no external URLs (a unit test
  enforces no `http://` / `https://`) and ships inside the Python
  distribution via `[tool.setuptools.package-data]`. Tests at
  `tests/test_skills_install.py` (16 cases). `__version__` bumped
  to 0.5.0 (alongside the pyproject bump). The `codex` and
  `gemini` profiles plus `--profile all` and parser-walked verb
  tables are step 3 of the RFC's path and remain deferred.

## 0.4.0 — 2026-05-08

### Added

- RFC 0016 V1 (dogfood-008): live dependency graph panel in
  `striatum dashboard`. The frame now appends a layered ASCII view
  of the run's workflow graph annotated with current job state
  (`Q`/`R`/`C`/`B`/`H`/`F`/`P`/`X`/`S`) when the terminal is at
  least 100 columns wide and 30 lines tall and the workflow has at
  least one edge. Auto-detection can be overridden with `--graph` /
  `--no-graph`; `--graph-only` hides the rest of the frame for
  graph-first viewing; `--graph-style {auto,layered,list,fancy}`
  forces a layout (`fancy` falls back to `layered` in V1);
  `--graph-no-cycles` suppresses dashed `~~>` back-edges. ANSI 16
  colors quantize the existing Mermaid state palette and are gated
  on `isatty()` plus `NO_COLOR` (de-facto standard). New
  `striatum run graph --format ascii` reuses the same pure renderer
  for one-shot snapshots. Refactor: `compute_node_states(conn, *,
  run_id)` lifted from `cli/introspect.run_graph` to
  `striatum.workflow` so the dashboard and the existing graph CLI
  share one source of truth for "current state after a requeue."
  Tests at `tests/test_dashboard.py` (11 new cases covering
  layered/list/no-cycles/color/no-color/graph-only/ASCII format
  parity and an ANSI-table-vs-Mermaid-fills coverage guard).

## 0.3.0 — 2026-05-08

### Added

- RFC 0013 V1 (dogfood-007): local web UI. Bundled vanilla-JS SPA at
  `src/striatum/web/static/{index.html,app.js,app.css}` served by
  `striatum serve --web` (no-op flag in 0.2.0; now serves the real
  UI). Five views: run list, run detail with live SSE event log,
  job detail, artifact viewer with per-kind front-matter formatting
  (decision badge, finding verdict + severity chip,
  harness-improvement-proposal target chip, synthesis input list),
  and doctor. Tiny in-house Markdown renderer with HTML escaped at
  the input boundary; no external CDN imports; CSP header on every
  static and artifact-raw response. New endpoint
  `GET /v1/artifacts/<id>/raw` streams artifact bytes for the
  viewer. Static assets ship inside the wheel via
  `[tool.setuptools.package-data]`. Tests at
  `tests/test_web_ui.py` (8 cases). Mutation buttons (step 7 of
  the RFC) deferred.

### Fixed

- CI release-metadata check now sources the expected version from
  `pyproject.toml` instead of a hardcoded constant, so version
  bumps don't require touching the script.
- Test service-readiness window bumped to 30s so cold imports on
  macOS GitHub runners don't false-fail.
- Unix-socket service test uses a short `tempfile.mkdtemp` path so
  macOS's ~104-byte AF_UNIX limit doesn't trigger.

## 0.2.0 — 2026-05-08

First tagged release since the V1 scaffolding. The backlog of RFCs
landed before this point (run recovery / dogfood fixes, reviewer
independence policy, support ledgers + critique-to-action loops +
harness meta-optimization, SQLite migrations, workflow
visualization, worktree isolation, long-lived process supervision,
tool harness profiles V1+V1.5+V2, session close + auto-close,
process adapter completion guarantees) is treated as the `0.1.0`
baseline. `0.2.0` lands RFC 0012 V1 on top of that baseline as the
first explicitly versioned release. Subsequent RFCs bump the minor
version on landing.

### Added

- RFC 0012 V1 (dogfood-006): local HTTP / Unix-socket service. New
  `striatum serve` command runs a `ThreadingHTTPServer` on TCP
  loopback (default `127.0.0.1`) or a Unix-domain socket; refuses
  non-loopback hosts at startup with exit 8. Endpoints:
  `/v1/health`, `POST /v1/invoke`, `/v1/runs`, `/v1/runs/<id>`,
  `/v1/runs/<id>/why`, `/v1/runs/<id>/dashboard`,
  `/v1/runs/<id>/events` (SSE), `/v1/doctor`. Mutations gated
  behind `--allow-mutations` (whitelist of read verbs); auth via
  filesystem permissions on Unix sockets or optional `--token` on
  HTTP (length-safe constant-time compare). Single-instance via
  PID file; graceful shutdown on SIGTERM/SIGINT. New module
  `src/striatum/service.py`; tests at `tests/test_service.py` (16
  cases). Closes the long-standing D006 promise of an "optional
  Unix-socket / local HTTP API later for Slack, TUI, and web
  adapters" — the four V1 acceptance criteria all pass.
- RFC 0014 V1 / issue #1 (dogfood-005): process adapter completion
  guarantees. After every `striatum adapter run` exit (including
  timeout-fired SIGTERMs), the runner inspects required
  `expected_artifacts` and, for review jobs, the verdict table. When
  any required output is missing — or the child exited non-zero or
  hit the timeout — the job transitions from `running` to `blocked`,
  a blocker row is inserted with a structured `blocker_kind`
  (`process_outputs_missing`, `process_review_verdict_missing`,
  `process_exit_nonzero`, `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`), and a privacy-safe diagnostic
  envelope is recorded as the new `blockers.payload_json` column.
  The envelope contains zero child stdout/stderr (D028 preserved); it
  carries `process_id`, `command`, `exit_code`, `duration_seconds`,
  `timeout_seconds`, `missing_artifact_paths`, `review_verdict_missing`,
  and operator-copyable `recovery_commands`. New CLI surface:
  `striatum adapter run --timeout-seconds <n>` (overrides
  `lanes.<id>.adapter_timeout_seconds`; capped at 86400) and
  `striatum recovery process-reconcile --run-id <id>` (mirrors the
  `recovery requeue-stale` lazy-on-CLI shape from D036). Two new
  doctor checks (`process_running_but_pid_gone`,
  `process_running_with_expired_lease`) and a `process_health`
  summary on `striatum status --run-id`. Migrations v8
  (`process_executions.state` enum + `'timed_out'` and `'lost'`) and
  v9 (`blockers.payload_json`); both idempotent against fresh DBs.
  Tests at `tests/test_process_adapter.py` (15 new cases). Closes
  [issue #1](https://github.com/halbritt/striatum/issues/1).
- `branch.mode` is now a closed enum (`"auto"` or `"confirm"`) and
  defaults to `"auto"` when omitted. In auto mode, `run prepare`
  atomically creates the suggested branch and transitions the run to
  `ready`, eliminating the separate `striatum branch confirm --create`
  step that was previously required. The response includes
  `branch_mode`, `branch`, `branch_created`, `current_git_branch`, and
  any warning. Workflows that explicitly want the manual gate can set
  `branch.mode: "confirm"`; behaviour there is unchanged. If git
  checkout fails during auto mode (dirty tree, conflicting branch),
  the run falls back to `needs_branch_confirmation` so the operator
  can resolve the issue and run `branch confirm` manually. Migrated
  the in-repo dogfood-001/-001-v2/-002/-003/-004 and the
  `examples/harness-profiles/` workflows to auto mode; remaining
  example fixtures keep `mode: "confirm"` for test-coverage symmetry.
  Five new tests in `tests/test_cli_mvp.py` cover the auto path,
  default-when-omitted, the still-functioning confirm path, unknown
  mode rejection, and the auto-without-suggested-name guard.
- RFC 0010 V2 / HARNESS-001 (dogfood-004): reference Claude Code
  supervised wrapper at `.striatum/bin/claude-supervised-wrapper.sh`.
  Bash `while IFS= read -r` loop that spawns a fresh `claude --print`
  per packet — each Striatum work packet is independent, so per-packet
  fresh-context matches the workflow's `fresh_session_required`
  defaults and avoids depending on Claude Code's undocumented
  multi-turn `--input-format stream-json` behaviour. Inner stdout
  and stderr go to `/dev/null` (RFC 0009 / D028); SIGTERM trap
  cleans up the in-flight inner process. Verification test at
  `tests/test_claude_supervised_wrapper.py` (4 cases, stub-claude on
  `$PATH` so it does not depend on the real binary). Closes
  `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1.5 (HARNESS-001 follow-up): workflow-validate lint warning
  for missing repo-relative process-lane command paths. Fires when
  `lane.command[0]` looks like a repo-relative path (contains a slash
  or starts with `./`/`../`) and the file does not exist under the
  workflow's repo root. Surfaces under the `warnings` key in
  `workflow validate --json` and `workflow plan --json`. Non-blocking;
  bare binary names and absolute paths are not checked. Closes the V1.5
  step of `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1 (dogfood-003): optional `harness_profiles` workflow map
  and per-lane `harness_profile_id` reference. When a lane references a
  declared profile, `claim-next` adds a `harness_profile` block to the
  work packet (passthrough projection of the profile body plus
  `profile_id`). Workflows that omit `harness_profiles` produce
  unchanged packets. Validation accepts the closed tool-family set
  `{generic, codex, claude_code, gemini_cli}`, requires `tool_family`
  and `strategy_version`, and enforces D021 accountability
  (`native_subagents = internal_to_parent_session`,
  `first_class_registration = not_supported`). Unknown sibling fields
  on profile bodies are accepted as lint warnings, surfaced under a
  `warnings` key in `striatum workflow validate --json` and
  `workflow plan --json`. Reference fixture lives at
  `examples/harness-profiles/workflow.json`. Tests in
  `tests/test_harness_profiles.py` cover validation, packet exposure,
  backwards compatibility, and fixture loading (including the
  dogfood-003 four-profile fixture).
- D055 follow-ups (post-RFC-0011): `recovery cancel-job --cascade`
  over a whole run now transitions `runs.state` to `'canceled'`
  (previously `'completed'`) when no job actually completed; auto-close
  fires under `source: "run_canceled"`, matching the source enum value
  RFC 0011 reserved. A new `test_run_failed_auto_closes_active_sessions`
  rounds out the source-enum matrix by exercising the reject-verdict
  path that drives a run to `'failed'`. Migration helper
  `striatum.migrations.rebuild_table()` extracts the FK-safe rebuild
  pattern (PRAGMA foreign_keys OFF + IF EXISTS partial-state recovery
  + DROP/RENAME) so future migrations against tables with
  self-referential FKs do not re-discover the requirement; v7 is
  retrofitted onto the helper. v5 remains untouched as immutable
  historical record.
- RFC 0011 (dogfood-002): explicit session close + run-terminal
  auto-close. New `striatum session close --session-id <id> --reason
  <text>` command transitions an `active` session to a new `closed`
  terminal state, recording `closed_at` and a non-empty
  `close_reason` and emitting a `session.closed` event with
  `source: "explicit"`. Idempotent against already-terminal sessions
  (returns the existing row plus a `note: "session was already
  <state>"`); refuses with exit 4 when the session holds an active
  lease (message points the operator at `striatum release`). When a
  run transitions to a terminal state, every still-active session on
  the run is auto-closed inside the same transaction with `source` of
  `"run_completed"`, `"run_failed"`, or `"run_canceled"` — eliminating
  the persistent `active_session_on_terminal_run` doctor warning that
  fired on every clean-finish run before this change. Migration
  version 7 adds the `closed` state value plus the `closed_at` and
  `close_reason` columns. `evidence export` and `run summary` carry a
  per-session block with the new fields; `RUN_SUMMARY.md` gains a
  `## Sessions` section.
- HARNESS-001 fixes (dogfood-001 v2): `docs/SPEC.md` "Supervised lane
  command contract" subsection making the three supervised-lane
  requirements explicit (alive across packets, NDJSON stdin, calls back
  via `striatum` CLI). New `doctor` problem record
  `supervisor_lost_with_held_lease` plus the stable `status` next-action
  `recover_orphan_supervisor` that fires when a supervisor row is
  `lost` while the session still owns an unexpired active lease.
  `striatum supervise stop` is idempotent against an already-`lost` or
  `stopped` supervisor: returns the existing terminal row plus a
  `note` describing the prior state instead of raising
  `InvalidTransitionError`.
- HARNESS-002 fixes (dogfood-001 v2): new `doctor` problem record
  `editable_install_outside_repo` warns when the running install is
  outside the repo argument and the repo is itself a Striatum source
  tree (suppressed when the repo is just a target, to avoid false
  positives). `striatum init` against a fresh DB now refuses with exit
  3 when the repo's source-tree `LATEST_VERSION` is higher than the
  running install's, with a clear message pointing at
  `pip install -e <repo>`. `Makefile install` resolves the install path
  via `$(MAKEFILE_DIR)` so `make install` from any cwd installs *this*
  Makefile's directory in editable mode (the previous `pip install -e
  .` was cwd-dependent and silently pinned to a Claude Code worktree).
- HARNESS-003 fixes (dogfood-001 v2): `docs/SPEC.md` "Reviewer
  Independence (advisory)" and "Byline Integrity" subsections making
  the runner's enforcement boundary explicit. New `doctor` problem
  record `reviewer_independence_unverified` flags two observable
  breaches — sessions that share a supervisor pid, or a reviewer
  session running unsupervised on a run whose author is supervised.
  `register-session --role reviewer` refuses when the workflow
  declares `reviewer_context_policy: fresh` and an active author
  session already exists, unless `--force-non-fresh --reason "..."` is
  passed; the reason is recorded in the new
  `sessions.non_fresh_reason` column. `publish-artifact` records the
  artifact file's actual `author:` line in the new
  `artifacts.author_line` column (NULL when the file omits it);
  evidence exports and run summaries read the actual column so a
  missing byline renders as `author: <missing>` rather than the
  workflow's declared expected. Migration version 6 adds both columns.
- HARNESS-004 fix (dogfood-001 v2): `docs/dogfood/001/roles/reviewer.md`
  now points reviewer harness proposals at
  `docs/dogfood/001/review/HARNESS-NNN.md` (inside the review job's
  `write_scope.allowed_paths`) instead of `docs/dogfood/001/findings/`
  (which is the author's path and is rejected by the publisher with
  exit 6). `tests/test_harness_v2_fixes.py::test_reviewer_role_doc_paths_match_write_scope`
  walks every dogfood reviewer role doc and asserts each
  `HARNESS-NNN.md` instruction path is contained in the corresponding
  review job's allowed paths.
- `striatum workflow graph --format dot <workflow.json>` emits a Graphviz
  `digraph striatum_workflow { ... }` alongside the existing Mermaid
  (default) and JSON outputs. Same nodes, dependency edges, parallel
  groups (rendered as `subgraph cluster_<group>` blocks), and bounded
  `needs_revision` cycle edges (rendered as dashed arrows with the
  `max_iterations` count). Pipe through `dot -Tsvg` to render.
- Three new artifact kinds and front-matter schemas (RFCs 0003/0004/0005,
  accepted): `support_ledger` (`striatum.support_ledger.v1`),
  `action_item_ledger` (`striatum.action_item_ledger.v1`), and
  `harness_improvement_proposal`
  (`striatum.harness_improvement_proposal.v1`). Migration version 5 drops the
  SQL `CHECK (artifact_kind IN (...))` on the `artifacts` table; allowed kinds
  now live in `striatum.artifacts.ALLOWED_ARTIFACT_KINDS` and are enforced by
  `publish-artifact` (`ArtifactError`, exit 6) and workflow validation
  (`WorkflowError`, exit 8). Reference fixture
  `examples/support-ledger-flow/` exercises the produce -> support ledger ->
  evidence audit -> final review pattern; "evidence audit" is a workflow
  convention name, not a new `job_type`.
- Reviewer independence policy fields on review jobs (RFC 0002, D051).
  `type: "review"` jobs may declare `reviewer_access_scope`
  (`document_only` | `artifact_augmented` | `repo_level`) and
  `reviewer_context_policy` (`fresh` | `cross_round`). The validator
  rejects unknown values, rejects the fields on non-review jobs, and
  rejects the explicit `reviewer_context_policy: "fresh"` +
  `fresh_session_required: false` conflict. Setting
  `reviewer_context_policy: "fresh"` without `fresh_session_required`
  silently stores the prepared job row with `fresh_session_required = 1`.
  Work packets gain a `review_policy` block (`access_scope`,
  `context_policy`, `instruction`) only when the workflow declares at
  least one of the fields; existing fixtures produce identical packets.
  The `examples/rfc-0014-operational-artifact-home/workflow.json` fixture
  now labels its three independent root reviews as `document_only` and
  `fresh`.
- `striatum run graph --run-id <id> [--format mermaid|json]` renders the
  workflow graph for an existing run with each node colored by current job
  state. Mermaid output appends a `classDef` palette plus per-node `class`
  assignments (completed/running/claimed/acked/blocked/stale_lease/
  waiting_human/failed/canceled/queued/pending); JSON output adds
  `current_state`, `attempt`, and a `latest_verdict` block on review nodes.
  The runner picks the highest-`attempt` row per `workflow_job_id` so
  requeued attempts show their latest state.
- `striatum list ...` subcommand group for read-only enumeration of runs,
  sessions, jobs, artifacts, and workflow snapshots. Each command returns a
  stable `{"items": [...], "count": N}` envelope shaped from existing SQLite
  state. `list runs` joins `workflow_snapshots` to surface `workflow_id`;
  `list sessions --run-id <id>` accepts `--state`, `--role`, `--lane`;
  `list jobs --run-id <id>` includes the latest verdict for review jobs and
  accepts `--state` and `--workflow-job-id`; `list artifacts --run-id <id>`
  embeds the structured author byline and accepts `--kind`; `list workflows`
  reports loaded snapshots with their `content_sha256`. Every run-scoped
  variant applies the lazy lease-expiry sweep before reading.
- `striatum checkpoint resolve --blocker-id <id> --action {continue|cancel}
  [--decision-id <id>]` resolves an open `human_checkpoint` blocker:
  `continue` re-queues the affected job and emits `checkpoint.resolved`;
  `cancel` marks the affected job `canceled` and emits
  `checkpoint.canceled`. Optional `--decision-id` validates a run-level
  decision artifact and records it on the resolution event payload.
- `striatum recovery cancel-job --run-id <id> --job-id <id> --reason <text>
  [--cascade]` is the explicit operator cancel for a non-terminal job.
  Refuses terminal-state jobs and refuses jobs with blocked dependents
  unless `--cascade` is set, in which case dependents are canceled
  transitively in the same transaction.
- Supervised-aware `claim-next`: when the claiming session has an
  `attached` supervisor, the runner writes the freshly built packet
  through the supervisor's stdin pipe inside the same transaction,
  refreshes `heartbeat_at`, and emits a `supervisor.packet_delivered`
  event. The CLI response gains an optional `supervisor_delivery` field.
  Pipe-missing or write-fail transitions the supervisor to `lost` while
  still committing and returning the packet so the caller can recover.
- Optional per-kind Markdown front-matter validation in `publish-artifact` for
  `decision` (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
  `findings_ledger` (`striatum.findings_ledger.v1`), and `synthesis`
  (`striatum.synthesis.v1`). Front matter is read with a minimal
  `key: <json-value>` parser, validated only when present, and never rewritten
  by the publisher. Other artifact kinds remain unschemaed.
- New example fixtures: `examples/human-checkpoint-flow/` (analyze -> review
  -> decide, where the decide job is a `human_checkpoint`-typed job whose
  session calls `block --severity human_checkpoint` to surface an operator
  checkpoint and the operator records the decision via
  `striatum decision record --outcome accepted`), and
  `examples/adapter-unavailable-flow/` (a process-lane workflow that requests
  `network=enforced` and is rejected at validation because the process adapter
  only provides `advisory_strict` for that constraint). Both are covered by
  end-to-end tests in `tests/test_cli_mvp.py`.
- `striatum dashboard` command: a compact, dependency-free terminal dashboard
  over the existing SQLite state that summarizes run state, job counts,
  verdicts, open blockers, claimable work, deterministic next actions, and
  the most recent events. Supports `--refresh` for live mode and `--once` for
  one-shot rendering in scripts and CI.
- Long-lived process supervision (RFC 0009). New `striatum supervise
  start | send | stop | status | list` commands hold an agent CLI alive
  across multiple work packets: `start` forks the lane command with
  `start_new_session=True` and a per-supervisor named pipe at
  `.striatum/scratch/<supervisor_id>/stdin.pipe`, `send` delivers a stored
  work packet as a newline-terminated JSON line through that pipe, `stop`
  sends `SIGTERM` (then `SIGKILL` after a five-second grace), `status` probes
  liveness and lazily transitions stuck rows to `lost`, and `list` reports
  supervisors for a run. The single-shot `striatum adapter run` command is
  unchanged — both flows coexist. Migration version 4 adds the new
  `process_supervisors` table with a partial unique index enforcing "at most
  one active supervisor per session". `expire_leases` marks supervised
  sessions `lost` without auto-killing the OS process, and `striatum doctor`
  flags supervisors whose pid is gone or whose stdin pipe is missing from
  disk. Stdout and stderr are sent to `DEVNULL`; the supervisor never
  captures transcripts or parses agent output for workflow state, preserving
  D028 and D037.
- `striatum workflow init [--style minimal|review|code-change] <path>` writes
  a starter workflow tree (`workflow.json` plus `roles/` and `prompts/`
  stubs) that validates cleanly with `workflow validate`. Refuses to
  overwrite an existing path. The `review` default mirrors the
  `examples/code-change-flow/` shape with placeholder paths; `minimal` skips
  review; `code-change` adds a one-shot `needs_revision` cycle.
- New example fixtures: `examples/code-change-flow/` (draft -> review -> apply
  with a one-shot needs_revision cycle) and
  `examples/failed-review-revision-cycle/` (single review whose second
  needs_revision opens a configured human checkpoint).
- Opt-in per-job git worktree isolation for parallel repo-write jobs
  (RFC 0008). Lanes declare `worktree_isolation: per_job` and the runner
  advertises `worktree_required: true` plus the `striatum worktree create`
  command on matching work packets without auto-creating anything. New CLI
  subcommands `worktree create | release | list` manage the worktrees,
  `publish-artifact` reads files from the active per-job worktree but
  records logical repo-relative paths so artifacts stay valid main-branch
  provenance, lease expiry marks worktrees `abandoned` for operator
  inspection, and `doctor` flags orphaned and missing-on-disk worktree rows.
  Migration version 2 adds the new `job_worktrees` table.
- Forward-only SQLite migration system. Schema version is tracked through
  `PRAGMA user_version`, the current schema is registered as
  `user_version = 1`, `striatum init` and every connect apply pending
  migrations inside a single `BEGIN IMMEDIATE` transaction, and a database
  newer than the runner supports is refused with the new exit code 9.
- Fourth adapter enforcement level `advisory_strict` (between `advisory` and
  `enforced`). The process adapter graduates `network=forbidden` and
  `repo_scope=local_only` to `advisory_strict`: proxy env vars are scrubbed
  from the child env when network is forbidden, and
  `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels are set so
  cooperating agents can honor the policy.
- RFC 0009 (proposed) describing the V2 long-lived process supervisor for
  agent CLIs that span multiple work packets.

### Changed

- Split `striatum.cli` from a single ~3.5k-line module into a package
  (`src/striatum/cli/`) organized by concern: `parser`, `dispatch`,
  `mutations`, `introspect`, `evidence`, `run_summary`, `recovery`,
  `worktree`, `supervise`, and `workflow_init`. Public surface is preserved
  via re-exports in `striatum/cli/__init__.py`; the `striatum.cli:main`
  console entry point and `python -m striatum.cli` continue to work
  unchanged. Behavior is identical (pure refactor, all existing tests pass);
  cross-module helper calls that need to honor `monkeypatch.setattr` against
  `striatum.cli` use a lazy `from striatum import cli as _cli` lookup.
- `striatum doctor --verbose` now augments the historical string `problems`
  list with a `problem_records` list of structured rows. Each record carries
  a stable `check` name (e.g. `active_job_without_active_lease`,
  `stale_queue_message_claim`, `worktree_path_missing_on_disk`), the
  affected `id`, and a small `context` map. The string list is preserved
  verbatim so callers that already grep `problems` keep working.
- `striatum run summary` Markdown output now groups verdicts by review job
  with an attempt count and rolled-up prior verdicts, appends the structured
  author byline (`author: <role>-<model>-<ordinal>`) to each artifact line,
  surfaces the recorded branch alongside the current git branch with an
  explicit `(MISMATCH)` annotation when they differ, and prints a Timing
  block with `created_at`, `started_at`, `completed_at`, and wall-clock
  `duration`.
- Workflow validator now rejects cross-job expected-artifact path
  collisions, write-scope `allowed_paths` that overlap `forbidden_paths`,
  expected artifacts outside the job's write scope, unsound revision cycles
  whose target does not feed back into the cycle source through workflow
  edges, and parallel groups that mix `repo_write` with review-only jobs.
- Workflow validator emits a deprecation warning to stderr when jobs declare
  the legacy `needs` field; `edges` remains authoritative.
- Cycle resolution now redirects downstream dependencies to the new review
  attempt so jobs gated on the review verdict unblock once the new attempt
  accepts.
- MCP wrapper now speaks LSP-style `Content-Length` framing by default with
  automatic line-delimited fallback. Real MCP clients (Claude Desktop, IDE
  MCP integrations) can connect cleanly; existing line-delimited scripts and
  tests keep working unchanged. Added `python -m striatum.mcp --framing
  {auto,line,framed}` for operators that need to pin the wire shape.
- `striatum branch confirm` now honors the previously inert `--create` and
  `--use-current` flags and adds a new `--strict` flag. `--create` runs
  `git checkout -b <branch>` (with idempotent fallback to `git checkout`),
  `--use-current` records the actual current git branch, and `--strict`
  refuses to record unless the working tree already matches. Default
  behavior remains records-only, and the JSON response now includes `mode`
  and `created` fields.
- Replaced the evidence-export key-name blocklist with a default-deny policy
  registry. Any field not explicitly classified as `safe` in
  `EVIDENCE_POLICY` is redacted from exported Markdown, so future schema
  additions cannot silently leak agent or user prose.
- Pushed the `fresh_session_required` filter in `claim_next` into a single
  SQL query using a `NOT EXISTS` correlated subquery, replacing the
  per-candidate Python loop. Added covering index migration for
  `work_packets(run_id, session_id)`.

### Tooling

- (No tooling-only changes pending in this Unreleased window. Tooling work
  in this cycle is bundled with the feature commits above.)

## 0.1.0 - 2026-05-07

- Split Striatum from Engram with history preserved from the former
  `agent-runner/` incubation directory.
- Renamed the package, CLI, workflow schema, and repo-local state directory
  to `striatum`.
- Replaced the initial all-rights-reserved status with Apache-2.0 licensing.
- Added standalone project metadata, CI, and a fresh-clone smoke script.
- Added workflow planning, run-summary export, stale-lease recovery
  introspection, local API wrapper, and minimal process-adapter launch
  support.
- Added workflow graph export, bounded stale-work requeue, decision-artifact
  recording, a local MCP-like stdio wrapper, and explicit adapter
  enforcement validation.
- Added stricter release checks with `ruff`, `mypy`, wheel/sdist smoke, and
  installed package metadata validation.
