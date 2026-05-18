# Striatum Architecture Review — Claude Opus 4.7 — 2026-05-18
author: reviewer-claude-opus-4.7-001

Reviewer voice convention, used throughout:

- **stated** — what the project's docs/READMEs claim
- **actual** — what the code actually does
- **mine** — my opinion as a peer reviewer

The project name resolved from `pyproject.toml` line 6 (`name = "striatum-orchestrator"`) and used consistently below is **striatum** (Python module name; PyPI distribution is `striatum-orchestrator`).

## 0. Files reviewed

- `README.md`
- `pyproject.toml`
- `Makefile`
- `AGENTS.md` (project instructions; same content surfaced via `CLAUDE.md`)
- `CHANGELOG.md` (lines 1–100; head-only)
- `contracts/daemon_methods.json` (lines 1–60 of 979)
- `docs/INDEX.md`
- `docs/SPEC.md` (lines 1–400 of 1810)
- `docs/DECISION_LOG.md` (header + spot rows; full table-of-IDs scan via `grep`)
- `docs/UBIQUITOUS_LANGUAGE.md`
- `docs/DDD.md`
- `docs/PRD.md`
- `docs/TODO.md` (lines 1–150 of 1500)
- `docs/ROADMAP.md` (lines 1–150 of 1248)
- `docs/operator/BRIEF.md`
- `docs/dogfood/` (directory listing only: 66 numbered run directories present)
- `src/striatum/cli/__init__.py`
- `src/striatum/cli/dispatch.py` (lines 1–300 of 1935)
- `src/striatum/cli/daemon_required.py` (lines 1–100 of 217)
- `src/striatum/daemon_rpc/registry.py` (lines 1–80 of 242)
- `src/striatum/schema.py` (lines 1–80 of 304)
- `STRIATUM_ARCHITECTURE_REVIEW_CODEX_GPT_5_2026-05-18.md` (first 30 lines, to read the prior reviewer's voice convention only)
- Source/test/doc inventory metrics produced by `find`/`wc -l` over the working tree.

## 1. Executive summary

- **The vocabulary is genuinely load-bearing.** `docs/DDD.md` is not a retrofit — verdict, posture, lease, byline, attestation, capability are enforced at the daemon RPC boundary (`contracts/daemon_methods.json`). The model isn't just a glossary; it's a refusal surface. This is the project's best decision and the thing most worth protecting.
- **The artifact-to-code ratio is upside-down.** 184k lines of Markdown in `docs/`, 60k lines of Python in `src/`, 32k lines of Go in `go/`, 55k lines of tests, 72 RFCs, 117 numbered decisions, 66 dogfood run directories, 25 version tags inside a six-day window (`grep -c '## v' CHANGELOG.md` = 25; CHANGELOG is 4,730 lines). For a single-operator demo-stage tool, the docs apparatus is doing more weight than the code can justify.
- **Three substrates coexist mid-cutover.** Repo-local SQLite, daemon-owned PostgreSQL via Python handlers (`src/striatum/daemon_pg/`, ~19k LOC), and Go daemon (`go/`, 25k LOC + 7k test). Recent commits (`git log --oneline -30`) are an aggressive `Quarantine legacy SQLite *` pass. The current state is "almost-done with both migrations, neither finished." That is the highest-priority concern in §5.
- **The Go-vs-Python daemon decision is not actually finished.** Docs (`docs/operator/BRIEF.md`) say Go is production. `Makefile` keeps `daemon-go-conformance` as a separate target. Python `daemon_pg/handlers/` is still 19k LOC of supported handlers. Whether the Python daemon is a maintained alternate or a temporary mirror is *the* architecture question, and the answer isn't visible in the code yet.
- **There are 277 `island-shared-*.js` files** in `src/striatum/web/static/build/` totalling 9.9 MB (`ls src/striatum/web/static/build/ | grep -c '^island-shared-'`). This is stale Vite output that was committed because the manifest tracks hashes but the build target doesn't atomically replace siblings. This is small, fixable, and embarrassing if anyone notices.
- **`final_status.json` (256 KB) and `status.json` (176 KB) at the repo root** look like development scratch that was committed by accident. Same for the seven `STRIATUM_*_REVIEW_*.md` and `STRIATUM_*_REMEDIATION_PLAN*.md` files at the root.
- **The single-operator framing is undermined by the architecture.** Eight capability scopes (`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, `surgical_recovery` — `daemon_rpc/registry.py:17`), per-repo capability tokens, audit-chain row-locking, cross-repo workflows, MCP mutation gating. These are reasonable for a multi-tenant control plane; for one person on a laptop they're tax.
- **The CLI dispatch surface is concentrated in one 1,935-line module** (`src/striatum/cli/dispatch.py`) that knows about daemon-routing, SQLite fixture compat, recovery, init, adopt, plugins, skills, and worktrees. It's the load-bearing seam for the whole substrate migration. It should be three or four modules and it's one.
- **Tests are the right shape, but volume is now a liability.** 220 test files, 55k LOC. The matrix is real (single-repo, multi-repo, Go-vs-Python daemon, with-PG vs without). A single contributor cannot rerun this matrix often enough to catch regressions; CI is already noted as "backlogged" in `docs/ROADMAP.md:36-38`.
- **The DDD doc, the SPEC, and the PRD agree, and the code agrees with them where it matters.** That is rare and worth saying. The mismatches are about *cleanup velocity*, not about *what the system is*.

## 2. What the project is trying to be

### stated

`README.md:3` and `docs/SPEC.md:11-28` give a tight, consistent product boundary: a local-first workflow runner for terminal-based AI coding agents, daemon-owned PostgreSQL as the only authoritative state, target repos as durable provenance, no hosted services, no telemetry, no transcript capture, no vendor SDK imports.

`docs/PRD.md:23-50` enumerates 33 seed decisions (D001–D033) that established: hybrid coordinator (deterministic + selected AI), model portability through lanes-as-config, fresh sessions, bounded cycles, JSON workflow config only, durable artifacts, no broad transcripts.

`docs/DDD.md` is the most useful doc in the tree: it argues explicitly that the vocabulary is the model. Aggregate roots, value objects, an event log, and a single write surface (daemon RPC) are the four pillars.

`docs/UBIQUITOUS_LANGUAGE.md` defines 200+ terms in a single glossary. Every concept introduced by an RFC gets added here first.

`docs/operator/BRIEF.md` says the current operational concern is the Go daemon port (D107/RFC 0068) and legacy SQLite quarantine.

### actual

The boundary in code matches the stated boundary at the daemon RPC level (`contracts/daemon_methods.json` enumerates 100+ methods; `src/striatum/daemon_rpc/registry.py:17` defines the 8-capability vocabulary). The DDD framing is in fact load-bearing: a reviewer cannot return "looks good" because the CLI surface only accepts the verdict enum.

However, the seed thesis (`docs/PRD.md:93-100`) reads like a small, opinionated coordinator. The 2026-05-18 reality is a 100-method daemon RPC contract, a Go and Python daemon both maintained simultaneously, a Vite/React frontend with five island bundles, MCP capability tokens, and cross-repo runs. The product framing did not predict this surface area.

The DDD doc's "this isn't a justification for adding more abstractions" (`docs/DDD.md:181`) is good intent. The CHANGELOG shows 72 RFCs and 117 decisions extending the model. Each one is individually defensible. Together they are far more model than a single-operator runner needs.

### mine

The thesis is sound: terminal-agent coordination with structured review, audit-chain provenance, no hosted state. The product boundary in SPEC is honest, and the DDD doc is the right justification. But the cadence of additions has overshot the thesis. Twenty-five versions in six days is not a stable substrate; it is a project mid-iteration. That is fine for an alpha, but you cannot also claim "demo-stage maturity" *and* publish a 100-method capability-token RPC surface. Pick one.

Mutually incompatible goals I observed:

1. **"Local-first, single-operator"** vs **"capability-scoped multi-tenant daemon"**. The 8-capability scope vocabulary, cross-repo runs, and capability-token registry are multi-user concerns. If the user is one person on a laptop, a single-token "you are the operator" is the right answer. If this is being pre-built for a hosted future, the docs should say so and the PRD should be updated.
2. **"No model dependency in the runner"** vs **"shipped first-class plugin bundles for `claude_code`, `codex`, `gemini_cli`"**. The plugins are configuration, not imports — so the strict claim survives — but the runner's mental model is clearly biased toward those three CLIs. That's fine; just acknowledge it.
3. **"Read SPEC if docs disagree"** vs **a 1810-line SPEC**. If the source of truth is too long to read in one sitting, it stops being the source of truth in practice. The DDD doc is 199 lines and is the *actual* load-bearing doc; SPEC needs a brutal cull.

## 3. Current architecture

### Components

- **CLI** (`src/striatum/cli/`, 18 modules, 7,968 LOC). The user-facing surface. Dispatch lives in `dispatch.py` (1,935 lines) and the argument parser in `parser.py` (1,343 lines).
- **Daemon RPC contract** (`contracts/daemon_methods.json`, 979 lines, 100+ method routes). Versioned shape with capability and audit-class annotations.
- **Python daemon-PG handlers** (`src/striatum/daemon_pg/`, ~19,000 LOC across ~50 files). Workflow loop, reads, recovery/evidence, run lifecycle, supervision, registry, worktree.
- **Go daemon** (`go/`, 24,595 LOC source + 7,390 LOC test). Independent implementation of the same RPC surface and Postgres schema. Eight Postgres migrations under `go/pkg/db/sql/`, matching the eight in `src/striatum/daemon_pg/sql/`.
- **Web service** (`src/striatum/service*.py`, ~14 modules) + **frontend** (`src/striatum/web/`, Jinja2 server-render + Vite/React islands).
- **MCP** (`src/striatum/mcp.py`, 602 LOC) — both stdio wrapper and daemon MCP resource surface.
- **Workflow engine** (`src/striatum/workflow.py`, 2,568 LOC) — validator + planner + generator hooks.
- **Legacy SQLite quarantine** (`src/striatum/legacy_sqlite/`, ~7,000 LOC across 11 modules). Lazy-loaded behind compat wrappers; production paths route through daemon.
- **Skills/plugins** (`src/striatum/skills/`, `src/striatum/plugins/`) — agent-side skill bundle generation per RFC 0015.

### Runtime

- One daemon process per machine, owning a PostgreSQL connection pool and a Unix-domain socket at `~/.local/state/striatum/runtime/striatumd.sock` (Linux) or `~/Library/Caches/striatum/runtime/striatumd.sock` (macOS) (`src/striatum/cli/daemon_required.py:82-94`).
- CLI invocations open a Unix-socket RPC, send a capability-scoped envelope, get an audit-chained response.
- Refusals are exit-coded: 11 = daemon unreachable, 12 = repo not migrated, 8 = invalid transition, 9 = schema version skew (`src/striatum/errors.py`).
- Supervised lanes write to FIFOs under `.striatum/scratch/<supervisor_id>/stdin.pipe`; supervised wrappers (`.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh`) consume one packet per line and shell out to the provider CLI with non-interactive approval flags (`docs/ROADMAP.md:110-122`).

### State / storage

- **Authoritative**: daemon-owned PostgreSQL. 8 migrations (`src/striatum/daemon_pg/sql/0001_baseline.sql` through `0008_lane_evidence_publish_guard.sql`). Per-repo workflow tables under a `repository_id` scope; daemon-global tables for registry, audit, capability, scheduler.
- **Scratch**: `.striatum/` next to each target repo for supervisor FIFOs, pidfiles, transient stdout.
- **Provenance**: durable Markdown artifacts in the target repo, with front-matter validation per kind (`decision`, `finding`, `findings_ledger`, `synthesis`, `support_ledger`, `action_item_ledger`, `harness_improvement_proposal`, `escalation`, `operator_brief`).
- **Schema-version spaghetti** (mine): `src/striatum/schema.py:5` still declares `SCHEMA_VERSION = "1"` (legacy SQLite, schema-only). `src/striatum/repo_local_schema.py:5` declares `LATEST_REPO_LOCAL_SCHEMA_VERSION = 16`. The PG substrate is at v6 per SPEC. That's three named schema versions for what should be one substrate. The first two are corpses; they should be deleted, not "quarantined."

### Surfaces

- **CLI**: 100+ verbs/subverbs through `striatum` console script (`pyproject.toml:43-44`).
- **Daemon RPC**: Unix socket; envelope per RFC 0030; capability-token authorization.
- **Web**: `striatum serve` + `--web` flag; localhost-only; Jinja2 + React islands; SSE event stream.
- **MCP**: production daemon MCP resource API plus a legacy stdio wrapper for compat.
- **Local Python API**: `striatum.api.invoke` (`src/striatum/api.py`) — kept for authoring/test compat.

### Tests

- 220 test files, 54,860 LOC. `pyproject.toml:62-64` registers a single custom marker `multi_repo`. `Makefile:141-154` runs the multi-repo suite separately under `STRIATUM_MULTI_REPO_REQUIRE_PG=1` against either Go or Python daemon core.
- `tests/_harness/` and `tests/fixtures/` carry the test infrastructure.
- Multiple Go test files (`tests/test_daemon_go_*.py`) invoke the Go binary and assert RPC parity.
- The test surface is broad and unequally instrumented across substrate variants — the Postgres path is by far the most exercised; the legacy SQLite path is currently being deleted out from under tests.

### Release

- 25 version tags in `CHANGELOG.md` between v1.31.0 (2026-05-13) and v1.55.0 (2026-05-15). The 2026-05-18 working tree has another `Unreleased` block.
- `Makefile:175` defines `release-check: check smoke`; `check` runs lint, typecheck, test, ui-check-bundle, ui-test, metadata-check, wheel-size, package-smoke.
- Wheel ships Go binaries via `src/striatum/_daemongo/binaries/striatumd-<plat>-<arch>` (`pyproject.toml:57`, `Makefile:107-121`). The build pipeline cross-compiles four platforms.
- CI is reported as "backlogged" in `docs/ROADMAP.md:36-38`; treat-as-stop-the-line for latest head.

### Where code disagrees with docs

1. **SPEC says SQLite is retired**; codebase still has `schema.py`/`migrations.py`/`db.py` (4,719 LOC) wired through compat wrappers. Production refuses to open SQLite, but the substrate code is still loaded into the process. The docs are being maintained ahead of the cleanup, which is one defensible direction but should be acknowledged in §6 of SPEC.
2. **README and SPEC say "the daemon is Go"**; `daemon_pg/handlers/` is still 19k LOC of Python handlers. Either Python is the test/parity reference and should be marked as such in code, or it's an alternate maintained implementation and should be marked as such.
3. **`pyproject.toml:22` says "Development Status :: 3 - Alpha"**; README and ROADMAP imply something closer to a working production tool. Pick one.

## 4. Strengths

These are specific decisions worth preserving, with the reasons that make them load-bearing.

- **DDD framing is honest and code-enforced**, not retrofit. The reviewer-can't-say-"looks good" example in `docs/DDD.md:138-148` is exactly the right justification for a vocabulary-first design. The 100-method `contracts/daemon_methods.json` is the data-driven realization of that boundary. Keep this; protect it.
- **The daemon RPC method registry is a single source of truth** (`contracts/daemon_methods.json` + generator script `scripts/generate_daemon_method_tables.py`). Capability and audit class are declared per method. This is the right shape for an authorization surface and the right shape for cross-implementation parity (Python vs Go).
- **Front-matter schemas are kind-specific and validate at publish time** (`src/striatum/artifact_contracts.py`, 621 LOC). The decision to make artifacts machine-checkable without forcing YAML/Pydantic dependency was correct.
- **Append-only events with chain anchors and FOR UPDATE chain heads** (Schema v6 per SPEC `docs/SPEC.md:51-55`). This is the only sane choice for an audit chain under concurrent appenders.
- **Hash-comparing UI bundle manifest** (`Makefile:62`, `ui-bundle-hash` target writing `manifest.sha256`). The drift-detection intent is correct; the implementation is broken in a different way (see §5).
- **`daemon-required` enforcement with paired test-harness gate** (`src/striatum/cli/daemon_required.py:73-79`). The bare `STRIATUM_DAEMON_REQUIRED=0` opt-out was rejected as an operator escape and the paired-marker requirement was added. This is exactly the kind of threat-model fix that should be celebrated.
- **`init` no longer creates SQLite fixtures** (`git log --oneline -30`, commit 10fb20a). Stopping the bootstrap path from materializing the legacy substrate is the right closing move on the migration.
- **Workflow validator refuses same-model implementer/reviewer pairings by default** (`docs/SPEC.md:208-212`). The reviewer co-blindness anti-pattern (`docs/ROADMAP.md:133-138`) is a real failure mode; encoding it as a lint refusal at validate time is the right place to put the check.
- **Provider portability is structural, not aspirational.** Lanes are configuration; supervised wrappers are per-tool shell scripts; the runner never imports a vendor SDK (`pyproject.toml:30` — `dependencies = ["jinja2>=3.1", "markdown-it-py>=4.0"]`, full stop). Two runtime dependencies for a system this large is impressive restraint.
- **The dogfood ledger pattern** (`docs/dogfood/HISTORICAL.md`, 66 run directories). Running the runner against itself is a real validation mechanism. The harness-friction patterns doc and proposal-artifact kind close the feedback loop. This is the genuine basis for the project's confidence claims.

## 5. Concerns

Ranked **blocker / serious / smell** with file evidence.

### Blocker

**B1. Two daemon implementations both maintained.** Python `src/striatum/daemon_pg/` (~19k LOC) and Go `go/` (~25k LOC source + 7k test) both implement the daemon RPC contract. `docs/operator/BRIEF.md` says Go is production. `Makefile:101-102` defines `daemon-go-conformance` as a multi-repo test target. The Python handlers are clearly still exercised by `tests/test_daemon_pg*.py`. This is the single largest unanswered architectural question in the tree: is the Python daemon a parity oracle, a deprecated path, or an alternate? Until that's decided, every new feature is built twice and tested in a matrix that one person cannot keep green. *Resolution*: pick one daemon, mark the other as a parity fixture with a written sunset date, and delete the divergent code paths the day the test matrix proves parity.

**B2. The legacy SQLite quarantine is half-shipped and the working tree is mid-cutover.** `git log --oneline -30` shows 15+ consecutive "Quarantine legacy SQLite *" commits. `src/striatum/legacy_sqlite/` is now 11 modules and ~7,000 LOC. `src/striatum/cli/dispatch.py:1638` still checks `STRIATUM_DAEMON_REQUIRED == "0"`. The production refusals are correct; the cleanup is the issue. Two failure modes are possible while this is mid-flight: (a) a fixture path imports SQLite by accident and a regression doesn't fire, (b) the test matrix passes against SQLite fixtures but production fails differently because daemon code paths weren't exercised. *Resolution*: complete the quarantine in a single push. Delete `src/striatum/schema.py`, `src/striatum/migrations.py`, `src/striatum/db.py` after porting the migration fixture tests to a sealed migration-only module. Stop importing `sqlite3` from `src/striatum/cli/dispatch.py` (currently at line 24 via `if TYPE_CHECKING` and line 86 via `import sqlite3 as _sqlite3` in the exception path).

### Serious

**S1. `src/striatum/cli/dispatch.py` is 1,935 lines and is the only place that knows about substrate routing, fixture compat, recovery, init, adopt, skills, and plugins.** This is the seam through which every CLI verb passes. The legacy-SQLite test-harness gating logic (`dispatch.py:27-31, 154-167, 640, 1638, 1711`) is interleaved with daemon-routing logic and direct command dispatch. Refactoring this into `legacy_dispatch.py`, `daemon_dispatch.py`, and a small `router.py` would make the SQLite eradication tractable.

**S2. The CHANGELOG and the version cadence are misleading.** 25 versions in 6 days, with each version block being multiple paragraphs of prose. There is no meaningful release contract here; tags are effectively snapshots. For a single-developer alpha, that's fine — but stop calling them version tags and pretending the artifact is a stable release. Either move to date-only snapshots (`v2026.05.18-1`) or stop bumping the minor on every commit.

**S3. 277 stale `island-shared-*.js` files committed in `src/striatum/web/static/build/`.** Total bundle directory size is 9.9 MB. Vite emits content-hashed bundles, but the build doesn't `rm -rf` the directory first (`Makefile:51` does `ui-clean` → `ui-build`, but only `make ui-build` does, not the manual `npm run build`). The accumulated files are tracked in git and bloat the wheel. *Fix*: make `ui-build` mandatory before any commit touching `src/striatum/web/static/build/`, and have the CI bundle check refuse if more than the named entries plus one `island-shared-<hash>.js` are present.

**S4. The `.striatum/state.sqlite3` removal story has a known hazard.** `docs/SPEC.md:104-120` says writable SQLite import windows are closed and the retired `migrate-repo-local` spelling refuses with exit code 12. *Actual*: the refusal is enforced (good). But `src/striatum/cli/dispatch.py:1667` keeps a `legacy_sqlite` fixture fallback object inside the recovery path. A single-bit mistake in the env-var gate exposes the SQLite codepath. The exposure surface should be reduced by deleting the fixture fallback entirely once `tests/_harness/` proves the migration tests don't need it.

**S5. The CLI argument parser is 1,343 lines.** `src/striatum/cli/parser.py`. Subparsers are added imperatively, not declaratively. Adding a verb requires editing this file *and* `contracts/daemon_methods.json` *and* (usually) a Python handler under `daemon_pg/handlers/` *and* a Go handler under `go/pkg/`. Four-place ceremony per verb is the kind of friction that produces the kind of "we should generate this" complaints that lead to RFC 0060 (single daemon method contract source — `docs/TODO.md:104` says it's done; that doesn't appear to fully cover the CLI parser).

**S6. `docs/SPEC.md` is 1,810 lines and is asked to be the source of truth.** It isn't, in practice, because nobody (including a future you) re-reads 1,810 lines on every task. The DDD doc is 199 lines and does the actual orienting work. Either SPEC needs a brutal cull to ~300 lines of contract-only material with the operational detail moved into per-RFC docs, or it should be relabeled as "behavioral reference" and the DDD doc should be promoted to "the contract."

**S7. The docs apparatus has surpassed the code's ability to evolve it.** 1,500 Markdown files in `docs/`. 72 RFCs. 117 numbered decisions. 66 dogfood run subdirectories, each with its own decisions/findings/synthesis tree. Cross-references between RFCs and decisions are abundant and stale references will accumulate faster than they can be fixed. *This is the single biggest threat to project comprehensibility going forward.* The DECISION_LOG, the RFC index, the dogfood ledger, and the operator brief are four pointers to the current state — when they disagree, the answer "read SPEC" doesn't scale.

### Smell

**Sm1. Top-level repo pollution.** `final_status.json` (256 KB), `status.json` (176 KB), seven `STRIATUM_*_REVIEW_*.md` files and four `STRIATUM_*_REMEDIATION_PLAN*.md` files at the repo root. `ENGRAM_DEVELOPER_REQUEST.md`, `GASTOWN_COMPARISON.md`, `PROJECT_COMPARISON.md`, `CLAUDE_DESIGN_UI_REWORK_PROMPT.md` also at root. These are working notes that belong in a `notes/` directory (or be deleted). The PyPI wheel `MANIFEST.in` excludes them from the wheel (presumably), but git history is forever and the root listing is the first thing a reader sees.

**Sm2. Eight capability scopes for a one-operator system.** `src/striatum/daemon_rpc/registry.py:17` defines `{"read", "write", "review", "claim", "apply", "admin", "recovery", "surgical_recovery"}`. For a single operator on a laptop, this is over-engineered. If the multi-user/hosted future is real, this is appropriate; if not, `read` + `write` covers it.

**Sm3. The `coordinator`-as-claimed-session path is documented but unused.** `docs/UBIQUITOUS_LANGUAGE.md:55` says: "declared in every dogfood workflow but never actually claimed in any run." A first-class role that's been declared-but-not-claimed for the lifetime of the project should be deleted or unblocked. Don't keep schema cruft around for a feature you never use.

**Sm4. `docs/SPEC.md:236-271` documents harness profiles** as a closed set `{generic, codex, claude_code, gemini_cli}` with strict validation. That's three providers and a fallback. Calling it "tool family" with a closed enum is fine; calling the design "model portability" is straining the term.

**Sm5. `src/striatum/cli/__init__.py` is a 100-line `_SYMBOL_MODULES` shim.** Lazy imports for backward compatibility (so historical `from striatum.cli import X` still works) is reasonable, but it shouldn't be 78 entries. Decide which API is public and delete the rest.

**Sm6. `from striatum.legacy_sqlite ...` is still imported from `src/striatum/service.py` and `src/striatum/workflow.py`** in the working tree (per `grep`). Production paths refuse to use SQLite, but the import is loaded — a defense-in-depth concern, not a correctness one.

## 6. North-star architecture

If I were rebuilding this greenfield, for the stated constraints (one operator, laptop/homelab, terminal-agent coordination, audit-quality provenance, no hosted state), here's what I would build.

**Substrate.** One process. SQLite WAL mode for the live state. A single Postgres install is overkill for a single operator on a laptop — Postgres is correct only if (a) you actually run multi-process appenders and need real serializable isolation, or (b) you want the audit-chain row-lock semantics specifically. SQLite WAL handles both cases at this scale, with a far smaller install footprint. I would walk back D094 and ship Postgres only as an optional backend for users who already have it running.

**Daemon vs library.** Make the daemon optional. The DDD argument (daemon RPC is the single write surface) holds even when the daemon is in-process: open a connection, run a transaction, append an event. The reason to have a separate process is *exactly* concurrent agent sessions. If a workflow has one supervised lane, there is no concurrent-writer problem; if it has five, you want the daemon. Build the same handlers behind a `local_invoke()` API and a `daemon_invoke()` RPC envelope; let the operator pick.

**One implementation language.** Either Python or Go, not both. Given the existing 60k LOC of Python and the surrounding tooling (Jinja2, MCP, the web UI), Python is the obvious answer. The Go daemon is a beautiful piece of work that I would not have written for this constraint set. The case for Go is "the daemon needs to survive the operator killing their Python venv mid-run," which is a real concern but solved more cheaply by making the daemon a managed-by-systemd Python process with proper signal handling.

**State shape.** Keep the eight aggregate roots from `docs/DDD.md:84-96`. They're correct. Keep the event log as the source of truth and the SQL state as the materialized projection. Keep front-matter validation at publish time. Keep the daemon-method-as-single-mutation-boundary invariant.

**Boundaries.** Three modules, not seventy:
- `striatum.engine` — schema, mutations, events, validators. Pure; no I/O beyond the SQL connection.
- `striatum.surface` — CLI argparse + dispatch, MCP, web service. Knows about `engine`, doesn't touch SQL.
- `striatum.adapters` — supervisor, process adapter, wrapper script generation. Plugins for Claude/Codex/Gemini live here.

The current tree has the right ideas but the modules are split by accident-of-history (`service.py`, `service_http.py`, `service_api_routes.py`, `service_routes.py`, `service_request_io.py`, `service_request_security.py`, `service_server.py`, `service_sse.py`, `service_state.py`, `service_runtime.py`, `service_command_policy.py`, `service_daemon.py` — `src/striatum/service*.py`). Twelve `service*.py` files is a code-smell that there's one service module trying to escape.

**RFC discipline.** Stop accepting RFCs that just rename or move concepts. RFCs 0050 (UI rework), 0053 (terminology truing), 0054 (day-zero guide), 0055 (marketing README), 0058 (operator progress surface) are all valuable individually but together they encode a culture of "more docs, more decisions." A doc-stable system needs fewer doc updates per code change, not more.

**Frontend.** Delete the React islands. Server-render everything. Five island bundles for a single-operator local web UI is wildly out of proportion. If you keep islands, ship them as a single `static.js` and stop content-hashing.

**Capabilities.** Two: `read` and `write`. Re-add finer-grained scopes the day there's a second consumer that needs them. Today there isn't.

**Documentation.** Three core docs: a 50-line README, a 300-line SPEC of contract-only behavior, and a 200-line DDD-style "what this is and isn't." RFCs as decision provenance, but archived by date and not as a load-bearing artifact set.

## 7. Recommended changes

Only changes I would personally make. Effort is "back-of-envelope, single-operator."

| Priority | Change | Rationale | Benefit | Risk | Effort |
|---|---|---|---|---|---|
| P0 | Decide Go vs Python daemon and delete the loser | B1 above; can't sustain both | Half the code, half the test matrix | One painful week of test refactor | 1–2 weeks |
| P0 | Finish legacy SQLite deletion in one push | B2 above; mid-cutover is the dangerous state | Eliminates whole class of "did the fixture path leak?" bugs | Some compat tests get deleted | 3–5 days |
| P0 | Move `final_status.json`, `status.json`, six `STRIATUM_*_REVIEW_*.md`, four `STRIATUM_*_REMEDIATION_PLAN*.md`, and three other root MD files into `notes/` (or delete) | Sm1; first impression of the repo | Repo root reads as a maintained project | None | 30 min |
| P0 | Atomically rebuild `src/striatum/web/static/build/` before every commit; delete the 270+ stale `island-shared-*.js` files | S3; bundle hygiene | Wheel size; reviewer cognitive load | Need to verify CI catches drift | 1 hour |
| P1 | Split `src/striatum/cli/dispatch.py` (1,935 LOC) into `router.py`, `daemon_dispatch.py`, `local_dispatch.py`, `legacy_compat.py` | S1; the SQLite eradication is gated on this | Cleanup tractable; reviewer can follow | Touches every test path | 2–3 days |
| P1 | Cull `docs/SPEC.md` to ~300 lines of contract-only behavior; move operational detail into per-RFC docs | S6; SPEC is too long to be the source of truth | Future-you re-reads it | Have to decide what's contract vs description | 1 day |
| P1 | Delete `coordinator`-as-claimed-session path | Sm3; never used in practice | Less schema cruft | Future workflows may want it; document explicitly that it's gone | 2 hours |
| P1 | Collapse 12 `service*.py` files into 3 modules (`service.py`, `service_http.py`, `service_state.py`) | N6 module shape | Easier to find things | Some import path changes | 1 day |
| P2 | Move from minor-bump-per-commit to date-stamped versioning (`v2026.05.18-1`) | S2; CHANGELOG is now narrative, not a release log | Stop pretending tags are release contracts | None until someone depends on the version | 1 hour + write a script |
| P2 | Delete `src/striatum/schema.py`, `src/striatum/migrations.py`, `src/striatum/db.py` (after porting fixture migration tests) | Three schema authorities is too many | One schema authority | Test refactor; see P0 #2 | Part of P0 #2 |
| P2 | Reduce capability scopes from 8 to 2 (`read`/`write`); keep the others as constants for future use | Sm2; single-operator system | Less authorization surface | None for current users | 2 hours |
| P3 | Generate `src/striatum/cli/parser.py` from `contracts/daemon_methods.json` | S5; four-place verb ceremony | Adding a verb is one edit | Loses argparse's hand-tuning | 1 week, optional |
| P3 | Stop accepting RFCs that just rename or move concepts; require a code delta in any RFC | S7; doc velocity outpaces code | Slows the docs treadmill | Some valuable terminology cleanups would have to wait | Policy change |

## 8. Functionality I'd add

Same table format. These are features I would actually build given the constraints.

| Priority | Change | Rationale | Benefit | Risk | Effort |
|---|---|---|---|---|---|
| P1 | A `striatum run replay <run_id> --until <event_id>` that re-renders the run's state up to that event | The event log is the right source; replay is implicit but not surfaced | Cheapest possible debugging tool | None | 1 week |
| P1 | An always-on `striatum doctor --watch` that runs the current `doctor` checks every N seconds and dings on degradation | The doctor exists; the "I noticed something was broken three hours ago" gap doesn't | Catches stuck supervisors, stale leases, missing wrappers early | None | 2–3 days |
| P2 | A `striatum diff-workflow <ref1> <ref2>` that shows what changed between two workflow snapshots, by aggregate | Workflow changes mid-run are a known footgun | Reduces "did I just change the contract?" anxiety | None | 2 days |
| P2 | A `striatum corpus query` over the redacted export bundle, returning JSONL | The corpus is for external augmentation but there's no local query surface | Self-service answers to "what did I ship?" without spinning up Engram | Some surface ambiguity vs evidence export | 3–5 days |
| P3 | A first-class `decision propose` verb that scaffolds a draft `decision`-kind artifact with auto-populated context and opens it in `$EDITOR` | The decision artifact is the load-bearing escalation surface; today it's hand-authored | One verb instead of a checklist | None | 1 day |
| P3 | An "agent autopilot" CLI loop that does `claim-next → run wrapper → publish-artifact → verdict` against a supervised lane for one run and exits | Streamlines the most common operator gesture; replaces a checklist with a verb | A "demo mode" that's honest about what's running | Easy to misuse if not bounded by run-id | 1 week |
| skip | Hosted mode | The product boundary explicitly forbids it | — | — | — |
| skip | A "team mode" with login | Single-operator is the constraint | — | — | — |
| skip | Built-in model billing/usage tracking | Out of boundary | — | — | — |

## 9. Execution roadmap

### Today (concrete first step, startable in the next hour)

Delete the 11 dev-scratch files at the repo root: `final_status.json`, `status.json`, the six `STRIATUM_*_REVIEW_*.md` (or move them into `docs/reviews/external/`), and the four `STRIATUM_*_REMEDIATION_PLAN*.md`. This is a no-risk thirty-minute change that makes the next reviewer's job much easier. Then do `ls src/striatum/web/static/build/ | wc -l`; if it's still 285, run `make ui-clean ui-build` and commit the result.

### Next month

P0 items from §7. In order: (1) Go-vs-Python daemon decision, (2) legacy SQLite deletion in one push, (3) split `dispatch.py`. These three together are the closing move on the substrate migration. While in flight, freeze new RFC acceptance.

### Next quarter

P1 items: SPEC cull, `coordinator`-as-claimed-session deletion, `service*.py` collapse. Also: cut the version bump cadence — pick a Friday and don't tag again until the following Friday at the earliest.

Optionally: ship the first P1 functionality item (`replay` or `doctor --watch`); both are small and demo well.

### Long-term

Decide whether striatum is one tool used by one person or a substrate that other people are expected to adopt. The decision determines whether capability scopes, multi-repo, MCP capability tokens, the cross-repo coordinator, and the React island UI stay or go. As of 2026-05-18 the docs hedge; the code commits to the more-ambitious answer; the maintainer (you) is one person. The hedge is the most expensive position to be in.

## 10. Open questions

Things I couldn't determine from the code, and that the next reviewer (or you, in a week) would need to confirm.

- **Is the Python daemon-PG handler tree a parity reference or a maintained alternate?** I couldn't tell from the docs whether `tests/test_daemon_pg*.py` exists to validate the Go daemon or to validate a still-shipping Python daemon mode.
- **Is `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` actually used outside `tests/conftest.py`?** If only the test harness uses it, the option could become `STRIATUM_TEST_HARNESS=1` alone and the daemon-required check could become unconditional in CLI dispatch.
- **Why is `final_status.json` (256 KB) committed?** It looks like a per-run debug dump. Is something reading it?
- **What's the actual end-to-end smoke that proves a fresh-clone install works on macOS today?** `scripts/fresh_clone_smoke.sh` exists; `Makefile:170` defines `smoke`; `docs/ROADMAP.md:36-38` says CI is backlogged. I cannot tell whether a fresh install actually works without running it.
- **Are any users running striatum besides the maintainer?** The PRD says Engram is the "reference customer." If there's a second user, the API surface tradeoffs are different. If there isn't, the eight capability scopes and the cross-repo coordinator should be deleted.
- **What's the operator's mean time between RFCs?** The dogfood ledger has 66 runs; the RFC index has 72 RFCs. That's roughly one RFC per dogfood run. Is that intentional, or are RFCs being used as a substitute for code design?
- **Is the React island toolchain actually used by anyone shipping a striatum install?** `pyproject.toml:30` does not list Node as a dependency. Operators install with `pip`. Are the islands ever rebuilt outside the maintainer's machine? If not, why ship them in the wheel?

---

*Closing note from the reviewer.* The technical core of striatum is good. The DDD framing is genuine. The product boundary is consistent. The runner does coordinate terminal-agent workflows with real audit-chain provenance and real refusal semantics. What it suffers from is *velocity asymmetry* — the docs and RFC apparatus are evolving faster than the code can converge, and two simultaneous substrate migrations have left the working tree in a half-finished state. The fixes are all small and local. Pick a Friday. Don't tag anything until the next Friday. Spend the week doing the P0 deletes.
