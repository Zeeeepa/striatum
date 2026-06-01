# Operator Report — RFC 0097 self-hosting proof (acceptance #5)

Date: 2026-06-01
Run: `run_d8e26daeb71523486614723789e8037d` (branch `striatum/rfc-0097-self-hosting-proof-v2`)
Outcome: **SUCCESS — a minimal dogfood was driven end-to-end through the live
runner with no operator intervention in the work itself. RFC 0097 self-hosting
acceptance #5 proven live (the chaos suite already proved it at the daemon level).**

## What this dogfood was

The smallest viable self-hosting proof: a single-claude-lane `document_only`
workflow (`dogfoods/rfc-0097-self-hosting/workflow.json`) with one `draft` job
that writes one tiny markdown artifact. Goal: show the runner can carry its own
development — orchestrate a job through the production RPC handlers, lane
bootstrap → claim → publish → complete, verified by returned JSON and the
on-disk artifact (not by terminal panes).

## What worked (proven live, JSON-verified)

- **Full loop through production handlers, hands-off.** `run prepare` → `branch
  confirm --create` → `run start` → `register-session` → `supervise start` →
  the lane self-drove `work.await_packet` → claim → write file → `artifact.publish`
  → `work.complete`, then the **run auto-finalized to terminal `completed`** (1/1
  jobs completed; `run cancel` refused because the run was already `completed`).
  No operator action touched the work — only setup and observation.
- **Per-session capability token minted** at `register-session` (RFC 0096 V2 #135
  live), lane `attested`, `lane_backend: tmux`, `agent_loop_mode: self_driving`.
- **#101 unblocker confirmed live.** The supervised claude lane bootstrapped
  straight past the update/welcome screen that wedged the prior dogfood — the
  pane showed it working in accept-edits mode, never parking on the update nag.
- **Artifact integrity.** Daemon record `art_beeed55f9d5601b47ac2e107272df9c1`
  (`kind: handoff`, byline `author: author-claude-opus-4.8-001`) carries
  `content_sha256 = 1262fd27…dbdb3`; the file on disk hashes **identically** —
  the published artifact is exactly the bytes written. Job timing: started
  22:26:26Z, completed 22:26:59Z (~33s of real work once unblocked).

## The finding (fixed mid-run)

The first run (`run_6c65e6e4f9ff72f2b72c6c5afc3e8767`) wedged: the lane was
configured with a **bare `["claude"]`** command, so the self-driving agent loop
parked on a **per-tool MCP permission prompt** ("Striatum daemon RPC method
work.await_packet — Do you want to proceed?") and never claimed its packet. The
canonical lanes already bypass permissions in the lane command
(`code-change-flow` uses codex `--yolo`); the claude equivalent is
`--dangerously-skip-permissions`. The daemon's `write_scope` guard remains the
real boundary, so bypassing claude's own interactive prompt for a sandboxed,
operator-launched, self-driving lane is correct. Because a running run is pinned
to its **frozen workflow snapshot**, the live run could not be edited — the fix
required tearing down and preparing a **fresh** run off the corrected workflow
(commit `792852fd`). The second run completed cleanly.

## Lesson for scaffolding claude agent_loop lanes

A bare-interactive claude lane (`["claude"]`) is **not** sufficient for a
self-driving supervised lane — it parks on MCP tool-permission prompts. Use
`["claude", "--dangerously-skip-permissions"]` (the `--yolo` analog). The older
scaffold note listing `["claude", "--print"]` is doubly wrong: `--print` is
retired and a bare command still prompts. This is a sibling of #101 (lanes
stalling at a screen) — a different screen (permission dialog), same failure
mode.

## Teardown

Supervisor stopped (tmux session killed), session `closed`, run terminal
`completed`. No lingering tmux sessions or supervisors. The produced artifact is
committed to `main` as durable provenance alongside this report.
