author: implementer-codex-1

# dogfood-042 Cross-Track Build Handoff (Phase 1 Consolidation)

Status: complete
Run ID: `run_8bd11d0dd1a043948d6190a3ec1de000`
Branch: `striatum/dogfood-042-multi-phase`

This handoff synthesizes the three parallel tracks of the dogfood-042
multi-phase workflow into a single consolidation record. The
`consolidate_phase_1` job was cascaded into cancellation by the
cycle-exhaustion overrides on Tracks A and C; the operator wrote this
file in its place, citing the per-track HANDOFF.md artifacts as the
authoritative implementation record for each track.

## Cross-Track Summary

| Track | Scope | RFC(s) | Build verdict (codex / claude / gemini) | Override |
|-------|-------|--------|------------------------------------------|----------|
| A     | Go daemon Steps 1+2 (foundation) | RFC 0039 V1 Steps 1+2 | needs_revision / accept_with_findings / accept_with_findings | D095 (operator) |
| B     | Engram Phase 1 implementation spec | RFC 0044 (draft) | accept / accept / accept | none |
| C     | Repo-local state to Postgres spec | RFC 0042 (draft) | needs_revision / accept / accept_with_findings | D096 (operator) |

All three tracks landed under run `run_8bd11d0dd1a043948d6190a3ec1de000`.
Tracks B and C are RFC-only (no source-tree mutation); Track A ships
the new `go/` source tree plus Python harness extensions.

## Track A — Go Daemon Steps 1+2 (RFC 0039 V1 Phase 1)

Source HANDOFFs:
- [`track_a/build/systems/HANDOFF.md`](track_a/build/systems/HANDOFF.md)
  (Go-side, `implementer-codex-gpt-5.5-003`)
- [`track_a/build/glue/HANDOFF.md`](track_a/build/glue/HANDOFF.md)
  (Python-glue + docs, `implementer-claude-opus-001`)

### Shipped

Go source tree under `go/`:

- `go/cmd/striatumd/main.go` — `striatumd-go` entry point that applies
  configured daemon DB migrations, binds an owner-only Unix socket, and
  serves newline-delimited envelope-v1 JSON.
- `go/pkg/rpc/` — envelope validation/serialization, error responses,
  the RFC 0030 method registry and capability vocabulary, in-memory
  capability helpers, handshake handling, `daemon.describe`, duplicate
  request detection, and an RPC server framework for read-only routes.
- `go/pkg/db/` — daemon Postgres config resolution/redaction, a
  dependency-free `psql` runner, migration loading/application,
  embedded copies of the existing daemon SQL migrations, and audit
  hash/recording helpers.
- `go/go.mod`, `go/go.sum`, `go/Makefile` — Go module + `build`,
  `test`, `lint` targets. Standard-library-only for this slice.

Python-side glue:

- `tests/_harness/daemon.py` — `DaemonProcess` gained keyword-only
  `daemon_core: Literal["python","go"]` parameter (default
  `"python"`); existing call sites unchanged. Go invocation resolves
  the binary via `STRIATUMD_GO_BIN` or `<repo>/go/bin/striatumd` and
  runs `make -C go build` once when the binary is missing. Refuses
  with a clear error when `make` / `go` and the override are absent.
  Passes `--socket`, `--db-url`, and `--migrations-dir` so the Go core
  runs the same Python-owned migration set without a `//go:embed`
  drift class (per synthesis §3.2).
- `tests/_harness/multi_repo.py` — `MultiRepoHarness.__init__` gained
  the kw-only `daemon_core` parameter; `start()` and
  `restart_daemon()` thread it into each `DaemonProcess`. New
  `daemon_core` read-only property exposes the chosen core to test
  code.
- Root `Makefile` exposes `daemon-go-build`, `daemon-go-test`,
  `daemon-go-lint`.

Documentation:

- `docs/HOW_TO_HUMAN.md` — "Running the Go daemon (developer preview,
  RFC 0039 Phase 1)" subsection.
- `docs/SPEC.md` — daemon section extended for the second daemon
  implementation under the same RFC 0030 envelope-v1 contract and
  RFC 0033 PostgreSQL substrate; V1-closed `{python, go}` core set;
  PostgreSQL-layer mutex; Phase 1 read-only verb list; cross-language
  v2 audit-row hash parity guarantee; explicit deferral of Steps 3-6.
- `docs/UBIQUITOUS_LANGUAGE.md` — new `daemon core` term row.
- `docs/rfcs/0039-go-daemon-core.md` — status line bumped.

### Build verdicts

- codex (build_review): **needs_revision** — overridden per D095.
  Codex/codex implementer+reviewer pairing converged on the same
  blind spots (anti-pattern; see TODO item 26).
- claude (build_review): **accept_with_findings**.
- gemini (build_review): **accept_with_findings**.

Per D095 (`dec_b75d66f38a3d40228891248c91a27774`,
`accepted_with_follow_up`), 2-of-3 reviewers accept_with_findings and
the codex findings fold into RFC 0039 V1.5 (TODO item 24).

### Deferred to Phase 2 (RFC 0039 Steps 3-6)

- `striatum daemon start --core go` Python CLI selection.
- Mutating workflow verbs on the Go core.
- Supervised-process plumbing in Go.
- Distribution / release artifacts / `daemon_core` parametrized CI
  matrix.
- The `tests/test_daemon_go_smoke.py` end-to-end smoke test and the
  `tests/_harness/audit_fixture.py` Python-side canonical fixture
  generator (out-of-scope for the Track A glue packet).
- Parametrize the five existing RFC 0035 e2e files across
  `daemon_core` once the Go core implements mutating verbs.

## Track B — RFC 0044 Engram Phase 1 Implementation Spec

Source HANDOFF: [`track_b/build/HANDOFF.md`](track_b/build/HANDOFF.md)
(`implementer-codex-gpt-5.5-001`).

### Shipped

- `docs/rfcs/0044-engram-phase-1-implementation-spec.md` — Phase 1
  implementation spec following the accepted Track B synthesis:
  pull-mode ingestion, Striatum-owned redacted JSONL export, Engram-
  owned `ingest-striatum`, standalone `engram-mcp-stdio`, four
  read-only retrieval tools, Engram-local `memory.*` capabilities,
  and a hard augmentation-not-dependency boundary.
- RFC text explicitly notes the numbering drift from RFC 0041: RFC
  0044 is the Phase 1 read-only implementation, not Phase 3 write-
  side ingestion.

### Build verdicts

- codex (build_review): **accept**.
- claude (build_review): **accept**.
- gemini (build_review): **accept**.

3-of-3 accept; no override required.

### Deferred

- Implementation of RFC 0044 V1 lands via a future dogfood (TODO item
  23).
- No Engram source-tree changes; Engram concepts cited from local
  Engram docs and the accepted Track B synthesis only.

## Track C — RFC 0042 Repo-Local State to Postgres

Source HANDOFF: [`track_c/build/HANDOFF.md`](track_c/build/HANDOFF.md)
(`implementer-codex-gpt-5.5-002`).

### Shipped

- `docs/rfcs/0042-repo-local-state-to-postgres.md` — proposal to move
  authoritative live workflow state from per-repository
  `.striatum/state.sqlite3` files into daemon Postgres keyed by
  `repository_id`, making the daemon mandatory for all state-touching
  reads and writes. Repo-local `.striatum/` retained as operational
  scratch only.
- The draft includes the required schema migration shape, all eighteen
  repo-local application tables plus non-migrated `schema_meta`,
  composite-key rules, the `striatum daemon migrate-repo-local --from
  sqlite --to pg --repo <path>` verb, daemon-unavailable refusal
  behavior, RFC 0039 scope revision, migration ordering and rollback,
  audit-chain preservation, acceptance criteria, implementation plan,
  D006/D007/D028 supersession statement, open questions, and domain
  modeling.

### Build verdicts

- codex (build_review): **needs_revision** — overridden per D096.
  Same codex/codex implementer+reviewer anti-pattern as Track A.
- claude (build_review): **accept**.
- gemini (build_review): **accept_with_findings**.

Per D096 (`dec_b81a0ec524964a518ed90c0ae5826408`,
`accepted_with_follow_up`), 2-of-3 reviewers accept-equivalent and the
codex findings absorb into the future RFC 0042 V1 implementation
dogfood (TODO item 22).

### Note on D093 citation

The Track C synthesis identifies D093 as the umbrella supersession
decision for RFC 0042 (superseding D006/D007/D028). The current
`docs/DECISION_LOG.md` row for D093 describes RFC 0040 operator-side
harness work. The RFC phrasing follows the work packet and synthesis
verbatim; the decision log was not altered by Track C. Future cleanup
may want to either (a) split the D093 citation, (b) renumber the
supersession decision, or (c) leave the discrepancy with a pointer
note. This is deferred to the RFC 0042 V1 implementation dogfood.

### Deferred

- Implementation of RFC 0042 V1 lands via a future dogfood (TODO item
  22).
- Once RFC 0042 V1 lands, RFC 0039 Phase 2 picks up Steps 3-6 against
  the Postgres-only repo-local substrate.

## Cross-Track Findings & Anti-Pattern

The codex/codex implementer+reviewer cycle-exhaustion pattern recurred
twice in this run (D095, D096). The reviewer's findings clustered
around the same blind spots the implementer had, producing apparent
"needs_revision" verdicts where the other two reviewers
(claude, gemini) returned accept-equivalent. The pattern was already
noted as a future harness improvement in TODO item 20 (dogfood-040)
and TODO item 21 (dogfood-041); two more observations in this run
escalate it to TODO item 26 (workflow validator rule against same-
model implementer↔reviewer pairs).

## Phase 2 Absorption

Phase 2 (a future dogfood) is expected to:

1. Land RFC 0042 V1 (repo-local state → Postgres) so the Go core has
   a single canonical substrate (TODO item 22).
2. Address RFC 0039 V1.5 build review findings — codex / claude /
   gemini — per D095 (TODO item 24).
3. Land RFC 0039 Steps 3-6: CLI integration, mutating verbs,
   supervised processes, distribution (TODO item 25).
4. Land the harness improvement forbidding codex/codex
   implementer+reviewer pairings in the workflow validator (TODO
   item 26).
5. Land RFC 0044 V1 (Engram Phase 1 read-only MCP) independently of
   the RFC 0039 / 0042 chain (TODO item 23).

## References

- Per-track HANDOFFs:
  [Track A systems](track_a/build/systems/HANDOFF.md) ·
  [Track A glue](track_a/build/glue/HANDOFF.md) ·
  [Track B](track_b/build/HANDOFF.md) ·
  [Track C](track_c/build/HANDOFF.md)
- Decisions: [D095](decisions/D095_cycle_exhaustion_track_a.md) ·
  [D096](decisions/D096_cycle_exhaustion_track_c.md)
- RFCs:
  [0039](../../rfcs/0039-go-daemon-core.md) ·
  [0042](../../rfcs/0042-repo-local-state-to-postgres.md) ·
  [0044](../../rfcs/0044-engram-phase-1-implementation-spec.md)
- Operator narrative: [PHASE_1_OPERATOR_NOTES.md](PHASE_1_OPERATOR_NOTES.md)
- Operator report: [OPERATOR_REPORT.md](OPERATOR_REPORT.md)
