# Designer Role (Dogfood 045)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0038 V1.5 F1-F4 + supply-chain hygiene. Synthesis
picks one path. Cite the existing code that your design changes — do
not propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0038-web-ui-feature-additions-and-frontend-toolchain.md`.
- `docs/dogfood/041/OPERATOR_REPORT.md` — F1-F4 findings narrative
  and gemini supply-chain findings.
- `src/striatum/web/frontend/vite.config.ts` — F1 placeholder plugin
  and F3 `island-shared` -> `main.ts` mapping land here.
- `src/striatum/web/frontend/src/main.ts` — shared-bundle entry. F3
  double-mount risk surfaces from this file's side effects.
- `src/striatum/web/frontend/src/islands/workflow-chooser/` — F2
  chooser island; React component prop-contract.
- `src/striatum/web/frontend/package.json` and `package-lock.json` —
  supply-chain surface; lockfile + audit baseline policy.
- `src/striatum/web/` — F2 server route emitting `{templates: [...]}`,
  F4 served bundle path and `package_data` shape.

Address: F1 placeholder removal (exact `vite.config.ts` change + the
real bundle filenames `make ui-build` produces); F2 prop-contract fix
(pick which side moves); F3 double-mount fix (pick one approach);
F4 output / package-data surface (named glob); supply-chain hygiene
(lockfile policy, audit baseline path, Makefile target).

**Backward compatibility for existing islands is non-negotiable** —
every currently-mounting island must keep mounting; served bundle URLs
under `src/striatum/web/static/build/` must keep resolving; the
`/workflows/new` page must continue to render its chooser after the
prop-contract fix. The design must explicitly note this.

Out of scope: hosted services, new island surface beyond F1-F4 wiring,
backend RFC work, Python daemon changes.
