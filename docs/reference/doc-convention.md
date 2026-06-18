---
type: reference
status: canonical
owner: halbritt
expires: null
---

# Documentation convention (striatum)

striatum follows the **shared** documentation convention single-sourced in
[`doc-convention-lint`](https://github.com/halbritt/doc-convention-lint) and
vendored here, pinned by SHA, via `.pre-commit-config.yaml`. There is one
convention across striatum + engram; this repo supplies only an extend-only
overlay (`./doc-convention.yaml`).

This convention is the **layout + enforcement** companion to
[`doc-map.md`](doc-map.md), which remains the **concept-ownership** contract
("one home per concept, every other doc cites it"). doc-map says *which doc owns
a concept*; this convention says *which shelf a doc lives on and how that is
machine-checked*.

## The model

Two axes. **Curated vs Exhaust:** curated docs are human-intent, mutable, edited
in place; exhaust is machine-generated, write-once, time-ordered run output
(committee reviews, audits, operator workflows, agent handoffs). **Diataxis**
(tutorials / how-to / reference / explanation) governs only the curated half.
Exhaust lands in one explicitly-named region, `docs/records/`, which is write-once
and perishable. The rolling `docs/decisions/decision-log.md` is curated reference;
individual dated decision records are exhaust. RFCs are curated-special and stay in
`docs/rfcs/`.

## TL;DR for an agent about to write a doc

1. Is this a side-effect of a run (review / audit / handoff / scaffold)?
   → `docs/records/<kind>/`, front-matter `type: record` + an `expires:` date. **Done.**
   (This is where loose root-level `STRIATUM_*_<DATE>.md` audits belong.)
2. Else it's curated: pick the Diataxis `type` by *purpose* (learning→tutorial,
   task→how-to, lookup→reference, understanding→explanation; design→rfc), write
   under the matching `docs/<type>/` path with `status: working|canonical`.

## Status

**Migration Phase 1 — warn-only.** The linter reports but does not block. striatum
already has the Diataxis directories, so its remaining migration is the fold of
`docs/operator/` + `docs/campaigns/` + `docs/_archive/` + `.agents/` run artifacts +
the loose root audits into `docs/records/` (+`_frozen/`), tracked in the
[migration plan](https://github.com/halbritt/doc-convention-lint/blob/master/MIGRATION_PLAN.md).
Run `doc-lint lint --all --warn-only` to see current drift.
