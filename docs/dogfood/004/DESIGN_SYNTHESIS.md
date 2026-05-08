---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/004/research/PIPE_BEHAVIOR.md", "docs/SPEC.md", "docs/rfcs/0009-long-lived-process-supervision.md", "docs/rfcs/0010-tool-harness-profiles.md", "docs/dogfood/003/findings/HARNESS-001.md", "docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md", "src/striatum/cli/supervise.py"]
---

# Wrapper Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V2 of `docs/dogfood/003/findings/HARNESS-001.md` — author
`.striatum/bin/claude-supervised-wrapper.sh` so that workflows
declaring a supervised Claude Code lane (per RFC 0009) actually run.

This synthesis takes the research handoff at
`docs/dogfood/004/research/PIPE_BEHAVIOR.md` and pins the smallest
implementation slice that satisfies the supervised lane contract from
SPEC's "Supervised Lane Command Contract" subsection.

## Decision: per-packet `claude -p` inside a bash loop

The wrapper is a bash `while IFS= read -r` loop that spawns a fresh
`claude --print --output-format stream-json` per packet. Stdout and
stderr of the inner `claude` process are redirected to `/dev/null`.

Rationale (from PIPE_BEHAVIOR.md):

- Claude Code's `-p` mode is single-shot per its public docs.
  `--input-format stream-json` is documented for single-turn
  structured input; multi-turn streaming over a long-lived process
  is unverified. The conservative V2 choice avoids that uncertainty.
- Striatum work packets are independent: each carries its own lease,
  job id, write scope, and CLI-callback commands. Per-packet
  fresh-context is the natural shape and matches
  `fresh_session_required: true` jobs by default.
- A bash loop is the smallest piece of code that satisfies the
  contract. No coproc, no state, no JSON parsing.

## Wrapper script (final form)

`.striatum/bin/claude-supervised-wrapper.sh`:

```bash
#!/usr/bin/env bash
# RFC 0009 / RFC 0010 V2 supervised lane wrapper for Claude Code.
#
# Reads newline-terminated work packets from stdin (the supervisor's
# named pipe at .striatum/scratch/<supervisor_id>/stdin.pipe) and
# spawns a fresh `claude --print` invocation per packet. The agent
# advances workflow state via `striatum` CLI commands the packet
# tells it to invoke.
#
# Stdin: newline-delimited JSON packets (Striatum supervisor delivers
#        one packet per line via os.mkfifo).
# Stdout/stderr: routed to /dev/null. Per RFC 0009 the supervisor
#                already DEVNULLs the wrapper's own stdout/stderr;
#                this is belt-and-braces for standalone use.
# Exit: 0 on EOF (writer-close), non-zero only on signal kills.
#
# Lifecycle:
#   - The loop exits cleanly when the supervisor closes the FIFO
#     (`supervise stop`) → EOF → `read` returns non-zero → loop ends.
#   - SIGTERM from `supervise stop` interrupts an in-flight `claude`
#     invocation; the trap below cleans up the inner process.
#   - A failing inner `claude` does NOT kill the wrapper; the next
#     packet gets a fresh process. Failed packets surface via lease
#     expiry, not via the wrapper.
set -euo pipefail

# Forward SIGTERM to a child claude process if one is running; do not
# leak it after `supervise stop`.
inner_pid=""
on_term() {
  if [ -n "$inner_pid" ] && kill -0 "$inner_pid" 2>/dev/null; then
    kill -TERM "$inner_pid" 2>/dev/null || true
  fi
  exit 0
}
trap on_term TERM INT

while IFS= read -r packet; do
  [ -z "$packet" ] && continue
  printf '%s\n' "$packet" \
    | claude --print --output-format stream-json \
        >/dev/null 2>&1 \
    &
  inner_pid=$!
  wait "$inner_pid" || true
  inner_pid=""
done
```

Permissions: `chmod 755`. The file is tracked under git so the
`.striatum/bin/` carve-out (the only permitted `.striatum/` write
in workflows that reference it) is meaningful.

## Verification test (final form)

`tests/test_claude_supervised_wrapper.py`:

```python
"""RFC 0010 V2 / HARNESS-001: verify the supervised wrapper."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import time
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
WRAPPER = ROOT / ".striatum" / "bin" / "claude-supervised-wrapper.sh"


pytestmark = pytest.mark.skipif(
    shutil.which("bash") is None,
    reason="bash not available",
)


def _spawn(wrapper: Path, fifo: Path, env: dict[str, str]) -> subprocess.Popen[bytes]:
    return subprocess.Popen(
        [str(wrapper)],
        stdin=open(fifo, "rb", buffering=0),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env=env,
    )


def test_wrapper_handles_multiple_packets_with_stub_claude(tmp_path: Path) -> None:
    """The wrapper must spawn one `claude` per newline-terminated packet."""
    log = tmp_path / "claude.log"
    stub = tmp_path / "claude"
    stub.write_text(
        "#!/usr/bin/env bash\n"
        f"cat >> {log}\n"
        f"printf '\\n---END---\\n' >> {log}\n",
        encoding="utf-8",
    )
    stub.chmod(0o755)

    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    env = os.environ.copy()
    env["PATH"] = f"{tmp_path}:" + env["PATH"]
    proc = _spawn(WRAPPER, fifo, env)

    try:
        with open(fifo, "wb") as writer:
            for i in range(3):
                writer.write((json.dumps({"i": i}) + "\n").encode())
                writer.flush()
                time.sleep(0.05)
        # Closing the writer triggers EOF on the wrapper's stdin.
        rc = proc.wait(timeout=10)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
    contents = log.read_text(encoding="utf-8")
    assert contents.count("---END---") == 3, contents
    for i in range(3):
        assert json.dumps({"i": i}) in contents


def test_wrapper_survives_failing_inner_claude(tmp_path: Path) -> None:
    """A non-zero `claude` exit must not kill the wrapper loop."""
    log = tmp_path / "claude.log"
    stub = tmp_path / "claude"
    stub.write_text(
        "#!/usr/bin/env bash\n"
        f"cat >> {log}\n"
        f"printf '\\n---END---\\n' >> {log}\n"
        "exit 1\n",
        encoding="utf-8",
    )
    stub.chmod(0o755)

    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    env = os.environ.copy()
    env["PATH"] = f"{tmp_path}:" + env["PATH"]
    proc = _spawn(WRAPPER, fifo, env)

    try:
        with open(fifo, "wb") as writer:
            for i in range(2):
                writer.write((json.dumps({"i": i}) + "\n").encode())
                writer.flush()
                time.sleep(0.05)
        rc = proc.wait(timeout=10)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
    assert log.read_text(encoding="utf-8").count("---END---") == 2


def test_wrapper_exits_cleanly_on_writer_eof(tmp_path: Path) -> None:
    """Closing the writer side must cause the wrapper to exit 0."""
    stub = tmp_path / "claude"
    stub.write_text("#!/usr/bin/env bash\ncat > /dev/null\n", encoding="utf-8")
    stub.chmod(0o755)

    fifo = tmp_path / "stdin.pipe"
    os.mkfifo(fifo)

    env = os.environ.copy()
    env["PATH"] = f"{tmp_path}:" + env["PATH"]
    proc = _spawn(WRAPPER, fifo, env)
    try:
        # Open and immediately close the writer side without sending
        # any packets.
        open(fifo, "wb").close()
        rc = proc.wait(timeout=5)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait(timeout=5)

    assert rc == 0
```

The three tests cover the contract surface:

- **multiple packets**: the loop semantics are right — one `claude`
  invocation per packet, not "consume all input then invoke once."
- **failing inner**: a crashed `claude` does not kill the wrapper.
- **writer EOF**: `supervise stop` semantics are honored.

If a deliberate breakage is introduced (e.g., remove the `while`
loop and `exec claude -p` once instead), the multi-packet test
fails. Reviewer can verify by patching the wrapper.

The tests do **not** require a real `claude` binary — they install a
stub on `$PATH` that records its stdin and exits. A separate
optional smoke test could `shutil.which`-gate a real `claude`
invocation, but it is out of scope for V2 (real `claude` invocation
quality is a manual operator concern).

## Stdout/stderr posture

Per RFC 0009 and D028:

- Supervisor pipes wrapper stdout/stderr to DEVNULL.
- Wrapper redirects inner `claude` stdout/stderr to DEVNULL inside the
  loop. This matters for standalone invocation (debug runs, tests)
  where the supervisor isn't in the picture.
- The agent's only durable output is artifacts and verdicts written
  via `striatum` CLI commands — same as Codex today.

No transcripts captured anywhere. The wrapper itself does not log.

## Error handling

| Scenario | Wrapper behaviour |
|---|---|
| Empty line on stdin | `continue` (skip; defensive). |
| Malformed JSON in packet | `claude` will fail and emit error to its stdout (DEVNULL'd). Wrapper continues; lease will eventually expire. |
| `claude` not on `$PATH` | First iteration fails with "command not found"; the `\|\| true` at the inner pipeline keeps the loop alive but no work happens. Lease expiry surfaces via doctor. |
| `claude` crashes mid-packet | `wait $inner_pid` returns non-zero; `\|\| true` swallows it; next packet gets a fresh process. |
| SIGTERM from `supervise stop` | `on_term` trap forwards SIGTERM to the in-flight inner `claude` (if any), then the wrapper exits 0. |
| Writer closes FIFO (`supervise stop`) | `read` returns non-zero (EOF); `while` loop ends; wrapper exits 0. |
| Wrapper itself crashes | Supervisor's `process_supervisors` row eventually transitions to `lost`; doctor flags `supervisor_lost_with_held_lease`. |

## Install and permissions story

- File path: `.striatum/bin/claude-supervised-wrapper.sh`. Tracked
  under git. The `.striatum/bin/` carve-out exists in the
  dogfood-004 implementer's `write_scope` exactly for this file.
- Executable bit set in the implementer's commit. CI honors the
  file mode (verified for prior shell scripts in `scripts/`).
- No version pinning needed; the wrapper is portable bash.
- No external dependencies beyond `bash`, `claude`, and POSIX
  utilities (`kill`, `printf`).

## Documentation updates the implementer must make

- **`docs/SPEC.md` "Supervised Lane Command Contract"** — add a
  sentence noting that the reference wrapper for Claude Code lives
  at `.striatum/bin/claude-supervised-wrapper.sh` and link to RFC
  0010's V2 implementation note.
- **`docs/rfcs/0010-tool-harness-profiles.md`** — add a "V2
  Implementation" section under "V1 Implementation Slice" that
  records HARNESS-001 V2 closure: wrapper shipped, verification
  test landed.
- **`docs/rfcs/0009-long-lived-process-supervision.md`** — add a
  reference-implementation pointer to the wrapper script.
- **`docs/dogfood/003/findings/HARNESS-001.md`** — flip status to
  "resolved" with a pointer to dogfood-004 evidence.
- **`docs/dogfood/004/BUILD_HANDOFF.md`** — implementer's handoff.
- **`README.md`** — note the wrapper exists; one line under
  "Harness Profiles".
- **`CHANGELOG.md`** — Unreleased entry under Added.

## Explicit deferrals

- **Long-lived `claude` session via stream-json input.** Possible
  future optimization if Anthropic documents the behavior; out of
  scope for V2.
- **MCP-based supervision.** Striatum-as-MCP-server with `claude`
  calling back via MCP rather than shell-out. Out of scope; would
  need a separate RFC.
- **Per-packet skill installation.** Deploying a `.claude/skills/`
  Striatum skill bundle so the agent has Striatum CLI commands as
  first-class slash commands. Useful future work; out of scope for
  V2 because it requires authoring + maintaining a skill bundle
  separate from the wrapper.
- **Real-claude smoke test in CI.** A test that invokes the actual
  `claude` binary against a trivial packet. Out of scope because
  CI does not have an Anthropic API key by default and the test
  would need network access.
- **Worktree-aware wrapper variant.** A future variant that
  pre-launches `git worktree`s before invoking `claude`. Out of
  scope; RFC 0008 already owns worktree creation at the
  Striatum-job level, not the wrapper level.

## Build slice (one follow-up job)

The implementer ships these changes in one PR-shaped diff:

1. `.striatum/bin/claude-supervised-wrapper.sh` (new, executable)
   — exact text in this synthesis.
2. `tests/test_claude_supervised_wrapper.py` (new) — three tests
   above.
3. `docs/SPEC.md` — one paragraph in the supervised lane contract.
4. `docs/rfcs/0010-tool-harness-profiles.md` — V2 section.
5. `docs/rfcs/0009-long-lived-process-supervision.md` — reference
   wrapper pointer.
6. `docs/dogfood/003/findings/HARNESS-001.md` — status flip.
7. `docs/dogfood/004/BUILD_HANDOFF.md` — required handoff
   artifact.
8. `README.md`, `CHANGELOG.md` — one-line additions.

The implementer must run `make lint`, `make typecheck`, `make
test` before publishing the build handoff. The new tests should
add 3 passing cases; the existing 184 should still pass.

## Open questions for review

- Should the wrapper add a per-packet log line (e.g., `printf
  '%s\n' "$(date -Iseconds): packet" >&2`) for debug? V2
  recommendation: no — supervisor DEVNULLs stderr, so the line is
  invisible anyway, and adding it complicates the contract.
- Should `chmod 755` be enforced via a test that asserts the
  permission, or just relied on at commit time? V2: rely on commit
  time; pre-commit hook (if added) can enforce.
- Should the wrapper accept a `--version` flag for the
  doctor/diagnostics surface? V2: no — keep the wrapper as the
  smallest piece that satisfies the contract.

## Acceptance gate

Per the dogfood-004 SKILL.md and `prompts/implement_wrapper.md`,
the implementation job blocks until a human acceptance decision is
recorded under `docs/dogfood/004/decisions/`. This synthesis does
not authorize implementation.
