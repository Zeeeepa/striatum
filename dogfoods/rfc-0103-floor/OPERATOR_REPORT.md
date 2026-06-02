# Operator Report — RFC 0103 umbrella FLOOR (live)

**Date:** 2026-06-02 · **Operator:** claude-opus-4-8 · **Run:**
`run_fb10589f6c6633dd110ac26e593ad7d7` · **Branch:** `striatum/rfc-0103-floor`.

## What this clears

The RFC 0103 **umbrella FLOOR** (`docs/rfcs/0103-self-hosting-production-hardening.md`
§ "Acceptance (behavioral)"): a multi-lane, review-gated dogfood driven through the
runner — two distinct sessions of the supported shape, one bounded `needs_revision`
cycle with a live interrogation, surviving one injected lane/daemon fault — that
completes end-to-end through the production handlers, hands-off, and lands an
artifact. (Codex is a supported seat but reads a static `config.toml` MCP endpoint
that the random per-restart port invalidates, so it cannot survive the W3 restart
leg; two claude sessions are the restart-robust, floor-compliant "two instances of
the one supported shape" the RFC permits until W2 lands.)

## Timeline (all through production handlers, hands-off)

1. **present (att 1)** — claude presenter authored `artifacts/FLOOR_SYNTHESIS.md`
   (no Limitations section), published, completed.
2. **review (att 1)** — claude reviewer opened a live interrogation against the
   presenter (`turn_count=2`, asked + answered), then voted **needs_revision** with
   one concrete finding (add `## Limitations`). The cycle **routed**
   (`revision.cycle_routed`); the prior presenter session closed
   (`revision_reopened`); `present` re-opened to **att 2**.
3. **present (att 2)** — a fresh presenter claimed the revision. **Mid-revision the
   W3 fault was injected: a real `systemctl restart striatumd`.** Both supervisor
   helpers (presenter + reviewer) **survived** the restart (the #141 fix:
   `KillMode=process` + `context.WithoutCancel`); no escalation. The presenter
   then added `## Limitations`, re-published, and completed att 2.
4. **recovery** — the restart stalled the original reviewer's att-2 claim; the
   production recovery sweep closed it (`recovery_stalled_transfer`) and requeued
   review att 2 for a fresh reviewer — **no operator escalation**.
5. **review (att 2)** — a fresh reviewer opened a **second** live interrogation
   against the att-2 presenter, then voted **accept**.
6. **run.completed** at 19:33:38 — `FLOOR_SYNTHESIS.md` (with Limitations) landed.

## Coverage in one run

| Floor requirement | Evidence |
|---|---|
| two distinct sessions of the supported shape | presenter + reviewer claude sessions (fresh per attempt) |
| review-gated | `present → review` edge; verdicts recorded through the daemon |
| one bounded `needs_revision` cycle | att 1 `needs_revision` → cycle routed → att 2 `accept` (max_iterations 1) |
| live interrogation | two interrogations (att 1: 2 turns; att 2: post-restart) |
| survives one injected daemon fault | `systemctl restart` mid-revision; both helpers survived; recovery requeued the stalled reviewer; no escalation |
| completes end-to-end, hands-off | `run.completed`; artifact landed |

## Notes

- W4 (#131) interrogation-window survival and W3 (#125 ack, #141 restart) are the
  fixes this floor exercises live; each also has its own hermetic/live gate.
- This is the **floor**, not the production-grade ceiling — that requires the same
  dogfood across a fault-class matrix (W1 isolation / W3 churn / W4 reviewer
  replacement) across both seats.
- Operator-local PTY/scratch under `.striatum/scratch/` is private diagnostics,
  not committed.
