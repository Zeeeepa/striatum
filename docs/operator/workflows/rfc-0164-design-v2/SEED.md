# Design-Run Seed — RFC 0164 P0 (REVISION v2)

> This is the **v2 revision** of the RFC 0164 P0 design run (untrusted-substrate
> read-side git hardening). v1 was a strong, source-anchored SPEC, but its
> single adjudication cycle returned **`needs_revision`** on **two standing,
> gate-critical falsifier challenges that were never rebutted** (the holder had
> no further turn). This run discharges both while carrying forward, unregressed,
> everything v1 cleared. **Required context docs** (read in full first):
> - `docs/operator/artifacts/rfc-0164-design/dialogue/holder/HOLDER.md` — the v1 SPEC you are revising (the base; do NOT rewrite from scratch).
> - `docs/operator/artifacts/rfc-0164-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — the v1 verdict; its `constraints:` C1/C2 + each `verification.gate` are the exact prescribed fixes.
> - `docs/operator/artifacts/rfc-0164-design/dialogue/falsifier_1/FALSIFIER.md` and `.../falsifier_2/FALSIFIER.md` — the two landed-unrebutted challenges.
> - `docs/rfcs/0164-untrusted-substrate-read-side-git-hardening.md` — the RFC (settled posture).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **revised falsifiable
implementation spec for RFC 0164 P0** the downstream `rfc-0164-build`
`code_change` run executes. It must **discharge C1 and C2** and **carry forward,
unregressed, everything cleared in v1**. A revision that leaves C1 or C2 open —
or regresses a carry-forward — has NOT cleared the gate.

## Carried forward — CLEARED in v1 (do NOT reopen, do NOT regress)

- **Layered-severance posture** (RFC Decision): Layer 1 config-from-objects
  severance, Layer 2 born-neutralized env + bounded subcommands, Layer 3 evidence
  + two-state recovery; denylist demoted to non-load-bearing telemetry. Settled.
- **Omission IS neutralization** (A7/A16): the `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`
  family beats `-c`, so omitting it from the born-closed env is the neutralization.
- **Env floor REFUSES, never degrades** (A8): typed `ErrGitEnvUnavailable`, no bare
  fallback to an ambient environment.
- **Slice-0 no-truncated-graph** + the alternates/refs/benign-key **parity harness**
  named as the binding Slice-2 gate (A21/A21b).
- **Evidence-contract mechanics** (A22 canonicalization golden vectors before
  hashing with adversarial cases; A25 no-attestation-before-exec hard doctor
  barrier; A23 decay-TOCTOU closed by exec re-attest).
- **The four §0 source corrections** C-1..C-4 (status.go mislabel; surface
  undercount; env-incomplete commit path; the `os.Environ()` false-negative that
  turns the invariant from a nil-check into an `os.Environ()`-ban). Treat as part
  of the hardened claim, not errors to re-find.

## The two binding constraints to DISCHARGE

### C1 — severance is NOT complete (gate-critical)

The v1 "exhaustive" C-2 surface **omits the `runGitWorktreeCommand`/`integrateGit`
mutation funnel**, which runs gadget-bearing git under the **daemon identity**
against attacker-controlled repos/worktrees (`status`→`core.fsmonitor`,
`add`→`filter.clean`, `commit`→`core.hooksPath`, plus
`for-each-ref`/`rev-parse`/`merge-base`/`rev-list`/`show`) — including a **live
`recovery_quarantine_lane.go:425` `status`→`fsmonitor` RCE** the v1 SPEC implies
is closed but is not. By the holder's own §4/A12 the SPEC cannot both ship a green
invariant within the §11 manifest and leave the funnel unrouted.

The revised SPEC must:
- Replace the C-2 site-list with a **complete git-surface TAXONOMY** enumerating
  every funnel (`localGit.output`, `readGitOutput`, `runGitWorktreeCommand`,
  `integrateGit`, `runGitWithEnv`, direct helper-package calls), **classify** each
  route (read-only / ref-plumbing / index-or-worktree mutation / lane-side /
  CLI-only), **route** every daemon-run route through the closed env, and route
  every in-repo-config-sensitive route (`status`/`add`/`commit`/`write-tree`/`diff`
  + textconv/filter/hook/fsmonitor carriers) through a **minted config** OR a
  **typed pre-exec refusal**.
- **Reconcile §4/A12 honestly:** the compile-time invariant MUST inspect helper
  call sites; the P0 §11 manifest + green-build claim must be made TRUE, or A2/A12
  retracted to a scoped claim with the residual stated honestly (like §1).
- **Explicitly close `recovery_quarantine_lane.go:425`.**
- Add the three corpus rows `quarantine_status_fsmonitor`,
  `porter_add_filter_clean`, `porter_commit_hookspath` (red-before / green-after).

### C2 — a false-positive WEDGES legitimate work (gate-critical)

A benign-but-retained `[alias]`/`[pager]` is flagged as a recognized gadget and
written as a **machine-clearable blocker whose only clear condition is fingerprint
decay** — which never fires on stable benign config, so the run is **wedged until a
human edits benign config**. The SPEC also self-contradicts: it calls the record a
`blocker` (A23 relies on blocker semantics) while claiming observability-only.

The revised SPEC must:
- Guarantee a benign `[alias]`/`[pager]` **never wedges**: either (a) split
  recognized detections into a **non-blocking `gate.read_gadget_observed`** state
  for neutralized/benign-safe keys and reserve a blocking state for conditions that
  genuinely require recovery, OR (b) define a **precise benign safe-list/parser**
  for `[alias]`/`[pager]` where safe benign keys create no blocking pin — and state
  which. Resolve the blocker-vs-observability contradiction (one coherent state
  model).
- Keep **unknown/unattested keys** on the hard-refuse-into-human-cleared path (no
  silent unknown-pass).
- Make **`false_positive_benign_test` load-bearing**: plant the benign config, run
  an allowlisted read, assert **no** job/run blocker, **no** `recovery.quarantine_lane`
  ref, and that a **second** read proceeds with no repo edit and no human clear;
  pair it with the negative case (an unknown/unattested key still hard-refuses).

## Falsifier guidance (re-attack the revision)

- **Falsifier 1 (C1 / severance-completeness):** Is the taxonomy actually
  COMPLETE — re-grep the tree for any daemon-run git exec outside the chokepoint +
  sanctioned allowlist. Is `recovery_quarantine_lane.go:425` truly closed? Do the
  three corpus rows go red-before/green-after? Does A12 inspect helper call sites,
  and is the §11 manifest now TRUE (or honestly retracted)?
- **Falsifier 2 (C2 / false-positive + carry-forward):** Can a benign
  `[alias]`/`[pager]` still wedge by any route? Is the observed-vs-blocker state
  model coherent and `false_positive_benign_test` real? Does an unknown key still
  hard-refuse? Then verify NO carry-forward regressed (layered severance, A7/A8/A16,
  A21/A21b, A22/A23/A25, the §0 corrections).

The adjudicator gates on whether C1 and C2 are each **genuinely discharged**
(mechanisms anchored to real source; the named corpus rows + `false_positive_benign_test`
specified) and whether any carry-forward regressed or any new material challenge
lands. A clearing verdict (`accept` / `accept_with_findings`) requires both
discharged with no standing regression. Keep the local-first product boundary (one
host, one daemon as single writer; no hosted services). This is the single allowed
v2 revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator.
