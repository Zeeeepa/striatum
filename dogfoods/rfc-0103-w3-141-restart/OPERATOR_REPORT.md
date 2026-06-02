# Operator Report — RFC 0103 W3 #141 daemon-restart survival (live)

**Date:** 2026-06-02 · **Operator:** claude-opus-4-8 · **Vehicle:** minimal
single-claude-lane repo-write dogfood, driven through the live daemon, with a
real `systemctl --user restart striatumd` injected mid-run.

## What this corroborates

The `[live-corroborated, PRIMARY for #141]` acceptance gate from
`docs/rfcs/0103-self-hosting-production-hardening.md` §W3: a real OS/systemd
restart that recreates the socket mid-run, after which the supervised lane
survives and the repo-write job completes through the production handlers — with
no escalation.

## Diagnosis (reproduce → instrument → fix)

A first live run (with only the `KillMode=process` unit change) showed the agent
lane + tmux survived the restart and the run **completed**, but the **helper
process itself died**. Instrumenting the surviving/dead PIDs and their cgroups
isolated two *independent* killers of the helper:

1. **systemd cgroup kill** — the helper sits in the `striatumd.service` cgroup;
   the default `KillMode=control-group` SIGKILLs that cgroup on restart. Fixed by
   `KillMode=process`. (The agent lane lives in tmux's own `tmux-spawn-*.scope`,
   so it already survived regardless.)
2. **the daemon's own `exec.CommandContext` cancellation** — the helper was
   spawned with the daemon-lifetime context, so the Go runtime SIGKILLed it when
   that context cancelled on shutdown. This is independent of systemd, so
   `KillMode=process` alone did not save the helper. Fixed by spawning the helper
   (and non-tmux pipe lanes) with `context.WithoutCancel`.

## Result

| Run | Fixes active | Daemon restart mid-run | Helper survived | Run state |
|-----|--------------|------------------------|-----------------|-----------|
| 1   | `KillMode=process` | yes | no (agent survived, completed anyway) | completed |
| 2   | `KillMode=process` + `WithoutCancel` | yes | **yes** | completed |

In run 2 the daemon re-bound the surviving helper as `tmux_ok` / `reattachable`
(not `helper_process_gone`), the agent-loop receiver re-dialed the recreated
socket, and the `write_restart_proof` job completed through the production
handlers. No escalation; no operator intervention.

## Notes

- The runner moved the job through daemon state transitions
  (`claimed → running → completed`), not terminal scraping.
- Artifact published to disk: `artifacts/RESTART_SURVIVAL_PROOF.md`.
- Operator-local PTY/scratch state under `.striatum/scratch/` is private
  diagnostics, not committed.
