# Implementer Role (Dogfood 004)

Before editing code, verify a human acceptance decision exists for
the wrapper design (preferably under `docs/dogfood/004/decisions/`).
If it does not exist, call `striatum block --severity human_checkpoint`
and explain that implementation needs the recorded decision.

If accepted, ship:

1. `.striatum/bin/claude-supervised-wrapper.sh` — the wrapper script.
   Set executable permissions. Pin the `claude` invocation form
   exactly as the accepted design specifies; do not improvise.
2. A verification test that drives the wrapper via `os.mkfifo` and
   asserts each newline-delimited JSON packet is read as a discrete
   user turn. Place it under `tests/`. The test must skip cleanly on
   systems without `claude` on `$PATH`, and must fail if the wrapper
   is wired incorrectly.
3. Documentation updates: SPEC's "Supervised Lane Command Contract"
   section, the `claude_code_default` profile's `supervision` block
   in `examples/harness-profiles/workflow.json`, the dogfood-003
   workflow if relevant, README, CHANGELOG, RFC 0009/0010.
4. `docs/dogfood/004/BUILD_HANDOFF.md` listing changed files, tests
   run, deferred work, and any harness friction encountered.

Stay inside the work-packet write scope. The scope explicitly allows
`.striatum/bin/` (a carve-out from the otherwise-forbidden
`.striatum/` tree); do not write anywhere else under `.striatum/`.

Use native subagents for independent codebase inspection or test
planning if available, but keep final edits in the parent session.

Publish the build handoff and complete the job.
