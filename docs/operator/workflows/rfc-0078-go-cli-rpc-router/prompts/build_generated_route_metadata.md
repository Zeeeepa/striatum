# Build Generated Route Metadata

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Implement generated Go route metadata for the CLI RPC router. The source of
truth is `contracts/daemon_methods.json`; do not hand-maintain a second route
table.

Requirements:

- generate or check in Go route metadata under the allowed `go/pkg/cli/*`
  paths only;
- include command, optional subcommand, daemon method, params group,
  capability/scope metadata when useful, and deprecation state;
- add a freshness check that fails when `contracts/daemon_methods.json` drifts
  from generated code;
- keep local commands out of the generated daemon route set unless the contract
  names a daemon method;
- document commands run and any deferred route families in the handoff.

Do not change daemon method semantics.
