# Build review codex interrogation chat

author: reviewer-codex-gpt-5.5-xhigh-001

Interrogation: `intg_5c90667022e4c7e258f3bd2116a41059`
Interrogator session: `sess_b516b3824e7c042b9508b4f9bc558bfe`
Target session: `sess_be356331bb2d0e88ef78541a9d2cfad2`

## Turn 0 - question

Message: `msg_2ebfc7138560ef4b5c137d60143e6ca4`

Threat-model review question: enumerate the concrete code paths changed for
RFC 0089 Phase 1, and for each trust boundary explain how the implementation
prevents tmux pane text, attach-client lifetime, stale pid reuse, or missing
tmux from becoming authoritative workflow state. Also state which targeted
tests verify those properties.

## Turn 1 - answer

Message: `msg_bebb69cfaa014914b8bb03b7175793d1`

The builder listed these changed paths and claimed test coverage:

- `go/pkg/supervisor/pty.go::launchPTY`: lane identity uses the tmux pane pid;
  attach client pid is diagnostic metadata only; tmux metadata records
  session/window/pane/pane pid/start token/attach command; optional tmux falls
  back to plain PTY while required tmux fails closed.
- `go/pkg/supervisor/tmux_liveness.go`: probes `has-session` and tmux format
  fields, not pane text; classifies missing session, missing pane, dead pane,
  pid mismatch, tmux unavailable, and ok states.
- `go/pkg/supervisor/helper.go::RunHelper`: helper-owned attach bridge exit
  emits `attach_client_exited` when the pane still probes live, and terminal
  tmux classes emit `agent_exited`.
- `go/pkg/mutations/supervision.go`: tmux-backed `attach_client_exited` reports
  keep the row attached when liveness says `tmux_ok` or `tmux_unavailable`;
  structured metadata is recorded without terminal bytes.
- `go/pkg/mutations/supervision_control.go`: delivery reconciliation probes
  tmux liveness; terminal classes mark lost; `tmux_unavailable` refuses
  delivery without marking lost on a single probe failure; stop kills the tmux
  session.
- `go/pkg/supervisor/liveness.go`: corrupt tmux metadata is marked
  `tmux_metadata_corrupt` rather than falling back to plain pid liveness.
- `go/pkg/reads/{supervision,status,dashboard,dashboard_all}.go`: read
  projections derive attestation from the probe and allowlist tmux metadata.

The builder also reported that `gofmt -l .`, `go vet ./...`, and
`go test ./...` passed under `go/`.

## Turn 2 - question

Message: `msg_6d7912388c52f46a39b072fffe229e2b`

Follow-up: RFC 0089 acceptance says attach-client exit must leave the
tmux-backed lane attached/attested and a live RFC 0088 agent-loop lane must
still complete work through MCP while monitored. If the helper-owned attach
bridge exits and delivery continuity is degraded until rebridge/send-keys
exists, what exact user-visible behavior remains broken, and should that be
treated as a blocking Phase 1 finding or a documented follow-up?

## Turn 3 - answer

Message: `msg_f12d5464cb7140e99707506a65162161`

The builder answered that ordinary operator attach/detach is protected, but the
helper-owned attach bridge path still has a delivery-continuity gap. Status or
dashboard can report the lane as attached and attested because the tmux pane is
live, while the daemon's FIFO-to-pane delivery handle may be gone. Autonomous
MCP calls from an already bootstrapped agent may continue, but prompt delivery
through `supervise.send` can block or fail.

The builder characterized this as blocking Phase 1 unless the acceptance scope
is explicitly narrowed to pane liveness and operator attach observability. The
recommended smaller fix is to mark helper-owned bridge exit as
delivery-degraded, detached, or lost until a rebridge/send-keys implementation
lands.
