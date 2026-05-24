# Workflow Authoring Cutover

Read RFC 0078 and compare Python workflow/artifact authoring code with
`go/pkg/workflowauthoring`, `go/pkg/workflowgenerate`, and
`go/pkg/workflowtemplates`. Produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/workflow-authoring/HANDOFF.md`.

Name Go parity gaps for validation, generation, upgrade, template catalog, and
artifact front-matter behavior. Implement only a safe, non-overlapping first
slice if it moves RFC 0078 forward without weakening validation.
