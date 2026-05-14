# Fix — author OPERATOR_INITIALIZATION_PROMPT.md

You are the **implementer** for GH #16. Your spec is
`docs/issues/16/SPEC.md` (the verbatim issue body) plus
`docs/issues/16/SCOPE.md` (the triage decision — what to touch, what
not to touch, the numbered acceptance checklist).

## Read these (only, in order)

1. `docs/issues/16/SPEC.md` — the GH issue body.
2. `docs/issues/16/SCOPE.md` — the triage call. Treat this as binding.
3. `prompts/OPERATOR_BOUNDARY_PROMPT.md` — current content. You may
   reuse its boundary section verbatim or rewrite per triage's call.
4. `prompts/README.md` — current state; you'll be editing it.
5. `docs/HOW_TO_AGENT.md` and `docs/SPEC.md` — for canonical command
   shapes and operator vocabulary.
6. `docs/ROADMAP.md` — §3 has the operator decision rules. **The new
   prompt should point at this rather than duplicate it.**

## Deliverables

1. **`prompts/OPERATOR_INITIALIZATION_PROMPT.md`** (new).
   - Title block: `Status: reusable` + Date + `author:` line in the
     standard repo style (see `prompts/OPERATOR_BOUNDARY_PROMPT.md`
     for the shape).
   - **A fill-in block** as the first usable section. One labeled line
     per SPEC "Required shape" item. Use placeholders like
     `<striatum-repo-path>` or `[fill in: striatum repo path]` — be
     consistent. Group related items (Run + Workflow + Branch
     together; Daemon/Postgres separately).
   - **Boundary rules section.** Reuse the OPERATOR_BOUNDARY_PROMPT's
     "Hard rule: do not do any role work yourself" content. If triage
     said refactor, refactor; if triage said keep boundary prompt
     untouched, copy the relevant rules verbatim with attribution.
   - **Operating rules section.** One bullet per SPEC "Required
     behavior" item. Phrased as instructions to the operator session,
     not third-person description.
   - **Recovery rules section.** Point at `striatum status`,
     `striatum why`, `striatum doctor`,
     `striatum dashboard --once`, `striatum recovery stale-leases`,
     `striatum recovery requeue-stale`, `striatum recovery
     process-reconcile`, `striatum recovery cancel-job`, and
     `striatum checkpoint resolve`. Show the canonical recovery
     decision tree (lease expired → recovery; needs_revision with no
     cycle → checkpoint resolve or override-verdict per
     `docs/ROADMAP.md` §3.2 + §3.6).
   - **First-action sequence.** Numbered list, one step per SPEC
     "Required first-action sequence" bullet. Each step should have a
     concrete command or directive (e.g., step 2: "Run
     `git status --short --branch`; if dirty, decide preserve-or-stash
     before any state-changing work.").
   - **Reporting expectations.** OPERATOR_REPORT.md path comes from
     the fill-in block; cadence is "per intervention, not only at
     end" per memory `feedback_operator_report_incremental`.
   - **Daemon/Postgres caveat.** Per the SPEC and the triage call,
     reflect the current transition state without claiming the flip
     is done. Reference RFC 0048 as the longer-term direction.
   - **Where to look next.** A short pointer table mirroring
     `docs/ROADMAP.md` §11 shape, ending with: "If a rule conflicts
     between this prompt and `docs/ROADMAP.md` §3, `docs/ROADMAP.md`
     wins — this prompt is a frozen instance; ROADMAP is the live
     source."

2. **`prompts/README.md`** (edit).
   - Add `OPERATOR_INITIALIZATION_PROMPT.md` to the "Reusable
     Prompts" section above `OPERATOR_BOUNDARY_PROMPT.md`.
   - One-sentence explanation per prompt: "Use OPERATOR_INITIALIZATION
     when starting/resuming a run; use OPERATOR_BOUNDARY as a focused
     guardrail addendum or for sessions that have already been
     initialized."

3. **`prompts/OPERATOR_BOUNDARY_PROMPT.md`** — touch only if triage
   said refactor. Default is no-touch.

4. **`docs/issues/16/HANDOFF.md`** — schema_version
   `striatum.handoff.v1`. **Cite each acceptance check from
   `SCOPE.md` § "Acceptance checklist"** with the file:line that closes
   it. Format:

   ```
   - [DoD-1] prompts/OPERATOR_INITIALIZATION_PROMPT.md:1 — Status: reusable in title block.
   - [DoD-2] prompts/OPERATOR_INITIALIZATION_PROMPT.md:14-120 — Fill-in block + operating rules + recovery + first-action sequence sections present.
   - [RS-1] prompts/OPERATOR_INITIALIZATION_PROMPT.md:20 — `Striatum repo path: <fill>` in fill-in block.
   - ...
   ```

   If any check is intentionally deferred, mark it `[deferred:
   <reason>]` and explain in HANDOFF prose; the verify job will
   refuse if a Definition-of-done bullet is deferred without strong
   justification.

## Constraints

- **STRICTLY GENERIC.** No `RFC0026`, `RFC0027`, no `Engram` paths, no
  `~/git/engram/`-specific examples, no Engram dogfood ordinals. Use
  placeholders or repo-relative paths. The SPEC and `AGENTS.md` both
  enforce this.
- **Write scope:** `prompts/` + `docs/issues/16/`. Do NOT touch `src/`,
  `tests/`, `docs/dogfood/`, `docs/rfcs/`. The packet's `forbidden_paths`
  will refuse you.
- **No `.striatum/` writes ever.**
- **Author byline:** use the byline the work packet supplies verbatim
  in any artifact you write (`HANDOFF.md` author line). Don't invent.
- **Operator decision rules: REFERENCE, don't duplicate.** The new
  prompt should say "see `docs/ROADMAP.md` §3" rather than re-encode
  the operator-on-behalf path, the fix-up dogfood pattern, or the
  anti-pattern catalog. Duplication will rot.

## Commands you'll need (verbatim from the work packet)

```
ack:              striatum ack --session-id <S> --message-id <M> --lease-id <L>
publish_artifact: striatum publish-artifact --session-id <S> --job-id <J> --lease-id <L> --kind handoff --logical-name handoff --path docs/issues/16/HANDOFF.md
complete:         striatum complete --session-id <S> --job-id <J> --lease-id <L>
heartbeat:        striatum heartbeat --session-id <S> --lease-id <L>     # call before long-running edits
```
