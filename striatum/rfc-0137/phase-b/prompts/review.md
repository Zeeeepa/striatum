# Review — RFC 0137 Phase B (failure-mode taxonomy)

Independent reviewer (different model family from the author). Review Phase B of
the `striatumd` Prometheus exporter against the RFC. Required context:
`docs/rfcs/0137-striatumd-prometheus-exporter.md`. Author summary:
`striatum/rfc-0137/phase-b/artifacts/DRAFT.md`.

**Verify against source, not prose.** Read `go/pkg/metrics/taxonomy.go`, the new
families in the snapshot/render code, the tests, and every edit under
`go/pkg/mutations/`. Keep making tool calls (read files, run
`go test`/`go build`) — do not go silent for long stretches.

## Acceptance checklist

1. **Enums closed + pinned.** `Origin`, apoptosis `reason`, necrosis `reason`,
   and the lease/liveness label sets are closed Go enums. A **union guardrail
   test** asserts the necrosis domain equals exactly
   `{agent_pid_dead, agent_exited_unsealed, recovery_exhausted}` and is anchored
   to the real source constants (`recovery_decision_tree.go:152/158`,
   `recovery_escalation.go:15`). A new stall class cannot silently widen it.
2. **F-A6 enforced.** `liveness_deadline_missed` is EXCLUDED from
   `necrosis_total` and routed to `striatum_liveness_deadline_events_total`.
   Confirm `TestLivenessMissCanRecoverWithoutNecrosis` actually drives
   `active → deadline_missed → recovered` and asserts necrosis did NOT move while
   the liveness-events counter did. This must be a real behavioral test, not a
   tautology.
3. **All six families present** with the RFC's types and **enum-only labels** —
   no raw run/job/session ids as label values (re-check the updated redaction
   golden + forbidden-content regex still pass).
4. **Tag-at-site is correct.** apoptosis is emitted by the intent-declaring
   terminator; necrosis only by the recovery/liveness paths that detect an
   unannounced exit. The counters are **tx-safe** (no over-count on rollback) and
   restart-consistent — verify the chosen mechanism actually delivers this
   (durable-event fold, or strictly-post-commit increments).
5. **Surgical mutations edits.** `go/pkg/mutations/` changes are minimal and at
   genuine emit sites only — no recovery-logic refactors, no `metrics` import in
   `mutations` that creates a cycle.
6. **No scope creep into C/D.** No `Classification`/`Register()` refusal, series
   budget, allowlist hash, `doctor_problems`, capability-scoping, consent, or
   alert rules.
7. **Green.** `make -C go build`, `go test ./pkg/metrics/...`,
   `go test ./pkg/mutations/...`, `go build ./...` all pass; change stays in
   declared write scope.

## Verdict

Record a finding with a single verdict:
- **accept** — all hold; correct, tested, in-scope, and the recovery/lifecycle
  edits are safe.
- **needs_revision** — cite each concrete defect (file/line, criterion, fix).
  Be specific so one cycle clears it. Reserve for real correctness/contract gaps
  (especially any necrosis/apoptosis mis-tag or tx-safety hole), not style.
