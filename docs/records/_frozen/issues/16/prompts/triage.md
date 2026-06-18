# Triage — bound scope for GH #16

You are the **triager** for GH #16. Your job is to produce
`docs/issues/16/SCOPE.md` — nothing else. No implementation, no draft
of the new prompt. Just scope alignment.

## Read these (only, in order)

1. `docs/issues/16/SPEC.md` — the GH issue body (verbatim).
2. `prompts/OPERATOR_BOUNDARY_PROMPT.md` — the existing focused guardrail prompt.
3. `prompts/README.md` — the prompts index that currently lists the boundary prompt.
4. `docs/HOW_TO_AGENT.md` — the canonical operator agent contract.
5. `AGENTS.md` (top-level) — the project boundary doc.
6. `docs/SPEC.md` — the live product spec.

Treat `docs/ROADMAP.md` §3 (operator decision rules) as background — do
not require the implementer to re-encode it; the new prompt should
reference it.

## Produce `docs/issues/16/SCOPE.md` with this shape

Front-matter (required for synthesis.v1):

```yaml
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/16/SPEC.md", "prompts/OPERATOR_BOUNDARY_PROMPT.md", "prompts/README.md", "docs/HOW_TO_AGENT.md", "AGENTS.md", "docs/SPEC.md"]
---

author: triager-unknown-model-001
```

Sections:

1. **Files in scope.** Exact paths the implementer will create or edit.
   Expected: `prompts/OPERATOR_INITIALIZATION_PROMPT.md` (new) +
   `prompts/README.md` (edit). Decide whether
   `prompts/OPERATOR_BOUNDARY_PROMPT.md` is (a) untouched, (b) trimmed
   to focused guardrail, or (c) refactored so the new prompt reuses its
   boundary section verbatim. Make the call and justify in 1-2
   sentences. The implementer must follow your call.

2. **Out of scope.** Files / docs the implementer must NOT touch. At
   minimum: `docs/dogfood/`, `docs/rfcs/`, `src/`, `tests/`,
   `.striatum/`.

3. **Acceptance checklist.** One bullet per requirement extracted
   literally from the SPEC's "Required shape", "Required behavior",
   "Required first-action sequence", and "Definition of done"
   sections. Number them so the verify job can cite each by number.
   Example:

   ```
   - [DoD-1] prompts/OPERATOR_INITIALIZATION_PROMPT.md exists and is marked Status: reusable.
   - [DoD-2] It is a complete initialization prompt, not merely a boundary warning.
   - [RS-1] Fill-in block includes Striatum repo path.
   - [RS-2] Fill-in block includes Striatum version / command path.
   ... (all 13 RS items)
   - [RB-1] The prompt instructs the operator to read project instructions and canonical docs before acting.
   ... (all 11 RB items)
   - [RFAS-1] First-action sequence begins with "Load the project instructions and listed canonical docs."
   ... (all 8 RFAS items)
   ```

4. **Generic-language guardrails.** Explicit list of forbidden tokens
   per the SPEC ("does not hardcode RFC0026/RFC0027 or Engram-specific
   paths"). Add anything else you see that would make the prompt
   non-generic. Cite where in `AGENTS.md` the generic-language rule
   lives.

5. **Daemon/Postgres caveat.** One short paragraph describing the
   current transition state (RFC 0043 V1 landed, daemon-required
   default not yet flipped per item 31(b), `STRIATUM_DAEMON_REQUIRED=0
   + STRIATUM_TEST_HARNESS=1` still works as test-harness escape).
   The implementer should fold this caveat into the prompt's fill-in
   block guidance for the "Daemon/Postgres state" field.

6. **Cross-references the implementer should embed.** The new prompt
   should not duplicate `docs/ROADMAP.md` §3 (operator decision rules)
   — it should point at it. List the exact docs/sections to reference
   in the prompt so the implementer doesn't reinvent.

7. **What "complete" means.** Two sentences defining "complete enough
   that a fresh AI session can become the operator without reading
   historical dogfood prompts." This is the verify job's primary
   ergonomics criterion.

## Constraints

- Stay inside `docs/issues/16/` for writes.
- Do NOT draft the new prompt's body. That's the fix job's work.
- Do NOT propose deleting `prompts/OPERATOR_BOUNDARY_PROMPT.md`. If you
  recommend refactor, the implementer keeps the file path.
- Word budget: 600-1000 words. The fix job needs every check item to be
  unambiguous; that's the only reason for length.

## Commands you'll need (verbatim from the work packet)

```
ack:              striatum ack --session-id <S> --message-id <M> --lease-id <L>
publish_artifact: striatum publish-artifact --session-id <S> --job-id <J> --lease-id <L> --kind synthesis --logical-name scope --path docs/issues/16/SCOPE.md
complete:         striatum complete --session-id <S> --job-id <J> --lease-id <L>
```
