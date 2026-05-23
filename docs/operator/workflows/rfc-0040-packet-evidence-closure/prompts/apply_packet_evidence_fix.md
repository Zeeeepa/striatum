# Apply Packet Evidence Fix

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Use `RESIDUALS.md` plus current source/tests to decide whether there is a
bounded packet-evidence fix. If there is, keep edits limited to:

- `src/striatum/daemon_pg/handlers/reads/_read_model.py`;
- `tests/daemon_pg/handlers/reads/test_list_read_handlers.py`;
- `tests/test_corpus_redaction.py`.

If source behavior already satisfies the residual, do not edit source. In
either case, write
`docs/operator/artifacts/rfc-0040-packet-evidence-closure/BUILD.md` with the
change/no-op rationale and validation commands.
