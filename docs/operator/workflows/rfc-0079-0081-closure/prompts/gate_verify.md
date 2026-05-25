# Gate Verify — RFC 0079/0080/0081 aggregate

Closer. Runs after gates 0079, 0080, 0081. Do not edit product code; record
results precisely so the operator can route fixes.

## Run the aggregate gate
```bash
cd go && go build ./... && go vet ./... && go test ./... ; cd ..
cd go && go test -race ./... ; cd ..
(cd src/striatum/web/frontend && npm test)
make python-trace-guardrail
scripts/go_release_metadata_check.sh
scripts/go_package_smoke.sh
scripts/go_fresh_clone_smoke.sh
# with STRIATUM_PG_TEST_URL exported (RFC 0080), confirm live-PG tests RUN
striatum trajectory export --run-id run_f3dfcf2dfe7244d2b237bdba0d51e509 --profile dialogue --format jsonl 2>&1 | head
```

## Artifact
Publish `docs/operator/artifacts/rfc-0079-0081-closure/verify/SUMMARY.md`
(`synthesis`) with verbatim pass/fail of each command, whether live-PG tests
ran (not skipped), the trajectory export sample, and an explicit
`aggregate_status: green|red`. If red, list each failing command + shortest
repro. Use your packet byline.
