Publish the final **build spec** for RFC 0136 as `DECISION.md`. This artifact is
the binding input to the *implementation workflow* that follows — it must be
concrete enough to build from without re-litigating the design.

Synthesize the arbitrated winner (and the best grafts from the runners-up, per
the tradeoff ledger + dissent review) into a single decision. The build spec
MUST resolve every open question and specify:

- **Chosen design**: granularity, per-table retention horizon, the exact PK/
  UNIQUE reshape for `events` and `audit_log`, the uniqueness posture, the
  owner-bundle layout (number(s)), and the chain-segment model.
- **Hash-chain preservation plan**: the precise handling of
  `repo_event_chain_heads` RI (moved into `append_event_row`), chain-segment
  sealing, and the invariant that partition DROP is the only retention path.
- **Phased implementation plan (P0–P5)**: each phase as a buildable unit with
  its migration/bundle, the Go changes, and what proves it (tests). Name the
  owner bundle number(s) and confirm the binary-before-bundle deploy ordering.
- **Acceptance criteria**: how the implementation will be verified (hash-chain
  invariance test across the boundary, backfill correctness, partition-DROP
  retention without tripping `*_no_delete`, #386 subsumption).
- **Explicit decisions on each of the five open questions**, each with a one-
  line rationale, and any deferred sub-questions recorded as follow-ups.

Also record: which option won and why, the load-bearing dissent (if any) and how
it was resolved or accepted, and a short note that this spec hands off to the
RFC 0136 implementation workflow. Keep it accurate to the codebase — do not
invent file paths or migration numbers; confirm the next free owner bundle.
