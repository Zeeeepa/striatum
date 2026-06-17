# Task: Support ledger for the performance-analysis draft

Produce the support ledger at the declared artifact path
(`docs/operator/artifacts/daemon-perf-analysis/support/SUPPORT_LEDGER.md`).
Stay inside your declared `write_scope`. It must validate against the
`support_ledger` V1 front-matter schema (the publisher refuses invalid front
matter with exit code 6).

The ledger maps every load-bearing claim in `DRAFT.md` to its evidence, making
the analysis auditable. For each claim record:

- **Claim** — the exact assertion from DRAFT.md (quote it).
- **Evidence** — the concrete, reproducible exhibit backing it. Admissible
  evidence is one of: a `striatumd.events` / `audit_log` row-range (cite
  `event_id` lo..hi, the partition key, and the row count summarized); a named
  captured artifact from the instrumentation plan (e.g. a `pg_stat_statements`
  top-N table, a captured `pg_locks` wait graph, a pool-saturation timeline); or
  a specific source location (`file:line` under `go/`). Wall-clock anecdotes are
  NOT admissible.
- **Boundary marked** — for any timestamp-derived latency, which physical
  boundary each timestamp marks (state-transition vs lock-acquire vs commit), so
  inter-event deltas are not silently mis-read as lock-hold time.
- **Strength / gap** — `direct`, `indirect`, or `blind-spot` (claim asserted but
  currently unmeasurable; cross-reference the DRAFT blind-spot ledger entry and
  the smallest additive column that would close it).

Flag any claim in DRAFT.md that has no admissible evidence as an explicit gap —
do not manufacture support. The ledger's job is to make the draft's confidence
honest, including where it must be downgraded.
