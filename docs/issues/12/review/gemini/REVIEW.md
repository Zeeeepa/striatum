author: reviewer-unknown-model-001

# Security Review: GH #12 & #13

Security-focused review of clipboard ergonomics and stale field indicators.

## Verdict: `accept`

The changes successfully mitigate the reported security and integrity issues without introducing new attack surface.

## Acceptance Checklist

### GH #12: Clipboard Poisoning Mitigation
- [x] **Restricted Matching:** `src/striatum/web/static/copy_on_click.js` now uses an explicit allowlist of container selectors (`.recipe-list`, `.recovery-auto-publish`, etc.).
- [x] **Double-Gated Activation:** The allowlist check is performed in both the accessibility enhancement path (`enhanceTargets`) and the interaction path (`findCopyTarget`).
- [x] **Safe Fallback:** `isAllowedCopyTarget` correctly handles non-element nodes and ensures only intended UI regions can trigger clipboard writes.
- [x] **Evidence:** `src/striatum/web/static/copy_on_click.js:15-28`, `src/striatum/web/static/copy_on_click.js:65`.

### GH #13: Workflow Editor State Integrity
- [x] **Field Purging:** `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx` implements `purgeStaleFieldsForType` to drop review-only fields when transitioning away from `review`.
- [x] **Data Binding Consistency:** `handleJobChange` ensures that the in-memory state is cleaned before synchronization, preventing "ghost" fields in node labels and serialized JSON.
- [x] **Immutable Safety:** The purging logic uses object spreads to avoid direct mutation of the state.
- [x] **Evidence:** `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:168-192`, `src/striatum/web/frontend/src/islands/workflow-graph-editor/WorkflowGraphEditor.tsx:1123-1133`.

## Verification Assessment

### Automated Tests
- **Python (GH #12):** `tests/test_copy_on_click.py` passes. It asserts the presence of the allowlist and the gated activation in the static asset.
- **Vitest (GH #13):** `src/striatum/web/frontend/src/__tests__/workflow-graph-editor.test.ts` passes with 34 tests. It includes comprehensive regression tests for field purging, covering serialization and label rendering.

### Manual Inspection
- Verified that `data-copy` usages in `_recovery_panel.html` and `RecoveryPanel.tsx` are correctly wrapped in allowed containers (`.recipe-list`, `.recovery-auto-publish`).
- Verified that `override_verdict.js` does not use `data-copy` and instead uses a standard form-based flow with its own context validation.

## Findings

No security weaknesses or actionable gaps were identified in the implementation of these specific issues. The approach to clipboard safety (closed allowlist) is a robust mitigation for the reported poisoning vector. The state purging in the workflow editor correctly addresses the reported data integrity issue.
