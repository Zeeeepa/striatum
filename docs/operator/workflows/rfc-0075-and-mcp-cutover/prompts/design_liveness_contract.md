# Design RFC 0075 Liveness Contract

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Turn RFC 0075 into a bounded implementation contract. The artifact must:

- propose the smallest first landable slice for tmux-observable MCP agent
  sessions;
- choose tentative daemon method names for pre-work readiness, heartbeat,
  question, and escalation, or justify a single typed method;
- define the minimal persisted metadata needed for supervisor liveness,
  protocol liveness, and lease liveness without storing transcripts;
- name the deadline defaults and where they should be configurable;
- list status/dashboard/operator-current-brief fields that should surface
  attach commands and stall reasons;
- define test cases for discovery, await, ack, heartbeat, structured
  question/escalation, and terminal-text-only stalls;
- preserve the no-terminal-authority and no-transcript boundaries.

Do not implement the design in this job.
