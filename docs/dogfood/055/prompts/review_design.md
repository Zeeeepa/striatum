# Design Review — RFC 0050 V1.5 synthesis

Posture: ergonomics_dx.

Verify the synthesis tracks RFC 0050 §V1.5 + UI_REWORK.md §4 + §8.
Reject if:

- V1 primitives are being redefined.
- V2 scope (islands, override-modal logic, copy-on-click,
  workflow-graph-editor) has bled in.
- V1.5 essentials missing: run_detail recovery panel,
  job_detail expected-artifacts + process-evidence,
  artifact_view provenance trail, run_posture_verdicts
  provenance + attestation columns, doctor recipes,
  view_file breadcrumb.
- New partials don't follow the working v1.41 byline pattern.

Write `docs/dogfood/055/review/design/REVIEW.md` with v1 front
matter (NO `author:` in front matter — byline on title-block
line). Verdict: accept / accept_with_findings / needs_revision.
