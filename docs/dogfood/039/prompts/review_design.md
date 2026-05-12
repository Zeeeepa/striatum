# Review Design Prompt (ergonomics_dx posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0037", "web-ui", "ergonomic-improvements"]
---
```

Review `docs/dogfood/039/DESIGN_SYNTHESIS.md` under the **ergonomics_dx** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan makes the new UI affordances discoverable and consistent from a first-time-user perspective.

The ergonomics_dx posture: "This is a developer-ergonomics review. Evaluate the artifact's surface from a first-time-user perspective; verdict acceptance means the affordances are discoverable and consistent."

In scope for this review:

- **Filter UX**: placeholder text specific? Default state non-surprising? Clear-filter affordance present? Empty-result state copy specific?
- **Duration column**: format choices match the synthesis's recommendation? Running-run polling cadence honest (not too aggressive)?
- **Localtime toggle**: visual placement specific? State indicator (UTC / Local) shows current mode? Default UTC preserves existing behavior?
- **Keyboard shortcuts**: mnemonic mapping (`g r` / `g w` / `g c` / `g d`)? Discoverable via `?` overlay? Input-focus guard specified? Help overlay dismissable via Esc?
- **Empty-state copy**: specific (names action) + actionable (copy-paste CLI) + linked (HOW_TO_HUMAN anchor)? Or boilerplate?
- **Next-actions banner**: aria-label specified? When-shown / when-hidden rule clear?
- **Doctor grouping**: default-collapsed threshold honest? "Hide terminal-run problems" default behavior reasonable?
- **Graph tooltips**: positioning prevents off-screen clipping? Hover-only doesn't break click-navigate?
- **Dark-mode parity audit**: complete (every app.css class with a decision)? Or has gaps?
- **JS architecture**: vanilla, no framework, no bundler? Or has crept toward complexity?
- **Staging plan**: lowest-risk-first ordering? Each step ships independently?
- **Deferred items**: clearly named (filter-state-querystring, sticky banner, keyboard config)?
- **No new runtime deps**: synthesis specifies this and doesn't sneak in a CDN script?
- **Scope discipline**: stays inside RFC 0037 ergonomic polish; doesn't wander into visual redesign / mobile-first / feature additions?

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

**IMPORTANT — write the REVIEW.md artifact directly in this single supervised invocation.** Per dogfood-036 OPERATOR_REPORT intervention #2 + dogfood-037 intervention #5: previous reviewer sessions surfaced strategy / asked clarifying questions and exited without producing the file. Do not repeat either pattern. The work packet's `expected_artifacts` requires the file at `docs/dogfood/039/review/design/ergonomics/REVIEW.md`. Use the EXACT front-matter shape above (`verdict_intent` not `verdict`; `severity` from {low,medium,high,critical} not `none`; `tags` as a JSON array; `author: <slug>` byline as plain markdown line AFTER the front-matter block, NOT a key inside it).

Stay inside the review write scope (`docs/dogfood/039/review/design/ergonomics/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.
