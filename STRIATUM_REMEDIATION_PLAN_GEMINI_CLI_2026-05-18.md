# STRIATUM ARCHITECTURE REMEDIATION PLAN
author: planner-gemini-cli-001

## 0. Source review

- **Source:** `STRIATUM_ARCHITECTURE_REVIEW_CODEX_GPT_5_2026-05-18.md`
- **Date:** 2026-05-18
- **Model:** CODEX_GPT_5
- **Staleness note:** The review is entirely fresh relative to the current repository state. Citations regarding the removal of `src/striatum/legacy_sqlite/daemon_registry.py` and the size of `src/striatum/cli/dispatch.py` match the working tree. No material code drift has occurred that would invalidate the review's architectural claims.

## 1. Executive summary

- The primary objective is to finalize the transition to the Go/PostgreSQL daemon by ruthlessly deleting the remaining legacy Python daemon and SQLite compatibility surfaces (P0).
- Secondary objectives (P1) focus on tightening boundaries: definitively identifying direct-PG bootstrap commands, securing the MCP daemon surface, and providing cryptographic provenance for the Go binary.
- Ergonomic operator features (like unified diagnostics) are scheduled as P1s because they drastically reduce the support burden for a single maintainer.
- Expanding daemon capabilities for features lacking product consensus (like archive browsing) and building custom documentation generators have been explicitly dropped from this plan to conserve maintainer bandwidth.

## 2. Disagreements with the review

- **Generate authority documentation from the method contract (R5):** I am dropping this. Building and maintaining a custom AST/JSON parser to auto-generate markdown tables is a poor ROI for a solo maintainer. Manually updating the authority matrix markdown during PRs is vastly cheaper.
- **Read-only archive/corpus browsing over the daemon (F3):** I am dropping this. The review itself notes that the `corpus v2` product direction remains unfinalized. We should not expand the daemon RPC surface area for a speculative feature.
- **Keep local authoring separate from live workflow state (R7):** I strongly agree with this, but it is an architectural principle, not a discrete, executable work item. Therefore, it does not have a dedicated P-tier ticket below.

## 3. P0 - blocking

### P0-LEGACY-DELETE
- **source:** R1 (delete the legacy Python daemon and SQLite substrate in stages)
- **what:** Convert or delete the remaining legacy SQLite fixture tests, then delete `src/striatum/daemon.py`, `src/striatum/db.py`, and the bulk of `src/striatum/legacy_sqlite/`.
- **why:** A massive, quarantined legacy backend slows every architectural change, confuses new routing, and creates the risk that a future command accidentally relies on the superseded SQLite state instead of the Go/PG daemon.
- **touches:** `src/striatum/daemon.py`, `src/striatum/db.py`, `src/striatum/legacy_sqlite/`, `tests/architecture/test_legacy_sqlite_quarantine.py`
- **effort:** 2 weeks
- **depends on:** none
- **acceptance:** The quarantine test allowlist is empty, and no production Python code imports `sqlite3` or `striatum.db` outside of explicitly named, single-file fixture importers.

## 4. P1 - serious

### P1-ADMIN-PLANE-DEFINE
- **status:** completed 2026-05-18; the command authority matrix now names
  the direct PostgreSQL bootstrap/admin plane, and an architecture guardrail
  scans Python client/CLI sources for unlisted direct daemon-PG helper imports.
- **source:** R2 (define and test the bootstrap/admin plane)
- **what:** Add an architecture guardrail test that explicitly allowlists the CLI/admin commands permitted to use direct PostgreSQL connections (e.g., `adopt`, `repo add`, `daemon doctor`).
- **why:** Without a formal boundary, future developers might use "bootstrap" as a loophole to bypass the daemon and directly mutate live workflow state from the CLI.
- **touches:** `tests/architecture/test_authority_guardrails.py`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
- **effort:** 1 day
- **depends on:** none
- **acceptance:** A new architecture test fails if any live-state workflow command imports or calls the direct-PG admin helpers.

### P1-MCP-DAEMON-SURFACE
- **source:** R3 & F1 (make daemon MCP the normal live-operation MCP surface)
- **what:** Replace the `LocalRpcServer` fallback in `src/striatum/mcp.py` with a generated, capability-filtered MCP manifest backed strictly by daemon RPC.
- **why:** Agents must only discover and use daemon-backed live-state verbs. Exposing CLI-shaped compatibility tools allows agents to bypass the strict orchestration rules enforced by the Go daemon.
- **touches:** `src/striatum/mcp.py`, `src/striatum/daemon_rpc/server.py`
- **effort:** 3 days
- **depends on:** none
- **acceptance:** An agent connecting via MCP only receives tools defined in `contracts/daemon_methods.json` that its token's capability explicitly grants.

### P1-GO-RELEASE-PROVENANCE
- **status:** completed 2026-05-18; Go daemon builds stamp package version,
  git SHA, and dirty/clean state into `striatumd --describe`, while the
  Python launcher rejects unstamped `go-dev` binaries and missing git
  provenance before socket bind.
- **source:** R4 (stamp the Go daemon binary with release/build metadata)
- **what:** Inject the package version, git SHA, and contract etag into the Go daemon build process, and expose them via a `striatumd --describe` command.
- **why:** A single operator needs to be able to trivially prove exactly which binary is running when debugging local orchestration failures; `go-dev` is practically useless.
- **touches:** `Makefile`, `go/cmd/striatumd/main.go`, `src/striatum/cli/daemon.py`
- **effort:** 2 days
- **depends on:** none
- **acceptance:** Running `striatumd --describe` returns the actual package version and git SHA, and package smoke tests assert this data matches the Python wheel.

### P1-FIRST-RUN-DIAGNOSTIC
- **source:** F4 (make first-run and package diagnostics a single operator command)
- **what:** Consolidate the outputs of `adopt --first-run-smoke` and `daemon doctor --authority` into a single, unified JSON diagnostic report.
- **why:** Reduces operator friction by providing a single, comprehensive command to verify the entire stack (binary freshness, DB connection, auth token, and MCP status).
- **touches:** `src/striatum/day_zero.py`, `src/striatum/cli/dispatch.py`
- **effort:** 2 days
- **depends on:** P1-GO-RELEASE-PROVENANCE
- **acceptance:** The operator can run one command to get a unified JSON report covering all local-first prerequisites with direct next-action hints.

## 5. P2 - smell / nice-to-have

### P2-ACCEPTED-RISK-PERSISTENCE
- **source:** F2 (persist accepted risk decisions in PostgreSQL)
- **what:** Add a PostgreSQL table to record accepted risk decisions (keyed by repo, run, artifact, accepting role, and rationale) and surface it in the dashboard read DTOs.
- **why:** The current linting warns about risks but lacks a durable place to record when an operator explicitly accepts them, leaving audit trails incomplete.
- **touches:** `src/striatum/daemon_pg/sql/`, `src/striatum/daemon_pg/handlers/`, `src/striatum/workflow.py`
- **effort:** 1 week
- **depends on:** P0-LEGACY-DELETE
- **acceptance:** Accepting a risk during a run inserts a row into the new PG table, which is then exposed in `striatum status --json` and evidence exports.

### P2-TRIM-UNUSED-HOOKS
- **source:** R6 (trim unused cross-repo runner hooks)
- **what:** Remove or unexport placeholder methods (e.g., `Prepare`, `HumanCheckpoint`) from the Go cross-repo runner interface until they are actively required.
- **why:** Keeping speculative, unused interfaces creates confusion about what orchestration features are actually supported and wired up.
- **touches:** `go/cmd/striatumd/main.go`, `go/pkg/crossrepo/lifecycle.go`
- **effort:** 4 hours
- **depends on:** none
- **acceptance:** The Go cross-repo interface only defines methods that have active, tested implementations.

## 6. Dependency map

The dependency chain is strictly sequential for the foundational architectural changes. **P0-LEGACY-DELETE** must happen first to clear the board of any legacy SQLite confusion before we start building new Postgres features like **P2-ACCEPTED-RISK-PERSISTENCE**.

Simultaneously, **P1-GO-RELEASE-PROVENANCE** must land before **P1-FIRST-RUN-DIAGNOSTIC**, because the diagnostic tool relies on the binary provenance data to report the daemon's exact version and SHA. The remaining P1 and P2 items (**P1-ADMIN-PLANE-DEFINE**, **P1-MCP-DAEMON-SURFACE**, **P2-TRIM-UNUSED-HOOKS**) are largely independent and can be tackled in any order.

- `P0-LEGACY-DELETE` must land before `P2-ACCEPTED-RISK-PERSISTENCE`.
- `P1-GO-RELEASE-PROVENANCE` must land before `P1-FIRST-RUN-DIAGNOSTIC`.

## 7. What I'd defer indefinitely

- **Generated Command Authority Matrix (R5):** As outlined in the disagreements section, writing doc-generation scripts for a single maintainer is less efficient than making manual markdown edits.
- **Read-only archive/corpus browsing over the daemon (F3):** We should not expand the daemon RPC surface area to support features (like `corpus v2`) that lack a firm product direction or decision.

## 8. Open questions

- **Tombstone Lifecycle:** Once legacy data is fully migrated to Postgres and `state.sqlite3.tombstone` files are left behind, what is the product decision on their lifecycle? Should the daemon eventually auto-delete them, or is the operator expected to manually `rm` them? If the latter, we risk accumulating disk clutter.
- **Cross-Repo Ambitions:** R6 asks to trim unused cross-repo hooks. Is cross-repo orchestration still a near-term roadmap priority? If not, we might want to excise the `crossrepo` package entirely rather than just trimming individual unused hooks.
