# Implement Archive Verification Follow-Up

You are the implementer for the TODO 59 archive and Corpus Contract V2
follow-up.

Read the mapping artifact first. Implement only the smallest mapped slice for
archive-default enforcement, deep verification, and read-only semantic
inspection. Keep changes tightly scoped to the workflow write scope.

Expected behavior:

- Archive verification should align with D126: replay/deep verification is the
  default posture for run archives, with no comparative replay.
- Deep-chain checks must remain local and deterministic. Do not require daemon
  reachability when verifying an existing bundle.
- Semantic inspection must be read-only. It may report archive consistency,
  replay details, and privacy-relevant metadata, but it must not mutate live
  workflow state or write operational scratch.
- Artifact content checks may remain explicitly opt-in through local
  repository paths where current behavior requires that boundary.
- Augmentation references, if touched, must be optional and must not make
  workflow progress depend on an external retrieval consumer.
- Do not add hosted services, telemetry, transcript capture, external
  persistence, or production repo-local SQLite paths.

Add or update focused tests for changed behavior. Run the narrowest useful
validation commands and record the results.

Produce `docs/operator/artifacts/todo-59-corpus-v2-archive/build/HANDOFF.md`
with changed paths, behavior summary, validation commands, and deferred work.
