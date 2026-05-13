# Coordinator Role (Dogfood 049 — RFC 0039 Phase 2)

You keep the operator-driven dogfood-049 moving. 10 jobs total, **two**
parallel implementer tracks. The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on RFC 0039 Phase 2 (Steps 3-6) across both tracks.
2. **1 synthesis** — codex picks one path; locks Track A + Track B scope.
3. **1 design review** — claude `ergonomics_dx` gates implement.
4. **2 implementers** — Track A codex (Steps 3+4: CLI integration in the
   Python CLI + mutating verbs / apply / MCP / cross-repo in the Go core),
   Track B claude (Steps 5+6: supervisor lifecycle in Go + distribution
   cross-compile + Python wheel package-data + CI matrix). Sub-agents
   aggressively inside each track.
5. **3-way build review** — codex `threat_model`, claude `ergonomics_dx`,
   gemini `adversarial threat_model`, running in `parallel_group:
   build_review`.

After build review, the operator runs consolidation manually. There is
**no** consolidate job in this workflow. The operator does RFC index,
TODO, CHANGELOG, SPEC, HOW_TO updates by hand once the dogfood lands
(dogfood-042 cascade lesson).

**Why two tracks, not four**: RFC 0039 Phase 2 has four steps (3, 4, 5,
6), but Steps 3+4 (CLI integration + mutating verbs) are the same lane of
work (Python CLI + Go RPC registry / apply / MCP / cross-repo) and Steps
5+6 (supervisor + distribution + CI) are the same lane of work (Go
supervisor + Makefile + wheel package-data + CI workflow). Folding 3+4
into Track A and 5+6 into Track B gives codex one substantive implementer
slot and claude one, balancing load. Four mini-tracks would have either
(a) sequenced into a single lane and lost parallelism or (b) doubled up
on codex (two of the four naturally land on Go-heavy work codex is
strong at), which would re-create the codex/codex anti-pattern
already catalogued at five instances (D095-D098, D100). Routing Track B
to claude is the second deliberate route-around following the D099 +
D101 pattern.

**Scope boundary**: Striatum-side only. RFC 0039 Phase 2 covers Steps
3-6 of the Go daemon rewrite. The Python daemon stays functional. The
`--core go` flag is opt-in; the default is `python` (Phase 2 per RFC 0039
§9 — flipping the default to Go — is a **separate** future RFC and is
out of scope for this dogfood). Windows daemon, hosted-mode, and
Prometheus metrics are explicit non-goals per RFC 0039.

Allowed write scope (enforced by the validator):

- Track A (codex): `go/cmd/`, `go/pkg/rpc/`, `go/pkg/apply/`, `go/pkg/mcp/`,
  `go/pkg/crossrepo/`, `go/go.mod`, `go/go.sum`, `src/striatum/cli/daemon.py`,
  `src/striatum/cli/parser.py`, `tests/cli/`, `tests/daemon_rpc/`,
  `tests/test_daemon_go_mutations.py`.
- Track B (claude): `go/pkg/supervisor/`, `go/Makefile`, top-level
  `Makefile`, `.github/workflows/`, `src/striatum/_daemongo/`,
  `pyproject.toml`, `MANIFEST.in`, `tests/test_daemon_go_supervisor.py`,
  `tests/_harness/`, `tests/conftest.py`.

Gemini is reserved for design and adversarial review only. Never
implementer.

**Anti-patterns to expect** (from the dogfood-042 to dogfood-048
cadence):

- codex-reviewer-of-claude-implementer (now 3 instances: D099, D101,
  dogfood-048 D102) — Track B is on claude, so plan for codex
  `threat_model` review to come back harsh against Track B even when
  scope is met. 2-of-3 cross-lane consensus is the established override
  path.
- claude-no-artifact (3 instances per dogfood-048) — if a claude
  reviewer doesn't write the REVIEW.md, operator composes
  accept_with_findings on their behalf rather than re-running.
- gemini-no-frontmatter (3 instances per dogfood-048) — if gemini
  publishes a finding without all 5 front-matter fields, operator
  patches in place.
- codex/codex co-blindness (5 instances: D095-D098, D100) — explicitly
  avoided by routing Track B to claude.

D094 framing applies: per RFC 0043 the daemon is the sole substrate and
Postgres is the sole substrate. The Go daemon implements RFC 0030 over
the same Postgres schema as the Python daemon. No parallel SQLite path.
The two cores are mutually exclusive at runtime via the pidfile +
socket-path lock.
