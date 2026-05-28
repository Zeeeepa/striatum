# RFC 0089 Phase 1 Build Handoff
author: implementer-codex-gpt-5.5-xhigh-001
status: complete
date: 2026-05-28

## What landed

- `go/pkg/supervisor/pty.go`: tmux-backed PTY launch now records pane identity as the supervised identity, inserts `--` before the lane command, keeps attach-client pid as diagnostics, and sets `remain-on-exit` for clearer dead-pane classification.
- `go/pkg/supervisor/helper.go`: attach-bridge exit now probes tmux liveness. `tmux_ok` and `tmux_unavailable` emit `attach_client_exited`; terminal tmux classes emit `agent_exited` with the class as `cause`.
- `go/pkg/mutations/supervision.go`: `supervise.report` keeps tmux-backed supervisors `attached` when an attach bridge exits while the pane is still live, refreshes heartbeat, and records `attach_client_last_exit` in pointer metadata. Legacy non-tmux/missing-liveness reports still detach.
- `go/pkg/mutations/supervision_control.go`: delivery lost events include `tmux_liveness`, and `supervise.stop` no longer sends an unconditional SIGTERM to the diagnostic attach-client pid after `tmux kill-session`.
- `go/pkg/supervisor/liveness.go`: corrupt tmux-backed metadata is marked lost with `tmux_metadata_corrupt` instead of silently falling back to plain PID liveness.
- `go/pkg/reads/{dashboard.go,dashboard_all.go,supervision.go}`: dashboard/status projections now derive lane attestation from the same tmux/pid liveness probe and carry tmux failure classes through `lane_attestation_reason`.
- Docs updated inside write scope: `docs/reference/spec.md`, `docs/reference/ubiquitous-language.md`, and `docs/reference/command-authority-matrix.md`.

## Tests added

- Helper attach-exit tests for live pane vs dead pane behavior.
- `supervise.report` regression for tmux `attach_client_exited` preserving `attached`.
- Liveness regression for malformed tmux metadata.
- Existing tmux probe/status/dashboard tests continue to cover `tmux_*` classes and D028 no-pane-text guardrails.

## Verification

- `cd go && go test ./pkg/supervisor` -> pass
- `cd go && go test ./pkg/mutations` -> pass
- `cd go && go test ./pkg/reads` -> pass
- `cd go && gofmt -l .` -> pass, no output
- `cd go && go vet ./...` -> pass
- `cd go && go test ./...` -> pass

## Remaining / blocked

- `CHANGELOG.md` was not updated because it is outside this work packet's write scope.
- `tmux send-keys` packet delivery and helper crash reattachment remain explicitly out of Phase 1.
- Universal tmux monitoring is now a configuration/default flip plus tests for the RFC 0088 agent-loop lane defaults, not a code redesign. The Phase 1 substrate is in place; the remaining risk is that byte delivery still rides the helper-owned attach PTY until the deferred send-keys phase.
