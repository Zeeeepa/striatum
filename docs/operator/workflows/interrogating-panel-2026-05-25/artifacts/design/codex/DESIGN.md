# Interrogation Log Chat UI Design
author: operator

## Recommendation

Land this as a narrow Go web-service slice:

- Reuse the existing daemon read methods, especially
  `reads.HandleInterrogationShow` at `go/pkg/reads/interrogation.go:36`.
- Add thin HTTP GET routes under the existing run route:
  `GET /v1/runs/{runID}/interrogations` and
  `GET /v1/runs/{runID}/interrogations/{interrogationID}`.
- Render the operator-facing chat section server-side from Go templates and
  embedded CSS. Do not make this feature depend on a React island.
- Do not close F36 in this slice. F36 is a broader web-asset product decision,
  not a prerequisite for showing curated interrogation turns.

The goal is a readable run-history chat transcript, not a frontend-platform
migration.

## Read And Endpoint Path

Use the existing read model:

- `interrogation.list` is already registered at `go/pkg/reads/reads.go:160`
  and implemented by `HandleInterrogationList` at
  `go/pkg/reads/interrogation.go:11`.
- `interrogation.show` is already registered at `go/pkg/reads/reads.go:161`
  and implemented by `HandleInterrogationShow` at
  `go/pkg/reads/interrogation.go:36`.
- `HandleInterrogationShow` already returns ordered turns from
  `queue_messages`, sorted by `turn_index`, `created_at`, and `message_id` at
  `go/pkg/reads/interrogation.go:59`.
- The RPC registry marks both read methods as read-capability methods at
  `go/pkg/rpc/registry_methods.go:12`.

Add HTTP routes in `routeRunGET`:

- `go/pkg/webservice/service.go:107` is the run-scoped GET sub-router.
- `go/pkg/webservice/service.go:118` is the switch where the new
  `"interrogations"` case belongs.

Route behavior:

- `GET /v1/runs/{runID}/interrogations` calls
  `h.callAndWrite(..., "interrogation.list", {"run_id": runID})`.
- `GET /v1/runs/{runID}/interrogations/{interrogationID}` calls
  `h.call(..., "interrogation.show", {"interrogation_id": interrogationID})`,
  verifies the returned `interrogation.run_id` equals the path `runID`, then
  writes the same JSON envelope. If the run does not match, return 404.

That nested-run check matters because `interrogation.show` is keyed by
`interrogation_id`, not by `run_id`. The HTTP route should not allow a URL
under one run to display a thread from another run.

Avoid `POST /v1/invoke` for the page path. It works today, but it is a generic
RPC escape hatch and reads poorly in a browser UI. Dedicated GET routes keep
the feature read-shaped, testable, bookmarkable, and aligned with the existing
`routeRunGET` pattern at `go/pkg/webservice/service.go:118-131`.

## Rendering Approach

Render server-side for this feature.

The current Go web service embeds only `go/pkg/webassets/static/*` and
`go/pkg/webassets/templates/*` via `go/pkg/webassets/assets.go:16`. The Vite
bundle lives outside that embed tree, which is the F36 gap documented at
`docs/TODO.md:1510`. Pulling in a React island would require deciding the
whole served-assets architecture: Vite `outDir`, embed globs, page mount
contracts, bundle hash expectations, and release packaging.

For this feature, a server-rendered chat section is enough:

- Add a typed render helper near `webassets.RenderPage` at
  `go/pkg/webassets/assets.go:39`.
- Add a run-detail template under `go/pkg/webassets/templates/`, keeping
  `page.html` at `go/pkg/webassets/templates/page.html:11` available for the
  current generic JSON page.
- Extend `go/pkg/webassets/static/base.css:1` with a compact chat layout:
  thread container, turn rows, question/answer alignment, speaker label,
  metadata line, and `white-space: pre-wrap` for authored bodies.
- Use `html/template` escaping and render turn bodies as plain text, not
  Markdown or raw HTML.

Suggested chat model:

- Thread header: topic when present, state, interrogator session id, target
  session id, opened/closed times when present.
- Turn row class from `kind`:
  `interrogation_question` -> reviewer/question styling;
  `interrogation_answer` -> target/answer styling.
- Turn body from `body`.
- Turn ordering exactly as returned by `HandleInterrogationShow`; do not sort
  again in the browser.

Do not render provider stdout/stderr or terminal data. The read handler already
projects curated fields only, and the template should stay on those fields.

## Run-Detail Integration Point

Use the Go web service as the integration boundary.

API integration:

- Add the route cases in `go/pkg/webservice/service.go:118`.
- Reuse `h.call`/`h.callAndWrite`, which already injects `repository_id` at
  `go/pkg/webservice/service.go:208`.
- Keep the route behind the same loopback host, service-token, and CSP
  behavior applied in `ServeHTTP` at `go/pkg/webservice/service.go:47`.

HTML integration:

- Add a WebEnabled run page route near `go/pkg/webservice/service.go:96`.
  Suggested path: `GET /run/{runID}`.
- The handler should call `run.detail` for the base run view. That read exists
  at `go/pkg/reads/detail.go:63` and is registered at
  `go/pkg/reads/reads.go:130`.
- The handler then calls `interrogation.list` for the same `runID`, calls
  `interrogation.show` for each returned interrogation id, builds a view model,
  and renders the server-side template.

This leaves the existing `/v1/runs/{runID}` JSON route behavior intact while
giving operators a real run-detail HTML page with the interrogation transcript.

## Test Plan

Add focused Go tests.

1. Route tests in `go/pkg/webservice/service_test.go`:
   - `GET /v1/runs/run_1/interrogations` reaches
     `interrogation.list` with `run_id=run_1` and injected
     `repository_id`.
   - `GET /v1/runs/run_1/interrogations/intg_1` reaches
     `interrogation.show` and returns ordered turn JSON.
   - A returned `interrogation.run_id` mismatch on the nested show route
     returns 404.
   - The routes work with `AllowMutations=false`.

2. Rendering assertion in `go/pkg/webservice/service_test.go` or
   `go/pkg/webassets/assets_test.go`:
   - Seed a fixture response with one question and one answer.
   - Request `GET /run/run_1`.
   - Assert the response is `text/html`, contains the question before the
     answer, contains speaker labels/classes for question and answer, and
     escapes HTML in the body.

3. Existing read coverage can remain where it is:
   `go/pkg/mutations/interrogation_test.go:173` already exercises
   `HandleInterrogationList` and `HandleInterrogationShow`; do not duplicate
   the database-level read test unless the implementation changes the read
   handler.

4. Verification commands for the build lane:
   - `go test ./pkg/webservice ./pkg/webassets ./pkg/reads`
   - `go test ./...`
   - `make lint typecheck test`

No `make ui-check-bundle` should be required unless the implementer chooses
to close F36, which this proposal explicitly avoids.

## Smallest Landable Slice

The smallest useful slice is:

1. Add the two run-scoped interrogation GET routes in `routeRunGET`.
2. Add a server-rendered `/run/{runID}` detail page that includes an
   "Interrogations" section when threads exist.
3. Render each thread as ordered chat turns using `kind`, `body`, and
   `turn_index` from `interrogation.show`.
4. Add compact CSS in `go/pkg/webassets/static/base.css`.
5. Add route and rendering tests.

Explicitly defer:

- React island work.
- Vite `outDir`/embed changes.
- Live SSE updates for newly arriving turns.
- Markdown rendering in turn bodies.
- Search, collapse/expand, filtering, and per-thread deep links.
- Any migration, new RPC method, or interrogation data-model change.

This gets the payoff for the current interrogating-panel run: an operator can
open the run detail page and read the reviewer questions and target answers as
a chat transcript, backed entirely by existing curated interrogation records.
