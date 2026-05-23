# D125 Auto-Finalize Live Build Evidence

Write `docs/operator/artifacts/d125-auto-finalize-live-build-evidence-2026-05-23/BUILD.md`
with the packet's exact `expected_artifacts[].author_line`.

Task:

- Produce a small `synthesis` artifact that records this as a build-lane
  auto-finalize probe.
- Include valid `striatum.synthesis.v1` front matter.
- Do not publish or complete the job yourself. Leave the active lease in place
  so the operator can run `recovery auto-finalize --dry-run` and then the
  workflow-opted-in live command.
- Do not use terminal output, transcripts, marker files, or pane text as
  authority.
