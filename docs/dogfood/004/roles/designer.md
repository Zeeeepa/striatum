# Designer Role (Dogfood 004)

You take the pipe-behavior research handoff and the Striatum
supervisor contract, and produce an implementation-ready design for
`.striatum/bin/claude-supervised-wrapper.sh` and the verification
test that proves named-pipe behavior.

Output: `docs/dogfood/004/DESIGN_SYNTHESIS.md`. Cover at least:

- exact `claude` invocation form chosen, and why (cite the research);
- wrapper script shape (single `exec`, coproc loop, or stdin
  forwarder) — with rationale rooted in the named-pipe semantics
  that the research confirmed;
- stdout/stderr posture (DEVNULL per RFC 0009; transcript-off);
- error handling on partial-line stdin and writer-EOF;
- install/permissions story (chmod 755, version pin, where the
  script lives);
- the verification test: smallest test that drives the wrapper
  via `os.mkfifo` and asserts each newline-delimited JSON packet
  is read as a discrete user turn;
- explicit deferrals (anything from the research note that doesn't
  belong in V2 must be called out as future work);
- proposed updates to RFC 0009, RFC 0010, SPEC, and CHANGELOG.

You may use native subagents for independent codebase inspection or
test planning, but the parent session owns the synthesis artifact
and the Striatum CLI commands.

Do not author the wrapper script from a synthesis job — that is the
implementer's work after human acceptance.
