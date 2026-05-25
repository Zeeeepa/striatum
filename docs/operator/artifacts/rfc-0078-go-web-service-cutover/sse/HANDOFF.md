---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# SSE Event Stream Handoff
author: operator [self-declared: sse-porter-codex-gpt-5-002]

## Changed Files

- `go/pkg/websse/sse.go`
- `go/pkg/websse/sse_test.go`
- `go/pkg/webservice/service.go`
- `go/cmd/striatumd/web_routes_test.go`

## Behavior

`go/pkg/websse` parses `Last-Event-ID` and `?since=`, writes named SSE frames,
emits keepalive comments, and exits on client cancellation. The webservice
route `GET /v1/runs/<run_id>/events` is backed by daemon RPC `run.events`,
using `since_event_id` and `limit`; it emits `striatum.event`,
`striatum.run_terminal`, or `striatum.error` frames.

The stream does not inspect terminal output, tmux panes, transcripts, marker
files, or process-local Python state.

## Validation

- `go test ./pkg/websse ./pkg/webservice` passed as part of the focused web
  package validation.
- `go test ./cmd/striatumd -run 'TestWebServiceAdapter|TestListenMCPHTTPRejectsNonLoopback'` passed.

## Remaining Parity Gaps

Python's service had an SSE concurrency counter per run. The Go service now
implements the same 32-stream-per-run guard. Long-poll/browser reconnection
behavior beyond `Last-Event-ID` parsing remains covered only by unit tests, not
browser automation.
