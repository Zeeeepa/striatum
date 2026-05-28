---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics-dx", "build-review", "tmux-liveness", "dashboard", "attestation"]
---

# Build Review finding: RFC 0089 Phase 1 Build (rerun2, ergonomics_dx)
author: reviewer-claude-opus-4.7-001
verdict: accept_with_findings
round_count: 0
stop_reason: interrogator_capability_denied

## Summary

This is the ergonomics_dx build review of RFC 0089 Phase 1: replacing
attach-as-liveness with tmux session/pane liveness. The substrate compiles,
passes static checks, and passes the entire Go test suite (see §1). The
review angle is developer-ergonomics: from a first-time operator's
perspective, is the new tmux-backed lane discoverable, are its failure
classes actionable, and is the fallback path explicit?

The implementation lands the core substrate cleanly. Five ergonomics gaps
remain that operators will hit during normal use; they are non-blocking for
Phase 1 acceptance but should be tracked into Phase 2 / runbook updates
before universal tmux defaults flip.

Final verdict: **accept_with_findings**.

A live peer interrogation of the codex builder could not be opened (see §3).
The two sibling interrogations (`devils_advocate` and `threat_model`) were
already completed against the same target before this review ran; their
artifacts on disk cover the structural and platform-risk territory, so the
operator already has a substantive live exchange on record for this build.
This review's findings are drawn from a direct read of the diff, the
builder's HANDOFF.md, and the verification run.

## 1. Verification Run

```text
$ cd go && gofmt -l .
(no output — all formatted)

$ cd go && go vet ./...
(no findings)

$ cd go && go test ./...
ok      github.com/halbritt/striatum/go/cmd/striatum    (cached)
ok      github.com/halbritt/striatum/go/cmd/striatumd   (cached)
ok      github.com/halbritt/striatum/go/pkg/agentloop   0.008s
ok      github.com/halbritt/striatum/go/pkg/mutations   0.087s
ok      github.com/halbritt/striatum/go/pkg/reads       0.049s
ok      github.com/halbritt/striatum/go/pkg/supervisor  0.410s
... (all 40 packages pass)
```

All Go packages compile, `go vet ./...` is clean, `gofmt -l .` reports no
diffs, and the new tmux-liveness, attach-client-exit, and corrupt-metadata
regression tests pass alongside the existing supervisor/mutations/reads
suites.

## 2. Curated ergonomics_dx findings

### Finding 1 (medium): No compact-view operator surface for the attach command

* **Context.** Tmux-backed lanes record a one-shot copy-paste attach command
  in supervisor pointer metadata (`tmux.attach_command =
  "tmux attach-session -t <session_name>"`) and project it through the
  `tmux` block in `supervise.status` JSON, the dashboard run view, and the
  fleet view (`dashboard_all`). The data path is correct.
* **Problem.** The intended operator path in RFC 0089 §3 is:
  `striatum supervise status --session-id <id> --json` followed by manual
  copy of the attach command. From a first-time-operator perspective, the
  compact terminal view we point new operators to —
  `striatum dashboard --run-id <id>` — does not surface the attach command
  as a copyable column; it is only reachable inside the nested
  `tmux.attach_command` JSON field. New operators will either grep JSON with
  `jq` or — more likely — type a bespoke `tmux attach-session` guess.
* **Suggested resolution.** Either (a) render a final `tmux attach …` line
  in the compact `striatum dashboard --once` output next to each
  tmux-backed lane, or (b) add a thin verb such as
  `striatum supervise attach --session-id <id> --print` that prints just the
  command (the RFC's "future polish" line already anticipates this). The
  runbook (`docs/how-to/daemon-runbook.md`) should also gain a
  "how to attach to a live lane" section.

### Finding 2 (medium): Doctor surfaces tmux failure classes without remediation hints for 4 of 5 classes

* **Context.** `doctor` now grows a `supervisors[]` list. Non-OK supervisors
  push a problem string of the shape `supervisor_liveness.<id>: tmux_pane_dead`.
  The `tmux` projection from supervision reads also adds a `remediation`
  hint — but only for the `tmux_unavailable` class
  (`reads/supervision.go` `attachTmuxLiveness` only sets remediation when
  `live.Class == TmuxLivenessUnavailable`).
* **Problem.** For the four remaining classes (`tmux_session_missing`,
  `tmux_pane_missing`, `tmux_pane_dead`, `tmux_pane_pid_mismatch`), the
  operator sees only the class name and no next action. There is also no
  CLI/MCP verb visible to *recover* a lost tmux-backed lane — there is no
  `supervise.relaunch` or similar; the operator has to know to
  `supervise.stop` and then re-claim through the workflow.
* **Suggested resolution.** Extend `attachTmuxLiveness` to fill
  `remediation` for the four lost-class cases too, with concrete next
  actions (e.g., `tmux_pane_dead` → "the agent CLI exited; run
  `striatum supervise stop --session-id <id>` and re-claim the packet";
  `tmux_pane_pid_mismatch` → "the pane was respawned outside Striatum's
  control; mark lost and re-spawn"). Mirror the same hints in the
  doctor problem strings (e.g.,
  `supervisor_liveness.<id>: tmux_pane_dead [run: striatum supervise stop ...]`).

### Finding 3 (medium): "tmux unavailable" tolerance counter is invisible to the operator

* **Context.** The liveness loop in `supervisor/liveness.go` tolerates up to
  3 consecutive `tmux_unavailable` ticks (`tmuxUnavailableTicks < 3`) before
  marking the supervisor lost with reason
  `tmux_unavailable_persistent`. Each skipped tick stamps a
  `tmux_probe_skipped_at` timestamp into the pointer metadata, overwriting
  the previous one.
* **Problem.** During a transient tmux hiccup, the operator has no visible
  signal that the probe is degraded — `supervise.status` will report
  `liveness: alive` because the last good probe still holds. The single
  `tmux_probe_skipped_at` timestamp only tells them when the most recent
  skip happened, not how many consecutive skips have accumulated nor how
  close the lane is to being declared lost. By the time the third skip
  fires, the operator sees a sudden `tmux_unavailable_persistent` with no
  prior warning.
* **Suggested resolution.** Surface the counter
  (`tmux_unavailable_consecutive_ticks`) and a `tmux_last_ok_at`
  timestamp in `supervise.status` JSON and the dashboard `tmux` block.
  Two-tick threshold could light a `liveness: degraded` projection so
  operators see the warning before the lane is lost. (The agy/design review
  already accepted a related "stale-while-revalidate" finding for the
  delivery path; this is the symmetric read-projection ask.)

### Finding 4 (medium): "needs reattach before delivery" error names no recovery verb

* **Context.** `supervise.send` now drains helper events first and, if the
  supervisor moved to `detached`, returns
  `invalid_transition: supervisor needs reattach before delivery (supervisor_id=…, state=detached)`.
* **Problem.** "Needs reattach" tells the operator *what* is wrong but not
  *how* to recover. There is no visible `striatum supervise reattach` verb;
  the operator can grep the codebase for the string and find nothing
  actionable. The error therefore points at a recovery path that the user
  cannot execute today.
* **Suggested resolution.** Either land a thin `supervise.reattach` verb
  (even a stub that calls `stop` + relaunch from the original launch-spec
  snapshot) or change the error to name the actual recovery path:
  `"supervisor detached; run striatum supervise stop --session-id <id> and let the workflow re-claim the packet"`. Document the recovery path
  alongside the failure classes in the runbook.

### Finding 5 (low / cross-reference): No operator-facing "delivery_degraded" signal after helper-owned bridge exit

* **Context.** Both sibling reviewers (devils_advocate by agy; threat_model
  by codex) caught this from their angles; the codex builder formally
  conceded the gap during interrogation. From the ergonomics_dx angle the
  same gap shows up as a discoverability problem: if the helper-owned
  attach bridge exits while the tmux pane is still live, the supervisor
  stays `attached` and `lane_attestation` stays `attested`, but future
  packet delivery may stall on the closed PTY master.
* **Problem.** An operator looking at the dashboard sees `attached / attested`
  and reasonably assumes the lane is healthy. There is no surface that says
  "tmux pane fine, byte path degraded." The first time the operator
  discovers it is when `supervise.send` errors at delivery time.
* **Suggested resolution.** Adopt the builder's conceded "delivery_degraded"
  intermediate state (or `pty_master_closed` lane_attestation_reason) so
  that the dashboard projection flags the gap before the operator tries to
  send. This dovetails with the deferred `tmux send-keys` delivery refactor
  but the read-projection signal can ship independently.

## 3. Interrogation Section

* **Total Rounds:** 0
* **Stop reason:** `interrogator_capability_denied`
* **Interrogation chat log:**
  [INTERROGATION_CHAT.md](INTERROGATION_CHAT.md)

The required peer interrogation against the live codex builder could not be
opened. Three `interrogation.open` calls against
`sess_be356331bb2d0e88ef78541a9d2cfad2` returned `capability_denied`
because this session's `capabilities_json` is empty even though
`workflow.json` declares `lanes.claude_code.capabilities =
["write","review","synthesis","interrogate"]`. A `session.report`
(`kind=question`) was filed at 17:09Z so the operator can decide whether to
grant the capability and re-run, or accept this verdict on the strength of
the two completed sibling interrogations.

The codex builder is live, attested, and reachable
(`supervise.status` confirms `state=attached`, `liveness=alive`,
`last_await_packet_at=2026-05-28T17:09:27Z`); the block is purely the
capability check in `HandleInterrogationOpen`, not a target-side problem.
The two interrogations completed against the same target before this
review's window opened give the operator substantive live coverage on the
structural and platform-risk gaps; the findings above are the
ergonomics_dx complement, derived directly from the diff and the live
verification run.

## 4. Verdict

* **Verdict:** `accept_with_findings`.
* **Justification:** The substrate is correctly implemented: tmux identity
  is captured as the supervised pid, the attach process is demoted to an
  observer with its own `attach_pid`, the five failure classes are
  consistently routed through one liveness probe, and the regression tests
  (`TestRunHelperAttachClientExitWithLivePaneIsNotAgentExit`,
  `TestLivenessMarksLostOnCorruptTmuxMetadata`,
  `TestSuperviseReportAttachClientExitTmuxOKKeepsAttached`) lock the
  intended behavior. The full Go suite passes.
* **Conditions for the findings list to clear:** Findings 1–4 are
  ergonomics polish layered on top of the substrate; they belong in the
  Phase 2 read-surface work + runbook updates that RFC 0089 already
  scopes. Finding 5 cross-references the sibling reviewers' delivery-path
  conceded gap and should land alongside Phase 2.
