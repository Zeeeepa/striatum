# Triage TODO 61

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Produce the TODO 61 cleanup triage artifact only. Identify implementation work
that remains after the current Go production daemon and native MCP changes,
with emphasis on:

- legacy SQLite compatibility modules or fixtures that still affect production
  assumptions,
- stale Python daemon or registry assumptions,
- migration/in-memory fixtures that should stay test-only or be deleted,
- product decisions about PostgreSQL-native operator composites that must stay
  blocked rather than guessed.

Do not modify source files. Cite concrete files, tests, and commands the
implementer should use.
