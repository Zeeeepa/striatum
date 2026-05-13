---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0043", "v1", "build"]
---

author: reviewer-unknown-model-001

# Threat-Model Review

## Verdict

needs_revision

This review used the document-only context supplied in the work packet:
RFC 0043, RFC 0030, RFC 0033, RFC 0039, and the decision log. I did not
inspect implementation files, tests, handoffs, ledgers, or repository
contents outside that review scope.

## Trust Boundaries And Attack Surfaces

RFC 0043 moves the authoritative workflow boundary from repo-local
SQLite to daemon-owned PostgreSQL. That creates four load-bearing trust
surfaces:

- The PostgreSQL schema must preserve per-repository isolation through
  `repository_id UUID NOT NULL`, repository-scoped indexes, and append-only
  grants on event/artifact history. RFC 0043 names these requirements for
  the repo-local table set and explicitly extends RFC 0033's role model to
  `events` and `artifacts` [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:131].
- The migration command must be an audited, authorized, transactional
  crossing from the retired SQLite trust boundary into the daemon DB. RFC
  0043 requires admin authorization before opening state, serializable
  migration, event-log replay, byte-for-byte anchor verification, and a
  checkpoint marker in one transaction [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:218].
- The daemon RPC method registry becomes the only mutation gate. RFC 0030
  says every RPC method is registry-bound to capabilities and audited
  [docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md:173], and
  RFC 0043 extends that principle to every repo-local mutation
  [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:261].
- The CLI must have no direct SQLite escape path. RFC 0043 removes
  `--no-daemon`, assigns daemon-down exit code 11, and states that no
  SQLite file is created, read, or used as fallback [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:176].

## Finding

**High: RFC 0039 still contradicts RFC 0043's no-fallback Go-daemon requirement.**

RFC 0043 requires RFC 0039 to be revised before acceptance so the Go core
drops SQLite entirely, covers the full method registry, and has no
Python-daemon-mediated fallback for unsupported methods
[docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:304].
It repeats that RFC 0039 revision as an acceptance criterion
[docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:514].

The current RFC 0039 text still preserves a coexistence model: Python and
Go daemons coexist, Python remains the default, operators select the core
by configuration, and Python retirement is deferred
[docs/rfcs/0039-go-daemon-core.md:102]. Its implementation status also
says the landed Go daemon exposes only read-only RPC methods and defers
mutating verbs, supervised processes, and distribution/CI matrix work to
a later phase [docs/rfcs/0039-go-daemon-core.md:389]. The step plan keeps
read-only CLI integration in Step 3 and mutating verbs in Step 4
[docs/rfcs/0039-go-daemon-core.md:419].

That leaves a concrete bypass and drift risk for the RFC 0043 cutover:
an operator or test matrix can satisfy "Go daemon exists" while still
running a Go core that does not own every repo-local mutation. In the new
threat model, partial method coverage is not a feature gap; it is a
mutation-authority split. It reintroduces exactly the unsupported-method
fallback surface RFC 0043 says must not exist.

Required revision: update RFC 0039 so its current proposal and acceptance
criteria match D094/RFC 0043. Either make Go-core Phase 1 explicitly
post-RFC-0043 and require the full repo-local method registry before
selection, or mark the current read-only/mutating phased plan as historical
pre-D094 text that cannot satisfy RFC 0043 acceptance.

## Non-Blocking Notes

The schema invariant surface is acknowledged in the RFC text: all
repo-local workflow tables gain `repository_id UUID NOT NULL`, existing
access patterns are indexed with `repository_id`, and append-only
enforcement on `events` and `artifacts` is expressed through revoked
UPDATE/DELETE grants [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:139].

The migration audit-chain surface is also acknowledged: dry-run reports
row counts, artifact anchors, and event-log head; full migration preserves
ordering, replays the event log, and verifies byte-for-byte anchors before
committing [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:229].
The acceptance criteria call out migration, idempotent rerun, and unified
harness coverage [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:481].

The method-registry surface is directionally covered, but the final build
should include an implementation-derived completeness test, not only the
illustrative table in the RFC. The table says the exact list lives in
implementation [docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:269];
that is acceptable only if CI compares the registered RPC methods against
the actual CLI mutation inventory and fails closed on drift.

The `--no-daemon` retirement is clearly specified: unknown-option parsing,
daemon-down exit code 11, unmigrated-repo exit code 12, and removal of the
direct SQLite module surface are all acceptance criteria
[docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:490].
