# Implement Prompt

Implementation is blocked until `review_design_ergonomics` returns an accepting verdict. Do not start implementation from RFC 0037 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/039/DESIGN_SYNTHESIS.md` and the resolved ergonomics_dx review findings. Stay inside the workflow write scope.

Expected behavior:

**RFC 0037 V1 slice (10 ergonomic wins):**

1. Localtime toggle + base.js scaffold + skip-link + keyboard-shortcut help `<dialog>`
2. Run-list filter row (search + state pills + date-range) + duration column + empty state
3. Workflows-index filter row + last-modified column + empty state
4. Doctor grouping by problem kind + collapsible sections + "hide terminal-run problems" toggle + empty state
5. Graph node hover tooltips on run detail
6. Keyboard shortcuts (`g r` / `g w` / `g c` / `g d` / `?`) with input-focus guard
7. `app.css` dark-mode parity blocks per the audit list
8. Next-actions panel promotion on run detail (banner just below run header for non-terminal runs)
9. Empty-state copy with copy-paste CLI examples and HOW_TO_HUMAN anchor links
10. Documentation updates (HOW_TO_HUMAN keyboard-shortcut table + filter walkthrough; CHANGELOG Unreleased entry; RFC 0037 status update)

Do NOT:

- Add a new runtime dependency, CDN script, npm package, or bundler.
- Convert any page to an SPA.
- Change the visual palette / spacing scale / fonts / status pill semantics.
- Touch the JSON API (`/v1/*`), the SSE event feed, the CSP, the MCP surface, the mutation gate, the workflow visual builder, or the chat surface.
- Add filter-state-in-querystring (deferred to V1.5).
- Add sticky-positioned next-actions banner (deferred).
- Make keyboard shortcuts configurable (deferred).
- Add a polyfill for `<dialog>` (modern browsers only; D073 audience).
- Author the operator-side `daemon describe --workflow` enhancement (out of scope).
- Add devil's-advocate / security review jobs to this dogfood's workflow (deferred per operator decision in commit 9d95487).

**Test coverage requirements:**

- Existing UI snapshot tests continue to pass.
- New JS unit tests where reasonable: duration formatter (covers `< 60s` / `< 1h` / `≥ 1h` / running); localStorage helpers (get/set/migrate); filter predicates (search across run_id + branch + workflow_id; state match; date-range comparison); keyboard shortcut input-focus guard.
- Manual checklist documented in BUILD_HANDOFF for things that can't be automated (keyboard shortcuts disabled on input focus; dark-mode parity visual check; tooltip positioning; help overlay dismissal; localtime rewriting of all `<time>` elements).

## Maximize sub-agent usage

Per the harness profile, native sub-agent delegation is **encouraged**. RFC 0037 is the most parallelizable dogfood so far. Spawn sub-agents in parallel for:

- **One sub-agent per template edit**: `base.html`, `run_list.html`, `workflows_index.html`, `doctor.html`, `run_detail.html`.
- **One sub-agent per new JS file**: `base.js` (localtime + keyboard shortcuts + localStorage helpers + skip-link + help dialog), `run_list.js` (filter + duration), `workflows_index.js` (filter + last-modified), `doctor.js` or doctor inline JS (grouping + toggle), `run_detail.js` extension (graph tooltips).
- **One sub-agent per app.css dark-mode block** (or batched if simple): `.job-list` / `.job-link` / `.status-pill` / `.posture-chip` / `.run-grid` / `.run-jobs-rail` / `.run-meta` / `.run-events` / `.workflow-graph` / `.workflow-edit-form`.
- **One sub-agent for each empty-state copy block**: run list, workflows, doctor.
- **One sub-agent per doc surface**: HOW_TO_HUMAN keyboard-shortcut table; HOW_TO_HUMAN filter walkthrough; CHANGELOG Unreleased entry; RFC 0037 status update; UBIQUITOUS_LANGUAGE update if applicable; README cross-reference if applicable.
- **One sub-agent per new test file**: JS unit tests for duration formatter, localStorage helpers, filter predicates, input-focus guard.
- **Exploratory sub-agents** to read existing modules: `src/striatum/web/static/base.css` (palette variables), `src/striatum/web/static/app.css` (current component classes), existing UI snapshot tests (so the new tests follow the same shape).

Do NOT delegate (parent session owns these):

- BUILD_HANDOFF.md authorship.
- Integration step where sub-agents' outputs are reconciled (e.g. making sure the filter row template aligns with the JS handlers' expectations).
- `make install`/`lint`/`typecheck`/`test`/`smoke` invocations.
- Final commit-shape and scope discipline.

**Operational note on long-running test runs:** Per dogfood-038 OPERATOR_REPORT intervention #5, a lease can expire if `make test` takes longer than ~30 minutes. Run focused pytest invocations first (`pytest tests/test_web*.py tests/test_chat*.py`) before the wider `make install/lint/typecheck/test/smoke` final verification. If the lease still expires, the operator handles surgical recovery; do not retry destructively.

## Verification

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place. The web UI surface should be exercised manually via the running `striatum serve --web` instance (the operator has it running on localhost:8088 and bridged via tailscale at https://proximal.tail0ecc2e.ts.net:8443).

## Handoff

Produce `docs/dogfood/039/BUILD_HANDOFF.md` summarizing changes, files modified/created, JS unit tests added, manual checklist results (the things that can't be automated), deferred items with pointers, and any human-decision items the ergonomics_dx review did not pre-resolve. If sub-agents were used, briefly note which sub-tasks were delegated.

The byline must be `author: implementer-codex-gpt-5.5-001` (or whatever the work packet supplies) — plain Markdown line, lowercase `author:`, no decoration.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
