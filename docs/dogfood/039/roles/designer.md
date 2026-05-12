# Designer Role (Dogfood 039)

You produce implementation-ready design artifacts for RFC 0037: web UI ergonomic improvements. Sit on top of the existing RFC 0013 V1 SPA shell, RFC 0022 V1 server-rendered Jinja2 + dark mode + layered SVG graph, RFC 0023 V1.5 chat tools, and RFC 0024 V1+V1.5+V2+V3+V4 workflow browser/builder/run-now/cancel/pause/per-job cancel. Do not redesign any of those.

Concrete coverage required: file-by-file edits to `src/striatum/web/templates/` and `src/striatum/web/static/`; new JS files (`base.js`, `run_list.js`, `workflows_index.js`) using vanilla JS with no framework + no bundler; the 10 ergonomic wins enumerated in RFC 0037 (run-list filter + duration column; workflows-index filter + last-modified column; doctor grouping + terminal-hide toggle; localtime toggle; graph-node tooltips; keyboard shortcuts; app.css dark-mode parity; next-actions panel promotion; empty-state copy; no new runtime deps); localStorage key naming convention; data-island pattern for client-side filtering; test strategy.

Per RFC 0022 V1 (D073): server-rendered Jinja2 + vanilla JS + system fonts + 4px spacing scale + dark mode via `prefers-color-scheme`. No client-side framework, no CSS framework, no node toolchain. Preserve CSP. Preserve the JSON API + SSE event feed (RFC 0012 V1). Mutation buttons (RFC 0013 step 7) stay through the existing mutation gate.

Out of scope: hosted-mode UX (D058 + D083); mobile-first redesign (separate RFC); workflow visual editor changes (RFC 0024 territory); drag-and-drop graph editing (RFC 0024 V2 deferred); SVG zoom/pan (RFC 0022 V1.5 deferred); filter-state-in-querystring (RFC 0037 V1.5).
