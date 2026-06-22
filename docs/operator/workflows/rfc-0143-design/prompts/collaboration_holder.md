You are the **Holder** for the RFC 0143 design run. Read the required context
doc `SEED.md` in full first — it carries the charter, a pointer to the committed
RFC `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`, the
four Open Questions, and an operator anchor-verification table (every cited
source path was verified ACCURATE against current `main` — build on those exact
anchors and file:line references).

Author the **leading falsifiable implementation spec** for lane credential
survival across a daemon boot-epoch rotation as your published `HOLDER.md`
artifact. This is the claim the falsifiers will attack and the adjudicator will
gate — make it concrete and falsifiable, not a restatement of the RFC.

Hold the root reframe: **a boot-epoch rotation must never force a lane to choose
between reading the daemon's full-authority bootstrap admin token and exiting
silently unsealed.** The fix must preserve the RFC 0096/#135/#296 session-bound
trust model — a lane authenticates as its own narrow, session-scoped token and
*never* as the shared operator admin override.

Your spec MUST:

1. **Resolve every one of the four Open Questions** with an explicit decision
   (which option / mechanism / why). Leaving any unresolved fails the charter:
   - **OQ1 — Which trust-model option.** RFC options: (1) status quo +
     operator requeue; (2) mint a durable, lane-owned, session-scoped *reseal*
     token (`striatum-lane`-owned `0600`) carrying only the minimal caps; (3)
     re-mint and re-inject the session-bound token into the live lane env/file as
     part of the boot-epoch rotation handshake; (4) narrow the fallback so a
     non-owner lane fails *legibly* (self-escalating "session unrecoverable
     across rotation") instead of silently unsealed. Decide the primary survival
     mechanism AND whether to also land option 4 as the safety net. Justify.
   - **OQ2 — Surviving authority + lifecycle.** If a new credential class is
     chosen: which exact capabilities survive a rotation (just `write` to seal
     the in-flight job? the lane's normal `{claim,write,read,review}`?), for how
     long (session TTL vs. a shorter reseal window), the file ownership/mode
     (`striatum-lane`-owned `0600`), and **how it is invalidated** when the
     session truly ends (on `session close`, on TTL, on a new boot epoch).
   - **OQ3 — Where the mechanism lives.** Name the exact code site(s): the
     mint site (`go/pkg/mutations/session_token.go` `mintSessionBoundToken`),
     the injection site (`go/pkg/mutations/supervision_env.go` `STRIATUM_MCP_TOKEN`),
     the rotation publish (`go/cmd/striatumd/main.go` boot-epoch), the resolution
     chain (`go/pkg/agentloop/token.go` `ResolveTokenMaterial`), and the #323
     recovery (`go/pkg/agentloop/loop.go` `ResolveTokenMaterialFresh`). For
     option 3, name the rotation handshake step that re-injects.
   - **OQ4 — Legible-failure fallback.** Even with a survival token, define the
     loud, self-escalating failure when the lane genuinely cannot reseal: make
     `go/pkg/agentloop/token.go` refuse to fall through to the admin client-token
     for a non-owner lane (step 3 of the chain) and surface a typed
     "session_unrecoverable_across_rotation" signal that the run's recovery
     routes — not a silent unsealed exit or a misleading "permission denied".

2. **Hold the security invariant as the spine.** Per the anchor table:
   `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`)
   grants the runtime client-token the FULL `bootstrapCapabilities` set
   `{admin, read, write, claim, review, apply, recovery, surgical_recovery}` and
   `writeRuntimeToken` writes it `0600` owner-only. Any option that lets a lane
   read that file, or that mints a lane-readable token carrying ANY of
   `{admin, apply, recovery, surgical_recovery}`, is **categorically out of
   bounds** — say so explicitly and design so it is structurally impossible.

3. **State each load-bearing claim as a falsifiable assertion + the named test /
   game-day step that would refute it.** At minimum:
   - **No-widening:** the new credential carries only reseal-minimal caps; a test
     asserts presenting it for `admin`/`apply`/`recovery` is REFUSED.
   - **No-replay:** a durable lane-readable token file (if any) is invalidated on
     session close and cannot be replayed past its session/TTL/boot-epoch; a test
     asserts a stale file is rejected.
   - **No split-brain:** a reseal across rotation only seals the in-flight job the
     daemon still recognizes; it cannot write into a session the daemon retired.
   - **Loud failure:** the option-4 path emits a self-escalating recovery signal;
     a game-day (restart daemon mid-job) shows the lane fails legibly and the run
     routes it, with NO silent unsealed exit.

4. **Stay inside the product boundary and the Non-Goals.** This RFC governs only
   *whether/how a lane holds a readable credential across a rotation*. Do NOT
   re-classify the downstream `agent_exited_unsealed` recovery policy (RFC 0152 /
   D249), do NOT change the committee POSIX-ACL repo provisioning (#537/#539),
   and do NOT touch `run drive`'s transient-socket behavior (#513). Local-first,
   single-host, daemon-owned PostgreSQL as the single writer.

5. **Flag the maintainer ratification gate.** The chosen option is a
   security/authz trust-model change. State plainly that the cleared spec is a
   RECOMMENDATION the maintainer ratifies before any build slice lands credential
   code.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
