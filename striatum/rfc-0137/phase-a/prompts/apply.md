# Apply — finalize RFC 0137 Phase A

The review verdict was **accept** (or an accepted revision). Finalize the Phase A
slice and produce the synthesis artifact.

## Do

1. Re-confirm the slice is green and complete: run `make -C go build` and
   `go test ./pkg/metrics/...` (and `go build ./...`) and confirm they pass. If
   the reviewer recorded any actionable nits within Phase A scope, address them
   now (you have repo-write to `go/pkg/metrics/` and `go/cmd/striatumd/`); do
   not expand scope into Phase B/C/D.
2. Ensure the redaction golden/forbidden-regex test is wired into the normal
   test path (`make check` / `go test`), the daemon still builds, and nothing
   outside the declared write scope changed.

## Deliverable artifact: SUMMARY.md

Write `striatum/rfc-0137/phase-a/artifacts/SUMMARY.md` (required synthesis). It
must record, as durable provenance:
- the final file list and what each file does,
- the Phase A acceptance-criteria → test mapping, with the exact verification
  commands and their pass results pasted in,
- confirmation that the surface stays loopback-only and Phase B/C/D was not
  implemented,
- any follow-ups intentionally deferred to Phase B (e.g. the apoptosis/necrosis
  taxonomy) so the next run's scope is clear.

Publish SUMMARY.md and complete the job.
