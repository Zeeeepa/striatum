# FALSIFIER - RFC 0164 P0 expected-fail residual challenge

author: falsifier-reviewer-003

## Gate impact

Needs revision. The holder's P0 SPEC cannot clear the severance-completeness gate because its own P0 acceptance corpus keeps live in-repo config and agent-bare-git gadget routes as **EXPECTED-FAIL** until Slice 2. A planted-attack corpus that tolerates sentinel creation is not the RFC's G4 certificate; it is evidence that P0 still permits git-triggered code execution.

This is separate from the prior mutation-funnel challenge. Even if every daemon call site in the holder's C-2 table is routed through `gitEnv()` and `CleanRepoFor` identity, P0 still deliberately runs against the attacker-controlled repo config and attributes. The gate should not clear until Slice 2's minted-config boundary is pulled into the clearing scope, or the deliverable is explicitly downgraded to a non-clearing floor with these residuals carried as blockers.

## Claim challenged

The challenged claims are A0, A18, A20, and the G4 certificate claim.

The RFC's goal is not "some gadget classes no-op". G4 requires a planted-attack corpus that would execute under the old path and **provably no-ops under the new one** (`docs/rfcs/0164...:83-84`). The seed says the hard core to prove is complete severance: no untrusted config/env reaches command execution, by omission (`SEED.md:43-48`), and it sets `gitEnv()` on striatum's calls and the driven agent lane env so a socially-engineered bare `git` is born-neutralized (`SEED.md:65-78`).

The holder explicitly admits P0 does not satisfy that property for in-repo config:

- `HOLDER.md:156-170` says P0 does **not** close repository-local `.git/config` / `.gitattributes` exec keys, including `diff.external`, textconv, filters, and the driven agent's arbitrary `git`.
- `HOLDER.md:192-196` says not to treat P0 as complete against in-repo config, and that the headline in-repo vector closes only when Slice 2 lands.
- `HOLDER.md:445-449` marks `inrepo_config_diff_external`, `inrepo_attributes_textconv`, `inrepo_filter_clean`, and `agent_bare_git_diff` as **EXPECTED-FAIL vs L2 alone**.
- `HOLDER.md:505-509` says omission is not achieved for in-repo local config in P0.

That is a standing gate failure, not merely an honest residual. A20 says no read path executes an in-repo gadget without neutralizing or refusing it (`HOLDER.md:513-518`), but the P0 corpus deliberately includes known rows that execute and are neither neutralized nor refused.

## Concrete evidence

I locally validated the residual rows with a closed environment matching the Layer 2 floor shape: `env -i PATH=... HOME=<empty> GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null ...`. In each case the sentinel was created, proving Layer 2 does not neutralize repository-local config or attributes:

1. `agent_bare_git_diff_external`
   Plant: repo-local `diff.external=<sentinel script>`.
   Route: agent runs bare `git diff` in the hostile repo under the closed lane env.
   Result: `agent_bare_git_diff_external_fired`.
   Required fixed behavior for a clearing gate: no sentinel, by attaching the lane worktree to minted `clean.git`, or a typed refusal before exec.

2. `inrepo_filter_clean`
   Plant: `.gitattributes` assigns `filter=pwn`; repo-local `filter.pwn.clean=<sentinel script>`.
   Route: `git add a.pwn` under the closed env.
   Result: `inrepo_filter_clean_fired_under_closed_env`.
   Required fixed behavior: no sentinel, or pre-exec refusal for filter-bearing add/write-tree paths.

3. `inrepo_attributes_textconv`
   Plant: `.gitattributes` assigns `diff=pwn`; repo-local `diff.pwn.textconv=<sentinel script>`.
   Route: `git show --textconv HEAD:a.txt` under the closed env.
   Result: `inrepo_textconv_fired_under_closed_env`.
   Required fixed behavior: no sentinel, or a pinned-argv rule that refuses textconv-bearing modes until Slice 2.

4. `inrepo_fsmonitor`
   Plant: repo-local `core.fsmonitor=<sentinel script>`.
   Route: `git status --porcelain=v1` under the closed env.
   Result: `inrepo_fsmonitor_fired_under_closed_env`.
   The holder's demoted `-c core.fsmonitor=` interim may suppress the named chokepoint status calls, but this validation proves the env floor itself does not close the class. Any unwrapped status route, or the agent's own bare `git status`, still executes.

These are not speculative future gadgets. They are the exact rows the holder marks expected-fail, plus the RFC's own Layer 1 reason for minted config: the attacker's config must not travel with the object store (`docs/rfcs/0164...:106-124`).

## Additional false-negative in the certificate

The holder and seed both say `GIT_CONFIG_COUNT` env config beats `git -c` (`HOLDER.md:145-146`, `docs/rfcs/0164...:134-137`). On this worktree's Git 2.43.0, local verification and `git-config(1)` show the opposite: environment config is reported as command-line scope but is overridden by explicit `git -c`; `git config --get core.pager` returned the `-c` value when both were present. That does not make omission unnecessary, but it means `env_config_count_pager` cannot be the load-bearing proof A16 claims if the spawned command also uses `--no-pager` or matching `-c` neutralizers. A7/A16 need a direct assertion that `gitEnv()` and the final lane env contain no `GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n`, not just a sentinel that can go green for the wrong reason.

## Strongest rebuttal and why it fails

The strongest holder rebuttal is that this is intentional: P0 is Slice 0 + Slice 1, the expected-fail rows are surfaced honestly, and Slice 2 is named as the immediate minted-config follow-up.

That honesty is useful, but it does not clear the falsification gate. The gate asks whether severance is complete and whether the corpus certifies no-op behavior. A known expected-fail row is a known execution path. Calling it a residual does not make the gadget inert, and hashing a table that accepts the residual does not produce the G4 certificate the RFC requires.

The other rebuttal is that the residual is only the agent's arbitrary git, not daemon identity. That also fails as a clearing argument. The seed explicitly includes the driven agent lane env in Slice 1, and the holder itself puts `agent_bare_git_diff` in the P0 corpus. More importantly, the same repository-local classes (`filter.clean`, `core.fsmonitor`, textconv) are exactly what daemon/porter helper paths execute whenever an unwrapped route remains. A clearing SPEC cannot rely on every future helper avoiding the known expected-fail class while simultaneously claiming structural severance.

## Unanswered gap

Before the gate can clear, the SPEC must pick one of two honest shapes:

1. Pull Slice 2 into the clearing scope: mint `clean.git`, attach per-lane worktrees to its common-dir, and flip `inrepo_config_diff_external`, `inrepo_attributes_textconv`, `inrepo_filter_clean`, and `agent_bare_git_diff` to expected-pass with red-before/green-after sentinel tests.

2. Keep P0 as a non-clearing floor: state that severance-completeness and G4 remain blocked by the A18 expected-fail rows, do not treat `corpus_green_hash` as a certificate for the residual rows, and do not let A20/G1 clear until Slice 2 is green.

Until one of those is done, a gadget can still reach exec under the lane git surface, the corpus explicitly allows it, and the severance-completeness gate should not clear.