# Implement task — RFC 0053 Phase B (codex, single track)

Build to `DESIGN_SYNTHESIS.md`. Sub-agents per layer (see
`roles/implementer.md` § "Sub-agents per layer").

**Required inputs:** `DESIGN_SYNTHESIS.md` +
`review/design/REVIEW.md`. Address every finding before publishing.

**Allowed paths:** see `roles/implementer.md`. Do not touch
`src/striatum/daemon_pg/sql/` (no migration) or
`src/striatum/daemon.py`.

**Acceptance bullets:**

- A1. `make lint typecheck test` green.
- A2. Every workflow.json in `docs/dogfood/*` + `examples/*` validates
      against the v1.2 validator (with deprecation warning OK).
- A3. `striatum workflow upgrade <path>` on each pre-rename workflow
      produces a v1.2 workflow that validates without warnings; run
      again is a no-op.
- A4. `striatum status --json` on a run in `waiting_escalation` state
      returns the new name; running against historical PG state with
      `runs.state = 'waiting_human'` returns the value as-is (dual-read).
- A5. CLI flag NAMES unchanged (existing operator scripts grep against
      flag names; only stderr/help text changes).
- A6. `docs/UBIQUITOUS_LANGUAGE.md` updated with new terms +
      back-compat note.

**Deliverable:** `docs/dogfood/063/build/HANDOFF.md`.
