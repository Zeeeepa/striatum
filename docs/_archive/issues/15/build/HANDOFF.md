author: implementer-unknown-model-001

# GH #15 — implementer handoff

## What this change does

Reconciles operator-facing documentation with D094 / RFC 0043 and the
shipped `striatum daemon migrate-repo-local` surface so the post-D094
PostgreSQL-first state model is told as one consistent story across
README, SPEC, getting-started, the operator + agent playbooks, the
CLI reference, the ubiquitous-language glossary, the skill / plugin
templates, and a new operator runbook. RFC 0048 is called out
explicitly as remaining substrate-port work where the daemon RPC
server still delegates some single-repo business logic through the
SQLite-backed CLI path under a test-harness escape. Historical
dogfood scaffolds, RFC source-of-truth text, INTERVIEW_LOG /
PRIOR_ART, and the DECISION_LOG D-rows remain frozen as decision
provenance; the new regression test allowlists those paths.

## Changed files

- `docs/POSTGRES_TRANSITION.md` *(new)* — operator runbook covering
  prerequisites, daemon-config surfaces, `daemon doctor
  --apply-migrations`, daemon startup, `repo add`, dry-run /
  apply / tombstone / delete variants of `daemon migrate-repo-local`,
  exit-code-11 / 12 refusal semantics, rollback and inspection
  limits, and the RFC 0048 caveat.
- `README.md` — replaced the V1 "SQLite under `.striatum/` is
  authoritative" framing with the daemon-required PostgreSQL story;
  refreshed the status section to point at CHANGELOG + the RFC
  README instead of a stale version pin; added the runbook to the
  documentation map. Stays inside the 250-line budget enforced by
  `tests/test_doc_links.py::test_readme_under_line_budget` (242
  lines).
- `AGENTS.md` — product-boundary bullets updated to the
  daemon-owned-PostgreSQL state model with a pointer at the
  transition runbook.
- `docs/SPEC.md` — § Product Boundary and § State Store rewritten
  to describe the daemon-owned PostgreSQL substrate under a
  `repository_id` scope, the retired `--no-daemon` flag, exit
  codes 11 / 12, and the RFC 0048 caveat. The daemon-coordination
  section now describes the RFC 0033 daemon-global cutover and the
  RFC 0043 per-repo cutover (`migrate-repo-local`) distinctly and
  removes the "repo-local SQLite remains the live run state" claim.
- `docs/GETTING_STARTED.md` — prerequisites now name system
  PostgreSQL; a new "Bootstrap the daemon" step shows daemon doctor
  + daemon startup + per-repo migration; the `.striatum/` directory
  description matches operational-scratch reality; the runbook is
  surfaced under "Where to next."
- `docs/HOW_TO_HUMAN.md` — `init` description aligned with the
  daemon-required model and the retired `--no-daemon` flag; the
  V1 daemon-storage block was rewritten to combine RFC 0033 and
  D094 / RFC 0043 and now documents `migrate-repo-local`'s
  command shape; the artifact-layout illustration no longer
  pretends `state.sqlite3` is operator-visible state.
- `docs/HOW_TO_AGENT.md` — "What you are looking at" + "What you
  should not do" updated to the daemon-bypass-prohibited framing.
- `docs/CLI_REFERENCE.md` — added `striatum daemon doctor` and
  `striatum daemon migrate-repo-local` to the daemon block and its
  prose; documented the retirement of `--no-daemon`; recorded
  exit codes 11 (`daemon_unreachable`) and 12 (`repo_not_migrated`)
  in the stable exit codes list.
- `docs/UBIQUITOUS_LANGUAGE.md` — terms `binary`, `repo-local
  control plane`, `live state`, `state store`, `state database`,
  `event log`, `message bus`, `message queue`, `job worktree`,
  `run dashboard`, `supervisor`, `Striatum daemon`, `daemon
  registry`, `daemon DB`, `daemon-owned supervisor`, `supervisor
  pointer`, `daemon DB migration`, `repository tenant`, `substrate
  version`, plus the Distinctions list updated to the post-D094
  model; added `operational scratch`, `daemon-required CLI`,
  `repo-local migration`, and `tombstone SQLite`.
- `docs/INDEX.md` — added the runbook to the onboarding table.
- `src/striatum/skills/templates/claude_code/{workflow,scaffold,mcp}.md.tmpl`,
  `src/striatum/skills/templates/generic/STRIATUM_AGENT_GUIDE.md.tmpl`,
  `src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`,
  `src/striatum/skills/context.py` — agent-facing skill guidance now
  teaches the daemon-required, daemon-as-single-writer model and
  the operational-scratch role of `.striatum/`; the canonical scaffold
  `init` description and boundary-bullet entries were updated in
  one place (`context.py`) so generated skills regenerate cleanly.
- `src/striatum/plugins/templates/{claude_code,codex,gemini}/skills/{workflow,scaffold,mcp}.md.tmpl`
  — synchronized byte-for-byte with the canonical claude_code skill
  templates to satisfy
  `tests/test_plugin_install.py::test_skill_templates_match_skills_module`.
- `tests/test_doc_links.py` — new
  `test_current_product_docs_do_not_claim_sqlite_authority` regression
  blocks stale authoritative-SQLite wording from coming back in current
  product docs; allowlist covers `docs/rfcs/`, `docs/dogfood/`,
  `docs/DECISION_LOG.md` (D-row provenance, out of scope for GH #15),
  and incubation provenance pages.

## Out of scope (deliberately not changed)

- `docs/dogfood/`, `docs/rfcs/`, `docs/issues/15/SCOPE.md`,
  `docs/issues/{14,16,17}/`, `docs/TODO.md`, `docs/ROADMAP.md`,
  `docs/DECISION_LOG.md` — frozen historical / decision artifacts or
  parallel issue workflows.
- All runtime source code, daemon RPC routing, migration logic
  — GH #15 is docs-only.

## Acceptance traceability (SCOPE §4)

- DoD-1 — runbook exists at `docs/POSTGRES_TRANSITION.md` and walks a
  new operator install → Postgres → doctor → daemon → repo migration
  → verification.
- DoD-2 — runbook documents `STRIATUM_DAEMON_DB_URL`, daemon config,
  and `--postgres-url` as the three supported connection surfaces.
- DoD-3 — runbook documents
  `striatum daemon doctor --postgres-url ... --apply-migrations`.
- DoD-4 — runbook documents the exact shipped
  `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path>
  [--postgres-url <url>] [--dry-run] [--keep-sqlite-readonly |
  --no-keep-sqlite-readonly --confirm-delete] [--json]` shape; CLI
  reference matches.
- DoD-5 — runbook explains tombstone vs delete semantics including
  the required `--confirm-delete` pairing.
- DoD-6 — runbook explains exit codes 11 `daemon_unreachable` and 12
  `repo_not_migrated`, with operator remediation; CLI reference §
  Stable exit codes matches.
- DoD-7 — README, SPEC, GETTING_STARTED, HOW_TO_HUMAN, HOW_TO_AGENT,
  CLI_REFERENCE, AGENTS, UBIQUITOUS_LANGUAGE all describe the
  post-D094 PostgreSQL-first model consistently.
- DoD-8 — `tests/test_doc_links.py::
  test_current_product_docs_do_not_claim_sqlite_authority` enforces
  the rule.
- DoD-9 — CLI reference + HOW_TO_HUMAN describe `--no-daemon` as
  retired and reference the existing argparse-rejection behavior;
  this matches `tests/cli/test_no_daemon_retired.py`.
- DoD-10 — POSTGRES_TRANSITION § "What changed" and SPEC §
  daemon-coordination distinguish RFC 0033 daemon-global PostgreSQL
  substrate from RFC 0043 / D094 per-repo workflow-state migration.
- DoD-11 — RFC 0048 remaining work is called out in SPEC,
  HOW_TO_HUMAN, CLI_REFERENCE, the new runbook, and the agent-facing
  skill bundle.
- DoD-12 — skill and plugin templates (claude_code, codex, gemini,
  generic) no longer teach the SQLite-authoritative model;
  `tests/test_plugin_install.py::test_skill_templates_match_skills_module`
  confirms the canonical and plugin trees stay byte-identical.
- DoD-13 — `tests/test_doc_links.py::
  test_current_product_docs_do_not_claim_sqlite_authority` blocks
  reintroduction in current product docs and allowlists frozen
  historical / RFC / DECISION_LOG context.
- DoD-14 — `striatum daemon migrate-repo-local --help` matches the
  shape documented in CLI_REFERENCE, the runbook, and HOW_TO_HUMAN.

## Tests run

- `PYTHONPATH=src python3 -m pytest tests/test_doc_links.py
  tests/test_skills_install.py tests/test_plugin_install.py
  tests/cli/test_parser_help.py tests/cli/test_no_daemon_retired.py`
  — 58 passed, 1 pre-existing failure
  (`test_decision_log_rows_under_word_budget`: D094 row over the
  200-word budget at 439 words; documented in
  `docs/ROADMAP.md` §9.2; DECISION_LOG.md is explicitly out of
  scope for this issue per SCOPE §3).
- `make lint` — clean.
- `make typecheck` — clean (216 source files).
- `striatum daemon migrate-repo-local --help` — matches the
  command shape documented in CLI_REFERENCE.md and the runbook.

## Tests not run

- Full `make test` — depends on Postgres + daemon; not part of
  the docs-only acceptance surface and was not part of the
  verification commands listed in SCOPE §6 beyond the targeted
  pytest subset above.
- `make smoke` — same reasoning as full `make test`.

## Residual risk

- Several other docs in `docs/` (e.g. `MCP.md`, `ROADMAP.md`,
  `TODO.md`, `rfcs/README.md`) were already modified during this
  parallel issue run by other workflows; they are not part of
  GH #15's scope and were not touched here.
- The `daemon doctor` help output is terser than the runbook's
  description; if the V1.6 help text is updated in the future to
  mention substrate / schema / audit fields explicitly, the
  runbook's matching paragraph should be tightened to quote the
  help verbatim.
- The pre-D094 SQLite migration list retained for the
  migrate-repo-local golden fixture is mentioned in SPEC.md; this
  matches RFC 0043 §7's fixture requirement. If RFC 0048 phase C
  eventually retires that fixture, the SPEC sentence + the new
  regression-test allowlist should be revisited.
- The `test_decision_log_rows_under_word_budget` failure was
  pre-existing and is tracked in ROADMAP §9.2; no new failures were
  introduced by this change.
