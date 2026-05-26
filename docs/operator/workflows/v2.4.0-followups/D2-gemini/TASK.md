# D2 task — Gemini fixes the Gemini problem (agent-loop reliability)

You are a Gemini lane. This task is for *you* to fix the reliability problem
Gemini lanes hit when driving the Striatum MCP agent-loop.

## The problem (observed 2026-05-25/26)

In live runs, the Gemini agent-loop lane:
1. **Wasted time exploring** the repo (ran `grep`/file searches) before and
   while holding a work packet, instead of going straight to MCP calls.
2. Used the **default short lease**, so by the time it reached
   `artifact.publish` / `work.ack` / `work.complete`, the 120s packet lease had
   **expired** and the mutations failed with opaque errors.

Net: Gemini could connect to MCP and write files, but could not dependably
complete a packet loop. claude and codex lanes do not have this problem.

## The fix (guidance the installed Gemini agent must follow)

Update the Gemini agent guidance so future Gemini lanes are reliable. Edit:
- `go/pkg/installers/templates/skills/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`
- `go/pkg/installers/templates/plugins/gemini/GEMINI.md.tmpl`

Add a clearly-marked **"Agent-loop reliability (Gemini)"** section to each,
stating these rules:
1. On `work.await_packet`, request a **long lease**: `lease_seconds: 900`
   (Gemini is slower; a short lease expires before you finish the loop).
2. **Do not grep or explore the repository** before claiming or while holding a
   packet. Read only the specific files the packet/objective names. Go straight
   to the MCP tool calls.
3. Pass `repository_id` and `session_id` on **every** Striatum MCP tool call,
   as an explicit JSON object argument.
4. Complete the loop promptly: `await_packet → do the work → artifact.publish
   (with kind) → work.ack → work.complete (session_id, job_id, lease_id)`.
   If a mutation reports a lease error, you were too slow — re-`await_packet`
   to refresh and retry without re-exploring.

Keep the wording consistent with the surrounding guide. Do not change unrelated
content. This is documentation/guidance only — no Go code change.

## Definition of done

- Both Gemini guide templates contain the new reliability section with the four
  rules above.
- A HANDOFF noting what changed and why.
