# RFC 0015: Self-Contained Agent Skills For Striatum

Status: accepted (V1)
Date: 2026-05-08
Context:
`docs/AGENTS.md`,
`docs/SPEC.md` § "Adapter Boundary",
`docs/DECISION_LOG.md` (D006, D007, D009, D020, D028),
RFC 0009 (long-lived process supervision),
RFC 0010 (tool harness profiles),
RFC 0014 (process adapter completion guarantees),
`src/striatum/cli/parser.py`,
`src/striatum/api.py`

## Problem

An agent driving a Striatum workflow needs to know, at each step, which CLI
verb to call: `register-session`, `claim-next`, `ack`, `heartbeat`,
`publish-artifact`, `verdict` / `submit-review`, `complete`, plus the
worktree, supervise, decision, and recovery surfaces. Today this contract
is documented in three places:

1. The `commands` block on each work packet (authoritative for the *next*
   step but silent on the surrounding flow).
2. `AGENTS.md` / `CLAUDE.md` at the root of the Striatum repository
   (authoritative for the surrounding flow, but only present for
   contributors who cloned the repo).
3. `docs/SPEC.md` and the RFC log (authoritative for semantics, but verbose
   and not designed to be loaded into an agent prompt).

For the intended deployment shape — install Striatum into the user's
environment, point it at an unrelated target repository, and have an agent
CLI drive a workflow there — none of these are reliable. The Striatum repo
may not be on disk, may be at the wrong version, or may be replaced by a
fork. Operators today fix this by pasting AGENTS.md into their target
repo's CLAUDE.md, which (a) is manual, (b) drifts as the runner's CLI
evolves, and (c) is a copy of an upstream artifact that wasn't designed to
travel.

The agent CLIs Striatum already targets — Claude Code, Codex, Gemini CLI —
have first-class conventions for "skills" / custom sub-agents / custom
agent roles that are loaded lazily by the agent at need. RFC 0010 already
records these capabilities per tool (`subagents`, `agent_teams`, `skills`,
`custom_agent_roles`). Skills are the right home for "how to drive
Striatum": the agent loads them on demand, they live in the target repo
(or user home), and they don't burden every prompt.

What's missing is a way for Striatum *itself* to put those skills on disk,
generated from the installed runner so they describe what this version of
the CLI actually does. This RFC defines that.

## Goals

- After `striatum skills install`, the target environment contains a
  self-contained skill bundle sufficient to drive the runner from a fresh
  agent session — no reading of the Striatum source repo required.
- The bundle covers the conversational verbs an agent needs: scaffolding
  (`init`, `workflow init`, `run prepare`, `branch confirm`), the
  claim/ack/publish/complete loop, worktree isolation (RFC 0008),
  supervised lanes (RFC 0009), recovery (RFC 0014), decisions, and
  doctor / status introspection.
- Skill content is generated from the installed runner's own command
  surface — parser introspection plus packaged templates — so it stays
  in lockstep with the version on disk.
- Skills emit no external network references. They invoke
  `striatum <verb> --help` for unknown options instead of pointing at
  docs URLs. (Preserves D020.)
- Per-tool idioms are honored through RFC 0010 profiles: Claude Code
  gets `.claude/skills/`, Codex gets its custom-agent surface, generic
  agents get a single Markdown bundle.
- The bundle is regenerable, idempotent, and refuses to clobber operator
  edits without `--force`.

## Non-Goals

- A hosted skill marketplace, version registry, or update-fetch service.
  D020 stands; skills ship inside the installed Python distribution.
- Auto-installing skills into a target repo without an explicit operator
  command. `striatum init` may opt in via a flag, but it does not
  silently scribble into `.claude/`.
- Replacing `AGENTS.md` or `docs/SPEC.md` as the contributor-facing
  documentation. Those describe how to *develop* Striatum; skills
  describe how an agent *drives* it.
- Generating workflow-specific task prompts. Task prompts live on the
  workflow's jobs, not in the skill bundle.
- Skills that mutate `.striatum/state.sqlite3` directly. They invoke the
  CLI; SQLite remains the runner's private boundary (D006, D009).
- Tracking which skill an agent actually consulted. Skill load is a
  client-side concern that Striatum does not observe.

## Proposal

Three landable changes, each scoped so it can ship in its own PR.

### 1. `striatum skills install`

A new CLI verb:

```text
striatum skills install
  [--target <path>]            # default: target repo root
  [--profile <id>]             # claude_code | codex | gemini | generic | all (default: claude_code)
  [--scope project|user]       # default: project; user writes to ~/.claude/...
  [--namespace <prefix>]       # default: striatum-
  [--force]                    # overwrite operator-edited files
  [--dry-run]                  # print plan, write nothing
  [--json]
```

Behavior:

- Reads packaged skill templates from
  `src/striatum/skills/templates/<profile>/*.md.tmpl`.
- Renders templates with values gathered from runner introspection: CLI
  verb syntax pulled from the parser (`striatum.cli.parser`), the
  current package version (`striatum.__version__`), the canonical work
  packet field names from `striatum.api`, and the front-matter schemas
  from `striatum.artifacts`.
- Writes one skill file per logical surface (see (3) below) plus a
  manifest (see (2) below).
- Refuses to overwrite a file whose on-disk SHA256 differs from the
  hash recorded in the manifest, unless `--force` is given. New files
  (no manifest entry) are written without prompting.
- On success, prints a JSON summary listing each path written, skipped,
  or refused.

The packaged templates are part of the Python distribution
(`include` / `package_data` in `pyproject.toml`), so an installed
runner has them on disk regardless of whether the source repo is
present.

### 2. Generation manifest

Each install writes
`.claude/skills/<namespace>workflow/.manifest.json` (path varies by
profile) with shape:

```json
{
  "schema_version": "striatum.skills.manifest.v1",
  "striatum_version": "0.7.3",
  "generated_at": "2026-05-08T18:42:11Z",
  "profile": "claude_code",
  "namespace": "striatum-",
  "files": [
    {
      "path": ".claude/skills/striatum-workflow/SKILL.md",
      "sha256": "ab12...",
      "template": "claude_code/striatum-workflow.md.tmpl"
    },
    ...
  ]
}
```

The manifest is the single source of truth for "did the operator edit
this skill?". It is not consulted by the runner at workflow time; it is
only read by `skills install` and `doctor`.

### 3. Skill bundle (Claude Code reference layout)

Five skills, each a SKILL.md with frontmatter and an instruction body.
Names and triggers are tuned to load on the verb the operator actually
says:

| Skill | Triggers when the agent should… | Covers |
|---|---|---|
| `striatum-workflow` | be told "use Striatum" / "drive this workflow" / "claim work" | top-level routing skill that points at the others; lists the no-go boundary (no SQLite writes, no marker files as state, no advancing state by printing phrases) |
| `striatum-scaffold` | scaffold a new workflow / start a run | `init`, `workflow init`, `workflow validate`, `run prepare`, `run start`, `branch confirm` |
| `striatum-claim-loop` | actually do work | `register-session`, `claim-next`, `ack`, `heartbeat`, `publish-artifact`, `verdict`, `submit-review`, `complete`, plus the worktree-create / worktree-release flow when `worktree_required: true` (RFC 0008) |
| `striatum-supervise` | run a long-lived agent session | `supervise start | send | stop | status | list` and the stdin-line packet contract (RFC 0009) |
| `striatum-recover` | something looked stuck | `status`, `why`, `doctor --verbose`, `recovery stale-leases`, `recovery requeue-stale`, `recovery process-reconcile` (RFC 0014), `checkpoint resolve`, `dashboard --once` |

Each SKILL.md has:

- YAML-style frontmatter: `name`, `description` (a single sentence that
  reliably triggers the skill), `version` (Striatum version that
  generated it), `striatum_min_version`, `profile`.
- A **When to invoke** section: bullet list of scenarios.
- An **Authoritative reference** section: canonical command shapes
  generated from the parser, with one-line summaries.
- A **Common patterns** section: copy-pasteable command sequences for
  the two or three flows the skill covers.
- A **What not to do** section: the boundary violations the runner
  silently tolerates but operators get burned by (writing to SQLite,
  treating marker files as state, expecting transcripts, parsing
  supervisor stdout for workflow state, deriving bylines from job
  titles).
- A **Where to look next** section that names other skills in the
  bundle by relative path. No external URLs.

A header line in each generated file reads, verbatim:

```
<!-- Generated by striatum {version}. Regenerate with `striatum skills install`. -->
```

so an operator who finds a stale skill knows exactly what to run.

### 4. Profile coverage

V1 ships these profiles. Path layouts come from RFC 0010 entries.

- **`claude_code`** — writes to `.claude/skills/<namespace>*/SKILL.md`.
  V1 reference profile; lands first.
- **`codex`** — writes to `.codex/agents/<namespace>*.md` using the
  custom-agent-role idiom captured in RFC 0010. Same content shape,
  format-adapted.
- **`gemini`** — best-effort; V1 falls back to the `generic` shape if
  RFC 0010's `gemini` profile does not yet declare a skill-equivalent
  surface. Promoting `gemini` to first-class is a follow-up once the
  CLI exposes a stable convention.
- **`generic`** — writes a single
  `<namespace>AGENT_GUIDE.md` at the target root. For any agent CLI
  that does not have a skill convention, this is the fallback the
  operator can `cat` into a system prompt.
- **`all`** — writes every profile. Useful for a target repo that is
  exercised under multiple agent CLIs.

`--scope user` rewrites the prefix from the target repo to the user
home (`~/.claude/skills/...` for Claude Code) so a developer who works
across many target repos installs once.

### 5. Bootstrap and drift detection

- `striatum init` gains `--with-skills [profile]`. When passed, the
  same code path as `striatum skills install` runs immediately after
  `.striatum/` is created. Default off, to preserve the current
  surface.
- `striatum doctor` adds two checks:
  - `skills_missing` — the namespace directory does not exist where
    the manifest says it should.
  - `skills_outdated` — the manifest's `striatum_version` is older
    than `striatum.__version__`, or the bundled templates' hashes
    differ from the manifest.
  Doctor never auto-regenerates; it surfaces the condition with the
  exact `striatum skills install` invocation that would fix it.

### 6. Self-contained guarantee

The bundle is "self-contained" in three concrete senses:

- **No external links.** Generation refuses to emit a URL outside the
  `.claude/skills/<namespace>*/` tree (or the profile's equivalent).
  Cross-skill references use relative paths.
- **No source-repo dependency.** Templates ship inside the installed
  Python package. An agent that has never seen the Striatum source
  tree can read SKILL.md, run `striatum --help`, and proceed.
- **Self-describing version.** Each file's header records the runner
  version that generated it. If a skill and the installed runner
  disagree, the operator is told to regenerate; the runner does not
  attempt to interpret a foreign version.

## Acceptance Criteria

- `striatum skills install --profile claude_code` in an empty target
  directory writes the five SKILL.md files and a manifest, and a
  second invocation with the same install is byte-identical.
- After install, an operator-modified `striatum-claim-loop/SKILL.md`
  is preserved across a re-run; `--force` overwrites it; `--dry-run`
  prints a plan that names the conflict.
- `striatum init --with-skills` produces a target tree containing both
  `.striatum/state.sqlite3` and the skill bundle.
- `striatum doctor` reports `skills_missing` for a target with no
  bundle and `skills_outdated` after the package is upgraded but
  `skills install` has not been re-run.
- A Claude Code session pointed at a fresh clone of an unrelated
  target repo, after `striatum init --with-skills`, can: register a
  session, claim a packet, satisfy a worktree-required job, publish a
  required artifact, and complete the job — using only the skill
  bundle, with no reference to the Striatum source repo.
- The Codex profile produces files at `.codex/agents/striatum-*.md`
  with the same logical content as the Claude Code profile.
- `tests/test_skills_install.py` covers: profile selection, idempotent
  regeneration, edit-detection refusal, `--force`, `--dry-run`,
  manifest shape, and the no-external-URL invariant.
- Generated skill content contains no URLs and no paths outside the
  bundle's namespace directory. A unit test enforces this on rendered
  output.

## Open Questions

- **Skill name collisions.** A target repo may already have a
  hand-written `.claude/skills/foo/`. The `striatum-` namespace
  reduces but does not eliminate the collision risk. The proposal
  reserves the prefix and exposes `--namespace` to operators with a
  conflict; an alternative is to write everything under a single
  `.claude/skills/striatum/` directory with sub-SKILL files. V1
  goes with one directory per skill (matching Claude Code's
  conventional layout) but flag the alternative if reviewers prefer
  it.
- **User-scope vs project-scope default.** Project scope keeps the
  bundle reproducible across machines; user scope means a developer
  doesn't have to install per target. V1 defaults to project; user
  scope is opt-in with `--scope user`.
- **Slash commands as well as skills.** Claude Code also supports
  `.claude/commands/<name>.md` for imperative slash commands. A
  `striatum-claim-next.md` slash command would be a thin wrapper
  around the CLI. V1 ships descriptive skills only; slash commands
  are a follow-up if operators ask for them.
- **Bundling the workflow JSON schema.** The skills could embed the
  V1 workflow schema or only point at `striatum workflow validate`.
  V1 takes the latter (smaller, runner is the source of truth);
  promoting to embedded schema is a follow-up if agents struggle
  without it.
- **Auto-regeneration on upgrade.** A post-install hook or
  `striatum init` could regenerate skills automatically when a version
  mismatch is detected. V1 leans operator-driven (mirrors D036's lazy
  recovery pattern): doctor flags drift; the operator runs
  `skills install` to refresh.
- **Skill-as-canonical-agent-doc.** Long-term, the Claude Code skill
  bundle could become the canonical "agent-driving Striatum"
  reference, with `AGENTS.md` narrowed to "developing Striatum".
  Worth a follow-up once the bundle has been used in anger; out of
  scope here.
- **Codex / Gemini parity timing.** V1 ships `claude_code` and
  `generic` first because Claude Code is the most exercised
  consumer. `codex` and `gemini` profiles can land in the same RFC
  V1 if reviewers want a single PR; otherwise (3) above splits them
  out. Default plan: same RFC, separate PRs.

## Relationship To Other RFCs

- **RFC 0010** — tool harness profiles. The skill installer is a
  consumer of profiles. Profile fields decide where files go and
  which agent-CLI idioms (`subagents`, `agent_teams`, `skills`,
  `custom_agent_roles`) the skill body cites.
- **RFC 0009** — long-lived process supervision. The
  `striatum-supervise` skill is the agent-facing summary of the
  supervisor lifecycle and the stdin-line packet contract.
- **RFC 0014** — process adapter completion guarantees. The
  `striatum-recover` skill teaches the diagnostic envelope and
  `recovery process-reconcile` flow once RFC 0014 lands.
- **RFC 0008** — worktree isolation for parallel jobs. The
  `striatum-claim-loop` skill teaches the worktree-create /
  worktree-release sequence triggered by `worktree_required: true`.
- **RFC 0012 / 0013** — local API and web UI. Independent. Skills
  describe how to drive the CLI; the API/UI are alternate front
  ends. A future skill (`striatum-serve`) could cover them.
- **D006 / D007 / D009** — SQLite is the live state, repo files are
  durable provenance, agents update state through the CLI. The skill
  bundle's "What not to do" sections enforce these boundaries.
- **D020** — no hosted services. Skills ship inside the installed
  package; generation is offline; bundle content emits no external
  URLs.
- **D028** — no transcripts. Skills do not instruct agents to capture
  stdout/stderr or model output; the boundary is reasserted in the
  `striatum-supervise` skill body.

## Implementation Path

V1 ships in three landable steps:

1. **Generator core + Claude Code profile.** Add
   `src/striatum/skills/` with templates and a renderer; add the
   `striatum skills install` verb; ship the five-skill bundle for
   `claude_code`; add `tests/test_skills_install.py`. Smallest
   tractable PR; gives operators a working bundle.
2. **`generic` profile + bootstrap convenience.** Add the
   `generic` profile (single Markdown guide), the
   `striatum init --with-skills` flag, and the `doctor`
   `skills_missing` / `skills_outdated` checks.
3. **`codex` profile (and best-effort `gemini`).** Add the Codex
   custom-agent-role idiom; produce `.codex/agents/striatum-*.md`
   with format-adapted content. Promote `gemini` from generic to
   first-class once that CLI's skill convention stabilizes.

Each step has its own acceptance test. RFC 0015 is "accepted" once
steps 1 and 2 land; step 3 promotes additional profiles without
re-opening the RFC.
