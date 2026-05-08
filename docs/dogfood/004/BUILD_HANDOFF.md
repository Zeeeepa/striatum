---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# Wrapper Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: run_341193641a8e4e528333a704908acda4
Decision artifact:
`docs/dogfood/004/decisions/WRAPPER_ACCEPTANCE.md`
(decision_id `dec_191214fea393400db73657720b6181bc`,
outcome `accepted_with_follow_up`).

## Scope landed

The reviewed V2 slice from `DESIGN_SYNTHESIS.md`, with the design-
review F1 finding (drop `--output-format stream-json` from the inner
invocation) and F3 finding (one-packet-then-EOF test variant) adopted
inline.

## Files changed

- **`.striatum/bin/claude-supervised-wrapper.sh`** (new, `chmod 755`)
  — bash `while IFS= read -r` loop spawning a fresh `claude --print`
  per packet. SIGTERM trap forwards to the in-flight inner process.
  Inner stdout/stderr → `/dev/null`. The `.striatum/bin/` carve-out
  in the implementer's write_scope is the only `.striatum/` write.
- **`tests/test_claude_supervised_wrapper.py`** (new) — 4 tests:
  - `test_wrapper_handles_multiple_packets` — three packets in,
    three stub-claude invocations out;
  - `test_wrapper_survives_failing_inner_claude` — non-zero inner
    exit does not kill the wrapper;
  - `test_wrapper_exits_cleanly_on_writer_eof` — empty input case;
  - `test_wrapper_exits_cleanly_after_one_packet_then_eof` —
    F3 finding; one packet then writer-close exits clean.
  Tests substitute a stub `claude` on `$PATH`; do not require the
  real binary. The spawn helper opens the FIFO with `O_NONBLOCK |
  O_RDONLY`, clears the flag, hands the fd to `subprocess.Popen`,
  and closes the parent's copy — avoids the FIFO-open deadlock
  that would otherwise block `Popen` until a writer connects.
- **`docs/SPEC.md`** — supervised lane contract subsection now
  points at the reference wrapper.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — F5 finding adopted: new
  "supervised lane wrapper" entry.
- **`docs/rfcs/0010-tool-harness-profiles.md`** — new "V2
  Implementation Slice" section before V1.
- **`docs/rfcs/0009-long-lived-process-supervision.md`** — status
  line updated with reference-wrapper pointer.
- **`docs/dogfood/003/findings/HARNESS-001.md`** — status flipped
  from `proposed` to `resolved`.
- **`README.md`** — one new paragraph under "Harness Profiles".
- **`CHANGELOG.md`** — F6 finding adopted: two-paragraph entry
  noting per-packet semantics.

## V1.5 lint closure (F4)

Before the wrapper file existed:

```bash
$ .venv/bin/striatum --repo . workflow validate \
    docs/dogfood/004/workflow.json --json
{"data":{"valid":true,"warnings":["lane 'claude_code' command path
'.striatum/bin/claude-supervised-wrapper.sh' does not exist under
/home/halbritt/git/striatum; supervised use will fail at exec time.
(RFC 0010 V1.5 lint; HARNESS-001 follow-up)"],"workflow_id":
"dogfood-004-claude-supervised-wrapper"},"ok":true}
```

After:

```bash
$ .venv/bin/striatum --repo . workflow validate \
    docs/dogfood/004/workflow.json --json
{"data":{"valid":true,"workflow_id":
"dogfood-004-claude-supervised-wrapper"},"ok":true}
```

The V1.5 lint surface was the bridge between RFC 0010 V1 (no wrapper,
warning fires) and RFC 0010 V2 (wrapper exists, warning silent). Both
the dogfood-004 fixture and `examples/harness-profiles/workflow.json`
no longer emit the warning.

## Tests run

- `make test` (full suite): **188 passed** in ≈130s. Up from 184
  (pre-V2: dogfood-003 + V1.5 baseline) — added 4 new wrapper tests.
- `tests/test_claude_supervised_wrapper.py`: 4 passed in 0.39s.
- `make lint`: clean (`ruff check .`).
- `make typecheck`: clean (`mypy`).

## Validation against design-review findings

| Finding | Status | Notes |
|---|---|---|
| F1 (drop `--output-format stream-json`) | done | wrapper invokes plain `claude --print >/dev/null 2>&1`. |
| F2 (null-byte note in script header) | done | docstring covers it. |
| F3 (EOF-after-one-packet test) | done | new `test_wrapper_exits_cleanly_after_one_packet_then_eof`. |
| F4 (BUILD_HANDOFF records V1.5 lint closure) | done | before/after captured above. |
| F5 (UBIQUITOUS_LANGUAGE entry) | done | "supervised lane wrapper" added. |
| F6 (CHANGELOG noting per-packet semantics) | done | two-paragraph entry. |

## Deferred work (out of scope for V2)

These remain follow-up RFC items per the design synthesis:

- Long-lived `claude` session via `--input-format stream-json`
  multi-turn input (unverified upstream).
- MCP-based supervision (Striatum-as-MCP-server).
- Per-packet skill installation (`.claude/skills/` Striatum bundle).
- Real-claude smoke test in CI.
- Worktree-aware wrapper variant.

## How to verify in this checkout

```bash
.venv/bin/python -m pytest tests/test_claude_supervised_wrapper.py -v
ls -la .striatum/bin/claude-supervised-wrapper.sh
.venv/bin/striatum --repo . workflow validate docs/dogfood/004/workflow.json --json
```

The wrapper file is `chmod 755`; the validate command no longer emits
a warnings array.

## Harness friction encountered

1. **FIFO open deadlock in test setup.** A naive
   `subprocess.Popen([...], stdin=open(fifo, "rb"))` blocks because
   opening a FIFO for read normally blocks until a writer connects.
   The fix is to open the read side with `O_NONBLOCK`, clear the
   flag, then pass the fd to Popen. Proposing a follow-up
   `harness_improvement_proposal` is unnecessary — this is a generic
   Python-FIFO gotcha, not a Striatum issue, and the test now
   documents the pattern in its `_spawn_wrapper` helper.

No other friction this run.
