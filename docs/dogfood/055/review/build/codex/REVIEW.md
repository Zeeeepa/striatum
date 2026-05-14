---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
severity: "info"
verdict_intent: "accept"
---

# RFC 0050 V1 build threat-model review

author: reviewer-unknown-model-001

## Verdict

Accept. The reviewed design and build handoffs acknowledge the trust
boundaries relevant to this threat model, and the fix-up handoff records
mitigations for the earlier provenance risks.

## Trust boundaries and attack surfaces reviewed

1. **Artifact byline rendering crosses from durable provenance into UI
   display state.** The design says artifact rows must use the literal
   `artifacts.author_line` recorded from disk, not recompute from the
   session's current row when attestation drifts mid-flight
   (`docs/design/UI_REWORK.md:68-73`). It also requires byline integrity
   comparison against `expected_author_line` at publish time
   (`docs/design/UI_REWORK.md:212-216`) and forbids model-author shapes
   for unattested sessions (`docs/design/UI_REWORK.md:38-47`,
   `docs/design/UI_REWORK.md:1173-1179`). The V1 fix-up handoff records
   the implementation mitigation: artifact attestation chips now derive
   from recorded `artifacts.author_line`, and `_shape_artifact_rows` uses
   that recorded path instead of live session state
   (`docs/dogfood/054b/build/HANDOFF.md:32-42`).

2. **Override verdict display crosses from operator action into review
   truthfulness.** The design forbids verdict laundering: an
   `operator_override` must show the operator rationale beside the verdict
   pill and keep the original natural verdict visible
   (`docs/design/UI_REWORK.md:57-60`, `docs/design/UI_REWORK.md:140-149`).
   The page-level requirements keep the rationale inline or in a dedicated
   column, never only in a hover tooltip (`docs/design/UI_REWORK.md:218-222`,
   `docs/design/UI_REWORK.md:510-511`). The older component text contained
   a weaker terminal/dashboard rule, saying dashboard rendering was only by
   count and per-verdict provenance was reserved for JSON consumers
   (`docs/design/UI_REWORK.md:887-892`), but the RFC and fix-up handoff
   close that gap: V1 acceptance requires rationale beside the pill in both
   dashboard and web surfaces (`docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md:125-127`),
   and the fix-up records dashboard rationale support with tests
   (`docs/dogfood/054b/build/HANDOFF.md:43-54`).

3. **Process evidence display crosses from process-adapter diagnostics into
   operator UI.** The design explicitly rejects transcript capture:
   no live terminal output panel, no child stdout/stderr mirror, and no
   default verbose-log stream (`docs/design/UI_REWORK.md:48-52`). The
   allowed evidence surface is a typed diagnostic envelope from
   `blockers.payload_json`, with visible fields limited to command,
   exit code, duration, timeout, missing artifact paths, review-verdict
   missing state, and recovery commands; it repeats that child
   stdout/stderr and model output must never render
   (`docs/design/UI_REWORK.md:1061-1088`). The V1 handoff claims text-mode
   parity for blocker evidence and muted lane evidence while preserving the
   no-green-claim posture (`docs/dogfood/054/build/HANDOFF.md:13-15`,
   `docs/dogfood/054/build/HANDOFF.md:33-34`).

4. **Operator-on-behalf publishing crosses from human recovery into lane
   provenance.** RFC 0050 requires operator-on-behalf publishes during this
   dogfood to use the explicit RFC 0046 path with
   `--allow-no-process-execution --override-rationale`, recording both an
   artifact-row rationale and a `provenance.publish_without_process_execution`
   event (`docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md:129-137`).
   That keeps recovery visible rather than silently upgrading an artifact to
   lane-produced provenance.

## Residual notes

- The design handoff still contains the older terminal-renderer wording that
  would be insufficient on its own for "override rationale prominent in every
  surface" (`docs/design/UI_REWORK.md:887-892`). I am not filing this as an
  open blocker because the RFC and fix-up handoff supersede it for V1 and
  explicitly record the dashboard mitigation and tests.
- I did not inspect repository implementation files in this review. The
  review policy was document-only, so this verdict is limited to the four
  referenced documents.
