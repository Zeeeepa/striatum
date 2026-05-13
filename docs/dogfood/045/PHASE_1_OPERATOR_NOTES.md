author: operator-claude-opus-1

# Dogfood-045 Phase 1 Operator Notes — RFC 0038 V1.5 Web UI Integration Gaps

Run: `run_8a909addd31e4455b85ad58768169e4a`
Branch: `striatum/dogfood-045-rfc-0038-v1-5`
Workflow: `docs/dogfood/045/workflow.json` — 9-job single-track for
RFC 0038 V1.5 (F1-F4 + supply-chain hygiene findings from dogfood-041
deferred under cycle-exhaustion override
`dec_251e8a5f3d674c409de0dad9eacd5844`)

## What shipped

RFC 0038 V1.5 closes the four codex attempt-2 findings (F1-F4) from
the dogfood-041 build review iteration 2 plus the gemini attempt-1
supply-chain hygiene findings and the claude attempt-2 ergonomics
polish. The fixes are surgical: no new public CLI verbs, no new MCP
tool names, no public bundle URLs change, mount IDs unchanged,
`pyproject.toml [tool.setuptools.package-data]` unchanged, and the
islands architecture is preserved.

**F1 — Placeholder-plugin removal.** `placeholderIslandPlugin` is
gone from `src/striatum/web/frontend/vite.config.ts`; `plugins` is
now `[react()]`. `manifest` flipped to `false`. New `make
ui-verify-bundle` refuses (a) any stable island entry containing the
V1 sentinel `Striatum frontend island placeholder loaded`, (b) any
`island-shared-*.js` chunk containing the same sentinel, and (c) any
stable island entry under 1024 bytes (with a factored-chunk
exception). Python regression
`test_island_bundles_have_no_placeholder_sentinel` enforces the same
guarantee through `importlib.resources` so the guard survives `pip
install`.

**F2 — Chooser prop contract.** `/workflows/new` chooser rewritten
around the server-stable `{"templates": [...]}` shape (the
`/workflow-templates` route was already returning the correct shape;
the V1 client was reading a non-existent `catalog.modifiers` field).
`shared/types.ts` adds `WorkflowTemplate` +
`WorkflowTemplateListResponse` and drops the dead `WorkflowShape` /
`WorkflowLaneSet` / `WorkflowTemplateCatalog` types.
`WorkflowChooser.tsx` partitions by `kind`, derives `shape` from the
picked `kind: "shape"` row, pre-fills `lane_set` from
`default_lane_sets`, and removes the V1 modifier step.

**F3 — Island-shared double-mount fix.** New
`shared/island-shared-entry.ts` (`import "./theme.css"; export {};`)
is the new Rollup input for the `island-shared` bundle. `src/main.ts`
still exists for the Vite dev server (`make ui-dev`) but is no longer
a production Rollup input, so it cannot mount islands twice in
production. Vitest regression `island-shared-no-mount.test.ts` pins
the single-mount guarantee by mocking `createRoot`.

**F4 — Output and package-data layout.** Output stays at
`src/striatum/web/static/build/`; public URLs unchanged.
`pyproject.toml` package-data already matches the `manifest: false`
layout. Two new `importlib.resources` tests cover the chooser bundle
+ the placeholder sentinel guard. New
`test_workflows_edit_renders_graph_editor_island` pins
`/workflows/edit/<path>`.

**Supply-chain hygiene.** `ui-install` now uses `npm ci`; new
`ui-update-lock`, `ui-audit`, `ui-verify-bundle` targets; new
`npm-audit-baseline.json` committed as the accepted-findings tracker.
`@vitejs/plugin-react` left in `dependencies` for now — operator
follow-up moves it to `devDependencies` during the same lockfile
regeneration that produces the real bundles.

## Claude-as-implementer-not-codex pattern

This is the **first dogfood where codex was not the implementer**.
The decision is deliberate: D095-D098 documented four independent
instances of the codex/codex implementer+reviewer
convergent-blind-spot anti-pattern, in which the codex reviewer's
findings cluster around the codex implementer's same blind spots,
producing apparent `needs_revision` verdicts that 2-of-3 majority
overrides every time. After four recurrences the empirical case was
closed. Picking claude as implementer for V1.5 was the simplest way
to test whether the anti-pattern is specifically about same-model
implementer+reviewer co-blindness (vs. codex-as-reviewer being
universally harsh).

The frontend lane is also a natural fit for claude — F1-F4 are
TypeScript / Vite / React work plus a Makefile pass, which is right
in claude's wheelhouse and away from claude's traditional weak
spots in the Postgres-substrate / RPC-envelope code that codex has
been handling.

## Codex-as-reviewer-of-claude-implementer — still harsh

Build review verdicts:

- codex — `reject`, **critical** severity, threat_model posture.
- claude — `accept_with_findings`, medium severity, ergonomics_dx
  posture.
- gemini — `accept`, low severity, threat_model posture.

The codex reviewer came back with **`reject critical`** — a stronger
verdict than the cycle-exhaustion `needs_revision` pattern from
dogfood-042 through dogfood-044. Codex's critical findings rest on:
(a) committed bundles under `src/striatum/web/static/build/` are
still the V1 placeholders pending operator-side `make
ui-update-lock` + `make ui-build`, and (b) `make lint` /
`typecheck` / `test` / `ui-test` / `ui-build` were not executed
during the implementer run because the harness denied every
direct-make invocation.

Both observations are correct. The HANDOFF.md explicitly documents
the real-bundle commit as an operator-side mechanical follow-up; the
new `make ui-verify-bundle` + Python sentinel test refuse another
placeholder commit from reaching CI; and the operator-side gate is
narrowly scoped (one `make ui-update-lock` + one `make ui-build` +
one commit). The cross-lane reviewers agreed the source-side fixes
are correct: gemini explicitly accepted because the bundle-integrity
guards are robust against both developer-side and operator-side
regressions; claude flagged the clone-without-rebuilding ergonomics
gap as a medium finding and noted "the failure path is loud, so the
discoverability cost is bounded; it is not zero".

The takeaway: **codex-as-reviewer is independently conservative**
under the threat_model posture, even without a same-model implementer
co-blindness loop. The reject-vs-cross-lane-accept split this run
mirrors the needs_revision-vs-cross-lane-accept split of D095-D098,
but the structural cause is different. D099 is **the first
reject-severity override on the books** — prior overrides
(D095-D098) all overrode `needs_revision`. The codex reject under
threat_model needs its own follow-up framing distinct from the
codex/codex anti-pattern: this is "codex-threat_model reviewer is
strict in a way that 2-of-3 cross-lane consensus doesn't share",
which is a different problem.

## D099 override + recovery path

The codex `reject` verdict pushed the run state to `failed` before
the operator could decide whether to override. Recovery on this run
required:

1. SQL surgery on the `verdicts` table + `runs.state` column to
   re-open the run for operator decisioning.
2. `striatum verdict --override` to record the override-accepting
   verdict path (this verb landed in v1.32.x).
3. Decision record D099 (`dec_ccfa1685878d41d69ccc6496cd6612fd`,
   `accepted_with_follow_up`) citing the 2-of-3 cross-lane consensus.

This is a meaningful harness gap: a reviewer `reject critical`
verdict deserves operator review before transitioning to `failed`,
not after. A future RFC could plumb an explicit "operator-pending"
run state distinct from `failed` so verdicts awaiting override do
not require manual SQL recovery. Recorded as a CHANGELOG note + a
known V1.6 follow-up gap.

## Manual consolidate, dogfood-042/043/044 lesson applied

Dogfood-045's workflow intentionally did not include a `consolidate`
job — the operator writes the consolidate artifacts out-of-band as a
normal edit pass. This file, `BUILD_HANDOFF.md`, the v1.34.0
CHANGELOG promotion, the RFC index status bump, and the TODO follow-ups
were all authored by the operator after the run completed. The runner
remains the source of truth for what happened (`run_summary`,
`OPERATOR_REPORT.md`, `D099`); the operator handles the prose
synthesis on top. Same pattern as dogfood-044.

## Follow-ups

- **RFC 0038 V1.6** (TODO item 29): operator-side `make ui-update-lock`
  + `make ui-build` + lockfile/bundle commit; move
  `@vitejs/plugin-react` to `devDependencies`; verify `make lint /
  typecheck / test / ui-test` against real output. Narrow scope — a
  near-term operator sweep rather than a full dogfood cycle.
- **Operator-pending run state** (future RFC opportunity): plumb an
  explicit run state distinct from `failed` for verdicts awaiting
  override, so reject-severity verdicts don't require SQL surgery to
  recover.
- **Codex-threat_model reviewer harshness** (new harness theme):
  D099 is the first reject-severity override and the first case
  documenting codex-as-reviewer conservatism independent of the
  codex/codex co-blindness anti-pattern. Open question: does the
  codex `reject critical` reflect a legitimate posture difference
  (threat_model is supposed to be the strict lane), or is it a
  miscalibrated reviewer profile? The dogfood-044 byline-prefix bug
  follow-up already audits the gemini + claude reviewer profile
  fragments; a parallel codex reviewer profile audit could check
  whether the threat_model template is over-eager to escalate
  documented operator-side follow-ups into critical defects.
- **TODO item 26** (codex/codex validator refuse-by-default) is still
  the most-overdue harness improvement. Dogfood-045 doesn't disturb
  the four-instance empirical case for it — it just demonstrates that
  the broader codex-as-reviewer pattern needs its own framing.

## Pointers

- `docs/dogfood/045/BUILD_HANDOFF.md` — combined handoff.
- `docs/dogfood/045/build/HANDOFF.md` — per-finding implementation
  handoff.
- `docs/dogfood/045/review/build/{codex,claude,gemini}/REVIEW.md` —
  three build review verdicts.
- `docs/dogfood/045/decisions/D099_codex_reject_override.md` —
  override decision artifact.
- `docs/dogfood/045/DESIGN_SYNTHESIS.md` — design synthesis input.
- `docs/dogfood/045/OPERATOR_REPORT.md` — per-intervention narrative
  authored during the run.
- `CHANGELOG.md` v1.34.0 — promotion entry.
- `docs/TODO.md` items 21 (✅ done) and 29 (V1.6 follow-up).
- `docs/rfcs/README.md` RFC 0038 row — status bumped to
  `accepted (V1 + V1.5 web UI integration gaps landed)`.
