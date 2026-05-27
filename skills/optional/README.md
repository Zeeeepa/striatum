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
- **License:** MIT
- **Provider constraint:** built on the Claude Agent SDK; Claude-only.
- **Install (agent-side, user-confirmed):** `npx skills add UditAkhourii/adhd`
- **Relationship to Striatum:** ADHD does divergence *inside one agent session*
  with no durable provenance. Striatum's server-side, provenance-tracked
  equivalent of the same diverge → prune → deepen pattern is proposed as a
  workflow shape in [`docs/rfcs/0087-divergent-ideation-workflow-shape.md`](../../docs/rfcs/0087-divergent-ideation-workflow-shape.md).
  The two are complementary: install ADHD for quick agent-side ideation; use the
  RFC 0087 shape when you need auditable, multi-lane divergence.

## Adding an entry

Append a `### <skill-id> — <one-line purpose>` section with: what it is, source,
license, any provider constraint, the user-confirmed install command, and its
relationship (if any) to Striatum features. Keep entries generic and honest
about the third-party, agent-side, opt-in boundary.
