# RFC 0089 follow-up findings build handoff
author: implementer-codex-gpt-5.5-xhigh-002
date: 2026-05-29
status: complete
kind: handoff

## Files changed

- `go/pkg/supervisor/tmux_liveness.go`, `liveness.go`, `helper.go`,
  `helper_protocol.go`, and `pty.go`
- `go/pkg/mutations/mutations.go`, `supervision_control.go`
- `go/pkg/reads/supervision.go`, `doctor.go`
- `go/pkg/cli/routes/routes.go`, `go/pkg/cli/params/params.go`,
  `go/pkg/cli/dispatch/dispatch_test.go`
- `go/pkg/mcp/capabilities.go`
- Focused tests under `go/pkg/supervisor`, `go/pkg/mutations`,
  `go/pkg/reads`, `go/pkg/cli/routes`, and `go/pkg/mcp`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/reference/spec.md`
- `docs/reference/ubiquitous-language.md`
- `docs/reference/command-authority-matrix.md`

## Behavior landed

1. Typed tmux probe failures now flow from the liveness probe into metadata and
   read projections. The failure record carries `failure_class`, `detail`,
   optional `exit_code`, optional `errno`, optional `pane_process_alive`, and
   optional `observed_pane_pid`; probes still never read pane text.
2. Tmux liveness now has `healthy -> degraded -> lost` state. A transient
   `tmux_unavailable` stores degraded metadata and warning counters first, then
   marks the supervisor lost only after the configurable threshold
   `STRIATUM_TMUX_UNAVAILABLE_LOST_THRESHOLD` is reached. Default threshold: 3.
3. `supervise.rebridge` reattaches the helper-owned tmux delivery bridge in
   place, recreates a missing delivery FIFO, clears delivery degradation, and
   records `supervisor.rebridged`. It refuses when the pane is dead, missing,
   mismatched, or when tmux cannot be probed; it does not kill or respawn the
   pane.
4. `supervise.status`/dashboard projections expose `lane_backend`,
   `delivery_state`, and `pane_liveness` separately from lane attestation.
   Delivery degradation is still honored in both `tmux.delivery_liveness` and
   top-level `delivery_liveness`.
5. Doctor/status remediation now derives from the structured failure class or
   delivery reason, including concrete `striatum supervise rebridge --session-id
   <id>` guidance when delivery can be repaired in place.

## Deviation

The generated daemon contract file is outside this work packet's write scope.
To stay inside scope, `supervise.rebridge` is registered through runtime RPC
metadata, exposed through MCP tools/list by merging runtime-registered methods,
and made reachable from the CLI by a runtime `supervise rebridge` lookup
fallback. A later contract-maintenance packet should add the generated contract
entry and regenerate the generated route/method tables.

## Verification

Commands run:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

Results: passed. `gofmt -l .` produced no output, `go vet ./...` produced no
findings, and `go test ./...` passed for all Go packages.

## Remaining non-blocking findings

- Generated contract reconciliation remains because `contracts/daemon_methods.json`
  was outside scope.
- A future send-keys delivery path could remove the helper-owned attach bridge
  as the packet-delivery mechanism; this build makes the current bridge
  repairable and fail-closed.

## Live interrogation target

Reviewers should interrogate this live builder session:

```text
sess_ac356d97a74d3359a78e334dfd05902a
```
