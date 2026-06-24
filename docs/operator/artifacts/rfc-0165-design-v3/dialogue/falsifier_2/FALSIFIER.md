# RFC 0165 v3 Falsifying Challenge: Same-User Claude Lanes Still Break Refresh-Token Custody
author: falsifier-reviewer-004

## Challenge

The v3 holder resolves F2 only after assuming a distinct lane OS user. The SPEC still leaves same-user Claude supervision in the supported product surface, and in that mode the lane runs as the daemon/operator user. That gives the lane a direct source-read path to the operator Claude credential file, including the raw refresh token.

This is still F2's forbidden condition: raw refresh-token custody by a readable store, plus the ability for a lane-side Claude refresh to rotate the operator credential family. The distinct-UID B1/B2 design may be OAuth-safe, but the SPEC does not make distinct UID a required precondition for Claude OAuth lanes and does not fail closed when same-user collapse applies.

I am not landing a separate control-plane-token finding. For distinct-UID lanes, B1 access-token-only projection and B2 `SO_PEERCRED` broker identity plausibly keep provider OAuth separate from Striatum session-bound capability tokens. The standing challenge is narrower and gate-stopping: a supported launch mode bypasses the lane/operator filesystem boundary the F2 proof relies on.

## Claim Challenged

The holder claims F2 is resolved because the lane never receives a refresh token, refresh authority is held only by the operator-side credential owner, the lane has no read path to the operator source, and concurrent/subsequent RTR tests cannot desynchronize or invalidate the source.

Those claims are true only for a distinct lane uid. The holder's own proof says the operator credential is safe because it is operator-owned `0600` and the lane OS user is distinct. But the implementation spec also names same-user collapse, and the spawn-time projection gate is scoped to `config.adapterName() == "claude"` only when a distinct lane OS user is configured. In same-user mode, the process identity that runs the lane is the same identity that owns and can read the operator credential.

## Evidence

The v3 seed required F2 to prove that a lane cannot obtain raw refresh-token custody by any route and cannot independently rotate the operator source credential family. The v1 cycle-1 ledger made that binding as C1: prevent lanes from receiving or independently refreshing raw Claude OAuth refresh tokens, or prove an equally explicit file model that handles RTR without source invalidation.

The v3 holder's F2 proof depends on a distinct-UID assumption:

- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:28-69` says lanes receive only an access token and have no source-read path because the lane OS user is a distinct uid.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:285-286` still defines same-user collapse as a destination mode.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:336-339` runs `runSuperviseClaudeCredentialGate` only when a distinct lane OS user is configured.

Current source confirms same-user supervision is live, not hypothetical:

- `go/pkg/mutations/supervision_env.go:228-237` collapses unset or same-as-daemon `STRIATUM_LANE_OS_USER` to empty `RunAsUser`.
- `go/pkg/mutations/supervision_env.go:259-265` executes the supervised command directly when `RunAsUser` is empty; there is no `sudo -u` identity split.
- `go/pkg/mutations/supervision_env.go:38-42` builds the same-user lane launch environment from the daemon/operator process environment.
- `go/pkg/laneproviderauth/resolver.go:78-90` resolves Claude credentials from `CLAUDE_CONFIG_DIR/.credentials.json` or `HOME/.claude/.credentials.json` in that launch environment.
- `docs/reference/spec.md:2638-2641` documents that when `STRIATUM_LANE_OS_USER` does not name a distinct user, Striatum preserves same-user behavior.
- `docs/reference/command-authority-matrix.md:426` even presents unsetting `STRIATUM_LANE_OS_USER` as a remediation for an unresolved lane user.

F1's recovery-classification change does not repair this custody path. It may classify a later expiry as provider-auth debt, but by then the same-user lane already had source read access to the raw refresh token and could have exercised the refresh flow.

## Concrete Counterexample

1. The host runs Striatum in documented same-user mode: `STRIATUM_LANE_OS_USER` is unset, or it names the daemon owner and collapses to empty `RunAsUser`.
2. The operator Claude credential at `$CLAUDE_CONFIG_DIR/.credentials.json` or `$HOME/.claude/.credentials.json` contains the normal OAuth `refreshToken` and access token.
3. A Claude supervised lane launches. Because there is no distinct lane OS user, source code runs it as the daemon/operator user. Under the v3 SPEC, the Claude projection gate is either a same-user verify-only no-op or is skipped by the "distinct lane OS user" condition.
4. The lane's Claude CLI resolves the same operator credential path from its launch environment. The lane can read the raw `refreshToken` and, when the access token expires, can perform the CLI's normal refresh flow.
5. Two same-user lanes, or one lane and the operator CLI, refresh near-simultaneously. There are now multiple refresh actors using the same refresh-token family. A lane-side action can rotate the operator source. Even if the shared file is updated rather than leaving a stale copied file behind, a lane has independently rotated the operator credential family, which C1 forbids.
6. A subsequent lane launches after lane A's refresh. It now relies on source state mutated by a prior lane action, not by the operator-only refresh authority. The named `TestSubsequentLaneAfterOperatorRefresh` does not cover this because its source rotation is operator-side, not lane-side.

This is not a theoretical B1/B2 helper failure. It is a supported mode where the filesystem boundary required by the proof does not exist.

## Strongest Rebuttal

The strongest rebuttal is operational: the dogfood host normally uses a distinct `striatum-lane` account, and the v3 design is plainly optimized for that shape. In that shape, an operator-owned `0600` source blocks lane reads, B1 writes no `refreshToken`, B2 returns only access tokens after `SO_PEERCRED` uid verification, and the concurrent/subsequent RTR tests are meaningful.

That does not satisfy the F2 contract. The product and source still support same-user supervised lanes, and the holder does not declare same-user Claude OAuth unsupported, unsafe, or fail-closed. F2 asks whether a lane can obtain raw refresh-token custody by any route. Same-user source read is such a route.

## Carry-Forward Regression

The same gap also weakens the carried-forward spawn-time freshness gate. The v3 SPEC says the projection gate runs only for Claude with a distinct lane OS user. In same-user mode, a Claude lane can launch against the operator credential directly, so the revised access-token projection, `refresh_token_absent_ok` receipt, and no-refresh-token tests never prove the surface that actually starts.

I did not find a separate durable leak to DB rows, repo artifacts, metrics, events, doctor output, or Striatum control-plane tokens in the distinct-UID design. The unresolved leak is runtime custody: the same-user lane process can read the operator source store itself.

## Required Revision

The SPEC needs an explicit same-user decision before F2 can clear:

- Preferred: for Claude OAuth self-driving lanes, fail closed when `config.RunAsUser == ""` or resolves to the daemon/operator identity. Add a typed launch-precondition error such as `provider_credential_same_user_unsupported`, and refuse before scratch, session-token minting, supervisor rows, helper/tmux, or Claude process launch.
- If same-user Claude lanes must remain supported, move the refresh source behind a daemon-only boundary the lane identity cannot read, then use only the broker/access-token path. With the same OS uid as the operator, an operator-home `0600` file cannot provide that boundary.
- Extend `TestLaneNeverReceivesRefreshToken` with a same-user fixture: operator source contains a known `refreshToken`, `STRIATUM_LANE_OS_USER` is unset or same-as-daemon, and `supervise.start` must refuse rather than launch a Claude process that can read the source.
- Add a same-user RTR test where two lanes attempt refresh from the shared operator credential. The expected result must be launch refusal, not "operator source changed due to a lane action."
- Extend the source-read assertion so it scans every credential surface readable by the lane launch identity, not only the newly written projection file or B2 broker response.

## Bottom Line

F2 is not genuinely discharged as written. The v3 holder plausibly fixes distinct-UID lanes, but current Striatum still supports same-user supervised lanes. In that mode a Claude lane can read and rotate the operator source refresh token directly. The SPEC must either remove same-user Claude OAuth from the supported surface or make it fail closed before launch.