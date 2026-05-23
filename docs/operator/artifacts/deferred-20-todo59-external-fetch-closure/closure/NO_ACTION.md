---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/ROADMAP.md", "docs/DECISION_LOG.md", "docs/rfcs/0057-corpus-contract-v2.md", "docs/rfcs/0066-replay-archive-corpus-v2-foundations.md", "src/striatum/workflow.py", "src/striatum/daemon_pg/handlers/context.py", "go/pkg/mutations/claim.go", "go/pkg/mutations/run.go", "tests/test_corpus_verify.py", "tests/test_workflow_field_errors.py", "go/pkg/mutations/claim_test.go"]
---

# TODO 59 External Fetch UX No-Action Closure
author: deferred20-todo59-codex-gpt-5-001
status: closed
date: 2026-05-23

## Result

No Striatum core source or test change is required for deferred item 20.
TODO 59's optional external-consumer fetch and UI UX should remain out of core
until a later optional-augmentation decision accepts a richer surface.

The current core behavior already closes the local Striatum side:
workflow-authored `augmentation.mode: "reference_only"` can name local
`corpus_bundle` sources; claimed work packets expose those sources under
`context.augmentation_references`; missing, unreadable, or malformed bundles
are reported as optional metadata and do not block claims or state
transitions; and packet references advertise `fetch_mode:
"agent_side_local_bundle"` rather than fetching through the runner.

## Evidence

- `docs/TODO.md` item 59 says core Corpus V2, archive, verification, and
  reference-only packet augmentation are done, and that external-consumer
  fetch UX remains out of core unless a later decision accepts it.
- `docs/ROADMAP.md` section 5.7 says the core packet-reference surface has
  landed and richer external-consumer fetch UX remains optional. The suggested
  implementer note says no core Striatum implementation lane is currently
  needed for Corpus V2 foundations.
- D126 accepts workflow opt-in augmentation by reference with agent-side
  fetch. Its consequence is explicit: Striatum still runs when augmentation
  sources are missing, slow, unreachable, or unconfigured.
- RFC 0057's context-injection policy chooses explicit workflow-level opt-in
  as the proposed V2 direction and treats runner-side fetch as boundary-risky
  unless fail-open and bounded.
- RFC 0066 says remaining work is bounded to any future
  augmentation-reference fetch surface, which must remain optional and local.
- `src/striatum/workflow.py` validates only `augmentation.mode:
  "reference_only"` and only local repo-relative `corpus_bundle` sources;
  `required: true`, external URLs, `.striatum/`, duplicate sources, and
  unknown jobs are rejected.
- `src/striatum/daemon_pg/handlers/context.py` builds packet
  `augmentation_references` with `required: False`, `content_mode:
  "references"`, `fetch_mode: "agent_side_local_bundle"`, manifest summaries
  only, and missing/unavailable statuses instead of failures.
- `go/pkg/mutations/claim.go` has the same packet reference behavior for the
  Go daemon claim path, while `go/pkg/mutations/run.go` validates the same
  local-only augmentation shape before run preparation.
- `tests/test_corpus_verify.py` pins the V2 augmentation boundary across
  corpus, CLI, daemon RPC, daemon PG, workflow, and Go run/claim code: no
  Engram imports and no `memory.*` Striatum capabilities.
- `tests/test_workflow_field_errors.py` covers valid reference-only
  augmentation and rejects remote URL or `.striatum/` corpus bundle paths.
- `go/pkg/mutations/claim_test.go` covers available and missing local bundle
  references, manifest summary redaction of absolute repo roots, and omission
  for jobs that did not opt in.

## Classification

Close deferred item 20 as no-action for Striatum core.

Adding a fetch button, external consumer discovery, an Engram availability
check, a runner-side retrieval command, or an operator UI over external
consumer fetch would create a new product surface. It would need a separate
accepted optional-augmentation decision for capability semantics, failure
mode, packet budget, privacy, UI behavior, and boundary regression coverage.

The current packet metadata is intentionally modest and sufficient: it tells
an opted-in agent where the local bundle is, whether the manifest is
available, which summary fields are safe to inspect, and that the agent or an
external sidecar owns any fetch. Striatum does not import or call the external
consumer.

## Changed Files

- `docs/operator/plans/deferred-20-todo59-external-fetch-closure.md`
- `docs/operator/workflows/deferred-20-todo59-external-fetch-closure/workflow.json`
- `docs/operator/workflows/deferred-20-todo59-external-fetch-closure/prompts/close_external_fetch_ux.md`
- `docs/operator/artifacts/deferred-20-todo59-external-fetch-closure/closure/NO_ACTION.md`

No shared TODO, roadmap, operator brief, decision-log, RFC, source, Go, or
test files were edited.

## Validation Evidence

Commands run for this closure:

- `PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/deferred-20-todo59-external-fetch-closure/workflow.json --json`
  - Result: `{"data":{"valid":true,"workflow_id":"deferred-20-todo59-external-fetch-closure"},"ok":true}`.
- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_corpus_verify.py::test_corpus_v2_surface_keeps_augmentation_boundary_local tests/test_workflow_field_errors.py::test_reference_only_augmentation_validates tests/test_workflow_field_errors.py::test_augmentation_source_must_be_local_corpus_bundle`
  - Result: `3 passed in 0.04s`.
- `(cd go && go test ./pkg/mutations -run 'TestAugmentation')`
  - Result: `ok github.com/halbritt/striatum/go/pkg/mutations (cached)`.
- `PYTHONPATH=src .venv/bin/python - <<'PY' ... validate_artifact_front_matter(...)`
  - Result: `front matter valid`.
- `git diff --check -- docs/operator/plans/deferred-20-todo59-external-fetch-closure.md docs/operator/workflows/deferred-20-todo59-external-fetch-closure docs/operator/artifacts/deferred-20-todo59-external-fetch-closure`
  - Result: passed.

## Shared Updates To Queue

None required from this worker. TODO #59 and roadmap section 5.7 already say
the optional external-consumer fetch UX remains out of core. If an operator
later wants a visible status note, it can point at this closure artifact
without changing the product boundary.
