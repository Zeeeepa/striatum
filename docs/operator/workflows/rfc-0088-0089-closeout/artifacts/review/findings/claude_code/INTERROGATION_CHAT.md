# Interrogation chat — RFC 0089 Phase 1 (ergonomics_dx)

author: reviewer-claude-opus-4.8-001

Curated `interrogation.show` Q/A log. Workflow messages only — no raw terminal,
tmux pane, or PTY output is reproduced here (none was present; interrogation
turns are curated daemon messages, not screen capture).

- Interrogation id: `intg_6edfe753c1cb2275cb6396818d3fe9e4`
- Interrogator: `sess_b9102a57f0b23607a1e1114c19db3469` (reviewer-claude_code-1, Claude Opus 4.8)
- Target (live builder): `sess_ac356d97a74d3359a78e334dfd05902a` (implementer-codex_builder-2)
- Topic: dashboard distinguishability of lane_backend/delivery_state/pane_liveness/attestation; doctor rebridge-vs-reclaim hint specificity; rebridge failure-message actionability
- Opened: 2026-05-29T05:06:03Z · Closed by interrogator after round 3 · Rounds: 3

---

## Round 1 — dashboard distinguishability & over-confident fallbacks

**Q (reviewer):** Findings #1/#2 ask the *dashboard* to make delivery-state,
pane-liveness, attestation, and tmux-vs-PTY backend distinguishable "at a
glance." The build adds `lane_backend`, `delivery_state`, `pane_liveness` as
separate projection keys, but `HandleDashboard` returns structured data and I
find no compact human-rendered frame — `dashboard --once` appears to emit JSON.
(a) Is there a human-rendered terminal frame surfacing the four signals
distinctly, or must the operator read raw JSON? (b) `deliveryState()` defaults to
`healthy` when `delivery_liveness` is absent and `laneBackend()` defaults to
`plain_pty` with no tmux metadata — including for a row with no attached
supervisor. Doesn't that risk a confident "healthy / plain_pty" for a lane whose
delivery is unknown or which has no live lane? Walk me through what an operator
sees for (i) healthy tmux lane, (ii) delivery-degraded tmux lane, (iii) session
with no attached supervisor.

**A (builder):** Concern valid. The build adds distinct structured fields to the
status/dashboard DTO, but the Go CLI dashboard path still prints JSON through
the generic dispatch writer; no compact human-rendered frame with visual badges
was added. So for `dashboard --once` the operator currently reads JSON fields —
weaker than "at a glance" if read as a human terminal frame.
- (i) Healthy tmux lane: `lane_backend=tmux`, `pane_liveness=tmux_ok`,
  `delivery_state=healthy`, `lane_attestation=attested`; healthy delivery is the
  absence of a degraded record, so `deliveryState()` falls back to `healthy`.
- (ii) Delivery-degraded tmux lane: `lane_backend=tmux`, `pane_liveness=tmux_ok`,
  `delivery_state=degraded`, `delivery_liveness.reason=attach_client_exited`,
  `lane_attestation=attested` if pane pid/start still verifies — the intended
  split (pane/byline true while delivery is not healthy).
- (iii) No attached supervisor: "this is where the current fallback is
  misleading." No-supervisor rows are later marked `lane_attestation=unattested`
  (`no_attached_supervisor`), but `attachSupervisorTmux` runs over empty metadata
  first and sets `lane_backend=plain_pty` and `delivery_state=healthy`. The safer
  projection is `lane_backend=none|unknown` and `delivery_state=unknown` when
  `supervisor_id` is absent; only default to `plain_pty`/`healthy` for an attached
  non-tmux supervisor. Also confirmed the live installed CLI emits JSON, not a
  textual frame, and the running build may show only older fields if not rebuilt
  from the working tree.

## Round 2 — doctor rebridge-vs-reclaim specificity & severity

**Q (reviewer):** On `doctor` specifically. In `HandleDoctor`, each supervisor
item's remediation comes only from `tmuxLivenessRemediation(class,...)`, and
`problems` excludes `class==tmux_ok`. For the canonical rebridge case — attach
bridge exited, pane still live — the fresh probe returns `tmux_ok`, so doctor
attaches no remediation and adds no `problems` entry. `doctor --verbose` stays
silent on a delivery-degraded lane — the exact state `rebridge` repairs. The
hint only appears in `supervise.status delivery_liveness.remediation`, not
doctor. (1) Is doctor intentionally blind to delivery degradation, or an
oversight? (2) Given the posture pairs "rebridge vs reclaim" as the doctor
decision, is the missing rebridge hint — plus the no-supervisor `plain_pty/healthy`
fallback — a Phase-1 blocker or a non-blocking follow-up? Phase 2 deletions are
irreversible and gated on my acceptance; I need your honest severity read.

**A (builder):** "Confirmed: doctor is currently blind to the canonical
delivery-degraded rebridge case. That was an oversight, not an intentional
boundary." Doctor derives remediation only from tmux pane-liveness class; when
the probe is `tmux_ok` but metadata has `delivery_liveness.reason=attach_client_exited`,
doctor adds neither remediation nor a `problems` entry (problem condition
excludes `tmux_ok`). The rebridge hint exists in `supervise.status` but doctor
does not lift it into verbose problem records. Honest severity: "this is a
Phase-1 blocker for the doctor/remediation part of RFC 0089 if the review bar is
'doctor tells the operator rebridge vs reclaim.' ... I would not treat those as
acceptable prerequisites for irreversible RFC 0088 Phase-2 deletion." Proposed
minimal fix: doctor inspects delivery liveness separately; for
`delivery_liveness.class==degraded`, add a verbose problem
(`supervisor_delivery_degraded.<id>`) with reason + `deliveryRemediation(...)`
and a stable problem string even when `tmux_liveness==tmux_ok`; and no-supervisor
projections should use `lane_backend=none|unknown` / `delivery_state=unknown`.

## Round 3 — pin the revision acceptance criteria

**Q (reviewer):** I'll record needs_revision. To land the revision in one pass,
confirm: (1) SCOPE — fix contained to read/projection + doctor layer, no schema
migration / no daemon-owner DDL / no rebridge-mutation or tmux-probe change?
(2) DOCTOR — lift `delivery_liveness.class==degraded` into a distinct verbose
problem with reason + `deliveryRemediation(...)`, added to `problems` even when
pane class is `tmux_ok` so `ok` flips false. (3) NO-SUPERVISOR — `lane_backend=
none|unknown` and `delivery_state=unknown` when `supervisor_id` absent, in both
status and dashboard. (4) TESTS — regression for doctor emitting the rebridge
problem/remediation on tmux_ok+degraded, and for no-supervisor rows never
reporting `delivery_state=healthy`/`lane_backend=plain_pty`. Anything else in the
ergonomics bucket to fold in, or is the human-rendered dashboard frame genuinely
out of Phase-1 code scope (so I should NOT block on it)?

**A (builder):** Confirmed on all four.
1. Scope: yes — contained to read/projection and doctor code plus focused tests;
   no migration, daemon-owner DDL, rebridge-mutation, or tmux-probe changes.
   Likely points: `reads/doctor.go`, `reads/supervision.go`
   (`statusSessions`, dashboard projection flow, `attachSupervisorTmux`), tests
   in `go/pkg/reads`.
2. Doctor: yes — lift degraded delivery into its own verbose problem with reason
   + `deliveryRemediation(...)`, append a stable `problems` string even when pane
   is `tmux_ok`, so `doctor --verbose` is non-silent for a repairable lane.
3. No-supervisor: yes — `lane_backend=none|unknown` and `delivery_state=unknown`
   when `supervisor_id` absent, in both status and dashboard. `plain_pty` should
   mean an attached PTY-backed supervisor, not the absence of one.
4. Tests: yes — doctor rebridge problem/remediation on tmux_ok+degraded, and
   no-supervisor rows never projecting `delivery_state=healthy`/`lane_backend=
   plain_pty` in both projections.
Plus one small **table test** to fold in: all terminal tmux classes
(`tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`,
`tmux_pane_pid_mismatch`, `tmux_unavailable`) and delivery reasons
(`attach_client_exited`, `stdin_reader_missing`) should produce non-empty
remediation — read-layer only, prevents another quiet hint gap. On the dashboard
frame: "I would not block this RFC 0089 Phase-1 revision on adding a full
human-rendered terminal dashboard frame ... For Phase 1, I would accept
structured dashboard/status fields if they are truthful, non-overconfident, and
doctor gives the right rebridge-vs-reclaim action." Record the rendered frame as
a UI/CLI follow-up unless the operator explicitly reads "compact dashboard" as
requiring a non-JSON renderer this phase.

---

## Reviewer summary

The builder was candid and corroborated both source-verified gaps and his own
Phase-1-blocker severity read. Outcome: **needs_revision** with a tightly-scoped,
read-layer-only revision (F1 doctor delivery-degraded remediation, F2
no-supervisor projection honesty, + regression/table tests). The human-rendered
dashboard frame (N1) is a non-blocking UI/CLI follow-up by mutual agreement.
