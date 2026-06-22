---
schema_version: striatum.synthesis.v1
artifact_kind: synthesis
author: adjudicator-reviewer-002
title: "RFC 0162 final collaboration summary"
run_id: "run_623ba123a529b1c867186c759ac02015"
status: accept_with_findings
inputs:
  - "docs/operator/workflows/rfc-0162-design/SEED.md"
  - "docs/rfcs/0162-lane-auth-silent-failure-observability.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/falsifier_1/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0162-design/dialogue/falsifier_2/FALSIFIER.md"
  - "docs/operator/artifacts/rfc-0162-design/DECISION_override_commit_fold_f1_f2.md"
  - "docs/operator/artifacts/rfc-0162-design/commit/proposal/PROPOSAL.md"
---

# RFC 0162 Final Collaboration Summary

author: adjudicator-reviewer-002

## Verdict

verdict: accept_with_findings

The RFC 0162 design run is cleared for downstream publication by the run-level operator decision `DECISION-rfc-0162-design-override-fold-f1-f2`, not by treating the ordinary collaboration gate as an unconditional accept. The collaboration trajectory found two material over-claim classes, and publication is valid because the committed proposal folds both as binding criteria and carries the residual obligations into the build and verify runs.

## Gate Record

| stage | disposition | effect |
| --- | --- | --- |
| Cycle 1 adjudication | `needs_revision` | F1 and F2 landed unrebutted: the scalar census could not cover non-expiring non-codex lanes with lane attribution, and Layer 1 did not prove it sampled the same non-codex credential the lane presents at runtime. |
| Cycle 2 re-attack | `needs_revision` recorded by the committed proposal and operator decision | The same two over-claim classes remained actionable, but the findings had concrete closest-acceptable repairs rather than showing a fundamental design failure. |
| Operator decision | `accepted_with_follow_up` | The human override authorized publication only if F1 and F2 were folded as binding criteria; the stated reason was a workflow-template limitation plus honest, repairable over-claims. |
| Final summary | `accept_with_findings` | The publication condition is satisfied by `PROPOSAL.md`; the remaining obligations are downstream implementation and game-day verification, not another design revision. |

## Publication Summary

`docs/operator/artifacts/rfc-0162-design/commit/proposal/PROPOSAL.md` is the downstream publication artifact. It supersedes the original RFC sketch where they disagree and re-anchors the implementation spec to current source behavior: the preflight success path is codex-only, `laneproviderauth.Check()` is pure and has no DB handle, the RFC 0137 exporter folds metrics from daemon-owned PostgreSQL on refresh, and `proximal` owns the alert-rule publication path.

The final proposal resolves all four open questions with concrete build choices: Layer 1 plus the roster/sample census and codex-scoped Layer 3 are the MVP; Layer 2 is a follow-up external prober; cardinality is bounded by closed labels and a per-family budget of 32; and staleness thresholds are operator-declared in the auth roster rather than derived from observed decay.

## Folded Findings

F1 is folded by replacing the scalar `striatum_lane_auth_expected_count` census with a per-lane expected vector `striatum_lane_auth_expected{lane,provider,kind}`, an observed vector `striatum_lane_cred_sample_present{lane,kind}`, and label-preserving `unless on(lane)` alert semantics. The proposal also narrows the MVP honestly: non-codex `api_key` lanes have absence coverage through `sample_present`, but positive validity remains an accepted/deferred Layer 2 risk.

F2 is folded by removing the non-codex claim that the codex `ResolveAuthHome` shape proves runtime credential resolution. The committed spec requires a provider-aware credential resolver tied to adapter identity and launch-env precedence, and it fails closed into `striatum_lane_cred_resolver_mismatch{lane,kind}` when the runtime credential source cannot be proven. No green expiry gauge is allowed from an unproven fallback path.

## Downstream Contract

`rfc-0162-build` should execute the spec contract-first: add the roster and threshold fold, implement the resolver and fail-closed mismatch event, sample resolver-proven credential expiry, emit codex-only auth-success heartbeats downstream of real `Passed()` results, and add the named tests for FA-1 through FA-7 plus FA-F1 and FA-F2.

`rfc-0162-verify` must prove the alerts in a game-day run: silent claude OAuth expiry, API-key sample absence, resolver decoy and resolver-mismatch cases, renewal-stalled detection, daemon/exporter-down shared fate, and codex heartbeat staleness. The `proximal` alert rules remain a separate publication to `observability/prometheus/rules/striatum-alerting.rules.yml` and must preserve lane attribution.

## Closing Note

The collaboration gate did its job: it stopped an over-broad provider-agnostic claim, preserved the credited parts of the design, and produced a narrower falsifiable implementation contract. The run can proceed with findings because the accepted findings are now explicit acceptance criteria rather than hidden assumptions.
