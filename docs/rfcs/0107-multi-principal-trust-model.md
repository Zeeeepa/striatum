# RFC 0107: Multi-principal trust model — make multi-user explicit (self-hosted, not SaaS)

Status: accepted (D160)
Date: 2026-06-02
author: proposer-claude-opus-4-8-001
Context: RFC 0028 (multi-repository control plane), RFC 0030 (RPC envelope + version skew), RFC 0053 (human-principal + AI-operator roles), RFC 0096 (supervised-lane trust boundary + session-bound capability tokens), RFC 0090 (workspace security/attestation parity); migrations `0001_baseline.sql` (clients, client_capabilities), `0022_session_bound_capability.sql`; `go/pkg/rpc/auth_pg.go`, `go/pkg/admin`.

## Problem

The deployment is **self-hosted on a server, serving multiple repositories
today, with multiple human users plausible in the near future** — and it is
explicitly **not** going to become a hosted SaaS control plane. The trust
substrate is already largely present and load-bearing for that reality:
per-client capability tokens scoped to capabilities and (optionally)
repositories (`client_capabilities`, `0001_baseline.sql`), session-bound tokens
(RFC 0096 V2 / `0022_session_bound_capability.sql`), and a daemon-global
hash-chained audit log.

But the **model** still assumes essentially one operator plus one escalation-only
human principal (RFC 0053). There is no explicit notion of *multiple* human
principals/operators sharing one `striatumd` + PostgreSQL across repos, each with
their own identity, capabilities, and audit attribution. Without that model,
multi-user usage would accrete ad hoc on top of the single-operator assumptions —
exactly the kind of drift this project should avoid.

This RFC makes multi-user a **deliberate, bounded design**, not an emergent
accident — while holding the product boundary that Striatum stays self-hosted and
never becomes a hosted/tenanted cloud service.

## Proposal (design RFC; phased implementation follows)

1. **Define "principal".** A principal is an identity (human or AI operator) that
   holds capability tokens. Specify how principals map to `clients` /
   `client_capabilities`, how a principal's actions attribute in the audit chain
   and in artifact bylines (extending RFC 0026/0090 provenance honesty to "which
   principal"), and how the RFC 0053 human-principal escalation role generalizes
   to several humans.
2. **Capability + scoping model.** Reuse the existing per-repository capability
   scoping (`client_capabilities.repository_id`) and session-binding (RFC 0096) as
   the multi-principal substrate: principal A's token cannot act for principal B's
   session (already enforced by `enforceSessionBinding`), and a principal may be
   granted capabilities on a subset of repositories. Specify capability grant /
   revoke / rotation across principals (`daemon.token.*` already exists).
3. **Audit + isolation guarantees.** State the cross-principal and cross-repo
   isolation invariants explicitly and back them with tests: a principal sees and
   mutates only what its capabilities allow; the audit chain attributes every
   mutation to a principal; no principal can read another repo's live state
   without a grant.
4. **The boundary that keeps it non-SaaS.** Document the line: one self-hosted
   daemon + PostgreSQL the operator owns and runs; no hosted control plane, no
   tenant provisioning service, no external identity provider dependency, no
   telemetry. Loopback/tailnet access (RFC 0085) is the access model; principals
   are local trust grants, not cloud accounts.

## Acceptance

- Capability/audit tests attribute actions to distinct principals and prove
  cross-principal + cross-repo isolation.
- `daemon doctor` surfaces configured principals and their capability/repo scope.
- `docs/reference/spec.md` carries the multi-principal model and the explicit
  non-SaaS boundary; `docs/decisions/decision-log.md` records the decision.

## Non-goals

- **Not SaaS / multi-tenant cloud.** No hosted control plane, tenant
  provisioning, hosted identity, or telemetry — that is an explicit product
  boundary, reaffirmed here.
- Not an external IdP / SSO integration — principals are local capability grants.
- Not required for the reliability foundation — this RFC is sequenced **after**
  RFC 0104/0105 and runs parallel to RFC 0103's W2/W3/W4; it does not block them.

## Relationship to prior RFCs

- **RFC 0096** (lane trust boundary, session-bound tokens) is the per-session
  substrate this RFC builds the per-*principal* model on; this is its natural
  multi-user successor.
- **RFC 0053** (human principal as escalation-only role) generalizes from one
  human to several here.
- **RFC 0028** (multi-repository control plane) already provides per-repo scoping;
  this RFC adds the per-principal dimension over it.
- **RFC 0085** (tailnet-identity UI auth) is the network access model that
  complements principal identity.
