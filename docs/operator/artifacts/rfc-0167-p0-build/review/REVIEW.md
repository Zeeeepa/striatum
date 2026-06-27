---
schema_version: striatum.finding.v1
artifact_kind: finding
verdict_intent: needs_revision
severity: high
tags: ["rfc-0167", "p0", "review", "a45", "docs", "needs-revision"]
author: reviewer-reviewer-002
run_id: run_a4c3e73e4f7fca11826ba96b7823f4e3
date: 2026-06-24
---

# Review: RFC 0167 P0 implementation, round 2

Verdict: `needs_revision`.

I reviewed the current draft worktree `.striatum/worktrees/wt_83a58fa4eb40b93a71d9f74e27721c94` at `0dc53ebf`, against the falsification-cleared SPEC in `docs/operator/artifacts/rfc-0167-p0-design-v4/commit/proposal/PROPOSAL.md` and RFC 0167. The required command `cd go && go build ./... && go vet ./...` passed. `go test ./pkg/db -run 'TestOperator' -count=1` also returned ok on this host; with no live `STRIATUM_PG_TEST_URL` available here, that primarily proves the operator pgtest file compiles and skips cleanly.

## Blocking finding

1. **A45 / F-2 is not yet source-truth coherent: authoritative docs and route guardrails still describe `operator bootstrap` as the old read-only local composite even though the current code now calls `operator.bootstrap` and writes the session-token presentation file.** The current source implements the rewire in `go/cmd/striatum/operator_bootstrap.go:440-463`, and `agentloop.ResolveTokenMaterial` consumes `STRIATUM_MCP_TOKEN_FILE` before the runtime token at `go/pkg/agentloop/token.go:18-29`. But the authoritative product spec still says the command is a "CLI-local read composite" and "not a daemon RPC method" (`docs/reference/spec.md:116-126`), while the new RFC 0167 section later says the same command calls `operator.bootstrap` and presents the minted operator token (`docs/reference/spec.md:241-250`). The decision log row for D263 is also stale: it records "Remaining thin follow-up: the `striatum operator bootstrap` CLI local-command rewire..." even though the current source has implemented that rewire (`docs/decisions/decision-log.md:35`). The local command classifier still gives `operator bootstrap` a "creates no new live state" rationale (`go/pkg/cli/localcommands/localcommands.go:17`), and the route freshness test still says it must remain local "unless a product decision adds an RPC method" (`go/pkg/cli/routestest/routes_freshness_test.go:80-83`) even though D263 and `contracts/daemon_methods.json` now add `operator.bootstrap`.

This matters because A45 / F-2 is not only a code path. The SPEC requires honest credential-segregation accounting, and this final review must discharge both binding Section F constraints. With the current docs and guardrail text, an operator reading the source of truth gets mutually exclusive claims about whether cold start mutates live state, whether the CLI calls the daemon operator-bootstrap RPC, and whether the CLI rewire is already done. The build therefore does not fully discharge A45's documentation/control-plane face yet. Revise the cold-start/spec text, local-command rationale/test text, and D263 consequences so they all match the current design: `operator bootstrap` is a custom local CLI entrypoint that calls the daemon `operator.bootstrap` RPC, creates live operator-session/token/handle state, and presents only the session-bound `{admin, read}` token for routine operator work.

## Positive checks

The second revision fixed the prior code-level blockers I could inspect:

- A44 Route 3 is now exercised as a composed `cc ⋈ oh ⋈ runs ⋈ spawn_authorization_grants` path and checks the client-id exception plus FK absence (`go/pkg/db/operator_identity_pg_test.go:216-285`).
- A29 / A7 / A27 and A40-A45 tests now mint through `HandleOperatorBootstrap`, authorize through `PostgresAuthorizer`, call real `run.prepare`, and hit `HandleVerifierAttest` for typed fence refusal (`go/pkg/db/operator_identity_pg_test.go:336-517`).
- `HandleOperatorHeartbeat` now records `operator_handles.last_heartbeat_at = now`, not the future expiry (`go/pkg/mutations/operator_session.go:299-318`).
- Owner bundle 0022 advances the owner frontier to 22 and includes the C2 column grants/projections; the named run/detail and archive star-readers I spot-checked now use explicit non-identity column lists.

Revise the stale source-truth and guardrail text above, then rerun build/vet and the pgtests under a live `STRIATUM_PG_TEST_URL` in the verifier stage.