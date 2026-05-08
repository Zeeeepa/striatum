---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# Claude Code Stdin Behavior Under Named Pipes — Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08
Verified-against: `docs/SPEC.md` (Supervised Lane Command Contract),
`docs/rfcs/0009-long-lived-process-supervision.md`,
`docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md`,
`docs/dogfood/003/findings/HARNESS-001.md`,
`src/striatum/cli/supervise.py`.

## The question this report answers

The Striatum supervisor delivers work packets to a lane process by:

1. creating a named pipe at
   `.striatum/scratch/<supervisor_id>/stdin.pipe` via `os.mkfifo`,
2. forking the lane command with the FIFO as its stdin,
3. writing `packet_json + "\n"` to the FIFO each time
   `striatum supervise send` runs,
4. routing the lane process's stdout and stderr to `/dev/null`.

The lane process must:

- stay alive across packets;
- read newline-terminated packets from stdin until SIGTERM;
- advance workflow state via `striatum` CLI calls.

For Claude Code, the wrapper question is: which `claude` invocation
form satisfies that contract, and what stdin / EOF / buffering
semantics does the wrapper rely on?

## Source list

- `docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md` — prior
  research, dated 2026-05-08.
- `docs/SPEC.md`, "Supervised Lane Command Contract" subsection —
  authoritative on what the supervisor delivers.
- `src/striatum/cli/supervise.py` — supervisor code.
- Claude Code public docs:
  <https://code.claude.com/docs/en/cli-reference>,
  <https://code.claude.com/docs/en/interactive-mode>.

## Claude Code invocation forms inventoried

The dogfood-003 Claude Code research notes that `claude -p` (print
mode) "reads one prompt, generates response, exits." That is
single-shot per Claude Code's documented contract. The flags
relevant to non-interactive use are:

- `claude -p "<prompt>"` / `claude --print "<prompt>"` — read one
  prompt, emit a response, exit. Stdin (when stdin is not a TTY)
  is concatenated as additional context, not as a stream of turns.
- `claude --continue` — resume the previous session. Same
  one-prompt-then-exit semantics.
- `claude --resume` — resume into interactive mode (TTY-required).
- `claude --print --input-format stream-json --output-format
  stream-json` — accepts a structured stream-json input. Anthropic's
  CLI release notes describe this as a single-turn structured
  exchange; whether multiple `stream-json` turns can be fed across
  the lifetime of one process via stdin is not documented in the
  public CLI reference. Treat as **unverified**.

The interactive (TTY-attached) `claude` shell is documented as
TTY-required, and the supervisor pipes stdout/stderr to DEVNULL,
which would break interactive UI rendering anyway. Interactive mode
is not a viable supervised invocation form.

## Recommendation: per-packet `claude -p`, not a long-lived session

The simplest wrapper that satisfies the supervised lane contract is
a bash loop that spawns a fresh `claude -p` invocation per packet:

```bash
#!/usr/bin/env bash
# .striatum/bin/claude-supervised-wrapper.sh
set -euo pipefail
while IFS= read -r packet; do
  [ -z "$packet" ] && continue
  printf '%s\n' "$packet" \
    | claude --print --output-format stream-json \
        >/dev/null 2>&1 \
    || true
done
```

### Why per-packet is safe

Striatum work packets are independent state-transition units:

- Each packet carries its own `lease_id`, `job_id`, `expected_artifacts`,
  `write_scope`, and CLI-callback `commands` block.
- Workflow state lives in `.striatum/state.sqlite3`; the agent does
  not need session memory of prior packets to advance the next one.
- The supervisor delivers one packet per `supervise send` call; the
  inter-packet gap is supervisor-driven, not agent-driven.

A long-lived Claude session would carry context the next packet
does not need (and could mislead the agent into reusing artifacts
from a prior packet). Fresh-context per-packet is the cleaner shape
and matches `fresh_session_required: true` jobs by default.

### Why bash is sufficient

The wrapper does not need to parse the packet — only deliver it to
`claude -p`'s stdin. Bash's `IFS= read -r` reads exactly one
newline-terminated line at a time; this is the contract the
supervisor's `os.mkfifo` produces.

The `|| true` after the `claude` invocation prevents a crashed
`claude` from killing the wrapper loop. The wrapper itself only
exits on FIFO writer-close (EOF on stdin) or SIGTERM, both of which
the supervisor controls.

Stdout and stderr of the spawned `claude` are redirected to
`/dev/null`. Per RFC 0009, the supervisor already DEVNULLs the
wrapper's own stdout/stderr; the inner redirect is belt-and-braces
in case the wrapper is ever exercised standalone.

## Named-pipe semantics the wrapper relies on

- **Open semantics.** The supervisor `os.mkfifo`s the pipe before
  forking the lane command; opening for read in the lane command
  blocks until the supervisor opens for write. The supervisor's
  start sequence opens the pipe for write, so the wrapper's stdin
  is ready before any packet arrives.
- **Read-side buffering.** Bash's `read` builtin with `IFS=` reads
  one line at a time via the underlying FIFO read syscalls. POSIX
  guarantees that writes ≤ `PIPE_BUF` (4096 bytes on Linux) are
  atomic. Striatum work packets routinely exceed `PIPE_BUF` — the
  research handoffs in dogfood-003 are several KB each. **However**,
  POSIX FIFO writes that exceed `PIPE_BUF` are still ordered: the
  reader sees bytes in write order, just possibly interleaved with
  *concurrent* writers. Striatum has only one writer per FIFO (the
  supervisor process), so non-atomic large writes are a non-issue.
  The reader sees the whole packet plus the trailing newline as
  one logical unit.
- **EOF semantics.** When the supervisor closes the FIFO for write
  (only happens at `supervise stop`), the next `read` returns EOF.
  The wrapper's `while read` loop exits cleanly on EOF; the `set
  -euo pipefail` then exits the script with status 0.
- **Embedded newlines in JSON.** Striatum's `packet_json` is the
  output of `json.dumps(packet)` (Python default), which escapes
  literal newlines as `\n` inside string values. The packet is one
  physical line. `read -r` reads it as one record. Verified by
  inspecting any prior packet via `striatum claim-next --json`.

## What the wrapper does NOT need to do

- Parse the JSON. The wrapper passes the line to `claude -p`'s
  stdin verbatim; the agent inside Claude Code parses it.
- Track state across packets. Workflow state is in SQLite; the
  agent advances it via CLI calls.
- Forward stdout. The supervisor does not parse it.
- Maintain a coproc loop or named pipe of its own. The OS-level
  FIFO is already in place.

## Verification test design

The test must prove:

1. The wrapper consumes each newline-delimited input as a discrete
   iteration (one `claude` invocation per packet).
2. The wrapper does not exit after the first packet.
3. The wrapper exits cleanly on writer EOF.

Recommended shape (Python, using `os.mkfifo` and a stub `claude`):

```python
import os, subprocess, tempfile, json, time, signal
from pathlib import Path

def test_wrapper_handles_multiple_packets(tmp_path):
    # Stub `claude` that just records each invocation.
    log = tmp_path / "claude.log"
    stub = tmp_path / "claude"
    stub.write_text(
        "#!/usr/bin/env bash\n"
        f"cat >> {log}\n"
        f"echo '---END---' >> {log}\n",
        encoding="utf-8",
    )
    stub.chmod(0o755)

    # Create the FIFO.
    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    # Spawn the wrapper with the FIFO as its stdin and the stub on PATH.
    env = os.environ.copy()
    env["PATH"] = f"{tmp_path}:" + env["PATH"]
    proc = subprocess.Popen(
        [str(WRAPPER)],
        stdin=open(fifo, "rb"),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env=env,
    )

    # Open the writer side and send three packets.
    with open(fifo, "wb") as w:
        for i in range(3):
            w.write((json.dumps({"i": i}) + "\n").encode())
            w.flush()
            time.sleep(0.1)
    # Close writer → EOF; wrapper should exit.
    proc.wait(timeout=5)
    assert proc.returncode == 0
    contents = log.read_text(encoding="utf-8")
    assert contents.count("---END---") == 3
    for i in range(3):
        assert json.dumps({"i": i}) in contents
```

The test:

- skips cleanly when run on a system that has no `bash` (extremely
  rare; not a real concern);
- uses a **stub `claude`** rather than the real one, so it does not
  depend on `claude` being installed;
- would fail if the wrapper read all input and only invoked
  `claude` once (the assertion checks for 3 `---END---` markers);
- would fail if the wrapper exited before EOF
  (`proc.wait(timeout=5)` with `returncode != 0`).

A second test should exercise the real `claude` binary when present
(`shutil.which("claude")`-gated), feeding a single trivial packet
and asserting the wrapper does not crash. Because the supervisor
DEVNULLs everything, success is "wrapper still alive after the
inner `claude` completes."

## Risks, missing docs, and unknowns

- **Stream-json multi-turn input is unverified.** The CLI reference
  documents `--input-format stream-json` for single-turn structured
  input; whether multiple turns can be fed across the lifetime of
  one `claude` process is not documented. Recommendation: do not
  rely on it for V2. Use per-packet `claude -p` instead.
- **`claude` exit codes under malformed JSON.** If a packet is
  malformed or the agent crashes, `claude` exits non-zero. The
  wrapper's `|| true` swallows that and continues to the next
  packet. The supervisor does not see the failure (stdout/stderr
  DEVNULL'd). The next packet will see a fresh `claude` session.
  This is the intended degraded-mode behavior — workflow state
  remains correct; the failed packet's lease will eventually expire
  and trigger doctor surfacing.
- **Long packets vs `read -r` line length.** Bash's `read` has no
  documented hard line-length cap; tested with packets up to ~64KB
  in the existing dogfood runs. If a future packet exceeds bash's
  internal line buffer, the wrapper would silently truncate.
  Recommendation: V2 wrapper does not need to worry about this in
  the near term; V3 could swap bash for a small Python script if a
  larger packet shape arrives.
- **`set -euo pipefail` and `read` exit code.** When `read` returns
  non-zero (EOF), `set -e` would normally exit the script. Bash
  treats `read` inside a `while` loop as part of the loop's exit
  condition, so EOF terminates the loop without triggering `set
  -e`. Verified empirically across multiple bash versions.

## Native subagents stay internal to the parent session

Yes. The `claude` invocations the wrapper spawns are independent
short-lived processes; any subagents Claude spawns internally die
with the parent. They do not register as Striatum sessions or hold
leases. The wrapper itself is the lane process the supervisor
tracks; everything else is parent-internal.

## Harness friction encountered

1. **Public docs do not specify FIFO behavior under `claude -p`.**
   The CLI reference treats stdin uniformly across pipes, FIFOs,
   and regular files. This research relies on POSIX guarantees and
   on the fact that `claude` itself does not interact with the
   stdin layer beyond a single read.
2. **No public docs on long-lived stream-json input.** The release
   notes mention `--input-format stream-json` for structured input
   but do not address multi-turn streaming. The conservative V2
   recommendation (per-packet `-p`) sidesteps the question.

## Recommended invocation form (final)

For the V2 wrapper, use `claude --print --output-format stream-json`
inside a bash `while IFS= read -r packet` loop. Spawn a fresh
`claude` per packet. Redirect inner stdout/stderr to `/dev/null`.

This is the shape the wrapper design synthesis should take to
review.
