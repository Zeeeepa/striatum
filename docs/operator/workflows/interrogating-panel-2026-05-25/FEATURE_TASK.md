# Feature task — Interrogation log in the workflow-history UI (chat view)

This is the operator-supplied task for the iterated-interrogating-panel run. The
design lanes each produce an independent design; the synthesis reconciles them;
the build lane implements the chosen design.

## Goal

Render a run's interrogation Q&A thread as a **chat-style transcript** in the
Striatum web UI's workflow/run-history view. An operator looking at a run should
be able to read the interrogation between a reviewer and the interrogated
(builder/synthesis) session as a back-and-forth chat: each turn shows who spoke
(reviewer question vs. target answer), the body, and ordering.

## What already exists (from the operator briefs — read these)

- `docs/operator/workflows/interrogating-panel-2026-05-25/briefs/interrogation-data.md`
  — `reads.HandleInterrogationShow` (`go/pkg/reads/interrogation.go:36`) already
  returns the interrogation metadata plus all turns ordered by `turn_index`, each
  turn exposing `kind` (`interrogation_question` / `interrogation_answer`),
  `body`, and `turn_index`. **No new migration or RPC is needed** — this is the
  chat data source. `interrogation.list` enumerates a run's interrogations.
- `docs/operator/workflows/interrogating-panel-2026-05-25/briefs/web-assets.md`
  — the Go web service embeds only `go/pkg/webassets/static/*`; the F36 gap is a
  path mismatch (Vite `outDir` vs the embed glob). The run-detail HTML is served
  by `routeRunGET`. Adding a chat view means either a small server-rendered
  section or extending the served assets; the missing piece is an HTTP GET route
  exposing `interrogation.show` / `interrogation.list` to the page.

## Constraints (product boundary — AGENTS.md)

- Local-first, daemon-owned PostgreSQL is authoritative; no hosted services,
  telemetry, transcript capture, or external persistence.
- Interrogation turns are curated records (D028) — never provider stdout/stderr.
- Reuse the existing read path (`HandleInterrogationShow` / `interrogation.list`);
  do not add a parallel data store or a new migration unless strictly required.
- Keep the change within the web layer + a read endpoint; do not alter the
  interrogation mutation/data model.

## Deliverable shape (per design lane)

A design proposal covering: (1) the read/endpoint path the UI calls (reuse vs new
HTTP GET route in `routeRunGET`); (2) how the chat transcript is rendered
(server-rendered HTML section vs. a JS island, given the F36 embed reality and
whether to close F36 now); (3) the run-detail integration point; (4) test plan
(a Go handler/read test + a rendering assertion); (5) the smallest landable
slice. Cite `file:line` for every integration point.

## Definition of done (for the build phase)

- An operator viewing a run with interrogations sees the Q&A rendered as an
  ordered chat transcript (question/answer turns, speaker, body).
- Backed by the existing interrogation read path; a Go test asserts the
  endpoint/render against a run that has interrogation turns.
- `make lint typecheck test` and `go test ./...` green.
- The payoff: this run's OWN interrogation thread (from the design/build panels)
  is viewable as chat.
