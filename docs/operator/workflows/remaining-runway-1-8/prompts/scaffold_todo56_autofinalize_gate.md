# Scaffold TODO 56 Auto-Finalize Dogfood Gate

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on D125: global auto-finalize remains default dry-run. Live
auto-finalize is workflow opt-in until the evidence gate is satisfied.

The scaffold must include:

- the evidence bar: three successful live dogfoods, at least two lane shapes,
  and zero contested audit-chain events;
- how to collect and name dogfood evidence without treating terminal output as
  authority;
- checks that preserve dry-run as the default posture;
- failure/reset criteria for the landed circuit breaker and lane-finalization
  visibility;
- implementation write scopes, tests, and release-gate sequencing;
- explicit non-scope for flipping default-on behavior in this track.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
