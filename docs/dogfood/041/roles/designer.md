# Designer Role (Dogfood 041)

You produce implementation-ready design artifacts for RFC 0038: web UI feature additions + Vite/React/TypeScript frontend toolchain. Sit on top of RFC 0013 V1 / RFC 0022 V1 / RFC 0023 V1.5 / RFC 0024 V1+V1.5+V2+V3+V4 / RFC 0034 V1; do not redesign those. D092 supersedes D073's "no node toolchain" rule.

Concrete coverage required: project layout under `src/striatum/web/frontend/` (package.json, vite.config.ts, tsconfig.json, src/islands/, src/shared/, src/__tests__/); Makefile targets (`ui-install`, `ui-build`, `ui-dev`, `ui-test`); CI node-22-LTS integration with bundle-hash check; wheel package-data for `src/striatum/web/static/build/`; island mounting pattern (data-attribute props, `createRoot()` into named DOM slots); the five feature additions per RFC 0038 §5; service.py route additions for `GET /v1/repo/tree`; Jinja2 template updates promoting the Edit affordance + adding island mount points.

Deployment shape: islands architecture (Jinja2 page shells + React islands). NOT full SPA. Server-rendered pages stay; framework usage is scoped to specific component slots. CSP unchanged.

Library choices per RFC 0038: react-flow for graph editor; shiki for syntax highlighting. Bundle the 8 named grammars (json, py, ts/js, sh, yaml, toml, md, sql). Vendored, no CDN.

Out of scope: hosted-mode UX; mobile-first responsive overhaul; replacing Jinja2 server-side rendering entirely; new auth surfaces; SVG zoom/pan (RFC 0022 V1.5 deferred); cross-platform Windows daemon.
