---
schema_version: "striatum.work_plan.v1"
artifact_kind: "work_plan"
plan_id: "plan_deferred-26-platform-tenancy-closure"
scope_kind: "initiative"
scope_ref: "docs/rfcs/0028-long-running-daemon-and-multi-repository-control-plane.md#v1-implementation-notes"
state: "closed"
opened_at: "2026-05-23"
closed_at: "2026-05-23"
closure_summary: "Deferred item 26 is split: systemd/launchd service-manager install is already current product, while Windows daemon support and local multi-operator tenancy remain out of current product and require dedicated RFCs before implementation."
supersedes: "plan_residual-deferred-closure-2026-05-23"
retrieval_priority: "high"
---

# Deferred 26 Platform Tenancy Closure
author: deferred26-platform-tenancy-codex-gpt-5-001

## Objective

Classify the historical RFC 0028 deferred item covering Windows daemon
support, service-manager install, and local multi-operator tenancy against
current Striatum product decisions, docs, source, and tests.

## Scope

Owned writable paths:

- `docs/operator/plans/deferred-26-platform-tenancy-closure.md`
- `docs/operator/workflows/deferred-26-platform-tenancy-closure/`
- `docs/operator/artifacts/deferred-26-platform-tenancy-closure/`

Protected shared status docs are intentionally not edited:

- `docs/TODO.md`
- `docs/ROADMAP.md`
- `docs/operator/BRIEF.md`

## Classification Rule

Close only the part that current product evidence proves. If a surface is
already landed, classify it as current product and cite the source/tests. If a
surface changes trust boundaries, platform support, transport semantics, or
operator identity, classify it as out of current product and name the bounded
RFC required before implementation.

## Result

Service-manager install/start/status is landed for Linux systemd user services
and macOS launchd agents. Windows daemon support and local multi-operator
tenancy remain outside current product. A future implementation should start
from two bounded RFCs:

- Windows local daemon support, including transport, runtime paths,
  process-supervision semantics, service installation, packaging, and CI.
- Local multi-operator tenancy, including OS-user mapping, token/session
  identity, repository ACLs, revocation/expiry, audit semantics, and recovery
  from compromised local clients.
