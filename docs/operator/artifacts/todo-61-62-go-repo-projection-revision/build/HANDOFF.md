# Go Repo State-Path Projection Handoff
author: implementer-codex-001

## Summary

Fixed the remaining Go-side repository projection divergence from cleanup F1:

- `repo.list` now serializes rows through the same public repository
  projection used by `repo.resolve`.
- `repo.resolve` and the already-registered `repo.add` path normalize stale
  `state_db_path` values ending in `.striatum/state.sqlite3` to the
  `.striatum/` operational scratch directory.
- The stored PostgreSQL value is not rewritten; normalization happens only in
  the response map.

## Files Changed

- `go/pkg/repositories/service.go`
- `go/pkg/repositories/service_test.go`

## Verification

- `go test ./pkg/repositories` from `go/` -> passed
- `git diff --check -- go/pkg/repositories/service.go go/pkg/repositories/service_test.go` -> passed

## Notes

This change intentionally did not implement Track 2/Track 3 cleanup follow-ups
or decide TODO 55, 56, 59, or 60. The worktree had pre-existing unrelated
edits outside this packet's Go repository/admin scope; this handoff only
describes the bounded projection fix above.
