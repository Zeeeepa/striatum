# Dogfood 004 Source List

Status: draft
Date: 2026-05-08

Use this file as the starting source list for the wrapper research and
design jobs. Verify each source is still current; prefer official
documentation over blogs or memory.

## Striatum-Side

- `docs/SPEC.md` (Supervised Lane Command Contract section).
- `docs/rfcs/0009-long-lived-process-supervision.md` — supervisor pipe
  contract; what the wrapper must satisfy.
- `docs/rfcs/0010-tool-harness-profiles.md` — `claude_code_default`
  profile and the `supervision.wrapper_required: true` declaration.
- `docs/dogfood/003/findings/HARNESS-001.md` — V2 step is the target
  of this run.
- `docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md` — the prior
  research that flagged the wrapper requirement.
- `src/striatum/cli/supervise.py`, `src/striatum/process_adapter.py`,
  `tests/test_supervise.py` — the supervisor's stdin pipe code and
  existing tests.

## Claude Code

- [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference)
  — flags, exit codes, `--print`, `--input-format`, `--output-format`.
- [Interactive mode](https://code.claude.com/docs/en/interactive-mode)
  — what `claude` does without a TTY.
- Claude Code release notes (look for `--input-format stream-json`
  changes since 2026-Q1).

Research focus:

- Whether `claude --print --output-format stream-json --input-format
  stream-json` reads stdin in a way that preserves per-line packet
  semantics under `os.mkfifo` back-pressure.
- Whether the process exits when stdin EOF is observed, or stays
  alive across multiple JSON inputs.
- Whether a single long-lived `claude` session can be fed multiple
  user turns via stream-json input, or whether each invocation is
  one-shot regardless of `--input-format`.
- Buffering surprises (line-buffered vs block-buffered) that change
  behavior between regular pipes and named pipes.

## POSIX / Python

- [`os.mkfifo`](https://docs.python.org/3/library/os.html#os.mkfifo)
  — POSIX named-pipe semantics, blocking-open behavior, EOF on writer
  close.
- [`subprocess.Popen` with named pipes](https://docs.python.org/3/library/subprocess.html)
  — how to pass an `os.mkfifo` to a child process as stdin.

Research focus:

- Whether the wrapper should `exec claude ...` or `python -c '...'` to
  drive `claude` from a coproc loop.
- Whether the wrapper needs its own stdin loop (read packet, write to
  `claude`'s stdin, repeat) or whether `claude` can consume the pipe
  directly.
- Whether stderr/stdout of the wrapper need to be `/dev/null`-routed
  (per RFC 0009 the supervisor sends them there anyway).

## Reference Implementations

- The dogfood-003 `examples/harness-profiles/workflow.json` lane
  command is the **caller** of the wrapper; the wrapper must satisfy
  the lane command contract.
- `src/striatum/cli/supervise.py:supervise_send` for the format the
  wrapper will see on its stdin.
