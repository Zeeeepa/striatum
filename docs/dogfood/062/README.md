# Dogfood 062 — Lane attestation forgery gap (GH #2 / #5 / RFC 0046 V1.7)

**Closes:** [GH #2](https://github.com/halbritt/striatum/issues/2),
[GH #5](https://github.com/halbritt/striatum/issues/5),
[`project_lane_attestation_gap` memory](../../../.claude/projects/-home-halbritt-git-striatum/memory/project_lane_attestation_gap.md).

**Why:** `supervise.start` currently records a session as `lane_attested`
without verifying that the supervised subprocess actually produced the
artifact. The V1.7 fix lives at the publish-artifact layer requiring a
matching `process_executions` row that proves the subprocess executed
and wrote the bytes.

Current state (session memory):

- `striatumd.process_supervisors` tracks attached supervisors.
- `striatumd.process_executions` tracks per-supervisor subprocess runs.
- `session_lane_attestation` (Python) checks process_supervisors only.
- Publish-artifact does NOT gate on process_executions — a forged on-disk
  artifact written by the operator's own shell would pass the existing
  RFC 0046 V1 check.

**Scope:**

- Add a `process_executions` row write to the supervised wrapper path
  (it may already exist; design verifies).
- Gate `publish_artifact_pg` on a matching `process_executions` row
  for the session_id + (optionally) the artifact path. RFC 0046 V1's
  `--allow-no-process-execution --override-rationale` operator path
  stays intact (audit-chained operator override).
- Add tests: forged byline + no matching process_executions row →
  publish refused; supervised path → publish allowed and audit row
  carries the `process_execution_id`.
- Update `session_lane_attestation` to also surface
  `process_execution_count` so dashboards can show "0/N attestations"
  when an agent claimed but didn't execute.

**Shape:** standard 8-job dogfood — 3 designs / synth / review_design /
implement / 3-way build review. Security-class — gemini adversarial
review is load-bearing.

**Branch:** `striatum/dogfood-062-rfc-0046-v17-attestation-gap`.

**Post-landing:** version bump to v1.57.0 (security fix); CHANGELOG;
ROADMAP § "V1.7 lane attestation forgery"; RFC 0046 status →
"V1.7 hardened"; tag.
