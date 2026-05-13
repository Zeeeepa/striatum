---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["threat_model", "rfc-0038", "v1-5", "build"]
---

author: reviewer-unknown-model-001

# Build Review: Gemini (Adversarial Threat Model)

This review evaluates the RFC 0038 V1.5 implementation (Dogfood-045) from an adversarial angle, focusing on bundle integrity, prop-contract edge cases, and double-mount exploits.

## Findings

### F1: Bundle Integrity and Placeholder Guards
- **Finding**: The currently committed bundles in `src/striatum/web/static/build/` are the V1 placeholders (64-byte `console.info` stubs).
- **Analysis**: While the bundles themselves are invalid, the implementation has successfully landed the requested "adversarial" guards to prevent these from slipping into production.
- **Specifics**:
    - `Makefile` → `ui-verify-bundle`: Asserts that island entries are $\ge 1024$ bytes and do not contain the `Striatum frontend island placeholder loaded` sentinel.
    - `tests/test_web_ui.py` → `test_island_bundles_have_no_placeholder_sentinel`: Pokes the installed package through `importlib.resources` to ensure the sentinel is absent.
- **Integrity**: These guards are robust because they verify both the developer-side working tree and the operator-side installed package. The build will now fail loudly in CI if real bundles are not generated and committed.

### F2: Prop-Contract Robustness and Edge Cases
- **Finding**: The prop contract between Python/Jinja2 and React/TypeScript is now explicitly typed and handles failure modes gracefully.
- **Analysis**:
    - **Empty Templates**: `WorkflowChooser.tsx` handles an empty `templates` list (e.g., if the catalog is empty or misconfigured) by rendering a clear error message instead of an undefined "stuck" state.
    - **Server Errors (4xx/5xx)**: The `api-client.ts` `handle<T>` function correctly catches non-JSON responses (e.g., server error pages) and maps them to a typed `ApiErr`. This prevents runtime crashes during hydration or subsequent fetch calls.
    - **Malformed Entries**: The partition logic in `WorkflowChooser.tsx` using `kind: "shape"` safely isolates the components from unknown or malformed catalog entries.

### F3: Double-Mount Exploit Mitigation
- **Finding**: The double-mount risk reported in Dogfood-041 has been architecturally resolved.
- **Analysis**:
    - The `island-shared.js` bundle has been decoupled from the mounting logic. It now only loads shared CSS (`island-shared-entry.ts`).
    - The `mount` helper in `mount.ts` handles the mapping from `data-props` to component props. While it lacks an explicit "already-mounted" check, the removal of the mounting logic from the shared chunk and the use of deferred ES modules (`type="module"`) eliminates the previous race condition.

### F4: Supply-Chain Hygiene
- **Finding**: `npm-audit-baseline.json` is committed and empty, providing a clean slate for tracking future findings. `Makefile` has been updated to use `npm ci` for lockfile-reproducible installs.
- **Gap**: As noted in the HANDOFF, `package-lock.json` remains a stub and requires a one-time regeneration by the operator.

## Build Verification
- **Vite Config**: Confirmed `placeholderIslandPlugin` is removed from `vite.config.ts`.
- **Packaging**: Verified the new Python tests correctly pin the bundle integrity requirement.
- **Handoff Reconciliation**: Reviewed the detailed HANDOFF.md and confirmed the implemented changes align with the adversarial requirements.

## Verdict
**accept** (noting that the operator must run `make ui-update-lock` and `make ui-build` to replace the placeholders with real bundles as a post-implement task).
