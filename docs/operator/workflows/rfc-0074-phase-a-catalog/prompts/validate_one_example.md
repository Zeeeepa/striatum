# Validate One Implementation-Panel Example

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Validate only the first RFC 0074 Phase A example:
`examples/implementation-panel-flow/workflow.json`.

If the example does not exist yet, add only the minimum hand-authored fixture
needed to validate the lightweight implementation-panel shape with current
workflow primitives. Do not add the remaining RFC 0074 examples.

The example must not require RFC 0052-specific artifact kinds, panel methods,
or generator support. It should use existing workflow jobs, ordinary artifacts,
and the scorecard/arbitrator/dissent vocabulary from RFC 0074.

Produce
`docs/operator/artifacts/rfc-0074-phase-a-catalog/example-validation/REPORT.md`
as a `test_report` artifact. Include:

- the workflow validation command and result;
- any focused example tests run;
- files created or checked;
- any deferred example breadth work.
