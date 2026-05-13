# Track A Go Core Handoff

author: implementer-codex-gpt-5.5-003
status: complete

## Scope Shipped

Implemented the Step 1+2 Go-side daemon foundation under `go/`:

- `go/cmd/striatumd/main.go` adds a small `striatumd-go` entry point that can apply configured daemon DB migrations, bind an owner-only Unix socket, and serve newline-delimited envelope-v1 JSON.
- `go/pkg/rpc/` implements envelope validation/serialization, error responses, the RFC 0030 method registry and capability vocabulary, in-memory capability helpers, handshake handling, `daemon.describe`, duplicate request detection, and an RPC server framework for read-only routes.
- `go/pkg/db/` implements daemon Postgres config resolution/redaction, a dependency-free `psql` runner, migration loading/application, embedded copies of the existing daemon SQL migrations, and audit hash/recording helpers.
- `go/go.mod`, `go/go.sum`, and `go/Makefile` define the Go module and `build`, `test`, and `lint` targets.
- The repository `Makefile` now exposes `daemon-go-build`, `daemon-go-test`, and `daemon-go-lint`.

The Go module intentionally uses only the Go standard library in this slice. That keeps this supervised invocation local-only and avoids fetching third-party dependencies while still establishing the package boundaries expected by RFC 0039.

## Verification

Passed:

```bash
cd go && make test
cd go && make lint
cd go && go build ./cmd/striatumd
```

The build command creates a local `go/striatumd` binary; I removed that generated binary after verification so it is not left as a tracked artifact candidate.

Not exercised in this environment:

- A live Postgres end-to-end RPC smoke with migrations applied, because no test `STRIATUM_DAEMON_DB_URL` was configured for this packet. The DB package has fake-runner tests for migration ordering/application and is wired for `psql` execution when a URL is supplied.

## Notes For The Next Slice

This is a Step 1+2 foundation, not the full RFC 0039 daemon. Mutating workflow verbs, Python CLI `--core go` launch glue, cross-repo lifecycle, MCP mutation routing, apply service, and supervised process ownership remain out of this job's write scope.
