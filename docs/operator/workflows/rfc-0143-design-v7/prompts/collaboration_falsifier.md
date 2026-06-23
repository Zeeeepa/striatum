You are a **Falsifier** for the RFC 0143 design run, and **this is a re-attack on
the SEVENTH-REVISION (v7) spec.** Six prior gates ran on this spec: v1 returned
`needs_revision` with seven findings F1–F7; v2 resolved F2/F4 and distilled the
residue into binding constraints BC1–BC5; v3 resolved BC2/BC3/BC4; v4 resolved BC5,
C2, and the daemon-observed positive intent; v5 **RESOLVED the big channel rework** —
it deleted the inherited-fd channel and adopted the **CONNECT-OUT topology** (the
pane wrapper dials OUT after `PR_SET_DUMPABLE(0)`; no fd crosses the tmux
client/server boundary; non-secret listener address; post-auth nonce), fixed a real
v4 exit-code drift (the `#{pane_dead_status}` backstop), and carried the v4-credited
set forward; v6 **RESOLVED BC1-W1-TOKEN** (the clock-domain defect: W1 now compares
field-22-to-field-22 via `LaunchResult.PaneKernelStartToken` captured via
`ProcessStartToken(identity.PanePID)`, keeping tmux `#{pane_start_time}` as liveness
metadata only, with the empty-token fail-closed rule (A3'') and the same-pid
stale-token negative (A3')). But v6 returned `needs_revision` on a single ground
BOTH falsifiers landed independently: **BC1-W1-CAPTURE — the kernel start token is
captured POST-LAUNCH and is not structurally proven to belong to the born wrapper.**
Read the required context docs: `SEED.md` (charter + RFC pointer + the
**`## Ratified design shape`** + the **`## Carried forward — resolved by v6`** set
+ the **`## The binding constraint v7 MUST resolve`** section stating BC1-W1-CAPTURE
with its prescribed fix, the verified source sites, and the named real-path test
extension), the v6 ledger
`docs/operator/artifacts/rfc-0143-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`,
the design-v6 `HOLDER.md`, and the Holder's **revised** (v7) `HOLDER.md` spec. Write a
**material falsifying challenge** in your `FALSIFIER.md` artifact — do not publish
the ledger. This is a security/authz-hot decision; refuse, don't rubber-stamp.

**FIRST, verify the revision did its job on the lens your objective assigns you**
(Falsifier 1: the BC1-W1-CAPTURE capture-boundary / same-uid lens that re-attacks the
capture race + checks the fix is structural not temporal; Falsifier 2: the
lifecycle/regression lens that verifies the whole v6-credited set is unregressed).
Judge whether the revised spec **genuinely resolves its target** per the prescribed
fix — a real mechanism, named code sites, and a named test that would actually fire
against the production path — not a restatement or a temporal hand-wave. A constraint
the adjudicator must still treat as **open** is a standing falsification, and a
regression of any v6-credited item is a standing falsification.

**If your lens is BC1-W1-CAPTURE (the capture-boundary / same-uid lens), press
hardest on the exact v6-unrebutted ground:**

- **(1) Is there a fail-closed capture-boundary re-verification?** After
  `CaptureTmuxIdentity` reports `identity.PanePID` and `ProcessStartToken(identity.PanePID)`
  is captured, does the spec re-verify via tmux/liveness (`ProbeTmuxLiveness` states
  at `tmux_liveness.go:257-269`: pane-dead, pane-pid mismatch, pane-start mismatch)
  that the pane is **still live** and **still the same pane id/pid** at the capture
  boundary **BEFORE** stamping `PaneKernelStartToken` and **BEFORE** any accept loop
  binds the channel? Recall the v6 defect: `launchPTY` starts the pane command with
  `tmux respawn-pane` (`pty.go:479-484`), THEN calls `CaptureTmuxIdentity`
  (`pty.go:493`), THEN `attachTmuxPTY` returns `LaunchResult{PID: identity.PanePID}`
  (`pty.go:517-533`); `ProcessStartToken` reads whatever process currently owns
  `/proc/<pid>/stat` field 22 at read time (`process_identity_linux.go:11-32`). So if
  the launched wrapper exits before the `/proc` read and its pid is reused by a live
  same-uid sibling, the helper stamps the reused process's non-empty field-22 token —
  and W1 accepts the first connection from that reused pid (both `peer.pid == result.PID`
  and `PIDLiveWithStartToken(peer.pid, capturedToken)` hold). Does the v7 spec add a
  re-verification step that closes this window structurally — or does it still rely
  only on temporal proximity ("the window is small")? A defense based only on timing
  is NOT structural and is a landed falsification for a security/authz channel.
- **(2) Does the fail-closed path route to the typed floor?** If the pane is dead,
  missing, pid-mismatched, or identity-unverifiable at the capture boundary, does the
  spec require the helper to NOT stamp the kernel token, accept NO control connection,
  and route through the `#{pane_dead_status}`/recovery-sweep typed
  `session_unrecoverable_across_rotation` floor — NEVER a raw launch/control error?
  A spec that leaves this path as an ad-hoc implementation choice, or that lets it
  leak a raw error class, does not fail closed structurally.
- **(3) Does the real-path test fire the race?** Does the spec extend
  `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or add a paired integration
  negative) with a **"pane dies before kernel-token capture"** case that forces the
  wrapper to exit before W1 capture, makes the token read observe a
  same-pid/reused-process token by stub or controlled hook, and asserts the helper
  REFUSES — not just that the positive case passes? A test that does not fire the
  race (pane-dies-before-capture path), or that asserts only the positive case, does
  not prove the capture-boundary invariant holds.
- **(4) Are the v6 operand-source components retained?** Does the spec RETAIN
  `LaunchResult.PaneKernelStartToken` captured via `ProcessStartToken(identity.PanePID)`,
  the kernel-field-22-to-kernel-field-22 W1 comparison via
  `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`, the empty-token
  `!= ""` fail-closed guard (A3''), and the same-pid stale-token negative (A3')? A
  regression of any of these is a standing falsification.

**If your lens is regression / lifecycle, verify NONE of the v6-credited set is
regressed by the v7 BC1-W1-CAPTURE fix**, pressing on: (1) **BC1-W1-TOKEN components
intact** — `LaunchResult.PaneKernelStartToken`, the `ProcessStartToken(identity.PanePID)`
capture, the field-22/field-22 comparison, tmux `#{pane_start_time}` excluded from W1
as liveness metadata only, the empty-token `!= ""` guard (A3''), the same-pid
stale-token negative (A3') — none of these are removed or weakened by the
capture-boundary fix? (2) **Connect-out topology + named plumbing sites** still
intact (no fd crosses the tmux boundary; the daemon captures the pane identity with
the capture-boundary re-verification BEFORE any channel binding; the listener still
racing-safe against the lease lifecycle)? (3) **Non-secret address + post-auth nonce
(W3), W2 ordering + dumpable-before-dial, `#{pane_dead_status}` backstop + C2** —
still intact and not weakened by the capture-boundary change? (4) **BC2/BC3** —
artifact identity still derived from daemon `expected_artifacts` state, `CapabilityReseal`
still a daemon-internal marker, frame still carries no job_id/path/kind/body; does
`resealInFlightJob` still find the bound session/job from daemon state, not from the
channel frame? (5) **BC4/BC5** — `jobs.recovery_generation` owner-bundle-0021 column
+ stamped-generation guard; `leases.reseal_grace_extended_at` in the same owner
bundle 0021 — `leases` owner-held; the corrected `work.complete` skip/replace/replay
lock-order map so expired-beyond-grace ALWAYS routes the typed class — still intact?
Does the capture-boundary change introduce any new ordering issue between the
liveness re-verification and the lease/channel lifecycle? (6) **Daemon-observed
positive intent + backend-gate bypass** — still intact? (7) **F7 file-mirror half,
AF1, AF4** — intact? (8) **The modified-since-baseline authored-path fold-in** —
the deliverable-observed condition still reuses `gitChangedPathSnapshots` /
`collectInScopeAuthoredPaths` attributed authorship (nil baseline for isolated
worktrees), so an unchanged pre-existing expected path is NOT resealed? Then hunt for
any **NEW lifecycle gap** the v7 capture-boundary fix introduced — e.g. a liveness
re-verification that itself introduces a TOCTOU (verify-then-stamp window), a
fail-closed path that does not route the typed floor but leaks a raw error, a case
where the pane-died re-verification cannot be completed (tmux unavailable at that
point) and the spec is silent on the failure mode, or a split-brain where the capture
re-verification races against the recovery sweep.

**THEN, regardless of lens, hunt for any NEW material gap** the revision introduced
or left, pressing hardest on the **security invariant**: no admin-token widening (no
lane-readable credential carrying `{admin, apply, recovery, surgical_recovery}`), **no
replay** (no same-uid-reachable channel a sibling lane can present — must hold
STRUCTURALLY on the REAL tmux launch path with W1's kernel token structurally bound to
the launched wrapper via capture-boundary re-verification, not as a temporal promise),
and **no split-brain** (no reseal into a session/job the daemon retired). Use these
lenses:

1. **Trust-model widening (the hottest dimension).** Show ANY path where the chosen
   mechanism lets a lane read the daemon's full-authority bootstrap admin client-token
   (`go/pkg/admin/bootstrap.go:18-27`), or where a new lane-readable credential could
   present `admin` / `apply` / `recovery` / `surgical_recovery`. Any such path is a
   landed falsification.
2. **Same-uid channel replay / false provenance on the REAL path.** Show where the
   connect-out W1 check is defeatable by a same-uid sibling — a capture-boundary
   re-verification that is itself racy (verify-then-stamp window introducing a second
   TOCTOU), a kernel token stamped before liveness re-verification completes, a
   connect-out socket a sibling can connect to that the daemon accepts without the
   launched wrapper being structurally identified at capture time, or the
   nonce/fd readable via `/proc` before `PR_SET_DUMPABLE(0)`.
3. **Split-brain across the rotation.** Show a case where a reseal writes into a
   session/job the daemon retired across the boot-epoch rotation (the generation guard
   or the lock order failing to serialize reseal vs the recovery sweep).
4. **Option-4 "loud failure" that is still silent / leaks a raw error.** Show where
   the typed `session_unrecoverable_across_rotation` class is not actually routed —
   e.g. a capture-boundary miss that produces a raw launch/control error instead of
   the typed floor, a post-exit reseal that leaks `invalid_transition` / a raw
   `lease_error` / backend error, or a primary connect-out entry point that silently
   never fires so the design leans on the backstops while claiming a working primary.
5. **A constraint "resolution" that is hand-waving** — a fix stated without a
   mechanism (no named code site, no real-path test, a capture-boundary re-verification
   that is only temporally defended and not liveness-checked, an unanchored claim that
   the window is small enough to be structurally safe), or one that breaches the
   Non-Goals (RFC 0152 / D249; #537 / #539; #513) or the product boundary.
6. **Boot-epoch / mirror interaction bug.** Show where the F7 file-mirror endpoint/
   epoch half is weakened, or where the survival mechanism contradicts the #316
   recycled-port defense or the #323 endpoint-rotation recovery.

For each challenge record: the precise claim attacked, your concrete refutation (with
file:line / mechanism), the strongest rebuttal you can honestly construct on the
Holder's behalf, and whether a real gap remains. Default to skeptical: for a
trust-model change, an unproven safety claim is a standing falsification.
