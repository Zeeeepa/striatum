# Design-Run Seed — RFC 0168 P0 (REVISION v5)

> This is the **v5 revision** of the RFC 0168 P0 design run, and the gate is
> nearly clear. v1 proved the hard core; v2 credited the C1/C2 structural halves;
> v3 **discharged C1-RESIDUAL** (fail-closed scrub classifier); v4 **discharged
> the C2 bearer-path sub-part** (moved `writeEphemeralMCPConfig` under
> `.striatum/scratch/<supervisor_id>/`, real A22, corrected citation). The v4
> cycle-1 adjudicator left **one** finding `open` — the **provider/credential-cache
> ACL ancestry** sub-part of OQ4 — and exhausted its cycle, routing it to the
> operator. This run discharges that **single** residual; everything else carries
> forward, unregressed. A **surgical one-point revision, not a rewrite.**
>
> **Required context docs** (read in full first):
> - `docs/operator/workflows/rfc-0168-design-v5/context/v4_HOLDER.md` — the **v4
>   SPEC you are revising** (the base; keep the discharged bearer-path fix verbatim).
> - `docs/operator/workflows/rfc-0168-design-v5/context/v4_LEDGER_cycle_1.md` — the
>   v4 verdict; its OPEN provider/credential-cache ancestry finding +
>   `closest_acceptable_answer` is the exact prescribed fix.
> - `docs/rfcs/0168-per-lane-security-principal.md` — the RFC (direction ratified D261).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. It must **discharge the provider/credential-cache
ancestry residual** and **carry forward, unregressed, everything v1–v4 cleared**
(including the v4-discharged bearer-path fix and the v3-discharged C1-RESIDUAL).

## The single residual to DISCHARGE — provider/credential-cache ACL ancestry (OQ4, verdict-driving)

The v4 "explicit forbidden provider set" assumed every forbidden path is a
**sibling** of a source top-level. But a configured credential cache can **nest
UNDER** an allowlisted source top-level. Verified chain: `command_env`
`CLAUDE_CONFIG_DIR` is allowed (`validateLaneCommandEnvKey` bars only
`PATH`/`STRIATUM_*`, `supervision_lane_config.go:440-451`), merged into the lane
env (`supervision_env.go:110-113`), survives the run-as filter
(`sensitiveRunAsEnvKey` drops only TOKEN/SECRET/PASSWORD/PASSWD/API_KEY/CREDENTIAL/DSN
substrings — `CLAUDE_CONFIG_DIR` matches none, `supervision_env.go:303-318`), and
`resolver.go:78-85` writes `.credentials.json` inside it. So
`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` nests the cache **under**
allowlisted `docs/`, and the OQ4.1(a) mandatory form (top-level prune + recursive
`setfacl -R` on each remaining source entry) **sweeps a `g:striatum-lanes`
access+default ACL onto that credential cache**.

The revised SPEC must make the forbidden set **ANCESTRY-AWARE** via ONE coherent,
enforced mechanism (pick and fully specify):
- **(a)** forbid provider credential/cache directories from resolving inside the
  repository (or beneath any group-ACL-allowlisted source top-level) at all; OR
- **(b)** extend the ACL **planner AND A23** to reject any recursive
  `g:striatum-lanes` op whose target is **equal-to, a descendant-of, OR an
  ANCESTOR-of** any configured credential-cache path — the **same** ancestor
  semantics A23 already applies to `.striatum/`; OR
- **(c)** walk allowlisted source trees with **exclude-at-traversal** that
  **prunes** forbidden descendants before applying any grant.

AND add a concrete test (extend A22/A23 or a new A25) that sets
`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`, provisions ACLs, and
asserts **no `g:striatum-lanes` access OR default ACL** on that directory or its
`.credentials.json`, **before / during / after**. Update the OQ4 invariant text
to state the forbidden set is ancestry-enforced.

## Carried forward — CLEARED in v1–v4 (do NOT reopen, do NOT regress)

- **C2 bearer-path sub-part — DISCHARGED (keep verbatim):** `writeEphemeralMCPConfig`
  moved under `.striatum/scratch/<supervisor_id>/` (threading `STRIATUM_SUPERVISOR_ID`)
  before re-keying ACLs (`mcpconfig.go:550-565`), real A22 (live-derived path, no
  residual root-level bearer), launch-consistent ACL (no #279 EACCES).
- **C1-RESIDUAL — DISCHARGED:** fail-closed `classifyPoolUIDTaskState` P1 (NOT
  `processZombie`) + `/proc` evidence + A21.
- **Hard core HC-A1..A5**; the **C1 four-state lease machine** (+ `uq_lane_uid_held`
  + 3-tx boundary + reaper + dirty-excluding exhaustion); the **C2 procedure fix +
  A23** (mandatory allowlist; raw-recursive-root prohibition) and the
  `.striatum/`/`.git/`-excluding **GROUP-ACL end-state invariant**; **OQ1/OQ3/OQ5/OQ6**;
  the **narrowing invariant**.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (provider/credential-cache ancestry):** Can a credential cache
  nested under an allowlisted source top-level (`CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude`)
  still receive a `g:striatum-lanes` ACL, before/during/after? Probe other nest
  sites, symlinked caches, caches created after provisioning. Is the mechanism
  enforced (planner/A23/traversal), not prose-only? Is the new test real?
- **Falsifier 2 (no-regression):** Did the ancestry fix regress the bearer-path
  discharge, C1-RESIDUAL, the C2 procedure guard (A23 + GROUP-ACL invariant), the
  hard core, the lease machine, OQ1/3/5/6, or the narrowing invariant?

The adjudicator gates on whether the **provider-ancestry residual is genuinely
discharged** (ancestry-aware + enforced + a real nested-cache test) and whether
the **bearer-path + C1-RESIDUAL stayed discharged** and **no carry-forward
regressed**, with no standing material challenge. A clearing verdict (`accept` /
`accept_with_findings`) requires all of these. This is the single allowed v5
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator. On a clearing verdict the operator ratifies **D272** (D271 is
reserved by the concurrent RFC 0170 P0 design). Build targets runtime migration
**0046+** (0045 is RFC 0170's) + owner-bundle bump **owner/0023+** for
`striatumd.lane_uid_leases`. Keep the local-first boundary.
