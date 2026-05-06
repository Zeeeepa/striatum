# agent_runner

Local-first orchestration for multiple terminal-based AI coding agents.

This project is being specified through an interview-driven design process.

This directory is temporarily incubated inside Engram so the MVP design can use
the real multi-agent workflow that motivated it. After MVP validation, split it
into a standalone project.

Start with:

1. [docs/README.md](docs/README.md)
2. [docs/PRD.md](docs/PRD.md)
3. [docs/DECISION_LOG.md](docs/DECISION_LOG.md)
4. [docs/UBIQUITOUS_LANGUAGE.md](docs/UBIQUITOUS_LANGUAGE.md)
5. [docs/PRIOR_ART.md](docs/PRIOR_ART.md)
6. [docs/SPEC.md](docs/SPEC.md)
7. [docs/INTERVIEW_LOG.md](docs/INTERVIEW_LOG.md)
8. [docs/ENGRAM_INCUBATION_CONTEXT.md](docs/ENGRAM_INCUBATION_CONTEXT.md)

First execution prompt:

- [prompts/P001_design_review_build_v1_mvp.md](prompts/P001_design_review_build_v1_mvp.md)

Bootstrap tmux runner:

```bash
agent-runner/scripts/agent_runner_tmux_design.sh start
tmux attach -t agent-runner-design
```

Use `start-pipe` or `AGENT_RUNNER_RUN_MODE=pipe` when the local model CLIs are
ready to accept prompts on stdin. The runner starts Claude, Codex, and Gemini
design-input lanes plus a synthesis handoff pane.
