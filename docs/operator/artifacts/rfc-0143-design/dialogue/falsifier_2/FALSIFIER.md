# FALSIFIER - RFC 0143 lifecycle and ergonomics challenge

author: falsifier-reviewer-002

## Material falsification

The holder proposal is directionally careful about not exposing the bootstrap admin token, but the Slice B survival path is not yet a buildable, falsification-cleared spec. The strongest remaining gap is that the proposed `CapabilityReseal` is described as a narrow new authority, while the current daemon authorization prelude, lease lifecycle, adapter reconnect path, and boot-epoch mirror all still require design decisions that the proposal treats as already solved.

My verdict: Option 4 as a loud floor is worth keeping, but Option 2 plus endpoint/epoch republish does not clear the gate until the gaps below are resolved with explicit mechanisms and tests.

## Challenge 1 - `CapabilityReseal` cannot reach the named methods under the current auth prelude

**Claim attacked.** OQ2 says the new credential carries only `CapabilityReseal`, not general `claim` or `write`, yet it authorizes `work.complete`, `artifact.publish`, and `interrogation.answer` for the session's current in-flight job. The proposal calls this structurally narrower than the normal session token.

**Concrete refutation.** The current RPC server authorizes before any handler-specific scoping runs. `go/pkg/rpc/server.go:91-112` looks up a single `MethodRegistry` entry, calls `Authorizer.Authorize(entry.RequiredCapability, ...)`, and calls `RequireAllowed` before routing into the handler. The authorizer contract is single-required-capability (`go/pkg/rpc/capability.go:111-113`), and the memory authorizer refuses unless the token has exactly that required capability (`go/pkg/rpc/capability.go:211-216`). The current registry requires `write` for `interrogation.answer`, `work.complete`, and `artifact.publish` (`go/pkg/rpc/registry_methods.go:10`, `:89-90`). A token carrying only `CapabilityReseal` never reaches the handler; it fails at the prelude with `capability_missing`.

The proposal names "new `rpc.CapabilityReseal` + handler scoping," but handler scoping is too late unless the method registry and authorizer are changed to support an OR capability, a route-specific alternate capability, or new reseal-only methods. If the implementation instead grants `write` so the prelude passes, it violates the holder's own OQ2 claim that the reseal token is not general `write` and cannot publish into another job except by handler checks.

**Best holder rebuttal.** The phrase "new capability + the authorizers" could be read as intending to change the RPC prelude, generated method contract, and authority matrix to allow `write OR reseal` on exactly those routes.

**Gap that remains.** That is not specified, and it is a load-bearing authority change. The cleared spec must say exactly whether `MethodEntry.RequiredCapability` becomes a set, whether `PostgresAuthorizer` projects alternate grants, how `AuthContext.Capability` records the selected grant, and which generated contracts/docs/tests change. Otherwise A1 is unbuildable or collapses back to `write`.

**Refuting test.** `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`: mint a token with `CapabilityReseal` only. `artifact.publish` and `work.complete` for the bound in-flight job must pass the auth prelude and then enforce job scope; `repo.write`, `work.block`, `work.send_message`, `conversation.say`, and publishing to a foreign job must fail before or inside the handler. A plain `CapabilityWrite` grant must not be present.

## Challenge 2 - the survival token races the active lease clock and cannot heartbeat

**Claim attacked.** OQ2 bounds the reseal token by session TTL or reseal window and authorizes publish/complete for the current in-flight job. The proposal presents that as sufficient to reseal after rotation.

**Concrete refutation.** `artifact.publish` and `work.complete` both require an active, unexpired lease. `artifact.publish` calls `activeLeaseFor` before publishing (`go/pkg/mutations/artifact.go:124-130`); `work.complete` does the same before sealing (`go/pkg/mutations/lifecycle.go:1178-1180`). `activeLeaseFor` rejects non-active, wrong-owner, wrong-job, and expired leases (`go/pkg/mutations/mutations.go:803-820`). The only lane verb that extends that clock is `work.heartbeat`: it updates the session heartbeat, extends `leases.expires_at`, records local-work activity, and emits `lease.heartbeat` (`go/pkg/mutations/lifecycle.go:835-893`). But the registry requires `claim` for `work.heartbeat` (`go/pkg/rpc/registry_methods.go:75-76`), and the holder explicitly denies general `claim` to `CapabilityReseal`.

So a daemon restart that takes longer than the lease window, or a Codex reconnect wedge that keeps the lane alive but unable to heartbeat, leaves the session token still within its 24h TTL while the job lease is expired. At that point the reseal token can be cryptographically valid and session-bound but still unable to publish or complete. The terminal error is the ordinary `lease_error` path, not the typed `session_unrecoverable_across_rotation` recovery class promised by OQ4.

**Best holder rebuttal.** The intended survival case may be "deliverable already complete, short reconnect, active lease still alive"; if the lease expires, status-quo recovery can requeue.

**Gap that remains.** Then the proposal must say that Slice B only survives rotations inside the lease window, not inside the token TTL, and Option 4 must explicitly route expired-lease-after-rotation as `session_unrecoverable_across_rotation`. Alternatively, `CapabilityReseal` needs a scoped `work.heartbeat` authority for the current lease only. Today neither decision is made, and no falsifiable assertion covers the gap.

**Refuting game-day.** `GD-1b`: restart `striatumd` mid-job, prevent reconnect until after `leases.expires_at`, then let the lane use the reseal file. The acceptable outcomes are either a scoped heartbeat renews only that active lease before publish/complete, or the lane emits the typed self-escalating recovery class. A raw `lease is expired`, stale-lease limbo, or operator-manual diagnosis refutes the lifecycle claim.

## Challenge 3 - the Codex adapter still cannot adopt a rotated endpoint/epoch in place

**Claim attacked.** OQ3 says #323 recovery will read endpoint+epoch+reseal-token from the lane scratch mirror and reconnect. OQ1 folds a minimal Option-3 republish into Option 2 to close the epoch gap without weakening #316.

**Concrete refutation.** The current agent-loop code already documents a hard adapter boundary for Codex: the MCP URL is baked into launch-time `-c` args, Codex does not reload those overrides while running, and `mcpConfigPath == ""`; when the endpoint rotates, `applyMCPEndpointRotation` can only log and inject a prompt saying the lane must be relaunched/reconnected (`go/pkg/agentloop/loop.go:625-645`). The boot-epoch header is likewise rendered from `STRIATUM_MCP_BOOT_EPOCH` into adapter config (`go/pkg/agentloop/mcpconfig.go:491-496` for the config body, and the Codex path is launch-arg based), not dynamically pulled from a mirror by the running Codex tool process.

That means the proposed mirror may help a component that can be taught to re-read it, but it does not by itself let a live Codex lane's MCP tool channel publish or complete after a boot-epoch rotation. Current code deliberately treats Codex rotation as a loud wedge, not a survival path.

**Best holder rebuttal.** The proposal might intend the agent-loop receiver or CLI fallback, not the Codex MCP client itself, to use the mirror; or it might accept operator relaunch for Codex while supporting survival for other adapters.

**Gap that remains.** The spec does not say that. A buildable Slice B must name the adapter-specific survival matrix: Claude config rewrite, Agy behavior, Codex relaunch/rebridge, or a daemon-side receiver path that can publish/complete without the Codex MCP client. Otherwise the proposal overclaims survival for the exact adapter family whose current source says it cannot reload in place.

**Refuting game-day.** `GD-Codex-Reseal-Rotation`: a supervised Codex lane holds an active job, publishes or is ready to publish, then `striatumd` restarts. Without operator relaunch, the lane must use the new mechanism to `artifact.publish` and `work.complete` through the fresh endpoint+epoch. If it only receives the existing "dead endpoint; must be relaunched" prompt, Slice B has not survived the rotation for Codex.

## Challenge 4 - the endpoint/epoch mirror needs an integrity story, not just readability

**Claim attacked.** A6 says epoch republish does not weaken #316 because the lane adopts a fresh epoch only from a daemon-written, owner-trusted per-session scratch mirror.

**Concrete refutation.** The current scratch model is explicitly lane-writable. The lane sandbox runbook says `.striatum/scratch` is prepared so non-Codex lanes can write ephemeral MCP config there (`docs/how-to/lane-sandbox.md:269-272`), and `scratchACLTargets` grants `u:<lane>:rwx` plus a default ACL on `.striatum/scratch` (`go/pkg/mutations/scratch_acl.go:31-48`). That is correct for writable adapter config, but it is not an integrity boundary for a boot-epoch mirror. The MCP handler's #316 check also remains permissive when the header is absent: if the request presents no `X-Striatum-Boot-Epoch`, validation returns nil (`go/pkg/mcp/http.go:681-688`); only a non-empty mismatch is rejected (`:690-699`).

If the new mirror or generated config is lane-writable, a stale lane can delete or replace the epoch material and fall back to the header-absent path, or carry untrusted epoch material into the reconnect. If the holder wants a daemon-written, lane-read-only mirror, it must specify a path, owner, mode/ACL, symlink defense, atomic replace semantics, and a supervised-lane rule that requires an epoch header on the reseal path. Today the proposal names the trust property but not the mechanism.

**Best holder rebuttal.** Endpoint and epoch are non-secret; the token remains session-bound, so an attacker cannot escalate merely by seeing them. The mirror can be daemon-owned and lane-read-only even under `.striatum/scratch` if the implementation deliberately avoids inherited write ACLs.

**Gap that remains.** Non-secret does not mean integrity-free. #316 is an anti-confusion guard; it works only if the client cannot silently suppress or spoof the presented epoch. The spec must say how the daemon-owned mirror avoids the existing lane-writable scratch defaults, and it must decide whether supervised lanes with reseal tokens are rejected when the boot-epoch header is missing.

**Refuting test.** `TestResealEpochMirrorRejectsTamperOrMissingEpoch`: after launch, make the lane user attempt to modify, delete, symlink, or chmod the endpoint/epoch mirror and generated config; then attempt reseal with no epoch header or untrusted epoch material. Expected: refused with `stale_daemon_identity` or `session_unrecoverable_across_rotation`, never accepted.

## Net recommendation

Do not clear Slice B yet. Clear or keep Slice A only if it explicitly routes every rotation/auth/lease failure through `session_unrecoverable_across_rotation` rather than raw permission, stale endpoint, or lease errors. For Slice B, the adjudicator should require a revised holder proposal that specifies:

1. Exact OR-capability or alternate-route mechanics for `CapabilityReseal` before handler dispatch.
2. Whether reseal can heartbeat the current lease, or a precise statement that reseal only works while the original lease remains active.
3. Adapter-specific reconnect behavior, especially Codex.
4. Daemon-owned, lane-read-only endpoint/epoch mirror permissions plus a missing-epoch failure rule for supervised reseal.

Until those are concrete, the proposal has not resolved OQ2/OQ3/OQ4 with a buildable lifecycle.
