# Interrogation Chat Log — Build Review (ergonomics_dx)
author: reviewer-claude-opus-4.7-001

- **Interrogation ID**: _not_opened_
- **Target Session ID**: `sess_be356331bb2d0e88ef78541a9d2cfad2` (codex_builder, attested, alive)
- **Interrogator Session ID**: `sess_7ebc9bec432451c932499bd292dbcce5`
- **Posture**: `ergonomics_dx`
- **Round Count**: 0
- **Stop Reason**: `interrogator_capability_denied`

---

## Why this chat log is empty

This curated chat log records that the build-review interrogation against
the live codex builder could not be opened, and why.

Three `interrogation.open` calls against
`sess_be356331bb2d0e88ef78541a9d2cfad2` returned `capability_denied`.
`HandleInterrogationOpen` (`go/pkg/mutations/interrogation.go:49`) requires
the interrogator session to carry the `interrogate` capability in
`capabilities_json`. `list.sessions` reports this session's
`capabilities_json` as `[]`, even though `workflow.json` declares
`lanes.claude_code.capabilities = ["write","review","synthesis","interrogate"]`.

The target is healthy and reachable: `supervise.status` returns
`state=attached`, `liveness=alive`, `lane_attestation=attested`, with a
recent `last_await_packet_at`. The block is purely the capability check on
the interrogator side; it is not a target window-closure, lease, or
liveness problem.

A `session.report` with `report_kind=question` was filed at 17:09Z
(`phase=lease_held`) so the operator can either grant the capability and
re-run, or accept the verdict on the sibling interrogations' coverage.

## Sibling interrogations already on record

Both peer reviewers already completed live interrogations against the same
codex builder target, before this review's window opened. Their curated
chat logs cover the territory a live ergonomics_dx interrogation would
otherwise overlap with the structural and platform-risk angles:

- **devils_advocate** — `intg_90d17b324a97c16e7be2a144eca4a5e4`
  (agy reviewer `sess_09f5a1fa94e4006e7016e9f393e9936d`, 6 turns,
  opened 17:00:10Z, closed 17:04:18Z). Builder conceded:
  (i) helper-owned attach bridge exit currently leaves the lane
  publishable as `attached` for delivery without a `delivery_degraded`
  transition (silent packet-delivery blackhole risk);
  (ii) `remain-on-exit` is set imperatively after `new-session` and races
  with early-bootstrap crashes;
  (iii) start-token verification degrades to `start_token_unverified`
  silently on non-Linux / older tmux, weakening the stale-pid guard.
- **threat_model** — `intg_5c90667022e4c7e258f3bd2116a41059` (codex
  reviewer `sess_b516b3824e7c042b9508b4f9bc558bfe`, 4 turns,
  opened 17:02:11Z, closed 17:05:49Z).

The full curated transcripts are in the sibling review directories:
[../agy/INTERROGATION_CHAT.md](../agy/INTERROGATION_CHAT.md) and
[../codex/INTERROGATION_CHAT.md](../codex/INTERROGATION_CHAT.md).

## Questions that would have been asked

For the operator's reference (and so a re-claim can pick these up directly
if the capability is granted), here are the three ergonomics_dx questions
this reviewer had drafted against the live builder:

1. **Attach command discoverability.** The tmux metadata exposes
   `attach_command` and the dashboard read projection now carries it. From
   a first-time operator's perspective, the compact `striatum dashboard
   --run-id <id>` view we point new operators to — does it render
   `attach_command` as a visible, copyable column for tmux-backed lanes, or
   does the operator need to drop to `--json` and pipe through `jq`?
   If the latter, what's the intended discoverable path before Phase 2's
   read-surface work lands?

2. **Failure-class actionability.** Only `tmux_unavailable` carries a
   `remediation` hint in `tmux.liveness`. For `tmux_session_missing`,
   `tmux_pane_missing`, `tmux_pane_dead`, and `tmux_pane_pid_mismatch`,
   what's the operator's next action — is there any CLI/MCP verb that
   drives "stop the lost lane and relaunch" cleanly, or does the operator
   have to manually `supervise.stop` and then re-claim via the workflow?
   And: the `supervise.send` "needs reattach before delivery" error
   points at a recovery verb that isn't visible — what is the intended
   recovery command?

3. **Fallback + degraded heartbeat visibility.** Two ergonomics moments:
   (a) The liveness loop tolerates up to 3 consecutive `tmux_unavailable`
   ticks before marking the supervisor lost and writes a
   `tmux_probe_skipped_at` timestamp each tick. Is the consecutive count
   visible to the operator anywhere, or only the most recent timestamp?
   (b) After the helper-owned attach bridge exits with `tmux_ok`, the
   supervisor stays `attached` while packet delivery may already be
   degraded. From the ergonomics angle: is there any operator-facing
   surface that distinguishes "tmux pane fine, byte path may be wedged"
   from "fully healthy", or does an operator only discover it by trying
   to send a packet and getting a stale error?

These questions are recorded in the REVIEW.md findings as items 1, 2/4, and
3/5 respectively, with proposed resolutions, so the verdict is not blocked
by the absence of a live answer.
