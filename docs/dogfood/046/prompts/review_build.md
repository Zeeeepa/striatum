# Build Review Prompt (RFC 0044 V1, 3-way)

Produce REVIEW.md at `docs/dogfood/046/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0044", "v1", "build"]
---

author: reviewer-<lane>-<model>-<ordinal>
```

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

Read the implementation handoff at `docs/dogfood/046/build/HANDOFF.md`.

Per-lane angle:

- **codex (threat_model)**: redaction completeness — no `.env` content, no transcripts, no `.striatum/state.sqlite3` blobs, no raw model output, no terminal output, no ambiguous live-state free text leaked into JSONL. Manifest hashes actually verify; tampered bundles detectable. Augmentation-not-dependency boundary enforced: `rg -n "engram" src/striatum/` clean except for documented allowed matches.
- **claude (ergonomics_dx)**: CLI verb discoverable from `--help`; `--since` and `--out` errors produce a useful JSON envelope; bundle layout obvious for the operator who then runs `engram ingest-striatum --repo ...`; no foot-guns when `--since` is omitted.
- **gemini (adversarial threat_model)**: break-the-bundle. Symlinked `--out` paths; dirty-tree flag honesty; non-ASCII / non-UTF-8 file content; very large RFC files; commit messages containing secrets-shaped tokens; run summary missing fields; re-export determinism under file mtime drift; concurrent export.

Required checks (all lanes):

- **CLI verb works**: a real `striatum corpus export --since <ref> --out <path>` invocation produces a bundle matching RFC 0044 §3 shape.
- **Replay-stability**: re-running export produces identical JSONL content and stable per-file hashes (only `generated_at` allowed to vary, per RFC 0044 acceptance criteria).
- **Augmentation boundary**: no Engram import under `src/striatum/`; regression test pins this; daemon RPC registry has no `memory.*` capability.
- **Redaction enforced**: cite the test asserting `.env`, transcripts, `.striatum/state.sqlite3` blobs absent.
- **Tests pass**: `make test` green; integration test against a real recent dogfood is in-tree.

Cite specific files / lines / test names. "Looks good" is not a review.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
