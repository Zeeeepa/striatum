# Striatum Daemon Research Prompt

Status: reusable
Date: 2026-05-10
author: coordinator-codex-gpt-5.5-001

Use this prompt with outside LLMs to critique or extend the proposed
long-running daemon / multi-repository control-plane direction.

```text
You are reviewing a proposed future architecture for Striatum, a local-first
workflow runner for terminal-based AI coding agents.

Please reason from first principles. Do not assume the current implementation
must remain, except for these product principles:

- local-first by default;
- workflow state advances only through structured commands;
- repository files are durable provenance, not the live message bus;
- marker files, terminal output, and provider hooks are not authoritative
  workflow state;
- no broad transcript capture by default;
- no hosted service, telemetry, or external persistence without an explicit
  product decision;
- provenance claims must be honest about their threat model.

Current shape, for context:

- Striatum is currently Python.
- The CLI is primary.
- Live state is repo-local SQLite under `.striatum/state.sqlite3`.
- There is a local HTTP/Unix-socket service, but it delegates to the CLI/API
  and is not the authoritative scheduler.
- There is an MCP-like stdio wrapper scoped to a single target repo.
- There is supervised process support, but it is operated through CLI verbs.
- Recovery can run as one-shot or per-run watch loops.

Proposed direction:

Create `striatumd`, a long-running local daemon that can manage multiple
target repositories and multiple active workflows at the same time. CLI, MCP,
web UI, and agent plugins become clients. The daemon may own scheduling,
supervision, recovery, event streaming, authorization, and eventually sealed
provenance apply/signing.

Questions to answer:

1. Is this product direction sound, or does it betray the local-first/simple
   CLI nature that makes Striatum valuable?
2. Should the long-term product be "CLI with optional daemon" or "daemon with
   CLI client"?
3. What should "multi-tenant" mean for a local tool: repository tenants,
   operator tenants, client capability tenants, or something else?
4. Should state remain per-repo SQLite, move to a central daemon store, use a
   hybrid registry + per-repo stores, or replace SQLite entirely?
5. If replacing SQLite, what storage substrate would you choose and why?
6. Should the daemon be implemented in Python first, rewritten in Go, built
   in TypeScript/Node, or split across languages?
7. How should MCP tools/resources be exposed without making prompt injection
   or confused-deputy attacks too easy?
8. Should daemon mode be required for sealed patch provenance, or merely a
   better host for it?
9. What are the top five failure modes of a long-running local daemon?
10. What is the smallest V1 slice that proves the daemon is worth building?

Please produce:

- an executive recommendation;
- an architecture sketch;
- storage recommendation;
- language/runtime recommendation;
- security and authorization model;
- migration plan from the current CLI/SQLite design;
- benefits and downsides;
- acceptance criteria for a first RFC slice;
- a list of things that should remain explicitly out of scope.

Be ambitious. If you believe the right answer is a rewrite in Go, say so. If
you believe the daemon is a trap and Striatum should stay CLI-first, say so.
Do not optimize for politeness; optimize for a design that will survive
dogfood.
```
