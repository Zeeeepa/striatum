# Web And Browser Test Migration

Read the coverage ledger, RFC 0078, Python web/service pytest files, Go read
and route code, web templates/static handling, and frontend tests under
`src/striatum/web/frontend/src/__tests__/`.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/web/WEB_TESTS.md`

Use this title block exactly:

```text
# Web And Browser Test Migration
author: operator [self-declared: web-tests-codex-gpt-5-001]
```

Port local web coverage to Go route/handler tests and browser or frontend
component tests where browser behavior matters. Keep the service loopback
local. Do not add hosted services, telemetry, cloud APIs, or transcript
capture.

The artifact must list:

- route/UI behaviors covered;
- pytest rows replaced, retired, or blocked;
- Go/frontend files added or changed;
- validation command evidence, including frontend test commands when used;
- remaining route/browser blockers before deleting Python web tests.
