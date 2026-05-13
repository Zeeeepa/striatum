# Synthesis Prompt: RFC 0039 V1.5 (Go daemon findings F1-F5)

Produce `docs/dogfood/047/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/047/design/codex/DESIGN.md", "docs/dogfood/047/design/claude_code/DESIGN.md", "docs/dogfood/047/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 designs into ONE concrete plan for RFC 0039 V1.5. Choose; do not enumerate. If the three designs disagree, pick one and justify in one sentence.

- **F1 Go RPC authorization**: exact Postgres token-validator implementation. Name the Go package / file path (e.g. `go/pkg/rpc/auth_pg.go`), the SQL query reused from `go/pkg/db/sql/`, cache policy (none vs TTL), denial envelope shape, and audit-on-deny hook. Name how `go/cmd/striatumd/main.go` swaps `AllowAllAuthorizer` for the new authorizer.
- **F2 daemon_core=go harness launch**: locked argv / env contract between `tests/_harness/daemon.py` and the Go binary. Exact `go/Makefile` target name and output binary path. Exact smoke command the operator runs to confirm.
- **F3 make test-multi-repo CORE=go**: exact Makefile target shape (parameter forwarding through env or argument), the pytest parametrize fixture in `tests/_harness/` (one chosen file), and the test selection rule (which tests opt in to the matrix).
- **F4 Go audit-chain transactional append**: exact `go/pkg/db/audit.go` function signature change, isolation level (`SERIALIZABLE` vs `READ COMMITTED` — pick one), exact regression test path under `tests/` (Python or Go-side — pick one), race scenario.
- **F5 pure-Go Postgres driver**: locked choice (`lib/pq` OR `pgx` — pick one with one-sentence justification). Exact `go.mod` line. Exact `go/pkg/db/connection.go` shape post-migration. Explicit acknowledgement this is the first third-party Go dep and what that means for supply-chain review.

Order the five items in implementation order (which gates which). Identify any cross-finding dependencies (e.g. F5 driver swap before F4 transactional append? or after?).

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
