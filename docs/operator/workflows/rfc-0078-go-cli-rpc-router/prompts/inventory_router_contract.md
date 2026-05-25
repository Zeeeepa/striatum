# Inventory Router Contract

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Build the router contract map for the RFC 0078 Go CLI RPC router gate. Produce
`docs/operator/artifacts/rfc-0078-go-cli-rpc-router/inventory/ROUTE_CONTRACT.md`.

Cover:

- every `cli_routes[]` entry in `contracts/daemon_methods.json`, grouped by
  command family;
- which routes are pure daemon RPC routes for this gate;
- which commands are local workflow-authoring/bootstrap exceptions and why;
- which commands, if any, must be deferred with a concrete next gate;
- the parameter-builder groups later jobs should own;
- the exact validation commands the implementation and review jobs should run.

Do not edit source code in this job.
