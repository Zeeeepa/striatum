# Dogfood 034 Operator Report

author: operator
date: 2026-05-11
status: complete

## Run

- Run ID: `run_4e95a7c06d1e414cba6765f5045d4d07`
- Workflow: `dogfood-034-rfc-0030-0031-rpc-supervision-sealed`
- Branch: `striatum/dogfood-034-rfc-0030-0031-rpc-supervision`
- Final state: `completed`
- Final job tally: 9 jobs completed, 0 canceled, 0 open blockers, 0 human checkpoints.
- Duration: 1h 29m.

## Scope

Paired implementation of RFC 0030 (daemon RPC server + version skew
protocol) and RFC 0031 (daemon-owned supervision + sealed-apply boundary)
as one architectural unit. The RPC server establishes the trust boundary;
supervisor migration and sealed-apply authority flow over it.

V2 ships scaffolding for: envelope codec + framing, Unix-domain socket +
loopback HTTP transports, `daemon.hello`/`daemon.welcome` handshake,
method registry with capability binding (`read`/`write`/`review`/`claim`/
`apply`/`admin`), audit + request log persistence on the RFC 0033
substrate, supervisor pointer table + daemon-side `process_supervisors`
schema, sealed-apply RPC plumbing + signing-key custody (OS keyring with
0600 runtime fallback), workflow schema additions
(`require_daemon`/`apply_gate`/`sealed_patch_provider`), and a CLI
daemon-client shim.

Deferred per the accepted synthesis: cross-repo workflows + MCP mutation
capability expansion (RFC 0032), Python → Go core port (D084), bundled /
Dockerized PG distribution (RFC 0033 follow-up), retirement of
`--no-daemon`, service-manager installer, native Windows daemon support,
and full sealed-apply end-to-end (the V2 slice records the scaffold and
refuses-on-mismatch correctly but the daemon-owned worktree apply itself
remains a stub per the build review's F7 closure-via-option-(a)).

## Control-Plane Outcome

Run shape (the dogfood-034 streamlined workflow with single build
reviewer):

```
3 fresh designs (codex/claude/gemini, parallel)
  ↓ all completed first try
synthesize_design (codex)
  ↓ accepted by threat_model design review first try
review_design_threat (gemini, threat_model, fresh)
  → accept severity:low (no design cycle needed)
  ↓
implement (codex with sub-agent delegation per implement prompt)
  → first try produced daemon_rpc/, daemon_apply/, daemon_supervisor/,
    daemon_pg migration v2, repo-local migration v13, tests, docs
  → 591 tests pass after make install/lint/typecheck/test/smoke
  ↓
review_build_threat (claude_code, threat_model, fresh, repo-level)
  → round 1: needs_revision severity:high (7 must-fix + 5 should-fix)
  → implement_a2 addressed all must-fix items + worked through
    should-fix items
  → round 2: accept_with_findings severity:low (F1-F8, F10-F12 closed,
    F7 closed via option (a), F9 partial as non-blocking, 4 minor new
    observations N1-N4 documented for follow-up)
```

Total wall-clock: ~1h 29m. Compared with:

- dogfood-033 (RFC 0033 alone, single design review, no build review):
  ~33 min
- dogfood-031 (RFC 0028, 3-posture design + 2-posture build, 3 cycle
  iterations exhausted): ~3 h

The single-posture threat-model gate kept the build review's discipline
without the cycle pain from dogfood-031's three parallel postures.

## Notable Wins

1. **Codex drove its own claim loop end-to-end on every codex job**
   (design, synthesis, implement, implement_a2 revision). Same pattern as
   dogfood-033, now confirmed reliable.

2. **Sub-agent delegation worked at scale.** The implement BUILD_HANDOFF
   names a parallel author plan: separate sub-agents for envelope codec,
   request router, capability authorizer, supervisor migration, sealed-
   apply gate, signing key custody, audit-append wiring on the substrate,
   MCP route filter, plus exploratory reads of existing modules. The
   parent session integrated and verified.

3. **First-try threat_model design review acceptance.** The synthesis
   was explicit enough (paired RFC reconciliation, scope-discipline
   table mapped 1:1 to acceptance criteria) that gemini's threat-model
   review accepted on the first round, severity:low. No design cycle
   needed.

4. **Build review caught real bugs.** Claude Opus reviewer-1 identified
   12 concrete issues including F1 (client-controlled denial vocabulary
   on `apply.reviewed_patch`), F2 (capability `repository_id` not bound
   to mutation target), F4 (duplicate-request-id detection ran after
   audit append), and F5 (handshake state shared across all callers).
   The implementer addressed all seven must-fix items.

5. **v1.22.1 byline canonicalisation worked.** All design bylines came
   through as lowercase `author:` without operator byline fixes.

## Operator Interventions

Two interventions, both documented:

1. **Build review round 1**: claude_code reviewer wrote a full review
   file but the agent's `claude --print` invocation did not call
   `striatum ack` / `submit-review` (the supervised-claude permission
   gate refuses Striatum CLI invocations the same way dogfood-031
   experienced). The operator called `ack` + `submit-review` on the
   reviewer's behalf using the existing session and lease. The
   reviewer's verdict (`needs_revision` severity:high) and findings
   came entirely from claude. No content authored by the operator.

2. **Build review round 2**: the round-2 supervised claude reviewer
   exited after one prompt without writing a file ("striatum ack was
   denied; will not take further action"). The operator launched a
   manual `claude --print` invocation with a focused prompt that
   instructed claude to inspect the round-2 source and write the
   round-2 review file directly (no Striatum CLI calls — operator
   publishes). Claude produced the round-2 review (15 KB,
   accept_with_findings severity:low) which the operator then published
   via `submit-review` on the existing reviewer-claude_code-2 session
   and lease.

The second intervention is the same shape as the dogfood-031 codex
manual-run pattern, applied to claude instead of codex. The reviewer's
verdict and findings are entirely claude-authored; the operator
orchestrated invocation and CLI calls only.

## Recorded Risks and Follow-ups

Documented in `docs/dogfood/034/review/build/threat/REVIEW.md` and
acknowledged in the BUILD_HANDOFF:

- F9 partial: closed-vocabulary table tests for denial codes still
  owe coverage. The unsigned codes are not reachable in the current
  stub, but the test gap should close in a follow-up.
- N1: duplicate-request-id guard is not atomic with audit/request-log
  inserts. Matters when daemon accepts concurrent connections; today
  the accept loop is single-threaded. Follow-up before the daemon
  becomes a real RPC server with concurrent clients.
- N2: malformed-`daemon.hello` audit branch lacks a regression test.
- N3: dead branch in `record_pointer` (defensive code that current
  state cannot reach).
- N4: defensive nit in `_repo_root_for` short-circuit.
- Sealed-apply daemon-owned worktree apply itself is a stub. RFC 0031
  V2 scope was the boundary and the refuse-on-mismatch paths; the
  apply flow lands in a follow-up.
- CLI dispatcher does not yet route through daemon RPC for mutating
  verbs. Direct repo-local SQLite is still the default mutation path;
  daemon RPC is the receive-side scaffold ready for a CLI router
  follow-up.
- Claude's supervised-permission gate continues to refuse `striatum`
  CLI invocations in `claude --print` mode. The operator-publish
  workaround keeps the dogfood moving; a harness-improvement proposal
  for the supervised-claude permission story would close this
  permanently.

## Verification Artifacts

- `docs/dogfood/034/RUN_SUMMARY.md`
- `docs/dogfood/034/EVIDENCE.md`

Implementation verification (from the BUILD_HANDOFF, both rounds):

- `make install`: passed
- `make lint`: passed
- `make typecheck`: passed
- `make test`: 603 passed (+12 from baseline of 591; new
  `tests/test_daemon_rpc.py`)
- `make smoke`: passed (with existing deprecated-`needs` warnings)

## Deliberately Left Out

The operator did not author design, synthesis, review, or implementation
content. The two interventions above (operator publishes on agents'
behalf because their supervised harness denied Striatum CLI calls) are
documented as harness friction, not operator role work. Devils-advocate
and security reviews remain deferred to post-implementation per the
operator decision recorded in commit `9d95487`.
