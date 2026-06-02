# Optional Agent Skills (operator-side)

This directory is a **curated registry of optional, third-party agent skills**
that a Striatum *operator* (the coding-agent CLI you launch as the interface to
the runner) may choose to install into its own agent environment.

These skills are **not part of Striatum**:

- They are **not** the RFC 0015 self-contained operator skill bundle. That
  bundle is Striatum-authored, version-stamped, and installed with
  `striatum skills install`. The entries here are external tools.
- They are **not** part of the runtime. The runtime is the `striatumd` daemon
  plus PostgreSQL; per `docs/SPEC.md` the runner never imports a model vendor
  SDK. Optional skills run **agent-side** (for example under `.claude/skills/`),
  alongside but separate from the Striatum operator pack.
- `striatum skills install` and `striatum plugin install` do **not** install,
  fetch, vendor, maintain, or call anything listed here. Striatum only
  *suggests* them.

## How the operator offers these

On first initiation/adoption of a repo, the Striatum operator skill bundle
prompts the user about optional skills (see the "Optional skills" step in the
scaffold skill / agent guide). The operator must:

1. Describe the available optional skills and what each is for.
2. Install **only** the ones the user explicitly confirms.
3. Use each skill's own documented install command (below) — these are
   third-party installs the user is opting into, not Striatum operations.

The operator bundle itself carries no URLs or install commands (RFC 0015
self-contained invariant); the concrete pointers live here in the source repo.

## Boundary rules for entries

- Third-party and separately licensed. Record the upstream source and license.
- May be provider-specific (e.g., Claude-only). Note the provider constraint.
- Suggested, never auto-installed; install requires explicit user confirmation.

## Catalog

### adhd — divergent ideation (tree-of-thought with pruning)

- **What it is:** spawns parallel divergent reasoning branches under different
  cognitive "frames," then runs a critic pass to score, cluster, detect traps,
  and deepen the survivors. Useful for architecture decisions, API design,
  naming, fuzzy-bug hypotheses, and "give me a few genuinely different ways to
  do this" prompts.
- **Source:** https://github.com/UditAkhourii/adhd
- **License:** MIT (see [`adhd/LICENSE`](adhd/LICENSE)).
- **Provider constraint:** the `SKILL.md` is a standalone prompt that drives the
  host agent's own parallel sub-agent calls; it targets Claude Code / the Claude
  Agent SDK. The optional `adhd-agent` npm CLI (for batch/non-Claude use) is
  **not** vendored here and is not required.
- **Vendored copy (offline):** [`adhd/SKILL.md`](adhd/SKILL.md) is a
  byte-faithful pinned copy of upstream `skills/adhd/SKILL.md`, fetched from
  `main` on 2026-05-27. Content hash (sha256):
  `bc821db683d56b78fbbff5244295f408110cd813db768d1008998722154d9ca4`. This copy
  is a reviewable reference; it is **not** auto-installed into the operator's
  environment and is **not** part of the Striatum operator bundle.
- **Install (agent-side, user-confirmed):** either copy the vendored
  [`adhd/SKILL.md`](adhd/SKILL.md) into the operator's skill directory (e.g.
  `.claude/skills/adhd/SKILL.md`) for an offline, pinned install, or pull the
  latest from upstream with `npx skills add UditAkhourii/adhd`.
- **Updating the vendored copy:** re-fetch upstream `skills/adhd/SKILL.md`,
  refresh the date and sha256 above, and re-confirm the license is unchanged.
- **Relationship to Striatum:** ADHD does divergence *inside one agent session*
  with no durable provenance. Striatum's server-side, provenance-tracked
  equivalent of the same diverge → prune → deepen pattern is proposed as a
  workflow shape in [`docs/rfcs/0087-divergent-ideation-workflow-shape.md`](../../docs/rfcs/0087-divergent-ideation-workflow-shape.md).
  The two are complementary: install ADHD for quick agent-side ideation; use the
  RFC 0087 shape when you need auditable, multi-lane divergence.

### supabase-postgres-best-practices — Postgres correctness & performance rules

- **What it is:** a reference skill of ~35 Postgres rules across 8 categories
  (query, connection, security/RLS, schema, concurrency/locking, data access,
  monitoring, advanced), each with incorrect-vs-correct SQL. Despite the name it
  is ~85% vanilla Postgres; only the `security-rls-*` references are
  Supabase-specific. Triggers when writing, reviewing, or optimizing SQL,
  schema, or DB configuration.
- **Source:** https://github.com/supabase/agent-skills (skill
  `skills/supabase-postgres-best-practices`).
- **License:** MIT, Copyright (c) 2026 Supabase (see
  [`supabase-postgres-best-practices/LICENSE`](supabase-postgres-best-practices/LICENSE)).
- **Provider constraint:** none — it is provider-agnostic reference Markdown, not
  a prompt that drives a specific agent runtime.
- **Vendored copy (offline):**
  [`supabase-postgres-best-practices/`](supabase-postgres-best-practices/) is a
  byte-faithful pinned copy of upstream (skill version `1.1.1`), fetched from
  `main` (`759fddf`) on 2026-06-02. `SKILL.md` sha256:
  `ccd6e4596bd51cf344fe76c464867c541ccc16b6d90ae7a9db449fb17588613b`; combined
  `references/*.md` sha256:
  `5b917809b25b849b1833fdbc0e241747bfd9a8c4aab966d6756a6e9e348433c1`. Reviewable
  reference only; **not** auto-installed and **not** part of the Striatum
  operator bundle.
- **Install (agent-side, user-confirmed):** copy
  [`supabase-postgres-best-practices/`](supabase-postgres-best-practices/) into
  the operator's skill directory (e.g.
  `.claude/skills/supabase-postgres-best-practices/`), or pull the latest from
  upstream with `npx skills add supabase/agent-skills`.
- **Updating the vendored copy:** re-fetch upstream, refresh the date/commit and
  both sha256 lines above, and re-confirm the license is unchanged.
- **Relationship to Striatum:** **high relevance for the `striatumd` Postgres
  layer**, which is ~half the codebase. The `lock-` rules in particular map
  directly onto the daemon's lease/claim/interrogation concurrency:
  `lock-deadlock-prevention` (consistent lock ordering) is the design rule behind
  the `sessions`↔`runs`↔`jobs` lock-order inversion that `go/pkg/mutations`
  currently *tolerates* with `withTxRetryOnDeadlock` rather than *prevents*;
  `lock-skip-locked` is the pattern the claim queue already uses
  (`claim.go` `FOR UPDATE OF qm SKIP LOCKED`); `lock-advisory` is what
  `lockRunInterrogation` already does (and could generalize). The `security-rls-*`
  references are **not applicable** — Striatum uses role grants + capability
  tokens, not RLS. Performance rules (`query-`, `data-`) are low-urgency at
  single-operator/laptop scale; the value here is concurrency *correctness*, not
  throughput.

## Adding an entry

Append a `### <skill-id> — <one-line purpose>` section with: what it is, source,
license, any provider constraint, the user-confirmed install command, and its
relationship (if any) to Striatum features. Keep entries generic and honest
about the third-party, agent-side, opt-in boundary.
