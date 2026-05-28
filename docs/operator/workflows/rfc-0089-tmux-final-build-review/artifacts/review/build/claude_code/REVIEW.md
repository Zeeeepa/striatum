---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics-dx", "build-review", "rfc-0089", "delivery-liveness", "tmux-liveness", "dashboard"]
---

# Final build review finding: RFC 0089 (claude_code, ergonomics_dx)
author: reviewer-claude-opus-4.7-001
verdict: accept_with_findings
round_count: 3
stop_reason: interrogation_completed_with_findings

## Summary

This is the ergonomics_dx final build review of RFC 0089: tmux-backed lane
monitoring with delivery-liveness separation. The substrate compiles, the
targeted test suite passes (§1), and the live codex builder supervisor is a
working proof of the central architectural fix: a helper-owned attach-bridge
exit now produces a real `delivery_liveness=degraded` signal that
`supervise.send` refuses, instead of a false-healthy lane (§2).

The review angle is developer-ergonomics: from a first-time operator's
perspective, are the new failure classes discoverable, are the recovery
verbs nameable, and does the compact terminal view surface the
delivery-vs-pane health distinction the substrate now models internally?

The interrogation with the live Codex builder (§3, three full rounds)
confirmed the substrate is solid and surfaced four concrete Phase 2
ergonomics gaps - each with builder-supplied resolution copy ready to land.
Final verdict: **accept_with_findings**.

## 1. Verification Run

```bash
$ cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
ok  	github.com/halbritt/striatum/go/pkg/supervisor	(cached)
ok  	github.com/halbritt/striatum/go/pkg/mutations	(cached)
ok  	github.com/halbritt/striatum/go/pkg/reads	(cached)
```

The three core packages required by the workflow prompt are clean.

## 2. Live substrate proof (codex builder supervisor)

`supervise.status --session-id sess_ac823cc767750f0857cd4a9fa2ced765` against
the live builder returned the exact state RFC 0089 prescribes:

- `state: attached`
- `liveness: alive`
- `lane_attestation: attested`
- `tmux.liveness.class: tmux_ok`
- `tmux.pane_pid: 532108`, `tmux.pane_start_token: 231172856`
- `tmux.attach_command: tmux attach-session -t striatum-...-56557e137fc8`
- `delivery_liveness: {class: degraded, healthy: false, reason: attach_client_exited}` (both top-level and under `tmux.delivery_liveness`)
- `tmux.attach_client_last_exit: {attach_pid: 532121, attach_exit_code: 1, observed_at: 2026-05-28T21:32:12Z, tmux_liveness: tmux_ok}`

This is the substrate working as designed: the helper-owned attach bridge
exited at startup (exit code 1), the pane survived, lane attestation was
preserved, and delivery was correctly downgraded to a refusal state - the
exact false-healthy gap the threat_model and devils_advocate reviews of the
rerun2 build flagged as the residual hazard.

## 3. Curated ergonomics_dx findings

Each finding cites a specific build location and (where applicable) the
exact resolution copy the builder offered during interrogation. The
interrogation transcript lives in `INTERROGATION_CHAT.md`
(`intg_b1d5d5681deb483c06f6ba590f89bbd2`, 3 rounds).

### Finding 1 (medium): compact dashboard does not surface `delivery_liveness`

* **Context.** The build promotes `delivery_liveness` to a top-level field in
  `supervise.status`, `dashboard`, and `dashboard_all` JSON projections
  (`reads/supervision.go::attachSupervisorTmux`,
  `reads/dashboard.go`, `reads/dashboard_all.go`). `supervise.send` refuses
  delivery with `supervisor delivery is degraded: <reason>` when the helper
  pipe lacks a reader or the attach bridge has exited
  (`mutations/supervision_control.go::supervisorPipeNoReaderDeliveryError`).
* **Problem (builder-confirmed).** The data exists in the JSON projection
  but the compact terminal view `striatum dashboard --once` shows
  `liveness: alive`, `lane_attestation: attested`, and `tmux.liveness.class: tmux_ok`
  without a dedicated `DELIVERY DEGRADED` badge. A first-time operator
  reading the compact view sees a healthy lane and only discovers the
  delivery gap when `supervise.send` fails later. This recreates a softer
  version of the same false-healthy hazard the substrate was built to fix.
* **Suggested resolution (builder copy).** Add an explicit per-session
  field separate from pane liveness in the compact renderer, e.g.
  `pane=tmux_ok delivery=degraded:attach_client_exited attestation=attested`.
  If fixed-width status badges are used, render `DELIVERY DEGRADED` in
  warning color and keep `TMUX OK` / `ATTESTED` as separate badges - do
  not collapse them into one lane status, because that would re-introduce
  the ambiguity RFC 0089 closed.

### Finding 2 (medium): no documented recovery sequence for delivery_degraded / detached states

* **Context.** `HandleSuperviseSend` returns
  `supervisor is detached; stop this supervisor and restart/reclaim before delivery (supervisor_id=...)`
  for `state=detached`, and
  `supervisor delivery is degraded: attach_client_exited` for the
  pipe-no-reader / attach-exited case. There is no `supervise restart` or
  `supervise reattach` mutation verb; `supervise.reattach_status` is read
  only.
* **Problem (builder-confirmed).** The error names an *operation* but no
  concrete CLI verb sequence. The build updates `docs/reference/spec.md`
  and `docs/reference/command-authority-matrix.md` to note delivery stays
  degraded until rebridge/restart, but `CHANGELOG.md` and
  `docs/how-to/daemon-runbook.md` do not include the exact operator
  sequence. Grep of the runbook shows no mention of `delivery`, `degraded`,
  `attach_client_exited`, or `reattach`. A first-time operator hitting the
  error cold has no documentation path to recovery.
* **Suggested resolution (builder copy).** Document the recovery sequence
  in the daemon runbook and in the next CHANGELOG entry as:

  ```text
  striatum supervise stop --session-id <session_id> --reason "delivery_liveness_degraded: attach_client_exited"
  striatum supervise start --session-id <session_id>
  # if a packet failed delivery:
  striatum supervise send --session-id <session_id> --packet-id <packet_id>
  ```

  Additionally upgrade the `supervise.send` error string from "stop this
  supervisor and restart/reclaim" to name the verbs verbatim.

### Finding 3 (medium): doctor and read projections lack per-class remediation hints for the four terminal tmux classes

* **Context.** `reads/doctor.go::HandleDoctor` now collects a
  `supervisors[]` list from `reattachStatusRows` and emits problem strings
  like `supervisor_liveness.<id>: tmux_pane_dead`. `reads/supervision.go::attachTmuxLiveness`
  sets a `remediation` field only when `live.Class == TmuxLivenessUnavailable`.
* **Problem.** For the four terminal classes
  (`tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`,
  `tmux_pane_pid_mismatch`), the projection exposes the class name only -
  no `remediation` field, no recovery verb, no `next_action` hint. The
  doctor problem string is similarly bare.
* **Suggested resolution (builder copy, verbatim hint text).**
  - `tmux_session_missing`: "The recorded tmux session is gone. Stop the
    supervisor to mark this lane terminal, then start/reclaim a
    replacement lane: `striatum supervise stop --session-id <id> --reason tmux_session_missing`;
    `striatum supervise start --session-id <id>`. Do not send packets to
    the old supervisor."
  - `tmux_pane_missing`: "The tmux session exists but the recorded pane id
    no longer resolves. Treat the lane identity as broken; stop the
    supervisor and start/reclaim a replacement. Do not infer identity
    from another pane in the session."
  - `tmux_pane_dead`: "The agent process exited and `remain-on-exit`
    retained the pane for private diagnostics. Inspect locally if needed,
    then stop the supervisor and restart/reclaim before sending more
    work."
  - `tmux_pane_pid_mismatch`: "The pane pid / start token no longer
    matches the recorded lane identity, likely external respawn or PID
    reuse. Treat as an authority break: stop the supervisor, do not
    signal stale pids except through token-gated cleanup, and
    start/reclaim a fresh lane."

  Extend `attachTmuxLiveness` to fill `tmux.remediation` for these four
  classes; mirror the hint into `reads/doctor.go`'s problem strings as
  `supervisor_liveness.<id>: <class> [next: <verb>]`.

### Finding 4 (medium): tmux_unavailable warning gradient is invisible in compact projections

* **Context.** The liveness loop in `supervisor/liveness.go` tolerates up
  to 3 consecutive `tmux_unavailable` ticks before marking the supervisor
  lost with reason `tmux_unavailable_persistent`. The build records
  `probe_unavailable_count`, `probe_skipped_at`, `last_ok_at`, and
  `last_unavailable_detail` in pointer metadata via
  `recordTmuxProbeUnavailable`; `tmuxMetadata` in `reads/supervision.go`
  projects all four fields into the `tmux` block.
* **Problem (builder-confirmed).** The data is there, but the
  human-visible liveness vocabulary stays at `alive` / `stalled` / `gone`.
  There is no intermediate `degraded` state between "running fine" and
  "lost (after 3 misses)". A first-time operator sees the lane flip
  abruptly to `tmux_unavailable_persistent` after the third miss with no
  prior warning. The build added a real `delivery_liveness=degraded`
  intermediate for the byte-path side - the tmux probe side is missing
  the symmetric gradient.
* **Suggested resolution (builder copy).** When
  `tmux.probe_unavailable_count > 0` and below the loss threshold,
  project `pane_liveness: degraded` (or `tmux_probe: degraded:N/3`) in
  `supervise.status` and the dashboard, keep `state=attached`, and
  surface `last_ok_at` and `last_unavailable_detail` alongside. This
  mirrors `delivery_liveness` and gives the operator a pre-failure
  warning.

### Finding 5 (low): plain-PTY fallback indistinguishable from tmux-backed in compact view

* **Context.** When tmux is missing and `require_tmux=false`, the build
  records `tmux.state=unavailable` with `unavailable_reason` and a
  remediation hint in pointer metadata; the projection passes both
  through. Workflows with `require_tmux=true` fail closed at launch.
* **Problem (builder-confirmed).** The read model distinguishes the two
  modes - tmux-backed rows have `tmux.state=backed` plus
  `attach_command` / session / pane fields; fallback rows have
  `tmux.state=unavailable` and `unavailable_reason`/`remediation`. But
  the compact `striatum dashboard --once` view shows no
  `transport=tmux` vs `transport=plain-pty` badge, so an operator cannot
  tell at a glance whether a lane is attachable. The builder considers
  the current field "sufficient for the narrow machine / read-model
  acceptance criterion that fallback is explicit in metadata and status"
  but "not sufficient for operator ergonomics in the compact dashboard".
* **Suggested resolution (builder copy).** Add a separate attachability
  field in the compact view, e.g. `transport=tmux attachable`,
  `transport=plain-pty no-tmux`, or `tmux=unavailable:<reason>`. Do not
  overload it into `lane_attestation` or protocol liveness.

## 4. Verdict

* **Verdict:** `accept_with_findings`.
* **Justification:**
  - The Phase 1 substrate landed correctly: tmux session/pane identity is
    captured at launch, `ProbeLaneLiveness` routes through the five
    failure classes, helper-owned attach exit no longer downgrades a live
    pane to `detached`, and `delivery_liveness=degraded` is now a
    first-class field that `supervise.send` honors. The targeted
    `pkg/supervisor`, `pkg/mutations`, `pkg/reads` test suites pass.
  - The live codex builder supervisor is a working proof of the central
    fix: the helper-owned bridge exited at startup, the lane stayed
    `attached`/`attested`, and delivery was correctly refused as
    `degraded:attach_client_exited` instead of presenting a false-healthy
    surface.
  - The five findings above are all developer-ergonomics polish on top of
    the substrate. Each has a concrete, builder-supplied resolution and
    each belongs in the Phase 2 read-surface / runbook work RFC 0089
    already scopes. None of them are correctness or safety regressions in
    Phase 1.
* **Phase 2 follow-up scope (suggested order).**
  1. Finding 2: ship runbook + CHANGELOG recovery sequence and upgrade the
     `supervise.send` error string. (Smallest, highest-leverage doc
     change.)
  2. Finding 3: add the four `remediation` strings to `attachTmuxLiveness`
     and mirror them into doctor problem hints.
  3. Findings 1, 4, 5: compact dashboard `pane=` / `delivery=` /
     `transport=` badge work, plus the `liveness: degraded` intermediate.

## 5. Interrogation Section

* **Interrogation id:** `intg_b1d5d5681deb483c06f6ba590f89bbd2`
* **Target session:** `sess_ac823cc767750f0857cd4a9fa2ced765`
  (implementer-codex_builder-1, Codex GPT-5.5 xhigh)
* **Total rounds:** 3
* **Stop reason:** `interrogation_completed_with_findings`
* **Interrogation chat log:**
  [INTERROGATION_CHAT.md](INTERROGATION_CHAT.md)
* **D028 posture:** no raw tmux pane text, PTY log content, or
  alternate-screen capture was inspected or referenced. Diagnostics in §2
  come from `supervise.status` JSON only.
