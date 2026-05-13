# Reviewer Role (Dogfood 046)

One design review (gating implement) plus a 3-way build review at the
end.

## Design review (claude, `ergonomics_dx`)

Does the synthesized design name exact `parser.py` and `dispatch.py`
functions, an exact `src/striatum/corpus/` layout, a chosen redaction
denylist, and exact test paths including the integration test against
a real recent dogfood? Is the augmentation-not-dependency regression
guard named? Is the JSON error envelope behavior consistent with
neighboring CLI verbs?

## Build review (3-way, `parallel_group: build_review`)

- **codex** `threat_model` — systems posture. Redaction completeness
  (`.env`, transcripts, `.striatum/state.sqlite3` blobs, raw model
  output, terminal output all excluded); manifest hash verification;
  augmentation boundary clean (no Engram import under
  `src/striatum/`); daemon RPC registry unchanged.
- **claude** `ergonomics_dx` — operator UX. `--help` discoverable;
  `--since` / `--out` errors yield a useful JSON envelope; bundle
  layout obvious; no foot-guns when `--since` omitted.
- **gemini** `adversarial threat_model` — break-the-bundle. Symlinked
  `--out` paths; dirty-tree honesty; non-UTF-8 content; oversized RFCs;
  commit messages with secrets-shaped tokens; replay determinism under
  file mtime drift; concurrent export.

## Required finding front matter (all 5 fields)

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0044", "v1", "dogfood-046"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

`schema_version` must be the exact string `"striatum.finding.v1"`
(not `"1"`). `artifact_kind` is `"finding"`. `verdict_intent` is one of
`accept | accept_with_findings | needs_revision | reject` (not
`verdict`). `severity` is one of `low | medium | high | critical`.
`tags` is a JSON array. The `author:` byline is a plain markdown line
AFTER the front-matter block — not inside it.

**IMPORTANT — write the REVIEW.md / finding artifact directly.** If
`striatum ack` is denied, write the artifact and exit normally; the
operator publishes on your behalf. Do not ask the operator clarifying
questions and exit. Per dogfood-037 intervention #5 + dogfood-041
friction patterns.
