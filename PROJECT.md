# Project: Striatum GitHub Issues Resolution (Gen 3)

## Architecture
Striatum is a standalone, local-first workflow runner for terminal-based AI coding agents.
- **cmd/striatum**: CLI entrypoint, providing commands like `complete`, `submit-review`, etc.
- **cmd/striatumd**: Daemon that manages persistent agent state, interacts directly with PostgreSQL database, and coordinates supervised lanes.
- **cmd/striatum-supervisor-helper**: Helper process spawned under PTY execution to manage I/O, FIFOs, and attestation.
- **pkg/db**: PostgreSQL database interface, using migrations and schema models.
- **pkg/mutations**: Implements mutations such as session registration, artifact publication, and review submission.
- **pkg/reads**: Implements read projections for dashboards, runs, and conversation histories.
- **pkg/lanehealth**: Unified classification authority for lane health (liveness, attestation, delivery).
- **pkg/supervisor**: Manages PTY lifecycle, supervision, attestation validation, and process-exit handling.
- **pkg/artifactcontracts**: Validates front-matter schemas for Markdown artifacts.

## Code Layout
- `go/cmd/`: Executable binaries (`striatum`, `striatumd`, `striatum-supervisor-helper`)
- `go/pkg/`: Core domain logic packages
- `go/pkg/db/`: Database models, schemas, and migrations
- `go/pkg/lanehealth/`: Lane health checks
- `go/pkg/mutations/`: Core state transition handlers
- `go/pkg/reads/`: Query and UI rendering projections
- `go/pkg/supervisor/`: PTY monitoring and session execution

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Exploration & Triage | Research all six outstanding issues (#49, #54, #57, #58, #59, #60) | None | DONE |
| 2 | CLI, Session & Front-Matter | Implement fixes for #57, #58, #59, and #60 | M1 | DONE |
| 3 | PTY Supervision & Rebridge | Implement fixes for #49 and #54 | M1, M2 | DONE |
| 4 | Verification & Audit | End-to-end Go test suite pass, lint verification, and Forensic Audit | M2, M3 | DONE |

## Interface Contracts
### CLI ↔ Daemon (RPC / MCP)
- CLI commands call Daemon via MCP or JSON-RPC methods to mutate state or query projections.
- Daemon owns authority guardrails, verifying caller roles and transaction integrity.

### Supervisor ↔ Database
- The supervisor process monitors child processes under PTY and records status transitions.
- Unexpected exits transition the state to terminal states in PostgreSQL.

### Artifact Verification ↔ Front-matter schemas
- Front-matter carry-overs validate against V1 schema using `pkg/artifactcontracts` to ensure structured integrity.
