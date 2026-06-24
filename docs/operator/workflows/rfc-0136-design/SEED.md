# Design-Run Seed — RFC 0136 (FRESH v1)

> Fresh v1 `falsification_gate` design run for RFC 0136 (range-partition
> `events` and `audit_log` by `created_at`; partition DROP as the retention
> path). **Required context docs** (read in full first):
> - `docs/rfcs/0136-range-partition-events-audit-log-by-time.md` — the RFC (the PK/unique-key reshape, owner-DDL discipline, #386 subsumption, source anchors).

## ⚠️ Dependency: this design is gated on RFC 0142 P5

RFC 0136 Wave-2 #9 is **blocked-by 0142 P5** (the rehearsal receipt +
expand/contract on an ephemeral two-role clone — P5 design done D258, **not yet
built**). The maintainer authorized **designing this in parallel** with P5's
build. The SPEC must be **honest** about the boundary: specify what P0 can pin
now (the reshape shape, the owner-bundle plumbing, the retention contract) and
what genuinely cannot be rehearsed/built until P5's harness exists. Do NOT
over-claim build-readiness ahead of the blocker.

## Charter

The deliverable (committed `PROPOSAL.md`) is the falsifiable spec the
`rfc-0136-build` `code_change` run executes **once P5 lands**. The single
load-bearing claim is: **declarative range partitioning forces a PK/unique-key
reshape on these two owner-held append-only chained tables**, and that reshape
must preserve every existing integrity guarantee.

## The hard core to PROVE

1. **Reshape correctness.** `events` PK `(repository_id, event_id)` and
   `audit_log` PK `(audit_id)` + `row_hash UNIQUE` must gain `created_at`
   (Postgres requires the partition key in every unique constraint) **without**
   breaking the six `events` FKs, the `audit_log` `segment_id` FK,
   `audit_segments`/`audit_chain_head`, or the **DEFERRABLE INITIALLY DEFERRED**
   FK `repo_event_chain_heads (repository_id, last_event_id) -> events(...)`
   (0006:72) across partitions.
2. **Chain + append-only integrity across partitions.** The in-DB hash chain
   (`append_event_row`/`event_v3_row_hash`; `append_audit_row`/`audit_v3_row_hash`)
   and the append-only triggers (`events_no_update`/`_no_delete`;
   `audit_log_no_update`/`_no_delete`) stay correct when consecutive rows land in
   different partitions.
3. **Owner-DDL safety.** The reshape is **owner DDL** (owner bundle **0020**,
   re-fetch next-free per D236/D239), never a runtime migration (the
   `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` guard; the D187/#244 two-role
   crash-loop). Executable on ~13.5M-row `events` without an unbounded lock
   outage (likely needs a P5-style expand/contract).
4. **Retention subsumes #386.** Partition DETACH/DROP as the retention path makes
   the delete-time RI seq-scan #386 (`0015`) covers **not happen at all** — while
   respecting the chain (you cannot DROP a chained segment without breaking
   verifiability; specify how retention interacts with the hash chain).

## Falsifier guidance

- **Falsifier 1 (reshape / chain-integrity):** break an FK across partitions, the
  cross-partition hash-chain advance, an append-only trigger, or row identity.
- **Falsifier 2 (owner-DDL / retention / P5-dependency):** show the reshape is
  unsafe as owner DDL / un-executable on 13.5M rows without P5, the retention
  DROP breaks the chain, or the SPEC over-claims readiness ahead of P5.

The adjudicator gates clearing on all four held + an honest P5-dependency
boundary. Single v1 revision cycle; a second `needs_revision` routes to the
operator (fresh `-v2`). Keep the local-first boundary.
