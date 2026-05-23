# Scaffold RFC 0074 Phase A Catalog/Generator Work

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on RFC 0074 Phase A catalog/generator metadata, read-only discovery, and
one validating example. Reconcile with any existing RFC 0074 Phase A artifacts
before proposing the next implementation slice.

The scaffold must include:

- the graph-shape, role-pack, adversary-pack, and catalog-variant vocabulary;
- catalog/generator metadata and read-only discovery work that belongs in
  Phase A;
- one example-validation path and the exact tests that prove it validates;
- deferred Phase B items such as generated expanded shapes, pack-selection
  flags, chooser pack selection, and cost warnings;
- implementation write scopes and serialization points;
- explicit non-scope for RFC 0052 debate artifacts, hosted templates, or
  runtime daemon state for packs.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
