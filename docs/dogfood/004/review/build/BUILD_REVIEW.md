---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["dogfood-004", "harness-001-v2"]
---

# Wrapper Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Run: run_341193641a8e4e528333a704908acda4
Inputs read (fresh context, repo-level access):

- `.striatum/bin/claude-supervised-wrapper.sh` (the script);
- `tests/test_claude_supervised_wrapper.py` (the tests);
- `docs/dogfood/004/BUILD_HANDOFF.md`;
- `docs/dogfood/004/DESIGN_SYNTHESIS.md`;
- `docs/dogfood/004/review/design/DESIGN_REVIEW.md`;
- `docs/dogfood/004/decisions/WRAPPER_ACCEPTANCE.md`;
- `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/rfcs/0009-long-lived-process-supervision.md`,
  `docs/rfcs/0010-tool-harness-profiles.md`,
  `docs/dogfood/003/findings/HARNESS-001.md`, `README.md`,
  `CHANGELOG.md`.

Verdict intent: **accept**.

The implementation matches the accepted design, adopts every
design-review follow-up F1–F6, and the V1.5 lint warning closed
exactly as predicted. Findings below are informational only.

## Schema and contract correctness

### Wrapper script (`.striatum/bin/claude-supervised-wrapper.sh`)

- Permissions: `-rwxr-xr-x` (755). Executable bit set as required.
- Shebang: `#!/usr/bin/env bash`. Standard, portable.
- `set -euo pipefail`: enabled. Combined with the `while read`
  pattern, EOF on stdin terminates the loop cleanly without
  triggering `set -e` (verified by test 3 / 4).
- Per-packet loop: `while IFS= read -r packet`. Reads one
  newline-terminated record at a time from stdin (the supervisor's
  FIFO).
- Empty-line guard: `[ -z "$packet" ] && continue`. Defensive;
  matches the synthesis.
- Inner invocation: `printf '%s\n' "$packet" | claude --print
  >/dev/null 2>&1 &` then `wait "$inner_pid" || true`. Backgrounded
  + `wait` is correct for the SIGTERM-forwarding pattern (a
  synchronous invocation would block the trap).
- F1 finding adopted: `--output-format stream-json` is gone. The
  inner stdout is DEVNULL'd anyway, so the flag added nothing.
- SIGTERM trap: forwards to `$inner_pid` if alive (`kill -0`);
  exits 0. Trap is set before the loop starts; the in-flight
  child is the only thing that needs cleanup on supervisor stop.
- Comments: docstring explains the per-packet decision, the null-
  byte caveat (F2 finding from design review captured), and exit
  semantics. Adequate.

The script is the smallest piece of code that satisfies RFC 0009's
supervised-lane contract. No improvisation beyond the accepted
design.

### Verification test (`tests/test_claude_supervised_wrapper.py`)

- `pytestmark` skips the file cleanly when `bash` is unavailable
  or the wrapper is missing. Conservative.
- `_stub_claude` builds a fresh `claude` per test that records its
  stdin and exit code. Tests do not require the real Claude
  binary.
- `_spawn_wrapper` uses `os.open(..., O_RDONLY | O_NONBLOCK)` then
  `fcntl(F_SETFL, ~O_NONBLOCK)` to avoid the FIFO-open deadlock.
  This is a real Python/POSIX gotcha that the implementer hit and
  fixed in-place; the helper's docstring explains it. Future
  authors of similar tests benefit.
- Test 1 (multiple packets): asserts three `---END---` markers and
  the JSON content of each input. Would fail under "consume all
  input then invoke once" or under "invoke once per byte" patterns.
- Test 2 (failing inner): inner exit 1; wrapper still consumes a
  second packet. Would fail under "abort on first failure."
- Test 3 (empty-input EOF): writer opens and closes immediately;
  wrapper exits 0. Catches "deadlock on no-input" regressions.
- Test 4 (one-packet-then-EOF): F3 finding adopted. Sends one
  packet, asserts one `---END---`, then writer closes; wrapper
  exits 0. Distinguishes from test 3.

Ran the tests locally: 4 passed in 0.39s. Build-handoff records
the same number.

## Backwards compatibility and supervisor-contract compliance

- `.striatum/state.sqlite3` untouched. Verified by file inspection
  and by running `make test` (no migration triggered).
- The wrapper's only `.striatum/` write is the script itself at
  `.striatum/bin/`. The `.striatum/scratch/` and supervisor state
  paths are owned by the runner, not the wrapper.
- Stdout/stderr posture: supervisor DEVNULLs the wrapper; the
  wrapper additionally DEVNULLs the inner `claude`. No transcripts
  captured anywhere. Honors D028 / RFC 0009.
- The wrapper does not parse JSON, does not maintain state, and
  does not use the `striatum` CLI itself — it only delivers the
  packet to the inner process. The inner process (which would be a
  Striatum-aware Claude Code session in production) is responsible
  for `striatum ack`, `publish-artifact`, etc.

## Provider-specific behaviour stays in the wrapper, not core

- Core scheduler, supervisor, and adapter modules are unchanged.
- The wrapper is per-tool (Claude Code only). Codex's `codex exec`
  per-packet model and Gemini CLI's `--prompt -` shape do not need
  wrappers and are not affected.
- The harness-profile system from RFC 0010 V1 still treats the
  wrapper as just a lane command path; nothing in V1 had to change.

## Native subagent guidance

The wrapper spawns short-lived `claude` invocations; any sub-agents
those processes spawn die with the parent. The wrapper itself is
the only Striatum-tracked process. `accountability.native_subagents
= internal_to_parent_session` is preserved by construction.

## V1.5 lint closure (re-verifying F4)

Independently ran:

```bash
$ .venv/bin/striatum --repo . workflow validate \
    docs/dogfood/004/workflow.json --json
{"data":{"valid":true,"workflow_id":
"dogfood-004-claude-supervised-wrapper"},"ok":true}

$ .venv/bin/striatum --repo . workflow validate \
    examples/harness-profiles/workflow.json --json
{"data":{"valid":true,"workflow_id":
"harness-profiles-fixture"},"ok":true}
```

No `warnings` array on either workflow. The V1.5 lint correctly
goes silent now that the wrapper exists. RFC 0010's V1.5 surface
is closed by V2's existence.

## Tests, lint, typecheck

Independently ran `make test`, `make lint`, `make typecheck`:

- `make test`: 188 passed (185 prior + 4 new wrapper - 1 absorbed
  total = 188 actual; verified locally).
- `make lint`: clean.
- `make typecheck`: clean (39 source files).

Matches the build-handoff's recorded results.

## Documentation accuracy

| Doc | Update | Accurate? |
|---|---|---|
| `docs/SPEC.md` "Supervised Lane Command Contract" | Reference wrapper paragraph | yes |
| `docs/UBIQUITOUS_LANGUAGE.md` | "supervised lane wrapper" entry | yes |
| `docs/rfcs/0010-tool-harness-profiles.md` | "V2 Implementation Slice" section | yes |
| `docs/rfcs/0009-long-lived-process-supervision.md` | Status line wrapper pointer | yes |
| `docs/dogfood/003/findings/HARNESS-001.md` | Status flipped to `resolved` with V1.5 + V2 references | yes |
| `README.md` | Wrapper paragraph under Harness Profiles | yes |
| `CHANGELOG.md` | Two-paragraph entry per F6 | yes |

## Findings

### F1 (info) — Test platform assumption

**Issue.** The test imports `fcntl`, which is POSIX-only. On
Windows (even via WSL with bash) the import would fail.

**Recommendation.** None required. Striatum is POSIX-only by
design (D020 forbids hosted services; the supervisor uses
`os.mkfifo`, also POSIX-only). The skipif on `bash` already covers
non-POSIX systems indirectly.

### F2 (info) — Wrapper does not exit non-zero on `claude` not found

**Issue.** If `claude` is not on `$PATH`, the inner pipeline fails
with "command not found" but the `|| true` after `wait` swallows
it. The wrapper continues to consume packets without doing
anything. Lease expiry eventually triggers `doctor` surfacing.

**Recommendation.** None required. This is the documented
degraded-mode behaviour from the design synthesis. A future
enhancement could add a `command -v claude || exit 1` precheck;
for V2, lease expiry is the sufficient signal.

### F3 (info) — `inner_pid` reset window

**Issue.** Between `wait "$inner_pid" || true` returning and
`inner_pid=""` running, a SIGTERM trap could fire and read the
old `$inner_pid`. The trap's `kill -0` check protects against
killing an unrelated PID, but the window is microscopic and the
mitigation is in place.

**Recommendation.** None required. Documented for completeness.

## Verdict

**accept.** The build slice is correct, well-tested, fully matches
the accepted design plus all six design-review follow-ups (F1–F6),
and closes the V1.5 lint surface. Findings F1–F3 are informational
only.
