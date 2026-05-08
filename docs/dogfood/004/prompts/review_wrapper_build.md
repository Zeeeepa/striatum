# Review Wrapper Implementation

Review the accepted wrapper build slice with fresh context and
repo-level access. Inspect the changed code, the wrapper script
itself, the verification test, the docs, and the build handoff.

Focus on:

- whether `.striatum/bin/claude-supervised-wrapper.sh` exists, is
  executable (`chmod 755` or equivalent), and pins the `claude`
  invocation form the accepted design specified;
- whether the verification test actually drives the wrapper via
  `os.mkfifo` and would fail under a wrong invocation form;
- whether transcripts stay off (RFC 0009 / D028) and stdout/stderr
  go to DEVNULL by default;
- whether the wrapper's only `.striatum/` write target is itself —
  no other `.striatum/` paths get touched;
- whether the changed docs accurately describe the new behavior
  (especially RFC 0009/0010 status updates and SPEC's supervised-
  lane contract);
- whether `make lint`, `make typecheck`, and `make test` are clean
  on the implementer's branch;
- whether the workflow-validate lint warning for the missing wrapper
  goes away when the wrapper file exists (run `striatum workflow
  validate docs/dogfood/004/workflow.json --json` and check that
  the `warnings` array no longer names the wrapper).

Write `docs/dogfood/004/review/build/BUILD_REVIEW.md` as a `finding`
artifact and submit a structured verdict. Use `needs_revision` for
issues that must be fixed before the run can finish.
