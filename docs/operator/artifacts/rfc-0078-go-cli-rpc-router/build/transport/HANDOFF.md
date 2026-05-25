# CLI RPC Transport Handoff
author: operator [self-declared: transport-builder-codex-gpt-5-001]

## Landed

- Added `go/pkg/cli/rpcclient`, a Unix-socket daemon RPC client for CLI routes.
- The client resolves `STRIATUM_DAEMON_SOCKET`, `STRIATUM_DAEMON_TOKEN`, `STRIATUM_DAEMON_TOKEN_FILE`, and the runtime `client-token` convention.
- The client performs the `daemon.hello` handshake before ordinary RPC routes and sends the existing daemon envelope.
- Error mapping preserves daemon-unreachable, version-skew, repo-not-registered, authorization refusal, schema, and method errors as stable CLI exit classes.

## Commands

- `go test ./pkg/cli/rpcclient ./pkg/rpc/...` passed as part of `go test ./pkg/cli/... ./pkg/rpc/...`.

## Risks

Text rendering is still generic JSON for newly routed daemon routes. Rich legacy text formatting remains a later parity slice.
