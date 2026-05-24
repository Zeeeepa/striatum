---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/workflow.py", "src/striatum/artifact_contracts.py", "go/pkg/workflowauthoring/", "go/pkg/workflowgenerate/", "go/pkg/mutations/artifact.go"]
---

# Workflow Authoring Cutover Handoff
author: operator [self-declared: workflow-porter-codex-gpt-5-001]

## Finding

Go workflow authoring exists, but remains a subset of Python behavior. Python
still has broader validation for cross-repo config, phases, provenance,
sealed patch, recovery policy, harness profiles, process lanes, constraints,
augmentation, reviewer policy, review posture, apply gates, repo-write
parallelism, and revision policy.

## First Slice Landed

This workflow closes a concrete Go artifact-contract gap:

- added Go allowed kinds for `operator_brief`, `work_plan`, `progress_note`,
  `operator_report`, `commit_request`, `pr_request`, and
  `auto_finalize_gate_evidence`;
- added Go front-matter schemas for those kinds;
- tightened escalation front matter to require blocker payload fields;
- added duplicate front-matter field rejection;
- added Go tests for the new kinds, escalation payload validation,
  auto-finalize gate evidence, and duplicate-field refusal.

## Remaining Sequence

1. Move artifact contracts into a dedicated Go package shared by publish, Git
   mutations, and workflow validation.
2. Expand `go/pkg/workflowauthoring.Validate` to full Python parity.
3. Align Go lint with Python semantics, including accepted-risk fingerprints.
4. Make `go/pkg/workflowgenerate` call workflowauthoring validation/lint.
5. Move template catalog ownership fully to Go and port or retire
   `templates render-md`.
6. Convert Python workflow/generator/artifact tests to Go.
