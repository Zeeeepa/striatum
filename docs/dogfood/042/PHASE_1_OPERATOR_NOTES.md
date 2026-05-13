author: operator-claude-opus-1

# dogfood-042 Phase 1 Operator Notes

Status: complete (with manual consolidation)
Run ID: `run_8bd11d0dd1a043948d6190a3ec1de000`
Branch: `striatum/dogfood-042-multi-phase`
Date: 2026-05-13

Operator-narrative summary for the [`OPERATOR_REPORT.md`](OPERATOR_REPORT.md)
to reference. The detailed cross-track build record lives in
[`BUILD_HANDOFF.md`](BUILD_HANDOFF.md); this file frames the run shape,
the cycle-exhaustion anti-pattern, and what Phase 2 absorbs.

## What shipped (by track)

### Track A — Go daemon foundation

RFC 0039 V1 Steps 1+2 landed:

- New `go/` source tree (`cmd/striatumd`, `pkg/rpc`, `pkg/db`,
  `go.mod`, `go.sum`, `Makefile`). Standard-library-only for this
  slice; Go module exposes `build` / `test` / `lint` targets, and the
  root `Makefile` mirrors them as `daemon-go-build` /
  `daemon-go-test` / `daemon-go-lint`.
- Python harness extensions (`tests/_harness/daemon.py`,
  `tests/_harness/multi_repo.py`) gained the keyword-only
  `daemon_core: Literal["python","go"]` parameter (default
  `"python"`). Backward-compatible by construction.
- Documentation updates in `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, and the RFC 0039 status line.

Steps 3-6 (Python CLI `--core go` selection, mutating verbs,
supervised processes, distribution) deferred to a Phase 2 dogfood per
RFC 0039 §9 and the synthesis split.

### Track B — RFC 0044 Engram Phase 1 spec

`docs/rfcs/0044-engram-phase-1-implementation-spec.md` drafted. Pull-
mode ingestion with Striatum-owned redacted JSONL export, Engram-
owned `ingest-striatum`, standalone `engram-mcp-stdio` MCP server,
four read-only retrieval tools, Engram-local `memory.*` capabilities,
and a hard augmentation-not-dependency boundary. Numbering drifts
from RFC 0041's 4-phase roadmap: this is the Phase 1 read-only
implementation, not Phase 3 write-side.

### Track C — RFC 0042 repo-local state to Postgres

`docs/rfcs/0042-repo-local-state-to-postgres.md` drafted. Moves
authoritative live workflow state from per-repository
`.striatum/state.sqlite3` into daemon Postgres keyed by
`repository_id`. Eighteen application tables, composite-key rules,
`striatum daemon migrate-repo-local --from sqlite --to pg --repo
<path>` migration verb, daemon-unavailable refusal behavior, audit-
chain preservation, and an RFC 0039 scope revision so the Go core
assumes Postgres-only repo-local state. Supersedes D006/D007/D028 per
D093.

## The cycle-exhaustion pattern

Two of the three tracks hit cycle exhaustion on build review and
required operator override:

- **Track A** — codex `needs_revision` overridden per
  [D095](decisions/D095_cycle_exhaustion_track_a.md). 2-of-3
  reviewers (claude, gemini) returned `accept_with_findings`. Codex
  findings absorbed into RFC 0039 V1.5.
- **Track C** — codex `needs_revision` overridden per
  [D096](decisions/D096_cycle_exhaustion_track_c.md). 2-of-3
  reviewers (claude accept, gemini accept_with_findings) returned
  accept-equivalent. Codex findings absorbed into the future RFC
  0042 V1 implementation dogfood.

The pattern that emerged: **codex/codex implementer+reviewer pairing
converges on the implementer's own blind spots**. The implementer
ships an artifact; a same-model reviewer is structurally biased
toward the same kinds of objections the implementer would also have
caught if they were going to. The result is a stream of
"needs_revision" verdicts that the other two reviewers don't share,
and the cycle counter exhausts before the implementer can find a
formulation the same-model reviewer is willing to accept.

This pattern was already noted as a future harness improvement after
dogfood-040 (TODO item 20) and dogfood-041 (TODO item 21). Two more
observations in this single run escalate it to TODO item 26: add a
workflow validator rule (warn or reject) for same-model
implementer↔reviewer pairs, or enforce it via catalog template
posture.

Track B used codex as the implementer with claude / gemini /
**codex** as reviewers and still got 3-of-3 accept. The anti-pattern
appears to be **codex implementer + codex reviewer on artifacts where
codex is the dominant author voice** specifically, not codex review
in general. The validator rule should be on the
implementer-vs-reviewer model pair, not on reviewer model alone.

## How the run completed despite cascaded cancellation

The `consolidate_phase_1` job sat downstream of all three tracks'
build jobs. When Track A and Track C build verdicts came in as
`needs_revision`, the job's dependency state cascaded into
cancellation rather than proceeding past unresolved review verdicts.
The operator overrides (D095, D096) landed as durable decision
artifacts but did not retroactively reverse the cascade.

To complete the run, the operator manually wrote the artifacts that
`consolidate_phase_1` would have produced:

1. `docs/rfcs/README.md` — RFC 0042 (proposed) and RFC 0044
   (proposed) added to the index; RFC 0039 status bumped to
   `accepted (V1 Steps 1+2 implemented; Steps 3-6 Phase 2)`.
2. `docs/TODO.md` — F41/F42/F43 done rows added; five new open
   follow-up items (22 RFC 0042 V1, 23 RFC 0044 V1, 24 RFC 0039
   V1.5, 25 Phase 2 Steps 3-6, 26 workflow validator harness
   improvement). Items 20 and 21 remain pending.
3. `CHANGELOG.md` — Unreleased block describing all three tracks,
   D095, D096, and a note that the consolidate_phase_1 job was
   cascaded into cancellation.
4. `docs/dogfood/042/BUILD_HANDOFF.md` — cross-track build handoff
   synthesizing the four per-track HANDOFF.md artifacts (Track A
   ships two: systems + glue).
5. This file.

Each of these would normally carry an implementer byline; the
build-handoff file carries `author: implementer-codex-1` because that
is the role consolidate_phase_1 was scoped against. This file
carries `author: operator-claude-opus-1` because it is operator
narrative, not implementer work.

## What Phase 2 absorbs

Phase 2 is a future dogfood, not part of this run. It is expected to
land:

1. **RFC 0042 V1** (TODO item 22) — repo-local state → Postgres,
   eighteen tables, migration verb, daemon-mandatory state reads.
   Goes first because RFC 0039 Phase 2 needs a single canonical
   substrate.
2. **RFC 0039 V1.5** (TODO item 24) — address Track A build review
   findings (codex / claude / gemini deltas).
3. **RFC 0039 Phase 2 Steps 3-6** (TODO item 25) — CLI integration
   (`striatum daemon start --core go`), mutating workflow verbs on
   the Go core, supervised processes in Go, and distribution
   (release artifacts, macOS/Linux CI matrix across
   `daemon_core={python,go}`).
4. **Harness improvement: forbid codex/codex implementer+reviewer
   pairing** (TODO item 26) — validator rule plus catalog template
   enforcement. May be folded into one of the Phase 2 sub-RFCs or
   shipped as a standalone follow-up.
5. **RFC 0044 V1** (TODO item 23) — Engram Phase 1 read-only MCP.
   Independent of the RFC 0039 / 0042 chain; ordering is flexible.

The Track C synthesis's claim that D093 supersedes D006/D007/D028 is
in tension with the current `docs/DECISION_LOG.md` row for D093
(which describes RFC 0040 operator-side harness work). The RFC text
follows the synthesis verbatim; resolving the citation drift is
deferred to the RFC 0042 V1 implementation dogfood.

## Notes for the OPERATOR_REPORT.md

- Three parallel tracks shipped; two required operator override at
  build-review time. Codex/codex implementer+reviewer is the
  recurring anti-pattern; escalate the harness improvement to a
  validator rule.
- `consolidate_phase_1` cascaded into cancellation; the operator
  wrote the five expected output files (README index, TODO,
  CHANGELOG, BUILD_HANDOFF, these notes) manually after recording
  the decisions.
- Phase 2 absorbs the deferred follow-up: RFC 0042 V1 first (Postgres
  substrate), then RFC 0039 V1.5 + Phase 2 Steps 3-6, plus the
  harness validator rule and RFC 0044 V1 in parallel.
