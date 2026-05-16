# Synthesis task — RFC 0051 V1 single-track lock

Read the three designer DESIGN.md files under
`docs/dogfood/061/design/{codex,claude_code,gemini}/` and reconcile
them into one concrete plan.

## What you must lock (front-matter `synthesis.v1`)

For each of the seven designer questions, pick ONE answer:

1. **Hook location** — single module + function signature.
2. **Per-session scan order** — exact numbered list.
3. **Atomic finalize sequence** — function-call list inside one
   `conn.transaction()`.
4. **Event payload shapes** — full JSON shape for each new event.
5. **Feature-flag check point** — file:line where the env-var read
   gates the whole path.
6. **Refusal table** — every disqualifier + its fall-through.
7. **Acceptance tests** — 4 test functions with full module paths
   and fixture inputs.

## Track shape

**SINGLE implement track only.** Auto-finalize is one cohesive
feature; the 058 dual-track cycle-exhaustion lesson applies. Do not
propose a build_a/build_b split. Sub-agents per cluster are fine
(hook+scan, finalize sequence, events, flag) but they all land in
the same implement job.

## Anti-patterns reviewers will bounce on

- Three-synth-attempt pattern (058 cycle exhaustion).
- Dual-track build_a/build_b.
- Menu of options on any locked decision.
- A locked file path that doesn't actually exist in main (cite
  with line numbers).

## Write scope

`docs/dogfood/061/DESIGN_SYNTHESIS.md` only.
