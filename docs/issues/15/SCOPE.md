---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/15/SPEC.md", "README.md", "docs/SPEC.md", "docs/GETTING_STARTED.md", "docs/HOW_TO_HUMAN.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "AGENTS.md"]
---

author: triager-unknown-model-001

# GH #15 - SCOPE

Bound scope for GH #15, "Docs: clarify PostgreSQL transition guidance."
The implementer must reconcile current operator-facing docs with D094/RFC
0043 and the shipped `daemon migrate-repo-local` surface, while preserving
RFC 0048 as remaining substrate-port work.

## 1. Issue covered

- GH #15 - Docs: clarify PostgreSQL transition guidance.

## 2. Files in scope

The implementer may edit or create only the current product/operator docs,
agent guidance templates, and regression tests needed for this issue.

- **EDIT** `README.md` - replace the stale "repo-local SQLite is
  authoritative" story with the current PostgreSQL-first transition story;
  keep README within the existing line-budget test.
- **EDIT** `AGENTS.md` - update the product-boundary bullets so contributors
  are not told that `.striatum/state.sqlite3` is current authoritative live
  state.
- **EDIT** `docs/SPEC.md` - reconcile the implementation contract with
  D094/RFC 0043, the daemon-required/Postgres direction, the current
  migration command, and the RFC 0048 caveat.
- **EDIT** `docs/GETTING_STARTED.md` - add the operator prerequisite and
  quick-start path for system PostgreSQL, daemon doctor, daemon startup, repo
  migration, and verification.
- **EDIT** `docs/HOW_TO_HUMAN.md` - replace pre-D094 daemon/SQLite wording,
  remove `--no-daemon` guidance, and point operators at the new transition
  runbook.
- **EDIT** `docs/HOW_TO_AGENT.md` - update the agent-facing state-substrate
  language so regenerated guidance and hand-authored agent docs agree.
- **EDIT** `docs/CLI_REFERENCE.md` - document `striatum daemon
  migrate-repo-local` and its flags, and remove the claim that `--no-daemon`
  forces direct mode.
- **EDIT** `docs/UBIQUITOUS_LANGUAGE.md` - update terms whose definitions
  still encode the pre-D094 hybrid model: `binary`, `repo-local control
  plane`, `live state`, `operator`, `workflow snapshot`, `state store`,
  `state database`, `event log`, `message bus`, `message queue`, `job
  worktree`, `run dashboard`, `supervisor`, `Striatum daemon`, `daemon
  registry`, `daemon DB`, `daemon-owned supervisor`, `supervisor pointer`,
  `repository tenant`, and the "Distinctions" bullets.
- **CREATE** `docs/POSTGRES_TRANSITION.md` - an operator runbook covering
  system PostgreSQL prerequisites, `STRIATUM_DAEMON_DB_URL` / daemon config /
  `--postgres-url`, `daemon doctor --postgres-url ... --apply-migrations`,
  daemon startup expectations, dry-run/full `daemon migrate-repo-local`,
  tombstone vs delete behavior, verification, exit codes 11 and 12, and
  rollback/inspection limits.
- **EDIT** `docs/INDEX.md` - add the new runbook to the documentation map.
- **EDIT** skill templates under `src/striatum/skills/templates/` - update
  generated Claude Code, Codex, Gemini, and generic agent guidance so it no
  longer teaches `.striatum/state.sqlite3` as the current live substrate.
- **EDIT** plugin templates under `src/striatum/plugins/templates/` - keep
  plugin-bundled skills byte-for-byte aligned with the canonical skill
  templates where `tests/test_plugin_install.py` requires it, and update
  plugin README/command text if it repeats the stale state model.
- **EDIT** `tests/test_doc_links.py` or add a focused docs regression test -
  assert current product docs do not reintroduce stale authoritative-SQLite
  claims outside allowed historical/migration contexts.
- **EDIT** `tests/test_skills_install.py` and `tests/test_plugin_install.py`
  only as needed to assert generated skills/plugins contain the updated
  state-substrate guidance.

## 3. Files and directories out of scope

The implementer must not edit:

- `docs/dogfood/` - frozen historical dogfood artifacts, including old
  `.striatum/state.sqlite3` references.
- `docs/rfcs/` - accepted/proposed design records are evidence, not targets
  for this docs cleanup.
- `docs/issues/15/SCOPE.md` - this triage artifact is an input to the
  implementation job, not part of the fix.
- `docs/issues/14/`, `docs/issues/16/`, `docs/issues/17/` - parallel issue
  workflows own their own scopes.
- `docs/TODO.md`, `docs/ROADMAP.md`, and `docs/DECISION_LOG.md` - preserve
  their open/deferred work and decision history for this issue. The
  implementation may cite them but should not rewrite them.
- `src/striatum/daemon_pg/`, `src/striatum/daemon_rpc/`,
  `src/striatum/cli/`, `src/striatum/db.py`, and other runtime code - no
  behavior change is part of GH #15.
- `prompts/P00*.md`, `prompts/STRIATUM_DAEMON_RESEARCH_PROMPT.md`, and
  other historical/bootstrap prompts unless a current generated skill/plugin
  template imports them directly.
- `.striatum/`, `.venv/`, caches, generated build output, transcripts, and
  private diagnostics.

## 4. Acceptance checklist

The verify job should cite each ID below.

- [DoD-1] `docs/POSTGRES_TRANSITION.md` exists and gives a new operator a
  linear path from install through PostgreSQL setup, daemon doctor, daemon
  startup, repo-local migration, and post-migration verification.
- [DoD-2] The runbook documents `STRIATUM_DAEMON_DB_URL`, daemon config, and
  per-command `--postgres-url` as the supported PostgreSQL connection
  surfaces.
- [DoD-3] The runbook documents `striatum daemon doctor --postgres-url ...
  --apply-migrations` or the current exact equivalent if help output shows a
  different spelling.
- [DoD-4] The runbook documents the exact shipped
  `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
  [--postgres-url <url>] [--dry-run] [--keep-sqlite-readonly |
  --no-keep-sqlite-readonly --confirm-delete] [--json]` shape.
- [DoD-5] The runbook explains tombstone vs delete behavior: the safe default
  keeps a read-only `state.sqlite3.tombstone`; deletion requires
  `--no-keep-sqlite-readonly --confirm-delete`.
- [DoD-6] The runbook explains exit code 11 `daemon_unreachable` and exit
  code 12 `repo_not_migrated`, including the operator remediation for each.
- [DoD-7] README, SPEC, GETTING_STARTED, HOW_TO_HUMAN, HOW_TO_AGENT,
  CLI_REFERENCE, AGENTS, and UBIQUITOUS_LANGUAGE tell one consistent
  PostgreSQL-first story and do not contradict D094/RFC 0043.
- [DoD-8] Current product docs no longer claim `.striatum/state.sqlite3` is
  authoritative live state except when explicitly describing historical V1
  behavior, migration fixtures, tombstone inspection, or RFC 0048 remaining
  delegated paths.
- [DoD-9] CLI reference and human docs do not say `--no-daemon` forces
  direct mode; they say it is retired/unsupported if mentioned at all.
- [DoD-10] Docs distinguish RFC 0033 daemon-global PostgreSQL substrate from
  RFC 0043/D094 repo-local workflow-state migration.
- [DoD-11] Docs clearly mark RFC 0048 as remaining work: daemon RPC still has
  single-repo business-logic delegation/substrate-port gaps until its phases
  land.
- [DoD-12] Skill and plugin templates no longer teach the old
  SQLite-authoritative model, and generated install tests cover at least one
  representative skill/plugin output.
- [DoD-13] A regression test blocks stale authoritative-SQLite claims in
  current product docs while allowing frozen historical dogfood artifacts,
  RFC history, migration fixtures, and low-level SQLite implementation tests
  to continue using accurate historical/technical wording.
- [DoD-14] `striatum daemon migrate-repo-local --help` and the docs agree on
  subcommand name, required `--from/--to` flags, optional `--repo` and
  `--postgres-url`, dry-run, tombstone/delete flags, and `--json`.

## 5. Risks and likely conflicts

- GH #16 and GH #17 also touch operator/agent prompt language. Avoid editing
  their scoped issue artifacts, and keep GH #15 changes focused on the
  PostgreSQL transition rather than broad operator-prompt rewrites.
- README has an enforced line budget in `tests/test_doc_links.py`; prefer
  pointing to `docs/POSTGRES_TRANSITION.md` instead of expanding README into
  a second runbook.
- `tests/test_plugin_install.py::test_skill_templates_match_skills_module`
  requires canonical skill bodies and plugin skill bodies to match
  byte-for-byte for shared skills. Update both sides together.
- Do not "fix" historical dogfood/RFC text. The acceptance condition is about
  current product docs, not rewriting provenance.
- RFC 0048 language must be precise: RFC 0043 landed the schema/CLI
  direction and migration surface, but RFC 0048 is still the remaining
  daemon-side business-logic substrate port.

## 6. Verification commands

Run these at minimum:

```bash
striatum daemon migrate-repo-local --help
make lint
make test
```

If the implementer adds a targeted docs regression test, also run the focused
test directly, for example:

```bash
pytest tests/test_doc_links.py tests/test_skills_install.py tests/test_plugin_install.py tests/cli/test_parser_help.py tests/cli/test_no_daemon_retired.py
```
