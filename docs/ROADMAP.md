# Striatum Roadmap

**Purpose:** This is the operator kickoff document. If you are picking up
Striatum cold — or resuming after a context compaction — read this first.
It sequences the deferred and blocked work in `docs/TODO.md`, the proposed
RFCs under `docs/rfcs/`, the open GitHub issues, and the in-flight
dogfood follow-ups; everything else is the authoritative status source.

This roadmap is *opinionated about ordering*. Items here exist in TODO,
RFCs, GH, or DECISION_LOG already — the roadmap only adds sequencing,
dependency edges, and "what would I do next" framing. Update on every
`vX.Y.0` version bump; treat as stale on minor bumps.

---

## 1. State as of 2026-05-17 (v1.55.0 + Unreleased remediation)

- **Latest tag:** `v1.55.0` is the latest released tag and
  `pyproject.toml` version. Current `main` also contains Unreleased
  architecture-remediation follow-through from 2026-05-17.
- **Latest substantive release:** v1.55.0 completed RFC 0048 V1.5 hardening,
  Schema v6 event-chain columns, capability-denial coverage, audit-chain row
  locking, append-only role-grant checks, and the inline helper wiring needed
  by recovery paths.
- **Current workstream:** TODO 61-64 / RFC 0068-0071 architecture remediation.
  D107 supersedes D105: the target is now a Go production daemon port, Python
  daemon retirement after parity, Python CLI/web clients where useful, and
  SQLite eradication from production and compatibility paths.
- **CI:** GitHub Actions has been backlogged during the 2026-05-17
  remediation commits. Treat latest-head CI failures as stop-the-line; queued
  and in-progress older runs are not by themselves blockers.
- **Active dogfoods:** none tracked as live in this roadmap. The current
  operator work is direct remediation/backlog execution, with dogfood-shaped
  coverage added where behavior changes.
- **Branches:** `main` is the active integration branch.

## 2. Just shipped (this week)

| Version | Scope | Notes |
|---:|---|---|
| v1.49.0 | RFC 0048 Phase A | Python PG-backed mutation handlers and router groundwork. |
| v1.50.0 | RFC 0048 V1.5 daemon accept loop | Unix-socket daemon RPC serving, role-provisioning runbook, and shutdown hygiene. |
| v1.51.0 | RFC 0048 Phase C dispatch | CLI verbs route through daemon RPC with fail-closed substrate plumbing. |
| v1.52.0 | RFC 0048 Phase C reads | Read-surface PG handlers for status/dashboard/list/run-summary/evidence/corpus paths. |
| v1.53.0 | Recovery and serve hardening | `requeue-stale --force --justification`, corrupted-state serve refusal, and `daemon doctor --explain`. |
| v1.54.0 | RFC 0048 Phase B read parity | Go read handlers plus Python PG-side stale-requeue message parity. |
| v1.55.0 | RFC 0048 V1.5 hardening + Schema v6 | Capability-denial matrix, audit-chain row lock, append-only grants, event-chain columns, and recovery inline-helper wiring. |
| Unreleased 2026-05-17 | Architecture remediation follow-through | Command authority guardrails, daemon-first web-service slices, escalation inbox foundation, PTY supervision hardening, and explicit product-decision blockers. |

## 3. Operator decision rules — read this before doing any work

These are historical operator patterns from recent dogfoods. Treat them as
recovery lore, not the default happy path.

### 3.1 Operator-on-behalf publish path (RFC 0046 V1, mandatory)

When an agent lane stalls but the on-disk artifact is valid:

```
striatum ack --session-id <S> --message-id <M> --lease-id <L>
striatum publish-artifact --session-id <S> --job-id <J> --lease-id <L> \
    --kind <K> --logical-name <N> --path <P> \
    --allow-no-process-execution \
    --override-rationale "<one-line reason>"
striatum verdict --session-id <S> --job-id <J> --lease-id <L> \
    --verdict <V> --rationale "<one-line reason>"
```

**Never** publish-on-behalf without `--allow-no-process-execution --override-rationale "..."`.
The 055b implementation now refuses model bylines without the override marker.
Every override gets audit-chained — that's the contract; respect it.

### 3.2 Operator verdict override (RFC 0046 V1)

When a reviewer's `needs_revision` is a packet-design artifact (e.g., the
review packet didn't include the fix-up's HANDOFF, so the reviewer correctly
refused on missing evidence), override after the fix-up dogfood ratifies:

```
striatum override-verdict --session-id <S> --job-id <J> \
    --verdict accept_with_findings \
    --auto-fresh-session \
    --rationale "<cite the fix-up dogfood commits + accepting reviewers>"
```

### 3.3 Fix-up dogfood pattern (054b → 055b → ...)

When an adversarial reviewer finds **V1 non-negotiable** violations:

1. Honestly submit the `needs_revision` verdict — do NOT override pre-fix.
   The run goes to `waiting_human` with a blocker.
2. Scaffold a `<N>b` fix-up dogfood whose implementer's spec is the
   adversarial REVIEW.md itself.
3. After 3-way build review of the fix-up ratifies the fixes, override
   the parent run's `needs_revision` verdict citing the fix-up's commits
   + accepting reviewers.
4. Both runs reach terminal `completed`. Ship as the parent run's version.

This pattern is in `docs/dogfood/054b/OPERATOR_REPORT.md` and `055b/OPERATOR_REPORT.md`.

### 3.4 Wrapper auth contract (v1.48.1)

`.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh` all enable
shell tool use without an interactive permission prompt:

- claude: `--permission-mode acceptEdits --allowedTools "Bash"`
- codex: `--dangerously-bypass-approvals-and-sandbox -c approval_policy=never`
- gemini: `--approval-mode yolo`

Filesystem boundaries are enforced by the packet's `write_scope`, not by
the CLI's permission system. **If you regenerate or reinstall wrappers,
verify these flags survive** — `striatum skills install --profile all` is
known to sometimes regenerate the wrappers.

### 3.5 Anti-patterns to recognize early

- **claude-no-publish** (was 10+ instances; mitigated by v1.48.1): claude
  wrapper alive, no `claude --print` subprocess produces work, no on-disk
  artifact. Check `$STRIATUM_SCRATCH_DIR/claude-logs/packet-NNNN.log`
  for the agent's last words — usually a permission prompt.
- **gemini-no-frontmatter** (3+ instances): gemini writes a valid review
  but the frontmatter lacks `verdict_intent` / `severity`. Operator must
  edit the file before publish-on-behalf succeeds. Don't fabricate a
  verdict the agent didn't intend — re-read the conclusion text.
- **codex/codex co-blindness** (5+ instances, D095-D098, D100):
  implementer and a reviewer are both codex; reviewer findings cluster
  around the implementer's blind spots, producing `needs_revision`
  verdicts that 2-of-3 cross-lane majority overrides. `workflow validate`
  now refuses same-model review-pair and revision-cycle lint findings by
  default; `--allow-same-model-pairing` is the explicit operator override.
- **packet-design gap** (observed dogfood-055b/056): fix-up review packets
  inherit the parent's `context.docs` and don't include the fix-up's
  HANDOFF + source diff. The reviewer correctly refuses on missing
  evidence. Either include the fix-up artifacts in the next workflow's
  `context_docs` or expect to override the codex verdict.

### 3.6 Cycle-exhaustion override

When a `needs_revision` verdict has no matching workflow cycle (workflow
declares 0 retries or the retry was already consumed), the runner opens
`blocker_kind: revision_routing` and `human_checkpoint`. Operator decides:

- **Real findings** → spawn a fix-up dogfood (§3.3).
- **Anti-pattern overrides** → record a D-decision (D095, D096, D097,
  D098, D099, D100, D101, D102 are precedents) and override via §3.2.
  Always document the anti-pattern variant in the decision record so
  future operators can recognize.

---

## 4. Active runway (this week, next 1-3 dogfoods)

### 4.1 ✅ shipped — Dogfood-057 / v1.49.0: RFC 0048 V1 Phase A handler port

**Reservation history.** §4.1 originally reserved dogfood-057 for the
v1.48.x security hardening (CSRF / origin-enforcement / context
validation) closing GH #9/#10/#11. That work shipped earlier the same
day via the `striatum/gh-issues-parallel` branch (commit `f5c8cca`
"Implement GH 9 10 11 security hardening"), making the original 057
reservation moot. **dogfood-057 was reassigned to RFC 0048 V1 Phase A**
(the substrate-facade fix) since it was the next biggest blocker on the
runway.

**Closes:** RFC 0048 V1 Phase A — the Python-side handler port. All 16
single-repo daemon RPC mutation handlers moved from SQLite-backed CLI
dispatch into native PG-backed handlers under
`src/striatum/daemon_pg/handlers/{workflow_loop,recovery_evidence}/`
with `DaemonRpcRouter._route` resolving the PG handler before the
legacy `CLI_ROUTES` fallback.

**V1 landing notes** (from the run's `OPERATOR_REPORT.md`):

- Codex F1-F4 (fail-closed routing, capability-denial tests,
  audit-chain concurrency, append-only role enforcement) accepted as
  V1.5 follow-up risk.
- Claude HIGH#1/#2 (byte-equivalence parity rig advertised but unused;
  `complete_inline`/`ack_inline` undefined and `recovery.resume
  --complete`/`recovery.auto` live-mode unreachable) accepted as V1.5
  follow-up risk.
- Schema migration for top-level `striatumd.events.previous_hash` /
  `row_hash` deferred — chain metadata currently lives inside
  `payload_json._event_chain` per implementer workaround.
- Run executed in legacy SQLite mode (`STRIATUM_DAEMON_REQUIRED=0
  STRIATUM_TEST_HARNESS=1`) because RFC 0048 is itself the gap that
  prevents the daemon-required CLI from working end-to-end. State-store
  corruption surfaced; SQLite was quarantined and reset.

**Follow-up:** completed through RFC 0048 V1.5 / v1.55.0. D105 briefly made
Python the primary production daemon core, but D107 / RFC 0068 supersedes that
constraint and restores the Go production daemon port as the target.

### 4.2 🟡 landed bounded slice — RFC 0051 V1 auto-finalize from front matter

**Updates:** [TODO item 56](TODO.md).

The bounded daemon slice has landed: `recovery.auto_finalize` supports dry-run
and workflow-opt-in live mode, records explicit `artifact.auto_finalized` and
`job.auto_finalized` events, projects eligibility/refusal state through status,
dashboard, and web surfaces, and leaves malformed/missing/byline-mismatched
artifacts on the existing operator recovery path.

**Current boundary:** global/default auto-finalize is intentionally not
enabled. That policy waits on live dogfood confidence plus an explicit product
decision about when Striatum may complete work from durable artifacts without
per-workflow opt-in.

### 4.3 ✅ completed — TODO #30 / RFC 0039 V1.6 Go support-runtime hardening

**Closes:** [TODO item 30](TODO.md#L527).

**Status:** complete for the historical helper-focused hardening slice. D107
later reopened full Go daemon parity under RFC 0068, so this item is now
supporting groundwork rather than the daemon-core end state:

- (F1) `go/Makefile verify` now runs `go mod verify` and
  `go mod tidy -diff`.
- (F2) A startup regression asserts `striatumd` refuses to serve without a
  Postgres URL/config and does not bind its Unix socket.
- (F3) CI currently runs `make daemon-go-helper-check`; RFC 0068 will add a
  production Go daemon conformance gate before default flip.
- (F4/F5) Helper boundary coverage now inspects transitive dependencies with
  `go list -deps ./cmd/striatum-supervisor-helper`; transitional Go RPC
  smoke/audit tests remain available but are not a parity gate.

**Suggested implementer:** claude (Go + Python harness). Deliberately
avoid codex (D101 precedent).

### 4.4 ✅ completed — Architecture remediation Phase 0: authority matrix and guardrails

**Closes:** [TODO item 48](TODO.md).

**Why now:** the 2026-05-16 architecture review found the main product
risk is not a missing feature; it is authority ambiguity across daemon
RPC, native Python PG handlers, Go handlers, contract route translations,
and legacy SQLite. The next work had to make that ambiguity measurable
before deleting fallbacks.

**Landed in this slice:**
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` inventories the current
  parser, route translator, daemon registry, Python PG handlers, and Go
  handler registrations.
- Guardrail tests fail when a daemon registry method lacks an explicit
  authority classification or when a handwritten fallback route appears
  without being named as transition debt.
- A SQLite-connect tripwire test covers representative production-mode
  commands under daemon-required enforcement.
- `recovery auto-publish` no longer emits the unregistered
  `recovery.auto_publish` method.

**Next after this ships:** TODO item 49/50 are now active. Keep the
authority matrix and contract tests current while deleting fallback paths.

---

### 4.5 🟡 substantially completed — Architecture remediation Phase 1: production fallback closure

**Updates:** [TODO item 49](TODO.md).

**Landed in this slice:**
- Native Python PG handlers now cover `run.graph`, `worktree.*`,
  `supervise.*`, and the `recovery watch` CLI-local scheduler now delegates
  to daemon `recovery.sweep` without a `recovery.watch` RPC, in addition to the
  earlier read, workflow-loop, recovery, run lifecycle, branch, checkpoint,
  and decision handlers.
- `src/striatum/daemon_rpc/server.py` no longer imports or calls
  `striatum.api.invoke`; handwritten server fallback routes are gone and
  guarded by tests.
- Contract-backed CLI translations now route `run graph`,
  `worktree create/release/list`,
  and `supervise start/send/stop/status/list` through daemon RPC.
- Mapped CLI commands now fail closed when the route layer raises an
  unexpected exception, with an architecture guardrail proving the path does
  not open repo-local SQLite.
- `repo.add`, `repo.list`, and `repo.remove` now route through daemon RPC
  and update `striatumd.repositories` directly; `repo add --init` creates
  only operational scratch and never creates `.striatum/state.sqlite3`.
- Production `striatum init` and `striatum adopt` now share the same
  scratch-only bootstrap, with repo-local SQLite init retained only for
  legacy test fixtures.
- Workflow authoring methods are explicitly CLI-local: daemon RPC refuses
  them with `not_implemented`, MCP tool listing hides them, and route tests
  prevent accidental daemon routing.
- `workflow upgrade` checks daemon PG for non-terminal runs and fails closed
  after repo-local SQLite cutover if PG cannot be reached.

**Remaining Phase 1 debt:** legacy SQLite domain code is still used by the
local service, adapter/byline/inbox helpers, dogfood compatibility tools, and
migration/test fixtures. Quarantine that code under a migration/service
namespace while executing Phase 4; it is no longer a daemon production
fallback path or repo-administration path.

---

### 4.6 ✅ completed — Architecture remediation Phase 2: daemon method contract source

**Updates:** [TODO item 50](TODO.md).

**Landed in this slice:**
- `contracts/daemon_methods.json` is the source for all 104 registered
  daemon RPC methods, including deprecated aliases.
- Python `src/striatum/daemon_rpc/registry.py` builds `METHOD_REGISTRY`,
  `METHODS_ETAG`, and `daemon.describe` output from the contract while
  preserving the public `MethodEntry` shape.
- Go `go/pkg/rpc/registry_methods.go` is generated from the same contract
  via `scripts/generate_go_rpc_registry.py`; `go generate ./pkg/rpc` is
  reproducible and Go contract parity tests catch drift.
- CLI/MCP contract tests now assert routed CLI methods are registered,
  workflow authoring stays CLI-local, daemon fallback routes are unused,
  and daemon MCP tools hide CLI-local and deprecated methods.
- Daemon MCP tool descriptors are now generated from `METHOD_REGISTRY`, so
  method name, required capability, and repository-scope mode are no longer
  hand-written in `mcp.py`.
- `scripts/generate_daemon_method_tables.py` renders
  `docs/architecture/DAEMON_METHOD_TABLES.md` from the daemon method
  contract, with `--check` coverage to catch checked-in documentation drift.
- The runtime CLI command-to-RPC route map is now declared in the contract's
  `cli_routes` section and loaded by `src/striatum/cli/daemon_rpc_route.py`;
  that module retains only CLI-local parameter extraction. Focused tests keep
  workflow authoring CLI-local and catch route/contract drift.
- `cross_repo.cancel` now delegates to the PG-native participant-cancel
  runner through the daemon RPC route map; remaining cross-repo work is
  lifecycle hardening and E2E coverage, not an explicit placeholder.

---

### 4.7 ✅ completed / superseded — Architecture remediation Phase 3: daemon core strategy

**Closes:** [TODO item 51](TODO.md).

**Decision:** D105 named Python as the primary production daemon core, but
D107 supersedes it. The active target is now RFC 0068: Go production daemon
port, Python daemon retirement after parity, Python CLI/web clients where
useful, and SQLite removal from production and compatibility paths.

**Landed in this slice:**
- `docs/DECISION_LOG.md` records D107 and marks D105 superseded.
- TODO item 25 is reopened under RFC 0068.
- TODO item 30 remains completed helper groundwork.
- TODO item 61 owns the Go daemon port and Python-daemon retirement.

**Next after this ships:** Phase 4 can make the local web service
daemon-first without needing to support two domain daemons.

---

### 4.8 🟡 partially completed — Architecture remediation Phase 4: daemon-first web service

**Updates:** [TODO item 52](TODO.md).

**Landed in this slice:**
- Added `src/striatum/service_daemon.py` as a narrow local-service daemon RPC
  helper.
- Web POST handlers for run cancel/pause/resume, job cancel/retry, and branch
  confirm now call daemon RPC instead of opening repo-local SQLite.
- Focused service tests tripwire `striatum.db.connect` for those POST paths.
- The web run-list page now calls daemon `list.runs` in production and renders
  the workflow identity/source DTO returned by the daemon handler. The legacy
  SQLite path is gated behind `STRIATUM_TEST_HARNESS=1
  STRIATUM_DAEMON_REQUIRED=0` for subprocess web fixtures only.
- Chat-session briefing now calls daemon `list.runs` for its active-run
  summary and has a SQLite tripwire regression for the daemon DTO path.
- The posture-verdict drill-down page now calls daemon
  `run.posture_verdicts` in production and retains the legacy SQLite path
  only for the test-harness escape.
- The `/v1` JSON read endpoints for status, doctor, why, dashboard, and
  run artifact rollups now call daemon read DTOs directly instead of routing
  through the legacy CLI invoke wrapper. Test-harness fallbacks preserve the
  old subprocess fixture path only.
- The `/doctor` HTML page now calls daemon `doctor` in production, with
  grouped problem records and per-record recovery recipes still shaped for
  the template. Direct SQLite remains only in the test-harness fallback.
- The artifact detail page now calls daemon `artifact.show` with optional
  web context for run scoping, expected author line, and operator-on-behalf
  provenance. The existing raw-artifact endpoint remains backward-compatible
  with the default `artifact.show` metadata response.
- `/v1/invoke` now derives daemon-routed read classification from
  `METHOD_REGISTRY.required_capability`; only CLI-local workflow authoring
  reads remain in an explicit service-side allowlist.
- Production service startup now verifies daemon/repository health through
  daemon `doctor` before binding. The old SQLite integrity check remains
  only for subprocess fixtures running under the legacy test-harness escape.
- The web SSE stream now uses daemon `run.events` in production and retains
  direct SQLite event tailing only for subprocess fixtures under the same
  test-harness escape.
- The workflow run-now POST path now calls daemon `run.prepare`,
  `branch.confirm`, and `run.start` in production, while preserving its
  historical field-level workflow validation response through daemon RPC
  error details. The direct SQLite lifecycle remains only in the subprocess
  test-harness fallback.
- The run detail page now calls daemon `run.detail` in production for run,
  job, session, recovery-panel, verdict, and phase-progress state. The web
  service still owns HTML/SVG rendering, and the legacy SQLite page read is
  limited to the subprocess test-harness fallback.
- The job detail page now calls daemon `job.detail` in production for job,
  expected-artifact, artifact, process-evidence, and verdict state. Override
  context-token minting remains local to the web service; the direct SQLite
  page read is limited to the subprocess test-harness fallback.
- `src/striatum/service_http.py` now owns the pure HTTP/security helpers
  for token comparison, JSON content-type validation, origin parsing, bind
  origin derivation, argv flag lookup, and web-context HMAC tokens. The
  names remain re-exported through `service.py` for existing callers and
  tests.
- `src/striatum/web/chat_session.py` now owns chat transcript projection,
  chat-briefing construction, JSONL append, timestamp, stable-hash,
  safe-git, multipart parsing, session path/listing, display-message, and
  workflow-write confirmation helpers. `service.py` keeps HTTP routing,
  provider/tool orchestration, and response handling.
- `src/striatum/legacy_sqlite/service.py` now owns the gated subprocess-fixture
  mutation fallbacks and legacy error mappers, narrowing `service.py` toward
  request handling plus rendering.
- `src/striatum/legacy_sqlite/service.py` also owns the remaining legacy page-read
  payload builders, view-file breadcrumb lookup, doctor-page fixture payload,
  SSE event tail, and legacy startup integrity check. `service.py` no longer
  imports or opens repo-local SQLite directly, and its compatibility aliases
  load the quarantined module lazily only when a legacy fallback is invoked.
- `src/striatum/web/static_assets.py` now owns bundled static asset lookup,
  path validation, and content-type mapping. `service.py` keeps HTTP response
  writing and CSP/header behavior for the `/static/*` route.
- `src/striatum/web/workflows.py` now owns workflow editor file resolution,
  new-workflow scaffold payloads, validation, atomic writes, and If-Match
  handling. `service.py` keeps HTTP request parsing, template rendering, and
  JSON response mapping for the workflow editor routes.
- `src/striatum/web/run_list.py` now owns run-list presentation helpers for
  GitHub remote parsing, workflow source-path normalization, workflow tree-link
  construction, and run state-chip shaping.
- `src/striatum/web/artifacts.py` now owns safe artifact file resolution, raw
  download content-type selection, and inline Markdown rendering helpers for
  artifact views.
- `src/striatum/service_command_policy.py` now owns `/v1/invoke`
  read/mutation command classification; `service.py` keeps the
  `is_read_command` compatibility import and route-level request handling.
- `src/striatum/web/view_file.py` now owns repository file-view path
  validation, binary detection, text/Markdown payload shaping, and inline
  Markdown rendering. `service.py` keeps route-level rendering and legacy
  run-breadcrumb fallback injection.
- `src/striatum/service_sse.py` now owns SSE replay offset parsing, event
  framing, and daemon-backed run-event stream-loop control. `service.py`
  keeps per-run slot accounting and legacy fixture fallback selection.
- `src/striatum/service_state.py` now owns process-local service state,
  GitHub remote/default-branch caching, shutdown signaling, web-context secret
  generation, and per-run SSE slot accounting.
- `src/striatum/service_runtime.py` now owns local service runtime helpers for
  version/mode reporting, loopback bind validation, PID-file single-instance
  checks, startup exceptions, and idle shutdown waiting.
- `src/striatum/web/template_env.py` now owns HTML escaping and Jinja
  environment construction for server-rendered web templates.
- `src/striatum/service_request_security.py` now owns request authentication,
  bearer-token checks, same-origin mutation policy, and override-verdict
  web-context validation decisions.
- `src/striatum/web/workflow_generation.py` now owns workflow template
  listing/show and workflow generation preview/write response shaping.
- `src/striatum/service_request_io.py` now owns request-body parsing and
  JSON/HTML response helpers. `service.py` keeps stable route-level wrapper
  methods for existing call sites and tests.
- `src/striatum/web/doctor.py` now owns doctor page DTO loading, gated legacy
  fallback selection, record recipe shaping, and problem grouping. `service.py`
  keeps template rendering and response mapping for `/doctor`.
- `src/striatum/web/workflows.py` now owns workflow browser index/detail page
  DTO shaping, including small index entries and detail graph-SVG selection.
  `service.py` keeps template rendering and HTTP error mapping for those pages.
- `src/striatum/web/job_detail.py` now owns job-detail template context
  shaping and override-context-token minting. `service.py` keeps daemon
  RPC/fallback and HTTP response mapping for the route.
- `src/striatum/web/artifacts.py` now also owns artifact-view template
  context shaping, byline display, recorded attestation chips, lane-evidence
  chips, and expected-artifact row shaping. The daemon-backed artifact page
  no longer reaches into `legacy_sqlite.service` for pure presentation
  shaping.
- `src/striatum/web/run_posture_verdicts.py` now owns posture-verdict
  template-context shaping and verdict-row filtering. `service.py` keeps the
  daemon RPC/fallback and HTTP error mapping for the route.
- `src/striatum/web/chat_routes.py` now owns chat page rendering, chat
  creation, provider send/tool-loop handling, workflow-write confirmation,
  stop redirects, and transcript SSE tailing. `service.py` keeps route
  dispatch and stable briefing/git-helper compatibility aliases.
- `src/striatum/web/run_pages.py` now owns run list/detail, job detail,
  artifact view, and posture-verdict page rendering, including daemon DTO
  loading, gated legacy fallback selection, graph rendering, and template
  context assembly. `service.py` keeps route dispatch and stable private
  handler wrappers for existing tests/callers.
- `src/striatum/web/artifacts.py` now also owns artifact raw download
  orchestration, including daemon metadata lookup, gated legacy fallback
  selection, file loading, content-type selection, and response header/body
  framing through callbacks supplied by the service wrapper.
- `src/striatum/web/run_actions.py` now owns workflow run-now,
  branch-confirm, run cancel/pause/resume, and job cancel/retry route
  handling, including mutation gates, request-body validation, daemon RPC
  dispatch, dirty-tree/schema error mapping, and legacy fixture fallback
  delegation. `service.py` keeps route dispatch and stable private wrappers.
- `src/striatum/web/workflows.py` now also owns workflow browser and
  visual-editor route rendering/saving, including index/new/detail/edit
  template rendering, edit POST body parsing, validation-error projection,
  and stale-write metadata. `service.py` keeps route dispatch and stable
  private wrappers.
- `src/striatum/web/view_file.py` now also owns repository file-view route
  rendering, including tree/file template selection, error mapping, and
  breadcrumb injection through a legacy callback supplied by `service.py`.
- `src/striatum/service_api_routes.py` now owns JSON read helpers, repo-tree
  reads, daemon-read fallback handling, and run-event SSE route control while
  `service.py` keeps dispatch, authentication, and stable private wrappers.
- `src/striatum/service_routes.py` now owns GET/POST route selection while
  `service.py` keeps stable handler wrapper methods and endpoint contexts.
- `src/striatum/service_server.py` now owns TCP/Unix binding, PID-file
  handling, signal shutdown, and serve-loop orchestration while `service.py`
  keeps private compatibility wrappers.

**Remaining Phase 4 debt:** continue splitting `service.py` along stable
non-SQLite request-handling and rendering boundaries after the daemon-routed
paths are stable.

---

### 4.9 🟡 partially completed — Architecture remediation Phase 5: real escalation inbox

**Updates:** [TODO item 53](TODO.md).

**Landed in this slice:**
- `escalation.list`, `escalation.show`, and `escalation.resolve` project
  human-principal escalations over existing blocker rows.
- The daemon method contract and generated Go registry include the escalation
  methods.
- CLI routing supports `striatum escalation list/show/resolve`.
- `striatum inbox --json` now runs as documented for the principal inbox,
  while `inbox --session-id` remains the session-packet helper.
- The `escalation` artifact kind and `striatum.escalation.v1` front matter
  schema landed, with workflow validation and publish-artifact coverage.
- Publishing an `escalation` artifact can now link to an existing
  escalation-class blocker via front matter; the linked artifact metadata is
  stored under `blockers.payload_json.escalation_artifact` and projected by
  `escalation.list` / `escalation.show`.
- Escalation projections verify the linked artifact still exists and matches
  id/path/hash metadata before surfacing it; idempotent escalation artifact
  publishes repair missing blocker links and reject conflicting existing links.

**Remaining Phase 5 debt:** decide artifact-only escalation creation policy,
consider a dedicated escalation table or stricter blocker payload schema, and
decide whether to rename the packet helper to `packet inbox`.

---

### 4.10 ✅ completed — Architecture remediation Phase 6: supervisor control channel

**Updates:** [TODO item 54](TODO.md).

**Landed in this slice:**
- `supervise.send` returns an explicit delivered-unacknowledged state.
- `supervise.report` records wrapper control events for packet acceptance,
  agent start, artifact observation, progress, and agent exit without reading
  or parsing model stdout.
- Supervision tests cover event recording and stopped-state transition on
  reported agent exit.
- A standalone Go `striatum-supervisor-helper` binary now launches agents
  under PTY, forwards packet bytes from stdin or a FIFO, and emits JSONL
  control events (`agent_started`, `packet_accepted`, `progress`,
  `agent_exited`, `helper_error`) without importing daemon DB/RPC,
  mutation, read, apply, or cross-repo authority packages.
- `supervise.report` now consumes helper event batches from JSONL text, a
  path, or object lists; it records helper events through the existing durable
  event path, preserves helper timestamps as `reported_at`, records
  `helper_error`, and uses the existing `agent_exited` stopped-state
  transition.
- `supervise.reattach_status` now has a real daemon PG handler. It returns
  a read-only supervisor health DTO classifying supervisors as
  `reattachable`, `lost_candidate`, `needs_repair`, `needs_verification`, or
  `terminal`, including pointer/daemon-row context, PID liveness, PID
  start-time identity, and recommended operator action. Daemon `doctor`
  now surfaces non-healthy reattach states for stale supervisors without
  changing supervisor state.
- Lanes can opt in to `supervision.transport: "pty_helper"`. The Python
  daemon launches `striatum-supervisor-helper`, persists helper pointer
  metadata, and drains helper JSONL events through `supervise.report` during
  start/send/stop/status.
- Pipe-transport lanes can opt in to
  `supervision.stdin_delivery: "one_shot_eof"` for single-prompt commands
  that read stdin until EOF. Default supervised lanes keep the persistent
  FIFO contract.
- Runner-owned supervisor stall detection now marks stale attached lanes as
  `liveness: "stalled"` in `supervise.status`, adds status/doctor surfacing,
  and opens `heartbeat_stall_lease_expired` blockers when an attached
  supervisor's active lease expires without progress. The recovery path does
  not auto-kill the OS process.
- PostgreSQL lane-liveness attestation now matches the stricter legacy
  semantics: an attached supervisor row attests only when its session/run,
  live PID, PID start-time token, and command match the immutable workflow
  snapshot lane command.
- The Postgres supervision handler suite now has a focused real-helper
  integration test that launches `go/bin/striatum-supervisor-helper`, sends
  a work packet through the PTY-helper transport, drains packet-acknowledged
  and agent-exited JSONL events, and verifies the Python daemon state/event
  projection.
- `make daemon-go-helper-integration` now builds the Go helper and runs that
  focused Postgres-backed integration test, and CI runs the target on
  Linux runners with the Postgres service.
- Existing supervisor paths now perform restart reconciliation before
  delivery: `supervise.status`, `supervise.send`, and claim-next
  auto-delivery record `supervisor.reattached` for surviving PID identity,
  update daemon-instance metadata, fail closed for repair/verification
  states, and mark stale PID identity `lost` before writing to stdin.
- `tests/test_claude_supervised_wrapper.py` now runs the supervised-wrapper
  loop fixture across `.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh`,
  proving multi-packet delivery, inner-command failure isolation, clean EOF
  exit, temp scratch logging, and the non-interactive approval flags required
  by v1.48.1.

**Remaining Phase 6 debt:** none.

---

### 4.11 🟡 partially completed — Architecture remediation Phase 7: workflow risk lint

**Updates:** [TODO item 55](TODO.md).

**Landed in this slice:**
- Added `striatum workflow lint <workflow.json> --json`.
- Lint findings are structured separately from validation errors.
- The first rules cover same-model review pairs/revision cycles,
  non-fresh review context, broad repo-write scope, repo-write without
  per-job worktree isolation, and review workflows with no revision or
  human-checkpoint escalation path.
- The local service read-command whitelist includes `workflow lint`.
- Opt-in strict mode landed: `workflow lint --strict` refuses warnings unless
  the operator supplies a non-empty `--override-rationale`, and JSON/API
  refusals include the lint payload under `error.details`.
- The workflow browser/detail pages surface lint warning counts and short
  warning lists without changing validation status.
- Lint now includes advisory coverage scoring for reviewer independence,
  fresh context, write isolation, revision/escalation path, and review
  posture diversity.
- `workflow validate` refuses same-model review-pair/revision-cycle lint
  findings by default unless `--allow-same-model-pairing` is supplied.
- Workflow generator preview envelopes and the workflow chooser surface the
  lint summary separately from generator warnings.
- Strict overrides can record an operator-supplied
  `--accepted-risk-decision-id` reference.

**Remaining Phase 7 debt:** blocked on product decision. Accepted lint-risk
persistence needs an explicit durable authority choice before implementation
continues; current `workflow lint` remains CLI-local/non-mutating and durable
evidence is the operator-recorded decision referenced by
`--accepted-risk-decision-id`.

---

### 4.12 🟡 partially completed — Architecture remediation Phase 8: auto-finalize from front matter

**Updates:** [TODO item 39](TODO.md), [TODO item 56](TODO.md).

**Landed in this slice:**
- Added `recovery.auto_finalize` as a daemon/Postgres recovery method with
  dry-run and workflow-opt-in live modes.
- The checker validates declared required expected artifacts, stable mtime,
  front matter, exact byline, active lease/session ownership, and lane
  evidence before mutating state.
- Live review auto-finalize publishes the finding, derives the verdict from
  `verdict_intent`, records the verdict, and completes the job atomically.
- Events `artifact.auto_finalized` and `job.auto_finalized` mark
  auto-from-artifact reconciliation, and PG evidence artifact summaries expose
  `publish_origin=auto_from_artifact`.
- CLI routing and the shared daemon method contract include the split
  `recovery.auto_finalize` method instead of overloading `recovery.auto`.
- Status/dashboard projections now include an `auto_finalize_dry_run` preview
  with eligible candidates and refusal reasons, and the web recovery panel can
  render the same preview without enabling live auto-finalize globally.
- The recovery method surface is split: `recovery.sweep` is the canonical
  daemon RPC for `striatum recovery auto`, `recovery auto-publish` emits
  `recovery.auto_publish_stale_artifacts`, and deprecated `recovery.auto`
  remains only as a compatibility alias for stale-artifact auto-publish.
- The sweep invokes live auto-finalize before lazy lease expiry only when the
  workflow opted in and never supplies the standalone force override.
- PostgreSQL sweep executes configured checkpoint-timeout escalation hooks in
  live mode, reports hook eligibility without side effects in dry-runs, and
  folds hook failures into `escalations[]`.
- Recovery sweep acceptance coverage now pins a dogfood-shaped run where
  three valid written review findings auto-finalize without
  operator-on-behalf or override provenance.

**Remaining Phase 8 debt:** blocked on live dogfood confidence plus a product
decision. Live auto-finalize remains workflow opt-in, and dry-run visibility
remains the default posture until evidence supports a default-on change.

---

### 4.13 ✅ completed — Architecture remediation Phase 9: UI packaging and bundle cleanup

**Updates:** [TODO item 57](TODO.md).

**Landed in this slice:**
- `make ui-build` now clears `src/striatum/web/static/build/` before Vite
  emits assets, so stale hashed chunks cannot accumulate across builds.
- `make ui-check-bundle` runs the existing bundle drift check plus a
  deterministic bundle-size gate.
- `@vitejs/plugin-react` moved from runtime dependencies to
  `devDependencies`, with the lockfile updated.
- Focused packaging tests pin the clean-build Makefile contract, build-only
  dependency placement, and bundle-size checker behavior.
- The package wheel now has a size gate aligned with the UI bundle gate.

**Remaining Phase 9 debt:** none currently actionable. Manual chunking is
monitor-only until bundle evidence shows the current Rollup output is a
problem; keep package-data/manifest loading aligned if Vite manifest output is
introduced later.

---

## 5. Near-term queue (after the active runway)

Order is **dependency-driven, not preference-driven**. Promote items up
when their blocker clears.

### 5.1 ✅ completed — RFC 0050 V2 ergonomics polish

**Closed:** [#12](https://github.com/halbritt/striatum/issues/12) (clipboard hijack), [#13](https://github.com/halbritt/striatum/issues/13) (ghost field).

The copy-on-click allowlist and workflow-editor purge are already covered by
targeted tests. The dogfood-056 ergonomic review items are not tracked as
active GitHub backlog unless they get promoted into explicit issues.

### 5.2 ✅ completed, superseded — D105 follow-up / Go supervisor protocol

**Historical note:** this slice was planned under D105. D107 / RFC 0068
later reopened TODO item 25's Go replacement-daemon phase.

Shipped scope from the D105 interval:
- Daemon RPC, authorization, audit, and domain transitions remain in Python.
- The Go support code handles the narrow PTY/process supervision protocol for start,
  send, stop/status, wrapper control events, reattach, and lost-state reporting.
- That interval did not deliver broad Go replacement-daemon parity.
- Focused CI and integration coverage validate the existing Python/Go boundary;
  RFC 0068 owns the broader conformance gate.

### 5.3 ✅ shipped — RFC 0048 daemon-side substrate migration (v1.49.0–v1.55.0)

All three phases landed:

- **Phase A** (v1.49.0): 16 single-repo mutation handlers ported into
  `src/striatum/daemon_pg/handlers/{workflow_loop,recovery_evidence}/`.
- **Phase B** (v1.50.0–v1.54.0 + follow-up): transition-era Go-core
  parity in `go/pkg/{reads,mutations}/`; daemon Unix-socket accept loop;
  12 read handlers byte-equivalent with the Python path. D105 temporarily
  narrowed future Go work; D107 / RFC 0068 later reopened full parity.
- **Phase C** (v1.51.0–v1.52.0): CLI dispatch routes ~30 mapped verbs
  through the daemon socket; mapped verbs fail closed instead of
  falling back to SQLite when the daemon is unreachable.
- **V1.5 hardening** (v1.55.0): F2 cap-denial test matrix
  (`tests/daemon_pg/test_capability_denial_matrix.py`), F3 audit-chain
  row-lock in `append_audit_row`, F4 append-only role-grant tests, HIGH#1
  parity rig (`tests/daemon_pg/handlers/_parity.py`), HIGH#2 inline
  helpers exported (`complete_inline`, `ack_inline`), schema migration
  0006 (events `previous_hash`/`row_hash` columns +
  `repo_event_chain_heads`).

### 5.4 TODO item 26 — Codex/codex pairing validator rule

5 documented instances (D095, D096, D097, D098, D100) of the implementer-
↔-reviewer co-blindness anti-pattern. Soft warning and strict lint refusal
with explicit override rationale have landed. The CLI `workflow validate`
path now refuses same-model review-pair/revision-cycle lint findings by
default, with `--allow-same-model-pairing` as the explicit operator override.

**Status:** complete for the validator-rule TODO. Durable accepted-risk
policy is tracked under TODO 55; do not add hidden daemon/generator refusals
without an accepted authority decision.

### 5.5 RFC 0049 (experimental) — Interactive claude lane via MCP — **SHELVED**

Decision D106 records the durable shelf decision. v1.48.1's wrapper auth fix
bought time; RFC 0049 is now a *capability* RFC, not a *blocker*. Reopen if
subscription-quota economics shift, Anthropic plan-credit terms change
materially, or an operator explicitly funds the PTY/MCP spike. (~100×
token-per-dollar improvement potential on Max 20x remains attractive but not
urgent.)

### 5.6 RFC 0047 — Decision-record propagation

Closes the GH #3 design surface (now-closed issue had no implementation
beyond an event row). Landed projection semantics: rejected decisions move
the run to `compromised` and supersede accepting verdicts; accepted
decisions can reopen a compromised run to `completed` while preserving the
supersession trail. The daemon/Postgres projection is carried by migration
0007.

### 5.7 Optional memory/corpus integration — Striatum Corpus Contract V2

**Driven by:** `~/git/engram/STRIATUM_MEMORY_ROADMAP.md` (Engram-side
roadmap dated 2026-05-14). Treat that roadmap as an external consumer
request, not a Striatum runtime dependency. Engram may augment Striatum
operators and workflow agents with retrieval-backed memory over exported
corpora, but Striatum must keep running with Engram absent and must not
pull from Engram unless an accepted policy explicitly opts a workflow or
operator surface into augmentation.

**RFC 0057 scaffold landed (2026-05-14).** See
[`docs/rfcs/0057-corpus-contract-v2.md`](rfcs/0057-corpus-contract-v2.md)
for the bounded V2 decision surface (contract version, multi-corpus
identity, redaction-tier metadata, incremental-export watermarks,
validation rules, V1→V2 backward compatibility, augmentation-boundary
regression coverage, optional context-injection policy). Filed through
the `docs/issues/17/` workflow; the scaffold is the Striatum side of
GH #17. Full V2 acceptance criteria are deferred until the design phase
of a future dogfood resolves the decisions.

**What already shipped on our side:**
- `striatum corpus export --since <ref> --out <dir>` (RFC 0044 V1,
  dogfood-046, v1.35.0) — nine JSONL files + `manifest.json`, redacted,
  with replay-stable hashes, under `tenant_id='striatum'` and
  `corpus_id='striatum'`.
- The augmentation-not-dependency boundary regression test in
  `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
  pinning that no `import engram` / no `from engram` / no `memory.*`
  capabilities exist in Striatum source.

**External consumer asks (Striatum-side):**

1. **Corpus Contract V2 RFC** — RFC 0057. Define bundle manifest shape,
   source kinds, required + optional metadata, stable item IDs, content
   hashes, instance and repository identity, privacy/redaction metadata,
   incremental-export watermarks, validation rules, and backward
   compatibility. This is the dependency for external consumers that
   ingest Striatum exports.

2. **Multi-corpus support in the exporter** — emit
   `corpus_id = striatum:<repo-or-instance-id>` rather than the V1
   single-corpus `striatum`. Lets one machine host multiple local
   application memories without mixing separate Striatum projects.

3. **Reciprocal augmentation-boundary record** — extend the V1
   regression test to cover any new Engram-integration entry points so
   the "Striatum runs without Engram" property survives the integration
   phases.

4. **Context-injection policy** (RFC-level decision, not implementation
   yet) — whether Striatum may request optional augmentation from an
   external memory service, per-packet memory budget defaults, and which
   workflows opt in. Candidate consumers include operator-startup
   summaries, workflow scaffolding, agent-packet prep, review-cycle prep,
   blocker/recovery investigation, and UI/CLI memory search, but none of
   these may become hard prerequisites.

**Open decisions to make before implementation** (from Engram's roadmap
§Open Decisions, applicable to our side):

- Striatum instance identity representation.
- `corpus_id` naming — human-readable, UUID-based, or both.
- Which log streams are mandatory vs. optional.
- How much git diff content to export by default.
- Redaction tier guarantees Striatum commits to before export.
- Incremental-export watermark storage location.
- How to record Engram availability without creating a runtime dependency.
- Default per-packet memory injection budget.

**Suggested implementer:** any lane. The next Striatum-side phase is a
design RFC + contract tests; no end-user surface changes yet. Subsequent
phases (multi-corpus exporter, then optional context-injection
integration) are separate dogfoods.

**Blocked on:** product decisions inside RFC 0057 for multi-corpus
identity, redaction tier, watermarks, and injection policy. This is not a
runtime blocker for Striatum's core daemon/remediation work.

**Forward link:** §11 lists the Engram-side roadmap for context;
Engram's full backlog is at `~/git/engram/STRIATUM_MEMORY_ROADMAP.md`.

### 5.8 Documentation + role-model runway (RFC 0052-0056)

Five RFCs scaffolded in one operator session on 2026-05-14. They
cluster around the AI-operator-as-default + human-principal-as-
escalation model and the doc surfaces that express it.

Landing order: RFC 0053 first (already shipped to main — RFC, D103,
and doc-side fixes in SPEC, GETTING_STARTED, HOW_TO_HUMAN, plus the
UBIQUITOUS_LANGUAGE softening in fb0175c). Then RFC 0054 / 0055 /
0056 in any order (all single-track doc work). RFC 0052 implementation
is unblocked by the completed RFC 0048 substrate flip, but remains lower
priority than the active remediation runway unless scheduled explicitly.

- **RFC 0052** (committee deliberation workflow) — TODO #43.
  Phase 0 scaffold + schema sketches landed. V1.9/V2.0 implementation.
  Composes with RFC 0053 (committee stalemate is one of the named
  escalation triggers). No longer blocked on RFC 0048; schedule as its
  own dogfood when committee workflow implementation becomes the next
  product priority.
- **RFC 0053** (human principal as escalation-only) — TODO #44.
  RFC body + D103 + doc-side prose realignment shipped on main.
  A follow-up wording sweep realigned reader-facing docs, CLI help,
  scaffold output, workflow-template text, and recovery skill templates
  around principal/operator language while preserving durable schema/state
  identifiers.
  Deferred Phase A landed under remediation Phase 5: `escalation`
  artifact-kind schema, publish-time blocker linkage, and daemon RPC
  projection methods.
  Deferred Phase B: workflow.json schema-field rename
  (`human_checkpoint` → `escalation_checkpoint`), `waiting_human`
  run-state rename.
- **RFC 0054** (day-zero usage guide) — TODO #45. Phase 0
  scaffold + **Phase A shipped in v1.55.0** (commit `a88f44d`):
  `docs/USING_STRIATUM.md` added as a new doc alongside
  `GETTING_STARTED.md` (resolved Open question 1 toward additive,
  not replacement). Tutorial-warm tone; under 200 lines.
- **RFC 0055** (marketing README + architecture graphics) — TODO
  #46. Phase 0 scaffold + **Phase A shipped in v1.55.0** (commit
  `a88f44d`): `README.md` rewritten with vision-first framing,
  value-bullets above-fold, Mermaid architecture diagram, and a
  demoted docs-link table at the bottom. SVG polish follow-up
  still optional.
- **RFC 0056** (consumer-repo directory-structure opinions) —
  TODO #47. Phase 0 scaffold + **Phase A shipped in v1.55.0**
  (commit `a88f44d`): `docs/CONSUMER_REPO_LAYOUT.md` added with
  ASCII tree, per-section rationale, mid-life adoption guidance,
  and dogfood-heavy-projects extension. Phase B (scaffold
  extension of `init --with-ddd-layout`) deferred.

**Suggested implementer:** any lane. Documentation phases are
single-track and additive — they touch docs and don't intersect
running workflow state. The Phase B work for RFC 0053 (schema /
prompt sweep) is its own dogfood; the workflow.json bump is a
breaking schema change and should land paired with a
`workflow upgrade` rule.

**Blocked on:** RFC 0053 Phase B is blocked on the workflow.json version
bump being scheduled. RFC 0052 implementation is unblocked but unscheduled.
The other doc phases are unblocked.

### 5.9 Architecture remediation sequence (TODO 49-64)

This sequence comes from `STRIATUM_ARCHITECTURE_REMEDIATION_PLAN_2026-05-16.md`.
Production daemon fallback is now closed for mapped Python paths, but D107
changes the active runway: port the production daemon to Go and eliminate
SQLite from production and compatibility paths.

Release order after Phase 0:

1. **TODO 49 / Phase 1:** production daemon fallback is closed; remaining
   legacy SQLite quarantine belongs with service/adapter cleanup.
2. **TODO 50 / Phase 2:** contract source plus Python/Go registry
   generation, generated MCP descriptors, generated docs tables, and
   declarative runtime CLI route translation landed.
3. **TODO 51 / Phase 3:** D105 decided Python-primary temporarily; D107 later
   superseded it.
4. **TODO 52 / Phase 4:** make the web service a daemon client rather
   than a parallel state-store peer.
5. **TODO 53 / Phase 5:** implement a real escalation inbox for the
   human principal.
6. **TODO 54 / Phase 6:** harden process supervision with PTY support,
   wrapper control acks, and reattach/lost-state handling.
7. **TODO 55 / Phase 7:** workflow risk lint, opt-in strict enforcement,
   web surfacing, generator preview surfacing, coverage scoring, and
   accepted-risk decision references landed; durable audit persistence
   policy is tracked in §4.11.
8. **TODO 56 / Phase 8:** auto-finalize daemon method, status/dashboard/web
   visibility, and bounded sweep integration landed; remaining default-policy
   and dogfood acceptance work is tracked in §4.12.
9. **TODO 57 / Phase 9:** clean-build, bundle-size, and wheel-size gates
   landed; chunking is monitor-only and tracked in §4.13.
10. **TODO 58 / Phase 10:** day-zero Postgres/daemon setup slice
    landed: role/grant repair, service helpers, guided adoption,
    first-run smoke, and a dev-only compose profile.
11. **TODO 59 / Phase 11:** replay/archive foundations landed, including
    offline event-chain, row-hash, and archived row-id verification for
    command requests, process supervisors, process supervisor pointers,
    verdicts, blockers, process executions, and job worktrees; Corpus
    Contract V2 fields wait on RFC 0057 decisions.
12. **TODO 60 / Phase 12:** optional Git/PR integration waits on a product
    decision for commit authority and hosted-provider boundaries.
13. **TODO 61 / RFC 0068:** port the production daemon to Go, add stale-binary
    and conformance gates, and retire the Python daemon after parity.
14. **TODO 62 / RFC 0069:** move daemon-global surfaces to PostgreSQL/Go.
15. **TODO 63 / RFC 0070:** complete daemon client/service boundaries and
    remove direct client DB access.
16. **TODO 64 / RFC 0071:** add post-cutover diagnostics once authority cleanup
    lands.

**Blocked on:** current blockers are Phase 7 accepted-risk persistence,
Phase 8 default auto-finalize policy, Phase 11 Corpus V2 decisions, Phase 12
Git/PR authority, and the normal dogfood substrate mismatch recorded in
dogfoods 064/065. The Go port itself is unblocked and should proceed without
waiting for human approval.

---

## 6. RFC follow-ups (cycle-exhaustion deltas)

These are codex `needs_revision` findings deferred via D095-D102 overrides.
Each is a list of file:line corrections that should land in a future
dogfood. Order them by impact, not by RFC number.

| TODO | RFC | Origin | Decision | Scope |
|---:|---|---|---|---|
| [27](TODO.md) | RFC 0045 V1.5 | dogfood-043 | D097 | ✅ Completed: cycle phase-jump, Python/editor phase-field mismatch, explicit synthesis-job metadata validation, frontend drag-drop phase bypass, and invalid/unknown phase display tolerance have landed. |
| [28](TODO.md) | RFC 0040 V1.6 | dogfood-044 | D098 | Composite publish-on-behalf failure observability landed; remaining packet-evidence debt is provenance/packet-design work. |
| [29](TODO.md) | RFC 0038 V1.6 | dogfood-045 | D099 | ✅ Completed: real-bundle commit + supply-chain polish. **First `reject critical` override.** |
| [30](TODO.md) | RFC 0039 V1.6 | dogfood-047 | D101 | ✅ Completed in 4.3 as helper groundwork; full Go daemon parity is reopened by D107 / RFC 0068. |
| [31](TODO.md) | RFC 0043 V1.5 | dogfood-048 | D102 | ✅ Completed / tracker stale: crash-recovery tombstone two-phase, daemon-required default flip, `daemon migrate-repo-local` subparser wiring, focused `make test-rfc0043`, and a foreground-daemon refusal smoke have landed. **Distinct from D095-D101 — both reviewers had real findings, not co-blindness.** |
| (NEW) | RFC 0050 follow-up | dogfood-056 | (no override) | 5 reviewer findings filed as GH #9-13; 1 ergonomic from claude review. Already in active runway as 4.1 + 5.1. |

---

## 7. Blocked / waiting

Item F1 is no longer listed here: `examples/three-lane-design-build-review/`
is the runner-owned historical bootstrap successor, and
`tests/test_example_workflows.py` guards the fixture shape and references.

| Item | Blocker | Unblock criterion |
|---|---|---|
| RFC 0049 spike | Shelved by D106; depends on external billing semantics and PTY/MCP stability | Explicit operator-funded spike + measurement. |
| RFC 0057 Corpus V2 | Product contract decisions for multi-corpus identity, redaction tier, watermarks, and injection policy | Accepted RFC 0057 design. |
| Phase 12 Git/PR integration | Product decision for commit authority and hosted-provider boundaries | Accepted RFC/decision before commit apply or hosted PR work. |
| Item 32 (Engram-side RFC 0044 Phase 1) | External repo (`~/git/engram/`) | Engram-side work; **not Striatum's TODO**. |
| Item 16 (generic language sweep) | Ongoing documentation hygiene | Active sweep on 2026-05-17; keep open as a standing review item. |

---

## 8. Resolved GitHub issue follow-ups

| # | Title | Closed by |
|---|---|---|
| [9](https://github.com/halbritt/striatum/issues/9) | CSRF on `/v1/invoke` — no Content-Type validation | RFC 0048 V1 Phase A / dogfood-057 security hardening. |
| [10](https://github.com/halbritt/striatum/issues/10) | Override modal trusts DOM `data-*` for job/session IDs | RFC 0048 V1 Phase A / dogfood-057 security hardening. |
| [11](https://github.com/halbritt/striatum/issues/11) | Recovery panel dry-run relies on CLI-side read-only guarantee | RFC 0048 V1 Phase A / dogfood-057 security hardening. |
| [12](https://github.com/halbritt/striatum/issues/12) | `copy-on-click` works on any `data-copy` — clipboard poisoning | RFC 0050 V2 ergonomics polish. |
| [13](https://github.com/halbritt/striatum/issues/13) | Workflow editor — `require_attested_lane` not purged on type change | RFC 0050 V2 ergonomics polish. |
| [14](https://github.com/halbritt/striatum/issues/14) | Recovery cannot clear terminal-run `process_exit_nonzero` blocker without lease | `docs/issues/14/` workflow with accepting review. |
| [15](https://github.com/halbritt/striatum/issues/15) | Clarify PostgreSQL transition guidance | `docs/issues/15/` workflow and transition-doc sweep. |
| [16](https://github.com/halbritt/striatum/issues/16) | Add complete operator initialization prompt | `b9add6f` via `docs/issues/16/` workflow. **First production use of the new GH-issue workflow type.** Verify verdict `accept` severity `info`. End-to-end 21 minutes wall-clock, zero operator-on-behalf publishes — empirically validated v1.48.1's wrapper auth fix. |
| [17](https://github.com/halbritt/striatum/issues/17) | Striatum doc consistency for Engram memory integration | `docs/issues/17/` workflow plus RFC 0057 Corpus Contract V2 scaffold; remaining V2 implementation is tracked under TODO 59. |
| [18](https://github.com/halbritt/striatum/issues/18) | Supervised lane stdin EOF hang for `cmd -` commands | Explicit `supervision.stdin_delivery: "one_shot_eof"` opt-in for pipe-transport lanes, with claim-next/send metadata and PG tests. |
| [20](https://github.com/halbritt/striatum/issues/20) | `supervise`: lane-stall timeouts and alarms should be in the runner | Runner-owned heartbeat/lease stall blockers, stalled liveness, and doctor/status surfacing. |

---

## 9. Cross-cutting operator concerns

### 9.1 CI health (v1.55.0)

CI's Multi-repo harness step now hard-fails on missing Postgres rather
than silently skipping. After TODO item 30, CI no longer carries a
`CORE=go` multi-repo parity axis; it runs the Python-core multi-repo
harness on ubuntu-latest and the helper-focused `make
daemon-go-helper-check` on every matrix leg. GitHub-hosted macOS
runners don't support `services:`, so the multi-repo step remains
Linux-only.

### 9.2 Test failures status (v1.55.0)

`make lint typecheck test` on `main`:

- `test_static_assets_no_external_urls` — **passes** (W3C namespace +
  reactflow.dev URIs are now whitelisted).
- `test_decision_log_rows_under_word_budget` — **passes** (D094 prose
  trimmed or budget raised; current rows fit).

The full pytest sweep on the local dev machine (with halbritt granted
CREATEDB + CREATEROLE on the local PG so ephemeral DB fixtures actually
run, and `striatum_daemon.schema_meta.substrate_version=6` applied) is
1254 passed / 7 skipped / 0 expected failures as of v1.55.0
post-burn-down (commits `f80b889` → `9fc02d6`).

### 9.3 Wrappers regenerate sometimes

`striatum skills install --profile all` (which every supervisor invocation
runs as its `lane.command` prefix) appears to occasionally regenerate the
wrapper scripts under `.striatum/bin/`. After v1.48.1, this is no longer
an active hazard for permission flags (they are committed to git and survive
regeneration), but verify after any future wrapper-template change that
`grep "claude --print" .striatum/bin/claude-supervised-wrapper.sh` shows
the `--permission-mode acceptEdits --allowedTools "Bash"` flags.

### 9.4 Memory items (operator-side)

Read these before driving a multi-step run:

- `~/.claude/projects/<encoded-striatum-repo>/memory/MEMORY.md`
  — operator lessons learned (dogfood-driven over free-form, autonomous
  run decisions, finalize-without-asking, OPERATOR_REPORT incrementality,
  claude-stall recovery, lane attestation gap, CI poll discipline).

---

## 10. How to kick off a new dogfood

For a fresh operator context. Assumes the target dogfood number is `<N>`
and the scope is one RFC phase or one self-contained fix.

```bash
# 0. Pre-flight
cd <striatum-repo>
git status                                 # main, clean
gh issue list --state open --label rfc-XXXX  # know what you're closing
cat docs/ROADMAP.md                        # this doc

# 1. Scaffold
mkdir -p docs/dogfood/<N>/{prompts,roles}
# Copy workflow.json from a recent similar dogfood (056 is the latest V1 + 3-way reviewer pattern)
cp docs/dogfood/056/workflow.json docs/dogfood/<N>/workflow.json
$EDITOR docs/dogfood/<N>/workflow.json     # update workflow_id, context_docs, objective, allowed_paths
# Write per-job prompts pointing at the concrete spec (RFC or REVIEW.md)
$EDITOR docs/dogfood/<N>/prompts/synth.md docs/dogfood/<N>/prompts/implement.md docs/dogfood/<N>/prompts/review_build.md
# Initial OPERATOR_REPORT.md scaffold
$EDITOR docs/dogfood/<N>/OPERATOR_REPORT.md

# 2. Validate + prepare + start
striatum workflow validate docs/dogfood/<N>/workflow.json --json
striatum run prepare --workflow docs/dogfood/<N>/workflow.json --json   # remember run_id
striatum run start --run-id <run_id> --json

# 3. Drive each job (per workflow job in dependency order)
striatum register-session --run-id <run_id> --role <R> --lane <L> --fresh --json
striatum supervise start --session-id <S> --json
striatum claim-next --session-id <S> --json    # may auto-fire under supervisor

# 4. Monitor
striatum why <run_id>     # tail events, see state, see blockers
striatum dashboard --run-id <run_id> --once     # compact frame

# 5. Per-job recovery if a lane stalls
#    Start with control-plane evidence:
striatum doctor --run-id <run_id> --verbose --json
striatum supervise status --session-id <S> --json
striatum why <job_or_blocker_id>
#    Wrapper logs are secondary evidence, not the stall detector.

# 6. Override needs_revision verdicts only after the fix-up ratifies (§3.2)

# 7. Ship
$EDITOR docs/dogfood/<N>/OPERATOR_REPORT.md           # final outcome + decisions
$EDITOR pyproject.toml CHANGELOG.md                   # bump minor or patch
git add -A docs/dogfood/<N>/ pyproject.toml CHANGELOG.md src/ tests/
git commit -m "vX.Y.Z: ..."
git tag vX.Y.Z
git push origin main
git push origin vX.Y.Z
git branch -d striatum/dogfood-<N>-...  || true       # if a branch was used
git push origin --delete striatum/dogfood-<N>-... 2>/dev/null || true

# 8. Update this doc
$EDITOR docs/ROADMAP.md                                # promote what's done, advance the queue
```

---

## 11. Where to look next

| If you want... | Read |
|---|---|
| Authoritative status of any item | `docs/TODO.md` |
| Architectural rationale for a decision | `docs/DECISION_LOG.md` (latest accepted rows) |
| RFC design + acceptance criteria | `docs/rfcs/<NNNN>-*.md` and `docs/rfcs/README.md` index |
| Per-dogfood outcomes + interventions | `docs/dogfood/<N>/OPERATOR_REPORT.md` |
| Operator-facing CLI verbs + skills | `docs/HOW_TO_AGENT.md`, `docs/SPEC.md` |
| Patterns that aren't in SPEC | §3 above, MEMORY.md |
| What's actively broken | §1, §9.1, §9.2 |
| What to do today | §4 (active runway) |
| Engram memory integration (external dependency) | `~/git/engram/STRIATUM_MEMORY_ROADMAP.md` and §5.7 above |

---

## 12. Promotion checklist (update this doc per release)

On every `vX.Y.0`:

- [ ] Move items from §4 to §2 if they shipped.
- [ ] Promote items from §5 to §4 if their blocker cleared.
- [ ] Recompute §7 (blocked) — what's still gated and on what.
- [ ] Add new GH issues to §8.
- [ ] Note any new anti-pattern instances in §3.5.
- [ ] Move §1 forward to the new commit/version/CI state.
