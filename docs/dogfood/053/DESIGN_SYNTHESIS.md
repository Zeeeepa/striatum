---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

author: designer-unknown-model-002

# DESIGN SYNTHESIS — RFC 0046 V1 lane evidence guard

Reconciles `design/{codex,claude_code,gemini}/DESIGN.md` against the
RFC 0046 spec.

## Decisions

- **F-schema:** add `attestation_override_rationale TEXT` column to
  `artifacts`. Bump `LATEST_VERSION` in
  `src/striatum/migrations.py`. New `.sql` for Postgres under
  `src/striatum/daemon_pg/sql/`.
- **F-guard:** in `src/striatum/artifacts.py::publish_artifact`,
  after byline validation, canonicalise the stored byline. If it
  matches a model byline (`<role>-<model>-<ord>`), look up matching
  `process_executions` rows. If none cover the artifact path, raise
  `ArtifactError("lane_evidence_missing: ...")` with the named
  remediation pointing at `--allow-no-process-execution`.
- **F-override:** add `--allow-no-process-execution` +
  `--override-rationale "..."` flags to `publish-artifact`. Override
  with empty rationale refuses (exit 2). Override + rationale stores
  rationale on the artifact row + emits event.
- **F-event:** register
  `provenance.publish_without_process_execution` in events module.
- **F-test:** `tests/test_lane_evidence_guard.py` covers four
  scenarios per RFC 0046 acceptance.

## Implementation order

1. Schema migration (`migrations.py` + daemon_pg SQL).
2. `publish_artifact` guard logic (model-byline detection + evidence
   lookup + refusal).
3. CLI flags on parser + dispatch wire-through.
4. Event registration.
5. Regression tests.

## Acceptance

- Model-byline publish with matching process_executions row → succeeds.
- Model-byline publish without row → refuses with exit 6.
- Override empty rationale → refuses with exit 2.
- Override + rationale → succeeds, event recorded, rationale stored.
- Operator-byline publish → passes through unchanged regardless.
