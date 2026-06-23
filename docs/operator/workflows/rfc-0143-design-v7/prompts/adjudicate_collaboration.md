You are the **Adjudicator** for the RFC 0143 design run, and **this adjudicates the
SEVENTH-REVISION (v7) dialogue.** Six prior gates ran on this spec: v1 returned
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
stale-token negative (A3')), but returned `needs_revision` on a single ground BOTH
falsifiers landed independently: **BC1-W1-CAPTURE — the kernel start token is
captured POST-LAUNCH and is not structurally proven to belong to the born wrapper.**
Read only the curated dialogue trajectory (the Holder's **revised** (v7) `HOLDER.md`
spec and the falsifiers' `FALSIFIER.md` re-attacks) plus the `SEED.md` charter (whose
`## The binding constraint v7 MUST resolve` section states BC1-W1-CAPTURE with its
prescribed fix, and whose `## Carried forward — resolved by v6` lists the credited
set) and the v6 ledger
`docs/operator/artifacts/rfc-0143-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
for what the revision had to fix. Publish a `collaboration_ledger` artifact whose
verdict reflects whether the revision genuinely resolved BC1-W1-CAPTURE and whether
any **material** new challenge landed and was **directly** rebutted. This is a
security/authz-hot decision: hold the bar high. Do not read raw terminal output.

**First, walk the single remaining binding constraint (BC1-W1-CAPTURE).** Record
whether the revised spec resolves it per its prescribed fix (a fail-closed
capture-boundary re-verification via tmux/liveness that structurally binds the kernel
token to the still-live launched pane + named code sites + the "pane dies before
capture" negative test) or whether it remains open. **BC1-W1-CAPTURE is resolved only
if ALL of the following hold:**

- **(1) A fail-closed capture-boundary re-verification exists.** After
  `CaptureTmuxIdentity` reports `identity.PanePID` and `ProcessStartToken(identity.PanePID)`
  is captured, the spec requires re-verification via tmux/liveness (`ProbeTmuxLiveness`
  states at `tmux_liveness.go:257-269`: pane-dead, pane-pid mismatch, pane-start
  mismatch) that the pane is **still live** and **still the same pane id/pid** at the
  capture boundary **BEFORE** stamping `PaneKernelStartToken` and **BEFORE** any accept
  loop binds the channel. Recall the v6 defect: `launchPTY` starts the pane command
  with `tmux respawn-pane` (`pty.go:479-484`), THEN calls `CaptureTmuxIdentity`
  (`pty.go:493`), THEN `attachTmuxPTY` returns `LaunchResult{PID: identity.PanePID}`
  (`pty.go:517-533`); `ProcessStartToken` reads whatever process currently owns
  `/proc/<pid>/stat` field 22 at read time (`process_identity_linux.go:11-32`). A
  fix that relies only on temporal proximity ("the window is small", "microseconds
  after launch") without a structural liveness re-verification does NOT resolve
  BC1-W1-CAPTURE.
- **(2) The fail-closed path routes to the typed floor.** If the pane is dead,
  missing, pid-mismatched, or identity-unverifiable at the capture boundary, the spec
  must require the helper to NOT stamp the kernel token, accept NO control connection
  for that launch, and route through the `#{pane_dead_status}`/recovery-sweep typed
  `session_unrecoverable_across_rotation` floor — NEVER a raw launch/control error.
  A spec that leaves this as an ad-hoc implementation choice or that may leak a raw
  error does NOT resolve it.
- **(3) The real-path test fires the race.** The spec extends
  `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or adds a paired integration
  negative) with a **"pane dies before kernel-token capture"** case that forces the
  wrapper to exit before W1 capture, makes the token read observe a
  same-pid/reused-process token by stub or controlled hook, and asserts the helper
  REFUSES — not just that the positive case passes. A test that does not fire the
  pane-dies-before-capture path, or that asserts only the positive case, does NOT
  resolve it.
- **(4) The v6 operand-source components are retained.** `LaunchResult.PaneKernelStartToken`
  captured via `ProcessStartToken(identity.PanePID)`, the field-22/field-22 comparison
  via `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`, the empty-token
  `!= ""` fail-closed guard (A3''), and the same-pid stale-token negative (A3'') must
  all be present; removing or weakening any of them is a regression.

A restatement does not resolve BC1-W1-CAPTURE, and a fix that defends only temporally
while the capture can still read a reused-pid process does not resolve it either.

**A clearing verdict requires BC1-W1-CAPTURE resolved AND structural no-replay
established on the REAL channel AND the v6-credited resolved set carried forward
UNREGRESSED** (the v6 BC1-W1-TOKEN clock-domain fix including
`LaunchResult.PaneKernelStartToken` + the empty-token `!= ""` rule + A3'/A3''; the
connect-out topology + named plumbing sites; the non-secret address + post-auth nonce
W3; the W2 ordering + dumpable-before-dial; the `#{pane_dead_status}` exit-code
backstop + C2; BC2, BC3, BC4, BC5; the daemon-observed positive intent; the
`ensureWorkSessionBackend` bypass; the W1/W2/W3 wall shapes; F2, F4, the F7
file-mirror half; AF1, AF4; the no-admin-token-widening invariant; the A1–A18
assertion discipline incl. A3'/A3''/A4'/A7') with the modified-since-baseline
authored-path build-test folded in. Any constraint still open — or only nominally
closed (a "fix" that still relies on temporal proximity without a liveness
re-verification, that does not fail closed to the typed floor on a boundary miss, or
whose real-path test does not fire the pane-dies-before-capture race) — or any
regression of a credited item forces `needs_revision`.

For each falsifier challenge, record in the ledger: the claim challenged, whether the
challenge was material (would change the spec or expose a real security defect),
whether the Holder's spec already rebuts it or it stands unrebutted, and the
disposition.

**Clearing condition (all must hold):** a clearing verdict (`accept` /
`accept_with_findings`) requires (1) **BC1-W1-CAPTURE resolved** with a structural
capture-boundary re-verification via tmux/liveness that binds the kernel token to the
still-live launched pane, AND (2) **structural no-replay established on the REAL
channel** (not as a temporal/probabilistic argument, not as a trackable
post-clearance finding), AND (3) **the v6-credited resolved set carried forward
unregressed** with the build-test folded in, AND (4) **no new material challenge**
standing unrebutted, AND the **security invariant held STRUCTURALLY** — no
admin-token widening, no replay (no same-uid-reachable channel a sibling lane can
present on the real path), no split-brain. If any one fails, the verdict is
`needs_revision` (or `reject` if a path widens admin-token exposure or mints a
credential carrying any of `{admin, apply, recovery, surgical_recovery}`).

Verdict guidance:

- **needs_revision** if any material challenge stands unrebutted — especially: a
  capture-boundary fix that still relies only on temporal proximity without a liveness
  re-verification; a liveness re-verification that itself introduces a second TOCTOU;
  a fail-closed path that does not route the typed floor; a "pane dies before capture"
  negative test missing from the real-path test; a regression of any v6-credited item
  (including the BC1-W1-TOKEN clock-domain fix components — `LaunchResult.PaneKernelStartToken`,
  empty-token rule, A3'/A3''); or any new material challenge that lands. Say exactly
  what the revision must fix. (One revision cycle is available; the falsifiers
  re-attack the revised spec.)
- **accept** / **accept_with_findings** only if BC1-W1-CAPTURE is resolved with a
  structural capture-boundary re-verification, structural no-replay is established on
  the real channel, the v6-credited set is carried forward unregressed, every material
  challenge was directly rebutted or incorporated, the security invariant holds
  structurally (no widening, no replay, no split-brain — enforced, not merely
  promised), the legible-failure path is self-escalating and routed, and each
  load-bearing claim carries a named falsifying test. A clearing verdict is `accept`
  or `accept_with_findings`, never the literal word `clear`.

Note for the ledger (carries regardless of verdict): Slice B (the daemon-internal
`rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
daemon-owned supervisor control channel with connect-out `SO_PEERCRED`
pid+kernel-start-token authentication, the reserved agentloop exit codes, the
`jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021
columns, and endpoint/epoch republish plumbing) is a security/authz trust-model
change requiring **maintainer ratification** before any build slice touches credential
code — the gate clears the *spec's soundness*, not the maintainer's product call.
Slice A (the Option-4 typed-exit-code floor) is zero-trust-change but, per
BC1-W1-CAPTURE, still must route over the connect-out non-PTY channel whose same-uid
authentication (W1) is **specified with one coherent kernel identity token
STRUCTURALLY BOUND to the launched wrapper** before it lands.

The ledger verdict — not falsifier completion — clears the phase gate.
