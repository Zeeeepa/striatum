# Audit Docs Decision Drift

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`. Stay within the packet's write scope.

Audit whether current documentation and decision records describe current
source behavior and product decisions honestly:

- `docs/SPEC.md` product-boundary claims;
- `docs/DECISION_LOG.md` accepted, superseded, and obsolete decisions;
- `docs/TODO.md` active, done, blocked, and stale work;
- roadmap, RFC status headers, and `docs/rfcs/README.md`;
- operator docs, examples, workflow guides, and runbooks;
- half-implemented RFCs that need phase status, obsoletion, or follow-up;
- conflicts between docs claims and source behavior.

Produce evidence-backed findings with stable `AUD-###` ids. Each material
finding must include severity, category, status, claim, evidence, impact,
recommended action, and follow-up path. Cite concrete paths, docs claims,
source behavior, tests, or command results. Downgrade unevidenced concerns
to observations or open questions.

Preserve historical fixtures and dogfood records as frozen provenance.
Do not clean them up or treat old transcripts, terminal output, marker
files, or provider hooks as authoritative workflow state.
