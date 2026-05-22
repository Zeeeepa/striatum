---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0075 / MCP Cutover Slice Summary
author: operator [self-declared: rfc0075-finalize-codex]

## Accepted Slice

The accepted first slice landed `session.report` as the structured MCP path for
pre-work session state. Agents can now report `ready`, `heartbeat`, `question`,
and `escalate` states before a work packet exists, without making terminal
text, tmux pane output, or provider transcripts authoritative workflow state.

The method is registered in the daemon method contract, the generated Go RPC
registry, the Python daemon-method mirror, and the authority matrix. It is
claim-capability gated, single-repository scoped, exposed through MCP
`tools/list`, and records metadata-only `session.reported` events.

The agent-loop bootstrap prompt now tells MCP agents to use `session.report`
for pre-`work.await_packet` questions or blockers instead of waiting silently in
terminal text. `docs/MCP.md` documents the method as the structured pre-work
reporting path.

## Cutover Movement

This slice moves one CLI-retirement prerequisite from open to landed:
pre-packet readiness, question, heartbeat, and escalation reporting now has a
native MCP daemon method. The legacy CLI workflow-control verbs remain in use
for operation and recovery. Full CLI retirement is not complete.

The run also exposed and repaired two cutover-support bugs:

- `review.override` can now recover a completed review that published an
  artifact but missed its verdict, returning `status: recovered` and enqueueing
  accepted-review downstream work.
- The Go dashboard no longer selects the nonexistent
  `striatumd.sessions.lane_attestation` column; it derives attestation from
  attached process supervisors.

The agent-loop MCP endpoint unit test was isolated from live daemon runtime
endpoint files so `go test ./pkg/agentloop` no longer depends on the operator's
current Striatum daemon.

## Verification

Commands run in this slice:

- `cd go && go test ./pkg/mutations ./pkg/reads ./pkg/agentloop`
- `cd go && go test ./...`
- `make daemon-go-install`
- `striatum override-verdict ... --verdict accept_with_findings --json`
- `striatum status --run-id run_01664d115a26e64f92368681579ee97c --json`

The live RFC 0075 run was recovered through the new supported
`review.override` path. `review_regression` now has an
`accept_with_findings` verdict, and `finalize_cutover` advanced from blocked to
queued.

## Remaining RFC 0075 Work

RFC 0075 remains proposed beyond this first slice. The next implementation
work should persist daemon-owned MCP activity timestamps and classify liveness
deadlines for discovery, await-packet, ack, heartbeat, question, and escalation
stalls.

The tmux-observable session work is still pending. The daemon does not yet
persist tmux session, window, pane, attach command, or liveness metadata as
operator status fields. That work must keep pane contents and transcripts out
of authoritative workflow state.

Other remaining cutover items:

- status/current-brief/UI surfaces for tmux metadata and stall classification;
- fake-agent tests for discovery, await, ack, heartbeat, question, and
  escalation stalls;
- MCP/UI parity for review evidence and human override workflows;
- CLI workflow-control retirement only after MCP/UI parity exists.

## Next Smallest Slice

Implement MCP activity timestamp persistence and status projection first. It is
smaller than full tmux supervision, directly supports RFC 0075 liveness
classification, and gives later tmux metadata work a daemon-owned protocol
timeline to correlate with process and pane identity.
