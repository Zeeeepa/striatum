# Review V1 Design

Fresh-context review of docs/dogfood/006/DESIGN_SYNTHESIS.md plus the
research handoff and RFC 0012. Verify:
- D020 (no remote serving) preserved by non-loopback refusal at startup.
- D006 (api.invoke is the single dispatch path) preserved; service does
  not import from striatum.db beyond events reads.
- D028 (no transcripts) preserved; the service does not log request
  bodies / response payloads.
- Mutation gating is whitelist or blacklist with a clear rule.
- SSE replay via ?since and Last-Event-ID is handled.
- Test plan covers all RFC 0012 acceptance criteria.

Write docs/dogfood/006/review/design/DESIGN_REVIEW.md as a finding
artifact and submit a verdict.
