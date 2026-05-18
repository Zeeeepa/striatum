# Documentation Map

Start here:

1. [PRD.md](PRD.md)
2. [DECISION_LOG.md](DECISION_LOG.md)
3. [UBIQUITOUS_LANGUAGE.md](UBIQUITOUS_LANGUAGE.md)
4. [PRIOR_ART.md](PRIOR_ART.md)
5. [SPEC.md](SPEC.md)
6. [INTERVIEW_LOG.md](INTERVIEW_LOG.md)
7. [TODO.md](TODO.md)

## Planning

- [TODO.md](TODO.md) - repo split checklist and product improvement backlog.

## Usage

- [README.md Usage Guide](../README.md#usage-guide) - end-to-end CLI flow.
- [README.md § 2a. Shape A Custom Run Scaffold](../README.md#2a-shape-a-custom-run-scaffold) -
  generic guidance for scaffolding a new target-repository run from an RFC,
  TODO, bug report, feature request, or other local proposal.

## RFCs

- [rfcs/](rfcs/) - `striatum` product RFCs. Engram RFCs are external
  reference fixtures; Striatum product decisions live here.

## Follow-Up Specs

- [RFC_0014_DOGFOOD_FIX_SPEC.md](RFC_0014_DOGFOOD_FIX_SPEC.md) — fixes
  proposed after the RFC 0014 validation dogfood run. This is now tracked as
  [rfcs/0001](rfcs/0001-run-recovery-and-dogfood-fixes.md).

## Runtime Evidence

- `striatum evidence export` writes a redacted Markdown run snapshot for
  commit and review while leaving `.striatum/` ignored.
- `striatum decision record` writes owner choices as durable Markdown decision
  artifacts with machine-checkable local metadata.
- `striatum submit-review` combines review artifact publication and verdict
  recording for the common review-gate path.
- [MCP.md](MCP.md) documents daemon MCP and the compatibility-only local stdio
  wrapper.

## Design

- [design/](design/) — design artifacts produced before implementation.

## Reviews

- [reviews/](reviews/) — review findings, ledgers, and syntheses.

## Prompts

- [../prompts/](../prompts/) — historical incubation prompts retained as
  provenance. They are not current standalone execution plans unless rewritten
  for the target repository and branch.

## Historical Bootstrap

- [../scripts/striatum_tmux_design.sh](../scripts/striatum_tmux_design.sh)
  — temporary tmux harness for collecting the three required V1 MVP design
  inputs before synthesis. The watched completion artifacts are the three
  `docs/design/V1_MVP_DESIGN_INPUT_*.md` files. It is retained as incubation
  provenance; active adapter work should use Striatum-owned workflow and
  process-adapter paths.
