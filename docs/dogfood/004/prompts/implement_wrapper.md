# Implement The Wrapper

Before editing code, verify a human acceptance decision exists for
the wrapper design (preferably under `docs/dogfood/004/decisions/`).
If it does not exist, call `striatum block --severity human_checkpoint`
and explain that implementation needs the recorded decision.

If accepted:

1. Author `.striatum/bin/claude-supervised-wrapper.sh` exactly as the
   accepted design specifies. Pin the `claude` invocation form; do
   not improvise a different form. Set executable permissions
   (`chmod 755`).
2. Author the verification test under `tests/`. The test must:
   - drive the wrapper through an `os.mkfifo`-backed stdin pipe;
   - assert that newline-delimited JSON packets are read as discrete
     user turns;
   - skip cleanly when `claude` is not on `$PATH` (use
     `shutil.which` or `pytest.importorskip` style skip);
   - actually fail when the wrapper is wired incorrectly (the
     reviewer should be able to introduce a deliberate breakage and
     observe a red test).
3. Update docs the design synthesis named:
   - `docs/SPEC.md` "Supervised Lane Command Contract" if the
     wrapper changes the contract surface in any way;
   - `docs/rfcs/0009-long-lived-process-supervision.md` to record
     V2 closure;
   - `docs/rfcs/0010-tool-harness-profiles.md` to flip the
     `claude_code_default` `supervision` block to a verified state
     if appropriate;
   - `examples/harness-profiles/workflow.json` if the lane command
     for `claude_code` should change shape now that the wrapper
     exists;
   - `README.md`, `CHANGELOG.md`.
4. Run `make lint`, `make typecheck`, `make test`. Do not skip
   failing tests; fix them.
5. Publish `docs/dogfood/004/BUILD_HANDOFF.md` listing changed files,
   tests run, deferred work, and any harness friction encountered.

Stay inside the work-packet write scope. The scope explicitly carves
out `.striatum/bin/` from the otherwise-forbidden `.striatum/` tree;
do not write anywhere else under `.striatum/` (especially not
`.striatum/state.sqlite3`).

Use native subagents for independent codebase inspection or test
planning if available, but keep final edits in the parent session.

Publish the build handoff and complete the job. Do not run the
build review yourself.
