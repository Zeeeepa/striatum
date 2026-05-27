# Design — RFC 0088 P1: interactive PTY lane + owned-PTY attestation (one of two parallel lanes)

Read docs/operator/workflows/rfc-0088-p1-pty-foundation/TASK.md and RFC 0088
(docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md, Decisions
1+2). Ground yourself in the real code: go/pkg/supervisor/pty.go (Launch, UsePTY,
LaunchResult.StdinWriter); go/pkg/mutations/supervision_control.go
(agentLoopModeSelfDriving ~line 30, the launch/cmd.Start path, supervisedEnv,
laneAttestation ~1680); go/pkg/agentloop/{loop.go,bootstrap.go,endpoint.go};
go/pkg/mutations/mutations.go:647 sessionLaneAttestation; go/pkg/mutations/claim.go:705
artifactAuthorIdentity(... attested bool ...); go/pkg/reads/supervision.go pidLiveWithStartToken.

You are one of two independent design lanes — do not coordinate. Produce DESIGN.md
in your lane dir covering: (1) how the bootstrap + per-turn prompts get SUBMITTED
through the PTY master to an interactive claude (submit key-sequence, TUI-readiness
detection, per-adapter structure even though only claude is proven in P1) — pick an
approach vs alternatives and justify; (2) how an owned-PTY persistent session earns
the lane byline (extend sessionLaneAttestation/artifactAuthorIdentity for a
long-lived pid + command-snapshot match) WITHOUT widening the forgery surface vs the
wrapper; (3) exact files + tests (fake/echo TUI binary for submit; attestation
derivation test). 2-3 alternatives; risks (submit-sequence fragility/version drift,
pid-reuse safety, command-snapshot drift across turns); rollout. No code. Stay in
your lane dir. Emit the submit-handoff packet when done.
