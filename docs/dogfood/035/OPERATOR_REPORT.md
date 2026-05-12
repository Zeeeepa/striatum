# Dogfood 035 Operator Report

author: operator
date: 2026-05-12
status: complete

## Run

- Run ID: `run_b37634824ee24c8995358cbdcfb11263`
- Workflow: `dogfood-035-rfc-0032-cross-repo-and-mcp-mutation`
- Branch: `striatum/dogfood-035-rfc-0032-cross-repo-mcp-mutation`
- Final state: `completed`
- Final job tally: 7 jobs completed, 0 canceled, 0 open blockers, 0 human checkpoints.
- Duration: 1h 6m 32s.

## Scope

RFC 0032 V2 slice: cross-repository workflow schema validation, daemon-
mediated cross-repo run lifecycle (mock-friendly helpers in
`src/striatum/cross_repo.py`), MCP mutation capability vocabulary expansion
with default-deny gating + per-token `tools/list` filtering, audit row
appended for every mutating MCP `tools/call` including denials, repo-local
migration v14 for `runs.cross_repo_run_id`, and daemon Postgres migration v3
for the `cross_repo_runs` table. Last dogfood of the daemon V2 follow-up
quartet on top of dogfood-031 / 033 / 034.

Deferred per the scaffold's explicit deferral built into every prompt:
multi-repo / cross-repo END-TO-END integration testing against a real
two-repo daemon harness. The dogfood-035 implementation shipped unit-level +
mock-based + schema/validator/write-scope/capability-gate tests; the
harness-level cross-repo integration tests will land after the follow-up
RFC queued as `docs/TODO.md` Open item 19 (multi-repo test harness).

## Control-Plane Outcome

Streamlined workflow shape (same as dogfood-034 with explicit deferral
documented in every prompt):

```
3 fresh designs (codex / claude / gemini, parallel)
  ↓ all completed first try
synthesize_design (codex)
  ↓ accepted by threat_model design review first try
review_design_threat (gemini, threat_model, fresh)
  → accept severity:low first try
  ↓
implement (codex with sub-agent delegation)
  → 617 tests pass after one self-corrected MCP error-code regression
  ↓
review_build_threat (claude_code, threat_model, fresh, repo-level)
  → accept_with_findings severity:medium first try
```

Total wall-clock: 1h 6m 32s. Compared with:

- dogfood-031 (RFC 0028 V1, 3-posture gates, 3 cycle exhaustions): ~3h
- dogfood-033 (RFC 0033, no build review): ~33 min
- dogfood-034 (RFC 0030+0031, 1 build cycle): ~1h 29m
- dogfood-035 (RFC 0032, 0 cycles needed): ~1h 6m

The deferral-in-prompt approach worked: the build reviewer accepted on the
first try (severity:medium with non-blocking follow-up findings)
rather than refusing for absent multi-repo E2E coverage.

## Notable Wins

1. **Codex drove its own claim loop end-to-end on every codex job**
   (design, synthesis, implement). Consistent with dogfood-033/034.

2. **Sub-agent delegation continues to scale.** Codex modified 30+
   files across `src/striatum/workflow.py`, `src/striatum/cross_repo.py`,
   `src/striatum/daemon_rpc/capability.py`, `src/striatum/daemon_rpc/registry.py`,
   `src/striatum/migrations.py`, `src/striatum/schema.py`,
   `src/striatum/daemon_pg/migrations.py`, plus daemon PG migration v3
   SQL, two new test files (`test_cross_repo_lifecycle.py`,
   `test_mcp_mutation_capabilities.py`, `test_daemon_rpc_registry.py`,
   `test_workflow_cross_repo.py`), and all the doc surfaces.

3. **One test failure caught, self-corrected.** Codex's first
   `make test` run failed
   `test_daemon_mcp_is_resources_only_and_excludes_audit` because the
   MCP `tools/call` error code changed from -32601 (method not found)
   to -32602 (invalid params). Codex fixed and re-ran without operator
   intervention.

4. **Deferral honored throughout.** The build reviewer (claude_code,
   threat_model) explicitly noted the multi-repo E2E coverage gap and
   marked it as deferred-to-follow-up rather than blocking. The
   prompt's "do not refuse for absent harness-level cross-repo tests"
   instruction worked.

5. **v1.22.1 byline canonicalisation worked again.** Gemini's
   `**Author:**` design byline was accepted by the publisher without
   operator intervention.

## Operator Interventions

Two routine mechanical operator publishes for the supervised
`claude --print` permission-gate friction that has been the consistent
shape since dogfood-031:

1. **Design review round 1**: claude_code design (designer-claude-opus-001)
   wrote the design file but the supervised harness denied the `striatum
   ack` call. The operator called `ack` + `publish-artifact` + `complete`
   on the existing session and lease. The design content is entirely
   claude-authored.

2. **Design review round 1**: gemini (designer-gemini-pro-001) same
   pattern. Operator published on the existing session and lease.

The build reviewer (claude_code reviewer-claude-opus-001) drove its own
claim loop end-to-end this time (different from dogfood-034's round-1
build reviewer). The threat-model build review came back
`accept_with_findings` on the first try without operator intervention
for the review itself.

## Recorded Risks and Follow-ups

Documented in `docs/dogfood/035/review/build/threat/REVIEW.md` and
acknowledged in the BUILD_HANDOFF:

- Multi-repo / cross-repo END-TO-END integration tests against a real
  two-repo daemon harness remain deferred to the follow-up RFC
  (`docs/TODO.md` Open item 19). The dogfood shipped unit + mock-based
  coverage; harness-level tests land after the harness RFC is written
  and implemented.
- Several should-fix items the build reviewer named as non-blocking
  follow-ups (recorded in the review's specific findings section).
  Land opportunistically in normal bugfix iterations.

## Verification Artifacts

- `docs/dogfood/035/RUN_SUMMARY.md`
- `docs/dogfood/035/EVIDENCE.md`

Implementation verification (from BUILD_HANDOFF):

- `make install`: passed
- `make lint`: passed
- `make typecheck`: passed (123 source files)
- `make test`: 617 passed (+14 from baseline of 603; new
  `test_cross_repo_lifecycle.py`, `test_mcp_mutation_capabilities.py`,
  `test_daemon_rpc_registry.py`, `test_workflow_cross_repo.py`)
- `make smoke`: passed

## Deliberately Left Out

The operator did not author design, synthesis, review, or implementation
content. The two design-phase publishes (claude + gemini) are routine
operator-on-behalf calls because the supervised `claude --print` and
gemini wrappers refuse to call `striatum ack` themselves; the design
content is entirely model-authored. Devil's-advocate and security
reviews remain deferred to post-implementation per the operator decision
recorded in commit `9d95487`. Multi-repo / cross-repo END-TO-END
integration testing remains deferred to the follow-up RFC queued as
`docs/TODO.md` Open item 19.
