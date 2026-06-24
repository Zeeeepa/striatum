# HOLDER — RFC 0164 P0 falsifiable implementation SPEC, v1 (read-side git neutralization + the gate-evidence recovery contract)

author: holder-author-001

> This is the **fresh v1** falsifiable implementation SPEC for **RFC 0164 P0**,
> published as the claim the two falsifiers re-attack. The RFC's **Decision is
> settled** — the allowlist posture realized as **layered severance** (Layer 1
> config-from-objects severance, Layer 2 born-neutralized environment + bounded
> subcommands, Layer 3 evidence + two-state recovery), the denylist demoted to
> **non-load-bearing telemetry**. I do **not** re-litigate that posture, nor the
> already-solved write-confinement / repo-config-validator SKIPs. I harden the
> design into **build-bearing constraints**: every load-bearing claim is a
> **falsifiable assertion (Ann)** paired with the **named test/corpus row that
> would refute it**, anchored to **verified current-branch source**
> (`go/pkg/...` `file:line`). I re-read every named call site on the
> `striatum/rfc-0164-design` worktree; **§0 records four corrections to the RFC's
> own call-site anchors** that the falsifiers should treat as part of the
> hardened claim, not as errors to re-find. Scope is **P0 only** (§1 fixes the
> boundary precisely and names the Slice 2 / Slice 3 seams P0 leaves).

---

## Coverage map — how this SPEC discharges Goals G1–G4

| Goal | What it demands | Where discharged | Load-bearing assertions |
|------|-----------------|------------------|--------------------------|
| **G1** | No untrusted config/env value reaches command execution under the daemon identity through *any* read-side git invocation — enforced **structurally**, not by a checklist a call site can forget | §2 (chokepoint), §3 (`gitEnv()` omission), §4 (compile-time invariant) | A1–A4, A6–A11, A12–A14 |
| **G2** | Neutralization is **auditable**: the live policy version + closed-env proof are recoverable for any historical call; a gate that silently passes an *unknown* gadget is a faked gate | §8 (canonicalization golden vectors, attestation contract, decay-TOCTOU invariant) — Slice 3 seam, constraints frozen in P0 | A22–A25 |
| **G3** | A tripped repo **recovers** without a code change or irreversible exclusion; a false-positive must not wedge a benign repo | §8.3 (two-state recovery), §7 (false-positive degrades observability only) | A20, A24, A26 |
| **G4** | The hardening is regression-tested by a **planted-attack corpus** that executes under the old bare path and no-ops under the new one | §5 (red-team corpus), §6 (completeness proof routes) | A15–A19 |

The full assertion ledger is **§10** (A1–A28). The P0 boundary + Slice 2/3 seams are **§9**. The build manifest is **§11**.

---

## §0 — Verified source baseline and four corrections to the RFC's call-site anchors

The holder verifies, does not trust. I re-read every named read-side site on this
branch. **Verified true (load-bearing, unchanged from the RFC):**

- `git_snapshot.go`'s `localGit.output` runs `status`/`rev-parse`/`log` with **no
  `cmd.Env`** set — it inherits the daemon's full environment
  (`go/pkg/reads/git_snapshot.go:193-212`, exec at `:200`). It already enforces a
  small subcommand allowlist (`rev-parse`/`status`/`log`) + an arg denylist
  (`validateGitArgs`, `:226-242`) — the seam this SPEC generalizes.
- The commit/apply path injects `-c core.hooksPath=<hooksPath>` on `commit`
  (`go/pkg/mutations/git_commit_apply.go:342-343`, exec at `:345`) — RFC 0127's
  hardening. **But it also sets no `cmd.Env`** (see CORRECTION C-3).
- `verifier/receipt.go`'s `runOnce` builds a **minimal caller-supplied env**
  (`PATH`+`HOME` only, no inheritance — `go/pkg/verifier/receipt.go:556-559`):
  the positive exemplar the `gitEnv()` floor generalizes.
- The build already has an **AST-walking invariant test** to mirror:
  `TestDaemonMutationGitInvocationsDoNotUseCheckoutOrWorkingTreeMerge`
  (`go/pkg/mutations/git_invocation_guard_test.go:13-85`), which parses every
  non-test `*.go` in the package and fails the build on a forbidden git call. §4
  extends exactly this machinery.

**CORRECTION C-1 — `status.go` is in `reads`, not `mutations`.** The RFC anchors
the `branch --show-current` read at `go/pkg/mutations/status.go:388`. The real
site is **`go/pkg/reads/status.go:388`** (`currentGitBranch`, exec at `:388`,
bare, no `cmd.Env`). A chokepoint refactor that greps `mutations/status.go` would
miss it. *(Verified this branch.)*

**CORRECTION C-2 — the RFC's "~6 call sites" is an undercount; the true
read-side bare-git surface is larger, and a grep-backed enumeration is the only
honest exhaustiveness claim.** A tree-wide grep of non-test `exec.Command(git…)`
in the daemon packages (`reads`, `mutations`, `verifier`) yields **11 read/ref
exec sites across 7 files plus 2 funnel helpers**, including **two sites the RFC's
list omits entirely**:

| # | Site | Subcommand | In RFC list? | `cmd.Env`? |
|---|------|-----------|--------------|-----------|
| 1 | `reads/git_snapshot.go:200` (via `localGit.output`) | `status`/`rev-parse`/`log` | yes | **nil** |
| 2 | `reads/doctor_artifact_anchor.go:537` | `cat-file -e` | yes (as `:511`, the *caller*) | **nil** |
| 3 | `reads/doctor_artifact_anchor.go:585` (via `readGitFileBytes`) | `show <commit>:<path>` | yes (as `:559`, the *caller*) | **nil** |
| 4 | `reads/worktree_refs.go:424` | `merge-base --is-ancestor` | yes (as `:393`) | **nil** |
| 5 | `reads/worktree_refs.go:446` (via `readGitOutput`) | `rev-parse`/`for-each-ref` | yes (as `:399`) | **nil** |
| 6 | **`reads/doctor_barrier.go:575`** | `rev-parse --verify --quiet` | **NO** | **nil** |
| 7 | `reads/status.go:388` | `branch --show-current` | yes (mislabeled `mutations/`) | **nil** |
| 8 | **`mutations/write_scope_guard.go:231`** | `status --porcelain=v1 -z` | **NO** | **nil** |
| 9 | `mutations/run.go:920/930/943/956` | `branch --show-current` / `rev-parse --verify HEAD` / `rev-parse --verify refs/heads/…` / `branch <new> <base>` | yes | **nil** |
| 10 | `verifier/receipt.go:606/609` | `add -A` / `write-tree` | yes | set, but **`os.Environ()`-sourced** (C-4) |

Two helpers funnel several of these: **`localGit.output`**
(`git_snapshot.go:193`) and **`readGitOutput`** (`worktree_refs.go:444`, used by
`doctor_artifact_anchor` and `worktree_refs`). The chokepoint refactor (§2) must
route **both helpers and every direct-exec site**; the build-time invariant (§4)
is what makes the enumeration self-checking so a future 12th site cannot regress
silently. **This correction is itself a falsifiable claim: A2 fails if a tree-wide
grep finds a daemon-process read/ref git exec outside the enumerated set + the
sanctioned chokepoint.**

> Also outside the daemon read path but on the same `git` surface, for the
> invariant's scope decision (§4): `agentloop/loop.go:268` (`cmd.Env = childEnv` —
> the **driven agent's lane env**, where the floor must compose, §3.4),
> `agentloop/mcpconfig.go:349` (`rev-parse --git-path info/exclude`, bare),
> `cmd/striatum/operator_bootstrap.go:472` (a CLI probe, bare),
> `mutations/repo_patch.go:186`, `mutations/recovery_quarantine_lane.go:441-443`
> (`cmd.Env = env`), `mutations/worktree.go:1604/1810` (`archive`).

**CORRECTION C-3 — the "hardened" commit path is env-incomplete: it injects `-c
core.hooksPath=` but does NOT pin `cmd.Env`, so by the RFC's *own*
`GIT_CONFIG_COUNT`-precedence finding its `-c` is env-overridable.**
`git_commit_apply.go:329-356` sets `-c core.hooksPath=<empty>` (`:342-343`) but
leaves `cmd.Env` nil (`:345`), inheriting the daemon environment. If the daemon
process's ambient environment ever carries
`GIT_CONFIG_COUNT=1;GIT_CONFIG_KEY_0=core.hooksPath;GIT_CONFIG_VALUE_0=<attacker
dir>`, env config beats `-c` and the hooksPath neutralizer is silently bypassed.
This is **out of P0's read-side scope** but it is the same structural hole, so the
invariant (§4) must reach the commit path too and the commit path must adopt
`gitEnv()` as a **named Slice-2 fast-follow** (§9). Recording it is required by
the product rule "if a doc claim disagrees with source, fix the doc": the RFC
calls this path "hardened"; it is hardened against `-c`-reachable config but **not
against env-injected config**.

**CORRECTION C-4 — a naive "`cmd.Env` is nil" invariant has a real false-negative
in-tree, and `git add`/`write-tree` are gadget-bearing, not pure reads.**
`verifier/receipt.go`'s `CwdTreeSHA` sets `cmd.Env = append(os.Environ(), …)`
(`:605-610`) — it *pins* `cmd.Env` (passes a nil-check) while inheriting the
**entire daemon environment**, including any ambient `GIT_*`/`GIT_CONFIG_COUNT`
gadget. Worse, `git add -A` and `git write-tree` honor **`.gitattributes`
`filter.<driver>.clean`** and **`core.fsmonitor`** — both **code-execution
gadgets** driven by in-repo config, not env. So (a) the §4 invariant cannot be a
nil-check; it must reject `os.Environ()`-sourced env, and (b) the
`add`/`write-tree` sites are closed only by **Layer 1 config severance** (a minted
config with no `filter.*`/`fsmonitor` keys), not by the env floor — they are a
**Slice 2** target, flagged here, and excluded from the §3 read subcommand
allowlist.

---

## §1 — The P0 boundary (the precise floor claim, the tested residual, the seams)

**Decision: P0 = Slice 0 (chokepoint seam) + Slice 1 (`gitEnv()` omission floor +
red-team corpus + compile-time invariant).** I adopt the SEED's framing and make
the floor claim **precise** rather than overstated, because an imprecise "P0
closes the headline exploit" is the first thing a severance-completeness falsifier
pops.

**What P0 closes — by OMISSION (the allowlist posture, structurally):**

1. The entire **ambient `GIT_*`** gadget surface (`GIT_EXTERNAL_DIFF`,
   `GIT_PAGER`, `GIT_SSH_COMMAND`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, …) — never
   present in `gitEnv()`'s closed allowlist.
2. The **`GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n`** family — omitted, which is *the*
   neutralization because env config beats `-c` (RFC, load-bearing).
3. **Global and system** config gadgets — `GIT_CONFIG_NOSYSTEM=1`,
   `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_SYSTEM=/dev/null`, sacrificial
   `HOME`/`XDG_CONFIG_HOME`.
4. The **driven agent's own bare `git`** inheriting any of (1)–(3) — the lane env
   is born-neutralized (§3.4).
5. Structurally: the surface is collapsed to **one chokepoint** (§2) under a
   **compile-time invariant** (§4), so an **unknown future gadget no-ops by
   omission**, never by being on a known-bad list.

**What P0 does NOT yet close — stated honestly, tested as expected-fail:**
**in-repo local `.git/config` / `.gitattributes` exec keys.** The env floor closes
ambient/global/system config; it does **not** reach the repository-scoped local
config that git reads via `-C <repoRoot>`. The live residual, precisely bounded:

- **`core.fsmonitor` on `git status`** — striatum runs `status` at
  `git_snapshot.go:200` and `write_scope_guard.go:231`; an in-repo
  `[core] fsmonitor = <payload>` fires on every status read. This is a **live
  daemon RCE on striatum's own reads after the env floor.** (§1 closes it with a
  demoted interim; Slice 2 closes it by omission.)
- **`diff.external` / `*.textconv` / `filter.*` for the driven agent's arbitrary
  `git`** — the agent can run `git diff`/`log -p`/`blame`/`add` in the hostile
  repo; in-repo `diff.external`/textconv/clean drivers fire. Only Slice 2's
  `clean.git` (the agent's worktree attached to a minted config common-dir) closes
  this by omission.

**Precision correction that narrows the residual:** `core.pager` is **largely
inert** on striatum's read path — git launches the pager only when stdout
`isatty`, and every chokepoint exec captures stdout into a buffer
(`cmd.Stdout = &stdout`, e.g. `git_snapshot.go:203`). Combined with `GIT_PAGER`
omission, in-repo `core.pager` does not fire on captured-output reads. The RFC's
headline "`git log` with `core.pager=<payload>`" overstates this specific vector;
the real in-repo residuals are **fsmonitor (on status)** and **diff.external /
textconv / filter (on the agent's diffs and on `add`/`write-tree`)**.

**P0's one demoted interim (NOT the security boundary).** Because `gitEnv()`
establishes a *closed* environment (the attacker cannot inject `GIT_CONFIG_COUNT`
into our exec), targeted `-c` neutralizers **regain their authority** on our own
invocations. The chokepoint therefore passes `--no-pager -c core.fsmonitor=` on
striatum's own reads to kill the one in-repo-local LIVE RCE the env floor can't
reach (fsmonitor-on-status). This is **explicitly the RFC's demoted denylist** —
non-load-bearing telemetry-grade defense-in-depth, **not** the omission boundary,
and the SPEC marks it as such so a falsifier does not mistake it for the posture.
The omission closure of **all** in-repo config (arbitrary textconv/filter driver
names a fixed `-c` list can never enumerate) is **Slice 2**.

**Recommendation to the gate:** sequence **Slice 2 immediately after P0** and do
**not** treat P0 as complete against in-repo config. P0 is a real, independently
verifiable floor (it closes four whole gadget classes by omission, collapses the
surface to one compile-checked chokepoint, and ships the regression corpus), but
the headline in-repo-config vector is closed only when Slice 2 lands. **Slice 3**
(gate refs + `gate.preflight_attested` + two-state recovery) is the audit+recovery
seam; its *contracts* are frozen in P0 as constraints (§8) so it cannot be built
wrong, but its tests are deferred.

### Falsifiable assertions

- **A0 (floor precision).** P0 neutralizes by omission the ambient-env /
  `GIT_CONFIG_COUNT` / global / system / agent-bare-env classes, and leaves
  exactly the in-repo-local-config residual bounded above. *Refuting test:* the
  corpus (§5) shows a sentinel created for any class P0 claims closed (refutes the
  floor), **or** a sentinel **not** created for the in-repo-config / agent-diff
  rows P0 admits as residual (which would mean the residual is mis-stated — a
  weaker but still-recorded inconsistency).

---

## §2 — Slice 0: the chokepoint seam `CleanRepoFor` (no behavior change)

### Design

Introduce one function and route **every** read-side site (§0 C-2 table) through
it:

```go
// go/pkg/safegit/cleanrepo.go
//
// CleanRepoFor returns the repo root a read-side git invocation must run against
// for lane laneID. In Slice 0 it returns repoRoot UNCHANGED (zero behavior
// change). In Slice 2 it returns a daemon-minted objects-only clean.git path.
// The seam exists so "every read is severed" is a property of ONE function.
func CleanRepoFor(repoRoot, laneID string) (cleanRepoRoot string, err error)
```

- **Slice 0 contract:** `CleanRepoFor(r, l) == (r, nil)` for all inputs. The
  refactor is mechanical: each of the 11 sites computes `root, err :=
  CleanRepoFor(repoRoot, laneID)` and uses `root` in place of the bare
  `repoRoot`. The two funnel helpers (`localGit.output`, `readGitOutput`) take the
  cleaned root once; their callers stop passing the raw root.
- **Reviewable in isolation:** because the return is identity, Slice 0 changes no
  behavior and no test fixture — it is a pure surface-collapse, diff-reviewable
  against the §0 C-2 enumeration.
- **Exhaustiveness is grep-backed, then compile-enforced.** The enumeration is the
  §0 C-2 table; the §4 invariant turns it into a build failure if any daemon-
  process read/ref git exec is neither inside the chokepoint nor on the sanctioned
  list. The list is not trusted prose — it is the test's allowlist.

### Falsifiable assertions

- **A1 (single chokepoint).** Every read-side git invocation in `reads` +
  the read/ref invocations in `mutations`/`verifier` obtains its repo root from
  `CleanRepoFor`. *Refuting test:* `chokepoint_routing_test` (a static/AST check,
  §4 sibling) asserts no enumerated site calls `exec.Command("git", …)` /
  `localGit.output` / `readGitOutput` with a root not sourced from `CleanRepoFor`.
  A bypassing site refutes A1.
- **A2 (exhaustive enumeration).** The §0 C-2 set is the complete daemon-process
  read/ref git surface. *Refuting test:* `git_surface_enumeration_test` greps the
  daemon packages for `exec.Command(Context)?("git"|g.path, …)` and diffs against
  the sanctioned allowlist; any unlisted site fails the build (this is the
  same mechanism as A12). A site found outside the set refutes A2.
- **A3 (Slice 0 is behavior-neutral).** `CleanRepoFor(r,l) == (r,nil)` in Slice 0.
  *Refuting test:* `cleanrepo_identity_test` asserts identity over a fixture
  matrix; the full existing `reads`/`mutations` suites stay green unchanged. A
  behavior delta refutes A3.

---

## §3 — Slice 1a: `gitEnv()` — the born-neutralized closed environment (Layer 2)

### Design

```go
// go/pkg/safegit/gitenv.go
//
// gitEnv returns a fully-pinned, closed environment for a read-side git spawn.
// It is NEVER os.Environ()-derived. It REFUSES (typed error) rather than
// degrading to a bare environment.
func gitEnv() ([]string, error)
```

`gitEnv()` returns exactly (and only) this closed allowlist:

```
PATH=<daemon-pinned minimal PATH>          # resolve the git binary only
GIT_CONFIG_NOSYSTEM=1                       # ignore /etc/gitconfig
GIT_CONFIG_GLOBAL=/dev/null                 # no ~/.gitconfig
GIT_CONFIG_SYSTEM=/dev/null                 # belt-and-braces with NOSYSTEM
HOME=<sacrificial empty daemon-owned dir>   # no ~/.gitconfig, ~/.ssh discovery
XDG_CONFIG_HOME=<same sacrificial dir>      # no ~/.config/git/config
GIT_TERMINAL_PROMPT=0                        # never block on credential prompts
LANG=C LC_ALL=C                             # deterministic output for digests (§8)
TZ=UTC                                       # deterministic timestamps
```

It enforces, by construction:

- **OMISSION of the `GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n`
  family** — never added. This is the load-bearing neutralization (env beats
  `-c`); omission *is* the closure.
- **OMISSION of every gadget env var** — `GIT_EXTERNAL_DIFF`, `GIT_PAGER`,
  `GIT_SSH_COMMAND`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_DIR`,
  `GIT_WORK_TREE`, `GIT_PROXY_COMMAND`, `GIT_ASKPASS`, `SSH_ASKPASS`, … — by
  building the list from a closed allowlist, not by subtracting from
  `os.Environ()` (a subtract-list is a denylist that rots).
- **The sacrificial dir is validated.** If `HOME`/`XDG` cannot be established as a
  stat-able empty daemon-owned dir, `gitEnv()` returns a typed
  **`ErrGitEnvUnavailable`** (§3.2). It **never** returns `os.Environ()` and the
  caller **never** falls back to bare `exec`.

### 3.1 The bounded subcommand allowlist

A read-side spawn is refused **before exec** unless its subcommand is in the
closed set, with pinned flags:

```
log, show, cat-file, rev-parse, status, for-each-ref, merge-base,
diff-tree, branch (only with --show-current)
```

`add` and `write-tree` (`receipt.go:606/609`) are **excluded** — they mutate the
index and honor `.gitattributes` clean/`fsmonitor` gadgets (C-4); they are routed
through the Slice-2 minted config, not the read allowlist. Anything outside the
set (including `fetch`/`pull`/`push`/`remote`/`clone`/`submodule`) is refused with
a typed error. This subsumes and generalizes the existing
`git_snapshot.go:226-242` allowlist.

### 3.2 Refuse, never degrade

```go
var ErrGitEnvUnavailable = errors.New("safegit: closed git environment unavailable; refusing bare exec")
```

If the closed env cannot be built, or the subcommand is not allowlisted, the
chokepoint returns the typed error and the **caller surfaces it** (the read fails
closed). There is **no** `exec.Command` path that runs with a `nil` or
`os.Environ()` env. ("Never fake a gate" — G1.)

### 3.3 The one demoted `-c` interim (§1)

On its own captured-output reads only, the chokepoint additionally passes
`--no-pager -c core.fsmonitor=` (the §1 interim, demoted/non-boundary). This is
the *only* `-c`/flag neutralizer in P0; it exists because the closed env makes
`-c` reliable, and it dies for the agent's arbitrary git (which P0 cannot wrap
flag-by-flag) and for arbitrary in-repo keys — both of which Slice 2 closes by
omission.

### 3.4 The driven agent's lane env

`gitEnv()`'s pins are composed into the agent's lane environment at
`agentloop/loop.go:268` (`cmd.Env = childEnv`): `childEnv` must include
`GIT_CONFIG_NOSYSTEM`/`GLOBAL`/`SYSTEM`, sacrificial `HOME`/`XDG`, and **omit** the
`GIT_CONFIG_COUNT` family and gadget vars — so a socially-engineered bare `git`
(via a hostile `AGENTS.md`/README) is **born-neutralized** at the env layer. (It
is still exposed to in-repo local config until Slice 2 attaches the lane worktree
to `clean.git`; §1 residual, §9 seam.)

### Falsifiable assertions

- **A6 (closed allowlist, not a subtraction).** `gitEnv()`'s output is exactly the
  pinned allowlist and contains no `os.Environ()` value. *Refuting test:*
  `gitenv_closed_test` sets `GIT_PAGER`, `GIT_EXTERNAL_DIFF`, `GIT_CONFIG_COUNT=…`,
  `LD_PRELOAD`, and a junk var in the process env, calls `gitEnv()`, and asserts
  **none** appear in the result and the result equals the pinned set. Any leak
  refutes A6.
- **A7 (`GIT_CONFIG_COUNT` family omitted).** No
  `GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n` is ever present. *Refuting test:* the
  corpus row `env_config_count_pager` (§5) runs an allowlisted read with
  `cmd.Env = gitEnv()` against a repo while the process env sets the family; assert
  the sentinel is never created. Sentinel creation refutes A7.
- **A8 (refuse, never degrade).** When the sacrificial dir is unstat-able,
  `gitEnv()` returns `ErrGitEnvUnavailable` and **no** git runs. *Refuting test:*
  `gitenv_refuses_test` points the sacrificial dir at a non-existent path; assert
  the typed error and that `exec.Command` is never reached (no bare fallback). A
  bare exec refutes A8.
- **A9 (bounded subcommands).** A non-allowlisted subcommand is refused before
  exec. *Refuting test:* `gitenv_subcommand_allowlist_test` drives `fetch`,
  `remote`, `submodule`, `add`; assert each is refused pre-exec. A spawn refutes
  A9.
- **A10 (lane env born-neutralized).** The agent's `childEnv` omits the
  `GIT_CONFIG_COUNT` family + gadget vars and pins NOSYSTEM/GLOBAL/SYSTEM.
  *Refuting test:* `lane_env_neutralized_test` asserts the composed `childEnv` at
  `loop.go:268` matches the floor; a present gadget var or missing pin refutes A10.
- **A11 (`add`/`write-tree` excluded from reads).** The read allowlist excludes
  index-mutating, filter-honoring subcommands. *Refuting test:* asserting
  `add`/`write-tree` are refused by the read chokepoint; inclusion refutes A11.

---

## §4 — Slice 1b: the compile-time invariant (every git call is neutralized, by the build)

### Design

Extend the existing AST guard (`git_invocation_guard_test.go:13-85`) into
`TestDaemonGitInvocationsAreNeutralized`, run over the daemon packages
(`reads`, `mutations`, `verifier`, and the agent-lane spawn in `agentloop`). For
each `*.go` (non-test), it parses the AST and flags any `git`-spawning
`exec.Command`/`exec.CommandContext` (first arg `"git"` or the `localGit.path`
receiver) that is **not** one of:

1. **inside the sanctioned chokepoint** (`safegit` package functions), or
2. a call whose `cmd.Env` is assigned from a **`gitEnv()`-derived** value
   (an AST data-flow check: the nearest `cmd.Env = X` where `X` traces to a
   `safegit.gitEnv()` / `safegit.LaneEnv()` call), **and** is **not**
   `os.Environ()`-sourced.

Because of C-4, the check is **not** a nil-test: a site that sets
`cmd.Env = os.Environ()` or `cmd.Env = append(os.Environ(), …)` **fails** the
build (this exact pattern exists today at `receipt.go:605`, so the invariant lands
red and forces the fix). The sanctioned-site **allowlist is the §0 C-2 table** —
adding a 12th read site without routing it fails the build (this is also A2/A12).

### Falsifiable assertions

- **A12 (compile-time completeness).** No daemon-process `git` exec runs with a
  `nil`, `os.Environ()`-sourced, or non-`gitEnv()` env outside the chokepoint.
  *Refuting test:* `TestDaemonGitInvocationsAreNeutralized` itself; a planted
  `exec.Command("git","log")` with bare/`os.Environ()` env must fail the build. If
  the test passes with such a plant, A12 is refuted.
- **A13 (os.Environ() ban is enforced, not just nil).** The invariant rejects an
  `os.Environ()`-sourced `cmd.Env`. *Refuting test:* a fixture file under the test
  with `cmd.Env = append(os.Environ(), "X=1")` must be flagged. Passing refutes
  A13.
- **A14 (commit path is in scope).** The invariant covers
  `git_commit_apply.go:345` (C-3), so the commit path must adopt `gitEnv()` (Slice
  2 fast-follow) or the build stays red. *Refuting test:* the invariant run over
  `mutations` flags `git_commit_apply.go:345` until it routes through the floor;
  green-while-bare refutes A14.

---

## §5 — Slice 1c: the red-team corpus (the certificate, G4)

### Design

`go/pkg/reads/gate_corpus_test.go` — a table test, one row per gadget. Each row
builds a temp repo / process-env condition planting a payload that **creates a
sentinel file** (`touch <sentinel>`), runs a **real allowlisted subcommand** with
`cmd.Env = gitEnv()` (and, for the agent rows, the lane env), and asserts the
sentinel **was never created**. The green-result hash is **`corpus_green_hash`**
(consumed by the Slice-3 attestation, §8).

| Row | Plant | Subcommand | P0 expectation | Closed by |
|-----|-------|-----------|----------------|-----------|
| `env_pager` | process `GIT_PAGER='touch S'` | `log` | **pass** (sentinel absent) | L2 omission + capture |
| `env_external_diff` | process `GIT_EXTERNAL_DIFF='touch S'` | `diff-tree` | **pass** | L2 omission |
| `env_ssh_command` | process `GIT_SSH_COMMAND='touch S'` | `rev-parse` | **pass** | L2 omission |
| `env_config_count_pager` | process `GIT_CONFIG_COUNT=1;KEY_0=core.pager;VALUE_0='touch S'` | `log` | **pass** | L2 omission (A7) |
| `global_gitconfig_pager` | `HOME/.gitconfig [core] pager='touch S'` | `log` | **pass** | L2 `GLOBAL=/dev/null` + sacrificial HOME |
| `system_gitconfig_fsmonitor` | `/etc`-style via `GIT_CONFIG_SYSTEM` plant | `status` | **pass** | L2 `NOSYSTEM`/`SYSTEM=/dev/null` |
| `inrepo_config_fsmonitor` | in-repo `.git/config [core] fsmonitor='touch S'` | `status` | **pass in P0** | the §3.3 demoted `-c core.fsmonitor=` interim |
| `inrepo_config_diff_external` | in-repo `.git/config [diff] external='touch S'` | `diff-tree`/agent `git diff` | **EXPECTED-FAIL vs L2 alone** | **Slice 2** clean.git (omission) |
| `inrepo_attributes_textconv` | in-repo `.gitattributes` + `[diff "x"] textconv='touch S'` | agent `git show`/`log -p` | **EXPECTED-FAIL vs L2 alone** | **Slice 2** clean.git (no driver) |
| `inrepo_filter_clean` | in-repo `.gitattributes` + `[filter "x"] clean='touch S'` | `add`/`write-tree` | **EXPECTED-FAIL vs L2 alone** | **Slice 2** clean.git (no driver) |
| `agent_bare_git_diff` | hostile in-repo `diff.external` + agent runs bare `git diff` in lane env | lane `git diff` | **EXPECTED-FAIL vs L2 alone** | **Slice 2** lane worktree → clean.git |

The **expected-fail rows are first-class assertions**: they encode *exactly* what
Layer 2 cannot do, so the layering is a tested claim, not prose. When Slice 2
lands, each flips to **expected-pass** in the same table (a one-line change),
which is the Slice-2 acceptance gate.

### Falsifiable assertions

- **A15 (env/global/system gadgets no-op).** Rows `env_*`, `global_*`, `system_*`
  create no sentinel. *Refuting test:* the rows themselves. A sentinel refutes A15
  and the floor.
- **A16 (the `GIT_CONFIG_COUNT` precedence is actually closed).** `env_config_count_pager`
  creates no sentinel. *Refuting test:* the row. A sentinel refutes A16 (and A7).
- **A17 (the demoted interim kills fsmonitor-on-status in P0).**
  `inrepo_config_fsmonitor` creates no sentinel. *Refuting test:* the row; a
  sentinel refutes A17 and means the §1 interim is insufficient (escalates the
  recommendation to pull Slice 2 into P0).
- **A18 (the residual is exactly the in-repo/agent rows).** Rows
  `inrepo_config_diff_external`, `inrepo_attributes_textconv`,
  `inrepo_filter_clean`, `agent_bare_git_diff` are **expected-fail** under L2
  alone and **expected-pass** under Slice 2. *Refuting test:* the rows as
  expected-fail in P0; an unexpected *pass* under L2 alone means the residual is
  mis-modeled (records a discrepancy), an unexpected *fail* under Slice 2 refutes
  the Slice-2 closure.
- **A19 (`corpus_green_hash` is deterministic).** The green-result hash is stable
  across runs (LANG/LC_ALL/TZ pinned, §3). *Refuting test:* run twice; a differing
  hash refutes A19 (and undermines the Slice-3 certificate).

---

## §6 — Hard core PROOF I: severance is COMPLETE (inert by omission)

The completeness claim is structural, discharged route-by-route. A
severance-completeness falsifier must find a route by which a gadget still reaches
exec; each known route is closed and named:

| Route to exec | Closed by | Assertion |
|---------------|-----------|-----------|
| An **unrouted read call site** | §2 chokepoint + §4 compile-time invariant (the enumeration is the test's allowlist; a new site fails the build) | A1, A2, A12 |
| The **agent's own bare `git`** honoring env/global gadgets | §3.4 lane env (born-neutralized) | A10 |
| The **`GIT_CONFIG_COUNT` family** beating `-c` | §3 omission (the family is never in the closed env) | A7, A16 |
| **Ambient `GIT_*` gadget vars** | §3 closed allowlist (built up, not subtracted) | A6 |
| **Global/system config** gadgets | §3 `NOSYSTEM`/`GLOBAL`/`SYSTEM`/sacrificial `HOME`/`XDG` | A6, A15 |
| An **unenforced subcommand** (e.g. `fetch`, `submodule`) | §3.1 bounded allowlist, refused pre-exec | A9 |
| A **bare-exec fallback** when the closed env can't be built | §3.2 typed `ErrGitEnvUnavailable`, no fallback | A8 |
| **`add`/`write-tree`** honoring `.gitattributes` filters | §3.1 exclusion + Slice-2 minted config | A11, A18 |
| An **unknown future gadget** (no known-bad entry) | **omission**: it is not in the closed env and its call site is chokepointed under the invariant, so it no-ops without being recognized | A0, A6, A12 |

**The omission property is the whole game and it is testable.** Because the
environment is an *allowlist* (built from a closed set) and the call surface is a
*single compile-checked chokepoint*, a gadget that did not exist when this code was
written is inert with **zero** code change: it is absent from the env and its call
site cannot escape the chokepoint. The corpus's `env_*` rows demonstrate this for
known gadgets; the invariant guarantees it for unknown ones by construction.

**Honestly bounded residual (the one place omission is NOT yet achieved in P0):**
**in-repo local config**, which the env floor cannot reach and the §3.3 interim
closes only for the single `fsmonitor`-on-status key by a *demoted denylist*. The
omission closure of in-repo config is **Slice 2** (§9). This residual is **A18**'s
expected-fail rows — surfaced, not hidden.

### Falsifiable assertion

- **A20 (no silent unknown-pass).** No read path executes an in-repo gadget
  without either neutralizing it (P0 env floor / Slice-2 clean.git) or refusing
  (unknown subcommand) — i.e., nothing **silently** passes. *Refuting test:* the
  corpus is the closed enumeration; the Slice-3 `doctor` barrier (§8.2) flags any
  `git.*` exec line lacking a preceding `gate.preflight_attested`. A silent pass
  refutes A20 (and G2).

---

## §7 — Hard core PROOF II: severance is CORRECT (wrong answers, not just safety)

The CORRECT property is "the neutralization never yields a **wrong answer**
(a truncated graph that silently resolves `merge-base`/`rev-parse`), only correct
answers or honest errors."

**The decisive P0 fact: P0 has NO truncated-graph risk, because Slice 0+1 do not
sever the object graph.** `CleanRepoFor` returns `repoRoot` unchanged in Slice 0,
so every read runs `-C <repoRoot>` against the **real** repository with its full
objects, packed/loose refs, alternates, `commit-graph`, and `multi-pack-index`
intact. P0 changes only the **environment** (ambient/global/system config + gadget
vars) and adds the one `-c core.fsmonitor=`/`--no-pager` interim. None of these
alter query *results*:

- `core.pager`/`GIT_PAGER` affect **output paging**, not content (and are inert on
  captured stdout anyway).
- `diff.external`/textconv affect **diff rendering**, not `rev-parse`/`merge-base`/
  `for-each-ref`/`log --format=%H` results.
- `core.fsmonitor` is a **performance accelerator**; disabling it yields identical
  status results (it changes speed, not the porcelain output).
- The **in-repo local config is still read** (we run against the real repo), so
  benign local keys (`core.ignorecase`, `core.quotepath`, i18n) are preserved —
  P0 does **not** strip them. This is a correctness *safety* of the env-floor
  approach: only global/system/env are neutralized.

So in P0 the CORRECT property reduces to: **does the env floor + the one interim
change any read's answer?** It must not.

**The truncated-graph risk is entirely a Slice 2 concern** (minted objects-only
`clean.git`). I therefore make it a **build-bearing Slice-2 gate, specified now**:

- **The parity harness (Slice-2 acceptance gate).** Before `clean.git` replaces the
  bare path, a corpus proves `clean.git` reads **equal** bare-repo reads over a
  matrix: repos with `objects/info/alternates`, multi-pack indexes, packed-refs,
  per-worktree refs (`worktrees/<id>/refs`, `commondir`), shallow clones,
  `core.ignorecase`, gitdir-relative `core.worktree`, and the exact subcommands the
  chokepoint runs (`merge-base --is-ancestor`, `rev-parse --verify`,
  `for-each-ref`, `log --format=%H`, `cat-file -e`, `show <c>:<p>`). The minting
  step **resolves alternates and carries all ref scopes**; the "tiny structural
  allowlist" of benign keys is **discovered empirically against the harness, not
  guessed**.

### Falsifiable assertions

- **A21 (P0 answer-equivalence).** For every chokepoint subcommand, the answer
  under `gitEnv()` + the §3.3 interim **equals** the answer under the prior bare
  path, over a benign-repo corpus. *Refuting test:* `read_parity_p0_test` runs each
  subcommand both ways over the benign matrix and asserts byte-equal stdout. A
  divergence refutes A21 (a correctness regression in P0 itself).
- **A21b (Slice-2 parity gate is named and binding).** `clean.git` does not replace
  the bare path until `read_parity_clean_git_test` (the alternates/refs/benign-key
  matrix) is green. *Refuting test:* the Slice-2 PR landing `clean.git` without a
  green parity harness refutes A21b. (Build-bearing seam, §9.)

---

## §8 — Discharged open design points (Slice-3 contracts frozen in P0)

Slice 3 (gate refs, attestation, two-state recovery) is post-P0, but its
**contracts are frozen here** so it cannot be built forgeable. Tests are deferred
to Slice 3; the *specifications* are P0 constraints.

### 8.1 Canonicalization golden vectors (before any hashing)

`argv_digest`, `env_allowlist_digest`, and `config_fingerprint` are specified as
**golden vectors committed before any hashing code**, because an under-specified
canonicalization is a forgeable digest:

- **`argv_digest`** = `sha256` over a canonical encoding of the argv vector:
  length-prefixed, NUL-joined, **order-preserving** (argv order is semantic;
  repeated flags are kept in order, not deduped/sorted), with **no** path
  normalization of operands (a path is a value, not normalized) but the git binary
  path canonicalized to its basename `git` (so absolute-path differences don't
  change the digest).
- **`env_allowlist_digest`** = `sha256` over the **sorted** `KEY=VALUE` lines of
  the closed `gitEnv()` set (env has no order semantics → sort for determinism),
  with the sacrificial `HOME`/`XDG`/`PATH` **values redacted to a fixed token**
  (their concrete paths are daemon-instance noise, not policy) and all other
  values verbatim.
- **`config_fingerprint`** = `sha256` over the canonical serialization of the
  in-repo config keys *observed* (for the demoted detector/telemetry), sorted
  `section.key=value`, lowercased section/key per git's own rules, value verbatim.

A `golden_vectors_test` pins ≥3 hand-computed vectors per digest (including the
adversarial cases: repeated `--format` flags, a key differing only in case, an
operand containing `=`/NUL). **Any hashing code is written against these frozen
vectors**, never the reverse.

### 8.2 The attestation contract + the decay-TOCTOU invariant

- **Per-call attestation (Slice 3).** Before git forks, append one KV
  `gate.preflight_attested{neutralizer_set@vN, set_hash, corpus_green_hash,
  argv_digest, env_allowlist_digest, config_fingerprint}`. A `git.*` exec line with
  **no** immediately-preceding `gate.preflight_attested` is a hard `doctor` barrier
  failure (you cannot back-date a KV append, so a clean run cannot be
  retroactively forged).
- **The decay-TOCTOU invariant (frozen now, A23).** Fingerprint decay may
  **clear the blocker** but must **never authorize the next call** —
  authorization always **re-attests at exec time**. Stated as an invariant so a
  future "optimize away the re-attest" change cannot reintroduce the bypass
  (present a clean config to the sweep, re-inject before the next fork).

### 8.3 The two-state recovery split (G3)

- **Recognized-and-neutralized** gadget → a **machine-clearable**
  `gate.read_gadget_detected` blocker pinning the offending `config_fingerprint`.
  `recovery.sweep` recomputes the live fingerprint each pass and, when it no longer
  matches, appends `gate.read_gadget_cleared` and deletes the pin — **idempotent
  decay, no human, no `un-quarantine` verb.** A repo that scrubs its hostile config
  self-heals.
- **Unknown / unattested** key (no allowlist entry, no green-corpus coverage) →
  **hard refusal** into the **existing human-cleared** `recovery.quarantine_lane`
  (`refs/striatum/recovery/`, cleared by `recovery accept-quarantined`). A silent
  pass of the unknown is the faked gate.
- Both grep under one `refs/striatum/` prefix; gate refs live under
  `refs/striatum/gate/<run>/<job>/<attempt>` and are excluded from integrate/fan-in
  sweeps via `NOT LIKE 'refs/striatum/gate/%'`.

### Falsifiable assertions

- **A22 (unforgeable canonicalization).** No two semantically-different
  argv/env/config pairs share a digest, and the adversarial golden cases hold.
  *Refuting test (Slice 3):* `golden_vectors_test`; a collision on the adversarial
  cases (case-only key diff, repeated flag, `=`/NUL operand) refutes A22.
- **A23 (decay never authorizes).** A cleared blocker does not authorize the next
  call; exec re-attests. *Refuting test (Slice 3):* `decay_toctou_test` — clear the
  blocker, re-inject the gadget, fork; assert the fork re-attests and re-blocks. An
  authorized fork on a stale clear refutes A23.
- **A24 (no silent unknown-pass; correct clearer-of-record).** Recognized →
  machine-decay; unknown → human-cleared; neither silently passes. *Refuting test
  (Slice 3):* `two_state_recovery_test` drives a known and an unknown gadget;
  a known one routed to the human lane, or an unknown one machine-cleared or
  passed, refutes A24.
- **A25 (attestation precedes every exec).** Every `git.*` exec has an immediately
  preceding `gate.preflight_attested`. *Refuting test (Slice 3):* the `doctor`
  barrier over a synthetic event log with a missing attestation must flag it. A
  pass refutes A25.

---

## §9 — P0 boundary, named seams, and Non-Goals honored

**In P0 (Slice 0 + Slice 1):** the chokepoint seam (`CleanRepoFor` ≡ identity),
the `gitEnv()` omission floor + the one demoted `fsmonitor`/`--no-pager` interim,
the lane-env neutralization, the red-team corpus (with expected-fail residual
rows), and the compile-time invariant.

**Named build-bearing seams P0 leaves:**

- **Slice 2 (Layer 1, recommended to immediately follow).** `CleanRepoFor` mints &
  caches the objects-only `clean.git` (hardlink-shared objects + all ref scopes +
  resolved alternates + read-only accelerators; minted config from a fixed
  template). Flips the §5 expected-fail rows to expected-pass. **Gated on the §7
  parity harness (A21b).** Also adopts `gitEnv()` on the commit path
  (`git_commit_apply.go:345`, C-3 fast-follow) and routes `add`/`write-tree`
  (`receipt.go:606/609`, C-4) + `archive` (`worktree.go:1810`) through the minted
  config so filter/fsmonitor gadgets are closed by omission.
- **Slice 3 (Layer 3).** `refs/striatum/gate/` subtree, `gate.preflight_attested`,
  the two-state machine-decay/human-accept recovery, and the `doctor` barrier — all
  consuming the §8 frozen contracts (golden vectors, decay-TOCTOU invariant).

**Non-Goals honored (not rebuilt, not re-litigated):**

- **No agent arbitrary-command sandbox** — that is RFC 0096; this SPEC narrows to
  git. (The lane-env neutralization §3.4 is git-specific, not a general sandbox.)
- **No write-confinement rebuild and no repo-config validator** — `ValidateSandboxJail`
  already confines writes; striatum ingests no repo-shipped config (RFC 0127). Both
  remain SKIPs.
- **No cross-lane learned immunity** — the rejected single-operator-scale trap. The
  corpus is per-build, not a shared signature ledger.
- **Local-first boundary intact** — `clean.git`, gate refs, and recovery are all
  daemon-owned local state under `$STATE`/`refs/striatum/`; no hosted service,
  telemetry export, or external persistence.

---

## §10 — Assertion ledger (the falsifiers' target list)

| A# | Claim | Refuting test/row |
|------|-------|-------------------|
| **A0** | P0 floor is precisely the env/global/system/agent-env classes by omission; in-repo-local is the tested residual | §5 corpus row expectations |
| **A1** | Single chokepoint: every read sources its root from `CleanRepoFor` | `chokepoint_routing_test` |
| **A2** | The §0 C-2 enumeration is the complete daemon read/ref git surface | `git_surface_enumeration_test` |
| **A3** | Slice 0 is behavior-neutral (`CleanRepoFor` ≡ identity) | `cleanrepo_identity_test` + existing suites |
| **A6** | `gitEnv()` is a closed allowlist, no `os.Environ()` leak | `gitenv_closed_test` |
| **A7** | `GIT_CONFIG_COUNT` family omitted | `env_config_count_pager` row |
| **A8** | Refuse (typed error), never bare-exec fallback | `gitenv_refuses_test` |
| **A9** | Bounded subcommand allowlist, refused pre-exec | `gitenv_subcommand_allowlist_test` |
| **A10** | Lane env born-neutralized | `lane_env_neutralized_test` |
| **A11** | `add`/`write-tree` excluded from reads | read-chokepoint refusal test |
| **A12** | Compile-time completeness (no bare/`os.Environ()` git exec outside chokepoint) | `TestDaemonGitInvocationsAreNeutralized` |
| **A13** | `os.Environ()`-sourced env is rejected, not just nil | invariant fixture (C-4 pattern) |
| **A14** | Commit path is in invariant scope (C-3) | invariant over `mutations` |
| **A15** | Env/global/system gadgets no-op | `env_*`/`global_*`/`system_*` rows |
| **A16** | `GIT_CONFIG_COUNT` precedence actually closed | `env_config_count_pager` row |
| **A17** | Demoted interim kills fsmonitor-on-status in P0 | `inrepo_config_fsmonitor` row |
| **A18** | The residual is exactly the in-repo/agent rows (expected-fail vs L2, expected-pass vs Slice 2) | `inrepo_*` / `agent_bare_git_diff` rows |
| **A19** | `corpus_green_hash` deterministic | corpus run-twice |
| **A20** | No silent unknown-pass | corpus closure + Slice-3 `doctor` barrier |
| **A21** | P0 answer-equivalence (env floor changes no result) | `read_parity_p0_test` |
| **A21b** | Slice-2 `clean.git` gated on a green parity harness | Slice-2 landing PR |
| **A22** | Unforgeable canonicalization (golden vectors) | `golden_vectors_test` (Slice 3) |
| **A23** | Decay clears the blocker but never authorizes the next call | `decay_toctou_test` (Slice 3) |
| **A24** | Two-state recovery never silently passes an unknown | `two_state_recovery_test` (Slice 3) |
| **A25** | Attestation precedes every exec | `doctor` barrier (Slice 3) |
| **A26** | A benign `[alias]`/`[pager]` false-positive degrades observability only, never wedges the run (Layers 1+2 already inert the config) | `false_positive_benign_test` |

---

## §11 — Build manifest (P0)

| Artifact | File | ~LOC | Tests |
|----------|------|------|-------|
| Chokepoint seam | `go/pkg/safegit/cleanrepo.go` (`CleanRepoFor`) | ~25 (identity in Slice 0) | `cleanrepo_identity_test`, `chokepoint_routing_test` |
| Closed env | `go/pkg/safegit/gitenv.go` (`gitEnv`, `ErrGitEnvUnavailable`, subcommand allowlist) | ~60 | `gitenv_closed_test`, `gitenv_refuses_test`, `gitenv_subcommand_allowlist_test` |
| Lane-env compose | edit `go/pkg/agentloop/loop.go:268` (childEnv) | ~10 | `lane_env_neutralized_test` |
| Call-site routing | edits at the §0 C-2 11 sites + 2 funnel helpers (`git_snapshot.go`, `doctor_artifact_anchor.go`, `worktree_refs.go`, `doctor_barrier.go`, `reads/status.go`, `write_scope_guard.go`, `run.go`, `receipt.go`) | ~40 | existing suites stay green (A3) |
| Compile-time invariant | `go/pkg/.../git_neutralized_guard_test.go` (extends `git_invocation_guard_test.go:13-85`) | ~90 | `TestDaemonGitInvocationsAreNeutralized` |
| Red-team corpus | `go/pkg/reads/gate_corpus_test.go` | ~220 | the §5 row table + `corpus_green_hash` |
| P0 parity | `go/pkg/reads/read_parity_p0_test.go` | ~80 | `read_parity_p0_test` (A21) |
| Golden vectors (contract, consumed Slice 3) | `go/pkg/safegit/golden_vectors_test.go` | ~60 | `golden_vectors_test` (A22) |

**This is the published v1 claim the falsifiers re-attack.** Falsifier 1
(severance-completeness): attack §6's route table, the §0 C-2 exhaustiveness, the
lane-env closure (§3.4), and whether the §1 residual is correctly bounded (did I
miss an in-repo exec key beyond fsmonitor on striatum's own reads? is `core.pager`
really inert under capture? does an allowlisted subcommand have a content-bearing
mode I missed?). Falsifier 2 (evidence/recovery): attack §8 — is the
canonicalization forgeable (the redaction of `HOME`/`XDG`/`PATH` values; the
operand-with-NUL case)? is the decay-TOCTOU invariant actually closed by
re-attest? can an unknown pass via a config_fingerprint collision? does
`corpus_green_hash` certify (is the green hash load-bearing or decorative)? does a
benign `[alias]`/`[pager]` false-positive degrade observability only (A26)?
