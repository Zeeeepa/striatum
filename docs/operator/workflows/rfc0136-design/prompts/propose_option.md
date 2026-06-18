Propose ONE complete, internally-consistent partitioning design for RFC 0136
that resolves ALL five open questions named in `PROBLEM_BRIEF.md`. Be a distinct
voice — do not hedge across options; commit to a concrete design and let the
panel score it against its rivals.

Read the problem brief and RFC 0136 (attached). Your option must specify:

1. **Granularity** — pick one (e.g. monthly) and justify against write rate +
   retention + partition count.
2. **Retention** — concrete horizon per table (`events` vs `audit_log`), and
   whether `audit_log` partitions are ever dropped.
3. **Key reshape** — the exact new PK/UNIQUE for each table that joins the
   partition key (`events` PK, `audit_log` PK + `row_hash` UNIQUE), and how you
   re-assert (or deliberately drop) uniqueness given the SD-sole-writer invariant.
4. **Hash-chain preservation** — exactly how the event/audit in-DB hash chain
   stays invariant across the boundary, including what happens to
   `repo_event_chain_heads`' FK (moved into `append_event_row` Go RI) and
   `audit_segments` / chain-head sealing before a partition is droppable.
5. **Owner-bundle layout** — owner bundle 0016 for both vs sibling bundles;
   the capability-parity-gated backfill (attach legacy heap as the historical
   partition) and binary-before-bundle deploy ordering.
6. **Chain-segment model** — generalize `audit_segments` into a shared
   abstraction for events, or a parallel `event_chain_segments` table; justify.

Also give a **phased P0–P5 implementation outline** for your design and call out
its single biggest risk. Publish your option as the artifact your packet
declares. State assumptions explicitly; the scorekeeper will grade you on
correctness, deploy-safety, simplicity, retention fit, and risk.
