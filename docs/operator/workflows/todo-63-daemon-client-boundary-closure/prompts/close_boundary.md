You are closing TODO 63 / RFC 0070 daemon client/service boundary residuals.

Read the context docs named in the workflow, especially RFC 0070, TODO item
63, D110, D112, and the current boundary tests.

Produce only the expected closure artifact unless you find an actual
implementation gap. If source/test edits are needed, keep them narrowly scoped
to the failing boundary and explain the evidence. Do not edit shared TODO,
roadmap, brief, RFC, decision-log, or architecture ledgers.

Required checks:

- confirm daemon-side `repo.resolve` exists as a daemon-global read method;
- confirm CLI/service normal repository resolution does not import
  `striatum.daemon_pg.connection`;
- confirm daemon-mapped `/v1/invoke` reads and mutations call daemon RPC
  rather than `striatum.api.invoke`;
- confirm web chat mapped reads/mutations use the same daemon-routing policy;
- confirm daemon MCP `tools/list` hides local authoring and removed composite
  methods, and `tools/call` refuses hidden/removed production tools;
- confirm `dogfood.publish_on_behalf`, `dogfood.surgical_recovery`, and
  `apply.reviewed_patch` remain absent from the production daemon contract.

If the evidence shows the remaining composite question is already answered by
D110/D112 and current tests, publish
`docs/operator/artifacts/todo-63-daemon-client-boundary-closure/closure/NO_ACTION.md`
as a `striatum.synthesis.v1` artifact with a concise verdict, validation
evidence, and shared-doc updates that should be queued separately.
