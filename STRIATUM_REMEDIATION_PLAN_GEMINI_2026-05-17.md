# STRIATUM ARCHITECTURE REMEDIATION PLAN

Status: superseded historical input

Supersession note, 2026-05-18: This root-level planning artifact is retained
as source material for RFC 0068-0071 and the architecture remediation
synthesis. It reflects D105-era assumptions that are no longer current. D107 /
RFC 0068 now targets the Go production daemon, Python daemon retirement after
parity, and SQLite removal from production and compatibility paths. Current
source-of-truth status lives in `docs/SPEC.md`, `docs/TODO.md`,
`docs/ROADMAP.md`, `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`, and
`docs/rfcs/0068-go-production-daemon-port.md`. Several recommendations below
have since landed or inverted: the dispatch fail-closed guard, PostgreSQL
daemon registry/global surface work, `repo.resolve`, dynamic RPC client
versions, `/v1/invoke` daemon routing, `daemon doctor --authority`, and the
Go daemon parity/retirement-ledger work.

## 0. Source review

- **Review File:** `STRIATUM_ARCHITECTURE_REVIEW_CODEX_GPT_5_2026-05-17.md`
- **Date:** 2026-05-17
- **Model:** Codex GPT-5
- **Repo State Context:** The review is entirely fresh and accurate. The code paths cited (e.g., `src/striatum/cli/dispatch.py` fallthroughs and `src/striatum/daemon.py` SQLite references) are present in the exact lines mentioned. No drift has occurred since the review was generated.

## 1. Executive summary

- The primary goal of this remediation cycle is to finalize the transition to the PostgreSQL daemon (D094) by ruthlessly eliminating lingering SQLite fallback paths.
- The highest priority (P0) is plugging a silent split-brain hole in the CLI dispatch and migrating the daemon's own global registry to Postgres.
- Secondary priorities (P1) focus on reducing client-side database connections and solidifying Python as the single production core.
- Feature additions like new `doctor` flags or archive inspections are explicitly deferred; the focus is exclusively on enforcing the architectural boundaries we already claim to have.

## 2. Disagreements with the review

- **`striatum doctor --authority` (Drop):** The review suggests adding a new doctor command to report which commands are legacy vs. daemon-native. I am dropping this. If we execute the P0 plan, the legacy commands physically fail or disappear. We don't need a diagnostic to tell us what is legacy if we just delete the legacy paths.
- **Archive/replay inspection & Durable accepted-risk linkage (Drop):** The review itself admits these are waiting on product decisions (corpus v2 and PR integration). We will not build complex inspection tooling for features that lack product consensus.
- **Generated Command Authority Matrix (Drop):** The review recommends auto-generating the matrix from the JSON contract. For a single maintainer, the ROI on building a doc-generator parser is lower than just manually updating a markdown table once a month.

## 3. P0 — blocking

These items represent split-brain risks or data integrity threats. They must land first.

### P0-DISPATCH-FAIL-CLOSED
- **source:** §7 Recommended Changes (Fail closed on daemon-route exceptions)
- **what:** Replace the broad `except Exception` fallthrough in `striatum.cli.dispatch.py` with a hard fail-closed `StriatumError` for any command present in the daemon contract.
- **why:** Prevents a scenario where a transient daemon RPC error causes the CLI to silently drop back to mutating the legacy `state.sqlite3` file, creating an unrecoverable split-brain state.
- **touches:** `src/striatum/cli/dispatch.py`, `tests/architecture/test_authority_guardrails.py`
- **effort:** 1 day
- **depends on:** none
- **acceptance:** A test setting `STRIATUM_SQLITE_CONNECT_TRIPWIRE=1` and forcing a route exception verifies that `run start` exits nonzero and prints an error without touching SQLite.

### P0-PG-REGISTRY-PORT
- **source:** §7 Recommended Changes (Port/disable SQLite daemon registry surfaces)
- **what:** Port `repo add/list/remove`, `dashboard.all`, daemon sweep, and daemon MCP resources to use the PostgreSQL registry instead of `striatumd.sqlite3`.
- **why:** The daemon currently uses Postgres for workflow state but still uses SQLite for its own registry. This violates D094 and forces clients to juggle two database paradigms.
- **touches:** `src/striatum/daemon.py`, `src/striatum/cli/daemon.py`
- **effort:** 1 week
- **depends on:** none
- **acceptance:** Running `striatum daemon start` and `striatum repo add` no longer creates or opens `striatumd.sqlite3`.

## 4. P1 — serious

These items cause architectural drag or operator confusion but don't actively risk data corruption today.

### P1-DAEMON-REPO-RESOLVE
- **source:** §7 Recommended Changes & §8 Functionality I'd Add
- **what:** Add a `repo.resolve` daemon RPC method and migrate the CLI/Web service to use it instead of opening the Postgres database directly to map directories to IDs.
- **why:** Clients should be pure RPC clients. Forcing the CLI to bundle `psycopg` and connect directly to the DB to resolve an ID breaches the daemon capability boundary.
- **touches:** `src/striatum/daemon_rpc/registry.py`, `src/striatum/cli/daemon_rpc_route.py`, `src/striatum/service_daemon.py`
- **effort:** 3 days
- **depends on:** P0-PG-REGISTRY-PORT
- **acceptance:** The `striatum` CLI and `/v1/health` web endpoints no longer import `psycopg` directly.

### P1-REMOVE-GO-DAEMON-OPTION
- **source:** §7 Recommended Changes (Enforce D105 in operator surfaces)
- **what:** Remove `go` from `VALID_DAEMON_CORES` in `src/striatum/cli/daemon.py` and hide it from all operator-facing documentation.
- **why:** Leaving the Go daemon as an operator-selectable choice contradicts D105 (Python is the core). The Go code should be strictly relegated to PTY helper duties to avoid splitting the maintainer's focus.
- **touches:** `src/striatum/cli/daemon.py`, `Makefile`
- **effort:** 1 day
- **depends on:** none
- **acceptance:** `striatum daemon start --core go` is rejected or marked explicitly as an undocumented harness flag.

### P1-DYNAMIC-CLIENT-VERSION
- **source:** §7 Recommended Changes (Remove hardcoded client versions)
- **what:** Replace the hardcoded `1.51.0` and `1.67.0` version strings with `striatum.__version__` in the RPC handshake logic.
- **why:** Hardcoded client versions will eventually cause obscure handshake rejections when the daemon schema bumps, creating frustrating debug loops.
- **touches:** `src/striatum/cli/daemon_rpc_route.py`, `src/striatum/day_zero.py`
- **effort:** 2 hours
- **depends on:** none
- **acceptance:** The RPC envelope sent by the CLI accurately reflects the installed package version.

## 5. P2 — smell / nice-to-have

These items improve long-term codebase health but are not urgently required for stability.

### P2-SERVICE-INVOKE-ROUTING
- **source:** §7 Recommended Changes (Split remaining local service command surface)
- **what:** Refactor the `/v1/invoke` web endpoint to route production mutations through the daemon RPC client instead of calling `striatum.api.invoke` directly in-process.
- **why:** Currently, the web UI bypasses the formal RPC boundary that the CLI uses. Routing it through the RPC client ensures identical capability gating.
- **touches:** `src/striatum/service.py`
- **effort:** 3 days
- **depends on:** P1-DAEMON-REPO-RESOLVE
- **acceptance:** Clicking a mutation button in the Web UI triggers a standard JSON-RPC envelope over the daemon socket instead of a direct Python function call.

### P2-MIGRATION-CLEANUP-REPORT
- **source:** §8 Functionality I'd Add (Migration cleanup report)
- **what:** Add verbose, structured success output to `migrate-repo-local` that confirms the PG rows exist, the event chain is anchored, and the old SQLite file is safely tombstoned.
- **why:** Gives the operator explicit confidence that the scary cutover to PostgreSQL worked correctly.
- **touches:** `src/striatum/daemon_pg/repo_local_migration.py`
- **effort:** 2 days
- **depends on:** none
- **acceptance:** The `migrate-repo-local` command prints a checklist of verified constraints upon completion.

## 6. Dependency map

The core sequence focuses on state authority first. **P0-PG-REGISTRY-PORT** must be completed before **P1-DAEMON-REPO-RESOLVE**, because we cannot move repo ID resolution into the daemon until the daemon's own global registry of repos is fully ported to Postgres. **P0-DISPATCH-FAIL-CLOSED** is standalone and can be tackled immediately to stop silent fallbacks. The P1 items (Go daemon removal, dynamic versions) and P2 items can be parallelized or picked up as isolated tasks.

- P0-PG-REGISTRY-PORT must land before P1-DAEMON-REPO-RESOLVE.
- P1-DAEMON-REPO-RESOLVE must land before P2-SERVICE-INVOKE-ROUTING.

## 7. What I'd defer indefinitely

- **Generated Command Authority Matrix:** Building automation to parse JSON contracts into a markdown matrix is heavy. A single maintainer can just manually update the matrix during PRs.
- **`striatum doctor --authority`:** We are enforcing the daemon boundary through code (P0-DISPATCH-FAIL-CLOSED). A diagnostic tool telling us what is legacy is redundant if we are actively deleting the legacy paths.
- **Archive/replay inspection:** Provenance replay is a huge feature. Until the `corpus v2` product direction is firmly decided and implemented, building CLI tools to inspect it is premature.
- **Durable accepted-risk linkage:** Requires a product decision on where risk is persisted (DAG vs. metadata vs. commit). Do not build schema for this until the decision is made.

## 8. Open questions

- **Performance of P0-PG-REGISTRY-PORT:** The `dashboard.all` command currently reads the SQLite registry to find all repos, then queries their status. When moved to Postgres, will this require a complex cross-schema JOIN, or will it remain an N+1 query pattern? If it's N+1, it may be slow on homelabs with many repos.
- **Tombstone Lifecycle:** Once `migrate-repo-local` completes, the `state.sqlite3.tombstone` file remains. Is there a product decision on when (if ever) the daemon automatically deletes these tombstones, or is the operator expected to manually `rm` them?
