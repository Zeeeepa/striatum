# Design Prompt: RFC 0044 V1 (Striatum-side corpus export)

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/046/design/<lane>/`).

Design **the Striatum-side of RFC 0044 V1**: the `striatum corpus export --since <ref> --out <path>` CLI verb and supporting module that produces a redacted JSONL bundle Engram's `ingest-striatum` can consume. Read `docs/rfcs/0044-engram-phase-1-implementation-spec.md` §3 ("Striatum Export Bundle") and §8 ("Augmentation-Not-Dependency") first. RFC 0041 supplies operator motivation.

**Scope is Striatum-side ONLY.** Engram-side ingestion, the MCP stdio server, and retrieval tools land separately under `~/git/engram/` and are explicitly out of scope here. Peek at `~/git/engram/migrations/` and `~/git/engram/src/engram/` to cite the `source_kind` enum precedent and the expected per-row shape — do NOT modify Engram.

Cover concretely:

- **CLI verb wiring**: where `corpus export` registers in `src/striatum/cli/parser.py` and dispatches in `src/striatum/cli/dispatch.py`. Cite the existing pattern (e.g. `run summary --json`, `recovery stale-leases`) you mirror. Argument shape, JSON error envelope, exit codes.
- **New module `src/striatum/corpus/`**: package layout (enumerator, redaction, writer, manifest). One module per concern.
- **Corpus enumeration**: how each `sub_kind` (rfc, decision_log_row, operator_report, run_summary, audit_chain_entry, changelog_entry, ubiquitous_language_term, harness_friction_pattern, commit) is sourced. Run summaries MUST go through `striatum run summary --json` (RFC 0044 §3 mandate), not free-text SQLite reads. Cite the existing readers.
- **Redaction policy**: explicit denylist (no `.env`, `.striatum/state.sqlite3` blobs, transcripts, terminal output, raw model output, ambiguous free-text live-state fields). Per-field redaction rules with citations to source paths.
- **JSONL emission**: exact line shape from RFC 0044 §3 (source_kind, external_id, sub_kind, content, provenance, observed_at). `external_id` table from the RFC. Deterministic ordering for stable hashes.
- **Manifest**: `manifest.json` fields per RFC 0044 §3 — striatum_version, repo path, git HEAD, dirty-tree flag, `since` ref, per-file SHA-256, schema version, row counts, generated_at.
- **Augmentation-not-dependency**: NO Engram client import; NO Engram method on the daemon RPC registry. Cite the `rg -n "engram" src/striatum/cli` acceptance check.
- **Tests**: unit per module + one integration test against a real run (recent dogfood) verifying bundle replay-stability hashes.

Cite existing code (function names, file paths). Hand-waving "we add a writer" without a pinpoint citation is grounds for design review to bounce.

Out of scope: Engram ingester, `engram-mcp-stdio`, `engram.search`, `engram.fetch_reference`, `striatum operator memory check`, the `striatum-engram` skill, doc updates beyond build/HANDOFF.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
