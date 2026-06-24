# FALSIFIER - RFC 0167 P0 v3 C1prime challenge

author: falsifier-reviewer-003

## Claim Challenged

C1prime / A29-A32 claims that v3 resolves the v2 operator-token authorization
gap by minting a distinct pre-run operator-session token with `{admin, read}`,
not by widening the ordinary lane token slice. The claim also says the
two-terminal proof now reaches real `run.prepare`, ordinary lane tokens remain
non-admin, and closed or expired operator sessions cannot keep stamping runs.

## Material Challenge

The v3 SPEC does close the original `run.prepare` authorization hole: it names a
distinct `mintOperatorSessionToken`, binds it to an operator session, and gives
it `admin` so the dispatcher can authorize `run.prepare` before the handler
needs `app.session_id`. It also keeps `sessionBoundCapabilities` at
`{claim, write, read, review}`, so the obvious "all lanes gain admin" regression
is avoided (`go/pkg/mutations/session_token.go:23-46,77-89`).

But the SPEC's chosen capability is still too coarse for the proof it is meant
to discharge. In current source, `CapabilityAdmin` is a repo-admin gate, not a
run-stamping gate. The registry maps the same capability to
`workflow.accept_risk`, `review.override`, `decision.record`,
`checkpoint.resolve`, `verifier.attest`, `branch.confirm`, `run.prepare`,
`run.start`, `run.pause`, `run.resume`, `run.cancel`, `run.retry_job`, and
`repo.init` (`go/pkg/rpc/registry_methods.go:102-117`; pinned subset in
`go/pkg/rpc/registry_rfc0043_test.go:19-31`). The RPC server authorizes by
required capability before routing and does not pass the method name into a
method allowlist check (`go/pkg/rpc/server.go:107-124`). The Postgres
authorizer accepts any non-revoked matching capability row for the repository
and then returns an allowed `AuthContext` carrying the bound session id
(`go/pkg/rpc/auth_pg.go:104-156`).

So a valid v3 operator-session token that clears `run.prepare` also clears other
admin routes unless those handlers explicitly refuse session-bound admin tokens.
Representative handlers do not. `review.override` validates review/job/session
state and inserts an override verdict, but it has no `auth.IsSessionBound()`
refusal (`go/pkg/mutations/review.go:172-231,269-328`).
`checkpoint.resolve` resolves human checkpoints and can complete the checkpoint
job without rejecting session-bound admin (`go/pkg/mutations/operator.go:197-245,
321-365`). `workflow.accept_risk` records accepted lint risk after admin
authorization with no session-bound gate
(`go/pkg/mutations/workflow_accepted_risk.go:15-97`). `branch.confirm` records
branch confirmation as `human` after admin authorization and likewise has no
session-bound-token refusal (`go/pkg/mutations/run.go:818-899`).

The codebase already treats this exact shape as dangerous in at least one admin
route. `verifier.attest` is registered as `CapabilityAdmin`, but the handler
explicitly refuses any session-bound token and says the refusal must hold "even
for a hypothetical admin-capable session token"
(`go/pkg/mutations/verifier_attest.go:13-33,49-59`). The v3 SPEC creates that
hypothetical token for ordinary operator terminals, and potentially many of
them at once. It then proves only that a lane token cannot call `run.prepare`;
it does not prove that the new operator-session token is constrained to the
run-lifecycle authority the C1prime stamp proof requires.

This is material because the seed asks the falsifier to test whether the
operator token over-grants strictly more than `run.prepare` and `run.start` need
or opens a privilege-escalation surface. Under the proposed `{admin, read}`
shape, the answer is yes. The token is session-bound and lifecycle-limited, but
while live it is still a general repo-admin credential for every route that has
not added its own handler-side session-bound refusal.

## Strongest Rebuttal I Can Justify

The holder's strongest rebuttal is that the human operator already has admin
today through the static runtime token, and that a TTL-bounded operator-session
token revoked on close is less durable than that static token. That is true as
far as standing lifetime goes, and the distinct mint path is the right direction.
The v3 shape should make the two same-human terminals proof produce two
non-NULL, distinct `created_by_handle_id` values through real `run.prepare`.

The rebuttal does not answer the narrower C1prime obligation. The design is not
only replacing one human-held secret with a less durable one; it is introducing
per-terminal, session-bound admin credentials into the same substrate whose
handlers already distinguish "session-bound token" from "operator token" for
trust boundaries. `verifier.attest` demonstrates that `admin` alone is not a
sufficient proof of operator-only safety. If the P0 boundary is "session-bound
operator tokens may use the entire repo-admin surface," the SPEC needs to say so
and justify the N-terminal blast radius. If that is not the boundary, the SPEC
needs a narrower capability, a method allowlist, or explicit session-bound
refusals on non-run-lifecycle admin routes.

## Unanswered Gap / Refuting Test

Add a negative control for the minted operator-session token itself:

1. Mint a valid pre-run operator-session token through `mintOperatorSessionToken`
   and prove it can call the real `run.prepare` path, yielding the required
   non-NULL `created_by_handle_id`.
2. With that same token, attempt representative non-stamping admin routes:
   `review.override`, `checkpoint.resolve`, `workflow.accept_risk`, and
   `branch.confirm`.
3. The build must either reject those calls for a session-bound operator token
   with a typed `capability_denied` / route-not-allowed result, or explicitly
   document and accept the full repo-admin surface as the P0 operator-session
   boundary.

Until that control exists, C1prime is only partially discharged. The original
`run.prepare` proof can pass, lane tokens can remain narrow, and closed/expired
tokens can be revoked, while the live operator-session token still carries more
repo-admin authority than the run-origin stamping proof needs.
