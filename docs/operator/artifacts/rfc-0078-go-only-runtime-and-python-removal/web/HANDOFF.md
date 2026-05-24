---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/service_routes.py", "src/striatum/web/", "go/cmd/striatumd/", "go/pkg/reads/", "go/pkg/mutations/"]
---

# Go Web And Service Cutover Handoff
author: operator [self-declared: web-porter-codex-gpt-5-001]

## Finding

Python still owns the entire local service/web HTTP surface. Go serves daemon
RPC and MCP HTTP/SSE, but not the `/v1/*` compatibility API or
server-rendered web UI.

## Route Map

Most retained route data is already available through Go daemon reads and
mutations: status, why, doctor, dashboard, list artifacts, artifact content,
run lifecycle, recovery, worktree, supervision, cross-repo, and escalation.
The missing piece is the Go local web/service layer that translates local HTTP
requests to daemon RPC, enforces local security policy, serves static assets,
and renders HTML.

Routes needing explicit keep/retire decisions:

- `POST /v1/invoke`: port only if RFC 0012 argv compatibility remains.
- `/chat/*`: substantial Go subsystem or retire/move to optional plugin.
- `/dogfood/*`: port only if historical dogfood browsing remains current.
- `/view/*` and workflow browser: need Go safe filesystem helpers and
  Markdown/code rendering if retained.

## Tests Needed

Add Go HTTP tests for loopback binding, auth/origin, mutation gate, route
shape, CSP/static assets, SSE framing/cleanup, and path-safety. Add route
tests for retained run/job/artifact/workflow/escalation/recovery/worktree and
supervision surfaces. Add browser/component checks for retained React islands.

## Blocker

Deleting Python web/service is blocked on a Go package for local HTTP service
lifecycle, route security, template/static embedding, filesystem browsing,
and a decision on chat and dogfood route retention.
