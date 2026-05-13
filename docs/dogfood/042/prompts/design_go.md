# Track A Design Prompt: Go Daemon Steps 1+2

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/042/track_a/design/<lane>/`).

Design the implementation for **RFC 0039 Phase 1, Steps 1+2 ONLY**:

- **Step 1**: Go skeleton (`go/cmd/striatumd/main.go`), envelope-v1 framing, `daemon.hello`/`daemon.welcome` handshake, `daemon.describe` method registry exposition, capability-bound method registry. Reads from Postgres only (no SQLite). Handles read-only RPC verbs first.
- **Step 2**: Postgres substrate — `go/pkg/db/connection.go`, `go/pkg/db/migrations.go` (loads from `src/striatum/daemon_pg/sql/*.sql` or embeds), `go/pkg/db/audit.go` (audit-chain hash helper).

**Do NOT design Steps 3-6** (CLI integration, mutating verbs, supervised processes, distribution). Those are Phase 2 of this multi-phase workflow or future dogfoods.

Cover concretely:

- `go.mod` module path + Go 1.23 toolchain.
- `go/pkg/rpc/{envelope,registry,capability,server}.go` exact responsibilities.
- Capability vocabulary: same closed set as RFC 0030 (`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, `surgical_recovery`).
- `go/Makefile` build targets.
- How the Go daemon coexists with the Python daemon (per RFC 0039 §9 Phase 1).
- RFC 0035 multi-repo harness extension shape for `daemon_core="go"` parameter.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly.

This is a one-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, the operator publishes on your behalf.
