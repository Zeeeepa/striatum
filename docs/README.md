# Documentation Map

Start here:

1. [PRD.md](PRD.md)
2. [DECISION_LOG.md](DECISION_LOG.md)
3. [UBIQUITOUS_LANGUAGE.md](UBIQUITOUS_LANGUAGE.md)
4. [PRIOR_ART.md](PRIOR_ART.md)
5. [SPEC.md](SPEC.md)
6. [INTERVIEW_LOG.md](INTERVIEW_LOG.md)

## Follow-Up Specs

- [RFC_0014_DOGFOOD_FIX_SPEC.md](RFC_0014_DOGFOOD_FIX_SPEC.md) — fixes
  proposed after the RFC 0014 validation dogfood run.

## Runtime Evidence

- `agent_runner evidence export` writes a redacted Markdown run snapshot for
  commit and review while leaving `.agent_runner/` ignored.
- `agent_runner submit-review` combines review artifact publication and verdict
  recording for the common review-gate path.

## Design

- [design/](design/) — design artifacts produced before implementation.

## Reviews

- [reviews/](reviews/) — review findings, ledgers, and syntheses.

## Prompts

- [../prompts/](../prompts/) — execution prompts.

## Bootstrap

- [../scripts/agent_runner_tmux_design.sh](../scripts/agent_runner_tmux_design.sh)
  — temporary tmux harness for collecting the three required V1 MVP design
  inputs before synthesis. The watched completion artifacts are the three
  `docs/design/V1_MVP_DESIGN_INPUT_*.md` files.
