# Review Design Prompt: RFC 0048 Phase C read-surface synthesis

Produce `docs/dogfood/060/review/design/REVIEW.md`. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/060/DESIGN_SYNTHESIS.md"]
review_posture: "ergonomics_dx"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

Fresh-session: read ONLY the synthesis + the cited code files + RFC 0048. Do NOT read the three design inputs.

## Mandatory checks (bounce on any failure)

1. **All read methods enumerated** with legacy-function citation (exact path:line_range or path:function_name), new handler path, and test path. Any missing → `needs_revision`.
2. **Return-shape parity contract** specified per method (exact top-level JSON keys, not "similar shape to legacy"). Any "TODO" / "see synthesis section X" → `needs_revision`.
3. **repository_id scoping** mechanism specified (WHERE clause discipline vs wrapper). Hand-waving → `needs_revision`.
4. **Single implement track** confirmed. Synthesis proposing dual-track → `needs_revision` (the cycle exhaustion lesson from dogfood-058 is non-negotiable for this dogfood).
5. **Parity test strategy** concrete: per-key diff vs legacy on a known fixture OR shape-only smoke. Either is acceptable; "we'll decide in implementation" is not.
6. **Decorator + signature** mirrors Phase A (`@register_pg_handler("<method>", read_only=True)` + `def handle(ctx, params) -> dict`). Synthesis proposing a new decorator pattern → `needs_revision`.
7. **Registration** locked: `daemon_pg/handlers/__init__.py` adds `from . import reads`; `reads/__init__.py` imports each method file. No alternate registration mechanism.

## Ergonomics_dx checks (degrade verdict, don't bounce)

- Handler error messages cite operator-actionable next commands.
- Parity test failure prints per-key diff (not raw `assert state_a == state_b`).
- `repo_not_registered` / no-rows / malformed-params responses are RPC errors with documented `code` strings, not silent `{}` returns.

## Output

Per-bullet evidence citing synthesis line numbers and code references.

Verdict:
- `accept` — every mandatory check passes; no ergonomics_dx degradation.
- `accept_with_findings` — every mandatory check passes; ergonomics_dx findings recorded as follow-ups.
- `needs_revision` — any mandatory check fails.

**Cycle config note**: `review_design → synth` has `max_iterations: 1` on this dogfood (one revision attempt only). If the first synth attempt + first revision both produce mandatory-check failures, the run goes to operator escalation (no attempt 3).

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
