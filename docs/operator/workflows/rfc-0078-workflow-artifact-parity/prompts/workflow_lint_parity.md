# Workflow Lint Parity

Compare Python workflow lint semantics with the Go implementation and RFC 0064.
Focus on lint as the accepted-risk surface: same-model implementer/reviewer
pairing, risky revision cycles, advisory warnings, workflow fingerprints, and
decision-linked overrides.

Implement the smallest coherent Go lint parity slice without making workflow
file metadata into live authority. Durable accepted-risk state must remain
daemon-backed when implemented; otherwise document the exact missing daemon
surface.

Produce
`docs/operator/artifacts/rfc-0078-workflow-artifact-parity/lint/HANDOFF.md`
with exactly:

`author: operator [self-declared: lint-porter-codex-gpt-5-001]`

Include parity behavior, accepted-risk authority notes, tests run, and any
remaining blocker.
