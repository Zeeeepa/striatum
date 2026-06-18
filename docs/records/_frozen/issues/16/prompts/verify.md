# Verify — does the fix close GH #16?

You are the **reviewer** for GH #16. Fresh session, no prior context
from the triage or fix jobs. Your output is a verdict against the
issue's own Definition of done.

Posture: **ergonomics_dx**. The primary question is whether a fresh AI
session pasted into the new prompt could become the Striatum operator
without reading historical dogfood prompts first.

## Read these (only — fresh-context rule)

1. `docs/issues/16/SPEC.md` — the GH issue body, your authoritative
   acceptance source.
2. `prompts/OPERATOR_INITIALIZATION_PROMPT.md` — the new file the
   implementer produced.
3. `prompts/README.md` — verify the index edit landed.
4. `prompts/OPERATOR_BOUNDARY_PROMPT.md` — verify it still exists and
   matches whatever the implementer's HANDOFF said (untouched, trimmed,
   or refactored).
5. `docs/issues/16/HANDOFF.md` — read the implementer's mapping from
   acceptance checks to file:lines, but **do not trust it without
   verifying each citation against the actual file content**.

You may consult `docs/ROADMAP.md` §3 to verify the new prompt's
operator-decision-rule pointers resolve. Do not pull in other context.

## Produce `docs/issues/16/review/REVIEW.md`

Front-matter (required for finding.v1):

```yaml
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "<accept | accept_with_findings | needs_revision>"
severity: "<info | low | medium | high>"
tags: ["ergonomics_dx", "gh-16", "operator-init-prompt"]
---

author: reviewer-unknown-model-001
```

Sections:

1. **Verdict.** One of:
   - `accept` — every Definition-of-done bullet, every Required-shape
     fill-in slot, every Required-behavior rule, and every
     Required-first-action-sequence step is present, and ergonomics is
     clean.
   - `accept_with_findings` — every Definition-of-done bullet is
     functionally closed, but cosmetic or low-severity gaps remain
     (e.g., a fill-in block label is slightly misordered, a recovery
     command is missing one variant). Findings recorded; not blocking.
   - `needs_revision` — at least one Definition-of-done bullet is
     unaddressed, OR the prompt is functionally incomplete from the
     "fresh AI session can become operator without historical
     prompts" lens, OR the generic-language rule is violated
     (RFC0026 / RFC0027 / Engram-specific paths present).

2. **Acceptance checklist.** Iterate the numbered list from
   `docs/issues/16/SCOPE.md` § "Acceptance checklist". For each item:

   ```
   - [DoD-1] PASS — prompts/OPERATOR_INITIALIZATION_PROMPT.md:1 reads `Status: reusable`.
   - [DoD-2] PASS — sections include fill-in block (L14-32), boundary rules (L34-66), operating rules (L68-92), first-action sequence (L94-115), reporting (L117-128). This is a complete initialization prompt.
   - [RS-1] PASS — fill-in block L16 has `Striatum repo path: <...>`.
   - [RS-7] FAIL — no fill-in slot for "Daemon/Postgres state and whether direct mode is allowed for this run" (SPEC §Required shape, bullet 7).
   - ...
   ```

   Cite file:line for every PASS and FAIL. A FAIL must include a
   one-line reason.

3. **Generic-language scan.** Run a substring check (literally) for:
   `RFC 0026`, `RFC0026`, `RFC 0027`, `RFC0027`, `engram`, `Engram`,
   `~/git/engram`, `engram-mcp-stdio`. List any hits with file:line.
   Per the SPEC, any hit is a `needs_revision`-grade finding.

4. **Daemon/Postgres caveat check.** Verify the prompt:
   - acknowledges the RFC 0043 V1 transition;
   - does NOT claim daemon-required default is flipped (RFC 0043 V1.5
     item 31(b) is still open);
   - mentions `STRIATUM_DAEMON_REQUIRED=0` + `STRIATUM_TEST_HARNESS=1`
     as the current test-harness escape per the SPEC's "RFC0048 caveat
     where needed" requirement.

5. **Ergonomics question — the primary acceptance bar.** In 2-3
   sentences, state whether a fresh AI session pasted into this prompt
   (with all fill-in slots completed for a specific run) has
   everything it needs to drive the run without reading historical
   dogfood prompts. If no, say what's missing.

6. **Findings (if any).** Severity-tagged. For each:
   - file:line of the gap;
   - what's wrong;
   - suggested fix in one sentence.

7. **Final verdict.** Restate, with rationale tying back to the
   checklist and ergonomics check.

## Constraints

- Verify each HANDOFF citation against the actual file. The HANDOFF is
  the implementer's claim; your job is to test it.
- Do not propose adding features beyond the SPEC. If the implementer
  did less than the SPEC asked, that's `needs_revision`; if the
  implementer did more than the SPEC asked, note as a finding but
  don't fail on it.
- **Fresh context discipline:** do not bring in patterns from prior
  reviewers' work on other issues. The SPEC is your only acceptance
  source.

## Commands you'll need (verbatim from the work packet)

```
ack:              striatum ack --session-id <S> --message-id <M> --lease-id <L>
publish_artifact: striatum publish-artifact --session-id <S> --job-id <J> --lease-id <L> --kind finding --logical-name verify_review --path docs/issues/16/review/REVIEW.md
verdict:          striatum verdict --session-id <S> --job-id <J> --lease-id <L> --verdict <V> --rationale "<short reason>"
complete:         striatum complete --session-id <S> --job-id <J> --lease-id <L>
```
