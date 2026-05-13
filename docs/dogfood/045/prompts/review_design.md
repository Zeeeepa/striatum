# Design Review Prompt (RFC 0038 V1.5)

Produce REVIEW.md at `docs/dogfood/045/review/design/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`: `ergonomics_dx`.

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version` must be the exact string `"striatum.finding.v1"`, not `"1"`):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["ergonomics_dx", "rfc-0038", "v1-5", "design"]
---

author: (role)-unknown-model-<NN>
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Expected shape `(role)-unknown-model-<NN>` (session ordinal, no lane prefix).

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the synthesis at `docs/dogfood/045/DESIGN_SYNTHESIS.md`. Apply the ergonomics_dx lens: is each finding's fix concrete (file paths + names, not "the plugin")? After `make ui-build`, can a first-time developer see real bundles land and mount on `/workflows/new` without surgery?

Specific checks:

- F1 names the exact `vite.config.ts` deletion and the resulting bundle filenames under `src/striatum/web/static/build/` (real, not placeholder).
- F2 picks ONE side of the prop-contract fix and names both the server route file and the chooser island file.
- F3 picks ONE shared-bundle approach (non-mounting entry, or guarded `main.ts`) and shows how islands import it without double-mount.
- F4 names the exact `package_data`/`MANIFEST.in`/`pyproject.toml` surface and the Python serving path.
- Supply-chain hygiene: lockfile policy is explicit; `npm audit` baseline lives at a named path; Makefile target name is concrete.
- Backward-compat assertion is explicit: existing islands mount, served URLs unchanged, `/workflows/new` keeps rendering.

Cite the synthesis section(s) you are challenging. Hand-waving findings ("the design is unclear") without a pinpoint citation will be down-weighted by the verdict gate.

**IMPORTANT — write the REVIEW.md directly in this single supervised invocation.** If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
