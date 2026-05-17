# Striatum Daemon Research Prompt

Status: historical/reference
Date: 2026-05-10
author: coordinator-codex-gpt-5.5-001

Historical note: this prompt captured the pre-D094 daemon decision point.
Current Striatum is daemon-required with daemon-owned PostgreSQL as live
state. Reuse it only after rewriting the current-shape section for a new
question such as packaging, helper runtime, supervision hardening, or hosted
boundary policy.

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

- Striatum's production daemon core is Python.
- CLI, MCP, and local web surfaces are clients of daemon RPC.
- Live state is daemon-owned PostgreSQL under a per-repository
  `repository_id`; `.striatum/` is operational scratch only.
- There is a local HTTP/Unix-socket service that acts as a daemon client.
- Daemon MCP exposes capability-gated tools and read resources.
- Supervised process support is daemon-owned; the Go tree is helper/runtime
  and developer-harness material rather than a peer production daemon core.
- Recovery can run as one-shot, per-run watch loops, or daemon sweeps.

Proposed direction:

Create `striatumd`, a long-running local daemon that can manage multiple
target repositories and multiple active workflows at the same time. CLI, MCP,
web UI, and agent plugins become clients. The daemon may own scheduling,
supervision, recovery, event streaming, authorization, and eventually sealed
provenance apply/signing.

Questions to answer:

1. Is this product direction sound, or does it betray the local-first/simple
   CLI nature that makes Striatum valuable?
2. Given the accepted daemon-required direction, where should CLI/MCP/web
   client boundaries remain narrowest?
3. What should "multi-tenant" mean for a local tool: repository tenants,
   operator tenants, client capability tenants, or something else?
4. How should daemon-owned PostgreSQL be packaged or provisioned without
   weakening the local-first boundary?
5. What helper-runtime responsibilities should remain outside the Python
   daemon core?
6. Which future hosted, multi-tenant, or bundled-distribution boundaries need
   explicit product decisions before implementation?
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
- migration or packaging plan from the current daemon/PostgreSQL design;
- benefits and downsides;
- acceptance criteria for a first RFC slice;
- a list of things that should remain explicitly out of scope.

Be ambitious. If you believe the right answer is a rewrite in Go, say so. If
you believe the daemon is a trap and Striatum should stay CLI-first, say so.
Do not optimize for politeness; optimize for a design that will survive
dogfood.
```
