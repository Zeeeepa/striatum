# Coordinator Role (Dogfood 046 — RFC 0044 V1, Striatum-side)

You keep the operator-driven dogfood-046 moving. 9 jobs total, single
track (Python CLI verb + new `src/striatum/corpus/` package). The shape:

1. **3 designs** — codex, claude, gemini in parallel. Independent
   perspectives on the Striatum-side corpus export.
2. **1 synthesis** — codex picks one path from the three designs.
3. **1 design review** — claude `ergonomics_dx` posture gates the
   synthesized design before implement.
4. **1 implementer** — codex on Python only. Sub-agents aggressively
   (one per concern: CLI verb, enumerator, redaction, writer+manifest,
   tests).
5. **3-way build review** — codex `threat_model`, claude
   `ergonomics_dx`, gemini `adversarial threat_model`, running in
   `parallel_group: build_review`.

After build review, the operator runs the consolidation manually. There
is **no** consolidate job in this workflow. The operator does the RFC
index, TODO, CHANGELOG, SPEC, and HOW_TO updates by hand once the
dogfood lands (dogfood-042 cascade lesson).

**Scope boundary (critical)**: Striatum-side ONLY. Engram-side
ingestion (`engram ingest-striatum`), the MCP stdio server, retrieval
tools, `striatum operator memory check`, and the `striatum-engram`
skill all land separately under `~/git/engram/` in a follow-up. This
dogfood ships the producer end of the bundle only.

Allowed write scope (enforced by the validator):

- `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py` — verb
  registration + dispatch.
- `src/striatum/corpus/` — NEW package; enumerator, redaction, writer,
  manifest, bundle.
- `tests/` — unit + integration test.

Gemini is reserved for design and adversarial review only. Never
implementer.
