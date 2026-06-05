# Compile RFC 0112 Tradeoffs

Read the three proposals and scorecards. Publish `TRADEOFF_LEDGER.md` at the
packet's expected artifact path.

The ledger must normalize the evidence by question:

- Field name and JSON shape for explicit consumers.
- Advisory vs hard-gated `required: true`.
- One target vs multiple targets in V1.
- Every terminal path that must call the generalized release hook.
- Exact RFC 0105 fixture cells needed before ACE can graduate.
- Work-packet namespace and target-session projection.

For each row, cite the proposal and scorecard that support or oppose it, record
the risk, and name the implementation consequence. End with the smallest
coherent plan the arbitrator can accept or reject.

Do not add new recommendations that are not traceable to panel evidence. Do not
update RFC status, the decision log, the spec, source code, or VERSION.
