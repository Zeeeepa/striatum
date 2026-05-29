# TASK — Implement RFC 0093 (Structured Live-Collaboration Workflow Shapes)

Land **RFC 0093** (`docs/rfcs/0093-structured-live-collaboration-workflow-shapes.md`)
as a working V1, end to end, with all gates green. This is a real
implementation run on the Striatum codebase itself (Go-only runtime, RFC 0078).

## What RFC 0093 adds

A family of **live-collaboration workflow shapes** composed from the existing
RFC 0082 interrogation + RFC 0086 conversation primitives, unified by a shared
**substance-gate**. The substrate is verified ready (see the RFC's "Substrate
readiness" section); this run builds the catalog + gate + artifact contract on
top of it.

## V1 scope (build, in this order)

1. **`collaboration_ledger.v1` artifact contract** — register
   `striatum.collaboration_ledger.v1` in `go/pkg/artifactcontracts`, validated
   at `publish-artifact` (exit 6 on invalid front matter). Schema per RFC §4:
   `shape`, `topic`, `participants[]`, `entries[]` (kind ∈
   claim|challenge|rebuttal|constraint|nomination, `by`, `refs[]` into RFC 0081
   trajectory turn ids, curated `text` only — D028, never raw stdout),
   `verdict` (accept|accept_with_findings|needs_revision|reject), `rationale`.
2. **Collaboration shape pack** — a catalog/authoring input (sibling to RFC
   0074 role/adversary packs and RFC 0087 frame packs) bound by the RFC 0034
   generator. No new live-dialog daemon method.
3. **Substance-gate / adjudicator** — a `phase_synthesis`-class gate job
   (RFC 0045) occupied by an `adjudicator` role that reads ONLY the RFC 0081
   `dialogue` trajectory, emits a `collaboration_ledger`, and gates the
   downstream commit on substance (a constraint extracted, a challenge landed
   AND rebutted) — NOT on dialog completion. Reviewer-independent (RFC 0064).
4. **V1 shapes (pure composition first):** `falsification_gate`,
   `cross_examination`, and the `scribe` participant modifier — these need no
   new primitive. `fog_of_war_review` and `synaptic_prune` may follow if time
   permits (the latter wants the optional `post_dialog_hook`); defer cleanly if
   not reached.
5. **Reference fixtures + docs** — a validating example workflow per shipped
   shape under `examples/`; update `docs/reference/workflow-types.md`,
   `docs/reference/ubiquitous-language.md` (`collaboration shape`,
   `substance-gate`, `adjudicator`, `collaboration_ledger`), and the
   `docs/reference/spec.md` shape list. Re-express RFC 0083 as a catalog entry.

## Acceptance criteria (from RFC §"Acceptance Criteria")

- `workflow generate --shape falsification_gate` (and `cross_examination`)
  produces a `striatum.workflow.v1.1` graph passing `workflow validate` /
  `workflow lint`, wiring existing RFC 0082/0086 calls in packet `commands`.
- The adjudicator gate makes the commit/proposal job **unreachable** until a
  clearing `collaboration_ledger` is published; `needs_revision` routes back
  into a bounded dialog round.
- **Anti-theater test (the bar):** a seeded transcript of hollow questions +
  fluent non-answers → `needs_revision` (commit stays gated); a transcript
  with a landed-and-rebutted challenge → clearing verdict.
- `striatum.collaboration_ledger.v1` registered + validated (exit 6 on
  invalid); fixture exercises every `entries[].kind`.
- D028 guard: no ledger field carries provider stdout/stderr.
- Reviewer independence: adjudicator lane ≠ holder/proposer; RFC 0064
  same-model refusal applies; override audited.
- No new daemon method, no floor-control primitive, no economy/reputation
  store, no vendor SDK import; RFC 0078 Go-only guardrails stay green.

## Constraints

- **Scope discipline:** build V1 (items 1–5). Defer `fog_of_war_review`
  beyond the gate/ledger/pure-composition core if it risks the run; record the
  deferral in the handoff.
- **Substrate seating:** keep the critical authoring path (synthesis, build) on
  claude + codex; agy is the third design/review seat (newest agent-loop path
  per RFC substrate note + GH #51 resolution).
- **Product boundary (AGENTS.md / spec):** daemon-owned PostgreSQL stays the
  sole live-state authority and sole writer; dialog turns stay curated
  authored-text-only (D028 as narrowed by RFC 0092); local-first, no external
  service, no vendor SDK; stay inside `write_scope.allowed_paths`; never write
  `.striatum/`.

## Verification commands (reviewers run these)

```sh
PATH="$HOME/go/bin:$PATH" make -C go check     # vet + lint + race tests
make test                                       # full suite
make lint && make typecheck
# new fixtures validate:
striatum workflow validate examples/<shape>/workflow.json
# the anti-theater + ledger tests pass under live PG (RFC 0080 pgtest):
STRIATUM_PG_TEST_URL=postgres:///postgres?host=/var/run/postgresql go -C go test ./pkg/...
```

Land only what the design synthesis + panel converge on. Write the build
handoff with what landed, what was deferred, and the exact verification
commands you ran with their results.
