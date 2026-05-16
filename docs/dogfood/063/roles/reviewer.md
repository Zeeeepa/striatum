# Reviewer Role (Dogfood 063 — RFC 0053 Phase B schema rename)

author: reviewer-role-001

See dogfood-061's `roles/reviewer.md` for the canonical verdict
shape (`accept`, `accept_with_findings`, `needs_revision`, `reject`)
and the cycle-1 discipline.

**Posture-specific bouncing checklists:**

### Design review (codex `threat_model`)

- Existing v1 workflows in `docs/dogfood/*` + `examples/*` all
  validate after the rename (with deprecation warning OK).
- Dual-name discipline covers EVERY existing reference (validator,
  generator, recovery CLI, parser --severity, list filters,
  introspect text, evidence ledger).
- The PG `waiting_human` → `waiting_escalation` transition is in
  code (handler accepts both, writes new); no SQL migration that
  rewrites historical rows.
- `workflow upgrade` is idempotent (proved by a test that runs it
  twice).
- No CLI flag NAMES change. (Operator scripts grep on flag names.)

### Build reviews (codex / claude / gemini, per workflow.json
objectives)

Standard discipline. Reject only on cycle exhaustion.

**Write scope:** `docs/dogfood/063/review/{design,build}/<lane>/REVIEW.md`.
