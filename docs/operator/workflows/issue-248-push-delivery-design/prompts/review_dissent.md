Try to falsify the arbitration before the final decision is recorded.

Attack the design from the hardest angles:

- product-boundary drift into daemon autonomy;
- missing scheduler/principal or capability-token semantics;
- untestable delivery guarantees;
- compatibility breaks for current agent-loop clients;
- unsafe interaction with `fresh_session_required`, leases, session closure,
  `run drive`, or `supervised_push`;
- scope too large for the next build workflow.

Return `accept` only if the design is safe to hand to a build workflow.
Use `needs_revision` if a concrete repair is needed before build.
