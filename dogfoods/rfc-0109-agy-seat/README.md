# Dogfood — RFC 0109 agy-seat (live-corroboration vehicle)

**Status: DRAFT scaffold — DO NOT LAUNCH yet.** This run is the vehicle for the
two `[live-corroborated]` acceptance legs of
`docs/rfcs/0109-agy-lane-first-class-seat.md`. It cannot pass until **P1 (#95
keystone)** lands — today the agy seat collapses after one turn (#95) and gates
at launch on a folder-trust prompt (#76/#139), so the agy reviewer would wedge on
the re-review. That wedge **is** the bug this RFC fixes; this dogfood proves the
fix, it does not work around it.

## What it proves (maps to the RFC's acceptance ledger)

- **[live-corroborated] 3-lane interrogating panel, agy holds its seat across a
  `needs_revision` cycle (the inverse of #139).** `present` (claude) is reviewed
  by `review_codex` (codex, neutral) **and** `review_agy` (agy, devils_advocate).
  The agy reviewer votes `needs_revision` on attempt 1, the presenter revises, and
  the **agy reviewer re-reviews on attempt 2 and accepts** — re-entering an
  **attested** session across the revision. That second agy turn surviving is the
  load-bearing proof.
- **[live-corroborated, restart]** inject `systemctl --user restart striatumd`
  mid-run (after `present` completes, while a review lease is held) and confirm the
  agy (and codex) lane resumes. This leg only passes once the **transport fix**
  (RFC 0109 §P1 cross-cutting; pairs with RFC 0103 W3/#141) lands — otherwise the
  config-file MCP port is stale after restart. Until then, tier the seat
  non-restart-robust and surface it (do not silently skip the leg).

## Launch preconditions (Phase B operator checklist)

1. **P1 #95 landed + deployed:** the agy submit-driver re-enters the same attested
   session across turns. Verify the daemon is running the fixed binary
   (`/proc/<MainPID>/exe` sha == freshly-built `striatumd`), not just `make install`.
2. **agy on PATH for daemon lanes:** symlink `agy` (and `codex`) into
   `~/.local/bin` — daemon-spawned lanes only get `~/.local/bin` on PATH.
3. **agy folder-trust pre-cleared (until #139 auto-trust lands):** add this repo's
   path to BOTH `~/.gemini/trustedFolders.json` and
   `~/.gemini/antigravity-cli/settings.json::trustedWorkspaces[]` (exact path —
   parent-dir trust does not transitively apply). `--dangerously-skip-permissions`
   does NOT bypass this today.
4. **Validate against the live daemon FIRST:** `striatum workflow validate` /
   `workflow.lint` this `workflow.json` — it is a DRAFT authored offline and has
   not been validated against the daemon (see memory
   `reference_dogfood_scaffold_gotchas`: document_only/inputs, review-lane model
   family, branch confirm, etc.). Expect to fix validate blockers before `run
   prepare`. The `degraded_seat_lane` warning (RFC 0109 P2) **will** fire on the
   agy lane — that is correct and expected; it is non-blocking.
5. **Worktree hygiene:** launch only when the target worktree is yours / coordinated
   (RFC 0108) — `main` is heavily concurrent.

## Shape

- `present` (synthesis, claude, interrogable) → `review_codex` (codex) +
  `review_agy` (agy); `cycle: review_agy → present on needs_revision`,
  `max_iterations: 1`. Independent model families across all three lanes.
- Modeled on the proven `dogfoods/rfc-0103-floor/` (single-claude floor that
  cleared the RFC 0103 umbrella, `run_fb10589f`). This extends it to the 3-lane
  panel + agy-seat survival the RFC 0103 floor deliberately could not run (codex
  and agy are not yet restart-robust seats).

See `~/.claude/plans/rfc0109-execution.md` (Phase B / B5) for the full driving plan.
