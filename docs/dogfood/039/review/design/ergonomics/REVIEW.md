---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0037", "web-ui"]
---

author: reviewer-gemini-pro-001

# Ergonomics-DX Review: RFC 0037 Web UI Improvements

## Summary

The synthesized design for RFC 0037 provides a comprehensive and ergonomic solution to the "noise wall" and discoverability issues identified in the current Web UI. The plan adheres strictly to the project's technical constraints (vanilla ES6, Jinja2, no bundler) while delivering high-leverage improvements to operator efficiency and triage flow.

The proposed affordances are consistent, discoverable, and follow established industry patterns (GitHub Actions, Vercel, GitLab) in a way that feels native to the Striatum ecosystem.

## Evaluation Against Posture Criteria

### 1. Discoverable and Consistent Affordances
The design introduces standard dashboard controls (filter bars, status pills, date pickers) that are immediately recognizable. The promotion of "Next Actions" to a prominent banner just below the run header significantly improves the primary triage flow. The use of a localtime toggle in the site header with a visible state indicator ensures that operators can work in their preferred time format without ambiguity.

### 2. Specific and Actionable Filter UX
The filter choices for the run list and workflows index are well-calibrated. Specifically:
- **Run List:** Free-text search + state pills + date-range select covers the most common "find my recent failures" and "find branch X" workflows.
- **Workflows Index:** Path/ID search + status filter addresses the friction of navigating large workflow collections.
- **Doctor:** The "hide problems on terminal runs" toggle is a critical ergonomic win for machines with high dogfood volume.

### 3. Keyboard Navigation and Discoverability
The mnemonic `g + <key>` sequence (Runs, Workflows, Chat, Doctor) matches the top-nav order and follows common power-user patterns. The `?` help overlay provides immediate discoverability for these shortcuts, and the focus-guard ensures they don't interfere with typing in inputs or textareas.

### 4. Developer Experience (DX) and Empty States
The empty-state copy is exemplary. Instead of boilerplate "No items found," it provides specific, copy-pasteable CLI commands (e.g., `striatum run prepare --workflow <workflow.json>`) that guide the user to the next logical step. This is a high-signal DX affordance.

### 5. JS Architecture and Dark-Mode Parity
The plan remains "honest" to the vanilla JS/CSS architecture, avoiding dependency creep. The `app.css` dark-mode audit is exhaustive, listing all key component classes to ensure visual consistency across themes. Using `localStorage` for state persistence (filters, toggle modes, collapsed sections) is appropriate for a local-first tool and avoids the complexity of server-side preference storage in V1.

### 6. Staging and Deferred Scope
The staging plan correctly prioritizes low-risk shared infrastructure (`base.js` scaffold) before moving to page-specific logic. The deferred items (querystring state, sticky banners, shortcut configurability) are clearly named and appropriately scoped out of the MVP.

## Conclusion

The design is accepted without reservation. It successfully transforms the Web UI from a scrappy introspection surface into a polished, operator-centric dashboard while preserving the project's architectural integrity.
