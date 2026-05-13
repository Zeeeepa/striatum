# Design Prompt: RFC 0039 V1.5 (Go daemon findings F1-F5)

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/047/design/<lane>/`).

Design **RFC 0039 V1.5 deltas** addressing the five findings from the dogfood-042 Track A build review. Read these first:

- `docs/rfcs/0039-go-daemon-core.md` — current spec.
- `docs/dogfood/042/track_a/review/build/codex/REVIEW.md` — F1-F5 source. Read primary.
- `docs/dogfood/042/track_a/review/build/claude/REVIEW.md` and `gemini/REVIEW.md` — corroborating angles.

Cover each finding concretely, with pinpoint citations to the existing Go source under `go/` and the Python harness under `tests/_harness/`:

- **F1 (high) Go RPC fail-open authorization** — `go/cmd/striatumd/main.go` wires `rpc.AllowAllAuthorizer`; replace with a Postgres-backed token validator. Cite `go/pkg/rpc/capability.go` (the `Authorizer` interface), the existing token table in `go/pkg/db/sql/`, and the Python parity surface in `tests/_harness/tokens.py`. Spec the validator interface, lookup path, cache policy (if any), denial response shape, and audit emission on deny.
- **F2 (high) `daemon_core="go"` harness launch broken** — flag mismatch and binary path. Cite `tests/_harness/daemon.py` (how it launches the Go daemon today), `go/Makefile` (build artifact name), and the flags the Go `main.go` actually accepts. Spec the corrected launch (env var contract or argv shape), the `make` target that produces the binary at the path the harness expects, and the smoke that proves it works.
- **F3 (high) `make test-multi-repo CORE=go` not wired** — add Makefile target + pytest parametrization. Cite the existing `make test-multi-repo` target, `tests/_harness/multi_repo.py`, and how `CORE` is (or should be) plumbed to fixtures. Spec the new target, the pytest parametrize matrix (`python` vs `go`), and which tests opt in.
- **F4 (medium) Go audit-chain race** — `go/pkg/db/audit.go` append is non-transactional. Cite the current append function and the SQL schema. Spec the transactional wrapper (`BEGIN ... COMMIT`, isolation level, hash-chain link read inside the transaction), and a regression test under `tests/` that races two appenders.
- **F5 (medium) replace `psql` shell-out** — `go/pkg/db/connection.go` shells out to `psql`. Spec the pure-Go driver choice (lib/pq vs pgx — pick one and justify in one sentence), the migration to `database/sql`, what changes in `go/go.mod`/`go.sum` (first third-party dep — call it out explicitly), and any connection-string / TLS handling differences from the shell-out path.

Cite existing code (function names, line ranges). Hand-waving "we add an authorizer" without a pinpoint citation is grounds for design review to bounce.

Out of scope: RFC 0039 V2 work, new RPC capabilities, harness rewrites beyond F2/F3 plumbing, doc updates beyond build/HANDOFF.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
