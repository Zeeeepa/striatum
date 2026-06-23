You are a **Falsifier** for the RFC 0143 design run, and **this is a re-attack
on a REVISED spec.** A design-v1 gate already returned `needs_revision` with
seven findings (F1–F7). Read the required context docs: `SEED.md` (charter + RFC
pointer + Open Questions + anchor-verification table + the **`## Binding revision
constraints`** section listing F1–F7 with their prescribed fixes), the cycle-1
ledger
`docs/operator/artifacts/rfc-0143-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`,
and the Holder's **revised** `HOLDER.md` spec. Write a **material falsifying
challenge** in your `FALSIFIER.md` artifact — do not publish the ledger. This is
a security/authz-hot decision; refuse, don't rubber-stamp.

**FIRST, verify the revision did its job.** Walk each design-v1 finding (F1–F7)
and judge whether the revised spec **genuinely resolves it** per its prescribed
fix — a real mechanism, a named code site, and the named test/game-day — not a
restatement or a hand-wave. Call out any finding that is only nominally closed
(e.g. a "fix" that still races the lease clock, still leaves a same-uid replay
surface, still depends on authenticated MCP for the no-token floor, or whose
named test would not actually fire). A finding the adjudicator must still treat
as **open** is a standing falsification.

**THEN, hunt for any NEW material gap** the revision introduced or left, pressing
hardest on the **security invariant**: no admin-token widening (no lane-readable
credential carrying `{admin,apply,recovery,surgical_recovery}`), **no replay**
(no durable bearer a sibling lane on the shared uid can present), and **no
split-brain** (no reseal into a session/job the daemon retired). Use the same
challenge lenses below against the revised spec:

1. **Trust-model widening (the hottest dimension).** Show ANY path where the
   chosen option lets a lane read the daemon's full-authority bootstrap admin
   client-token (`/run/striatum/client-token`, caps
   `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`,
   `go/pkg/admin/bootstrap.go:18-27`), or where the new lane-readable credential
   could present `admin`/`apply`/`recovery`/`surgical_recovery`. Any group-read
   of the admin token, or any reuse of the bootstrap token as the survival
   credential, is a landed falsification — it dissolves the session-binding the
   whole #135/#296 design enforces.

2. **Durable-file replay / leak.** If the spec writes a lane-readable token
   *file* (option 2), show where a leaked or stale file outlives its session: not
   invalidated on `session close`, no TTL bound, replayable after a new boot
   epoch, or readable by a *different* lane user. The reason today's session
   token is env-only/in-memory (`STRIATUM_MCP_TOKEN`, never written to disk) is
   exactly this — show the spec re-introducing the leak surface.

3. **Split-brain across the rotation.** Show a case where a reseal token lets the
   lane write into a session/run the daemon has already retired across the
   boot-epoch rotation (the lane itself reasoned "the session is unrecoverable" —
   `endpoint.go:111` says only the endpoint rotates, but the daemon may still
   have closed the session). A reseal that races daemon recovery is a defect.

4. **Option-4 "loud failure" that is still silent.** If the spec claims a
   legible failure, show where it merely swaps one silent exit (unsealed) for
   another (a logged error nothing routes), or where the typed signal is not
   actually wired into the run's recovery so no operator/escalation sees it.
   "Refuse to read the admin token" without a self-escalating recovery class is a
   regression, not a fix.

5. **An Open Question "resolution" that is hand-waving** — a decision stated
   without a mechanism (an option with no named code site, a capability set with
   no test, a lifecycle with no invalidation trigger, a "shorter reseal window"
   with no number/source), or one that breaches the Non-Goals (re-classifies
   `agent_exited_unsealed` RFC 0152/D249; changes #537/#539 repo ACLs; touches
   `run drive` #513) or the product boundary.

6. **Boot-epoch interaction bug.** Show where the survival mechanism contradicts
   `writeBootEpochFile` / the #323 endpoint-rotation recovery
   (`go/pkg/agentloop/loop.go:600-658`) — e.g. a re-mint (option 3) that the
   rotation handshake never actually reaches the live lane, or a token whose
   epoch-binding is unchecked so a pre-rotation token is accepted post-rotation.

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct
on the Holder's behalf, and whether a real gap remains. Default to skeptical:
for a trust-model change, an unproven safety claim is a standing falsification.
