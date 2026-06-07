# Apply review findings and finalize (#164 / RFC 0114 / D173)

The review verdict is in. Your job:

1. Read the reviewer's finding at
   `docs/operator/artifacts/rfc-0114-impl-164/review/REVIEW.md`.
2. If the verdict is `accept_with_findings`, apply each finding (or record
   an explicit, justified decline per finding) in the implementation.
3. Re-run the full gate set after any change: `make lint`,
   `make typecheck`, `make test`, the pg-gated guard tests via the
   `STRIATUM_PG_TEST_URL` env var, and the exact CI golangci-lint
   (`0 issues`).
4. Finalize the `CHANGELOG.md` Unreleased entry.

Publish `docs/operator/artifacts/rfc-0114-impl-164/SUMMARY.md` (synthesis,
valid front matter, byline from your work packet) containing:

- the final file list (every file the implementation touches),
- the named gates and their actual observed results (paste output lines),
- per-finding disposition (applied / declined-with-reason),
- the **operator runbook**: exact steps to apply owner bundle 0006 to the
  production database out-of-band (RFC 0079 §5) and to verify the doctor
  posture flip to `partial_projection_gated` afterwards,
- what stays open on #164 after this lands (R2/R3 surfaces,
  `private_read_denial` remains false).

Commit all work in your per-job worktree. Stay inside your declared write
scope.
