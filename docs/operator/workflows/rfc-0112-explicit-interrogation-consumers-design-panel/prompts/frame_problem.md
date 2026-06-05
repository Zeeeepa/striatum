# Frame RFC 0112

Read the required context docs, especially
`docs/rfcs/0112-explicit-interrogation-consumers.md`.

Publish `PROBLEM_BRIEF.md` at the packet's expected artifact path. The brief
should make the design panel efficient:

- State the ACE failure precisely: `convener_draft` stays behind
  `convener_synthesis`, and direct-dependent window ownership closes the
  preserved-context session before cross-examiners can consume it.
- List hard constraints from RFC 0082, RFC 0095, RFC 0098, RFC 0105, and
  RFC 0106.
- Explain why fake `convener_draft -> cross_examiner_*` edges are not allowed.
- Define the decision criteria for a good implementation plan.
- Name the six questions the panel must answer:
  `interrogation_targets` field name/shape, `required` semantics, multiple
  targets in V1, terminal paths for the release hook, the RFC 0105 fixture that
  proves ACE can graduate, and the work-packet namespace.

Do not edit RFC status, the decision log, the spec, source code, or VERSION.
