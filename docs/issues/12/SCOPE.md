---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/12/SPEC.md", "docs/issues/13/SPEC.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/SPEC.md", "AGENTS.md"]
---

author: triager-unknown-model-001

# GH #12-#13 Polish Scope

This workflow covers exactly the RFC 0050 ergonomics polish bundle for:

- GH #12 -- LOW: copy-on-click works on any data-copy element -- clipboard poisoning surface.
- GH #13 -- LOW: workflow editor -- require_attested_lane not purged when job type changes.

The bundle is intentionally narrow. `docs/ROADMAP.md` groups these two low-severity findings, while `docs/issues/12/prompts/fix.md` explicitly limits implementation to the #12 allowlist and #13 stale-field purge. Do not pull in GH #9-#11 security hardening or unrelated dogfood-056 ergonomic refinements.

## Files In Scope

Source files:

- `src/striatum/web/static/copy_on_click.js` -- restrict activation and enhancement to an explicit allowed context for legitimate copy controls.
- `src/striatum/web/static/base.css` -- update copy affordance selectors if needed so styling follows the same allowlist boundary.
- `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx` -- purge review-only fields, especially `require_attested_lane`, when a job changes away from `type: "review"`; keep review jobs able to preserve and render the field.
- `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts` -- add or update Vitest coverage for the stale-field purge.

Test files:

- `tests/test_copy_on_click.py` -- update the static asset contract so the global `[data-copy]` selector is no longer the accepted behavior and the allowlist is asserted.
- Any focused existing web/UI test under `tests/` or `src/striatum/web/frontend/src/__tests__/` needed to prove the two issue requirements.

Generated frontend bundle files:

- `src/striatum/web/static/build/` and `src/striatum/web/static/build/manifest.sha256` are in scope only if `make ui-build` changes committed bundle output after the TypeScript source change.

Workflow artifacts:

- `docs/issues/12/build/HANDOFF.md` -- implementer handoff.
- `docs/issues/12/review/` -- downstream review artifacts produced by this workflow.

## Files Out Of Scope

- `src/striatum/service.py`, `src/striatum/recovery*`, and `/v1/invoke` request validation -- GH #9-#11 security-hardening work belongs to the active runway, not this bundle.
- `src/striatum/web/static/override_verdict.js` -- out of scope unless a test-only import or fixture update is mechanically required by the two scoped fixes.
- Recovery panel behavior, dry-run semantics, override modal submit feedback, and unrelated dogfood-056 polish notes -- out of scope unless they are directly necessary to constrain `copy_on_click.js` to known safe copy controls.
- `docs/ROADMAP.md`, `docs/TODO.md`, `docs/DECISION_LOG.md`, `docs/SPEC.md`, and `AGENTS.md` -- read-only context for this workflow.
- `docs/issues/9/`, `docs/issues/10/`, `docs/issues/11/`, `docs/issues/13/`, and `docs/dogfood/` -- historical or parallel issue material; do not edit.
- `go/`, daemon/Postgres substrate code, packaging metadata, release docs, and unrelated CLI behavior.
- `.striatum/` -- live runner state; never edit by hand.

## Acceptance Checklist

1. [GH12-1] `copy_on_click.js` must not attach copy behavior to every document-level `[data-copy]` element.
2. [GH12-2] Copy activation must be limited to elements inside explicit allowed contexts, such as `.recipe-list`, `.code-recipe`, or `.copyable-token`, or to an equivalent closed allowlist justified in the handoff.
3. [GH12-3] `enhanceTargets()` must only add keyboard/click affordances to allowed copy controls, not to arbitrary injected `[data-copy]` elements.
4. [GH12-4] Click and Enter-key handlers must ignore `[data-copy]` targets outside the allowlist without copying, showing success state, or preventing ordinary page interaction.
5. [GH12-5] Existing legitimate copy recipes in recovery panels or code-recipe surfaces must still copy the intended command text.
6. [GH12-6] Tests must fail on the old global-selector behavior and pass on the allowlisted behavior.
7. [GH13-1] Changing a workflow job from `type: "review"` to any non-review type must remove `require_attested_lane` from the in-memory job object.
8. [GH13-2] Serialized workflow output must not include `require_attested_lane` on non-review jobs after a type change.
9. [GH13-3] Node labels and textual summaries must not display `require_attested_lane=true` for non-review jobs.
10. [GH13-4] Review jobs must continue to preserve, edit, serialize, and display `require_attested_lane=true` when explicitly enabled.
11. [GH13-5] Add a Vitest regression that starts with a review job carrying `require_attested_lane: true`, changes the type to a non-review job, and asserts the serialized job omits the field.
12. [GH13-6] The fix should normalize any other review-only fields touched by the same type-change path only when doing so is directly necessary to prevent the same ghost-field class; broad workflow-editor cleanup is out of scope.

## Risks And Conflicts

- GH #9-#11 may touch adjacent web assets and tests, especially `copy_on_click.js`, recovery panel recipes, and static web security tests. If branches conflict, preserve this bundle's narrow allowlist behavior while letting the #9-#11 branch own request-origin and `/v1/invoke` hardening.
- The copy allowlist must not break existing recovery-panel command copying. The likely safe implementation is a small predicate shared by `findCopyTarget()` and `enhanceTargets()` rather than styling-only changes.
- The graph-editor change must avoid deleting `require_attested_lane` during ordinary review-job edits. Purge only on type change away from `review` or during a targeted normalization before save.
- If `make ui-build` updates bundled files, the implementer must include the generated bundle/hash changes or explicitly state why the bundle was not rebuilt.

## Verification Commands

Run the focused checks first:

```bash
make ui-test
PYTHONPATH=src python3 -m pytest tests/test_copy_on_click.py tests/test_run_detail_recovery_panel.py tests/test_recovery_panel_dry_run.py
```

Run the bundle drift check if frontend source changed:

```bash
make ui-check-bundle
```

Run the broader project checks when time permits:

```bash
make lint
make typecheck
make test
```
