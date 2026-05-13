---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "reject"
severity: "critical"
tags: ["threat_model", "rfc-0038", "v1-5", "build"]
---

author: reviewer-unknown-model-001

# Build Review — Codex Threat Model

Verdict: `reject`

The V1.5 implementation improves the source-side controls, but the shipped artifact boundary is still broken: committed bundles under `src/striatum/web/static/build/` remain placeholders, and the handoff explicitly says the real Vite bundle commit did not happen. This fails the central supply-chain and "no placeholder bundles shipped" acceptance condition for this review.

## Trust Boundaries And Attack Surfaces

The new frontend toolchain creates four material boundaries:

1. Contributor dependency resolution: `package.json` / `package-lock.json` / npm install output become inputs to trusted shipped JavaScript. The implementation adds `npm ci`, `ui-update-lock`, `ui-audit`, and an audit-baseline file, but the handoff still records a follow-up to regenerate the lockfile and optionally move `@vitejs/plugin-react` out of runtime dependencies (`docs/dogfood/045/build/HANDOFF.md:171`, `docs/dogfood/045/build/HANDOFF.md:188`, `docs/dogfood/045/build/HANDOFF.md:305`).
2. Build artifact provenance: Vite source and package data cross into committed static files that the wheel serves. RFC 0038 makes committed bundles authoritative wheel artifacts (`docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md:203`, `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md:217`), so placeholder output is not a cosmetic failure; it is the code users receive.
3. Server-to-island prop contracts: Jinja templates serialize `data-props`; React islands trust the shape. The handoff says the chooser was rewritten around the server-stable `templates` response (`docs/dogfood/045/build/HANDOFF.md:63`, `docs/dogfood/045/build/HANDOFF.md:83`, `docs/dogfood/045/build/HANDOFF.md:97`), which is the right boundary to harden.
4. Production mount topology: `base.html` loads `island-shared.js`, while page templates load per-island bundles. The handoff moves `island-shared` to a non-mounting entry and preserves public bundle URLs (`docs/dogfood/045/build/HANDOFF.md:101`, `docs/dogfood/045/build/HANDOFF.md:116`, `docs/dogfood/045/build/HANDOFF.md:129`).

## Findings

### F1 — Critical — Shipped bundles are still placeholders

Required check: real bundles. Result: failed.

The handoff says real Vite bundles were not regenerated and that the placeholder bundles remain committed in `src/striatum/web/static/build/` (`docs/dogfood/045/build/HANDOFF.md:233`, `docs/dogfood/045/build/HANDOFF.md:235`, `docs/dogfood/045/build/HANDOFF.md:237`). I spot-checked `src/striatum/web/static/build/island-workflow-chooser.js:1`; it contains only:

```js
console.info("Striatum workflow-chooser island placeholder loaded");
```

That is not compiled React output. Because RFC 0038 requires bundled output to live under `src/striatum/web/static/build/` and ship with the wheel (`docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md:361`), accepting this state would ship a known inert UI artifact.

### F2 — High — Build verification is intentionally absent for this run

Required check: `make ui-build`, `make lint`, `make typecheck`, and `make test` pass per the handoff; spot-check at least one. Result: failed.

The handoff states `make lint`, `make typecheck`, `make test`, `make ui-test`, and `make ui-build` were not executed (`docs/dogfood/045/build/HANDOFF.md:257`, `docs/dogfood/045/build/HANDOFF.md:259`). It also records that `make ui-build` and related npm commands were denied (`docs/dogfood/045/build/HANDOFF.md:287`, `docs/dogfood/045/build/HANDOFF.md:291`). I did not substitute an untracked build because this review is about whether the implementation artifact is shippable as handed off; it is not.

### F3 — Medium — Source-side mitigations are promising but not proven against real output

Required check: F1-F4 plus supply-chain covered. Result: partially covered.

The handoff documents source changes for placeholder plugin removal (`src/striatum/web/frontend/vite.config.ts`, `docs/dogfood/045/build/HANDOFF.md:18`), chooser contract alignment (`src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`, `docs/dogfood/045/build/HANDOFF.md:83`), double-mount prevention (`src/striatum/web/frontend/src/shared/island-shared-entry.ts`, `docs/dogfood/045/build/HANDOFF.md:103`), package-data layout (`docs/dogfood/045/build/HANDOFF.md:139`), and supply-chain targets (`Makefile`, `docs/dogfood/045/build/HANDOFF.md:178`; `npm-audit-baseline.json`, `docs/dogfood/045/build/HANDOFF.md:173`). Those controls address the right attack surfaces, but with no regenerated bundle and no executed tests, they remain source claims rather than validated shipped behavior.

### F4 — Medium — Backward-compatible URLs are preserved, but runtime mount behavior cannot be accepted while bundles are placeholders

Required check: backward compatibility. Result: source checklist present; shipped runtime failed.

The handoff preserves stable mount IDs and public URLs (`docs/dogfood/045/build/HANDOFF.md:196`, `docs/dogfood/045/build/HANDOFF.md:201`) and cites regressions for `/workflows/new`, `/workflows/edit/<path>`, `/view/<path>`, and `/view/` (`docs/dogfood/045/build/HANDOFF.md:206`, `docs/dogfood/045/build/HANDOFF.md:209`, `docs/dogfood/045/build/HANDOFF.md:211`, `docs/dogfood/045/build/HANDOFF.md:213`). But a placeholder bundle at the unchanged URL means the served path is stable while the served behavior is still nonfunctional. `/workflows/new` may render a server shell, but its island entry cannot run the chooser wizard until real compiled JavaScript replaces the placeholder.

## Acceptance Bar

To move this to `accept_with_findings` or `accept`, the follow-up must commit real Vite output and a regenerated `manifest.sha256`, then run at least `make ui-check-bundle`, `make ui-test`, and the Python regression suite that covers package-data resolution. The committed bundle files must no longer contain the placeholder sentinel and must be reviewed as the exact bytes that the wheel will serve.
