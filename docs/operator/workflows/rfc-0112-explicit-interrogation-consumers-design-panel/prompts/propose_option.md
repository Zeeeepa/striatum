# Propose An RFC 0112 Implementation Option

You are one of three independent proposal lanes. Follow the specific option
focus in your work packet objective, and do not coordinate with the other
proposal lanes.

Publish your proposal at the packet's expected artifact path. Cover:

- Your chosen workflow field shape, including exact JSON and validation rules.
- The `required: true` V1 behavior: advisory packet signal or hard gate, with
  the reason.
- Whether V1 should allow one target or multiple targets per consumer.
- The complete set of terminal mutation paths that must run the generalized
  release hook.
- The work-packet projection namespace and fields, including available,
  unavailable, and not_ready states.
- Revision-reopen behavior and how stale target sessions retire.
- The smallest RFC 0105 reliability fixture that proves ACE gets genuine
  preserved-context cross-examination after implementation.
- Risks, migration/backward-compatibility notes, and rejected alternatives.

Do not write code. Do not update RFC status, the decision log, the spec, source
docs, or VERSION.
