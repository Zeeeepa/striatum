# RFC 0089 final build handoff
author: implementer-codex-gpt-5.5-xhigh-001
date: 2026-05-28
status: complete

## Files changed in this final pass

- `docs/operator/workflows/rfc-0089-tmux-final-build-review/artifacts/build/HANDOFF.md`

No code or product-doc patch was needed in this final pass. The current working
tree already contains the final RFC 0089 implementation and focused tests.

## Final behavior verified

- `session.register` defaults to workflow lane capabilities when the caller
  omits explicit capabilities, preserving explicit capability overrides.
- A helper-owned attach bridge exit with a live tmux pane keeps pane liveness
  attached and lane attestation eligible, while persisting
  `delivery_liveness: {class: "degraded", healthy: false,
  reason: "attach_client_exited"}`.
- `supervise.send` refuses delivery when delivery liveness is degraded, both
  under `tmux.delivery_liveness` and top-level `delivery_liveness`, without
  recording a packet-delivered event.
- Missing FIFO readers mark delivery degraded with reason
  `stdin_reader_missing` using nonblocking FIFO open semantics.
- `supervise.status`, `status`, `dashboard`, and `dashboard_all` projections
  surface delivery liveness separately from tmux pane liveness and lane
  attestation.
- Tmux liveness probing uses session/pane identity and formatted tmux fields,
  not pane text. Report/status sanitizers and guard tests keep raw pane text,
  stdout, stderr, and transcripts out of daemon events and read projections.

Primary implementation locations already present in the tree:

- `go/pkg/mutations/lifecycle.go` and `go/pkg/mutations/lifecycle_test.go`
- `go/pkg/supervisor/helper.go`, `helper_protocol.go`, `pty.go`,
  `tmux_liveness.go`, and related supervisor tests
- `go/pkg/mutations/supervision.go`,
  `go/pkg/mutations/supervision_control.go`, and related mutation tests
- `go/pkg/reads/supervision.go`, `dashboard.go`, `dashboard_all.go`,
  `status.go`, `doctor.go`, and related read tests

## Verification

Command run:

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

Result: passed. `gofmt -l .` produced no output, `go vet ./...` produced no
findings, and `go test ./...` passed for all Go packages, including
`go/pkg/mutations`, `go/pkg/reads`, and `go/pkg/supervisor`.

## Remaining non-blocking findings

- RFC 0089 still defers replacing the attach-PTY delivery bridge with a
  `tmux send-keys` or rebridge design. The current build makes that residual
  risk visible and refuses delivery instead of presenting a false healthy path.
- Phase 2 operator polish remains useful: compact dashboard attach-command
  rendering, clearer remediation hints for terminal tmux classes, and visible
  consecutive `tmux_unavailable` probe counters.

## Live interrogation target

Reviewers should interrogate this live builder session:

```text
sess_ac823cc767750f0857cd4a9fa2ced765
```
