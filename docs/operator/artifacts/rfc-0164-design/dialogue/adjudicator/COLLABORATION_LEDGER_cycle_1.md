---
schema_version: "striatum.collaboration_ledger.v1.1"
artifact_kind: "collaboration_ledger"
shape: "falsification_gate"
author: adjudicator-author-001
workflow: "rfc-0164-design"
run_id: "run_9f7d019646d985c44878b24fcdc94186"
cycle: 1
topic: "RFC 0164 P0 falsifiable implementation SPEC — read-side git neutralization (layered severance: CleanRepoFor chokepoint + closed gitEnv() omission floor + compile-time invariant + red-team corpus) and the gate-evidence two-state recovery contract"
participants:
  - holder
  - falsifier_1
  - falsifier_2
  - adjudicator
entries:
  - kind: claim
    by: holder
    refs: ["dialogue:1"]
    text: "The v1 SPEC claims it discharges G1-G4 for RFC 0164 P0 (Slice 0 + Slice 1). G1 by structure: one `CleanRepoFor(repoRoot, laneID)` chokepoint routing an enumerated read-side surface (§0 C-2: 11 sites across 7 files plus the `localGit.output` / `readGitOutput` funnel helpers), a closed `gitEnv()` that OMITS the `GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n` family and all gadget env vars (built from an allowlist, never `os.Environ()`), refuses with typed `ErrGitEnvUnavailable` rather than degrading (A8), born-neutralizes the agent lane env at `loop.go:268` (A10), and a compile-time AST invariant (`TestDaemonGitInvocationsAreNeutralized`, A12-A14) failing the build on any nil or `os.Environ()`-sourced `cmd.Env`. CORRECT (G2-truncation): P0 has NO truncated-graph risk because Slice 0 returns `repoRoot` unchanged, so the parity reduces to an env-floor answer-equivalence test (A21) with the alternates/refs/benign-key parity harness named as the binding Slice-2 gate (A21b). Evidence/recovery (G2/G3) frozen as P0 contracts: canonicalization golden vectors before any hashing (A22), no-attestation-before-exec is a hard doctor barrier failure (A25), decay-TOCTOU closed by exec re-attest (A23), two-state recovery known->machine-decay / unknown->human-accept (A24). G4: a red-team gadget corpus with first-class expected-fail residual rows and a deterministic `corpus_green_hash` (A15-A19). It also publishes four source corrections C-1..C-4 (status.go is in `reads`; the surface is larger than the RFC's ~6; the commit path pins no `cmd.Env`; an `os.Environ()`-sourced env is a real in-tree false-negative)."
  - kind: challenge
    by: falsifier_1
    refs: ["dialogue:2"]
    correspondence: landed_unrebutted
    text: "Severance-completeness gap (refutes A2, A12, A20). The holder's `exhaustive` C-2 surface omits the daemon's second git funnel: `defaultRunGitWorktreeCommand` (`mutations/worktree.go:1603`) and `integrateGit` (`mutations/integrate.go:194`), which set no `cmd.Env`, carry no closed config, run in target-repo / per-job-worktree dirs, and are called throughout mutation, recovery, fan-in, durability and reconstructability code. Concrete still-live gadget routes after P0's gitEnv() + read chokepoint land: `recovery_quarantine_lane.go:425` runs `git status --porcelain` (fires in-repo `core.fsmonitor`); `artifact_durability.go:138` runs `git add -f` (fires `filter.<x>.clean`) and `:157` runs `git commit` (fires `core.hooksPath`); `artifact_reconstructability.go:305/383/389/400`, `worktree.go:181/192/200/738/1325/1336/1351/1793/1810`, and `barrier_fanin.go:906/933/952` run `for-each-ref`/`rev-parse`/`merge-base`/`rev-list`/`show`/`archive` through the same nil-env helper. These are read/ref operations by the holder's OWN standard, so they belong in the surface A2 calls complete. The bind: §4/A12 says the invariant runs over `mutations` and bans any nil/`os.Environ()`-sourced git exec outside the chokepoint — so either the invariant flags the whole funnel (P0 cannot ship green within the §11 manifest) or it carves the funnel out (A12/A2 completeness is false). The existing guard (`git_invocation_guard_test.go:45-55`) proves production git is intentionally hidden behind these helpers, so an AST check over raw `exec.Command` either misses the surface or cannot use the C-2 allowlist (many required subcommands absent). Adds three required red-before/green-after corpus rows: `quarantine_status_fsmonitor`, `porter_add_filter_clean`, `porter_commit_hookspath`, each locally validated to create a sentinel today. The scope rebuttal (mutation/porter is Slice 2) fails on the holder's own invariant-over-mutations claim and on these being read/ref ops."
  - kind: challenge
    by: falsifier_2
    refs: ["dialogue:3"]
    correspondence: landed_unrebutted
    text: "Recovery-contract false-positive wedge (refutes A24, A26 / G3). The two-state split makes a recognized-and-neutralized gadget a machine-clearable `gate.read_gadget_detected` blocker pinning `config_fingerprint`, and the ONLY specified machine-clear condition is `the live fingerprint no longer matches` (HOLDER §8.3). For a benign repo that legitimately KEEPS `[alias] co = checkout` / `[pager] log = less -FRX`, the fingerprint is stable, so the pin never decays and the run is blocked until a human edits benign config — exactly the wedge G3 (and the RFC risk section, RFC:239-244) forbids. A26 asserts the no-wedge property but never specifies the state transition that delivers it. The detector is a key-family denylist; `alias.*` and `core.pager` ARE known gadget families, and the RFC's own risk text says a benign `[alias]`/`[pager]` is indistinguishable from a hostile one until executed — so the `recognized means known-hostile, not benign` rebuttal is unspecified and unsound without a value-level safe-list parser or a non-blocking state, neither of which the SPEC provides. `corpus_green_hash` does not save it (A19 only requires determinism), and the §11 manifest omits any load-bearing false-positive recovery test; A26's `false_positive_benign_test` as described asserts only `no sentinel`, not job-state / recovery-ref behavior. The contract internally contradicts itself: it calls the record a `blocker` with decay semantics (A23 depends on blocker semantics) while claiming observability-only — it must pick one and specify it, with unknown/unattested keys still hard-refusing."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:2"]
    text: "C1 (carries into the single revision): replace the C-2 site-list with a complete git-surface TAXONOMY that enumerates every funnel (`localGit.output`, `readGitOutput`, `runGitWorktreeCommand`, `integrateGit`, `runGitWithEnv`, and direct helper-package calls), classifies each route (read-only / ref-plumbing / index-or-worktree mutation / lane-side / CLI-only), and routes every daemon-run route through the closed env, with every in-repo-config-sensitive route (`status`, `add`, `commit`, `write-tree`, `diff`, and textconv/filter/hook/fsmonitor carriers) through a minted config OR a typed pre-exec refusal. Reconcile §4/A12 with the funnel explicitly: the invariant MUST inspect helper call sites, and the P0 §11 manifest + green-build claim must be made true (or A2/A12 retracted to a scoped claim with the funnel residual stated honestly like §1). Specifically close `recovery_quarantine_lane.go:425` status->fsmonitor, which the v1 SPEC implies is closed but is not. Add the three corpus rows `quarantine_status_fsmonitor`, `porter_add_filter_clean`, `porter_commit_hookspath` with red-before/green-after results."
  - kind: constraint
    by: adjudicator
    refs: ["dialogue:3"]
    text: "C2 (carries into the single revision): resolve the blocker-vs-observability contradiction for recognized-benign detections so a benign-but-retained `[alias]`/`[pager]` NEVER wedges. Either (a) split recognized detections into a non-blocking `gate.read_gadget_observed` state for neutralized/benign-safe keys and reserve a blocking state for conditions that actually require recovery, OR (b) define a precise benign safe-list/parser for `[alias]`/`[pager]` where safe benign keys do not create a blocking pin — and state which. Keep unknown/unattested keys on the hard-refusal-into-human-cleared path (no silent unknown-pass, A24). Make `false_positive_benign_test` load-bearing: plant the benign config, run an allowlisted read, assert no job/run blocker, no `recovery.quarantine_lane` ref, and that a second read proceeds with no repo edit and no human clear; pair it with the negative case (an unknown/unattested key still hard-refuses)."
verdict: "needs_revision"
rationale: "needs_revision. The v1 SPEC is high-quality and source-anchored: its posture is correct (it does not re-litigate layered severance), its OMISSION reasoning is right (the `GIT_CONFIG_COUNT` family beats `-c`, so omission IS the neutralization — A7/A16), the env floor REFUSES rather than degrades (typed `ErrGitEnvUnavailable`, no bare fallback — A8), the CORRECT/truncation property is handled cleanly for P0 (Slice 0 keeps `repoRoot` unchanged, so there is no truncated graph; the alternates/refs/benign-key parity harness is named as the binding Slice-2 gate — A21/A21b), and the evidence-contract MECHANICS are sound and went unchallenged (canonicalization golden vectors before hashing with adversarial cases — A22; no-attestation-before-exec as a hard doctor barrier — A25; decay-TOCTOU closed by exec re-attest — A23). The four source corrections C-1..C-4 (status.go mislabeled, the surface undercount, the env-incomplete commit path, the `os.Environ()` false-negative that turns the invariant from a nil-check into an `os.Environ()`-ban) are real contributions. BUT both gate-critical properties carry a verified, standing material challenge and neither was rebutted in v1. (1) SEVERANCE IS NOT COMPLETE: the `runGitWorktreeCommand`/`integrateGit` mutation funnel is omitted from the `exhaustive` C-2 surface yet runs gadget-bearing git (`status`->`core.fsmonitor`, `add`->`filter.clean`, `commit`->`core.hooksPath`, plus `for-each-ref`/`rev-parse`/`merge-base`/`rev-list`/`show`) under the daemon identity against attacker-controlled repos and worktrees — including a live `recovery_quarantine_lane.go:425` status->fsmonitor RCE the SPEC implies is closed. By the holder's own §4/A12 (invariant over `mutations`, ban nil/`os.Environ()` env) the SPEC cannot both ship a green invariant within the §11 manifest and leave the funnel unrouted; A2 and A12 are refuted and A20 (no silent in-repo gadget exec) is not established. (2) A FALSE-POSITIVE WEDGES LEGIT WORK: a benign-but-retained `[alias]`/`[pager]` is flagged as a recognized gadget and written as a machine-clearable blocker whose only clear condition is fingerprint decay, which never fires on stable benign config — so the run is wedged until a human edits benign config, contradicting A26/G3; the SPEC also internally contradicts itself by calling the record a `blocker` (A23 relies on blocker semantics) while claiming observability-only, without specifying the non-blocking state or a benign safe-list. The rubric requires severance PROVEN complete and a false-positive that degrades observability ONLY for a clearing verdict; both are missing and both falsifier challenges stand unrebutted. This is the single allowed revision cycle: discharge C1 (a complete git-surface taxonomy + route the funnel + close `recovery_quarantine_lane.go:425` + add the three corpus rows + reconcile A2/A12 honestly) and C2 (a non-blocking observed state OR a benign safe-list so a stable benign `[alias]`/`[pager]` never wedges, with a load-bearing `false_positive_benign_test` and the unknown-key hard-refusal kept). Both repairs are concrete and in-P0/in-contract buildable, so the gate is recoverable in one revision — this is needs_revision, not reject. A second needs_revision ends the gate uncleared and routes to the operator (a fresh -v2 run with a revising holder)."
findings:
  - id: F-FUNNEL-SURFACE-INCOMPLETE
    severity: critical
    posture: severance_incomplete
    status: converted_to_constraint
    challenge: "The `exhaustive` §0 C-2 read/ref surface omits the daemon's second git funnel `defaultRunGitWorktreeCommand` (`mutations/worktree.go:1603`) and `integrateGit` (`mutations/integrate.go:194`), which set no `cmd.Env` and run gadget-bearing git (`status`->fsmonitor at `recovery_quarantine_lane.go:425`; `add`->filter.clean at `artifact_durability.go:138`; `commit`->hooksPath at `:157`; plus `for-each-ref`/`rev-parse`/`merge-base`/`rev-list`/`show`/`archive` across `artifact_reconstructability.go`, `worktree.go`, `barrier_fanin.go`) under the daemon identity against attacker-controlled repos. These are read/ref ops by the holder's own standard, so A2 is refuted; and §4/A12 (invariant over `mutations`, ban nil/`os.Environ()` env) cannot ship a green build within the §11 manifest while leaving the funnel unrouted, so A12 is refuted and A20 unestablished. A live daemon-identity RCE route (fsmonitor-on-status via the quarantine path) remains open after P0 as written."
    affected_invariants: ["A2", "A12", "A20", "A0"]
    source_refs: ["dialogue:2"]
  - id: F-FALSEPOS-RECOVERY-WEDGE
    severity: critical
    posture: false_positive_wedge
    status: converted_to_constraint
    challenge: "The two-state recovery makes a recognized-and-neutralized gadget a machine-clearable `gate.read_gadget_detected` blocker whose only specified clear condition is fingerprint decay (HOLDER §8.3). A benign repo that legitimately retains `[alias] co = checkout` / `[pager] log = less -FRX` has a stable fingerprint, so the pin never decays and the run is blocked until a human edits benign config — the exact wedge G3/A26 and RFC:239-244 forbid. The contract internally contradicts itself (it calls the record a `blocker` with decay semantics that A23 depends on, while A26 claims observability-only) and provides neither a non-blocking observed state nor a value-level benign safe-list, so the `recognized means known-hostile not benign` rebuttal is unspecified and unsound. The §11 manifest omits a load-bearing false-positive recovery test; A26's named test as described asserts only `no sentinel`, not job-state/recovery-ref behavior."
    affected_invariants: ["A24", "A26"]
    source_refs: ["dialogue:3"]
constraints:
  - id: C1-FUNNEL-TAXONOMY-AND-ROUTE
    source_finding: F-FUNNEL-SURFACE-INCOMPLETE
    posture: severance_incomplete
    severity: critical
    kind: gate
    binding: true
    text: "The revised SPEC must replace the C-2 site list with a complete git-surface TAXONOMY enumerating every funnel (`localGit.output`, `readGitOutput`, `runGitWorktreeCommand`, `integrateGit`, `runGitWithEnv`, direct helper-package calls), classify each route (read-only / ref-plumbing / index-or-worktree mutation / lane-side / CLI-only), route every daemon-run route through the closed env, and route every in-repo-config-sensitive route (`status`/`add`/`commit`/`write-tree`/`diff` and textconv/filter/hook/fsmonitor carriers) through a minted config OR a typed pre-exec refusal. It must reconcile §4/A12 with the funnel (the invariant inspects helper call sites; the §11 manifest + green-build claim is made true, or A2/A12 are retracted to a scoped claim with the funnel residual stated honestly), explicitly close `recovery_quarantine_lane.go:425` status->fsmonitor, and add the three red-before/green-after corpus rows."
    source_refs: ["dialogue:2"]
    verification:
      gate: "Corpus rows quarantine_status_fsmonitor, porter_add_filter_clean, porter_commit_hookspath are red on current source and green after the fix; the build-time invariant flags any funnel/helper git exec with nil or os.Environ()-sourced env; a tree-wide grep of the taxonomy finds no daemon-process read/ref/mutation git exec outside the chokepoint + sanctioned allowlist."
      expected_stage: "holder revision (cycle 2) + rfc-0164-build"
    final_review_required: true
  - id: C2-FALSEPOS-NONBLOCKING-STATE
    source_finding: F-FALSEPOS-RECOVERY-WEDGE
    posture: false_positive_wedge
    severity: critical
    kind: gate
    binding: true
    text: "The revised SPEC must guarantee a benign-but-retained `[alias]`/`[pager]` NEVER wedges: either (a) split recognized detections into a non-blocking `gate.read_gadget_observed` state for neutralized/benign-safe keys and reserve a blocking state for conditions that genuinely require recovery, OR (b) define a precise benign safe-list/parser where safe benign keys create no blocking pin — and state which, resolving the blocker-vs-observability contradiction. Unknown/unattested keys must still hard-refuse into the human-cleared lane (no silent unknown-pass). `false_positive_benign_test` must become load-bearing."
    source_refs: ["dialogue:3"]
    verification:
      gate: "false_positive_benign_test plants the benign [alias]/[pager] config, runs an allowlisted read, and asserts no job/run blocker, no recovery.quarantine_lane ref, and a second read proceeds with no repo edit and no human clear; the paired negative case asserts an unknown/unattested key still hard-refuses into the human-cleared lane."
      expected_stage: "holder revision (cycle 2) + rfc-0164-build Slice 3"
    final_review_required: true
branches:
  severance_incomplete: "blocked"
  false_positive_wedge: "blocked"
---

# Collaboration Ledger — RFC 0164 P0 design (cycle 1)

**Verdict: `needs_revision`.** This is the single allowed revision cycle. The v1
SPEC is strong, well-anchored to current `striatum/rfc-0164-design` source, and its
posture and omission reasoning are correct — but **both** gate-critical properties
(severance is COMPLETE; a false-positive degrades observability ONLY) carry a
verified, standing material challenge that v1 did not rebut, so the gate does not
clear.

## Per-goal disposition (G1–G4)

| Goal | Demand | Status | Basis |
|------|--------|--------|-------|
| **G1** | No untrusted config/env value reaches command execution under the daemon identity through *any* read-side git invocation — enforced structurally, not by a forgettable checklist | **NOT MET** | The chokepoint + `gitEnv()` + invariant are well-designed for the enumerated surface, but Falsifier 1 shows the `runGitWorktreeCommand`/`integrateGit` mutation funnel (nil-env, gadget-bearing `status`/`add`/`commit`/`for-each-ref`/`rev-parse`/`merge-base`/`rev-list`/`show`) is omitted from the `exhaustive` surface and reaches exec — including a live `recovery_quarantine_lane.go:425` status→fsmonitor RCE. A2/A12 refuted, A20 unestablished. **F1 standing.** |
| **G2** | Auditable: live policy version + closed-env proof recoverable; a gate that silently passes an *unknown* gadget is a faked gate | **PARTIALLY MET** | The contract MECHANICS are sound and unchallenged: canonicalization golden vectors before hashing (A22), no-attestation-before-exec as a hard doctor barrier (A25), decay-TOCTOU closed by exec re-attest (A23). But coverage inherits G1's gap — the attestation/barrier surface must enumerate the funnel exec sites it currently misses, or unattested funnel execs pass without attestation. Sound-but-incomplete. |
| **G3** | A tripped repo recovers without a code change or irreversible exclusion; a false-positive must not wedge a benign repo | **NOT MET** | Falsifier 2: a benign-but-retained `[alias]`/`[pager]` becomes a machine-clearable blocker whose only clear condition (fingerprint decay) never fires on stable benign config → wedge. A26 asserts no-wedge without the state transition that delivers it; the contract contradicts itself (blocker vs observability). **F2 standing.** |
| **G4** | Regression-tested by a planted-attack corpus that executes under the old path and no-ops under the new one | **NOT MET** | The corpus design is genuinely strong (one row per gadget, expected-fail residual rows as first-class assertions, deterministic `corpus_green_hash` — A15–A19). But it omits the three funnel rows (F1) and lacks a load-bearing false-positive recovery test (F2); determinism alone (A19) does not certify the full surface. |

## Per-falsifier disposition

- **Falsifier 1 — severance-completeness (dialogue:2): MATERIAL, STANDING (`landed_unrebutted`).**
  The omitted `mutations/worktree.go:1603` (`defaultRunGitWorktreeCommand`) +
  `mutations/integrate.go:194` (`integrateGit`) funnel runs gadget-bearing git
  under the daemon identity. The decisive point is internal to the SPEC: §4/A12
  declares the invariant runs over `mutations` and bans nil/`os.Environ()` env, so
  the funnel cannot be both green-under-the-invariant and unrouted within the §11
  manifest; and several funnel calls are read/ref ops, so they belong in A2's
  "complete" surface regardless of P0 read-side scoping. The scope rebuttal does
  not save v1. → **C1.**
- **Falsifier 2 — recovery-contract false-positive (dialogue:3): MATERIAL, STANDING (`landed_unrebutted`).**
  A stable benign `[alias]`/`[pager]` becomes an unbounded blocker because the only
  machine-clear condition is fingerprint decay. A26 is asserted, not specified; the
  `recognized = known-hostile` rebuttal is unspecified and unsound without a
  value-level safe-list, and the SPEC uses blocker semantics (A23) while claiming
  observability-only. → **C2.**

## What the v1 SPEC gets right (carry forward, do not re-litigate)

- Posture and **omission** reasoning: `GIT_CONFIG_COUNT` family OMITTED because env
  config beats `-c` (A7/A16); the closed env is an allowlist built up, not an
  `os.Environ()` subtraction (A6).
- **Refuse, never degrade:** typed `ErrGitEnvUnavailable`, no bare-exec fallback (A8).
- **CORRECT/truncation handled for P0:** Slice 0 keeps `repoRoot` unchanged → no
  truncated graph in P0; the alternates/refs/benign-key parity harness is named as
  the binding Slice-2 gate (A21/A21b).
- **Evidence-contract mechanics:** golden vectors before hashing with adversarial
  cases (A22); no-attestation-before-exec hard barrier (A25); decay-TOCTOU closed by
  re-attest (A23) — all unchallenged.
- The four source corrections **C-1..C-4** are real and load-bearing (notably the
  `os.Environ()`-ban that the invariant must be, not a nil-check).

## What clears the gate on revision

Both defects are concrete and in-P0/in-contract buildable — recoverable in one
revision (hence `needs_revision`, not `reject`):

1. **C1** — replace the C-2 site list with a complete git-surface **taxonomy**,
   route the `runGitWorktreeCommand`/`integrateGit` funnel (every in-repo-config
   carrier through minted config or typed refusal), explicitly close
   `recovery_quarantine_lane.go:425` status→fsmonitor, reconcile A2/A12 honestly
   with the §11 manifest, and add the three corpus rows
   (`quarantine_status_fsmonitor`, `porter_add_filter_clean`,
   `porter_commit_hookspath`) red-before/green-after.
2. **C2** — make a benign-but-retained `[alias]`/`[pager]` non-wedging via a
   non-blocking `gate.read_gadget_observed` state *or* a precise benign safe-list
   (state which), keep unknown/unattested keys on hard-refusal, and make
   `false_positive_benign_test` load-bearing (assert no blocker, no
   `recovery.quarantine_lane` ref, a second read proceeds with no repo edit / no
   human clear; paired with the unknown-key hard-refusal negative case).

This is the single allowed revision cycle: a second `needs_revision` ends the gate
uncleared and routes to the operator (a fresh `-v2` run with a revising holder).
