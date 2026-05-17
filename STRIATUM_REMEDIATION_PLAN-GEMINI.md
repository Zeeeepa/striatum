# Striatum Architecture Remediation Plan

Based on the architectural review dated 2026-05-16, the following step-by-step plan outlines the remediation strategy to correct the identified concerns and align the codebase with the target architecture.

## Step 1: Eradicate SQLite Fallbacks & Update Documentation (Immediate)
- **Goal:** Masking daemon failures and state-split risks caused by legacy SQLite fallbacks must be eliminated.
- **Action Items:**
  1. Delete production SQLite fallback pathways for all core workflow verbs.
  2. Implement a hard failure/error response if the daemon RPC is unreachable.
  3. Update all relevant documentation (e.g., `README.md` and `GETTING_STARTED.md`) to definitively state PostgreSQL as the sole authoritative state substrate.
  4. Verify resolution of GH #15.

## Step 2: Resolve Daemon Core Strategy & Unify Contract (Near-Term)
- **Goal:** Eliminate the split-brain logic bifurcated between Python (`daemon_pg/`) and Go (`go/pkg/`).
- **Action Items:**
  1. Record a definitive architectural decision (RFC/Decision Log) for the daemon substrate. (Recommendation: Transition fully to Go).
  2. Halt new feature development on Python `daemon_pg` handlers if Go is the chosen path.
  3. Define and generate a single source-of-truth RPC contract file to strictly bind the CLI client to the daemon handlers.
  4. Port remaining essential Python handlers to Go.

## Step 3: Enforce Reviewer Diversity (Near-Term)
- **Goal:** Prevent "co-blindness" exhaustion loops caused by using the same model for both implementation and review.
- **Action Items:**
  1. Implement a workflow validation rule that explicitly refuses identical model pairings in the `implement` and `review` lanes.
  2. Add an `--allow-same-model-pairing` flag to allow intentional overrides for specific testing or fallback scenarios.

## Step 4: Daemon-First Web Service (Medium-Term)
- **Goal:** Ensure a single-source-of-truth paradigm for the state.
- **Action Items:**
  1. Refactor the local web dashboard/service to remove its direct PostgreSQL database connections.
  2. Route all dashboard state queries and mutations exclusively through the daemon's RPC layer.

## Step 5: Process Supervision & Resilience Upgrades (Medium-Term)
- **Goal:** Prevent stalling from non-interactive terminal restrictions and increase robustness against agent crashes.
- **Action Items:**
  1. Implement "Auto-Finalize from Front Matter" (RFC 0051) to allow the system to successfully complete jobs if an agent crashes after writing valid artifact schemas to disk.
  2. Upgrade the process supervisor (adapter) to allocate true PTYs (e.g., via `creack/pty` in Go) rather than standard Unix FIFOs.

## Step 6: Expand Capabilities (Long-Term)
- **Goal:** Polish the dual-role operating model and increase project-level context for agents.
- **Action Items:**
  1. **Corpus V2 Integration:** Finalize Engram memory integration (RFC 0052/0057) allowing AI agents to access historical run provenance.
  2. **Human Principal Inbox:** Build a dedicated terminal/web cross-repo inbox that aggregates all `waiting_human` states and workflow blockers into a single review queue.
  3. **Git Integration (Optional):** Add plugin-based functionality to automatically stage, commit, and push synthesized artifacts to a remote branch or open a Pull Request.