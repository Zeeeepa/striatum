---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# DESIGN SYNTHESIS — Interrogation log as a chat transcript

author: operator

Reconciles three independent design proposals into one buildable plan for the
interrogation-log chat UI:

- **codex** — `artifacts/design/codex/DESIGN.md`
- **claude_code** — `artifacts/design/claude_code/DESIGN.md`
- **gemini** — `artifacts/design/gemini/DESIGN.md`

## Where all three agree (the spine)

1. **Reuse the existing reads; no migration, no new RPC.** All three build on
   `reads.HandleInterrogationShow` (`go/pkg/reads/interrogation.go:36`) and
   `interrogation.list` — turns already arrive ordered by `turn_index` with
   `kind`/`body` projected. (codex §Read; claude_code §a; gemini §2.)
2. **Expose via dedicated GET routes, not `POST /v1/invoke`.** Add a
   `case "interrogations":` arm to the `routeRunGET` switch at
   `go/pkg/webservice/service.go:118`, using `callAndWrite`, which injects
   `repository_id` (`service.go:208`). (codex, claude_code, gemini all land here.)
3. **Do NOT close F36 in this slice.** F36 is the Vite/React embed-glob gap
   (`go/pkg/webassets/assets.go:16`, `docs/TODO.md:1510`); none of these
   designs need it. (Unanimous.)
4. **Curated fields only (D028).** Render `kind` + `body`, never provider
   stdout/stderr.

## The one real divergence: render approach

| Lane | Render | Discovery hookup |
|------|--------|-----------------|
| codex | server-side template + new `/run/{runID}` HTML page | run.detail + fan-out show |
| claude_code | server-side template, sibling panel page + link, keep `<pre>` | link from run page |
| gemini | **client-side** `app.js` rewrites the `<pre>` JSON into bubbles | extend `HandleStatus` |

**Chosen: server-side HTML rendering (codex + claude_code, 2 of 3).**

Rationale for rejecting gemini's client-side `app.js` path: it ships the raw
RPC JSON into the browser and re-implements escaping in JS, which is a weaker
D028/XSS posture than `html/template`'s automatic escaping. Server-side keeps
the curated-field guarantee at the render boundary and adds zero JS toolchain
coupling — consistent with leaving F36 closed. Gemini's instinct (smallest
build-system footprint) is honored; we just satisfy it server-side instead.

**Chosen integration shape: claude_code's low-risk variant over codex's full
page.** Keep the existing `renderRPCPage` `<pre>` run view intact
(`service.go:96`, `:281`) and add a *sibling* rendered chat page reachable by
link, rather than surgically rebuilding the run-detail page now. This avoids
the larger `/run/{runID}` fan-out (codex §Run-Detail), which is good UX but
more surface area than the tracer bullet needs. Codex's run-detail page is the
natural follow-up once the panel proves out.

**Discovery:** the rendered chat-page handler calls `interrogation.list` itself
at render time (codex/claude_code style) rather than mutating the shared
`HandleStatus` projection (gemini §4). The task constraint is "reuse the read
path; do not alter the data model" — calling the existing read keeps the widely
used `status` projection untouched while still surfacing the threads.

**Adopted from codex specifically — the run-ownership check.** Because
`interrogation.show` is keyed by `interrogation_id`, not `run_id`, the nested
route `GET /v1/runs/{runID}/interrogations/{id}` must verify the returned
`interrogation.run_id == runID` and return 404 on mismatch (codex §Read,
lines 47-54). This prevents a URL under one run from leaking another run's
thread. claude_code/gemini omit this; it is required.

## The one buildable plan

**Endpoints** (`routeRunGET` switch, `go/pkg/webservice/service.go:118`):

```go
case "interrogations":
    if len(parts) >= 3 && parts[2] != "" {
        // /v1/runs/{runID}/interrogations/{id}  — verify run ownership, 404 on mismatch
        h.renderOrShowInterrogation(w, r, runID, parts[2])
        return
    }
    // /v1/runs/{runID}/interrogations  — list for this run
    h.callAndWrite(w, r.Context(), "interrogation.list",
        map[string]any{"run_id": runID})
```

- JSON (default): `callAndWrite("interrogation.show", {"interrogation_id": id})`,
  then assert `run_id == runID` before writing.
- HTML chat (`?view=chat`): new `renderInterrogationPage` modeled on
  `renderRPCPage` (`service.go:281`), calling the new render helper.

**Render** (`go/pkg/webassets`):

- Add `RenderInterrogation(meta, turns)` beside `RenderPage`
  (`assets.go:24-57`) and a template `templates/interrogation.html`.
- Per turn: `<div class="turn turn-{question|answer}">` with speaker label from
  `kind` (`interrogation_question` → "Reviewer", `interrogation_answer` →
  "Target") and an auto-escaped `{{.body}}`. Order exactly as the read returns
  (no re-sort).
- Add chat-bubble styles to the already-embedded `static/base.css`
  (linked at `page.html:7`): left/right alignment by `turn-question` /
  `turn-answer`, `white-space: pre-wrap` on bodies.

## Smallest landable slice

**Slice 1 (tracer bullet, lands alone):** the `case "interrogations":` arm at
`service.go:118` wired to the two existing reads, **including the run-ownership
404 check**, plus the handler test. The thread is retrievable over HTTP GET
with zero render code and zero data-model risk — usable by curl/CI immediately.
(All three converge here; this is the unanimous first unit.)

**Slice 2 (delivers the DoD payoff):** `RenderInterrogation` +
`interrogation.html` + base.css bubbles + the sibling chat page (`?view=chat`)
+ a link from the run page + the render/escaping test. After this, an operator
opens this run's own design/build interrogation thread as a chat transcript.

**Deferred:** codex's full `/run/{runID}` fan-out page; gemini's `HandleStatus`
key; React/Vite (F36); live SSE turn updates; Markdown bodies; search/filter.

## Test plan (Go only, `httptest` + seeded `db.Runner`)

1. **Route → read wiring** (`webservice` test): seed one interrogation + two
   `queue_messages` turns (question/answer, `turn_index` 0/1). Assert
   `GET .../interrogations` → 200, count 1; `GET .../interrogations/{id}` →
   turns length 2, ordered, correct `kind`. (codex test 1; claude_code test 1;
   gemini §5.)
2. **Run-ownership 404** (codex test 3): a `show` whose `run_id` differs from
   the path `runID` returns 404.
3. **Render + escaping** (`webassets` test): `RenderInterrogation` with the
   two-turn fixture — question body precedes answer body, each in its
   `turn-question`/`turn-answer` container with the right speaker label; a
   body containing `<script>` is escaped (guards D028 + XSS in one assert).
   (claude_code test 2.)
4. Works with `AllowMutations=false` (codex test). Existing read-level coverage
   stays; don't duplicate the DB read test.

**Gate:** `go test ./pkg/webservice ./pkg/webassets ./pkg/reads`, then
`go test ./...`, then `make lint typecheck test`. No `make ui-check-bundle`
(F36 stays out of scope).
