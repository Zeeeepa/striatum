# Design-Run Seed — RFC 0164 P0 (FRESH v1)

> This is the **fresh v1** `falsification_gate` design run for RFC 0164
> (untrusted-substrate hardening — read-side git neutralization + the
> gate-evidence recovery contract). The RFC's **Decision is already settled**
> (allowlist posture realized as **layered severance**; denylist demoted to
> non-load-bearing telemetry) with a Slice 0-3 plan. This run hardens that
> design into **falsifiable, build-bearing acceptance criteria** and stress-tests
> the severance-completeness and evidence/recovery claims.
> **Required context docs** (read in full first):
> - `docs/rfcs/0164-untrusted-substrate-read-side-git-hardening.md` — the RFC (Decision, the 3 layers, Slice 0-3, Goals G1-G4, Risks, rejected traps).

## Charter

The deliverable (committed `PROPOSAL.md`, "the SPEC") is the **falsifiable
implementation spec for RFC 0164 P0** the downstream `rfc-0164-build`
`code_change` run executes. Do **not** re-litigate the posture (the RFC's
`/adhd` pass already chose layered severance over a denylist wrapper, and
already logged write-confinement + repo-config-validator as solved/moot — do
not rebuild them). The SPEC must turn the settled design into build-bearing
constraints, each a concrete falsifiable assertion + the test/corpus row that
would refute it, and define the **P0 slice** precisely.

## The threat (do NOT re-litigate the posture — D-of-record in the RFC)

striatum drives agents against **untrusted repositories**: a target repo's
`.git/config` / `.gitmodules` / `.gitattributes` / ambient `GIT_*` are
attacker-controlled, and git treats `core.pager`, `diff.external`, `merge.tool`,
`core.fsmonitor`, `core.hooksPath`, `core.sshCommand`, `url.<base>.insteadOf`,
`*.textconv`, `alias.*` as **code-execution gadgets** that run under the daemon
identity on an innocent `git log`/`diff`/`show`/`status`. The commit/apply path
is hardened (`git_commit_apply.go:342`, RFC 0127); the **read paths run bare**
(`git_snapshot.go:193-212`, `doctor_artifact_anchor.go:511/559`,
`worktree_refs.go:393/399`, `status.go:388`, `run.go:920-956`,
`verifier/receipt.go:606/609`). A `git log` against a repo with
`core.pager=<payload>` is a **daemon-host compromise**. There is no
`tests/redteam/` planted-attack regression today.

## The hard core to PROVE

The whole design leans on two properties:

1. **Severance is COMPLETE.** Every read-side git invocation is neutralized so
   **no** untrusted config/env value can reach command execution under the
   daemon identity — enforced **structurally** (one chokepoint + a closed
   environment + a build-time invariant test), not by a checklist a future call
   site can forget. An unknown future gadget must be inert **by omission**, never
   by being on a known-bad list.
2. **Severance is CORRECT.** The objects-only `clean.git` (and the env floor)
   must not yield **wrong answers** — a missed `objects/info/alternates`,
   packed/per-worktree ref, or benign-but-needed key (`core.ignorecase`,
   gitdir-relative `core.worktree`) makes `merge-base`/`rev-parse` silently
   resolve against a truncated graph. Wrong answers, not errors, are the sharp
   edge.

## P0 slice boundary (the security floor lands first)

Propose **P0 = Slice 0 + Slice 1** (independently verifiable, lands the floor):

- **Slice 0 — the chokepoint seam (no behavior change).**
  `CleanRepoFor(repoRoot, laneID)` returning `repoRoot` unchanged, with ALL ~6
  bare read-side call sites routed through it. Collapses the attack surface to
  one function; reviewable in isolation. The SPEC must prove the call-site list
  is **exhaustive** (grep-backed, including indirect spawns + verifier/doctor).
- **Slice 1 — `gitEnv()` + the red-team corpus (Layer 2).**
  `go/pkg/safegit/gitenv.go` returns a fully-pinned `cmd.Env` (never
  `os.Environ()`): **OMITS** the entire `GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n`
  family (load-bearing — env config beats `-c`, so omission *is* the
  neutralization), pins `GIT_CONFIG_NOSYSTEM=1` + `GLOBAL`/`SYSTEM=/dev/null` +
  sacrificial `HOME`/`XDG`, hard-drops gadget env vars, bounds the subcommand
  allowlist, and **REFUSES with a typed `gitEnvUnavailable`** rather than
  falling back to bare `exec`. Set on striatum's calls **and** the driven
  agent's lane env (so a socially-engineered bare `git` is born-neutralized). The
  corpus (`gate_corpus_test.go`) runs each gadget asserting a sentinel is never
  created, with the in-repo `.git/config` row as an **expected-fail against Layer
  2 alone** documenting exactly what Slice 2 closes. Plus the **build-time
  invariant test** failing the build if any `exec.Command(git…)` leaves
  `cmd.Env` nil or `os.Environ()`-sourced.

Name **Slice 2** (minted objects-only `clean.git`, Layer 1, gated on a parity
harness) and **Slice 3** (gate refs + `gate.preflight_attested` + two-state
machine-decay/human-accept recovery, Layer 3) as the build-bearing seams P0
leaves — and decide whether either is pulled into P0. Keep the local-first
boundary; honor the RFC Non-Goals.

## Open design points to DISCHARGE (each → a constraint + test)

- The **argv_digest / env_allowlist_digest / config_fingerprint canonicalization**
  written as **golden vectors** before any hashing (arg order, repeated flags,
  path normalization, env sort + value policy) — an under-specified
  canonicalization is a forgeable digest.
- The **fingerprint-decay TOCTOU**: decay may *clear the blocker* but must
  **never authorize the next call** — exec always re-attests, or it is a bypass.
- The **two-state recovery split**: known-and-neutralized → machine-clearable
  `gate.read_gadget_detected` (idempotent decay via `recovery.sweep`, no human);
  unknown/unattested → hard refusal into the human-cleared
  `recovery.quarantine_lane`. No path silently passes an unknown (a faked gate).
- The **parity harness** that proves Slice 2's `clean.git` reads equal bare reads
  over a corpus (so severance is correct, not just safe).

## Falsifier guidance (attack the v1 proposal)

- **Falsifier 1 (severance-completeness lens):** find any route by which a gadget
  still reaches exec (an unrouted call site; the agent's own bare git; the
  `GIT_CONFIG_COUNT` env family; a worktree inheriting attacker config via
  common-dir/alternates/`core.hooksPath` in the shared object store; a missing
  benign key yielding wrong answers; an unenforced subcommand allowlist; a
  bare-exec fallback when the closed env can't be set).
- **Falsifier 2 (evidence/recovery-contract lens):** is the attestation forgeable
  (canonicalization under-specified; back-datable KV append)? Is the decay TOCTOU
  open? Can an unknown gadget pass silently? Does the corpus actually certify
  (`corpus_green_hash` load-bearing)? Does a false-positive on a benign
  `[alias]`/`[pager]` wedge a legit repo (must degrade observability only — G3)?

The adjudicator gates on whether severance is proven **complete and correct**,
the env floor refuses rather than degrades, the evidence contract is unforgeable
with the decay-TOCTOU closed, the two-state recovery never silently passes an
unknown, and the red-team corpus certifies — i.e. Goals G1-G4 met, not merely
claimed. A clearing verdict (`accept` / `accept_with_findings`) requires all of
that with no standing falsifier challenge. This is the single allowed v1
revision cycle; a second `needs_revision` ends the gate uncleared and routes to
the operator (who spins a fresh `-v2` run with a revising holder).
