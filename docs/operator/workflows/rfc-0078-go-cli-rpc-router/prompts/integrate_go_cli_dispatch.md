# Integrate Go CLI Dispatch

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Wire the generated route metadata, CLI RPC transport, parameter adapters, and
local-command boundary into `go/cmd/striatum`.

Requirements:

- keep the `striatum` binary as a thin parser/router;
- prefer generated daemon route lookup over hand-maintained per-command
  dispatch;
- route daemon-backed commands through the Go RPC envelope;
- route explicit local commands through the local boundary;
- preserve existing `workflow validate` behavior from the first RFC 0078 slice;
- add representative command tests for read, mutation, local, unknown command,
  and daemon-error paths;
- record the exact validation command set and any remaining command gaps in the
  handoff.
