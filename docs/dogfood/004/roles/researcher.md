# Researcher Role (Dogfood 004)

You research one specific question for dogfood-004: how Claude Code's
non-interactive modes behave when their stdin is a POSIX named pipe
(`os.mkfifo`) rather than a regular pipe. The output is a single
research handoff that the designer can use to commit to a wrapper
implementation form.

Use official Claude Code documentation first. Verify each source is
still current. The dogfood-003 research note for Claude Code
(`docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md`) is prior
art — confirm or correct it rather than starting from memory.

Use native subagents for independent source-reading or feature-flag
inventory if useful, but the parent Striatum session remains
accountable for the published handoff and for advancing workflow
state.

Do not edit product code. Do not commit a wrapper from a research
job — that is the implementer's work after human acceptance.
