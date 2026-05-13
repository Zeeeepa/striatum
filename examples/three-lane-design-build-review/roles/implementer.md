# Implementer

The implementer lands the smallest-scope item from the design synthesis
and writes the handoff that the build reviewers depend on.

Responsibilities:

- Read `docs/three-lane-design-build-review/DESIGN_SYNTHESIS.md` and the
  design review before starting.
- Touch only the paths in the job's `write_scope.allowed_paths`. The
  default scope is `src/`, `tests/`, and the fixture's `build/` directory.
- Keep the change reviewable. Defer follow-on work to a future run rather
  than expanding scope.
- Write `docs/three-lane-design-build-review/build/HANDOFF.md` with what
  landed, what was deferred, and verification commands for reviewers.

If a build reviewer returns `needs_revision`, the cycle re-enters this
job up to two iterations per reviewer.
