# Gate Verify — Aggregate validation

You are the closer for RFC 0078. Runs AFTER Gates E and F. Do not edit product
code; if a gate command fails, record the failure precisely so the operator can
route a fix — do not paper over it.

## Run the aggregate gate

```bash
make python-trace-guardrail            # strict: must report blocked=0 unclassified=0
cd go && go test ./... ; cd ..
(cd src/striatum/web/frontend && npm test)
scripts/go_release_metadata_check.sh
scripts/go_package_smoke.sh
scripts/go_fresh_clone_smoke.sh
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/verify/SUMMARY.md`
(`artifact_kind: synthesis`) with the verbatim pass/fail of each command above,
the final `make python-trace-report` class counts, and an explicit
`aggregate_status: green|red`. If red, list each failing command and the
shortest reproduction. Use your packet byline.
