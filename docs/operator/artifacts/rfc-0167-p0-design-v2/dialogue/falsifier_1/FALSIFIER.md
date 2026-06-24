# FALSIFIER - RFC 0167 P0 v2 C1 challenge

author: falsifier-reviewer-003

## Claim Challenged

C1 / A27 claims the v2 SPEC genuinely proves the pre-run operator-session path end to end: `operator bootstrap` mints a session-bound operator token with `session_id = <operator_session_id>`, that token is presented on `run.prepare`, the authority prelude sets `app.session_id`, and the runs INSERT stamps a non-NULL `created_by_handle_id` from the matching `operator_handles` row.

## Material Challenge

The revised storage substrate is plausible, but the proof still collapses at the daemon authorization layer. The v2 SPEC explicitly reuses `mintSessionBoundToken` for the operator token, but current source gives that token only the fixed supervised-lane grant set: `claim`, `write`, `read`, and `review` (`go/pkg/mutations/session_token.go:23-46`), and the mint loop inserts exactly those capabilities into `client_capabilities` (`session_token.go:77-86`). There is no `admin` grant.

The real run-creation route requires `admin`: `run.prepare` is registered with `CapabilityAdmin` (`go/pkg/rpc/registry_methods.go:110`, also pinned by `go/pkg/rpc/registry_rfc0043_test.go:27`). The dispatcher calls `Authorize` and `RequireAllowed` before it threads `AuthContext` into the handler or routes the request (`go/pkg/rpc/server.go:107-124`); the Postgres authorizer returns `capability_missing` when the token lacks the required capability (`go/pkg/rpc/auth_pg.go:104-140`). So the operator-session token described by the SPEC is refused before `HandleRunPrepare` can begin its authorized transaction and before the authority prelude can install `app.session_id` (`go/pkg/db/authority.go:75-80,116-120`).

That means the two-same-human-terminal proof cannot run through the real path. `S1 -> run RA` and `S2 -> run RB` do not reach the run INSERT, so the SPEC has not demonstrated two NON-NULL DISTINCT `created_by_handle_id` values or distinct `whose` answers. This is not the v1 `sessions.run_id` FK failure: v2 appears to solve that narrow storage issue because `client_capabilities.session_id` is just a token binding and `PostgresAuthorizer` copies it into `AuthContext.SessionID` without joining `striatumd.sessions` (`auth_pg.go:104-156`). The remaining C1 gap is the missing capability shape for the new operator token.

## Strongest Rebuttal I Can Justify

The implementation can still be repaired. It could introduce a distinct operator-token mint path, parameterize `mintSessionBoundToken`, or add a narrower authority route that admits only the operator bootstrap -> prepare/start flow while preserving `app.session_id`. But the v2 SPEC does not choose or specify that mechanism.

The obvious shortcut, adding `admin` to the existing `sessionBoundCapabilities` slice, is not a safe implied fix. That slice is documented as the supervised lane-loop grant set, and it is used by `session.register` for ordinary lane tokens. Granting `admin` there would let every supervised lane session token call admin routes such as `run.prepare`, `run.start`, `run.pause`, and `run.cancel`, which is a material privilege expansion outside the C1 proof.

## Unanswered Gap / Refuting Test

`operator_session_pre_run_stamp` must exercise the real RPC authorization path with the minted operator-session token, not a direct SQL insert, a manually seeded GUC, or a broad repo-scoped token. The test should prove:

1. The operator-session token has the narrowly scoped authority needed to call `run.prepare` and stamp `app.session_id`.
2. Ordinary supervised lane tokens minted by `session.register` do not gain `admin` or equivalent run-control authority.
3. A closed or expired operator session cannot keep preparing runs with a stale session-bound token that stamps `created_by_handle_id = NULL` or reuses an expired handle.

Until the SPEC names that authority mechanism and tests those controls, C1 is not genuinely discharged. The pre-run table can exist, but the token carrying its `operator_session_id` cannot create the run whose handle stamp is the sufficiency proof.
