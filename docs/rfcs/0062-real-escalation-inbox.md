# RFC 0062: Real Escalation Inbox

## Status
Partially implemented; D130 closes artifact-only creation as link-only

## Summary
Escalations now have daemon-backed projection routes and an operator inbox:
`escalation.list`, `escalation.show`, `escalation.resolve`, `striatum inbox`,
and the `striatum.escalation.v1` artifact front-matter schema have landed.
Artifact linkage into the inbox projection is implemented for the shipped
schema.

## Motivation
RFC 0053 made the human principal an escalation-only role, but the product
needed a real projection rather than scattered blocker prose and artifact
conventions.

## Proposed Implementation
Completed work covers list/show/resolve daemon methods, the CLI inbox
projection, escalation artifact validation, and artifact linkage. D130 closes
the artifact-only escalation creation question as link-only: publishing an
escalation artifact may link to an existing escalation-class blocker, but it
does not synthesize blocker rows or escalation inbox rows. Remaining work is
schema hardening: whether blocker payload shape should be tightened further or
moved into a dedicated typed escalation table.
