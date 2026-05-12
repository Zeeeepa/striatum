# Dogfood 040 Operator Report

author: operator
date: 2026-05-13
status: complete

## Run

- Run ID: `run_907a9b013113416ba66aa818f2f5d0d1`
- Workflow: `dogfood-040-rfc-0040-mcp-driven-harness`
- Branch: `striatum/dogfood-040-rfc-0040-mcp-driven-harness`
- Final state: `completed`
- Final job tally: 9 jobs completed + 2 canceled (attempt-3 cycle override), 0 open blockers, 0 human checkpoints.
- Duration: ~6h 18m (run prepare ~15:03Z → run.completed 21:21:36Z, with most time spent in 3-way build review + cycle iteration).

## Scope

RFC 0040 V1: MCP-driven dogfood harness for operator sessions. First of three autonomous-run dogfoods this session (040 / 041 / 042 = RFC 0040 / 0038 / 0039 in run order). Pre-flight verified: node v24.15, npm 11.13, Go 1.23.4 (installed from upstream), psycopg 3.3.4, system PG socket-reachable.

Ships:

1. MCP chat-tool exposure of dogfood-lifecycle RPC verbs (12 verbs).
2. `dogfood.publish_on_behalf` composite tool.
3. `dogfood.surgical_recovery` composite tool with new admin-only `surgical_recovery` capability.
4. Daemon-side supervised-progress watcher (file-mtime-based, addresses dogfood-038 friction).
5. Per-model harness-profile fragments baked into RFC 0034 V1 generator catalog.
6. `striatum workflow upgrade <path>` CLI verb.
7. Documentation: new `docs/HARNESS_FRICTION_PATTERNS.md` + MCP + HOW_TO_HUMAN + HOW_TO_AGENT updates.

## Run Shape (NEW)

First dogfood using the new workflow shape:

- Design phase: 3 lanes (codex / claude / gemini) parallel fresh design
- Synth: codex
- Design review: **claude_code** (switched from gemini)
- Implement: **SPLIT into 2 parallel jobs** with disjoint write scopes:
  - `implement_systems_codex` (codex): daemon-side
  - `implement_ergonomics_claude` (claude_code): chat tools + harness fragments + workflow upgrade + docs
- Build review: **3-WAY PARALLEL** (codex + claude + gemini, fresh, repo-level, threat_model)

Gemini is reserved for design creator + build reviewer only — never implementer.

Anti-friction notes baked into all three harness profiles (every observed pattern from dogfoods 036/037/038/039):

- "No follow-up questions" instruction (claude/codex/gemini)
- "Front-matter completeness — all five fields required, none optional" instruction (gemini)
- "Focused pytest before make test to avoid lease expiry" instruction (codex)

## Operator Interventions (running log per D091)

### 1. 2026-05-13 15:19Z — Publish-on-behalf for `design_claude_code` + `design_gemini`

Routine pattern. Both wrote DESIGN.md with valid bylines; both exited without calling ack. Operator published via the established pattern.

- claude: `sess_81e827e3...` / `lease_4a8be548...` / `art_6d1a6455...`
- gemini: `sess_49a055b7...` / `lease_5342cf59...` / `art_3aba4168...`

### 2. 2026-05-13 19:39Z — Publish-on-behalf for `review_design_threat`

First time claude_code performed design review under the new workflow shape (was gemini in dogfoods 035-039). Claude wrote REVIEW.md with FULL correct front matter (all 5 required fields, valid severity, JSON array tags, byline after the block). Anti-friction notes from RFC 0040 design landed perfectly. Verdict: `accept_with_findings` severity:medium.

- Session: `sess_b0d0fcef320d40d4b558396aa3c084ca` (reviewer-claude_code-1, fresh)
- Lease: `lease_f20fb934802e4f12afdd7b7317edcba8`
- Artifact: `art_a2a7d40ace9043a198a98bd4532c4e02`
- Verdict ID: `verdict_268afe531ada48c59f6cb54c4279c4fd`

### 3. 2026-05-13 19:40Z — Fresh implementer sessions for the split implement phase (PARALLEL)

NEW PATTERN: both implementer lanes registered + claimed in parallel for the first time. Codex owns systems half (daemon + composites + capability), claude_code owns ergonomics half (chat tools + harness fragments + workflow upgrade + docs). Disjoint write scopes enforced by workflow validator.

- Codex: `sess_43fe1250...` / sup `sup_d3b7a97f...`
- Claude: `sess_6793a624...` / sup `sup_43ba2e93...`

### 4. 2026-05-13 20:09Z — Publish-on-behalf for `implement_ergonomics_claude` (attempt 1)

Claude wrote BUILD_HANDOFF.md + ergonomics/HANDOFF.md at 19:57Z; supervised `claude --print` denied ack. Operator published both expected artifacts (per workflow.json's two-artifact-per-job declaration) + completed. Routine pattern.

- Session: `sess_6793a624...`
- Lease: `lease_23878151...`
- Artifacts: `art_0028b74a...` (ergonomics_handoff), `art_809b9b42...` (build_handoff)

Codex's `implement_systems_codex` drove its own claim/ack/publish/complete loop (18 min, no operator intervention). Focused-pytest anti-friction note continues to work.

### 5. 2026-05-13 20:30Z — 3-way build review (FIRST RUN OF NEW PATTERN)

All three reviewers ran in parallel after both implement halves landed. Outcomes:

- **codex** (systems angle): `needs_revision` severity:high. Six findings (F1-F6, four high + two medium). The serious ones: (F1) daemon MCP `tools/call` authorizes + audits but never dispatches the method → composite tools are non-functional with misleading audit success; (F2) `surgical_recovery` in Python registry but missing from daemon-RPC registry; (F4) supervised-progress watcher not wired into daemon. Codex's review correctly caught real implementation gaps the other two reviewers did not surface.
- **claude** (ergonomics angle): `accept_with_findings` severity:medium. Non-blocking findings on docs polish + workflow-upgrade verb edge cases. Operator published on behalf (routine permission-gate).
- **gemini** (adversarial angle): `accept` severity:low. Gemini wrote `schema_version: "1"` in front matter (invalid — must be `"striatum.finding.v1"`). Operator hand-corrected the front-matter value and republished. Verdict reasoning is entirely gemini-authored; only the schema_version string was fixed.

**This is the first time the 3-way build review pattern caught what single-reviewer would have missed.** Codex's needs_revision verdict fired the cycle: `implement_systems_codex_a2` is queued for attempt 2. claude + gemini's attempt-1 verdicts stand (they accepted; cycle path is codex-only). When attempt 2 lands, only `review_build_codex_a2` re-runs.

- codex review: `art_b7...` (will get from event log) / `verdict_8a64...` / `summary=needs_revision`
- claude review: `art_7eb4a43b58554e1badb0f3bf723d6f07` / `verdict_e0c0707c22bc4376b22e7deaede3a35a`
- gemini review: `art_783db8c1c4be4493901db89ca9505811` / `verdict_1e0e12a970b441d1aa892440eae93ff4` (front-matter shape fix)

### 6. 2026-05-13 20:31Z — Fresh implementer-codex-2 for attempt 2 of `implement_systems_codex`

Codex's needs_revision verdict fired the cycle. Registered fresh `implementer-codex-2` session, claimed `implement_systems_codex_a2` packet. Codex will address the 6 findings (notably F1 MCP dispatch wiring, F2 surgical_recovery in daemon registry, F4 watcher wiring into daemon).

- Session: `sess_fb401c3c...`
- Supervisor: `sup_82931d8e...`

### 7. 2026-05-13 21:07Z — Codex re-review attempt 2 still `needs_revision`

Codex re-reviewed `implement_systems_codex_a2` and again returned `needs_revision` with similar-shaped findings (F1 daemon MCP dispatch wiring still not landing; F4 supervised-progress watcher still not wired into daemon; F2/F3 publish_on_behalf composite atomicity + verdict-recording semantics). The same-model-evaluating-itself loop (codex implementer ↔ codex reviewer) produces tight feedback that may not converge in finite attempts.

### 8. 2026-05-13 21:21Z — Operator decision: cycle exhaustion override

Per the dogfood-031 pattern + the workflow.json max_iterations=2 ceiling + the user's autonomous-run posture ("don't block at acceptance gates; record decisions and continue"): operator recorded a decision artifact (`docs/dogfood/040/decisions/cycle-exhaustion-codex-build-review.md`, `dec_af557de1402d44489c0b9af7c93b0a5c`) overriding codex's `needs_revision` verdict at iteration 2.

- `override-verdict` on `review_build_codex_a2`: `accept_with_findings` with the codex review artifact (`art_15aaad55...`) as the findings reference. Previous verdict `needs_revision` recorded as overridden.
- `recovery cancel-job` on `implement_systems_codex_a3` with `--cascade`: also canceled `review_build_codex_a3`. Cascade-canceled count: 1.
- Run state transitioned to `completed`.

The six findings (F1-F6) are documented and become RFC 0040 V1.5 follow-up scope.

## Notable Wins

1. **3-way build review pattern proved its value.** Codex caught implementation gaps (F1 dispatch wiring, F4 watcher wiring) that claude (ergonomics-angle) and gemini (adversarial-angle) did not surface. The single-reviewer pattern of dogfoods 035-039 would have accepted with findings and the gaps would have slipped to production.

2. **Split implement pattern worked cleanly.** Codex systems (18 min, drove own loop) and claude_code ergonomics (operator publish-on-behalf at 28 min for routine ack-denied) shipped in parallel with disjoint write scopes. Workflow validator caught one initial write-scope conflict at scaffolding time (build/systems/HANDOFF.md outside scope), corrected in 30 seconds before launch.

3. **Cycle exhaustion override path is now exercised end-to-end.** First time the autonomous-run posture has invoked decision-record + override-verdict + cancel-job-cascade as a unit. Pattern is now documented for dogfoods 041-043.

4. **Claude as design reviewer worked perfectly.** Anti-friction notes from RFC 0040 design landed — claude wrote REVIEW.md with full correct front matter (all 5 fields), valid severity, JSON array tags, byline after the block. No operator front-matter shape fix needed.

5. **Gemini's adversarial-angle build review accepted with one minor schema_version typo.** Anti-friction notes mostly landed; gemini wrote `schema_version: "1"` instead of `"striatum.finding.v1"`. Operator hand-corrected. Verdict reasoning is entirely gemini-authored.

## Operator Decisions Recorded

- 2026-05-13 21:21Z — Cycle exhaustion override on codex build-review needs_revision at iteration 2. Decision artifact `dec_af557de1402d44489c0b9af7c93b0a5c`. The six findings (F1 daemon MCP dispatch, F2/F3 publish_on_behalf atomicity, F4 watcher wiring, F5 watcher race + signal hardening, F6 test coverage end-to-end vs mocked) become RFC 0040 V1.5 follow-up scope.

## Recorded Risks and Follow-ups

- **RFC 0040 V1.5 follow-up** required for the six codex findings. The most material gaps:
  - F1: daemon MCP `tools/call` authorizes + audits but doesn't actually dispatch through the method registry → composite tools `dogfood.publish_on_behalf` + `dogfood.surgical_recovery` are non-functional through the MCP path (Python-side dispatch works; only the daemon-MCP route is broken).
  - F4: supervised-progress watcher module exists but is not invoked by the daemon supervisor lifecycle → leases still expire under active load until V1.5.
  - F2/F3: composite-tool atomicity gaps (single audit row records success even if a composed step fails partway).
  - F5: watcher race + signal hardening incomplete.
  - F6: tests cover mocked gating only, not end-to-end execution paths.
- Pattern observed: codex implementer ↔ codex reviewer loop produces tight feedback that may not converge. Worth a future harness-improvement note: when the implementer and the build reviewer share the same lane / same model, the cycle should ceiling at iteration 1 instead of iteration 2 to avoid this pattern. Alternatively, the implementer and the build reviewer for the same scope must be different lanes (e.g., implement_systems_codex's reviewer should be claude or gemini, not codex).

## Verification Artifacts

- `docs/dogfood/040/RUN_SUMMARY.md` (exported 21:22Z)
- `docs/dogfood/040/EVIDENCE.md` (exported 21:22Z)
- `docs/dogfood/040/BUILD_HANDOFF.md` (claude_code at 19:57Z, attempt 1)
- `docs/dogfood/040/build/systems/HANDOFF.md` (codex attempt 2 at 20:47Z)
- `docs/dogfood/040/build/ergonomics/HANDOFF.md` (claude_code at 19:57Z, attempt 1)
- `docs/dogfood/040/decisions/cycle-exhaustion-codex-build-review.md`
- `docs/dogfood/040/review/design/threat/REVIEW.md` (claude design review)
- `docs/dogfood/040/review/build/codex/REVIEW.md` (iteration 2 needs_revision, overridden)
- `docs/dogfood/040/review/build/claude/REVIEW.md` (accept_with_findings)
- `docs/dogfood/040/review/build/gemini/REVIEW.md` (accept; schema_version corrected by operator)

Implementation verification (from BUILD_HANDOFF):

- `make install/lint/typecheck`: passed
- `make test`: passed
- `make smoke`: passed

## Deliberately Left Out

The operator did not author design, synthesis, review, or implementation content. The five publish-on-behalf calls are routine operator-on-behalf invocations because supervised wrappers can't call `striatum ack`. Codex's review findings text is entirely codex-authored. Claude's review text is entirely claude-authored. Gemini's review text is entirely gemini-authored; operator only corrected the `schema_version` string. The cycle-override decision is operator-authored; this is the documented autonomous-run pattern per the prior dogfoods' OPERATOR_REPORTs.

The six codex findings (F1-F6) become RFC 0040 V1.5 follow-up scope; they are NOT addressed in v1.29.0.


## Deliberately Left Out

The operator does not author design, synthesis, review, or implementation content. Any operator-on-behalf publishes will be documented above.
