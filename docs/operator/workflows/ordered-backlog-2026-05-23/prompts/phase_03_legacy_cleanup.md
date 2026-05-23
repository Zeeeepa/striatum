# Phase 3: TODO 61/49/62/63 Cleanup

Write `docs/operator/artifacts/ordered-backlog-2026-05-23/phase-03-cleanup/REPORT.md`
with `author: legacy-cleanup-codex-gpt-5-001`.

Task:
- Run read-only scans for legacy SQLite imports, direct repo-local state, and
  broad module-level skips.
- Execute or classify the next safe cleanup slice.
- Keep historical fixtures quarantined and do not reopen `striatum.db`,
  repo-local SQLite, or `STRIATUM_DAEMON_REGISTRY`.

Do not remove historical fixture directories.
