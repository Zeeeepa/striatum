# Synthesize Phase 1 Follow-Up Order

Read the four TODO 55/56/59/60 implementation packets. Produce the expected
synthesis artifact only.

The synthesis must:

- order the four implementation slices by dependency and risk;
- identify which slices can proceed in parallel with disjoint write scopes;
- call out shared files that must be serialized, such as daemon contracts,
  migrations, registry generation, or command authority docs;
- define an immediate first batch that can be implemented safely;
- list validation commands for the batch;
- preserve D124-D127 without inventing new product decisions.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
