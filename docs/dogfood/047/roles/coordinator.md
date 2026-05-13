# Coordinator Role (Dogfood 047 — RFC 0039 V1.5, Go daemon F1-F5)

You keep the operator-driven dogfood-047 moving. 9 jobs total, single
track addressing the five findings from the dogfood-042 Track A build
review of the Go daemon core. The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on the F1-F5 deltas.
2. **1 synthesis** — codex picks one path from the three designs and
   locks the implementation order across the five findings.
3. **1 design review** — claude `ergonomics_dx` posture gates the
   synthesized design before implement.
4. **1 implementer** — **claude** on mixed Go (`go/`) + Python harness
   (`tests/_harness/`). Sub-agents aggressively, one per finding.
   **NOT codex** — deliberate avoidance of the 5-time codex/codex
   anti-pattern.
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial threat_model`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** consolidate job in this workflow. The operator does the RFC
index, TODO, CHANGELOG, SPEC, and HOW_TO updates by hand once the
dogfood lands (dogfood-042 cascade lesson).

**Scope (the five findings):**

- F1 (high) Postgres-backed token authorizer replaces `AllowAllAuthorizer`
  in `go/cmd/striatumd/main.go` + `go/pkg/rpc/capability.go`.
- F2 (high) `daemon_core=go` harness launch fix in
  `tests/_harness/daemon.py` + `go/Makefile`.
- F3 (high) `make test-multi-repo CORE=go` Makefile target + pytest
  parametrization.
- F4 (medium) Go audit-chain transactional append in `go/pkg/db/audit.go`.
- F5 (medium) replace `psql` shell-out in `go/pkg/db/connection.go` with
  a pure-Go Postgres driver (first third-party Go dep — supply-chain
  callout required).

Allowed write scope (enforced by the validator):

- `go/` — daemon, RPC, db packages, Makefile.
- `tests/_harness/`, `tests/` — harness fixes + regression tests.
- `Makefile` — F3 target.
- `docs/rfcs/0039-go-daemon-core.md` — V1.5 deltas section only.
- `docs/dogfood/047/build/HANDOFF.md` — handoff.

Gemini is reserved for design and adversarial review only. Never
implementer.
