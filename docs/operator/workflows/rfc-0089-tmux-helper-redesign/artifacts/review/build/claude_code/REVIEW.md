---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0089", "phase-1", "review", "tmux", "ergonomics_dx", "build"]
---

# Build Review finding: RFC 0089 Phase 1 (ergonomics_dx)

author: reviewer-claude-opus-4.7-001
role: reviewer
lane: claude_code
date: 2026-05-28
review_posture: ergonomics_dx
target_branch: striatum/rfc-0089-tmux-helper-redesign
review_inputs:
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign/TASK.md`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
- `docs/operator/workflows/rfc-0089-tmux-helper-redesign/artifacts/build/HANDOFF.md`
- working-tree diff (60 files; 1239 insertions / 516 deletions on `striatum/rfc-0089-tmux-helper-redesign`)

## Verdict

**accept_with_findings** — Phase 1 ergonomics goals are met. Operators can:
discover the tmux attach command (`tmux.attach_command`), distinguish a live
lane from a transient attach client (`tmux.liveness.class` + structured
`pid_identity`), see the structured failure classes in primary read surfaces,
and infer fallback when tmux is unavailable (`tmux.state == "unavailable"` +
`unavailable_reason` + `remediation`). The findings below are surface polish
that does not block acceptance.

## Posture summary

The four ergonomics_dx axes from the packet:

| Axis                                | Status   | Where exposed                                                                          |
|-------------------------------------|----------|----------------------------------------------------------------------------------------|
| Attach command visibility           | **good** | `supervise.status.tmux.attach_command`, copy-pasteable shell string                    |
| Failure classes operators can act on| **partial** | `tmux.liveness.class` (5 classes), `reattach_state` + `recommended_action` in `reattach_status` |
| Fallback behavior (plain PTY)       | **good** | `tmux.state = "unavailable"`, `unavailable_reason`, `remediation` text                 |
| Status / dashboard clarity          | **partial** | `supervise.status`, `status`, `dashboard_all`, `doctor` all route through the probe; remediation is asymmetric across classes |

## Findings

### F1 — Only `tmux_unavailable` carries a `remediation` string in `supervise.status`

Severity: **low**. Posture: ergonomics_dx.

**Observation.** `reads/supervision.go::attachTmuxLiveness` only writes
`tmux.remediation` when `live.Class == TmuxLivenessUnavailable`. The other
four failure classes (`tmux_session_missing`, `tmux_pane_missing`,
`tmux_pane_dead`, `tmux_pane_pid_mismatch`) surface only the class name and
the `tmux.liveness` payload. A first-time operator who runs `striatum
supervise status … --json` and sees `"class": "tmux_pane_dead"` has the
diagnosis but no copy-pasteable next step.

```go
// go/pkg/reads/supervision.go (attachTmuxLiveness)
if live.Class == string(gosupervisor.TmuxLivenessUnavailable) {
    tmux["remediation"] = "install tmux on the daemon host or set STRIATUM_TMUX_PROBE_DISABLE=1 as a temporary rollback"
}
```

The reattach projection (`reads/supervision.go::reattachState`) does map
each tmux failure class to a structured `recommended_action`
(`mark_lost_or_reconcile`, `verify_before_reattach`, `reattach`), so the
information exists — it just isn't surfaced on the primary status endpoint.

**Why it matters for ergonomics_dx.** RFC 0089 §"Probe tmux directly for
liveness" promises that "these classes appear in `supervise.status`,
`doctor --verbose`, status next-actions, and recovery sweep details."
"Next-actions" is the operator-facing affordance, and today the next-action
is implicit for four of the five classes.

**Suggested follow-up (non-blocking).** Extend `tmuxUnavailableRemediation`
(or a sibling function) to cover the lost-class set with short remediation
text. Suggested mappings:

| Class                       | Suggested remediation                                                   |
|-----------------------------|-------------------------------------------------------------------------|
| `tmux_session_missing`      | "lane is gone; call `striatum supervise stop --reason tmux_session_missing` then restart the lane" |
| `tmux_pane_missing`         | "tmux session exists but the lane pane is gone; restart the lane"       |
| `tmux_pane_dead`            | "lane process exited inside tmux; review `supervise.reattach_status` then restart" |
| `tmux_pane_pid_mismatch`    | "pane was replaced (likely respawn); call `supervise stop` to clear the supervisor row and relaunch" |

### F2 — `recommended_action` is on `supervise.reattach_status`, not on `supervise.status`

Severity: **low**. Posture: ergonomics_dx.

**Observation.** `reattach_state` and `recommended_action` are calculated in
`reattachState()` and exposed by `HandleSuperviseReattachStatus`, but
`HandleSuperviseStatus` does not fold the latest reattach view into its
response. An operator following the RFC 0089 §"Route operator attach through
metadata" recipe (`supervise status --json` → `tmux attach-session`) sees the
failure class but does not see that there is a separate `reattach_status`
endpoint where the next-action lives.

**Why it matters for ergonomics_dx.** Discoverability — the first endpoint
an operator reaches is `supervise.status`, and `recommended_action` only
appears one verb away.

**Suggested follow-up (non-blocking).** Surface
`reattach_state` + `recommended_action` (the same fields already emitted by
`HandleSuperviseReattachStatus`) on `HandleSuperviseStatus` for tmux-backed
lanes. The data is already computed in the same response path
(`reattachStatusRows(...)` is already called on lines 152–154 and used for
`lane_attestation_reason`); promoting the two action fields up to the
top-level is a small read-projection change, not new state.

### F3 — Plain-PTY fallback is signalled by *absence*, not by an explicit `backed_by` marker

Severity: **low**. Posture: ergonomics_dx.

**Observation.** When `launchPTY` falls back to `launchPlainPTY` (no tmux,
or `tmux_identity_capture_failed`, or missing `STRIATUM_RUN_ID`), the
metadata block correctly records `tmux.state = "unavailable"` with
`unavailable_reason` and a remediation string. There is no positive
`tmux.backed_by` / `lane.transport` field that says, "this lane is on a
plain PTY; you cannot `tmux attach-session` to it." Operators must infer
fallback from the absence of `attach_command` and `session_name`.

The underlying `LaneLiveness.Backed` value (`"tmux"` vs `"plain_pty"`) is
already computed in `ProbeLaneLiveness` and is the single load-bearing
distinction here, but it is dropped when the metadata is projected.

**Why it matters for ergonomics_dx.** A `tmux.state` value of
`"unavailable"` next to a stale `attach_command` is a known footgun shape —
docs/scripts that key off `attach_command != ""` will silently work in
tmux mode and silently break in fallback mode.

**Suggested follow-up (non-blocking).** Project `LaneLiveness.Backed` onto
the `tmux` block in `tmuxMetadata` and `attachTmuxLiveness` (e.g.
`tmux.backed_by: "plain_pty"` for the fallback case). The
`TmuxLivenessPayload` builder already writes `"backed_by": "tmux"` into the
liveness sub-object, so this is a consistency fix, not a new contract.

### F4 — `attach_client_exited` event vs `agent_exited` event — operators need a single dashboard hint

Severity: **low**. Posture: ergonomics_dx.

**Observation.** `supervisor/helper.go::attachClientExitPayload` correctly
distinguishes a transient attach-client exit (tmux still alive → emit
`HelperEventAttachExited` and move the supervisor pointer toward `detached`)
from a real lane death (`HelperEventAgentExited` with optional `cause:
tmux_<class>`). `supervision_control.go` then refuses `supervise.send` on
a `detached` supervisor with the message `"supervisor needs reattach
before delivery (supervisor_id=…, state=detached)"`. This is exactly the
ergonomic that the RFC promised. **Good.**

The remaining ergonomics nit is that the **dashboard / status read
projections do not yet surface a `detached` hint distinct from `attached`
beyond the bare state string** (`reads/status.go:212` and `dashboard.go:154`
still filter on `ps.state = 'attached'` for the lane-attestation badge,
which is correct for attestation but means a `detached` lane is rendered as
"no live attached supervisor" without explaining that the tmux session is
still alive). For the operator looking at the dashboard, "is my lane still
running inside tmux?" is the question, and the current dashboard answer is
implicit.

**Suggested follow-up (non-blocking, fits Phase 2 read-surface work).** In
`dashboard.go` and `status.go`, when the underlying tmux liveness is
`tmux_ok` but `ps.state = 'detached'`, render a distinct
`lane_status = "detached_but_alive"` (or equivalent) instead of falling
into the same `no_attached_supervisor` bucket. This is the dashboard-
clarity counterpart to F2 on the JSON side.

### F5 — `pane_start_token` is captured but not surfaced in `tmuxMetadata` curated projection

Severity: **low** (informational). Posture: ergonomics_dx.

**Observation.** `pty.go::tmuxBackedMetadata` records `pane_start_token`
when available, and `tmux_liveness.go::ProbeTmuxLiveness` uses it to drive
the `tmux_pane_pid_mismatch` classification. The curated metadata projection
(`reads/supervision.go::tmuxMetadata`) does include `pane_start_token` in
its allowlist — so this is not a leak, it is exposed. The ergonomics
asymmetry is that the **liveness probe** records `observed_pane_start` in
`tmux.liveness.observed_pane_start` but the **identity capture** records the
expected token under `tmux.pane_start_token`. When the two diverge (the
`tmux_pane_pid_mismatch` case), an operator has to compare the two strings
themselves with no narrative ("expected X, observed Y"). For a posture
focused on "what do I do next," this is at the threshold of useful detail
vs noise; I am flagging it informationally rather than as a fix.

## What is solid

- **Attach command is a copyable shell line.** `tmux.attach_command =
  "tmux attach-session -t <name>"` is the exact form the RFC describes.
  Operators copy/paste; the daemon's MCP session continues to drive workflow
  state regardless of attach.
- **Failure classes are a closed enum.** `TmuxLivenessClass` is a named-
  string set, not free text, and it is consistent across `supervise.status`,
  `supervise.reattach_status`, `doctor`, `dashboard`, and
  `recovery_process_reconcile`. A future operator tool can switch on the
  set without parsing prose.
- **`tmux_unavailable` is non-fatal.** `liveness.go::Liveness.run` treats
  `TmuxLivenessUnavailable` as a transient probe-skipped state for up to
  three ticks before marking the supervisor lost
  (`tmux_unavailable_persistent`), which is the correct
  "operator-actionable, not punitive" shape for a probe that can fail
  for tmux-server flakes. The `remediation` string is the one place the
  read projection answers "what do I do next" today, and it does it
  competently.
- **D028 / private diagnostics are preserved.** `tmuxMetadata` is an
  allowlist projection (`for _, key := range []string{...}`); raw pane
  text and PTY-log content do not flow through it. The metadata-only
  contract is structurally enforced rather than relying on reviewer
  vigilance.
- **Stop-path actually kills the tmux session.** `stopTmuxBackedLane`
  routes through `tmux kill-session` first, with a `terminateProcess(panePID)`
  fallback that fires only when `kill-session` itself errors, and an
  attach-client-pid teardown that fires when the attach pid is distinct
  from the pane pid. This is the behaviour the RFC §"Acceptance Criteria"
  bullet on `supervise.stop` requires.

## Interrogation outcome

Interrogation `intg_d2a22b3735a3265cf4221eb902397700` was opened against
the live codex builder session
(`sess_035ec75a35be4f74b5e83b6e21f96138`) at 2026-05-28T16:01:30Z with
topic `"RFC 0089 Phase 1 build review - ergonomics_dx"`. The single
question (Q0 above, posted at 16:01:40Z) asked the implementer to walk a
first-time operator through the status JSON and to enumerate the
operator-facing next-action per failure class — the
ergonomics_dx-defining question for this review.

**No answer turn was delivered within the review window.** The target
session was already in `stall_class: agent_question_pending` since
2026-05-28T16:01:26Z (target has an outstanding `session.report` of kind
`question` to the operator and is paused on its own `attention` deadline).
A concurrent threat-model interrogation by the codex reviewer
(`intg_fbc8795f8d00015abf1bd8ae651999d0`) experienced the same blockage and
closed at 16:03:40Z with zero answers. The agy reviewer's interrogation
preceded the stall and completed three rounds normally
(`intg_da8fa7445604984ba46973318812e231`).

This interrogation was therefore closed with
`reason: target_unanswerable_stalled_on_session_report_question` to free
the review lease. The full curated chat log is in
`INTERROGATION_CHAT.md`. Because the verdict is `accept_with_findings`
and rests on close code reading of the diff and the existing curated
documentation (HANDOFF.md, RFC 0089, RFC 0088 P3 FINDINGS.md, and the
authoring diff itself), the missing answer is **not load-bearing** for
acceptance; it would have refined the F1/F2 findings but would not have
overturned them. The findings above are derived from the implementation
surfaces, not from interrogator-provided claims.

Rounds completed: **0 of up to 3 permitted**. Stop reason:
`target_unanswerable_stalled_on_session_report_question`.

## Verification run

From `go/`:

| Command                         | Result                          |
|---------------------------------|---------------------------------|
| `gofmt -l .`                    | clean (no output)               |
| `go vet ./...`                  | clean (no output)               |
| `go test ./...`                 | PASS — 31 packages, 0 failures  |

Targeted re-runs of the touched packages
(`./pkg/supervisor/... ./pkg/mutations/... ./pkg/reads/...`) also pass.
This matches the HANDOFF.md verification claim and confirms the
implementer's own gate.

## Decision

`accept_with_findings`. The five low-severity findings (F1–F5) are
read-surface polish items that fit naturally into Phase 2 ("Make attach
commands first-class in reads") and do not block Phase 1's promise:
attach-as-liveness is gone, the tmux session/pane is the lane identity,
and operators can read the structured liveness classification on every
documented surface (`supervise.status`, `supervise.reattach_status`,
`doctor`, `dashboard`, `recovery_process_reconcile`).
