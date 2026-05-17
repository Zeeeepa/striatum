# Operator Report — Dogfood 063 (RFC 0053 Phase B schema rename)

author: operator

**Branch:** `striatum/dogfood-063-rfc-0053-phase-b-schema-rename`
**Workflow:** `docs/dogfood/063/workflow.json`
**Closes:** RFC 0053 Phase B + TODO #44 + ROADMAP §5.8.

## Status

Scaffolded 2026-05-16 as the schema-rename planning slice. The escalation
artifact-kind decision was deferred from this run and later landed in the
Phase 5 escalation-inbox work; remaining escalation policy questions are
tracked in the roadmap/TODO rather than here.

## Pre-flight

Same as dogfood-061. Additionally:

- `grep -rn "human_checkpoint\|waiting_human" src/striatum/ --include="*.py" | wc -l`
  → capture the pre-rename literal count for the implementer to verify
  all sites are covered.
- `find docs/dogfood examples -name "workflow.json" | xargs grep -l "human_checkpoint" | wc -l`
  → count of workflows that will produce deprecation warnings post-rename.

## Run

```bash
WORKFLOW=docs/dogfood/063/workflow.json
striatum --repo . workflow validate "$WORKFLOW" --json
striatum --repo . run prepare --workflow "$WORKFLOW" --json
# → run_id
striatum --repo . branch confirm --run-id <run_id> --branch striatum/dogfood-063-rfc-0053-phase-b-schema-rename --json
striatum --repo . run start --run-id <run_id> --json
striatum --repo . dashboard --run-id <run_id>
```

## Friction log

(empty)

## Decisions

- Escalation artifact-kind scope deferred from this run; follow-up Phase 5
  shipped `striatum.escalation.v1`, inbox projection, and list/show/resolve.

## Post-landing checklist

- [ ] `make lint typecheck test` green.
- [ ] All workflows in `docs/dogfood/*` + `examples/*` validate.
- [ ] `workflow upgrade` idempotent on all of them.
- [ ] `pyproject.toml` minor bump.
- [ ] `CHANGELOG.md` with operator-facing deprecation note.
- [ ] `docs/UBIQUITOUS_LANGUAGE.md` reflects new vocabulary.
- [ ] RFC 0053 Phase B → ✅ shipped.
- [ ] Merge dogfood; tag; push.
