# Synthesize Wrapper Design

Read the pipe-behavior research handoff, RFC 0009 (especially the
"Supervised Lane Command Contract" section in SPEC), the
`claude_code_default` harness profile in
`examples/harness-profiles/workflow.json`, and the supervisor-side
code at `src/striatum/cli/supervise.py`.

Produce `docs/dogfood/004/DESIGN_SYNTHESIS.md` with:

- the exact `claude` invocation form chosen (cite the research's
  recommendation; do not pick a form the research could not justify);
- wrapper script shape and language. Bash is preferred if the
  invocation form supports a single `exec claude ...`; Python (with
  a coproc loop) is acceptable if the invocation form requires the
  wrapper to read packets from its own stdin and forward them to a
  long-lived `claude` session;
- stdout/stderr posture (DEVNULL by default per RFC 0009; the
  wrapper should not write transcripts);
- error handling: what to do on partial-line stdin, on writer EOF,
  on `claude` process death, on signals;
- install and permissions story: the wrapper lives at
  `.striatum/bin/claude-supervised-wrapper.sh`, must be `chmod 755`,
  and must be tracked under git (the `.striatum/bin/` carve-out is
  the only permitted write under `.striatum/`);
- the verification test:
  - smallest test that drives the wrapper via `os.mkfifo` and
    asserts each newline-delimited JSON packet is read as a
    discrete user turn;
  - skip-cleanly behavior on systems without `claude` on `$PATH`
    (per `pytest.importorskip` or shutil-based skip);
  - assertion strategy that would actually fail if the wrapper used
    a wrong invocation form;
- explicit deferrals: anything from the research that does not
  belong in V2 (e.g., MCP-based supervision, Claude Code's own
  agent-team coordination, transcript capture);
- proposed updates to RFC 0009 (close the V2 follow-up), RFC 0010
  (transition `supervision.compatible` for `claude_code_default`
  to a verified state), SPEC's "Supervised Lane Command Contract"
  subsection, and CHANGELOG;
- a small build slice the implementer can ship in one follow-up job;
- open questions and deferred items.

Do not author the wrapper script in this synthesis job. The output is
a Markdown synthesis artifact only. Use native subagents for
independent code inspection if useful, but the parent session owns
the final synthesis.

Publish the synthesis and complete the job.
