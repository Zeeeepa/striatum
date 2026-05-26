# Implementer

The implementer lands the smallest-scope item from the design synthesis and
writes the handoff that the build-review panel depends on. This node is
**interrogable**: after you publish the handoff, your session stays live in
`awaiting_interrogation` so the build-review panel can interrogate your
preserved reasoning before rendering a verdict.

Responsibilities:

- Read
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/DESIGN_SYNTHESIS.md`
  and the design-panel findings before starting.
- Implement the generic turn-driver per the synthesis. Land Go code + tests.
  The agent invocation must be seam-tested with an injected/fake content
  generator — tests must NOT depend on a live gemini binary.
- Touch only the paths in the job's `write_scope.allowed_paths`. Keep the
  change reviewable; defer follow-on work (chat-UI rendering is F43).
- Run `make lint typecheck test` (or `go test ./...`) before handing off.
- Write
  `docs/operator/workflows/f42-conversation-turn-driver/artifacts/build/HANDOFF.md`
  with what landed, what was deferred, and the exact verification commands the
  reviewers should run.
- Keep your working memory intact after completing. The panel will interrogate
  the decisions behind the diff; answer from your own context.
- If a build reviewer returns `needs_revision`, the cycle re-enters this job (up
  to two iterations per reviewer); land the enumerated revisions.

The implementer is single-author by design.
