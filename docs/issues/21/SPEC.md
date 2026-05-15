# GH #21 — striatum serve restart mid-run loses repo-local state — 3rd occurrence in one session

Source: https://github.com/halbritt/striatum/issues/21

## Summary

Restarting \`striatum serve\` while a run is active loses the entire active-run state from \`.striatum/state.sqlite3\`. Reproduced 3 times in a single 8-hour operator session today (2026-05-14 → 2026-05-15).

The mechanism appears to be: serve startup runs \`striatum init\` (or equivalent) which creates a fresh state.sqlite3, OR opens the file with a write mode that truncates / clobbers concurrent supervisor writes, OR a race between serve's first-write and the running supervisor processes' final-commit. End result: state.sqlite3 shrinks from MB-scale (active run + history) to KB-scale (history only or pure-fresh init).

## Repro

1. Start a multi-job dogfood with \`striatum run prepare\` + \`striatum run start\`.
2. Register sessions, start supervisors, claim packets. Run progresses; state.sqlite3 grows to MB-scale.
3. Restart \`striatum serve --web --allow-mutations\` (e.g. operator sees a UI 500, kill -TERM the serve PID, re-launch).
4. After restart, \`striatum dashboard --run-id <active-run>\` returns \`unknown run_id\`. \`sqlite3 state.sqlite3 'SELECT count(*) FROM runs'\` returns the pre-active-run count.
5. The supervisor processes are still alive (\`ps -ef | grep -E 'codex|claude|gemini.*wrapper'\`) but cannot make progress; any callback they attempt fails because their session/lease IDs aren't in the DB.

## Observed instances (this session)

1. dogfood-057 mid-run: bounced serve to debug an unrelated UI issue → run_f6c7076a63484f9daedd9ef3f0850130 disappeared; eventually committed the on-disk artifacts and pushed; state corruption surfaced separately.
2. Operator-UI restart during dogfood-058: state.sqlite3 went from healthy to integrity_check failure ("database disk image is malformed"). Forced full reset (quarantine to \`.corrupt\`, \`striatum init\` fresh DB).
3. dogfood-060 mid-build-review-phase: operator restarted serve because UI was returning 500. run_0d07b5cc8ad0450d8c1f830ef828f0c1 vanished; state.sqlite3 shrank from ~8MB to 385KB; only dogfood-058's old completed run row survived.

## Cost

Each occurrence costs the active dogfood iteration: ~30-60 minutes of agent work lost (designs, synth, review_design, implement). Across the session: 3 dogfoods aborted at various stages = several hours of agent time + tokens.

## Required fixes

1. **Serve startup must NOT call \`init\` if state.sqlite3 already exists and is healthy.** Read-only attach to existing DB; error out (not init-over) if the file is unreadable.
2. **Serve must use SQLite WAL mode + acquire only a SHARED lock for reads.** Any exclusive lock should be held only for serve-side writes (which only happen on the mutation surface), and even then must use \`BEGIN IMMEDIATE\` to coexist with concurrent supervisor writes.
3. **Document the serve-restart hazard** in HOW_TO_HUMAN.md and OPERATOR_INITIALIZATION_PROMPT.md — \"never restart serve during an active run; if the UI is unhealthy, diagnose without bouncing the process.\"
4. **\`striatum serve --read-only\`** flag — serve becomes a pure-read process that never opens the DB in write mode. Operator UI doesn't strictly need write authority (mutations go through MCP/RPC); a read-only mode would close the hazard entirely.
5. **Process supervision discipline**: when a serve process dies, the runner should detect the orphaned supervisor processes (\`process_supervisors\` table) and either reattach or mark them \`abandoned\` cleanly, instead of leaving them as orphan ghosts.

## Severity

HIGH — same as #19 (stale-lease recovery). Together these two issues are the dominant operator-friction pattern: any serve hiccup or UI restart kills an active dogfood. Each lost dogfood is 1-3 hours of agent work.

## Provenance

\`docs/dogfood/FRICTION_LOG.md\` will get a dogfood-060 F2 entry referencing this issue.
