# Designer Role (Dogfood 047)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0039 V1.5 — the five findings from the dogfood-042
Track A build review. Synthesis picks one path and locks
implementation order. Cite the existing Go code that your design
changes — do not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0039-go-daemon-core.md` — current V1 spec.
- `docs/dogfood/042/track_a/review/build/codex/REVIEW.md` — primary
  F1-F5 source.
- `docs/dogfood/042/track_a/review/build/claude/REVIEW.md` and
  `gemini/REVIEW.md` — corroborating angles.
- `go/cmd/striatumd/main.go` — daemon entry, current
  `AllowAllAuthorizer` wiring.
- `go/pkg/rpc/capability.go` — `Authorizer` interface.
- `go/pkg/db/audit.go` — append path (F4 race surface).
- `go/pkg/db/connection.go` — `psql` shell-out (F5).
- `go/Makefile` — build artifact name and target (F2).
- `tests/_harness/daemon.py` — Go daemon launch path (F2).
- `tests/_harness/multi_repo.py` — multi-repo harness surface (F3).
- `tests/_harness/tokens.py` — Python-side token parity (F1 reference).

Address each finding with: exact file path, exact function or symbol
touched, locked interface / SQL / argv contract, and the test that
proves the fix.

**F5 supply-chain note**: this is the first third-party Go dep landing
in `go/go.mod`. Justify the choice (lib/pq xor pgx) in one sentence and
note the supply-chain implication explicitly.

Out of scope: RFC 0039 V2 work, new RPC capabilities, harness rewrites
beyond F2/F3 plumbing, doc updates beyond build/HANDOFF and the RFC
0039 V1.5 deltas section.
