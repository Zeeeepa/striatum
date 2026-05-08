# Research Claude Code Stdin Behavior Under Named Pipes

Read the work packet first. Include the packet's exact `author:` line
near the top of the artifact you publish.

Research one focused question: which `claude` invocation form supports
long-lived stdin streaming under POSIX named-pipe (`os.mkfifo`) input,
preserving per-line packet semantics under back-pressure, and what
buffering / EOF / lifecycle semantics the wrapper must rely on.

Start from `docs/dogfood/004/SOURCES.md`. Verify each official source
is current. The dogfood-003 Claude Code research note
(`docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md`) is prior
art; confirm or correct it.

Use native subagents for independent source-reading or feature-flag
inventory if useful. Do not delegate overlapping artifact writes.
The parent Striatum session remains accountable for the final handoff.

Publish `docs/dogfood/004/research/PIPE_BEHAVIOR.md` covering:

- the canonical `claude` invocation forms that read stdin
  non-interactively (e.g., `--print`, `--input-format stream-json`,
  `--continue`);
- which of those stay alive across multiple inputs vs. exit on EOF;
- whether stdin is line-buffered or block-buffered under a regular
  pipe and under `os.mkfifo`;
- behavior when the writer closes the pipe before the reader exits
  (EOF handling);
- behavior when partial JSON arrives across two writes;
- recommended invocation form for the V2 wrapper, with rationale;
- alternative invocation forms considered and why they were rejected;
- risks, missing docs, and unknowns the designer should treat as
  blockers vs. acceptable known unknowns;
- any harness friction encountered.

If the official docs cannot answer the buffering question definitively,
say so explicitly and recommend the smallest empirical test the
designer or implementer can run to settle it. Do not invent a
behavior claim the docs don't support.
