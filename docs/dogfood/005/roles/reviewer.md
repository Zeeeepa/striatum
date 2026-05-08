# Reviewer Role (Dogfood 005)

Use fresh context. Do not rely on prior conversation memory.

For the **design review** (`review_v1_design`):

- inspect `docs/dogfood/005/DESIGN_SYNTHESIS.md`,
  `docs/dogfood/005/research/CURRENT_ADAPTER.md`, and
  `docs/rfcs/0014-process-adapter-completion-guarantees.md`;
- assess whether the diagnostic envelope schema preserves D028 (no
  child stdout/stderr) and contains only metadata Striatum already
  collects plus output-validation deltas;
- assess whether the blocker-reason vocabulary is complete for the
  four failure modes plus the post-reconcile case;
- assess whether the CLI surface and lane field land cleanly without
  breaking workflows that omit them;
- assess whether the test plan covers every failure mode plus the
  externally-killed reconciliation path;
- write `docs/dogfood/005/review/design/DESIGN_REVIEW.md` with the
  finding-artifact front matter and submit a verdict.

For the **build review** (`review_v1_build`):

- inspect the changed source, tests, docs, fixture (if any), and
  the `BUILD_HANDOFF.md`;
- run `make lint`, `make typecheck`, `make test` if you can;
- assert the issue #1 reproduction shape works (a fixture workflow
  with a stub command that exits 0 without producing the artifact
  reaches the blocked state without operator intervention);
- write `docs/dogfood/005/review/build/BUILD_REVIEW.md` and submit
  a verdict.

Use `accept` or `accept_with_findings` only if a human could
reasonably approve. Use `needs_revision` for issues that must be
fixed before the run can finish.
