---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# Interrogation chat - RFC 0089 final build review (claude_code, ergonomics_dx)
author: reviewer-claude-opus-4.7-001

- Interrogation id: `intg_b1d5d5681deb483c06f6ba590f89bbd2`
- Interrogator session: `sess_8bd2769163d707ce0268d28830cded6f` (reviewer-claude_code-1)
- Target session: `sess_ac823cc767750f0857cd4a9fa2ced765` (implementer-codex_builder-1, Codex GPT-5.5 xhigh)
- Topic: RFC 0089 final build review (ergonomics_dx posture)
- Opened at: 2026-05-28T21:39:31Z
- Closed at: 2026-05-28T21:43:46Z (reason: `interrogation_completed_with_findings`)
- Rounds: 3

This is a curated transcript. Raw terminal/tmux/PTY output is not included.

## Round 1 - delivery_liveness operator surface clarity

**Reviewer (claude_code, ergonomics_dx):**
The final build promotes `delivery_liveness` to a top-level field on
`supervise.status`, `dashboard`, and `dashboard_all`, and `supervise.send`
now refuses with `supervisor delivery is degraded: attach_client_exited` when
the helper-owned bridge has exited but the tmux pane is still alive.
Inspecting your live supervisor right now, `supervise.status` returns
`state=attached, liveness=alive, lane_attestation=attested,
tmux.liveness.class=tmux_ok` but `delivery_liveness.class=degraded` with
`reason=attach_client_exited`.

From a first-time operator's perspective walking up to
`striatum dashboard --run-id <run> --once` (the compact terminal view, not
raw JSON): what specifically appears next to the codex_builder lane today to
signal that pane is fine but delivery is degraded? Is there a single
column/badge that says "delivery degraded", or does the operator have to
read the JSON projection to see the difference between pane health and
delivery health? If the latter, is there a concrete plan in this build (or
an issue ID) to bring that signal into the compact view, and what would you
recommend the v1 compact-view representation be?

**Codex builder (implementer):**
I checked the live dashboard command before answering. Today, the signal is
present in the dashboard data, but not as a dedicated human-friendly compact
badge/column. For the codex_builder session, the dashboard output includes
top-level `delivery_liveness={class:degraded, healthy:false,
reason:attach_client_exited}`, while the same session still shows liveness
attention/no_lease, `lane_attestation=attested`, and
`tmux.liveness.class=tmux_ok`. The tmux block also repeats
`tmux.delivery_liveness` and `attach_client_last_exit`.

So a JSON reader can distinguish pane health from delivery health today, but
a first-time operator using a compact terminal view does not get a single
obvious "delivery degraded" badge next to codex_builder. They must notice
the `delivery_liveness` field in the JSON/projection. That is why I left
this as a non-blocking Phase 2 operator-polish finding in the handoff rather
than claiming the compact view is solved.

I do not have a separate issue id from this build beyond the RFC 0089
Phase 2 / read-surface follow-up and the handoff finding. My recommended
v1 compact representation is an explicit per-session field separate from
pane liveness, for example: `pane=tmux_ok delivery=degraded:attach_client_exited attestation=attested`.
If the compact renderer has fixed-width status badges, I would render
`DELIVERY DEGRADED` in warning color and keep `TMUX OK` / `ATTESTED` separate,
because collapsing them into one lane status would recreate the ambiguity
RFC 0089 is trying to remove.

## Round 2 - recovery affordance from the operator's terminal

**Reviewer (claude_code, ergonomics_dx):**
In this build, `supervise.send`'s refusal message now reads
`supervisor delivery is degraded: attach_client_exited`
(`HandleSuperviseSend`'s `invalid_transition` branch). The detached-state
branch reads
`supervisor is detached; stop this supervisor and restart/reclaim before delivery (supervisor_id=...)`.

The "stop + restart/reclaim" phrasing names an *operation* but no concrete
verb. Concretely:

- (a) For the degraded case (pane alive, helper bridge exited): what is the
  exact CLI verb sequence the operator should run today to clear the
  degradation? Is `striatum supervise stop --session-id <id>` followed by a
  workflow re-claim the canonical path, or is there a `supervise restart` /
  `reattach` verb in this build that I missed? If neither, has this build
  added the recovery path to the daemon runbook or CHANGELOG so a first-time
  operator can find it without grepping source?
- (b) For the four terminal tmux failure classes (`tmux_session_missing`,
  `tmux_pane_missing`, `tmux_pane_dead`, `tmux_pane_pid_mismatch`): the
  doctor reads now emit problem strings like
  `supervisor_liveness.<id>: tmux_pane_dead`. Looking at the build's
  `attachTmuxLiveness` projection, the `remediation` hint is only set when
  `live.Class == TmuxLivenessUnavailable`. Was the four-class remediation
  gap left intentionally for Phase 2, and if so, what should the per-class
  hint actually say? I want to record the per-class hints you would ship if
  this were not deferred, so the finding is concrete rather than gesturing.

**Codex builder (implementer):**
(a) There is no `supervise restart` or `supervise reattach` mutation verb
in this build. `supervise.reattach_status` is read-only. The concrete
recovery path today is stop then start/reclaim, using existing verbs. For
the same still-active session, the direct sequence is:

```text
striatum supervise stop --session-id <session_id> --reason "delivery_liveness_degraded: attach_client_exited"
striatum supervise start --session-id <session_id>
```

If a packet failed delivery, then resend that packet with the packet id
from the failed send / work packet:

```text
striatum supervise send --session-id <session_id> --packet-id <packet_id>
```

If the workflow has already requeued or the lease was recovered, return to
the normal claim/await path instead of inventing a reattach operation. For
a completed / interrogation-only builder like this session, there may be no
packet to resend; the stop/start path is still the current way to clear the
degraded bridge if further daemon-to-lane delivery is required. This build
updates SPEC and the command-authority matrix to state that delivery remains
degraded until rebridge/restart, but it **does not add a daemon-runbook or
CHANGELOG section with the exact operator sequence**. That is a real
ergonomics gap.

(b) Yes, the four terminal-class remediation hints were left for Phase 2 /
operator polish. `attachTmuxLiveness` currently sets `remediation` only for
`tmux_unavailable`. The concrete hints I would ship are:

- `tmux_session_missing`: "The recorded tmux session is gone. Stop the
  supervisor to mark this lane terminal, then start/reclaim a replacement
  lane: `striatum supervise stop --session-id <id> --reason tmux_session_missing`;
  `striatum supervise start --session-id <id>`. Do not send packets to the
  old supervisor."
- `tmux_pane_missing`: "The tmux session exists but the recorded pane id no
  longer resolves. Treat the lane identity as broken; stop the supervisor
  and start/reclaim a replacement. Do not infer identity from another pane
  in the session."
- `tmux_pane_dead`: "The agent process exited and `remain-on-exit` retained
  the pane for private diagnostics. Inspect locally if needed, then stop
  the supervisor and restart/reclaim before sending more work."
- `tmux_pane_pid_mismatch`: "The pane pid / start token no longer matches
  the recorded lane identity, likely external respawn or PID reuse. Treat
  as an authority break: stop the supervisor, do not signal stale pids
  except through token-gated cleanup, and start/reclaim a fresh lane."

For all four, the compact message should name stop + start/reclaim
explicitly, because there is no in-place reattach command yet. A future
rebridge verb can replace the second step once implemented.

## Round 3 - fallback discoverability + tmux_unavailable warning gradient

**Reviewer (claude_code, ergonomics_dx):**
Two ergonomics_dx threads I want to close before verdict:

- (a) Fallback discoverability. When tmux is missing and a workflow does
  NOT set `require_tmux: true`, the build records `tmux.state="unavailable"`
  with `unavailable_reason` in pointer metadata, and your `tmuxMetadata`
  projection sets a remediation hint for that case. But the operator's
  first signal that they are running plain-PTY instead of tmux-backed is
  buried in `supervise.status --json` under `tmux.state`. Two specific
  questions:
  1. Does the compact `striatum dashboard --once` view distinguish plain-PTY
     lanes from tmux-backed lanes today, or do they read identically to a
     first-time operator?
  2. The RFC says fallback "must be explicit in metadata and status so the
     operator can see that the lane is not tmux-attachable" - do you
     consider the current `tmux.state=unavailable` field plus the JSON-only
     remediation hint sufficient to satisfy that acceptance criterion, or
     is a status badge / dashboard column still owed?
- (b) tmux_unavailable warning gradient. The `tmuxUnavailableTicks < 3`
  grace in `liveness.go` keeps the lane alive for up to ~3 ticks before
  marking it lost with `tmux_unavailable_persistent`. The build added
  `probe_unavailable_count` and `probe_skipped_at` to the projected `tmux`
  block (per the `tmuxMetadata` key list). Two questions:
  1. Does the projection actually expose the running counter to operators
     today, or only the most-recent skip timestamp? If both, where exactly
     does a first-time operator see "2 of 3 ticks elapsed" before the lane
     flips to lost?
  2. Should the dashboard project a `liveness: degraded` (not just `alive`
     / `gone`) intermediate when the counter is non-zero but below the
     threshold, so the operator gets a *warning* state before the
     supervisor goes lost? That symmetric warning gradient seems like the
     missing piece relative to the `delivery_liveness` work you just landed.

Keep answers concrete (yes/no with the field name or projection location),
so the finding lands as a precise gap to close in Phase 2 rather than a
wishlist.

**Codex builder (implementer):**
(a1) No, the human compact terminal presentation does not yet give a clear
plain-PTY-vs-tmux-backed badge/column. The read model distinguishes them:
tmux-backed rows have a tmux object with `state=backed` plus
`attach_command`/session/pane fields; tmux fallback rows have
`tmux.state=unavailable` and `unavailable_reason`/`remediation`; plain
non-tmux metadata may have no tmux object. But a first-time operator has to
inspect the JSON/projection to see that.

(a2) I would call the current field sufficient for the narrow machine /
read-model acceptance criterion that fallback is explicit in metadata and
status. I would not call it sufficient for operator ergonomics in the
compact dashboard. A Phase 2 dashboard/status badge is still owed. The v1
compact representation should be a separate attachability field, e.g.
`transport=tmux attachable`, `transport=plain-pty no-tmux`, or
`tmux=unavailable:<reason>`, and it should not be overloaded into
`lane_attestation` or protocol liveness.

(b1) The counter is exposed today in the JSON projection as
`tmux.probe_unavailable_count`, with `tmux.probe_skipped_at`,
`tmux.last_ok_at`, and `tmux.last_unavailable_detail`. That comes from
`recordTmuxProbeUnavailable` in `go/pkg/supervisor/liveness.go` and
`tmuxMetadata` in `go/pkg/reads/supervision.go`. There is no explicit
"2 of 3" text or max/threshold field; the operator only sees
`probe_unavailable_count: 2` if they inspect JSON. `supervise.status` may
also run a fresh probe and show `tmux.liveness.class=tmux_unavailable` when
tmux is currently unavailable, but the warning-gradient display is not
packaged as a compact operator hint.

(b2) Yes, dashboard/status should project a degraded intermediate before
lost. Today the data needed for that exists, but the liveness vocabulary
shown to operators is still essentially `alive`/`stalled`/`gone` plus the
nested tmux fields. The Phase 2 gap I would record is: when
`tmux.probe_unavailable_count > 0` and below the loss threshold, render
`pane_liveness=degraded` or `tmux_probe=degraded:N/3`, keep row state
`attached`, and show `last_ok_at` plus `last_unavailable_detail`. That
would mirror `delivery_liveness` and make transient tmux probe failure
visible before `tmux_unavailable_persistent` marks the supervisor lost.

## Curation note

This transcript records the literal Q/A turns exchanged through
`interrogation.ask` / `interrogation.show`. No raw terminal output, tmux
pane capture, or PTY log content was inspected or copied here (D028).
