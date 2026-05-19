# Triage -- GH #25 scope

You are the triager for this issue workflow. Produce only the declared
scope artifact for this workflow. Do not implement source changes.

## Read

1. `docs/issues/25/SPEC.md`
2. `src/striatum/cli/` -- specifically the dispatch path for the `repo`
   subcommand. Find the verb that handles `repo list` (without --json)
   vs (with --json), and the pre-flight SQLite check that produces
   the `repo_not_migrated` error. Likely candidates: `cli/dispatch.py`,
   `cli/repo.py` (if it exists), `cli/parser.py` for the verb
   registration.
3. The shared SQLite-presence helper (grep for `repo_not_migrated` or
   `state.sqlite3` checks in the CLI module).
4. The daemon RPC route table -- which RPC method does `repo list
   --json` use? Confirm the read-side method name in
   `contracts/daemon_methods.json` and `go/pkg/reads/`.
5. `docs/POSTGRES_TRANSITION.md` for the SQLite-retirement context.

## Output

Write `docs/issues/25/SCOPE.md` with `striatum.synthesis.v1` front matter
and the exact `author:` line from the work packet. Include:

- the exact files in scope (CLI dispatch / `repo list` handler / shared
  pre-flight helper / tests);
- the exact files out of scope (do NOT touch `adopt` or `repo add`,
  do NOT touch other unrelated `repo_not_migrated` callsites that
  belong on mutation verbs);
- an acceptance checklist with one numbered check per bullet in
  `docs/issues/25/SPEC.md` "Acceptance / Definition of done";
- a concrete plan for the table format (column choices, sort order,
  cwd-highlight marker);
- verification commands (ruff + mypy + a targeted pytest + a manual
  `striatum repo list` round-trip on a daemon-registered repo);
- risks: this is the kind of read-side cleanup that can subtly
  regress operator scripts that parsed stderr; if any test parses
  the `repo_not_migrated` string, list it.
