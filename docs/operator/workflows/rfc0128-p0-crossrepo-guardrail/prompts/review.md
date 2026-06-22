# Review: RFC 0128 P0 cross-repo guardrail re-land (issue #575)

Review the author's source changes + `draft` handoff artifact against RFC 0128 P0
(D196). Re-run the checks yourself — do not trust the author's claims.

Verify:
- `RefuseCrossRepoWriteScope` refuses every `write_scope.allowed_paths` entry that
  resolves outside the registered repo root with the **exit-7** structured error
  (absolute paths, parent-escaping relative paths). Interior `..` that nets back
  inside the repo is NOT falsely refused. Cross-repo reach is refused, never
  silently narrowed.
- Foreign-repo-slug prompt **warning** is present; the `secondary_repos` manifest
  is declined and `TestSecondaryReposManifestIsNotHonored` passes.
- CLI/validate wiring is adapted to **current** `main` (not a stale verbatim copy),
  and the exit-7 contract + ordering hold.
- `cd go && go build ./... && go vet ./...` clean and
  `go test ./pkg/workflowauthoring/... ./cmd/striatum/...` green when YOU run them.

Record a single review finding with verdict `accept`, `accept_with_findings`, or
`needs_revision`. For `needs_revision`, list each concrete defect with file:line so
the author can discharge it in one revision.
