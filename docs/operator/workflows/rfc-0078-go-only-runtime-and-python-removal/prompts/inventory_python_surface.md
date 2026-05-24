# Inventory Python Surface

Read RFC 0078 and produce
`docs/operator/artifacts/rfc-0078-go-only-runtime-and-python-removal/inventory/CUTOVER_LEDGER.md`.

Classify tracked active Python traces into `port`, `retire`, `rewrite_doc`,
`historical_provenance`, or `delete_after_gate`. Include source, tests,
packaging, scripts, CI, docs, workflows, skill templates, plugin templates,
and examples. Name exact file groups and the dependency that prevents deletion.

Do not edit product code in this job.
