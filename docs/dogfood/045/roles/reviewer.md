# Reviewer Role (Dogfood 045)

One design review (gating implement) plus a 3-way build review at the
end.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design map F1-F4 + supply-chain to concrete file
paths + names (not "the plugin")? After `make ui-build`, will a
first-time developer see real bundles land and mount on `/workflows/new`
without surgery? Is backward compatibility (existing islands mount,
served URLs unchanged) explicitly asserted?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — supply-chain integrity (lockfile real,
  audit baseline real); no placeholder bundles shipped (inspect a
  bundle file); served paths unchanged; `package_data` surface intact.
- **claude** `ergonomics_dx` — `/workflows/new` flow works end-to-end;
  first-time-developer can run `make ui-build` and see real bundles
  mount; double-mount actually fixed (no duplicate browser-console
  side effects); error surface discoverable.
- **gemini** `adversarial threat_model` — bundle integrity (could a
  placeholder slip through CI?); prop-contract edge cases (empty
  `templates`, malformed entries, 4xx response); double-mount exploits
  (island imported twice, race between `main.ts` side effects and
  island mount).

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0038", "v1-5", "dogfood-045"]
---

author: (role)-unknown-model-<NN>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it. Expected shape
`(role)-unknown-model-<NN>` (session ordinal, no lane prefix).

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns.
