# RFC 0089 Phase 1 Build Handoff
author: implementer-codex-gpt-5.5-xhigh-001

## Summary

RFC 0089 Phase 1 is implemented for the Go runtime. Tmux-backed supervised
lanes now record tmux session/window/pane identity at launch and treat the pane
PID as the supervised lane identity. The `tmux attach-session` process remains
the PTY byte-delivery handle, but it is metadata-only for liveness.

Attach-client exit now emits `attach_client_exited` and moves the supervisor to
`detached` instead of `stopped`/`lost` when the pane probe still shows the lane
alive or unverifiable. `supervise.send`, `supervise.status`,
`supervise.reattach_status`, status/dashboard read projections, doctor, recovery
process reconcile, and `supervise.stop` all route tmux-backed rows through the
tmux session/pane liveness path.

## Files Changed

- `go/pkg/supervisor/`: tmux identity capture, tmux liveness probe, shared PID
  start-token helper, helper attach-exit event handling, liveness heartbeat
  routing, and focused unit/integration tests.
- `go/pkg/mutations/`: delivery reconciliation, helper event recording,
  stop behavior, recovery process reconcile, and tests.
- `go/pkg/reads/`: supervise/status/dashboard/doctor tmux liveness projections,
  reattach classification, D028 metadata allowlist coverage, and tests.
- `go/pkg/db/supervisor_pointers.go`: pointer metadata/start-token round-trip.
- `go/cmd/striatum/main_test.go`: clears inherited `STRIATUM_REPOSITORY_ID` in
  daemon-route tests so supervised lane env does not break CLI test isolation.
- Docs: `docs/reference/spec.md`, `docs/decisions/decision-log.md`,
  `docs/reference/ubiquitous-language.md`,
  `docs/reference/command-authority-matrix.md`, and RFC 0089 status.

## Remaining Work

No Phase 1 blocker remains. Phase 2 still needs a first-class reattach path for
restoring the daemon-owned PTY byte-delivery handle after an attach client exits.
Pane text and PTY logs remain private diagnostics only; no transcript/export
path was added.

Universal tmux monitoring can now be enabled by workflow/default configuration
alone for the liveness model. No further liveness-code change is required, but
operators should account for the Phase 2 reattach gap if an attach client exits.

## Verification

Run from `go/`:

```bash
gofmt -l .
go vet ./...
go test ./...
```

Results: all three commands passed. `gofmt -l .` and `go vet ./...` produced no
output; `go test ./...` completed successfully across all Go packages.
