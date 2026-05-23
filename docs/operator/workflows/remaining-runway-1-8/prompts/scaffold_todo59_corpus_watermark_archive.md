# Scaffold TODO 59 Corpus Watermark And Archive Follow-Through

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the remaining Corpus Contract V2 follow-through after the V2 manifest
and archive-default slices: incremental watermarking, archive verification
follow-through, and any local-only augmentation-reference handoff that remains
bounded by D126.

The scaffold must include:

- current-state assumptions from RFC 0057, RFC 0066, D126, and recent archive
  closure evidence;
- proposed watermark identity/storage semantics and verification behavior;
- archive follow-through checks that stay local, deterministic, and read-only
  where appropriate;
- augmentation-reference notes that preserve augmentation-not-dependency;
- implementation write scopes, compatibility tests for V1/V2 bundles, and
  no-Engram guardrails;
- explicit non-scope for hosted persistence, runtime retrieval dependency, or
  transcript capture.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
