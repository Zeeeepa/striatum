# Reviewer Role (Dogfood 004)

You review the wrapper design or the wrapper implementation with
fresh context. Do not rely on memory of earlier rounds.

For the design review (`review_wrapper_design`):

- inspect `docs/dogfood/004/DESIGN_SYNTHESIS.md` and the prior
  research handoff;
- assess whether the chosen `claude` invocation form actually
  supports long-lived stdin streaming under named-pipe back-pressure;
- assess whether the verification test would actually fail if the
  wrapper used a wrong invocation form (a test that always passes
  is no test);
- check that transcripts stay off and `.striatum/state.sqlite3` is
  untouched;
- check that the design preserves Striatum's no-hosted-services and
  no-transcript boundaries;
- write `docs/dogfood/004/review/design/DESIGN_REVIEW.md` as a
  `finding` artifact and submit a structured verdict.

For the build review (`review_wrapper_build`):

- inspect the actual `.striatum/bin/claude-supervised-wrapper.sh`
  file, the verification test, and any docs the implementer changed;
- run the verification test locally if you can;
- check that the wrapper is executable, has `set -euo pipefail` (or
  Python equivalent if it ships in Python), and DEVNULLs stdout/
  stderr per RFC 0009;
- write `docs/dogfood/004/review/build/BUILD_REVIEW.md` as a
  `finding` artifact and submit a verdict.

Use `accept` or `accept_with_findings` only if a human could
reasonably approve and either land the wrapper (design review) or
ship it as run output (build review).
