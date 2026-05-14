# Review Design Prompt: RFC 0048 V1.5 fix-up synthesis

Produce `docs/dogfood/058/review/design/REVIEW.md`. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/058/DESIGN_SYNTHESIS.md"]
review_posture: "ergonomics_dx"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

Fresh-session: read ONLY the synthesis + the cited code files + the dogfood-057 reviews that are the inputs. Do NOT read the three design inputs.

## Mandatory checks (bounce on any failure)

1. **All 6 V1 findings explicitly addressed** — codex F1 (fail-closed), F2 (cap-denial tests), F3 (chain-lock), F4 (append-only grants), claude HIGH#1 (parity rig), claude HIGH#2 (dead code). For each: synthesis points to a concrete file+function+test.
2. **Accept-loop design is concrete** — exact function name, concurrency model (asyncio / threading / select picked, not enumerated), bytes flow CLI → socket → router → handler → response, test path under `tests/daemon_rpc/`.
3. **Schema migration 0006 is byte-equivalent for existing rows** — re-anchor algorithm specified, idempotent guard cited.
4. **Parity rig has a real diff helper** — function name, output shape (per-key diff), failure prints actual vs expected.
5. **Capability-denial enumerates all 6 cases per handler** — missing/revoked/expired/wrong-cap/wrong-repo/replay.
6. **Track boundaries don't conflict** — Track A doesn't touch `recovery_evidence/`; Track B doesn't touch `daemon.py` / `daemon_rpc/server.py` / `daemon_rpc/registry.py`. Integration order locked.
7. **No 'TODO' or 'see V1.6' on a non-negotiable** — every V1 finding has a V1.5 destination.

## Ergonomics_dx checks (degrade verdict, don't bounce)

- `daemon doctor --explain` output is operator-actionable.
- `POSTGRES_TRANSITION.md` runbook is copy-pasteable.
- Parity rig diff is readable (per-key, not raw dict-vs-dict).
- Dead-code decisions justify per-symbol (no bulk "delete all").

## Output

Per-bullet evidence citing synthesis line numbers and code references.

Verdict:
- `accept` — every mandatory check passes; no ergonomics_dx degradation.
- `accept_with_findings` — every mandatory check passes; ergonomics_dx findings recorded as required follow-ups.
- `needs_revision` — any mandatory check fails.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.
