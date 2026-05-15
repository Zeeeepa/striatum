# Build Review Prompt: RFC 0048 Phase C read-surface PG handlers

Your work packet specifies the review posture (`threat_model`, `ergonomics_dx`, or adversarial `threat_model`). Produce REVIEW.md at the packet's path. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/060/build/HANDOFF.md", "docs/dogfood/060/DESIGN_SYNTHESIS.md"]
review_posture: "<copy from work packet>"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

Fresh-session: read HANDOFF.md + synthesis + the source under `src/striatum/daemon_pg/handlers/reads/` and the tests under `tests/daemon_pg/handlers/reads/`. Do NOT read implementer chat transcripts or other reviewers' findings.

## Cross-posture mandatory checks

1. **Every read method on the synthesis list has a handler file** at the synthesis-locked path.
2. **Every handler has a test file** with parity assertion per synthesis strategy.
3. **Every handler scopes by `ctx.repository_id`** — grep the source to confirm; no SELECT lacks a `WHERE repository_id = $1` (or equivalent).
4. **`DaemonRpcRouter._route` actually picks up the new handlers** — `resolve_pg_handler("<method>")` returns the new handler for each read method. Confirm by running the import path mentally: `import striatum.daemon_pg.handlers` → `from . import reads` → `from . import status, dashboard, ...` → decorators register.
5. **Tests run by default** — no `RFC0048_PARITY` env-gating; no `@pytest.mark.skip` on the parity assertions.

Any "no" → degrade verdict.

## Posture-specific scope

### threat_model (codex)

- Repository scoping is enforced via SQL: no handler uses string interpolation for IDs; all SELECTs use parameterized queries.
- Read handlers are pure reads: no `INSERT`/`UPDATE`/`DELETE` in handler bodies.
- Capability auth happens in the router before the handler runs (verify the test that monkeypatches authorize asserts handler is not reached).
- Cross-repo isolation: a handler called for repo A cannot leak repo B rows via union queries or unscoped joins.
- Sensitive fields (capability tokens, secrets, audit hashes) are not echoed in handler responses unless the legacy function did the same.

### ergonomics_dx (claude)

- `striatum status --json` post-migration returns the same top-level keys as the pre-migration SQLite path (or the diff is documented in HANDOFF).
- `striatum dashboard --once` renders post-migration (output shape unchanged).
- `striatum list runs --json` returns rows in the same sort order with the same columns.
- Error responses cite operator-actionable next commands (`repo_not_registered` → `daemon migrate-repo-local`, etc.).
- Parity tests print per-key diffs on failure (not raw dict comparisons that say "expected={huge dict}, got={huge dict}").
- After v1.52.0 lands, operator can run a migrated repo end-to-end without `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`.

### adversarial (gemini)

- A handler that returns rows from the wrong repository (forgot WHERE filter).
- A handler that silently drops rows when a sub-query fails (caught exception → empty list).
- A status handler that returns stale data because it queries a view that lags writes.
- A list.* handler that paginates inconsistently (different sort order between calls).
- A parity test that passes because both the PG handler AND the legacy SQLite path return empty for the wrong reason (e.g., neither queries the right table).
- A handler that bypasses RFC 0030 capability auth because reads are "just queries" (capability still required per the registry).
- An evidence.export handler that leaks redacted prose into the export payload.
- A handler that crashes on `params={}` instead of returning a defined error envelope.

## Output

Per-finding entries: severity (HIGH/MEDIUM/LOW), evidence (file:line, test path, or reproducer), required follow-up.

Verdict:
- `accept` — no HIGH findings; cross-posture mandatory checks pass.
- `accept_with_findings` — mandatory pass; MEDIUM/LOW recorded as follow-ups.
- `needs_revision` — any HIGH or mandatory failure.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
