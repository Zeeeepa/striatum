# Phase 1 build — RFC 0089 follow-up findings

Read:

- `docs/operator/workflows/rfc-0088-0089-closeout/TASK.md`
- `docs/rfcs/0089-tmux-backed-lane-monitoring.md`
- `docs/operator/workflows/rfc-0089-tmux-final-build-review/artifacts/build/HANDOFF.md` (what Phase 1 of RFC 0089 already landed)

Then inspect the current working tree (`go/pkg/supervisor`, `go/pkg/mutations`,
`go/pkg/reads`, the doctor/status/dashboard projections, and the agent-loop /
tmux helper paths).

## Goal

Resolve the five accepted RFC 0089 follow-up findings as one coherent design.
The intended shape — refine if the code tells you otherwise, but justify any
deviation in the handoff:

1. **Typed probe-failure record.** A single authoritative struct populated on a
   tmux probe miss, capturing at least: exit code, errno string, whether the
   pane process is still alive (`tmux list-panes -F '#{pane_pid}'` + kill-0),
   and a typed `failure_class`. Every downstream consumer (state machine,
   dashboard, doctor, rebridge) branches on this one record. Land this as a
   pure data-model change first.
2. **Graded liveness state machine (finding 4).** healthy -> degraded -> lost.
   A probe miss moves a lane to `degraded` (warning), not straight to `lost`;
   only persistent misses (configurable threshold, default 3) escalate to
   `lost`. Pane-process death is distinct from delivery degradation.
3. **`rebridge` verb (finding 3).** Re-attaches the tmux delivery/attach path in
   place without sending SIGTERM or resetting the pane, restoring delivery
   without losing session context. It is valid ONLY while pane-process liveness
   still holds; if the pane is dead it must refuse and point at reclaim /
   stop-start. Regenerate any per-lane MCP socket/FIFO the dead attach-client
   cleaned up.
4. **Distinct dashboard signals (findings 1, 2).** `dashboard --once` and the
   compact dashboard must render delivery-state (`healthy|degraded|lost`)
   distinctly from pane liveness AND from lane attestation, plus a lane-backend
   badge distinguishing tmux-backed lanes from plain-PTY fallback lanes.
5. **Failure-class-derived remediation hints (finding 5).** `doctor` and
   `status` derive hint text from the captured `failure_class` (e.g. socket
   ENOENT -> "tmux server not running"; attach-client exit -> "delivery client
   crashed, run: striatum rebridge <lane>"), not a generic menu.

## Guardrails

- Stay strictly inside the write scope in your work packet.
- Do not let raw tmux/PTY bytes become daemon state or a durable artifact; the
  probe record stores typed metadata (exit code, errno, failure class), not
  screen text.
- Add or update tests for every behavior change. In particular, add a
  regression test that asserts a warn-level degraded state is emitted before a
  lane is marked lost on persistent `tmux_unavailable` (finding 4), and a test
  that `rebridge` refuses when pane-process liveness is gone.
- If the daemon needs to be rebuilt for these changes to take effect in a live
  lane, say so explicitly in the handoff.

## Verify

```bash
cd go && gofmt -l . && go vet ./... && go test ./...
```

## Publish

Write `docs/operator/workflows/rfc-0088-0089-closeout/artifacts/build_0089/HANDOFF.md` with:

- files changed and the final behavior landed for each of the five findings;
- the exact verification commands and their results;
- any deviation from the intended design and why;
- remaining non-blocking findings, if any;
- the live session id reviewers should interrogate if it is not obvious from
  `list.sessions`.

Stay live for the three-lane interrogation panel after publishing and
completing.
