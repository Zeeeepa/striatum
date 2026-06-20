# Phase B — failure-mode taxonomy (apoptosis/necrosis + lifecycle emit)

You are implementing **Phase B of RFC 0137** (`striatumd` Prometheus exporter).
Required context doc: `docs/rfcs/0137-striatumd-prometheus-exporter.md`. **Phase A
already landed** — `go/pkg/metrics/` has `MetricsSnapshot`, the
`atomic.Pointer` read path, `render.go`, the `/metrics` handler, the
sweep-tick fold, and the redaction/scrape tests. Build **on top of it**.

**Implement Phase B only.** Do NOT build Phase C (the `Classification` /
`Register()` refusal, the per-family series budget / `cardinality_clipped_total`,
the `metrics_allowlist.json` boot hash check, the `doctor_problems` collector) or
Phase D (capability-scoping, consent, alert rules). A reviewer rejects scope
creep.

## Deliverables (RFC §Design-Sketch 3 + §Roadmap Phase B)

1. **`go/pkg/metrics/taxonomy.go`** — closed, CREATE-new Go enums (they do NOT
   exist as source constants today; define them here) **anchored to** the real
   constants that do exist, and **pinned by a union guardrail test**:
   - `Origin`: `daemon-core | reconcile-sweep | supervisor | lane`.
   - **apoptosis `reason`** (healthy programmed self-termination):
     `run_completed`, `job_succeeded`, `lease_handoff`, `supervisor_drained`,
     `session_closed_clean`.
   - **necrosis `reason`** = the **confirmed-dead** set ONLY:
     `agent_pid_dead`, `agent_exited_unsealed`
     (`go/pkg/mutations/recovery_decision_tree.go` — `stallClassAgentPIDDead`
     line 152, `stallClassAgentExitedUnsealed` line 158), plus
     `recovery_exhausted` (`go/pkg/mutations/recovery_escalation.go` —
     `recoveryExhaustedBlockerKind` line 15). The guardrail test must assert the
     necrosis domain equals **exactly** that set, so a new stall class cannot
     silently enter it.
2. **Families** (add to the snapshot + render, low-cardinality enum labels only —
   never raw ids):
   - `striatum_apoptosis_total` (counter; labels `origin`, `reason`)
   - `striatum_necrosis_total` (counter; labels `origin`, `reason`)
   - `striatum_lease_transitions_total` (counter; labels `from`, `to`, `reason`)
   - `striatum_run_wedge_age_seconds` (histogram; label `origin`)
   - `striatum_liveness_deadline_margin_seconds` (histogram; label `origin`)
   - `striatum_liveness_deadline_events_total` (counter; label `reason` =
     `deadline_missed` | `recovered`)
3. **F-A6 — the load-bearing correctness constraint.**
   `liveness_deadline_missed` is a **reversible** pre-death observation
   (`session.liveness_deadline_missed` / `session.liveness_recovered` are emitted
   at `go/pkg/mutations/recovery.go:1229` / `:1244`; the recover path proves it is
   not death). It must be **EXCLUDED** from the necrosis domain and routed to
   `striatum_liveness_deadline_events_total` instead. A recoverable stall moves
   the liveness-events counter, **never** `necrosis_total`.
4. **Tag apoptosis vs necrosis at the lifecycle-termination code sites.** The two
   share the same terminal DB transition, so the distinction must be tagged where
   the lifecycle ends: the terminator that declares intent emits `apoptosis`;
   only the recovery/liveness paths that detect an *unannounced* exit emit
   `necrosis`. Tag at the site (e.g. alongside the existing durable lifecycle
   event), and count for the metric.
5. **Ship `TestLivenessMissCanRecoverWithoutNecrosis`** — drives
   `active → liveness_deadline_missed → liveness_recovered` and asserts
   `striatum_liveness_deadline_events_total` moved, `striatum_necrosis_total` did
   **not** increment, and any lifecycle-balance/conservation value stayed
   consistent (no necrosis on a recoverable stall).

## Design guidance (make a defensible choice, justify it in DRAFT.md)

The counters must be **transaction-safe and restart-consistent**: a rolled-back
lifecycle transaction must not over-count, and the numbers should survive a
daemon restart rather than resetting to zero silently. Two viable mechanisms —
pick one and justify it:
- **Fold from durable events at the sweep tick** (preferred for robustness):
  count the already-durable lifecycle/liveness events the tick scans, so the
  counters are derived read-model values consistent with Phase A's snapshot
  model and are tx-safe + restart-consistent by construction. The "tag at the
  site" requirement is then satisfied by tagging the durable event's reason.
- **In-memory atomic counters incremented post-commit** at the lifecycle site —
  only if you guarantee increments happen strictly after the tx commits and you
  document the restart-reset behavior (Prometheus tolerates counter resets).

Avoid import cycles: `go/pkg/metrics` must not import `go/pkg/mutations`. If the
lifecycle sites need to signal the metrics layer, expose a small
increment/observe API from `metrics` that `mutations` calls, or (preferred) keep
the derivation inside the fold.

## Constraints

- Write scope: `go/pkg/metrics/`, `go/pkg/mutations/`, `go/cmd/striatumd/`, and
  your artifact dir. `go/pkg/mutations/` holds delicate recovery/lifecycle code
  whose bugs have caused real wedges — make **minimal, surgical** edits only at
  the genuine emit/tag sites; do not refactor recovery logic.
- Keep everything green: `make -C go build`, `go test ./pkg/metrics/...`,
  `go test ./pkg/mutations/...`, and `go build ./...`. The Phase A redaction
  golden will change because new families appear — UPDATE the golden
  deliberately and confirm the forbidden-content regex still passes (no raw ids
  in the new labels).
- Hand-render Prometheus text (no client library). Enum labels only; no raw
  run/job/session ids as label values.

## Deliverable artifact: DRAFT.md

Write `striatum/rfc-0137/phase-b/artifacts/DRAFT.md`: files touched, the
mechanism you chose for tx-safe counters and why, the enum→source-constant
anchoring table, how F-A6 is enforced, the acceptance-criteria → test mapping
(incl. `TestLivenessMissCanRecoverWithoutNecrosis` and the necrosis-domain
guardrail test), the exact verification commands + pass results, and explicit
confirmation that Phase C/D were not implemented.

Do the work, prove it green, publish DRAFT.md, complete the job.
