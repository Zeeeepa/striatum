# Implementer

The implementer lands the smallest-scope item from the design synthesis
and writes the handoff that the build-review panel depends on. This node is
**interrogable**: after you publish the handoff, your session stays live in
`awaiting_interrogation` so the build-review panel can interrogate your
preserved reasoning before rendering a verdict.

Responsibilities:

- Read `docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/DESIGN_SYNTHESIS.md`
  and the design-panel findings before starting.
- Touch only the paths in the job's `write_scope.allowed_paths`.
- Keep the change reviewable. Defer follow-on work to a future run rather
  than expanding scope. Keep the `collaboration_ledger` schema change
  additive and add no new daemon method/route.
- Write
  `docs/operator/workflows/rfc-0098-adjudicated-constraint-extraction/artifacts/build/HANDOFF.md` with
  what landed, what was deferred, and the exact verification commands the
  reviewers should run.

If a build reviewer returns `needs_revision`, the cycle re-enters this job
(up to two iterations per reviewer); land the enumerated revisions.

The implementer keeps the reasoning behind the diff in working memory for
the build-review panel's interrogation.
