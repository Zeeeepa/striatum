# Implement: RFC 0039 V1.5 (claude — mixed Go + Python harness)

Blocked until `review_design` returns an accepting verdict.

Implement RFC 0039 V1.5 per `docs/dogfood/047/DESIGN_SYNTHESIS.md`. **You are the claude lane implementer** — deliberately NOT codex, to avoid the 5-time codex/codex anti-pattern from prior cascades. You write **both Go (under `go/`) and Python harness (under `tests/_harness/`)** as the synthesis requires.

**Your scope:**

- `go/cmd/striatumd/main.go` — F1 swap `AllowAllAuthorizer` for the Postgres-backed authorizer; F2 align the accepted flags with the harness contract.
- `go/pkg/rpc/capability.go` and the new authorizer file the synthesis names (e.g. `go/pkg/rpc/auth_pg.go`) — F1.
- `go/pkg/db/audit.go` — F4 wrap append in a transaction at the synthesis-locked isolation level.
- `go/pkg/db/connection.go` — F5 replace `psql` shell-out with the synthesis-locked pure-Go driver. Update `go/go.mod` / `go/go.sum`. This is the first third-party Go dep — call it out in HANDOFF.
- `go/Makefile` — F2 binary target / path so `tests/_harness/daemon.py` finds it.
- `tests/_harness/daemon.py` — F2 launch flags / binary path. F3 surface `CORE` plumbing.
- `tests/_harness/multi_repo.py` (or the file the synthesis chose) — F3 pytest parametrize matrix.
- `Makefile` — F3 add `test-multi-repo CORE=go` target.
- `tests/` — unit + regression tests per finding. F4 race test required.
- `docs/rfcs/0039-go-daemon-core.md` — append a V1.5 deltas section summarizing F1-F5. Do not rewrite V1 sections.
- `docs/dogfood/047/build/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per finding, dispatched in parallel:

- Sub-agent F1: Postgres token validator + `main.go` swap + denial audit hook.
- Sub-agent F2: harness launch fix + `go/Makefile` target + binary path smoke.
- Sub-agent F3: Makefile target + pytest parametrize + opt-in test selection.
- Sub-agent F4: transactional append + race regression test.
- Sub-agent F5: driver swap + `go.mod` updates + first-third-party-dep callout.

Reconcile sub-agent outputs yourself before writing HANDOFF. Respect the implementation order the synthesis locks (e.g. F5 may gate F4 if connection shape changes).

**Do NOT write to**: anything outside `allowed_paths`. **No README / TODO / CHANGELOG / RFC index / SPEC / HOW_TO updates** — the operator handles those manually after the dogfood lands (no in-workflow consolidate job; dogfood-042 cascade lesson).

Verification: `make lint`, `make typecheck`, `make test` all pass. `make test-multi-repo CORE=go` passes locally with the Go daemon (F2 + F3 acceptance). Go-side: `go test ./...` under `go/` passes including the F4 race test.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
