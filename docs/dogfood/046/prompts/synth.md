# Synthesis Prompt: RFC 0044 V1 (Striatum-side corpus export)

Produce `docs/dogfood/046/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/046/design/codex/DESIGN.md", "docs/dogfood/046/design/claude_code/DESIGN.md", "docs/dogfood/046/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 designs into ONE concrete plan for RFC 0044 V1 Striatum-side corpus export:

- **CLI verb wiring**: exact functions in `src/striatum/cli/parser.py` and `src/striatum/cli/dispatch.py` where `corpus export` registers and dispatches. Argument shape (`--since`, `--out`), exit code semantics, JSON error envelope behavior.
- **`src/striatum/corpus/` module layout**: exact file names and their single-responsibility (enumerator, redaction, writer, manifest, bundle). Pick one shape — do not enumerate three.
- **Enumeration sources**: per `sub_kind`, the exact reader function or CLI shellout. Run summaries MUST route through `striatum run summary --json` (RFC 0044 §3). Cite functions.
- **Redaction policy**: one chosen denylist + per-field redaction rules. Explicit handling for `.env` filenames, audit-chain rows that may contain free text, commit messages.
- **JSONL emission shape**: locked to RFC 0044 §3 row shape and `external_id` table. Specify ordering rule (lexicographic by `external_id`?) so re-export produces stable hashes.
- **Manifest**: locked fields per RFC 0044 §3 plus the exact `generated_at` formatting rule.
- **Augmentation-not-dependency**: explicit assertion that no Engram import lands in `src/striatum/`. Name the regression check (e.g. a test that greps the package).
- **Tests**: exact test file paths under `tests/` for unit (enumerator, redaction, writer, manifest) and one integration test against a real run with replay-stability hashes.

Choose; do not enumerate. Output is a SPECIFIC plan ready to implement against. If the three designs disagree, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
