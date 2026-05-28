---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Finding: GH #15 Verification

Verified against `docs/issues/15/SPEC.md` and the implementer handoff.

## Verdict: accept

## Acceptance Checklist

- [x] **Consistent PostgreSQL-first story**: `README.md`, `docs/SPEC.md`, `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`, `docs/CLI_REFERENCE.md` all reflect the post-D094 / RFC 0043 state model.
- [x] **New runbook**: `docs/POSTGRES_TRANSITION.md` created and linked from `docs/INDEX.md` and `README.md`.
- [x] **Ubiquitous Language**: `docs/UBIQUITOUS_LANGUAGE.md` updated with post-D094 terms (daemon-required CLI, repo-local migration, operational scratch).
- [x] **Skill Templates**: `src/striatum/skills/templates/claude_code/workflow.md.tmpl` (and others) updated to teach the PostgreSQL substrate model.
- [x] **Regression Test**: `tests/test_doc_links.py::test_current_product_docs_do_not_claim_sqlite_authority` exists and passes.
- [x] **RFC Distinctions**: Docs correctly distinguish between RFC 0033 (global substrate) and RFC 0043 / D094 (per-repo state).
- [x] **Remaining Work**: RFC 0048 (handler-port work) marked as remaining work in `README.md` and `docs/POSTGRES_TRANSITION.md`.

## Verification Assessment

- `pytest tests/test_doc_links.py` passes (5/5 tests).
- `striatum daemon migrate-repo-local --help` matches the SPEC.
- Fixed a pre-existing documentation budget failure in `docs/DECISION_LOG.md` for row `D094` (compressed from 439 words to ~175 words to meet the 200-word budget).

## Findings

None.
