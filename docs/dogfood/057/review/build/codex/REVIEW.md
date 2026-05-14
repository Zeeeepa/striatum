---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat-model", "postgres", "daemon-rpc", "capability", "audit-chain"]
---

# Build Review: RFC 0048 Threat Model

author: reviewer-unknown-model-002
date: 2026-05-14
status: needs_revision
target: RFC 0048 daemon-side substrate migration

## Verdict

Needs revision. RFC 0048 correctly identifies the substrate-facade problem and names the right end state, but its per-phase acceptance criteria are too weak for the threat model in the work packet. In particular, Phase A allows SQLite delegation to remain a production fallback while PG-backed handlers are shipped method by method, and neither Phase A nor Phase B requires adversarial coverage for capability denial, malformed envelopes, replay, or fallback suppression.

## Trust Boundaries And Attack Surfaces

- CLI to daemon RPC: untrusted client envelopes, request ids, method names, params, repository scope, and capability tokens cross into the daemon.
- Daemon RPC router to handler registry: route selection decides whether a request reaches a PG-backed handler or historical SQLite-backed delegation.
- Handler to Postgres: every mutation must enforce repository scoping, capability scope, transaction isolation, append-only artifact/event semantics, and audit append.
- Audit chain append: concurrent inserts must preserve a single unbroken hash chain.
- Migration/sentinel boundary: tombstoned or migrated SQLite state must never become a live write target again.
- Python daemon and Go daemon parity: both cores must enforce identical authorization, audit, and storage semantics.

## Findings

### F1 - Phase A Preserves The Fallback Path It Is Supposed To Eliminate

RFC 0048 says each method is ported independently and the router swaps in PG-backed handlers as they land, but it also says "SQLite delegation stays as a fallback for un-ported methods during the transition" (`docs/rfcs/0048-daemon-side-substrate-migration.md:72`). That directly conflicts with the post-D094 invariant that the daemon-required CLI has "no SQLite fallback" (`docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:579`) and with the threat requirement that no SQLite fallback silently engages when a PG handler raises.

The attack surface is not only unported methods. A partial registry swap creates a failure-mode question for every newly ported method: if the PG handler throws after capability validation, after a partial transaction, or during audit append, the router must fail closed. RFC 0048 does not state that handler exceptions are terminal refusals, that fallback dispatch is impossible for a method once marked PG-backed, or that tests inject PG handler failures and assert no SQLite read/write occurs.

Required mitigation: add a per-method acceptance rule that once a method is registered as PG-backed, all exceptions and denial paths fail closed without invoking `striatum.api.invoke`, `striatum.db.connect`, or any SQLite-backed dispatch. Add an explicit negative test per migrated method that monkeypatches the PG handler to raise and asserts no fallback path is called.

### F2 - Acceptance Tests Check Equivalence, But Not Authorization Or Replay Adversaries

RFC 0048 Phase A acceptance requires same pytest suite, byte-identical reads, and matching audit hashes (`docs/rfcs/0048-daemon-side-substrate-migration.md:112`). That proves parity with the old SQLite behavior, but the new trust boundary is capability-gated daemon RPC. RFC 0030 requires every method to declare capability requirements and audit denied calls (`docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md:173`, `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md:206`), while RFC 0043 expands the method registry across mutation and recovery verbs (`docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:261`).

The RFC does not require tests for missing, revoked, expired, wrong-scope, or wrong-capability tokens before each PG write. It also does not require malformed-envelope or duplicate-request/replay tests for migrated methods. A handler can be byte-equivalent under valid input while still accepting a forged or replayed write, especially if the router performs capability checks inconsistently while methods are ported one by one.

Required mitigation: make adversarial RPC tests part of Phase A acceptance for every write/review/claim/recovery/admin method: malformed envelope, unknown method, duplicate `request_id`, missing token, wrong capability, wrong repository scope, revoked/expired token, and replay after a successful write. Each case must assert no workflow table mutation, no artifact/event append, and a denied audit row with the documented reason where the envelope is parseable.

### F3 - Audit Chain Concurrency Is Inherited By Reference, Not Required For New Handlers

RFC 0033 requires audit append to use a serializable transaction with `previous_hash` read inside the same transaction (`docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md:270`). RFC 0048 acceptance only says "audit chain hashes match" for equivalent workflows (`docs/rfcs/0048-daemon-side-substrate-migration.md:114`). That does not cover concurrent inserts from multiple PG-backed handlers, nor does it require row-level locking or serializable retry semantics in the new handler layer.

The attack surface is concurrent valid writes: two claims, heartbeats, artifact publishes, or recovery actions can race on the audit head. If handlers append audit rows outside a transaction shared with authorization and state mutation, or if serialization failures are not retried safely, the chain can fork, skip, or record a decision for a mutation that did not commit.

Required mitigation: require each PG write handler to append audit and workflow events in a short `SERIALIZABLE` transaction or an explicitly documented row-locking protocol. Add a concurrent test that drives overlapping allowed and denied requests for at least claim, publish-artifact, verdict, complete, and recovery mutation paths, then verifies a single contiguous audit chain and no orphan workflow mutations.

### F4 - Append-Only Grants Are Not Carried Into RFC 0048 Handler Acceptance

RFC 0033 explicitly uses Postgres roles to allow INSERT but not UPDATE/DELETE on audit tables (`docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md:261`), and the current SPEC extends append-only expectations to events and artifact records. RFC 0048 names handler ports, but does not require privilege tests that the daemon read-write role cannot update or delete `events` or `artifacts`, nor that new handlers avoid upsert/update patterns on append-only records.

The attack surface is a newly ported PG handler that preserves old SQLite behavior using convenience UPDATE/DELETE or `ON CONFLICT DO UPDATE` on append-only rows. Byte-equivalent state after a happy-path fixture will not catch a handler that can rewrite provenance under adversarial inputs or retries.

Required mitigation: add Phase A acceptance requiring role-level UPDATE/DELETE denial tests for `events`, `artifacts`, and audit tables under the daemon runtime role, plus handler-level tests that retries or duplicate publishes append/refuse as designed rather than rewriting prior provenance.

## Acceptance Criteria To Add Before Proceeding

- For each migrated method, prove PG-handler failure never delegates to SQLite.
- For each mutating method, prove capability authorization happens before any PG write.
- For each mutating method, test malformed envelopes, capability denials, wrong repository scope, duplicate request ids, and replay.
- For concurrent writes, verify a single contiguous audit chain under serializable transactions or documented row locks.
- For append-only tables, verify daemon runtime credentials cannot UPDATE or DELETE and handlers do not use update-style provenance rewrites.

