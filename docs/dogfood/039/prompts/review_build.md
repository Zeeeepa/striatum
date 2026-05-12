# Review Build Prompt (ergonomics_dx posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0037", "web-ui", "build"]
---
```

Review the implementation under the **ergonomics_dx** posture. Verify behavior, tests, docs, and UI mechanics. Actually try the surfaces if you can: walk through the run list filter, workflows index filter, doctor grouping, localtime toggle, keyboard shortcuts, graph tooltips.

ergonomics_dx posture: "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent."

Required checks (try each surface):

- **Run list filter row**: free-text search filters across run_id + branch + workflow_id substrings; state pills filter correctly; date-range select filters correctly; filter state persists in localStorage across reloads; clear-filter affordance present; empty-result state copy specific.
- **Duration column**: shows correct format (mm:ss / hh:mm / running-relative); running runs update on setInterval; terminal runs show static duration.
- **Workflows index filter**: search + status filter work; last-modified column shows mtimes; localtime toggle affects them.
- **Doctor grouping**: problems grouped by kind with collapsible sections; "hide terminal-run problems" toggle works; default-collapsed for groups > 5; per-kind collapsed state persists.
- **Localtime toggle**: switching `UTC` ↔ `Local` rewrites every `<time>` element on the page; preference persists; default is UTC.
- **Keyboard shortcuts**: `g r` / `g w` / `g c` / `g d` navigate correctly when focus is not on an input; disabled when input has focus; `?` opens help overlay; Esc/outside-click closes overlay.
- **Graph tooltips**: hover over an SVG `<g.node>` shows tooltip with job state + role + duration; positioning doesn't go off-screen; doesn't break the click-navigate.
- **Next-actions banner**: shown for non-terminal runs only; positioned below run-header; aria-label present.
- **Empty-state copy**: specific, actionable, with copy-paste CLI examples + HOW_TO_HUMAN anchor links.
- **Dark-mode parity**: every app.css class listed in the audit has a `prefers-color-scheme: dark` block; visual check across the run list / run detail / doctor / workflows / chat pages.
- **No new runtime dependencies**: no new `pyproject.toml` deps; no `package.json`, no `node_modules`, no bundler entries.
- **JS architecture honest**: vanilla, no framework, no bundler; each new JS file self-contained; loaded with `defer`.
- **Existing UI snapshot tests pass**: server-side render unchanged except for filter row markup + new JS includes; existing snapshots updated only where new markup is added.
- **New JS unit tests present**: duration formatter, localStorage helpers, filter predicates, input-focus guard.
- **Documentation honest**: HOW_TO_HUMAN keyboard-shortcut table accurate; CHANGELOG entry reflects shipped scope; no overclaim of mobile-first or visual redesign.

Use `needs_revision` for: behavior gaps in the shipped scope, missing tests for the surfaces above, dark-mode parity gaps, accessibility regressions (keyboard nav blocked, no focus indicator, missing ARIA labels), new runtime deps snuck in, or documentation that overstates shipped behavior. Use `accept_with_findings` for non-blocking cleanup or follow-up RFC scope.

**IMPORTANT — write the REVIEW.md artifact directly in this single supervised invocation.** Per dogfood-037 OPERATOR_REPORT intervention #5: a previous claude review session failed at the ack step, asked the operator a clarifying question, and exited without writing the file. Do not repeat that pattern. If permission to call `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf. The work packet's `expected_artifacts` requires the file on disk at `docs/dogfood/039/review/build/ergonomics/REVIEW.md`.

Stay inside the review write scope (`docs/dogfood/039/review/build/ergonomics/`). Do not modify the implementation. Do not call striatum CLI; the operator publishes otherwise.
