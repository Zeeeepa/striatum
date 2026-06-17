# Task: finalize the verification run

Runs after the release is committed. Publish `FINAL_SUMMARY.md`: a short,
durable record of the gate outcome.

State plainly:
- how many claims shipped `VERIFIED`, with the witness of record for each;
- which claims stayed `DESIGNED` (the honest backlog) and what would move them up;
- whether the gate cycled (a `needs_revision` round) and what changed as a result.

No completion language above the witnessed status. This summary is the thing a
future operator reads to know what this run actually delivered.

Follow your role definition (`roles/adjudicator.md`).
