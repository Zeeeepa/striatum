# Design-Run Seed — RFC 0168 P0 (REVISION v6)

> This is the **v6 revision** of the RFC 0168 P0 design run, and the gate is one
> definitive fix from clear. v3 discharged C1-RESIDUAL; v4 discharged the C2
> bearer-path sub-part; v5 implemented the prescribed **ancestry** mechanism and
> closed the `CLAUDE_CONFIG_DIR` case. The v5 cycle-1 adjudicator left **one**
> finding `open` — the credential-cache **COMPLETENESS / FAIL-CLOSED** sub-part —
> and routed it to the operator. This run discharges that **single** residual with
> a class-closing fix; everything else carries forward, unregressed. A **surgical
> one-point revision, not a rewrite.**
>
> **Required context docs** (read in full first):
> - `docs/operator/workflows/rfc-0168-design-v6/context/v5_HOLDER.md` — the **v5
>   SPEC you are revising** (the base; keep the discharged ancestry + bearer-path
>   fixes verbatim).
> - `docs/operator/workflows/rfc-0168-design-v6/context/v5_LEDGER_cycle_1.md` — the
>   v5 verdict; its OPEN credential-cache completeness/fail-closed finding +
>   `closest_acceptable_answer` is the exact prescribed fix.
> - `docs/rfcs/0168-per-lane-security-principal.md` — the RFC (direction ratified D261).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0168 P0** the downstream `rfc-0168-build`
`code_change` run executes. It must **discharge the credential-cache
completeness/fail-closed residual** and **carry forward, unregressed, everything
v1–v5 cleared** (the v5 ancestry mechanism for modeled selectors, the v4
bearer-path fix, the v3 C1-RESIDUAL, and all prior carry-forwards).

## The single residual to DISCHARGE — credential-cache COMPLETENESS + FAIL-CLOSED (OQ4, verdict-driving)

The v5 ancestry mechanism is **allowlist-scoped** — the protected set = the
parents of the resolver's *modeled* credential paths. It is **not fail-closed**
against **unmodeled** provider credential selectors, and the live Claude runtime
reads one the resolver does not model today. Source-verified:
`CLAUDE_SECURESTORAGE_CONFIG_DIR` is admitted by `validateLaneCommandEnvKey` (bars
only empty/`PATH`/`STRIATUM_*`, `supervision_lane_config.go:440-451`), merged
(`supervision_env.go:110-118`), survives the run-as filter
(`sensitiveRunAsEnvKey`, `supervision_env.go:303-318`), and points the provider at
a credential dir the `resolver.go` roster (`resolver.go:78-85`) never models — so
it can nest a credential cache inside the repo and **escape the ancestry ban**.

The revised SPEC must make the protected credential-cache set **PROVIDER-COMPLETE
and FAIL-CLOSED**:
- **(b) primary (strongly preferred — covers present AND future/unmodeled
  selectors):** FAIL THE LANE LAUNCH CLOSED with a typed floor (e.g.
  `lane_credential_cache_inside_repo`, or a sibling typed refusal) whenever ANY
  provider credential-dir / secure-storage / config-dir env key present in
  `command_env` is **NOT covered by the resolver roster AND resolves inside the
  repository** (or beneath any group-ACL-allowlisted source top-level). Make this
  **generic** (coverage-gap / key-name-pattern based), not another hardcoded
  allowlist entry — so an unmodeled present-or-future selector cannot silently
  bypass.
- **(a) defense-in-depth:** extend the resolver roster to **model**
  `CLAUDE_SECURESTORAGE_CONFIG_DIR` (and any other config-dir/secure-storage env
  key the in-scope provider CLIs actually read) so OQ4.1.1's resolution-domain ban
  and the A25 test cover it explicitly.

Extend A25 (or add A26) to exercise the **REAL** unmodeled selector: set
`CLAUDE_SECURESTORAGE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/secure` and assert the
lane launch **FAILS CLOSED** with the typed floor (NOT a silent group-ACL grant),
before/during/after — plus a **positive control** that a modeled selector
resolving OUTSIDE the repo still launches. Update the OQ4 invariant to state the
credential-cache protection is provider-complete and fail-closed for **every**
credential selector the in-scope provider CLI reads, not only the modeled subset.

## Carried forward — CLEARED in v1–v5 (do NOT reopen, do NOT regress)

- **v5 ancestry mechanism (modeled selectors)** — three chokepoints + A25, the
  `CLAUDE_CONFIG_DIR=<repoRoot>/docs/.lane-auth/claude` case.
- **C2 bearer-path sub-part — DISCHARGED:** `writeEphemeralMCPConfig` re-rooted
  under `.striatum/scratch/<supervisor_id>/` (`mcpconfig.go:550-565`), real A22, no
  #279 EACCES.
- **C1-RESIDUAL — DISCHARGED:** fail-closed `classifyPoolUIDTaskState` P1 (NOT
  `processZombie`) + `/proc` evidence + A21.
- **Hard core HC-A1..A5**; the **C1 four-state lease machine**; the **C2 procedure
  fix + A23** + the `.striatum/`/`.git/`-excluding **GROUP-ACL end-state
  invariant**; **OQ1/OQ3/OQ5/OQ6**; the **narrowing invariant**.

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (completeness/fail-closed):** Can an UNMODELED credential
  selector (`CLAUDE_SECURESTORAGE_CONFIG_DIR`, or a third `*_CONFIG_DIR` /
  `*_CACHE_DIR` / secure-storage variant) resolving inside the repo still get a
  group ACL or silently bypass? Is the floor GENERIC (coverage-gap/key-pattern),
  not a hardcoded allowlist entry? Is the test real (unmodeled key → typed
  refusal)?
- **Falsifier 2 (no-regression + not over-broad):** Did the fail-closed floor
  regress the ancestry mechanism, bearer-path, C1-RESIDUAL, the C2 procedure
  guard, the hard core, the lease machine, OQ1/3/5/6, or the narrowing invariant?
  AND does a legitimate lane (no in-repo credential dir; modeled selector outside
  the repo) still launch (positive control)?

The adjudicator gates on whether the **credential-cache fail-closed residual is
genuinely discharged** (generic typed refusal for any uncovered in-repo
credential selector incl. `CLAUDE_SECURESTORAGE_CONFIG_DIR`, with a real test and
a positive control) and whether the **ancestry + bearer-path + C1-RESIDUAL stayed
discharged**, **no carry-forward regressed**, and **no legitimate lane is
over-broadly refused**, with no standing material challenge. A clearing verdict
(`accept` / `accept_with_findings`) requires all of these. This is the single
allowed v6 revision cycle; a second `needs_revision` ends the gate uncleared and
routes to the operator. On a clearing verdict the operator ratifies **D272** (D271
is reserved by the concurrent RFC 0170 P0 design). Build targets runtime migration
**0046+** (0045 is RFC 0170's) + owner-bundle bump **owner/0023+** for
`striatumd.lane_uid_leases`. Keep the local-first boundary.
