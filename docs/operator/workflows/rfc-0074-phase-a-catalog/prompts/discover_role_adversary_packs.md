# Discover Role And Adversary Packs

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`.

Produce `docs/operator/artifacts/rfc-0074-phase-a-catalog/discovery/PACK_DISCOVERY.md`.
Use valid `striatum.synthesis.v1` front matter.

Read:

- `docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md`
- `docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md`
- `docs/operator/artifacts/active-runway-1-5/phase4/SYNTHESIS.md`
- `docs/operator/artifacts/rfc-0076-audit-remediation/catalog-followup/PLAN.md`
- `src/striatum/workflow_templates/catalog.json`

The artifact must include:

- the Phase A graph-shape metadata entries to add first;
- role-pack and adversary-pack entries, including required fields;
- how `code_doc_audit`, `authority_docs_operator_audit`, and
  `code_doc_audit_postures` fit without crowding out the broader RFC 0074
  examples;
- name overlaps, especially `operator_ergonomics`;
- explicit Phase B deferrals for generation, chooser pack selection, and cost
  estimation.

Do not edit source, docs, examples, tests, or catalog files in this job.
