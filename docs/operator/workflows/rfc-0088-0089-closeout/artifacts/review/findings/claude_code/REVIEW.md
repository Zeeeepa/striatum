---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "medium"
tags: ["rfc-0089", "phase-1", "ergonomics_dx", "doctor", "rebridge", "dashboard"]
---

# Phase 1 review — RFC 0089 findings (ergonomics_dx)

author: reviewer-claude-opus-4.8-001

- Run: `run_b5dfd0e3c280c19fcc13bf69801eacca`
- Workflow: `rfc-0088-0089-closeout`, Phase 1
- Posture: `ergonomics_dx` (developer-ergonomics; verdict acceptance means the
  affordances are discoverable and consistent from a first-time-user view)
- Reviewer session: `sess_b9102a57f0b23607a1e1114c19db3469`
  (`reviewer-claude_code-1`, Claude Opus 4.8)
- Builder under review: `implementer-codex_builder-2`
  (`sess_ac356d97a74d3359a78e334dfd05902a`), handoff
  `artifacts/build_0089/HANDOFF.md`

## Verdict

**needs_revision.**

Not a structural rejection. The probe model, the `supervise.rebridge` verb, and
the documentation surface are well-built and the verification suite is green.
The block is a tightly-scoped, **read-projection-layer-only** ergonomics gap:
the operator's primary health surface (`striatum doctor`) is silent in the exact
failure state the new `rebridge` verb exists to repair, and no-supervisor session
rows project a self-contradictory, over-confident health signal. Because Phase 2
(the irreversible RFC 0088 `--print`/`exec`/`single_shot`/turn-driver deletions)
is gated on this acceptance, these must land before Phase 1 closes. The live
builder independently reached the same severity read (see
`INTERROGATION_CHAT.md`, round 2: "I would not treat those as acceptable
prerequisites for irreversible RFC 0088 Phase-2 deletion").

## Interrogation

- Interrogation id: `intg_6edfe753c1cb2275cb6396818d3fe9e4`
- Target (live builder): `sess_ac356d97a74d3359a78e334dfd05902a`
- Rounds: **3** (the full ergonomics_dx budget)
- Stop reason: posture fully explored and acceptance criteria for the revision
  confirmed with the builder across all four points (scope, doctor behavior,
  no-supervisor projection, regression tests); interrogation closed by the
  interrogator after round 3. Curated Q/A log in `INTERROGATION_CHAT.md`.

The interrogation opened and ran with a live, attested builder lane (not a
cold-context `--print` author), satisfying the workflow's hard interrogation
requirement.

## Verification run

```bash
cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
```

Result: **PASS**

```
ok  	github.com/halbritt/striatum/go/pkg/supervisor	0.655s
ok  	github.com/halbritt/striatum/go/pkg/mutations	0.074s
ok  	github.com/halbritt/striatum/go/pkg/reads	0.040s
```

The build's own handoff additionally reports `gofmt -l .` clean, `go vet ./...`
clean, and `go test ./...` green; I did not re-run the full module suite, only
the packet-mandated three packages.

## What is solid (would otherwise be accept)

- **Three-state graded liveness** (`healthy → degraded → lost`) with a typed
  `probe_failure` record (`failure_class`, `exit_code`, `errno`,
  `pane_process_alive`, `observed_pane_pid`) — `go/pkg/supervisor/tmux_liveness.go`.
  `pane_process_alive` is a genuinely useful at-a-glance signal for whether
  rebridge is even viable. No pane text leaks into the record (D028 preserved).
- **Transient `tmux_unavailable` is degraded, not lost**, until a configurable
  threshold (`STRIATUM_TMUX_UNAVAILABLE_LOST_THRESHOLD`, default 3) — directly
  resolves RFC 0089 follow-up finding #4. Warning counters
  (`probe_unavailable_count`, `probe_skipped_at`, `last_ok_at`) are exposed.
- **`supervise.rebridge` is fully wired and discoverable**: handler +
  `rpc.MethodRegistry` entry + MCP `tools/list` merge of runtime-registered
  methods (`go/pkg/mcp/capabilities.go`) + CLI runtime route fallback
  (`go/pkg/cli/routes/routes.go`) + spec + glossary + command-authority-matrix.
  The "generated contract is out of write scope" deviation is handled honestly
  and a contract-reconciliation follow-up is recorded.
- **Rebridge refusal messages are actionable**: a dead pane is refused with
  `"...pane liveness is tmux_pane_dead; stop and restart or reclaim the lane"`
  and the refusal does **not** mutate rows (regression:
  `TestSuperviseRebridgeRefusesDeadPane`). It re-attaches in place via the
  extracted `attachTmuxPTY` without killing/respawning the pane.
- **`supervise.status` distinguishes the four signals** as separate keys
  (`lane_backend`, `delivery_state`, `pane_liveness`, `lane_attestation`) and
  carries a concrete `delivery_liveness.remediation` naming the exact
  `striatum supervise rebridge --session-id <id>` command (regression coverage
  in `go/pkg/mutations/supervision_control_test.go`).
- Glossary, spec, and command-authority-matrix updates are thorough and
  consistent with the code (`lane backend`, `rebridge`, `tmux probe failure`
  all defined).

## Blocking findings (must fix before Phase 1 lands)

### F1 — `doctor` is blind to delivery degradation; gives no rebridge guidance

**Severity: medium (blocker for the posture bar).**
`reads/doctor.go HandleDoctor` derives a supervisor item's remediation **only**
from `tmuxLivenessRemediation(class, ...)`, and the `problems` list explicitly
excludes `class == tmux_ok`. For the canonical rebridge scenario — the helper
attach bridge exited but the pane process is still live — the fresh probe
returns `tmux_ok`, so `doctor`:

1. attaches **no** remediation, and
2. does **not** add the supervisor to `problems` (so `doctor.ok` stays `true`).

`striatum doctor` therefore stays silent on a delivery-degraded-but-repairable
lane — the exact state `supervise.rebridge` exists to fix. The rebridge hint
lives only in the fuller `supervise.status` `delivery_liveness.remediation`, not
in `doctor`. This fails the ergonomics_dx posture's explicit bar: "are the
doctor hints specific enough that an operator knows whether to `rebridge` vs
reclaim?" — today doctor only ever points at the reclaim path. Verified in
source; builder confirmed it as an oversight (round 2/3).

**Required fix (read-layer only):** in `HandleDoctor`, inspect
`delivery_liveness.class == "degraded"` independently of the tmux pane class.
When degraded, add a distinct verbose problem record (e.g.
`supervisor_delivery_degraded.<supervisor_id>`) carrying the delivery `reason`
(`attach_client_exited` / `stdin_reader_missing`) and `deliveryRemediation(...)`,
and append a stable `problems` string **even when `pane_liveness == tmux_ok`**,
so `doctor.ok` flips `false` for a repairable delivery-degraded lane and the
operator is pointed at `rebridge` rather than reclaim.

### F2 — No-supervisor session rows project false-confident `plain_pty` / `healthy`

**Severity: medium (blocker for the posture bar).**
In `reads/status.go statusSessions`, `attachSupervisorTmux(row, ...)` runs over
empty metadata **before** the no-supervisor branch and sets
`lane_backend = "plain_pty"` and `delivery_state = "healthy"`
(`laneBackend()` defaults to `plain_pty`; `deliveryState()` defaults to
`healthy`). The subsequent branch then overrides only `lane_attestation =
unattested` / reason `no_attached_supervisor` and `pid = nil` — it leaves the
backend/delivery fields untouched. The dashboard path has the same shape.

Net result for a session with **no live lane at all**: `lane_attestation =
unattested (no_attached_supervisor)` sitting next to `lane_backend = plain_pty`
and `delivery_state = healthy`. That is a self-contradictory at-a-glance signal
— precisely the confusion RFC 0089 follow-up findings #1/#2 set out to remove —
and it implies a confident healthy plain-PTY delivery path where none exists.
Verified in source; builder confirmed.

**Required fix (read-layer only):** when `supervisor_id` is absent, project
`lane_backend = "none"` (or `"unknown"`) and `delivery_state = "unknown"` in
**both** `statusSessions` and the dashboard projection. `plain_pty` should mean
"attached supervisor backed by plain PTY," never "no supervisor."

## Non-blocking findings / follow-ups

### N1 — No human-rendered compact `dashboard --once` frame (do NOT block)

The four signals are distinct **JSON keys**, but `HandleDashboard` returns a DTO
and the Go CLI dispatch emits JSON; there is no compact textual frame with
visual badges, so "at a glance" today means reading JSON. Both reviewer and
builder agree a visual terminal renderer is broader CLI-presentation work,
**out of RFC 0089 Phase-1 code scope**. Record as a UI/CLI follow-up. Phase 1
acceptance should require only that the structured fields are truthful and
non-over-confident (which F1/F2 ensure) and that doctor gives the right action.
(If the operator interprets RFC 0089 "compact dashboard" as mandating a non-JSON
renderer this phase, reclassify N1 as blocking — flagging the interpretation
explicitly rather than deciding it here.)

### N2 — Fold a remediation-coverage table test into the F1/F2 revision

Add a read-layer table test asserting that every terminal tmux class
(`tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`,
`tmux_pane_pid_mismatch`, `tmux_unavailable`) and every delivery degradation
reason (`attach_client_exited`, `stdin_reader_missing`) produces a non-empty
remediation string. Prevents another quiet hint gap. (Builder agreed, round 3.)

### N3 — Contract reconciliation (already recorded by builder)

`contracts/daemon_methods.json` was out of this packet's write scope, so
`supervise.rebridge` is runtime-registered rather than in the generated
contract. A later contract-maintenance packet should add the generated entry and
regenerate route/method tables. Non-blocking for Phase 1.

## Revision acceptance criteria (confirmed with builder, round 3)

A revision flips this to **accept** if, with no schema migration / no
daemon-owner DDL / no change to the rebridge mutation or tmux probe (read +
doctor layer only, tests in `go/pkg/reads`):

1. `doctor` lifts `delivery_liveness.class == degraded` into a distinct verbose
   problem carrying reason + `deliveryRemediation(...)` and adds it to
   `problems` even when pane class is `tmux_ok` (F1).
2. No-supervisor rows project `lane_backend = none|unknown` and
   `delivery_state = unknown` in both status and dashboard (F2).
3. Regression tests: (a) doctor emits the rebridge problem+remediation for a
   `tmux_ok` + delivery-degraded lane and flips `ok` false; (b) a no-supervisor
   row never reports `delivery_state = healthy` / `lane_backend = plain_pty` in
   either projection (F2).
4. Remediation-coverage table test for all terminal tmux classes + delivery
   reasons (N2).

N1 (human-rendered dashboard frame) is explicitly **not** a Phase-1 blocker.
