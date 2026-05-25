# Validate Router Gate

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Run the focused validation set for the generated Go CLI RPC router gate and
write `docs/operator/artifacts/rfc-0078-go-cli-rpc-router/validation/VALIDATION.md`.

Include:

- generated-router freshness check;
- Go CLI command tests;
- Go RPC registry/contract tests;
- workflow-authoring tests touched by local-command dispatch;
- `go test ./...` if feasible in the current environment;
- any skipped or failing command with exact output summary and blocker class.

Do not modify implementation files in this job. Validation artifacts may quote
short command excerpts, but summarize long output.
