# RFC 0088 + 0089 two-phase closeout

Close out two RFCs in one correctly-sequenced run. The load-bearing constraint
is **verify before delete**: RFC 0088 deletes the only previously-attested
one-shot authoring path (`--print`/`exec` wrapper), the `single_shot`
capability, and the turn-driver. Those deletions are irreversible and must not
land until the replacement — the daemon-owned owned-PTY agent-loop — is proven
live. This workflow makes that proof a structural gate: Phase 2 (the deletions)
cannot start until all three Phase 1 reviews accept, and Phase 1 itself is
executed by a **codex owned-PTY agent-loop lane over tmux**, so a clean Phase 1
*is* the RFC 0088 codex P3 live-verify the prior session left open.

## Phase 1 — RFC 0089 follow-up findings (build, then 3-lane interrogating review)

Resolve the five accepted RFC 0089 follow-up findings as a coherent design, not
five disconnected patches:

1. Compact dashboard needs a delivery-degraded signal **distinct** from pane
   liveness and lane attestation.
2. Dashboard must distinguish tmux-backed lanes from plain-PTY fallback lanes.
3. There is no in-place delivery **rebridge/restart** command; degraded delivery
   currently forces stop/start or reclaim and loses session context.
4. The tmux probe must surface a **warning/degraded** state before a lane is
   marked lost on persistent `tmux_unavailable`.
5. Doctor/status projections need concrete **remediation hints** per terminal
   tmux failure class.

The intended shape (builder may refine, but justify deviations): a three-state
graded liveness model (healthy -> degraded -> lost) driven by a typed probe
failure record that captures exit code / errno / pane-pid-alive / failure class;
a single `rebridge` verb that re-attaches the tmux delivery path in place
without killing the pane (only valid while pane process liveness still holds);
dashboard fields that render delivery-state and lane-backend distinctly from
pane liveness and attestation; doctor/status hints derived from the captured
failure class rather than a generic menu.

## Phase 2 — RFC 0088 closeout (gated on Phase 1 acceptance)

Only after Phase 1 reviews accept:

- Delete the turn-driver, the `single_shot` adapter capability, and the
  `--print`/`exec` supervised wrapper, apoptosis-clean: update or remove every
  call site and export first; no dangling references; no test that passes while
  a real lane path is dead.
- Add a retired-vocabulary grep gate (`docs/reference/retired-vocabulary.txt`
  plus a build/test check) so `gemini_cli`, `single_shot`, `turn-driver`, and
  the retired `--print`/`exec` wrapper terms cannot reappear outside the
  `_archive/` tree.
- Draft decision-log entries **D148-D151** (per RFC 0088's proposed-decision
  section). Each entry must reference the concrete evidence from this run: the
  Phase 1 codex live-verify session id and the run id, so a future auditor can
  reconstruct that the agent-loop was proven before the wrapper was deleted.
- Land the SPEC, glossary (`ubiquitous-language.md`), and
  command-authority-matrix updates, and flip RFC 0088 status proposed -> accepted.

## Lanes

- Builder (both phases): Codex GPT-5.5 xhigh, owned-PTY agent-loop over tmux.
- Reviewers (both panels): Codex GPT-5.5 xhigh (threat_model), Claude Opus 4.8
  (ergonomics_dx), AGY display model Gemini 3.5 Flash High (devils_advocate).

The installed `agy` binary exposes no model flag; the workflow records the AGY
lane identity as Gemini 3.5 Flash High and relies on the operator's Antigravity
configuration for the actual provider-side model. If the local `claude` CLI
does not recognize `claude-opus-4-8`, the operator may downgrade that lane's
model before launch.

## Interrogation requirement

Each reviewer must interrogate the live builder before recording a verdict. A
zero-round or capability-denied interrogation is not acceptable: block instead
of publishing a review if interrogation cannot be opened.

## Privacy boundary

No raw tmux pane text, PTY log bytes, or terminal transcripts may enter daemon
state or durable artifacts. Interrogation chat logs are curated workflow
messages, not screen capture. Do not commit `.striatum/`.
