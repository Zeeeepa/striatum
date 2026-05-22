# Synthesize RFC 0074 Phase A

Read the RFC 0074 catalog metadata and runnable-example artifacts. Produce the
expected synthesis artifact only.

The synthesis must:

- define the exact Phase A patch sequence;
- identify disjoint write scopes and shared serialization points;
- decide what remains deferred to Phase B;
- keep RFC 0076 generator/catalog integration in this RFC 0074 lane;
- list validation commands and guardrails.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
