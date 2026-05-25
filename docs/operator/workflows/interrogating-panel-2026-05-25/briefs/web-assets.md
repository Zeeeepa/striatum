# Web Assets Brief — Interrogation Chat Panel (2026-05-25)

Reconnaissance for the feature that will render a run's interrogation Q&A
thread as a chat-style transcript in the workflow-history web UI.

---

## 1. How the Go web service serves HTTP today

**Entry point:** `go/cmd/striatumd/web_service.go:18`
`newWebServiceHandler` wraps `webservice.New(webservice.Config{...})` and
returns a `*webservice.Handler`.

**Core handler:** `go/pkg/webservice/service.go:47`
`Handler.ServeHTTP` enforces loopback-host check, bearer-token auth, and
dispatches to `routeGET` or `routePOST`.

### Route table (GET)

| Pattern | Handler | RPC method |
|---------|---------|------------|
| `/v1/health` | inline | — |
| `/v1/runs` | `callAndWrite` | `status` |
| `/v1/runs/{runID}` | `routeRunGET` | `status` (run-scoped) |
| `/v1/runs/{runID}/why?id=` | `callAndWrite` | `why` |
| `/v1/runs/{runID}/dashboard` | `callAndWrite` | `dashboard` |
| `/v1/runs/{runID}/artifacts` | `callAndWrite` | `list.artifacts` |
| `/v1/runs/{runID}/events` | `streamRunEvents` (SSE) | `run.events` |
| `/v1/artifacts/{id}/raw` | `serveArtifactRaw` | `artifact.get_content` |
| `/workflow-templates` | `callAndWrite` | `workflow.templates.list` |
| `/workflow-templates/{id}` | `callAndWrite` | `workflow.templates.show` |
| `/ ` or `/run` (WebEnabled) | `renderRPCPage` | `status` |
| `/static/{file}` (WebEnabled) | `serveStatic` | — |

`service.go:72-105`

### Route table (POST)

| Pattern | Handler |
|---------|---------|
| `/v1/invoke` | `handleInvoke` — dispatches any RPC method by name |
| `/workflows/generate/preview` | `handleWorkflowGenerate` |
| `/workflows/generate` | `handleWorkflowGenerate` |

`service.go:137-153`

**Key point:** There is NO dedicated HTTP route for interrogation. Interrogation
reads must go through `POST /v1/invoke` with `{"method":"interrogation.list"}`
or `{"method":"interrogation.show"}`. The `routeRunGET` sub-router
(`service.go:107-135`) has no `interrogation` case.

---

## 2. Static asset embedding

**Embed directive:** `go/pkg/webassets/assets.go:16`
```go
//go:embed static/* templates/*
var embedded embed.FS
```

**Files embedded** (the only three files in `go/pkg/webassets/`):
- `go/pkg/webassets/static/app.js` — one line: sets `document.documentElement.dataset.striatumWeb = "go"`
- `go/pkg/webassets/static/base.css` — minimal system-font body styles
- `go/pkg/webassets/templates/page.html` — renders `<h1>{{.Title}}</h1><pre>{{.Payload}}</pre>`, loads `/static/base.css` and `/static/app.js`

`LoadStatic(relative)` reads from `embedded` at `static/<relative>` and sets
MIME type. `RenderPage(title, payload)` executes `templates/page.html` with
JSON-encoded RPC data. `assets.go:24-57`

**Served page:** Only `/ ` and `/run` (when `WebEnabled`) call `renderRPCPage`
which calls `webassets.RenderPage`. The rendered output is a raw `<pre>` dump
of the `status` RPC response. `service.go:281-295`

---

## 3. The F36 gap: React/Vite bundle built but not embedded

**F36 statement (docs/TODO.md:1510-1518):**
`go/pkg/webassets` embeds only the three hand-authored files above. The
React/Vite islands bundle is built to `src/striatum/web/static/build/` and
bundle-hash-checked in CI (`make ui-check-bundle`), but it is **NOT embedded
or served by the Go daemon's web service**.

**React frontend source:** `src/striatum/web/frontend/` (Vite + React 19 +
TypeScript)

**Vite build config:** `src/striatum/web/frontend/vite.config.ts:17`
```
outDir: resolve(rootDir, "../static/build")
```
resolves to `src/striatum/web/static/build/` — separate from the Go embed
tree at `go/pkg/webassets/`.

**Build output (present on disk):** `src/striatum/web/static/build/`
- `island-shared.js`
- `island-tree-browser.js`
- `island-recovery-panel.js`
- `island-code-viewer.js`
- `island-shiki-BzAFxaqU.js` (code highlighting, ~large)
- `api-client-D8suZpXg.js`
- `jsx-runtime-BixbmuNB.js`
- `style.css`, `style2.css`
- `manifest.sha256` (CI integrity check)

**Why it is not served:** The embed directive `//go:embed static/* templates/*`
at `go/pkg/webassets/assets.go:16` only covers files inside
`go/pkg/webassets/static/` and `go/pkg/webassets/templates/`. The Vite
`outDir` writes to `src/striatum/web/static/build/` — a completely different
tree. No `//go:embed` directive covers that path; no Makefile target copies
build output into the Go embed tree; no route in `webservice.Handler`
references the build directory. The Go service has no awareness of the Vite
bundle's existence.

**CI check:** `make ui-check-bundle` (Makefile:133) runs `ui-build` +
`ui-verify-bundle` + `ui-bundle-size` + `git diff --exit-code` to keep the
committed bundle in sync with the source. This confirms the bundle is
tracked in git and CI-validated, but the Go service never reads it.

---

## 4. Existing workflow-history view

### Vanilla JS SPA (currently served)

The served UI is a vanilla ES-module SPA. Source lives in
`src/striatum/web/static/` (not embedded in Go — these are the Python-era
assets now tracked for the Node-side pipeline). The Go embed only serves the
stub `app.js` and `base.css` from `go/pkg/webassets/static/`.

The SPA routing is in `src/striatum/web/static/app.js:640-672`:
- `#/` → `renderRunList()` (line 224) — calls `GET /v1/runs`, renders a table of runs
- `#/runs/{runID}` → `renderRunDetail(runId)` (line 247) — calls `GET /v1/runs/{runID}`, renders jobs + SSE live-update
- `#/runs/{runID}/jobs/{jobID}` → `renderJobDetail`
- `#/runs/{runID}/artifacts/{artifactID}` → `renderArtifactView`

`renderRunDetail` (app.js:247) shows run state, job list, and artifact list.
There is **no interrogation panel** in the existing run-detail view. No call
to `interrogation.list` or `interrogation.show` is made anywhere in the SPA.

### React islands (built, not served by Go)

The React app has no run-detail or workflow-history page. The four existing
islands are:
- `island-tree-browser` — directory browsing (`TreeBrowser.tsx`)
- `island-code-viewer` — syntax-highlighted file view (`CodeViewer.tsx`)
- `island-recovery-panel` — stale-lease recovery actions (`RecoveryPanel.tsx`)
- `island-shared` — shared entry for common components

None of these islands renders run history or interrogation threads.
`src/striatum/web/frontend/src/islands/`

---

## 5. What an implementer must do to add the interrogation panel

To wire a React island into the Go-served UI, the implementer must resolve
the F36 gap. The two concrete options named in TODO.md:1515-1517:

**Option A (embed the Vite bundle in Go):** Change Vite `outDir` from
`src/striatum/web/static/build` to `go/pkg/webassets/static/build`; extend
the `//go:embed` glob; extend `page.html` with island mount points; add a
route in `routeRunGET` that renders a page calling the new island.

**Option B (keep the minimal server-rendered surface):** Extend the
`page.html`/`app.js` stub surface with vanilla JS to call
`POST /v1/invoke {"method":"interrogation.list","params":{...}}` and render
the thread inline — no React bundle change needed.

The interrogation data is accessible today via `POST /v1/invoke`. No new
HTTP route is required for data access; the gap is purely presentation-layer.
