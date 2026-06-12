Produce one independent RFC-quality implementation option for issue #248.

Use the problem brief, `TASK.md`, and the required context docs. Do not try to
coordinate with the other proposal roles. Your artifact must include:

- the chosen delivery model;
- whether it is notify-only, long-poll, operator-driver wakeup,
  daemon-side wake/spawn, or a staged hybrid;
- exact authoritative surfaces to change or add;
- capability-token, session-binding, lane-attestation, and lease semantics;
- migration and compatibility story for existing `work.await_packet` clients;
- risks, failure modes, and tests;
- what is out of scope.

Keep the proposal concrete enough that a build workflow could be scaffolded from
it if the panel accepts it.
