# FALSIFIER - RFC 0143 design-v2 security/authz re-attack

author: falsifier-reviewer-003

## Verdict

The revision is materially safer than v1: it keeps the categorical admin-token refusal, retires the lane-readable `0600` reseal bearer, names a concrete reseal-time predicate, stops overclaiming in-place Codex MCP survival, and strengthens the epoch path. It still does not clear as written.

The core new mechanism is now a "daemon-owned supervisor/PTY session-tied channel." That is the right direction, but the spec treats it as already trusted and already routed. Current source says the helper control stream carries helper-originated lifecycle metadata and byte counts only, while agent output bytes stay out of the control channel. The revised spec never defines the trusted frame boundary that turns a no-token lane's PTY output or exit into a durable `session_unrecoverable_across_rotation` blocker or a scoped reseal mutation. That leaves F1 only nominally closed and reopens F2/F6 around a different surface.

## F1-F7 Revision Check

- **F1 - not genuinely resolved.** The holder names a typed exit code plus a structured helper/PTY line that becomes a durable blocker. But the exact non-MCP route is still not specified at the source boundary: no new helper event type, no parser, no blocker creation site, and no rule distinguishing helper-originated control events from child output.
- **F2 - resolved for the v1 bearer file, not yet for the replacement channel.** There is no lane-readable reseal token file, which resolves the specific same-uid bearer replay finding. The replacement channel still needs proof that a sibling same-uid process or the lane's own stdout cannot spoof a privileged reseal/control event.
- **F3 - mostly resolved.** The revised `resealInFlightJob` predicate is concrete in shape: live session, same job/lease/session, lease active or narrowly resealable, no recovery-generation change, expected artifact path, and accepted epoch. This is the right split-brain guard if the channel and grace semantics are nailed down.
- **F4 - partially resolved.** The route-alternate mechanics are now named. But the spec also says no lane-readable bearer exists. A public RPC `CapabilityReseal` alternate requires a bearer token to authorize; a daemon-internal supervisor projection bypasses that prelude. The principal and call path are still ambiguous.
- **F5 - partially resolved.** Scoping Slice B to the active lease window is the right correction. The added "bounded daemon-side reseal grace" has no number, source, or schema hook, while current `activeLeaseFor` rejects expired leases outright. That leaves GD-1b under-specified.
- **F6 - conditionally resolved.** The revision stops claiming in-place Codex MCP survival and shifts Codex sealing to the supervisor receiver path. That only holds if the receiver path is specified and trusted; today it is not.
- **F7 - mostly resolved.** Retiring the lane-writable scratch mirror and rejecting missing epoch on the supervised path directly addresses the v1 issue. The remaining ambiguity is that endpoint+epoch republish is still described as helper channel **or** daemon-owned file; the cleared spec should choose one concrete mechanism.

## C1 - The Supervisor/PTY Channel Is Asserted, Not Specified

**Claim attacked.** OQ4/F1 and OQ1/F2/F6 claim that a no-token lane can emit a typed exit code plus a structured helper/PTY line, the daemon-owned helper forwards it, and the daemon records either a durable `session_unrecoverable_across_rotation` blocker or a scoped reseal mutation. The same channel is the F2 replay defense and the F6 Codex receiver path.

**Concrete refutation.** The current helper protocol does not provide that trust boundary:

- `go/pkg/supervisor/helper_protocol.go:41-43` says `HelperControlEvent` is emitted on the helper's control-event stream and that payloads carry lifecycle metadata and byte counts only; agent output bytes stay out of the control channel.
- `go/pkg/supervisor/helper.go:120-127` says `RunHelper` does not open Postgres, call daemon RPC, inspect workflow state, publish artifacts, complete jobs, or acknowledge work; it only moves process bytes and reports control events.
- `go/pkg/supervisor/helper.go:386-405` reads PTY output and emits `progress` events with `bytes`, `total_bytes`, and optional `meaningful`; it does not parse child output content.
- `go/pkg/mutations/supervision.go:19-28` and `:217-234` whitelist the current helper event types and reject unknown event types. There is no `reseal_requested`, no `session_unrecoverable_across_rotation`, and no domain-specific helper event in the accepted set.
- `go/pkg/mutations/supervision.go:298-306` maps `agent_exited` to supervisor stopped with an exit code, and `:424-425` curates only `exit_code`, `error`, `cause`, `pty_log_path`, and `pty_log_bytes`. It does not create a blocker or route a recovery class.

So the proposed floor still has no concrete route from "the lane cannot authenticate" to "the daemon recorded a durable blocker." If the new mechanism is "parse a structured line printed by the child," then a lane can request a privileged mutation by printing bytes, and the same-uid replay problem moves from a bearer file to PTY/output spoofing unless the spec proves the PTY slave cannot be written by a sibling lane and that ordinary model output cannot forge the frame. If the mechanism is "helper emits an event based only on a typed exit code," then the spec must define the reserved exit-code enum, the helper mapping, the new helper event schema, and how the daemon turns that event into a blocker without trusting arbitrary stdout. If the mechanism is a separate helper-to-daemon socket/FIFO, the spec must name that descriptor, ownership, framing, and lifecycle.

**Strongest rebuttal for the holder.** The existing helper already has a daemon-owned control-event stream, `supervise.report` already drains helper JSONL, and adding one new helper event type plus a daemon handler is plausible. The design direction is safer than the v1 bearer file.

**Gap remains.** Yes, material. The revised spec must choose and specify the trusted frame boundary: event type, payload schema, source of truth, parser, blocker-creation mutation, and negative tests such as `TestUntrustedPTYLineCannotResealOrBlock`, `TestHelperExitCodeRecordsSessionUnrecoverableBlocker`, and GD-1. Without that, F1 is only nominally closed and the F2 non-bearer defense is unproven.

## C2 - `CapabilityReseal` Is Both Public Bearer Auth And Internal Projection

**Claim attacked.** F4/OQ3 says `MethodEntry.ResealAlternate` lets `interrogation.answer`, `work.complete`, and `artifact.publish` authorize `CapabilityReseal`; F2/OQ2 says no bearer reaches the lane and the daemon internally projects `AuthContext{Capability: CapabilityReseal, SessionID: s}` from the supervisor-proven path.

**Concrete refutation.** Those are two different designs, and the revised spec does not choose the principal.

Today the public RPC path authorizes `envelope.CapabilityToken` before dispatch (`go/pkg/rpc/server.go:107-111`). The in-memory authorizer denies missing tokens, splits the bearer, looks up a client record, and grants only if that record carries the required capability (`go/pkg/rpc/capability.go:182-230`). The PostgreSQL authorizer likewise calls `striatumd.authorize_capability` with `tokenID` and `secret` and returns the resolved capability only after a token-backed decision (`go/pkg/rpc/auth_pg.go:169-205`). If there is no reseal bearer, a public `CapabilityReseal` alternate is not reachable by the lane. If the daemon bypasses public RPC and directly constructs `AuthContext`, the route alternate and `PostgresAuthorizer` change are not the mechanism that admits the no-token path.

The payload contract is also missing. Existing handlers require method-specific parameters: `artifact.publish` requires `session_id`, `job_id`, `lease_id`, `kind`, `logical_name`, and `path`, with optional `body_base64` (`go/pkg/mutations/artifact.go:52-60`); it then reads from the active worktree or materializes `body_base64` (`artifact.go:150-170`). `work.complete` requires `session_id`, `job_id`, and `lease_id` (`go/pkg/mutations/lifecycle.go:1124-1129`). `interrogation.answer` requires `session_id`, `interrogation_id`, and `body` (`go/pkg/mutations/interrogation.go:217-221`). The revised spec never defines whether a `HelperEventResealRequested` carries these fields, whether artifact bodies are read from the active worktree or included over the helper channel, how multiple expected artifacts are ordered, or how author-line/front-matter validation failures route back to Option 4.

**Strongest rebuttal for the holder.** The daemon can add a private `resealInFlightJob` mutation that maps `supervisor_id` to `session_id`, constructs an internal `AuthContext`, then calls the same lower-level publish/complete routines against the active worktree. The public route alternate could still be useful for tests or for a future daemon-owned helper token that the lane cannot read.

**Gap remains.** Yes, material for buildability and authz review. The cleared spec must state whether `CapabilityReseal` is a real token-backed public capability, a daemon-internal capability marker only, or a daemon-owned helper principal. It must also define the helper reseal payload and the exact validation path for publish/complete/answer. Otherwise A1/A9 cannot be tested honestly: the test would either exercise a bearer path the spec says does not exist, or bypass the route alternate whose safety it claims.

## C3 - The Lease Grace Is Still A Hand-Wave

**Claim attacked.** F5 says Slice B survives only inside the active lease window, with a bounded daemon-side reseal grace that may extend the same lease row for the seal round-trip.

**Concrete refutation.** Current `activeLeaseFor` rejects an expired lease directly (`go/pkg/mutations/mutations.go:803-820`, expiration check at `:817-819`). The revision does not name the grace duration, the configuration/source of that duration, the database predicate that proves the lease was not already requeued, or the exact mutation that extends the lease. "Bounded" is not a falsifiable bound.

**Strongest rebuttal for the holder.** The intended shape is narrow: one same-lease extension, only before generation change, otherwise fall to the typed class. That is a safe shape if the actual bound is short and pinned.

**Gap remains.** Yes, smaller than C1/C2 but still a required F5 concretization. GD-1b needs a named value/source and a test that proves beyond-grace expiry routes to `session_unrecoverable_across_rotation`, never raw `lease_error` or stale-lease limbo.

## Required Revision To Clear

A clearing revision should preserve the non-bearer direction, but make it concrete:

1. Define the helper event/control protocol for `session_unrecoverable_across_rotation` and reseal requests, including who emits it and what untrusted bytes are ignored.
2. Prove same-uid sibling processes cannot spoof the channel, or explicitly limit the claim to helper-originated events that do not parse child stdout.
3. Choose the `CapabilityReseal` principal model: token-backed, daemon-internal marker, or daemon-owned helper principal.
4. Specify the reseal payload schema and reuse path for `artifact.publish`, `work.complete`, and `interrogation.answer` validation.
5. Pin the lease grace duration/source and its generation checks.

Until then, the security invariant is headed in the right direction, but the revised spec still has a material authz/routing hole at the exact no-token boundary this RFC exists to fix.