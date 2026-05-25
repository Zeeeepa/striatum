# Design Proposal: Interrogation-Log Chat UI

**Lane:** Gemini (Design)
**Run ID:** run_f9753a8c54ca534f79aea95e08bae53e
**Date:** 2026-05-25

## 1. Goal
Render interrogation Q&A threads as a chat-style transcript in the Striatum web UI.

## 2. Read/Endpoint Path
We will reuse the existing `reads.HandleInterrogationShow` and `HandleInterrogationList` methods. To enable direct browser access and simpler UI fetching, we will add dedicated HTTP GET routes.

**Integration Point:** `go/pkg/webservice/service.go` within `routeRunGET` (around line 120).

**Routes:**
- `GET /v1/runs/{runID}/interrogations`
  - Calls `interrogation.list`.
  - Returns a list of all interrogations associated with the run.
- `GET /v1/runs/{runID}/interrogations/{id}`
  - Calls `interrogation.show`.
  - Returns the full interrogation metadata and all ordered turns.

## 3. Render Approach: Progressive Enhancement
Given the **F36 gap** (React islands built but not served by Go), we propose a **Vanilla JS enhancement** of the existing Go-served pages. This is the smallest landable slice that avoids a major build-system refactor.

**Strategy:**
- **Go Template:** Continue using `go/pkg/webassets/templates/page.html` which renders the RPC response as a JSON string in a `<pre>` tag.
- **Frontend Transformation:** Update `go/pkg/webassets/static/app.js` to:
  1. Detect if the page contains interrogation data (check `title` or payload structure).
  2. If found, hide the `<pre>` tag.
  3. Dynamically create a chat-bubble UI by iterating over `turns`.
  4. Distinguish between `interrogation_question` and `interrogation_answer` using CSS classes.

**F36 Decision:** Do not close F36 (React embed) in this slice. By keeping the logic in `app.js`, we remain compatible with the eventually-served React islands while delivering immediate value.

## 4. Run-Detail Integration
Operators should discover interrogations from the run-history view.

**Integration Point:** `go/pkg/reads/status.go:HandleStatus`.
- When a `run_id` is provided, the `status` response will include a new `interrogations` key containing the output of `HandleInterrogationList`.
- This ensures the existing `/v1/runs/{runID}` view (which renders the `status` response) automatically shows available interrogations.

## 5. Test Plan
- **Go Handler Test:** Extend `go/pkg/webservice/service_test.go` to verify the new `/v1/runs/{runID}/interrogations` routes.
- **Read Logic Test:** Add `go/pkg/reads/interrogation_test.go` to assert that `HandleInterrogationShow` correctly orders turns by `turn_index` and correctly projects fields.
- **Manual Verification:** Confirm `curl -H "Authorization: Bearer ..." http://localhost:PORT/v1/runs/RUN_ID/interrogations/INTG_ID` returns the expected transcript.

## 6. Smallest Landable Slice
1. Add the HTTP GET routes in `service.go`.
2. Update `HandleStatus` to include the `interrogations` list for a run.
3. Enhance `app.js` and `base.css` with minimal chat rendering logic.
