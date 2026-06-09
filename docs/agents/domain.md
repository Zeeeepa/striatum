# Domain Docs

How the engineering skills should consume this repo's domain and decision documentation when exploring the codebase.

## Before exploring, read these

- `docs/reference/ubiquitous-language.md` for Striatum's project vocabulary.
- `docs/reference/spec.md` for the product boundary and current behavior.
- `docs/decisions/decision-log.md` for accepted, superseded, deferred, and rejected product or architecture decisions.
- `docs/operator/BRIEF.md` for the current operator state and bounded plan links.

If a referenced file does not exist in a future branch, proceed silently. Do not suggest creating generic `CONTEXT.md`, `CONTEXT-MAP.md`, or `docs/adr/` files for this repo unless a user explicitly asks for that layout.

## Layout

This repo uses Striatum's existing single-context documentation layout:

```text
/
├── docs/
│   ├── reference/
│   │   ├── spec.md
│   │   └── ubiquitous-language.md
│   ├── decisions/
│   │   └── decision-log.md
│   └── operator/
│       └── BRIEF.md
└── go/
```

## Use the project's vocabulary

When your output names a Striatum concept, use the term as defined in
`docs/reference/ubiquitous-language.md`. Do not drift to generic synonyms for
core concepts such as target repository, runner state, artifact, adapter, lane,
session, and work packet.

If the concept you need is not in the vocabulary yet, note it as a documentation gap instead of inventing a new term silently.

## Flag decision conflicts

If your output contradicts an accepted or superseded decision in
`docs/decisions/decision-log.md`, surface it explicitly rather than silently
overriding it:

> Contradicts D### in `docs/decisions/decision-log.md`; reopen only with a new accepted decision.
