# RFC 0164: Untrusted-Substrate Hardening — Read-Side Git Neutralization and the Gate-Evidence Recovery Contract

Status: proposed
Date: 2026-06-22
Context: RFC 0090 (workspace security & attestation parity), RFC 0096 (lane sandbox), RFC 0127 (retire lane git identity); `git_commit_apply.go`, `git_snapshot.go`, `recovery_quarantine_lane.go`
Author: claude-opus-4-8 (prior-art promotion pass)
CONSOLIDATED-FROM: showerthoughts/prior-art-recall.md

---

## Problem

striatum drives external coding agents against **untrusted repositories**. A repo
striatum is told to operate on is, for threat-modeling purposes, a hostile clone:
its on-disk `.git/config`, `.gitmodules`, `.gitattributes`, and ambient `GIT_*`
environment are all attacker-controlled. Git treats large parts of that surface as
**code-execution gadgets** — `core.pager`, `diff.external`, `merge.tool`,
`core.fsmonitor`, `core.hooksPath`, `core.sshCommand`, `url.<base>.insteadOf`,
`*.textconv`, and arbitrary `alias.*` all run a command line under the daemon's
identity when an otherwise-innocent `git log`/`diff`/`show`/`status` fires.

The codebase neutralizes this surface **asymmetrically**:

- The **commit/apply path is hardened**: it injects `-c core.hooksPath=<empty-tmpdir>`
  into commit invocations (`go/pkg/mutations/git_commit_apply.go:342`), and the
  daemon supplies its own identity rather than inheriting `user.name`/`user.email`
  from ambient config (RFC 0127).
- The **read paths run bare**, inheriting the untrusted repo's ambient config and
  the daemon's full environment. Verified call sites that pass no config/env
  neutralizers:
  - `go/pkg/reads/git_snapshot.go:193-212` — `localGit.output()` runs `status`,
    `rev-parse`, `log` with no `cmd.Env` pinning.
  - `go/pkg/reads/doctor_artifact_anchor.go:511,559` — `git cat-file -e`, `git show`.
  - `go/pkg/reads/worktree_refs.go:393,399` — `git merge-base --is-ancestor`,
    `git rev-parse`, `git for-each-ref`.
  - `go/pkg/mutations/status.go:388`, `go/pkg/mutations/run.go:920-956` —
    `git branch --show-current`, `git rev-parse --verify HEAD`.
  - `go/pkg/verifier/receipt.go:606,609` — `git add -A`, `git write-tree`.

A single `git log` against a repo whose `.git/config` sets `core.pager=<payload>`
executes `<payload>` as the daemon user. This is a daemon-host compromise, not a
lane-scoped one.

This RFC decides **how** striatum neutralizes the read-side git gadget surface, and
**how a tripped lane proves the gate held and recovers** — the latter answering an
open question about whether a pre-flight refusal reuses the existing post-failure
quarantine machinery or stands as a distinct state.

### What is already solved (do not re-litigate)

Two steals that the source note (`prior-art-recall.md`) proposed are **already-have**
or **moot** in striatum, and this RFC explicitly does *not* re-build them:

- **Write confinement is solved.** `ValidateSandboxJail()`
  (`go/pkg/mutations/artifact.go`) does recursive symlink resolution and a
  `sameOrInside(target, repoRoot)` jail check, with an escape test at
  `go/pkg/mutations/artifact_integration_test.go:424-455`. Go's `os.WriteFile`
  cannot pass `O_NOFOLLOW`, but the path is canonicalized-before-write, which closes
  the symlink-escape vector the note worried about. **No new write primitive is
  proposed.**
- **A repo-shipped-config validator has no attack surface.** striatum reads **no**
  config file *from* the untrusted repo (no `.striatum.yml` ingestion anywhere in the
  tree) and never inherits git identity from ambient config (RFC 0127). The note's
  "type-validated fail-safe config validator" steal defends a door striatum does not
  have. **Logged as a SKIP**, per the house rule that a clean negative result is an
  asset, not an omission.

The genuine gap is the **read-side git surface** and the **absence of any
planted-attack regression test** (`tests/redteam/` does not exist).

## Goals

- **G1.** No untrusted-repo config or environment value can cause command execution
  under the daemon identity through *any* read-side git invocation — and the property
  is enforced structurally, not by a checklist a future call site can forget.
- **G2.** The neutralization is **auditable**: for any historical git call, an
  operator can show exactly which policy version was live and that the call ran in a
  closed environment. "Never fake a gate" extends to security — a gate that silently
  passes an *unknown* gadget is a faked gate.
- **G3.** A repo that trips the gate **recovers** without a code change and without a
  permanent, irreversible exclusion (decay is a feature). A false-positive on a benign
  repo must not wedge the run.
- **G4.** The hardening is regression-tested by a **planted-attack corpus** that would
  execute under the old bare path and provably no-ops under the new one.

## Non-Goals

- Sandboxing the agent's *arbitrary* (non-git) command execution — that is RFC 0096's
  lane-sandbox surface; this RFC narrows to git specifically.
- Re-deriving write confinement or a repo-config validator (see "already solved").
- Cross-lane "learned immunity" / shared gadget-signature propagation. striatum is a
  single-operator system; a fleet-immunity ledger solves a scale problem we do not
  have (logged as a trap below).

## Decision

Adopt the **allowlist posture** (an unknown future gadget must be inert by omission,
never relied on being on a known-bad list), realized as **layered severance** rather
than a single `-c`-injecting wrapper. The denylist *detector* is retained but
**demoted** to a non-load-bearing telemetry/quarantine trigger — it never makes
execution safe; it only explains and records.

Three layers, each independently sufficient to block a class, stacked so that a gap in
one is covered by the next:

### Layer 1 (security boundary) — Sever config from objects via one chokepoint

Route **all** read-side git through a single seam, `CleanRepoFor(repoRoot, laneID)
→ cleanGitDir`. The seam resolves to a striatum-minted `clean.git` (a bare repo at a
daemon-owned path, e.g. `$STATE/lanes/<id>/clean.git`) whose **object store is
hardlink-shared** from the untrusted repo (`.git/objects`, `packed-refs`, loose
`refs/`, and the read-only accelerators `commit-graph` / `multi-pack-index`) and
whose **`config` striatum writes from a fixed template** — `core.bare=true`, no
`pager`/`external`/`hooks`/`alias`/`insteadOf`/`fsmonitor` keys at all. Git objects
are content-addressed and immutable, so sharing them is cheap and safe; the attacker's
config simply does not travel with them. This is **strictly stronger than `-c`
injection because striatum never parses the untrusted config at all** — there is no
decoder eating attacker bytes.

The chokepoint is the load-bearing move: it collapses the ~6 scattered bare call sites
to one place to harden, and makes "every read is severed" a property of one function
rather than a convention spread across the reads package. Per-lane worktrees attach to
`clean.git` as their common-dir, so they inherit the minted config, not the
attacker's.

### Layer 2 (defense-in-depth) — Born-neutralized environment + bounded subcommands

A single `gitEnv() ([]string, error)` builder returns a fully-pinned environment
(never `os.Environ()` passthrough), set explicitly as `cmd.Env` on **every** git
spawn — striatum's own calls *and* the driven agent's lane environment, so an agent
socially-engineered (via a hostile `AGENTS.md`/README) into running bare `git` is
*born* neutralized. `gitEnv()`:

- **Omits the entire `GIT_CONFIG_COUNT` / `GIT_CONFIG_KEY_n` / `GIT_CONFIG_VALUE_n`
  family.** This is load-bearing and non-obvious: env-injected config takes
  **precedence over command-line `-c`**, so a wrapper that only injects `-c`
  neutralizers is silently bypassable. Omission *is* the neutralization.
- Pins `GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null`,
  `GIT_CONFIG_SYSTEM=/dev/null`, and a sacrificial `HOME`/`XDG_CONFIG_HOME` at an
  empty daemon-owned dir.
- Hard-drops gadget env vars by allowlist (`GIT_EXTERNAL_DIFF`, `GIT_PAGER`,
  `GIT_SSH_COMMAND`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, … never appear).
- A **bounded subcommand allowlist** (`log`, `diff-tree`, `show`, `cat-file`,
  `rev-parse`, `status`, `for-each-ref`, `merge-base`, with pinned flags); anything
  else is refused before exec.
- **Refuses rather than degrades:** if the closed environment cannot be established
  (sacrificial dir unstat-able, builder errors), the call returns a typed
  `gitEnvUnavailable` error. It never falls back to bare `exec.Command`. (G1, "never
  fake a gate".)

Layer 2 exists because Layer 1's severance, while strong, must be *complete* about
refs/alternates (see Risks); the env floor guarantees that even an un-severed call, or
the agent's own git, cannot honor an env/global-config gadget.

### Layer 3 (evidence + recovery) — The gate proves itself, and decays cleanly

Neutralization that cannot be shown is indistinguishable from a faked gate. Add a
**verb-prefixed `refs/striatum/gate/<run>/<job>/<attempt>` subtree** (paralleling the
existing `recovery/` and `staged/` prefixes, so one `for-each-ref refs/striatum/`
glob still enumerates everything while `NOT LIKE 'refs/striatum/gate/%'` keeps gate
refs out of the integrate/fan-in sweeps).

- **Per-call attestation.** Before git forks, append one KV event
  `gate.preflight_attested` carrying `{neutralizer_set: "neutralizer-set@vN",
  set_hash, corpus_green_hash, argv_digest, env_allowlist_digest,
  config_fingerprint}`. A `git.*` exec line with **no** immediately-preceding
  `gate.preflight_attested` is itself a hard audit failure — you cannot back-date a KV
  append, so a clean run cannot be retroactively forged.
- **The red-team corpus is the certificate.** A CI table test runs each known gadget
  (pager, `diff.external`, `url.insteadOf`, `fsmonitor`, `alias`, the
  `GIT_CONFIG_COUNT` family) through the neutralizer and asserts a provable no-op; the
  green-result hash *is* `corpus_green_hash`. The allowlist's authority comes from a
  passing adversarial suite, not a promise.
- **Two states, split by clearer-of-record** (this resolves the open sub-question):
  - A **recognized-and-neutralized** gadget writes a *machine-clearable*
    `gate.read_gadget_detected` blocker, pinning the offending `config_fingerprint`.
    The existing `recovery.sweep` (`recovery.auto` path) recomputes the live
    fingerprint each pass and, when it no longer matches, appends
    `gate.read_gadget_cleared` and deletes the pin — **idempotent decay, no
    `un-quarantine` verb, no human in the loop.** A human can never fake a clear, and
    a repo that scrubs its hostile config self-heals.
  - An **unknown / unattested** key (no allowlist entry, no green-corpus coverage)
    takes a **hard refusal** into the *existing human-cleared*
    `recovery.quarantine_lane` (`refs/striatum/recovery/`, cleared by
    `recovery accept-quarantined`) — because an unrecognized gadget is exactly the
    case a human must adjudicate. A silent pass of the unknown is the faked gate.

  Both states grep under one `refs/striatum/` prefix but are governed separately:
  machine-decay for the known, human-accept for the unknown. **Crucially, decay may
  *clear the blocker* but must never *authorize the next call*** — authorization
  always re-attests at exec time (Layer 3's pre-flight), or the fingerprint-decay
  becomes a TOCTOU bypass (present a clean config to the sweep, re-inject before the
  next fork).

## Slice plan

Sliced so the security boundary lands first and each slice is independently
verifiable.

- **Slice 0 — the chokepoint seam (no behavior change).** Introduce
  `CleanRepoFor(repoRoot, laneID)` returning `repoRoot` unchanged, and route the ~6
  bare read-side call sites through it (`git_snapshot.go`, `doctor_artifact_anchor.go`,
  `worktree_refs.go`, `status.go:388`, `run.go`, `verifier/receipt.go`). This collapses
  the attack surface to one function with zero functional change — reviewable in
  isolation.
- **Slice 1 — the red-team corpus + `gitEnv()` (Layer 2).** Add
  `go/pkg/safegit/gitenv.go` (`gitEnv()`, ~30 LOC, errors if the sacrificial dir is
  missing) and `go/pkg/reads/gate_corpus_test.go`: one row per gadget
  (`GIT_CONFIG_COUNT`+`KEY_0=core.pager`+`VALUE_0='touch <sentinel>'`,
  `GIT_EXTERNAL_DIFF`, `GIT_PAGER`, `GIT_SSH_COMMAND`, a malicious `HOME/.gitconfig`,
  and a poisoned in-repo `.git/config [core] pager=`), each running a real allowlisted
  subcommand with `cmd.Env = gitEnv()` against a temp repo and asserting the sentinel
  is **never** created. The in-repo `.git/config` row is an *expected-fail* against
  Layer 2 alone — it documents precisely the residual vector Layer 1 closes, so the
  layering is a tested claim, not prose.
- **Slice 2 — minted `clean.git` behind the seam (Layer 1).** Make `CleanRepoFor`
  mint and cache the objects-only clean repo; flip the corpus's in-repo-config row to
  expected-pass. Gate on a parity harness (clean.git reads vs bare-repo reads over a
  corpus) before it replaces the bare path.
- **Slice 3 — gate refs + attestation + two-state recovery (Layer 3).** The
  `refs/striatum/gate/` subtree, `gate.preflight_attested`, the machine-decay /
  human-accept split, and a `striatum doctor` barrier check that flags any `git.*`
  exec lacking a matching preceding attestation.

A build-time invariant test (mirroring the existing
`TestDaemonMutationGitInvocationsDoNotUseCheckoutOrWorkingTreeMerge` guard) should fail
the build if any `exec.Command(git…)` leaves `cmd.Env` nil or sources it from
`os.Environ()` — making "every git call is neutralized" a compile-time property.

## Risks and trade-offs

- **Severance completeness (Layer 1's sharp edge).** An objects-only clone that misses
  `objects/info/alternates`, packed/per-worktree refs, or a benign-but-needed key
  (`core.ignorecase`, gitdir-relative `core.worktree`) yields **wrong answers, not
  errors** — `merge-base`/`rev-parse` silently resolve against a truncated graph.
  Mitigation: the minting step must resolve alternates and carry all ref scopes, and
  Slice 2 is gated on a parity harness. The "tiny structural allowlist" of benign keys
  is discovered empirically against that harness, not guessed.
- **False-positive gadget detection wedging legit repos.** A benign `[alias]`/`[pager]`
  is indistinguishable from a hostile one until executed; an over-eager detector pushes
  real work into the human-cleared lane and erodes trust in the gate. Mitigation:
  detection is **telemetry, not the security boundary** — Layers 1+2 already make the
  config inert, so a false positive degrades observability, never correctness; and
  `gate_read_gadget` decay-vs-human metrics make an over-aggressive pattern visible.
- **Fingerprint-decay TOCTOU.** Covered above: decay clears the blocker, exec
  re-attests. Spelled out as an invariant so a future "optimize away the re-attest"
  change can't reintroduce the bypass.
- **Canonicalization of `argv_digest`/`env_allowlist_digest`.** An under-specified
  canonicalization is a forgeable digest; write it as a golden-vector test before any
  hashing code (arg order, repeated flags, path normalization, env sort + value
  policy).

### Traps considered and rejected

- **Denylist pre-flight as the *primary* mechanism** (scan `.git/config`, refuse on
  known keys, leave git otherwise bare): rejected. It can never prove completeness,
  rots every time git ships a new exec surface, and has a TOCTOU window between scan
  and exec. Retained only as the demoted detector in Layer 3.
- **Cross-lane antigen-presentation / learned-signature ledger:** rejected as premature
  scale for a single-operator system.
- **Dry-run exploit confirmation in a throwaway container:** rejected — adds a
  container dependency and latency to confirm something Layers 1+2 make moot.

## Provenance

This RFC is the design-of-record promotion of the PULL verdict in
`showerthoughts/prior-art-recall.md` (mining of the MIT, fully-local `recall` Claude
Code memory plugin, which faces the identical "host repo is a hostile clone" threat
model). The single open design decision — *how to shape read-side neutralization, and
the pre-flight/quarantine relationship* — was taken via an `/adhd` divergence pass
(5 isolated frames: attacker, 3am-on-call, regulator, remove-the-assumption, biology →
30 ideas → scored/clustered → 3 deepened branches grounded in this codebase). The
reframe that the real axis is "neutralize the config" vs "remove the config from the
code path," and the `GIT_CONFIG_COUNT`-precedence finding, came out of that pass. Patterns
are re-expressed in striatum's own terms; no source code was copied.
