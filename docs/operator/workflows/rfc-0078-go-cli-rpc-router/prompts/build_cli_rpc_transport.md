# Build CLI RPC Transport

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement the Go CLI transport used by generated daemon routes.

Requirements:

- use the existing Go daemon RPC envelope and Unix-socket/runtime-token
  conventions where possible;
- keep errors stable for daemon unreachable, repo not registered, capability
  refusal, version skew, and method errors;
- do not read or write PostgreSQL directly from the CLI;
- do not reopen Python or SQLite fallback paths;
- add focused tests for request construction, token/socket resolution seams,
  and response/error mapping;
- record validation commands and unresolved transport risks in the handoff.
