# Design-Run Seed — RFC 0168 P0 (REVISION v4)

> This is the **v4 revision** of the RFC 0168 P0 design run. v1 proved the hard
> core (per-lane uid dissolves `BC1-W1-ORACLE`) + OQ1/OQ3/OQ5/OQ6; v2 credited the
> C1/C2 structural halves; v3 **DISCHARGED C1-RESIDUAL** (the fail-closed scrub
> postcondition: `classifyPoolUIDTaskState` blocks `T`/`t`/unknown/unreadable, not
> the binary `processZombie`) and credited the C2 *procedure* fix, but the v3
> cycle-2 adjudicator left **one standing residual** — **C2-RESIDUAL
> (`OQ4-ACL-PROVISIONING-TRANSITION`)** — `open`, and exhausted its revision
> cycle, routing it to the operator. This run discharges that **single** residual
> while carrying everything else forward, unregressed. A **narrow bearer-path
> fix, not a rewrite.**
>
> **Required context docs** (read in full first):
> - `docs/operator/workflows/rfc-0168-design-v4/context/v3_HOLDER.md` — the **v3
>   SPEC you are revising** (the base; only the C2 bearer-path sub-state changes).
> - `docs/operator/workflows/rfc-0168-design-v4/context/v3_LEDGER_cycle_2.md` — the
>   v3 verdict; its `OQ4-ACL-PROVISIONING-TRANSITION` finding + `closest_acceptable_answer`
>   is the exact prescribed fix.
> - `docs/rfcs/0168-per-lane-security-principal.md` — the RFC (direction ratified D261).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. It must **resolve C2-RESIDUAL** and **carry forward,
unregressed, everything v1/v2/v3 cleared** — including the already-discharged
C1-RESIDUAL. A revision that leaves C2-RESIDUAL open, or regresses a
carry-forward, has NOT cleared the gate.

## The single residual to DISCHARGE — C2-RESIDUAL (`OQ4-ACL-PROVISIONING-TRANSITION`, verdict-driving)

The v3 SPEC falsely asserted (marked `CONFIRMED`, citing `mcpconfig.go:241/266`)
that the `0600` MCP bearer already lives under `.striatum/scratch/<supervisor_id>/`.
**Source-false:** the live bearer is written by `writeEphemeralMCPConfig`
(`go/pkg/agentloop/mcpconfig.go:550-565` — `dir := filepath.Join(repoRoot,
".striatum","scratch")` at `:555`; `os.CreateTemp(dir, "lane-mcp-config-*.json")`
at `:559`; chmod `0600` at `:565`) **directly under `.striatum/scratch/` root**,
and it does **not** thread `STRIATUM_SUPERVISOR_ID`. (`:241/266` are the
gemini-cli `settings.json` write + backup-marker dir, not the bearer; only the
`pty.log` half — `loop.go:145`/`:300` — is source-true.) Two verdict-driving
consequences: **(1)** the v3 final ACL (`.striatum/scratch → u:<leased-uid>:--x`
traverse-only; `rwx` only on `.striatum/scratch/<supervisor_id>`) **breaks lane
launch** — `scratch_acl.go:31-48` grants `rwx`+default on `.striatum/scratch`
*itself* precisely because the writer `CreateTemp`s there
(`supervision_control.go:114-126`, issue **#279**), so a faithful `--x`-only-root
build makes `os.CreateTemp` fail `EACCES`; **(2)** the **A22** transition test is
**fake** — it plants a fixture at `.striatum/scratch/<S1>/lane-mcp-config-*.json`
while the real bearer is at `.striatum/scratch/lane-mcp-config-*.json`.

The revised SPEC must (four source-anchored parts):
1. Add, as a **required P0 build step**, moving `writeEphemeralMCPConfig`
   (`mcpconfig.go:550-565`) from `<repoRoot>/.striatum/scratch` to
   `<repoRoot>/.striatum/scratch/<supervisor_id>/` — threading
   `STRIATUM_SUPERVISOR_ID` exactly as `loop.go:145` and the gemini markers do —
   **before** re-keying the scratch ACLs, so the `--x`-only-scratch-root /
   `rwx`-on-supervisor-subdir final state is achievable **without** the
   `EACCES`/#279 launch break.
2. Make **A22** `TestPoolACLProvisioningNeverTransientlyExposesScratch` **derive
   and exercise the exact path `writeEphemeralMCPConfig` resolves** (not a
   hand-planted fixture) and assert **no residual root-level
   `.striatum/scratch/lane-mcp-config-*` bearer** after the transition.
3. Make the provider/token-cache **forbidden top-level set explicit**
   (`.gemini`, `.claude`, `.codex`, configured credential caches) in the OQ4
   allowlist, so a re-provision cannot sweep a provider auth path into the source
   allowlist.
4. **Correct the false source citation** — the bearer is `mcpconfig.go:550-559`
   (`writeEphemeralMCPConfig`), not `:241/266`.

Keep **A23** (`TestACLPlannerRejectsRawRecursiveRootWhileScratchExists`) — the
pure planner guard; necessary but not sufficient.

## Carried forward — CLEARED in v1/v2/v3 (do NOT reopen, do NOT regress)

- **Hard core HC-A1..A5 — PROVEN.** Per-uid `0700` tmux socket; cross-uid `0600`
  DAC (HC-A2, the surface that makes RFC 0143 Slice B's reseal token safe); cross-uid
  `ptrace`/`setns`/`/proc` denial; `SO_PEERCRED`; residual-surface closure.
- **C1 four-state lease machine** + `uq_lane_uid_held` + 3-tx boundary + reaper +
  quarantine-survives-restart + dirty-excluding exhaustion.
- **C1-RESIDUAL — DISCHARGED (preserve verbatim):** the fail-closed
  `classifyPoolUIDTaskState` P1 (NOT `processZombie`) + `/proc` evidence in
  `scrub_proof` + A21 `TestStoppedOrTracedUIDProcessBlocksReturn`.
- **The C2 procedure fix + A23** (mandatory allowlist; forbidden raw-recursive-root)
  and **the C2 final `.striatum/`-excluding GROUP-ACL end-state invariant**.
- **OQ1 / OQ3 / OQ5 / OQ6** and the **narrowing invariant** (no admin-token
  widening, no lane-readable shared reseal bearer; surface only ever removed).

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C2 bearer-path realness):** Is `writeEphemeralMCPConfig` really
  re-rooted under `.striatum/scratch/<supervisor_id>/` and threading the
  supervisor id? Is the migrated writer consistent with the final ACL so
  `os.CreateTemp` does not `EACCES` (#279)? Does A22 derive the path from the
  live writer (not a fixture) and assert no residual root-level bearer? Can a
  provider auth path enter the source allowlist?
- **Falsifier 2 (no-regression):** Did the bearer-path fix regress C1 (P1 still
  `classifyPoolUIDTaskState`?), the C2 procedure guard (A23 + GROUP-ACL
  invariant), the hard core, the lease machine, OQ1/3/5/6, or the narrowing
  invariant?

The adjudicator gates on whether **C2-RESIDUAL is genuinely discharged** (writer
move stated as a required build step anchored to `mcpconfig.go:550-559`, citation
corrected, final ACL launch-consistent, A22 real, forbidden set explicit) and
whether **C1-RESIDUAL stayed discharged** and **no carry-forward regressed**, with
no standing material challenge. A clearing verdict (`accept` /
`accept_with_findings`) requires all of these. This is the single allowed v4
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator. On a clearing verdict the operator ratifies **D272** (D271 is
reserved by the concurrent RFC 0170 P0 design). Build targets runtime migration
**0046+** (0045 is RFC 0170's) + owner-bundle bump **owner/0023+** for
`striatumd.lane_uid_leases`. Keep the local-first boundary (one host, one
PostgreSQL, one daemon as single writer; no hosted services).
