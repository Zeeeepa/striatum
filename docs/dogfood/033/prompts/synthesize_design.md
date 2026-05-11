# Synthesize Design Prompt

Produce `docs/dogfood/033/DESIGN_SYNTHESIS.md`. The file must start with a `striatum.synthesis.v1` front matter block (JSON-encoded values; quote strings; JSON arrays for lists). Example shape (verbatim, with your concrete inputs):

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/033/design/codex/DESIGN.md", "docs/dogfood/033/design/claude_code/DESIGN.md", "docs/dogfood/033/design/gemini/DESIGN.md"]
---
```

The byline appears AFTER the front matter block, as a plain Markdown line `author: <slug>` (lowercase, no bold/italics/heading/quotes).

Read all three design artifacts and synthesize one implementation plan for the RFC 0033 substrate rewrite. The synthesis must explicitly choose, not just enumerate.

Required sections:

- **Accepted Implementation Scope**: each RFC 0033 §Acceptance Criteria bullet mapped 1:1 to a concrete code-and-test plan, with one named owner per bullet (which `src/striatum/` module, which test file).
- **Deferred Scope**: bundled / Dockerized distribution, Python → Go substrate port, repo-local SQLite changes, signing-key migration (lives in RFC 0031), cross-repo coordinator state (lives in RFC 0032). Each line says why deferred and where it lands.
- **Schema Decision**: concrete Postgres schema layout for registry, capability tokens, audit rows, audit segment manifests, scheduler cursors, daemon metadata. Pick one shape; do not enumerate options.
- **Migration Plan**: forward-only migration order, daemon-startup behavior on schema mismatch, schema version recorded in every audit row.
- **Audit Chain Mapping**: row trigger logic, role-enforced append-only insert privileges, `daemon doctor` verification against retained rows.
- **V1 SQLite → V2 Postgres Cutover**: operator UX, dry-run behavior, byte-equivalent audit chain validation, post-cutover V1 read refusal.
- **Concurrency**: serializable isolation usage, `SELECT ... FOR UPDATE SKIP LOCKED` patterns, deadlock avoidance.
- **Daemon Doctor Onboarding Story**: platform-specific install hints, role-creation snippets, minimum PG version.
- **Test Harness**: per-test isolation shape, teardown guarantees, integration test for V1 → V2 cutover.
- **Documentation Deltas**: SPEC.md / MCP.md / UBIQUITOUS_LANGUAGE.md / CLI_REFERENCE.md / HOW_TO_HUMAN.md / RFC 0033 status block.
- **Test Matrix**: adversarial cases for malformed connection URL, role privileges, schema drift, two daemons against one DB, PG version drift.
- **Staging Plan**: what lands in this dogfood vs deferred to a future dogfood. Avoid overclaim of RFC 0030/0031 guarantees that depend on later RFCs.
- **Human-Decision Questions**: any open questions the implementer cannot resolve from the synthesis alone; map back to RFC 0033 §Open Questions where applicable.

If the three designs disagree, pick one path and explain the tradeoff. If a guarantee is advisory, label it advisory. Do not silently expand scope; the operator decision is to keep V2 substrate scope tight so RFC 0030 can ship.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block AFTER the front matter and a blank line.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
