# Implement Prompt

Implementation is blocked until `review_design_devils`, `review_design_security`, and `review_design_threat` all have accepting verdicts. Do not start implementation from RFC 0028 alone.

After the gate opens, implement only the accepted V1 slice in `docs/dogfood/031/DESIGN_SYNTHESIS.md` and the resolved design review findings. Stay inside the workflow write scope.

Expected behavior:

- introduce `striatumd` (or equivalent named entry point) as an optional local daemon, while keeping the existing direct CLI mode as a working fallback unless the accepted plan explicitly retires a verb;
- add the registry storage option chosen by the synthesis, with migrations and tests that cover existing `.striatum/state.sqlite3` repositories registering against the daemon without data rewrite;
- expose the V1 client surfaces named in the synthesis (Unix socket default, loopback HTTP, MCP resources/tools, optional event stream) with mutation tools default-denied and capability-gated;
- move the named scope of supervision, recovery, and dashboard concerns into daemon-resident code, replacing per-run `recovery watch` for the surfaced acceptance-criteria bullets only;
- record every mutating client request in an audit log with client id, repository id, command, authorization result, and timestamp; do not let the audit log become a transcript;
- update `docs/SPEC.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DECISION_LOG.md`, `docs/TODO.md`, `docs/MCP.md`, `docs/rfcs/0028-...`, `docs/rfcs/README.md`, `README.md`, and `CHANGELOG.md` only as required by the accepted plan; describe daemon guarantees honestly and explicitly mark advisory behaviors as advisory;
- add adversarial tests for capability default-deny, mutation refusal without capability, daemon restart with pre-existing registry and at least one registered repo-local state store, multi-repo dashboard correctness, registry tamper detection where applicable, and symlink/path-traversal refusal at `striatum repo add`;
- do not claim sealed source-write containment, signing authority, or full lane attestation beyond what RFC 0026 and RFC 0027 actually provide today.

Produce `docs/dogfood/031/BUILD_HANDOFF.md` summarizing changes, tests run, compatibility notes, deferred scope, and any human decisions still required.
