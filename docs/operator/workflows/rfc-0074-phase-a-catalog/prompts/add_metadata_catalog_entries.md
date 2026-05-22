# Add Metadata Catalog Entries

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement the smallest RFC 0074 Phase A catalog metadata slice and produce
`docs/operator/artifacts/rfc-0074-phase-a-catalog/build/HANDOFF.md`.

Required scope:

- add metadata entries for `implementation_panel`, `strategy_review`, and
  `premortem`, plus the deferred RFC 0076 `code_doc_audit` entry when it fits
  the metadata shape cleanly;
- add role-pack and adversary-pack metadata from the discovery artifact;
- extend catalog loading, validation, list/show/render helpers, and read-only
  service discovery so packs are inspectable;
- add focused tests for pack metadata and unchanged runtime/generator
  semantics;
- update operator-facing docs only where the work packet write scope allows.

Do not implement:

- `workflow generate --shape implementation_panel`;
- `--role-pack`, `--adversary-pack`, or score-dimension generation options;
- web chooser controls for selecting packs;
- RFC 0052 debate/panel artifact schemas or daemon methods;
- new artifact front-matter schemas.

The handoff must list changed files, validation commands, and any intentional
Phase B deferrals.
