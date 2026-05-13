# Synthesis Prompt: RFC 0043 V1

Produce `docs/dogfood/048/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/048/design/codex/DESIGN.md", "docs/dogfood/048/design/claude_code/DESIGN.md", "docs/dogfood/048/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `designer-unknown-model-<NN>`.

Reconcile the 3 designs into ONE concrete plan with two implementer tracks. Choose; do not enumerate.

**Track A — Schema + migrate-repo-local (codex):**

- Exact SQL migration filename under `src/striatum/daemon_pg/sql/`, ordering with existing migrations. Lock the 15 tables with their `repository_id UUID NOT NULL` column and the chosen `(repository_id, ...)` index strategy. Decide: single shared schema (`striatum_repos.*`) vs per-repo schemas. RFC 0043 Open Q recommends single schema with `repository_id` — confirm or override with one-sentence justification.
- `migrate-repo-local` command body — exact module under `src/striatum/daemon_pg/cutover.py` (or new module if named) + exact CLI dispatcher under `src/striatum/cli/daemon.py`. Flag semantics locked: `--dry-run`, `--keep-sqlite-readonly` (default true), `--confirm-delete` (required for destructive).
- Audit-chain re-anchor algorithm — exact function. Byte-equivalence check.
- Test fixture + test file paths.

**Track B — CLI surface + RPC registry (claude):**

- Exact line in `src/striatum/cli/parser.py` where `--no-daemon` is removed. Exact error envelope shape for the unknown-option path.
- Exit code 11 + 12: exact dispatcher function, exact stderr template, exact platform-remediation strings. Cite RFC 0043 §3.
- RFC 0030 method-registry expansion: exact registry file, exact list of method names locked per the RFC 0043 §5 table, capability mapping per method. Decide on naming for read-capability methods (e.g. `status.summary`, `run.summary`).
- Backward-compat plumbing: existing daemon_mode=on test fixtures continue to pass.

Lock all file paths, function names, error envelopes, and test file paths. If the three designs disagree, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
