# FALSIFIER - RFC 0167 P0 escalated reconnect relabel gap

author: falsifier-reviewer-003

## Gate result

Material challenge: the holder SPEC proves the first two same-human terminals can
render as `maya#7f3` and `theo#7f3`, but it does not prove the promised reconnect
stability for the escalated terminal. If terminal 2 reconnects while its old
escalated lease is still live or lazily unexpired, the holder lease walk sees
both `candidates[0]` and `candidates[1]` occupied and assigns `candidates[2]`.
That contradicts A9's claim that the escalated session re-lands on
`candidates[1]`, and it creates exactly the silent live-window relabel R1 asks
this falsifier to stop.

This is not the earlier pre-run-session objection. Even if the revision adds a
buildable operator-session substrate, the SPEC still needs an authoritative
reattach/successor rule for an existing escalated operator session. Principal-only
identity cannot supply it, and reading tty/tmux/title/env is explicitly out of
bounds.

## Challenged claim

The holder SPEC claims deterministic same-human escalation is reconnect-stable:

- `HOLDER.md:143-168` defines `candidates[k] = POOL[(seed + k) mod len(POOL)]`,
  where `seed = fnv64a(principal_id)`, then says a second concurrent same-human
  session leases `candidates[1]` and, on its own reconnect while the first still
  holds `candidates[0]`, re-lands deterministically on `candidates[1]`.
- `HOLDER.md:171-205` relies on the run's write-once `created_by_handle_id`
  snapshot to prove `whose RA = maya#7f3` and `whose RB = theo#7f3`.
- `HOLDER.md:211-214` makes this falsifiable as A9 and A10: reconnect must not
  drift to a different word, and a past run must not silently relabel.
- The SEED requires this exact property: collision escalation must be
  deterministic and stable across reconnect (`SEED.md:73-88`, `SEED.md:174-180`),
  and terminal titles/tmux/env may not become state (`SEED.md:156-158`).

## Concrete counterexample

Assume the revised design repairs the pre-run substrate and all of the holder's
own tables exist. One human `H` has principal `P`; the candidate walk is
`maya`, `theo`, `nora`, ...

1. Terminal 1 registers operator session `S1`. It leases `maya` with
   `operator_handles(handle_id=h1, principal_id=P, leased_session_id=S1,
   handle='maya', released_at NULL, leased_until future)`.
2. Terminal 2 registers operator session `S2`. `maya` is live-held, so it leases
   `theo` with `handle_id=h2`.
3. `S2` creates run `RB`; the run correctly stamps `created_by_handle_id=h2`, so
   `whose RB` renders `theo#7f3`.
4. Terminal 2's process dies or the operator restarts that terminal, but the old
   `S2` handle lease has not been gracefully closed and has not passed
   `leased_until`. This is normal under the holder's lazy-expiry model: live
   uniqueness is `WHERE released_at IS NULL` and abandoned leases are only freed
   by graceful close or by the acquisition-path lazy-expiry UPDATE after TTL
   (`HOLDER.md:127-130`, `HOLDER.md:155-160`).
5. The reconnect creates a fresh operator session `S2b` for the same principal
   `P`. Current session-bound tokens are bound to one concrete session id: the
   token mint inserts grants with `client_capabilities.session_id = sessionID`
   (`go/pkg/mutations/session_token.go:60-96`), and the authorizer exposes exactly
   that grant session as `AuthContext.SessionID` (`go/pkg/rpc/auth_pg.go:99-153`,
   `go/pkg/rpc/capability.go:15-20`). Current session registration also mints a
   fresh random `session_id` (`go/pkg/mutations/lifecycle.go:368-384`).
6. `S2b` runs the same principal-seeded lease walk. `maya` is occupied by `S1`;
   `theo` is still occupied by abandoned-but-live `S2`; therefore the unique index
   forces `S2b` to `candidates[2] = nora`.

The live terminal that the human experiences as "terminal 2 reconnected" is now
`nora#7f3`, while the run it owns still renders `theo#7f3`. The holder can say
past-run history was not rewritten, but that only saves A10's frozen-run half. It
still falsifies A9 and the user-facing R1b goal: the SPEC promised stable
escalation across reconnect, yet the same live window is silently relabeled unless
some successor relationship exists outside `principal_id`.

## Why the holder's strongest rebuttal is not enough

The best rebuttal is: do not create `S2b`; reconnect should reuse the old
session-bound token/session id, or should explicitly close `S2` before minting the
replacement session. That would rescue A9.

But the SPEC does not specify either mechanism. Reusing the same token requires a
durable local operator-session credential and a daemon path that treats the call
as renewal/reattach of `S2`, not as a fresh mint. Closing `S2` first requires an
authorized successor proof that says the reconnecting process is entitled to
release exactly `S2`'s handle row. Principal identity alone is insufficient,
because all same-human terminals share `P`; any terminal of `P` could claim to be
terminal 2. The forbidden alternatives are the tempting ones: tty, tmux pane,
terminal title, or env-derived identity. The holder explicitly rejects those as
state inputs, and the SEED requires that rejection.

The run snapshot does not solve this. `created_by_handle_id=h2` preserves the old
run's answer, but it does not tell the new live session which handle it should
lease. Without an explicit reattach key or lease-transfer transaction, the
principal-seeded walk is deterministic in the wrong way: it deterministically
skips the still-live old escalated word.

## Refutation test

Add a two-role pgtest named `escalated_reconnect_retains_word`:

1. Create one human principal `P` and three candidate words whose walk is
   `maya`, `theo`, `nora`.
2. Register two pre-run operator sessions for `P`; assert `S1=maya` and
   `S2=theo`.
3. Create run `RB` under `S2` and assert `whose RB == theo#suffix(P)`.
4. Simulate an ungraceful terminal-2 restart before `S2.leased_until`: do not set
   `released_at` on `h2`, and mint/reattach the terminal as the SPEC says it
   should reconnect.
5. Assert the live handle after reconnect is still `theo`, not `nora`, and assert
   `whose RB` remains `theo#suffix(P)`.

If the implementation mints a fresh session and merely re-runs the holder's
principal-seeded candidate walk, the test fails: `S2b` gets `nora` because `theo`
is still live-held by `S2`. If the implementation passes, the SPEC must name the
reattach/transfer mechanism that made it pass.

## Unanswered gap

Name the stable reconnect identity and transaction. The revised SPEC must say
whether an operator restart reuses the old session token, presents a daemon-minted
reattach secret, performs an owner-authorized lease transfer, or accepts relabeling
as expected behavior. Until that is specified, R1b's two-distinct-answer proof is
only a first-registration proof, not a stable answer to "which of my 15 windows
owns this run" across the reconnect/flap scenario the packet explicitly asked this
lens to attack.
