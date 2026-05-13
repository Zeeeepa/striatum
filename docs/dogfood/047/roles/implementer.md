# Implementer Role (Dogfood 047 — claude, mixed Go + Python harness)

Single implementer, **claude lane** — deliberately not codex, to avoid
the 5-time codex/codex anti-pattern from prior cascades. The workflow
validator enforces the write scope — stay strictly inside your job's
`write_scope.allowed_paths`.

Owns:

- `go/cmd/striatumd/main.go` — F1 swap `AllowAllAuthorizer` for the
  Postgres-backed authorizer; F2 align accepted flags with the harness
  contract.
- `go/pkg/rpc/capability.go` + the new authorizer file the synthesis
  names (e.g. `go/pkg/rpc/auth_pg.go`) — F1.
- `go/pkg/db/audit.go` — F4 transactional append at the
  synthesis-locked isolation level.
- `go/pkg/db/connection.go` — F5 replace `psql` shell-out with the
  synthesis-locked pure-Go driver. Update `go/go.mod` / `go/go.sum`.
- `go/Makefile` — F2 binary target / output path the harness expects.
- `tests/_harness/daemon.py` — F2 launch flags + binary path; F3 `CORE`
  plumbing.
- `tests/_harness/multi_repo.py` (or the file the synthesis chose) —
  F3 pytest parametrize matrix.
- `Makefile` — F3 `test-multi-repo CORE=go` target.
- `tests/` — unit + regression tests per finding. F4 race test required.
- `docs/rfcs/0039-go-daemon-core.md` — append a V1.5 deltas section
  summarizing F1-F5. Do not rewrite V1 sections.
- `docs/dogfood/047/build/HANDOFF.md` — handoff.

Use sub-agents aggressively — one per finding in parallel (F1 authorizer,
F2 launch, F3 matrix, F4 transactional append, F5 driver swap).
Reconcile outputs yourself before writing HANDOFF. Respect the
implementation order the synthesis locks.

**F5 supply-chain callout (non-negotiable)**: first third-party Go dep
landing in `go/go.mod`. Call it out in HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. No
`docs/rfcs/README.md`, `docs/TODO.md`, `CHANGELOG.md`, `docs/SPEC.md`,
`docs/HOW_TO_*.md`, `docs/UBIQUITOUS_LANGUAGE.md` — operator handles
those manually (dogfood-042 cascade lesson).

One-shot supervised invocation. Do not ask follow-ups. If `striatum
ack` is denied, write the artifact and exit normally. Lease can expire
if `make test` exceeds ~30 min — prefer focused pytest / `go test`
first. Per D089/D091, OPERATOR_REPORT.md is the operator's
responsibility, not yours.
