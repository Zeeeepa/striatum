# Design - RFC 0089 tmux helper redesign

Read `TASK.md`, `docs/rfcs/0089-tmux-backed-lane-monitoring.md`, and the
current supervisor code before designing. Focus on:

- `go/pkg/supervisor/pty.go`
- `go/pkg/supervisor/helper_protocol.go`
- `go/pkg/mutations/supervision_control.go`
- status/doctor/recovery read paths that surface supervisor liveness
- existing supervisor and helper tests

You are one of three independent design lanes. Do not read the other design
lanes. Write `DESIGN.md` only in your lane directory.

Produce a concrete implementation design for Phase 1: replace
attach-as-liveness with session/pane liveness. Name exact data captured at
launch, the liveness probe API, failure classes, where status/recovery/doctor
call the probe, and how `supervise.stop` terminates the tmux lane. Include tests
for attach-client exit and missing/dead/mismatched tmux panes. Keep D028 intact:
tmux text is never workflow state or provenance.

Do not implement code. Publish the handoff artifact and complete.
