# Unblock Authority Guardrails

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Work only in `tests/architecture/test_authority_guardrails.py`,
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`, and your handoff artifact
directory.

Do the bounded follow-up named by the Track 2 regression review:

- remove or narrow the module-level
  `pytest.skip("legacy sqlite eradicated", allow_module_level=True)`;
- classify any current daemon methods that cause the authority guardrail to
  fail;
- remove stale allowlist entries for the deleted legacy SQLite package;
- update `COMMAND_AUTHORITY_MATRIX.md` only as needed to match the current
  daemon method contract;
- run the authority guardrail test and note the exact result.

Do not decide TODO 55, 56, 59, or 60.
