# DESIGN — Interrogation log as a chat transcript in the run-history UI

author: operator
lane: claude_code (designer, independent)

## Position in one sentence

Add **two thin GET routes** that re-expose the already-complete
`interrogation.list` / `interrogation.show` reads, and render the Q&A thread
**server-side** as an HTML chat fragment — leaving the F36 React/Vite embed
gap untouched, because this feature does not need it.

The data layer is done (`reads/interrogation.go:36`, turns already ordered by
`turn_index`). The whole feature lives in `go/pkg/webservice` +
`go/pkg/webassets`. No migration, no new RPC method, no mutation-path change.

---

## (a) Read / endpoint path — reuse the RPC, add GET shorthands

**Decision: reuse `HandleInterrogationShow` / `HandleInterrogationList`
verbatim; add GET routes, do NOT use `POST /v1/invoke` from the page.**

`routeRunGET` (`service.go:107-135`) already maps run-scoped paths onto reads
via `callAndWrite`. The interrogation reads are read-only, no mutation gate
(`rpc/registry_methods.go:12-13`), so they slot into the same pattern. Add to
the `switch parts[1]` block at `service.go:118`:

```go
case "interrogations":
    if len(parts) >= 3 && parts[2] != "" {
        // /v1/runs/{runID}/interrogations/{interrogationID}
        h.callAndWrite(w, r.Context(), "interrogation.show",
            map[string]any{"interrogation_id": parts[2]})
        return
    }
    // /v1/runs/{runID}/interrogations
    h.callAndWrite(w, r.Context(), "interrogation.list",
        map[string]any{"run_id": runID})
```

`callAndWrite` (`service.go:192`) injects `repository_id` from
`h.config.RepositoryID` (`service.go:208`), so the handler params stay minimal
and match the existing route ergonomics exactly.

Why not `POST /v1/invoke`: the brief notes it works today, but the run-detail
page is reached by GET and the rest of the run sub-tree is GET shorthands. A
GET route is cacheable, linkable, testable with `httptest` GETs, and
consistent with `why`/`dashboard`/`artifacts`. `/v1/invoke` stays as the
escape hatch; we don't build UI on it.

**No change to `reads/interrogation.go`.** The turn projection
(`reads/interrogation.go:71-82`) already gives `kind`, `body`, `turn`,
`turn_index`, `target_session_id` — everything the chat view needs.

---

## (b) Render approach — server-rendered HTML, F36 stays open

**Decision: server-rendered chat fragment. Do NOT adopt a JS island. Do NOT
close F36 as part of this feature.**

Rationale:

- The only served HTML today is `templates/page.html` (`webassets`), a raw
  `<h1>` + `<pre>{{.Payload}}</pre>` dump rendered by `renderRPCPage`
  (`service.go:281-295`). There is no React on the served surface at all.
- F36 (Vite `outDir` → `src/striatum/web/static/build/`, embed glob →
  `go/pkg/webassets/static/`, `webassets/assets.go:16`) is a *bundling*
  problem: a different output tree, a CI bundle-hash check
  (`make ui-check-bundle`), and `page.html` mount points. Solving it pulls in
  the entire Vite/React toolchain delivery story.
- This feature's Definition of Done — "operator sees Q&A as an ordered chat
  transcript" — is fully reachable with server-side templating. Coupling it to
  F36 would balloon a half-day read-render task into the frontend-delivery
  epic and add a CI bundle dependency to a docs-adjacent panel.

**Verdict on F36: leave it open.** This feature is the evidence that the
server-rendered path is sufficient for read panels; F36 should be reframed as
"interactive islands (tree/code/recovery) delivery," which this work does not
need. Note this explicitly so synthesis doesn't fold F36 into scope.

Concrete render plan: add a second template,
`go/pkg/webassets/templates/interrogation.html`, and a
`RenderInterrogation(meta, turns)` helper beside `RenderPage`
(`webassets/assets.go:24-57`). The template walks `turns` and emits, per turn,
a `<div class="turn turn-{{.turn}}">` with a speaker label derived from `kind`
(`interrogation_question` → "Reviewer", `interrogation_answer` → "Target") and
an HTML-escaped `body`. Add chat bubble styles to the existing
`static/base.css` (already embedded and linked at `page.html:7`) — left/right
alignment by `turn-question`/`turn-answer`. `text/template` auto-escapes
`{{.body}}`, satisfying D028's "authored text only, never raw stdout" because
we render exactly the curated `body` field and nothing else.

---

## (c) Run-detail integration point — `file:line`

The run-detail HTML page is `renderRPCPage` reached at `service.go:96`
(`clean == "/" || clean == "/run"`). It currently `<pre>`-dumps the `status`
RPC. Two-step integration:

1. **Data route** (above): the `switch parts[1]` at **`service.go:118`** gains
   the `case "interrogations":` arm. This is the load-bearing integration
   point and the smallest landable unit (see (e)).

2. **Rendered chat page**: add a third GET arm so an operator can open the
   thread as HTML, e.g. `/v1/runs/{runID}/interrogations/{id}?view=chat`
   detected inside the new `case`, calling a new
   `renderInterrogationPage(w, ctx, interrogationID)` modeled on
   `renderRPCPage` (`service.go:281`) but calling `RenderInterrogation`
   instead of `RenderPage`. The run-detail `<pre>` dump at **`service.go:96`**
   gets one added line: when the `status` payload exposes interrogations,
   append a "Interrogations" links section. (Because the served page is a
   `<pre>` today, the lowest-risk move is a sibling page reachable by link,
   not surgery inside the `<pre>` block — keep the dump intact, add a panel
   page next to it.)

So: routes at `service.go:118`; render helper next to `webassets/assets.go:24`;
template next to `templates/page.html`. No file outside those two packages is
touched.

---

## (d) Test plan

All Go, all `httptest` + an in-memory/seeded `db.Runner`; mirrors how other
`routeRunGET` arms are tested.

1. **Route → read wiring (handler test), `webservice` package.**
   Seed a run with one interrogation row in `striatumd.interrogations` and two
   `queue_messages` turns (one `interrogation_question`, one
   `interrogation_answer`, `turn_index` 0/1). `GET
   /v1/runs/{runID}/interrogations` → assert 200, `data.count == 1`,
   `items[0].interrogation_id`. `GET
   /v1/runs/{runID}/interrogations/{id}` → assert `data.turns` length 2,
   ordered by `turn_index`, `kind` values correct. This proves the route maps
   onto `interrogation.show` (`reads/interrogation.go:36`) with no behavior
   drift.

2. **Render assertion (template test), `webassets` package.**
   Call `RenderInterrogation(meta, turns)` with the two-turn fixture; assert
   the output contains both bodies, in turn order (question's byte offset <
   answer's), each inside a `turn-question` / `turn-answer` container, with
   "Reviewer"/"Target" speaker labels. Assert an HTML-meta-character body
   (e.g. `<script>`) is escaped — guards D028 + XSS in one assertion.

3. **Read-path regression (already-green guard).**
   A focused `reads` test asserting `HandleInterrogationShow` turn ordering and
   `kind` projection — likely exists; if not, add it so the route test isn't
   the only coverage of the projection at `reads/interrogation.go:71-82`.

Gate: `make lint typecheck test` + `go test ./...` green (per DoD).

---

## (e) Smallest landable slice

**Slice 1 (lands alone, this is the tracer bullet):** the `case
"interrogations":` arm at `service.go:118` wired to the two existing reads,
plus handler test #1. This makes the interrogation thread retrievable over
HTTP GET with zero new render code and zero data-model risk. Shippable and
useful (curl/scripts/CI can read the thread) on its own.

**Slice 2:** `RenderInterrogation` + `interrogation.html` + base.css bubbles +
render test #2 + the linked panel page. This is what makes an operator *see*
the chat and satisfies the full DoD, including the payoff: this run's own
design/build interrogation thread becomes viewable as chat.

F36 is explicitly **out of scope** for both slices.

---

## Summary table

| Concern | Choice | Anchor |
|---------|--------|--------|
| Data source | reuse, unchanged | `reads/interrogation.go:36` |
| HTTP exposure | new GET arm, not `/v1/invoke` | `service.go:118` |
| Render | server-side template + base.css | `webassets/assets.go:24`, `page.html:7` |
| Run-detail hookup | sibling panel page + link, keep `<pre>` | `service.go:96`,`281` |
| F36 | leave open, decouple | `webassets/assets.go:16` |
| Migration / new RPC | none | — |
| Smallest slice | GET route + handler test | `service.go:118` |
