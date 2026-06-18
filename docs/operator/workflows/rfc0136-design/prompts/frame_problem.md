You are framing a **design decision** for RFC 0136 — range-partition the
`striatumd.events` and `striatumd.audit_log` tables by time. RFC 0136 is a
**proposal** (status: proposed). Turn it into a crisp problem brief a design
panel will use to converge on a **build spec**. Do NOT propose solutions — only
frame the space.

Read first (context docs are attached to your packet):
- `docs/rfcs/0136-range-partition-events-audit-log-by-time.md` (proposal + Open Questions)
- `go/pkg/db/sql/0005_repo_local_workflow_state.sql` (events table + PK + FKs)
- `go/pkg/db/sql/0001_baseline.sql` (audit_log + PK + row_hash UNIQUE + segment_id FK)

Publish `PROBLEM_BRIEF.md` covering:

- **The decision**: how to declaratively range-partition two owner-held,
  append-only, SECURITY-DEFINER-write-only logs by `created_at`/`ts`.
- **Open questions to resolve** (RFC 0136): (1) partition granularity (monthly?
  weekly? quarterly?); (2) retention horizon — possibly different for `events`
  (operational) vs `audit_log` (forensic, maybe never-dropped); (3) re-assert
  narrowed uniqueness vs rely on the SD-function-is-sole-writer invariant; (4)
  one owner bundle 0016 for both tables vs sibling slices; (5) generalize
  `audit_segments` into a shared chain-segment abstraction for events vs mint a
  parallel `event_chain_segments` table.
- **Hard constraints** (non-negotiable): the partition key must join every
  PK/UNIQUE (a key reshape); the in-DB event/audit hash chain must stay
  invariant across the boundary; `repo_event_chain_heads`' DEFERRABLE FK into
  the events PK cannot survive the reshape (the D215 FK-to-owner-table trap) so
  its RI moves into `append_event_row` in Go; owner-held DDL ships as an owner
  bundle (D187/D215) with capability-parity-gated backfill; retention drops a
  partition (DDL) only after its chain segment is sealed — never a row DELETE
  (the `*_no_delete` triggers forbid it).
- **Goals**: defuse the latent seq-scan / VACUUM / retention cliff; make
  partition DROP/DETACH the retention mechanism (no child-RI scan); cleanly
  subsume #386 (the FK indexes shipped as owner bundle 0015 are interim).
- **Non-goals**: do NOT implement code now — this run produces a *design / build
  spec*, not an implementation. Do not change hash-chain inputs.
- **Decision criteria** (how options are scored): correctness of hash-chain
  preservation across the boundary; deploy-ordering safety (binary-before-bundle
  tolerance); operational simplicity for a single-node local-first daemon;
  retention/compliance fit; migration/backfill risk; how cleanly it subsumes #386.

Frame the question so three independent proposers can each propose a complete,
distinct partitioning design that resolves all five open questions.
