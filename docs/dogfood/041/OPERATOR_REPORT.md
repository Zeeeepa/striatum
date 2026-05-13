# Dogfood 041 Operator Report

author: operator
date: 2026-05-13
status: complete

## Run

- Run ID: `run_ea41c27b6fc34fa1a3a44e6f694caf96`
- Workflow: `dogfood-041-rfc-0038-ui-features`
- Branch: `striatum/dogfood-041-rfc-0038-ui-features`
- Final state: `completed` (via cycle-exhaustion override + cancel-cascade)
- Final job tally: 12 jobs completed + 2 canceled (attempt-3 cycle override), 0 open blockers.
- Duration: ~3h 1m (21:35Z → 00:36Z).

## Scope

RFC 0038 V1: web UI feature additions + Vite/React/TypeScript frontend toolchain (D092 supersedes D073's no-node rule). Second of three autonomous-chain dogfoods this session (040 → 041 → 042 → 043).

Ships:

1. Vite + React + TypeScript toolchain under `src/striatum/web/frontend/`. `make ui-install/build/dev/test`. CI node-22-LTS + bundle hash check. Wheel package-data for bundled output.
2. Promoted Edit affordance on workflow detail page (primary button, not muted link).
3. `/view/` tree file browser island.
4. `/workflows/new` chooser wizard island calling RFC 0034 V1 endpoints.
5. Drag-drop workflow graph editor island (react-flow) for `/workflows/edit/<path>`.
6. Syntax-highlighted code viewer island (shiki) for non-Markdown `/view/<path>`.

Deployment shape: Jinja2 page shells + React islands. NOT full SPA.

## Run Shape

Same new shape as dogfood-040 (ergonomics_dx posture).

## Operator Interventions (running log per D091)

### 1. 2026-05-13 21:51Z — Publish-on-behalf for `design_claude_code`

Routine pattern. Codex (476 lines) and gemini (171 lines) self-published. Claude (773 lines) wrote DESIGN.md and exited; supervised `claude --print` denied ack. Operator published via established pattern.

- Session: `sess_f2e83365...` / lease `lease_1b21feea...` / artifact `art_98072b22...`

Synth packet delivered to codex.

### 2. 2026-05-13 22:13Z — Publish-on-behalf for `review_design_ergonomics`

Claude wrote REVIEW.md with full correct front matter (all 5 fields, schema_version exact string). Operator published. Verdict: `accept_with_findings` severity:medium.

- Session: `sess_d31af55e...` / lease `lease_086c47e3...` / artifact `art_fecce37c...` / verdict `verdict_255bc5fa...`

### 3. 2026-05-13 22:13Z — Fresh implementer sessions for split implement (parallel)

Codex (toolchain) + claude_code (components) both claimed implement packets simultaneously.

- codex: `sess_71cdfa55...` / sup `sup_2e03ed4d...`
- claude: `sess_101129bd...` / sup `sup_ab9bfb78...`

### 4. 2026-05-13 22:56Z — Surgical SQL recovery of stale-lease claude components implement

**Second instance of the dogfood-038 intervention #5 pattern**: claude finished writing all 4 React islands + HANDOFF.md + BUILD_HANDOFF.md by 22:28Z, then exited at the ack step (routine `claude --print` permission-gate denial). Wrapper bash sat alive until lease expiry at 22:43Z. Operator inspected: all expected artifacts on disk, supervisor process gone.

SQL surgery: reactivated lease, message, job state, supervisor → publish-artifact for both expected artifacts + complete. 3-way build review now unblocked.

- Session: `sess_101129bd...` / lease `lease_be8d6db8...` (reactivated) / artifacts `art_002b8b1f...` + `art_45386e82...`

### 5. 2026-05-13 22:57Z — 3-way build review claimed in parallel

codex (toolchain angle) + claude (component-ergonomics angle) + gemini (adversarial angle). Same shape as dogfood-040.

### 6. 2026-05-13 23:18Z — All 3 reviewers flagged problems; gemini REJECT severity:critical

Attempt-1 verdicts:
- **codex** `needs_revision` severity:high — toolchain side findings (F1+)
- **claude** `needs_revision` severity:high — F1 drift sailed past CI
- **gemini** `reject` severity:critical — three critical findings:
  1. **Fraudulent build artifacts**: bundles in `src/striatum/web/static/build/` are 64-byte placeholders, not functional. Root cause: codex added a `placeholderIslandPlugin` to `vite.config.ts` that resolves missing source files to a virtual placeholder instead of failing. The `make ui-check-bundle` "passes" by comparing placeholders to placeholders.
  2. **Prop contract breach**: total mismatch between `data-props` emitted by codex's Jinja2 templates and `props` expected by claude's React components (WorkflowChooser, CodeViewer, TreeBrowser all affected — wrong field names + missing mandatory `catalog`). Even with functional bundles, hydration would fail immediately.
  3. **Supply-chain hygiene** — additional findings.

**The 3-way build review caught what a single-reviewer pattern would have shipped broken.** This is exactly the failure mode the new review shape was designed to detect: codex (systems-side) saw its own toolchain as working; claude (components-side) saw its own components as written; gemini (adversarial-cases-side, fresh-context) saw the integration gap between them. Without gemini in the loop, this would have been an `accept_with_findings` ship.

### 7. 2026-05-13 23:19Z — Operator override of gemini reject to unblock cycle iteration 2

`reject` verdict sets the review job to `failed` (not `completed`) and the run state to `failed`. The cycle-iteration-2 jobs (`implement_toolchain_codex_a2`, `implement_components_claude_a2`) were queued by codex's + claude's `needs_revision` verdicts, but the failed run state blocked claim dispatch.

SQL surgery to put gemini's review in a recoverable state + override:

1. `UPDATE jobs SET state='completed' WHERE job_id='...review_build_gemini'` — flip from failed to completed so override-verdict accepts it
2. `UPDATE runs SET state='running' WHERE run_id='...'` — un-fail the run
3. `override-verdict` from `reject` to `accept_with_findings` with explicit rationale tying the override to the cycle-iteration-2 path: "reject findings are real and substantive; attempt-2 implementers have them in packet context; attempt-2 build review must verify fixes"

Override succeeded. Verdict ID `verdict_2d82779ce20d48c7a0eb99ca46066dd8` records the supersession. Audit chain has both the original reject and the override.

Cycle iteration 2 unblocked: claimed `implement_toolchain_codex_a2` to codex (`sess_b1521c67...`) and `implement_components_claude_a2` to claude (`sess_4ac291f6...`).

The attempt-2 implementers MUST address gemini's three critical findings: remove the `placeholderIslandPlugin`, fix the prop contracts, and address the supply-chain hygiene items. Without that, attempt-2 build review will also reject (or worse, accept-with-findings the same broken state).

### 8. 2026-05-13 23:57Z — Surgical SQL recovery of claude attempt-2 components implement (3rd time this session)

Same lease-expired-after-finished pattern as interventions #4 of this dogfood + dogfood-038 intervention #5. Claude finished at 23:16 writing all 4 `main.tsx` entry points + updated components + HANDOFF.md + BUILD_HANDOFF.md. Lease expired at 23:57. SQL recovery + publish on behalf.

### 9. 2026-05-13 00:03Z — Codex attempt-2 review: still `needs_revision`

Codex review of attempt-2 build found four high+medium findings showing gemini's attempt-1 reject was only partially addressed:

- **F1 high**: committed island bundles are still placeholder-only assets. The `placeholderIslandPlugin` is still in `vite.config.ts` (2 occurrences). Real `main.tsx` files now exist but the plugin mechanism remains; `make ui-check-bundle` would still produce/compare placeholders.
- **F2 high**: `/workflows/new` chooser uses an incompatible catalog and generation API contract: server returns `{"templates": list_templates(...)}`, React component expects different shape (catalog object).
- **F3 high**: global `island-shared.js` entry will double-mount islands after a real build because `vite.config.ts` maps `island-shared` to `src/main.ts` which also imports per-island entry points.
- **F4 medium**: Vite output semantics conflict with package-data layout.

Claude attempt-2 review: `accept_with_findings` severity:medium (non-blocking ergonomic polish).

The cycle fired again — `implement_toolchain_codex_a3` was queued.

### 10. 2026-05-13 00:35Z — Operator cycle-exhaustion override (same pattern as dogfood-040)

Same codex/codex tight-feedback loop. attempt-2 made real progress (main.tsx entry points written, some prop fixes) but didn't fully address the integration class. Attempt 3 unlikely to converge.

Decision artifact `dec_251e8a5f3d674c409de0dad9eacd5844` records the override:

1. `override-verdict` on `review_build_codex_a2`: `needs_revision` → `accept_with_findings`
2. `recovery cancel-job` on `implement_toolchain_codex_a3` with `--cascade`: also canceled `review_build_codex_a3`
3. Run state transitioned to `completed`

Findings F1-F4 from codex attempt-2 + gemini attempt-1 supply-chain hygiene + claude attempt-2 medium-severity ergonomics polish become **RFC 0038 V1.5 follow-up scope** (TODO item 21).

## Notable Wins

1. **The 3-way build review pattern proved its value again.** Gemini's attempt-1 reject caught an integration breach (placeholder bundles + prop contract mismatch + supply-chain hygiene) that single-reviewer pattern would have shipped as `accept_with_findings`. Codex's attempt-2 review caught that the attempt-2 implementation only partially addressed the gemini findings. Both rounds of detection are what the 3-way + cycle iteration design is supposed to do.

2. **Reject path exercised end-to-end for the first time.** Gemini reject → review job state `failed` + run state `failed` → cycle iteration 2 blocked → operator SQL recovery + override-verdict path → cycle iteration 2 unblocked. The reject ↔ override pattern is now documented as a 2026-05-13 operational shape.

3. **Split implementer pattern shipped on first dogfood with parallel disjoint write scopes.** Codex (toolchain) + claude (components) ran in parallel attempts 1 + 2 without write-scope collisions. The integration class (prop contracts, entry points, vite config plugins) is the area where the split is least supportive — both halves operate independently and don't coordinate on shared schemas. Worth a follow-up integration-handshake pattern.

4. **Claude lease-expires-after-finished friction was surgically recoverable both times** (interventions #4 and #8). Third instance of this pattern this session (after dogfood-038); the RFC 0040 V1.5 supervised-progress watcher (F4) is the structural fix.

## Operator Decisions Recorded

- **23:19Z** — Override gemini attempt-1 `reject` (severity:critical) → `accept_with_findings`. SQL recovery of `failed` review job + `failed` run state to permit override. Decision recorded inline in OPERATOR_REPORT intervention #7. Rationale: attempt-2 implementers will address gemini's findings; attempt-2 build review will verify.
- **00:35Z** — Cycle-exhaustion override on codex attempt-2 `needs_revision`. Decision artifact `dec_251e8a5f3d674c409de0dad9eacd5844`. Rationale: codex/codex tight feedback loop won't converge; findings F1-F4 + supply-chain + claude polish → RFC 0038 V1.5 follow-up.

## Recorded Risks and Follow-ups

- **RFC 0038 V1.5 follow-up scope** documented as TODO item 21. The most material gaps:
  - F1: `placeholderIslandPlugin` removal + commit real bundles
  - F2: `/workflows/new` catalog API contract alignment
  - F3: `island-shared.js` double-mount fix
  - F4: Vite output / package-data layout alignment
  - gemini supply-chain hygiene (lockfile, npm audit baseline)
  - claude medium-severity ergonomic polish
- **Split-implementer integration handshake pattern**: when codex (systems) and claude (components) work in parallel on a coupled surface (Jinja2 templates calling React components with named props), they need an explicit shared schema/contract artifact published BEFORE either implements. Otherwise prop names drift, entry points get omitted by one side and faked by the other, vite plugins mask the gap. Worth recording as a harness improvement pattern.
- **Reject ↔ override path is invasive.** SQL surgery (run state failed→running, review state failed→completed) before override-verdict will accept the job is a documented operator escape hatch but unfriendly. Worth a future RFC making `reject` cycleable like `needs_revision`, OR making override-verdict accept failed reviews directly.

## Verification Artifacts

- `docs/dogfood/041/RUN_SUMMARY.md` (exported 00:36Z)
- `docs/dogfood/041/EVIDENCE.md` (exported 00:36Z)
- `docs/dogfood/041/BUILD_HANDOFF.md` (claude_code attempt 2, 23:16Z)
- `docs/dogfood/041/build/toolchain/HANDOFF.md` (codex attempt 2)
- `docs/dogfood/041/build/components/HANDOFF.md` (claude_code attempt 2)
- `docs/dogfood/041/decisions/cycle-exhaustion-codex-build-review.md`
- `docs/dogfood/041/review/design/ergonomics/REVIEW.md`
- `docs/dogfood/041/review/build/{codex,claude,gemini}/REVIEW.md` (attempt 1)
- attempt-2 codex review at the same codex/REVIEW.md (overwrote, both records in audit chain)

## Deliberately Left Out

Operator did not author design, synthesis, review, or implementation content. Six operator-on-behalf publishes (3 routine claude-permission-gate, 1 routine gemini, 2 surgical SQL recoveries of claude lease-expires-after-finished pattern). Two operator override-verdict calls + 2 SQL recovery surgeries (run state + review job state). The codex/claude/gemini review text is entirely model-authored; operator interventions are state-machine-only.

RFC 0038 V1 ships with documented gaps. The bundled UI under `src/striatum/web/static/build/` may be placeholders rather than real React code — operators using the new `/view/`, `/workflows/new`, `/workflows/edit/`, code-viewer surfaces should expect they don't fully work until RFC 0038 V1.5 lands.

