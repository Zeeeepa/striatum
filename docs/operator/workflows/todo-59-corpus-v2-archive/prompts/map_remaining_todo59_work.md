# Map Remaining TODO 59 Work

You are mapping the remaining TODO 59 Corpus Contract V2 and archive follow-up.
Produce the expected synthesis artifact only. Do not edit source, tests, or
shared docs in this job.

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Map current behavior against D126, `docs/TODO.md` item 59, `docs/SPEC.md`, RFC
0057, RFC 0066, current archive/corpus source, and existing tests. The map must
separate:

- completed behavior already covered by source and tests;
- archive-default enforcement still missing or ambiguous;
- deep-chain verification behavior that is implemented, optional, or only
  metadata;
- read-only semantic inspection gaps;
- incremental watermarking gaps;
- any augmentation-reference fetch surface that should stay optional and local.

Recommend one smallest implementation slice for the next job, with exact files
to inspect or modify, focused tests to run, and deferrals that should remain
outside the slice.

Use valid `striatum.synthesis.v1` front matter.
