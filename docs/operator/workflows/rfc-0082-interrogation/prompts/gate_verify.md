# Gate Verify — RFC 0082 interrogation sessions

Closer. Runs after the impl gate. Do not edit product code; record results
precisely so the operator can route a fix.

## Run the aggregate (the intention test is the bar)
```bash
cd go && go generate ./... && git diff --exit-code   # generator round-trip clean
cd go && go build ./... && go vet ./... ; cd ..
cd go && go test ./... ; cd ..
cd go && go test -race ./... ; cd ..
(cd src/striatum/web/frontend && npm test)
make python-trace-guardrail
# Confirm the RFC 0082 Required Tests exist and pass, especially the
# end-to-end intention test (builder answers from PRESERVED context):
cd go && go test ./pkg/mutations ./pkg/reads -run 'Interrogation' -v 2>&1 | tail -40
```

## Artifact
Publish `docs/operator/artifacts/rfc-0082-interrogation/verify/SUMMARY.md`
(`synthesis`) with verbatim pass/fail of each command, an explicit list of which
RFC 0082 Required Tests (1-9) are present and green (call out the e2e intention
test 7 by name + result), and `aggregate_status: green|red`. If red, list each
failing item + shortest repro. Use your packet byline (unattested operator
sessions use `author: operator`).
