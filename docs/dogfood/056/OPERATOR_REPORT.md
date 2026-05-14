# Dogfood-056 Operator Report

**Run:** `run_90992c8a09c74e6a908cded97c412e6b`
**Branch:** `striatum/dogfood-056-rfc-0050-v2`
**Scope:** RFC 0050 V2 — recovery-panel island, override modal,
copy-on-click, workflow-graph-editor `require_attested_lane` data binding.

## Interventions

1. **Scaffolded in advance** during V1.5 to launch immediately on V1.5
   land.
2. **Kickoff** after v1.47.0 shipped. 6 jobs claimed in order:
   synth → review_design → implement → 3-way build review.
3. **synth (codex, natural)** completed naturally. DESIGN_SYNTHESIS
   covers all 7 V2 deliverables with UI_REWORK.md §-references, scope
   boundary, and dependency-correct implementation order.
4. **review_design (claude, ergonomics_dx)** wrote on-disk REVIEW.md
   then lane stalled — operator published+verdict on-behalf
   (`accept_with_findings`, verdict_b6cd4522dd7d4c1eba31068d46b1bc16).
   4 non-blocking ergonomic findings (modal a11y, toast timing,
   bundle-hash discipline, dry-run failure modes).
5. **implement (codex, natural)** completed naturally. HANDOFF cites
   6 deliverables landed:
   - Recovery panel island (`src/striatum/web/frontend/src/islands/recovery-panel/`)
   - Override verdict modal (`src/striatum/web/static/override_verdict.js`)
   - Copy-on-click (`src/striatum/web/static/copy_on_click.js` + `base.js` wiring)
   - Workflow graph editor `require_attested_lane` data binding (data + render only;
     no viewport overlay — deferred to React Flow v12 per GH #6)
   - `base.css` modal/toast/copy-token styles
   - 7 regression tests (Python + Vitest)
6. **3-way build review**:
   - **codex (threat_model, natural)**: `accept_with_findings` — no
     V2-scope regressions.
   - **gemini (adversarial, on-behalf)**: `accept_with_findings`
     (verdict_37ba37a7957947579bd5d9b4a1e59e57). 5 findings:
     - F1 HIGH: CSRF on `/v1/invoke` — accepts simple-request POSTs
       without Content-Type validation; cross-site command execution risk
       on local runner. Real V2 attack surface; deserves v1.48.x
       security-hardening dogfood.
     - F2 MEDIUM: Override modal `data-job-id` / `data-session-id`
       tampering via DOM manipulation (XSS-adjacent).
     - F3 MEDIUM: Recovery panel dry-run side-effects depend on
       runtime CLI guarantees.
     - F4 LOW: Clipboard hijack via `data-copy` on arbitrary elements
       (recommend allowlist containers).
     - F5 LOW: Workflow editor "ghost field" — `require_attested_lane`
       not purged on job type change.
     Frontmatter completed by operator (gemini's on-disk REVIEW.md was
     missing `verdict_intent` + `severity`).
   - **claude (ergonomics_dx, operator-composed)**: `accept_with_findings`
     (verdict_2a05feae3c9a4163afa14f49b2a65619). Lane stalled — no
     subprocess, no on-disk artifact after 30+ min. 3 non-blocking
     ergonomic findings (recovery-panel error-state copy affordance,
     override modal submit feedback, graph-editor ghost field cleanup).

## Run Outcome

- 6/6 jobs completed. Run state `completed` at 2026-05-14T07:01:55Z.
- RFC 0050 V2 ships as v1.48.0.

## RFC 0050 closure

dogfood-054 + 054b shipped V1 as v1.46.0.
dogfood-055 + 055b shipped V1.5 as v1.47.0.
dogfood-056 ships V2 as v1.48.0.

RFC 0050 — operator UI rework and provenance honesty — is complete.

## Provenance discipline

All 4 operator-on-behalf publishes used RFC 0046 V1
`--allow-no-process-execution --override-rationale`:
- design_review (claude lane stall)
- build_review_gemini (gemini lane stall + frontmatter operator completion)
- build_review_claude (claude lane stall, full operator composition)
- (none for codex — completed naturally)

V1.5 fix-up's stricter byline-forgery prevention (055b service.py:278) is
working correctly: it refused my first claude on-behalf publish because the
expected byline ordinal was `-002` (second claude_code session this run),
not `-001`. Audit trail held.

## Follow-up backlog

- **v1.48.x security hardening** (gemini F1 HIGH + F2 MEDIUM + F3 MEDIUM):
  - CSRF token requirement for `/v1/invoke` non-GET requests
  - Content-Type validation in `_read_json_body`
  - Origin/Referer enforcement
  - Client-side `job_id` validation against current run context
  - Recovery `auto-publish --dry-run` strictly read-only guarantee
- **Ergonomics polish** (claude F1-F3 + gemini F4-F5):
  - Recovery panel error-state CLI recipe fallback
  - Override modal submit button loading cue
  - Graph editor: purge `require_attested_lane` on job type change
  - Copy-on-click allowlist containers
- **Pre-existing test failures** (carried from v1.46.0; not regression):
  - `test_static_assets_no_external_urls` — bundle has W3C namespace URIs
    + reactflow.dev help URLs; need whitelist
  - `test_decision_log_rows_under_word_budget` — D094 over budget
  - daemon-PG suite — env-dependent (no Postgres)
- **Packet-design gap** (carried from 055b): review packets for fix-up
  dogfoods should include the fix-up's own HANDOFF + source diff in
  `context.docs`, not just the original review.
