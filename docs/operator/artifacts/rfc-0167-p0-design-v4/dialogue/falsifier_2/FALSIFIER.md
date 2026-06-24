# FALSIFIER - RFC 0167 P0 design v4 C1 double-prime challenge

author: falsifier-reviewer-002

## Claim Challenged

The v4 holder says C1 double-prime is resolved by justified acceptance: the operator-session token may carry `{admin, read}` because the accepted repo-admin surface is the operator's legitimate authority, `verifier.attest` is fenced by the existing `IsSessionBound()` refusal, daemon-global admin routes are unreachable because the token is repo-scoped, and the N-token blast-radius is a reduction because each token is TTL-bounded and revoked on close. The specific challenged claims are A40/A41/A42 and the blast-radius analysis in `docs/operator/artifacts/rfc-0167-p0-design-v4/dialogue/holder/HOLDER.md:313-425`, especially the conclusion at `HOLDER.md:393-397` that this replaces a long-lived static admin token with narrower self-expiring operator tokens.

## Material Challenge: The Blast-Radius Argument Depends On An Unproved Replacement Of The Static Admin Token

The trust-root fence itself is real: `verifier.attest` refuses any session-bound token at `go/pkg/mutations/verifier_attest.go:49-59`, and a repo-scoped operator capability row cannot satisfy daemon-global methods under the exact-match authorizer in `go/pkg/rpc/auth_pg.go:104-140`. The problem is the holder's N-token comparison. It treats the operator-session tokens as a replacement for the static bootstrap admin credential, but the current source shows the static credential is a separate, broader, non-expiring bearer and the v4 SPEC does not define or test a transition that removes it from the operator blast radius.

Source evidence:

- The bootstrap runtime token grants much more than `{admin, read}`. `go/pkg/admin/bootstrap.go:18-27` defines `bootstrapCapabilities = {admin, read, write, claim, review, apply, recovery, surgical_recovery}`. `BootstrapRuntimeTokenIfNeeded` mints a `bootstrap-admin` client through `insertTokenClient` with `expiresAt = nil` at `go/pkg/admin/bootstrap.go:92-98`, then writes that bearer to the daemon runtime `client-token` file at `go/pkg/admin/bootstrap.go:29-35,105-116`.
- The tests pin the important security properties: the file is `0600` and the bootstrap grants are repository-unscoped (`nil`) and include all eight capabilities, not just admin/read (`go/pkg/admin/bootstrap_test.go:25-36,63-75`).
- `insertTokenClient` accepts a nil expiry and nil repository scope, then writes each capability row as supplied (`go/pkg/admin/tokens.go:286-315`). So the static token is a standing daemon-wide credential unless explicitly revoked or operationally segregated.
- The v4 operator token is narrower, but also different: the holder specifies `operatorSessionCapabilities = {admin, read}`, repo-scoped and session-bound (`HOLDER.md:501-505`, `HOLDER.md:987-989`). Nothing in the v4 build manifest or `operator_token_admin_surface` control proves the old static credential is revoked, removed from the operator process, hidden from the MCP client, or prevented from being used for the same run-start/admin workflows.

That leaves an unresolved fork:

1. If the operator-session token truly replaces the static bootstrap token for operator work, the SPEC has not shown the operator job remains complete. The current bootstrap credential carries `write`, `claim`, `review`, `apply`, `recovery`, and `surgical_recovery` in addition to `admin/read`; the v4 token does not. The holder argues a run-lifecycle-only capability would break the operator's job, but then compares against a replacement token that is still narrower than today's operator bootstrap capability set in several non-admin operator surfaces.
2. If the static token remains available for those other surfaces, the blast-radius conclusion is not honest. The system now has the static daemon-wide bearer plus N repo-scoped session-bound admin/read bearers. Each new token is individually shorter-lived than the static token, but the total live credential set is additive, not strictly-less-standing. A leaked terminal now leaks a repo-admin session token that did not exist before, while the old static token still exists elsewhere.

The current C1 control does not catch this. `operator_token_admin_surface` proves the new token can run representative repo-admin routes, is refused at `verifier.attest`, cannot call a daemon-global method, and dies after close/expiry (`HOLDER.md:399-408`, `HOLDER.md:821-827`). It does not prove the static token is absent from the operator environment, not used by `striatum operator bootstrap` / MCP setup, or retired after the session token is minted. It also does not include a positive test for the non-admin operator capabilities that the static token currently carries.

## Strongest Rebuttal And Why It Does Not Clear The Gate

The strongest rebuttal is that the static `0600` token is the daemon bootstrap/root credential, not the routine credential that should be presented to repo-admin routes after RFC 0167 P0 lands; the new operator-session token intentionally reduces the routine run-start path to repo-scoped, TTL-bounded, close-revoked authority. That would be a sound direction.

But it is not yet a falsifiable SPEC claim. The holder never states the operational invariant that the static token is segregated from the operator session after minting, never adds a control that `run.prepare`/`checkpoint.resolve`/`review.override` are actually presented with the operator-session token rather than the static token, and never documents what still uses the static token for recovery/apply/surgical surfaces. Without that boundary, the N-token analysis has an accounting error: it compares each new token individually to the static token, while the live system may contain static-plus-N credentials.

## Required Refutation Test / Fix Shape

Add a C1 double-prime credential-boundary control before clearing the justified acceptance. It should prove one of these explicit designs:

- Replacement design: after operator bootstrap, routine operator repo-admin routes (`run.prepare`, `checkpoint.resolve`, `review.override`, `branch.confirm`) are invoked with a client whose capability rows are session-bound to the operator session, and the static `bootstrap-admin` token is not present in the launched operator process or MCP client config. Separately prove any non-admin operator verbs still required for the operator job have an intended credential path.
- Additive design: keep the static token, but rewrite the blast-radius acceptance as static-plus-N, not strictly-less-standing, and add controls that bound where the static token remains usable and where the new session tokens are injected.

Until the SPEC chooses and tests one of those, A40's justified-acceptance is unproved. The `verifier.attest` fence can be correct and the C1 double-prime gate can still fail because the N-token blast-radius premise is not source-honest.