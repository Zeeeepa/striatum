# CLI Test Migration

Read the coverage ledger, RFC 0078, `contracts/daemon_methods.json`,
`go/cmd/striatum/`, current Go RPC/registry tests, Python CLI pytest files,
and smoke scripts that invoke `striatum`.

Produce:
`docs/operator/artifacts/rfc-0078-python-test-migration/cli/CLI_TESTS.md`

Use this title block exactly:

```text
# CLI Test Migration
author: operator [self-declared: cli-tests-codex-gpt-5-001]
```

Port CLI coverage to Go command tests, daemon RPC route tests, or shell smoke
only where process-level behavior is the point. Keep mutating live workflow
behavior behind daemon RPC; do not rebuild daemon authority inside the CLI.

The artifact must list:

- command families covered;
- pytest rows replaced, retired, or blocked;
- Go/shell files added or changed;
- validation command evidence;
- remaining CLI parity gaps blocking pytest deletion.
