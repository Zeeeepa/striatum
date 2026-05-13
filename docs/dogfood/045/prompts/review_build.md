# Build Review Prompt (RFC 0038 V1.5, 3-way)

Produce REVIEW.md at `docs/dogfood/045/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0038", "v1-5", "build"]
---

author: (role)-unknown-model-<NN>
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`. Expected shape `(role)-unknown-model-<NN>` (session ordinal, no lane prefix).

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the implementation handoff at `docs/dogfood/045/build/HANDOFF.md`.

Per-lane angle:

- **codex (threat_model)**: supply-chain integrity (lockfile committed, npm-audit baseline real, dependency tree reviewed); no placeholder bundles shipped (bundle file contents are real compiled JS, not `console.info` stubs); served paths unchanged; `package_data` surface intact.
- **claude (ergonomics_dx)**: `/workflows/new` flow works end-to-end after the prop-contract fix; first-time-developer can run `make ui-build` and see real bundles mount; island-shared double-mount actually fixed (no duplicate side effects in browser console); error surface is discoverable.
- **gemini (adversarial threat_model)**: bundle integrity (could a committed placeholder slip through CI?); prop-contract edge cases (empty `templates` list, malformed entries, server returns 4xx); double-mount exploits (island imported twice on the same page, race between `main.ts` side effects and island mount).

Required checks (all lanes):

- **F1-F4 + supply-chain covered**: cite the implementation site for each finding (file + function or line).
- **Real bundles**: inspect a shipped bundle file and confirm it is compiled output, not the placeholder `console.info` template.
- **Backward compatibility**: existing islands mount; served bundle URLs unchanged; `/workflows/new` renders. Regression-test names cited.
- **Build verification**: `make ui-build`, `make lint`, `make typecheck`, `make test` all pass per the HANDOFF; spot-check at least one.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
