# Build Review — RFC 0043 V1.6

Read:
- `docs/dogfood/052/DESIGN_SYNTHESIS.md`
- `docs/dogfood/052/build/HANDOFF.md`
- The source files the HANDOFF cites
  (`src/striatum/cli/daemon_required.py`, `src/striatum/db.py`,
  `src/striatum/daemon_pg/repo_local_migration.py`,
  `src/striatum/cli/parser.py`, `tests/conftest.py`,
  new test files).

Posture supplied in work packet (`threat_model`, `ergonomics_dx`,
adversarial). Write to assigned
`docs/dogfood/052/review/build/<lane>/REVIEW.md` with v1 finding front
matter.

Required checks:
- F-escape: bare `STRIATUM_DAEMON_REQUIRED=0` is rejected; only the
  test-harness pair enables it.
- F-split-brain: connect refuses fresh DB when sentinel present.
- F-lock: concurrent migrate-repo-local refuses cleanly.
- F-help: every flag has `help=`.

Cite file:line. Verdict: accept / accept_with_findings / needs_revision.
