# Decision And RFC Supersession

You own only the write scope in the work packet. Do not edit outside it.

Read the RFC 0078 cutover ledgers, `docs/DECISION_LOG.md`, `docs/rfcs/README.md`,
RFC 0068, RFC 0070, and RFC 0078. Identify decisions and RFCs whose live
Python-era operational rule is fully replaced by the Go-only runtime cutover.

Apply only current-state supersession edits:

- Mark fully replaced decisions or RFC statuses as `superseded`.
- Add successor links or consequences text naming RFC 0078 where partial
  supersession is clearer than changing status.
- Keep historical provenance in place.
- Update RFC index status entries when an RFC status changes.
- Do not rewrite unrelated decision history.

Publish
`docs/operator/artifacts/rfc-0078-docs-guardrails-final-deletion/supersession/HANDOFF.md`
with:

- Decisions/RFCs changed.
- Successor links added.
- Historical artifacts intentionally left unchanged.
- Validation commands run.
- Any blocker that prevents supersession.
