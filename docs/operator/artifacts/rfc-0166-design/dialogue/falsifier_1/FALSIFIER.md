# FALSIFIER - RFC 0166 AND safety depends on unstated local-work keepalive coverage

author: falsifier-reviewer-003

## Challenge

Part 2's AND predicate still false-kills a legitimately slow, single-final-seal lane unless the SPEC makes RFC 0140-style local-work keepalives a build-bearing precondition of arming the destructive rung. The holder proves safety for lanes that are inside a daemon-visible tool call or that keep the tool timeline fresh. It does not prove safety for a healthy lane that is doing long local/model work after an earlier tool call, emits PTY output or plain lease heartbeats, but does not issue `work.heartbeat` with `local_work=true` before `ToolProgressSeconds` expires.

That omission matters because the proposed predicate consumes the exact #324 tool-progress clock. In current source, that clock ignores PTY/local output and plain `work.heartbeat`; it is refreshed by `last_tool_call_started_at` / `last_tool_call_finished_at`, and only `local_work=true` heartbeats intentionally advance `last_tool_call_finished_at` during long tool-call-less local work. Without that cooperative keepalive, the AND collapses into an effective finite wall-clock cap for never-sealed jobs that have any prior tool-call history.

## Claim Challenged

Holder Claim 2.1 defines:

```go
sealedSilenceBreached := SealedSilenceSeconds > 0
                         && missed(floor, SealedSilenceSeconds, now)
toolWedged             := sessionliveness.ToolProgressWedged(activity, policy, now)
fire                   := sealedSilenceBreached && toolWedged
```

Holder Claim 2.2 and the hard-core proof then treat this as excluding #145 CASE-2 false-kill for the single-final-seal lane. The proof is narrower than the claim: it protects lanes that are tool-fresh, inside a daemon-visible tool call, or otherwise emit a signal that refreshes the tool-progress base. It does not make that signal mandatory for all legitimate long work before the rung can close/requeue the owner.

## Counterexample

Construct a single-final-seal `repo_write` lane whose only durable seal is its terminal required artifact plus `work.complete`.

1. `T0`: the lane claims and acks work. No artifacts or verdicts exist, so `jobSealedProgressAt` is null and the sealed clock floor is the current lease acquisition / job start.
2. `T0+2m`: the lane has issued normal MCP calls for discovery, ack, file reads, or one small source query. The tool-call history precondition is now satisfied.
3. `T0+3m`: it enters a legitimate long planning/model-generation interval or a long local command path that is not represented as an in-flight Striatum `tools/call`. The supervised process is alive and may emit PTY output; a plain heartbeat may keep the lease warm. It has not sealed anything because the job has no intermediate deliverable.
4. `T0+20m`: policy has `ToolProgressSeconds = 600`; choose any armed `SealedSilenceSeconds <= 1200`.

At `T0+20m`, `sealedSilenceBreached` is true because there is no sealed event and the current-lease floor is older than the sealed-silence budget. `toolWedged` is also true because the lane has tool-call history, is not currently inside a daemon-visible tool call, and its last tool-call start/finish is older than `ToolProgressSeconds`. The live PTY output and plain heartbeat do not refresh that base. Therefore `fire == true` while the lane is healthy and still working toward its only terminal seal.

This is not a syntactic OR bug, but it is an effective OR-safety failure for never-sealed jobs: once the lane has prior tool history and no local-work keepalive, both clocks age from early in the job and the destructive rung becomes a finite wall-clock deadline on a legitimate single-final-seal task. That is exactly the #145 CASE-2 shape the RFC says must not regress.

## Evidence

The source predicate is explicit. `go/pkg/sessionliveness/liveness.go` defines `ToolProgressSeconds` as the window after a lane with recorded tool-call history makes no tool-call progress. `toolProgressWedged` returns false only when the policy is disabled, the tool-progress base is nil, or the lane is currently inside an in-flight tool call; otherwise it ages the later of `last_tool_call_started_at` and `last_tool_call_finished_at`.

The local-work mitigation exists, but it is a separate cooperative contract. `go/pkg/mutations/lifecycle.go` says a plain `work.heartbeat` only stamps `last_work_heartbeat_at`, keeping the lease rung satisfied while not advancing the #324 tool timeline. Only `work.heartbeat` with `local_work=true` also advances `last_tool_call_finished_at`, so the lane never crosses `ToolProgressSeconds` during honest long local work. `TestHeartbeatLocalWorkAdvancesToolProgress` confirms exactly that: plain heartbeat must not move `last_tool_call_finished_at`; a `local_work` heartbeat must.

RFC 0140 also names the residual: client-side keepalive works but relies on cooperative clients, and an adapter that does not implement it still decays. That is acceptable for RFC 0140's specific attestation fix only if the product accepts the residual. RFC 0166's holder SPEC instead states the stronger claim that the AND rung closes #576 without reintroducing #145 mid-work kill. That stronger claim needs the keepalive dependency as part of the P0 arming contract.

## Strongest Rebuttal

The holder has strong mitigations available:

- If the long work is a daemon-visible in-flight `tools/call`, `inToolCall` keeps `ToolProgressWedged` false for the #324 half.
- If the agent loop emits `work.heartbeat` with `local_work=true` every less than `ToolProgressSeconds`, the tool-progress base stays fresh and the AND cannot fire.
- Current lane instructions tell agents to heartbeat during long local work, and RFC 0140 added the server seam for that heartbeat to advance the tool-progress base.
- Shadow-first arming means a bad threshold does not have to become destructive on day one.

Those mitigations do not clear the claim as written. They prove a conditional safety property: no false-kill for cooperative/instrumented long work. They do not prove the unconditional Part 2 property for a legitimate lane whose adapter, model turn, or local command path fails to emit the specific `local_work=true` signal. Threshold calibration is also not a proof; any finite `ToolProgressSeconds` and armed `SealedSilenceSeconds` can be exceeded by a healthy long turn.

## Unanswered Gap / Required Revision

The gate should not clear Part 2 until the SPEC does one of the following:

1. make `local_work=true` keepalive coverage part of RFC 0166 P0 arming, with build-bearing adapter-conformance tests that every supported lane mode refreshes the tool-progress base during long local/model work before destructive `wedged_no_sealed_progress` action is enabled; or
2. narrow the safety claim to say the destructive rung is safe only for tool-fresh, in-flight-tool, or local-work-heartbeating lanes, and explicitly accept that non-cooperative long local work can be requeued mid-work; or
3. keep P0 advisory-only until a false-kill reprieve covers the live-PID / fresh-PTY / stale-tool / never-sealed case without relying on periodic client cooperation.

The required falsification test is concrete: seed a never-sealed single-final-seal job with current lease floor older than `SealedSilenceSeconds`, stale prior tool-call history older than `ToolProgressSeconds`, fresh PTY or plain lease heartbeat, and a live supervised PID, but no `local_work=true` heartbeat. If the new rung closes or requeues that owner, the AND safety claim is false.
