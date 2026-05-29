# Aggregated Synthesis Plan: Striatum Gen 2 Hardening

This synthesis aggregates the findings and implementation roadmaps from our three parallel Explorer subagents to address:
1. GitHub Issues & TODOs (MCP cleanup, Supervised exits, Conversation UI)
2. Workspace Security & Attestation Parity (RFC 0090)
3. Lane Health Module Alignment (RFC 0091)

## Sequential Implementation Strategy

To prevent merge conflicts and file clobbering in our shared workspace, we will execute the implementation track in three sequential phases:

### Phase 1: GitHub Issues & TODOs
- **Objective:** Clean up `.gemini/settings.json`, persist unexpected supervisor exits in Postgres, and render the conversation UI.
- **Worker Target:** `teamwork_preview_worker_m1`
- **Reference Plan:** `~/git/striatum/.agents/teamwork_preview_explorer_m1_1_gen2/handoff.md`

### Phase 2: RFC 0090 (Workspace Security & Attestation Parity)
- **Objective:** Implement Sandbox Path-Jailing, dynamic advisory lock derivation, ENXIO resilience named-pipe ring buffer, unprivileged test connection pool, Darwin proc_pidinfo process attestation, and dynamic port loopback discovery.
- **Worker Target:** `teamwork_preview_worker_m2`
- **Reference Plan:** `~/git/striatum/.agents/teamwork_preview_explorer_m1_2_gen2/handoff.md`

### Phase 3: RFC 0091 (Lane Health Module Alignment)
- **Objective:** Build unified `lanehealth` package and Checker interface, migrate ad-hoc checks in mutations, reads, and interrogations, and remove all duplicate attestation parsing logic.
- **Worker Target:** `teamwork_preview_worker_m3`
- **Reference Plan:** `~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen2/handoff.md`

---

## Unified Verification Plan

After each phase:
1. **Compilation & Unit Tests:** The worker must verify that `go test -race ./...` compiles and passes cleanly with zero race conditions.
2. **Lints & Style Check:** The worker must run `make check` / `make lint` and fix any warnings or lints.
3. **Forensic Integrity:** The Forensic Auditor will verify that all changes conform to design boundaries.
