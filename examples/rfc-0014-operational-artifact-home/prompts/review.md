# Review RFC 0014 Spec Handoff Package

Review the RFC 0014 handoff package:

- `docs/rfcs/0014-operational-artifact-home.md`;
- `docs/process/operational-artifact-home-spec.md`.

Treat the RFC as the proposal/history record and the spec handoff as the
implementation contract under review. Do not file a finding merely because an
explicit choice lives in the spec instead of being duplicated in the RFC, as
long as the RFC clearly points to the spec and the two documents do not
contradict each other.

Review the package against its stated context, especially RFC 0013 and the
multi-agent review loop.

Check:

- whether `docs/operations/` cleanly separates operational run state from model
  review feedback;
- whether RFC 0013 marker precedence and redaction rules survive the proposed
  move as specified in the handoff package;
- whether the spec handoff is specific enough for a later implementation
  prompt without requiring the reviewer to infer choices;
- whether legacy marker compatibility is adequate;
- whether the proposal risks committing private corpus content;
- whether this RFC is a good bounded target for `agent_runner` validation.

Write findings first, ordered by severity. Use tight references to files or RFC
sections where possible. If there are no blocking findings, say so clearly.
Near the top of the artifact, include the exact lowercase `author:` byline from
the work packet expected artifact metadata.

End with:

```text
Verdict: accept | accept_with_findings | needs_revision | reject
```
