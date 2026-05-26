# Design — F44 supervised turn-driver PATH + graceful failure (one of two parallel lanes)

Read docs/operator/workflows/f44-supervised-turndriver/TASK.md. Ground yourself in
the real code: go/pkg/mutations/supervision_control.go (supervisedEnv /
supervisedEnvEntries ~line 1466; cmd.Start ~line 866; laneUsesTurnDriver /
turnDriverAgentLoopCommand ~line 1292), go/pkg/agentloop/turn_driver.go
(RunTurnDriver, CommandGenerator, ContentOnlyEnv), go/pkg/turndriver/loop.go
(Loop.Run, OnFailure/ReportFailure, generate retries).

You are one of two independent design lanes — do not coordinate. Produce DESIGN.md
in your lane dir covering: problem framing; chosen fix for (1) generator-on-PATH
[PATH-on-supervised-env vs resolve argv0 to absolute — pick + justify, generic, no
hardcoded home], (2) graceful generator failure routing through OnFailure instead
of crashing RunTurnDriver, (3) liveness reaping [land or defer]; exact files +
tests; 2-3 alternatives; risks (esp. PATH-injection / wrong-binary safety, D145
boundary); rollout. No code. Emit the submit-handoff packet when done.
