# Finding: interrogation attestation vs. agent-loop (2026-05-25)

A blocking architectural gap surfaced while running the iterated-interrogating-
panel live (run_f9753a8c…), directly relevant to "deprecate `--print`, run all
workflows on the MCP agent-loop."

## What happened

The design loop ran genuinely with three real models on the MCP agent-loop:
- `design_codex`, `design_claude`, `design_gemini` each produced an independent
  design proposal (artifacts/design/*/DESIGN.md), all three jobs completed.
- `synth` (claude, `interrogable: true`) reconciled them into DESIGN_SYNTHESIS.md,
  completed, and stayed live (`active`) in its post-complete `await_packet` loop,
  ready to answer interrogation questions.

Then `interrogation.open` against the live synth session failed:

```
target session is not attested (no attached supervisor);
interrogation requires a live, attested session
```

## Root cause

`requireLiveTarget` (`go/pkg/mutations/interrogation.go:387`) enforces RFC 0082's
rule that an interrogation target must be **`active` AND `attested`** (D026).
Attestation = an attached supervisor with a pid-bound `process_supervisors` row,
created by `supervise start` — the **supervised-wrapper** mechanism (D080).

But the wrapper spawns a fresh `claude --print` per packet with **no preserved
context** (see SUBSTRATE_VALIDATION.md). The only lanes that preserve the
context interrogation is meant to query are the **headless agent-loop** lanes —
and those are **not attested** (`lane_attestation: unattested`,
`no_attached_supervisor`). So:

> The sessions that *can* be interrogated (attested, via the wrapper) have no
> preserved reasoning to interrogate; the sessions that *have* preserved
> reasoning (agent-loop) cannot be attested, so they cannot be interrogated.

This is a direct contradiction between RFC 0082 (interrogation needs the
agent-loop's preserved context) and D026/D080 (attestation bound to the
supervised wrapper). It blocks genuine model-to-model interrogation on the
agent-loop today, and it is squarely in the path of the RFC 0083 / D140
`--print` deprecation.

## Options (for a follow-up RFC)

1. **Attest agent-loop sessions.** Let an agent-loop launch register a
   pid-bound attestation for its session (the agent-loop already owns the PTY and
   the session identity), so `requireLiveTarget` is satisfied without the
   `--print` wrapper. Most aligned with the deprecation goal.
2. **Relax the interrogation attestation requirement** for agent-loop sessions
   whose liveness is provable by an active lease + recent MCP heartbeat, gating
   on a new provenance mode rather than supervisor pid binding.
3. **Keep attestation wrapper-bound** and accept that interrogation requires the
   wrapper — i.e. do NOT deprecate `--print` for interrogable runs. (Rejects the
   stated goal.)

## State of the run

Design loop complete (4/4 through synth). `review_design_codex` and
`review_design_claude` are claimable but their interrogation step is blocked by
the gap above. The persisted interrogation thread (the "view the dialog as chat"
payoff) cannot be produced on agent-loop targets until one of the options lands.
