# Apply: finalize RFC 0128 P0 re-land (issue #575)

The review accepted (possibly with minor findings). Discharge any minor findings
from the review, then confirm the change is complete and green:
- `cd go && go build ./... && go vet ./...` clean.
- `cd go && go test ./pkg/workflowauthoring/... ./cmd/striatum/...` green
  (including `TestSecondaryReposManifestIsNotHonored`).

Do not introduce new scope beyond RFC 0128 P0. Write the `summary` synthesis
artifact: final list of files changed, how the `main.go`/`workflow.go` drift was
resolved, the verbatim build/vet/test results, and explicit confirmation that the
exit-7 cross-repo refusal contract holds. Reference issue #575.
