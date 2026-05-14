# Synthesis Prompt: RFC 0048 V1.5 fix-up

Produce `docs/dogfood/058/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/058/design/codex/DESIGN.md", "docs/dogfood/058/design/claude_code/DESIGN.md", "docs/dogfood/058/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `designer-unknown-model-<NN>`.

Reconcile the 3 designs into ONE concrete fix-up plan. Pick one path per decision; do NOT enumerate.

## Track A (codex) lock items

For each item, specify exact file + function + signature + test path:

1. Fail-closed routing pattern in `DaemonRpcRouter._route` — registry-lookup function name, error-envelope shape, regression test path.
2. Audit-chain locking strategy (SERIALIZABLE vs explicit row-lock) — per-handler pattern, concurrent-test fixture path.
3. Unix-socket accept loop in `run_daemon_foreground` — asyncio / threading / select pattern; envelope read/write through `daemon_rpc.framing`; handshake path; end-to-end test path under `tests/daemon_rpc/`.
4. Append-only role enforcement — new SQL migration (file name + line range) OR amendment to existing migration; test that asserts privilege.

## Track B (claude) lock items

For each item, specify exact file + function + signature + test path:

1. Parity rig — `parity_seed` fixture path, per-key diff helper function, list of 16 test files wired, env-gate removal line.
2. Capability-denial test matrix — helper-fixture function name, list of 6 cases per handler, per-handler test file additions.
3. Schema migration 0006 — exact SQL DDL, re-anchor function name, idempotent guard.
4. Dead-code decision per symbol (`complete_inline`, `ack_inline`, `recovery.resume --complete`, `recovery.auto`) — keep+wire vs delete; one-sentence justification per symbol.
5. `daemon doctor --explain` flag — exact argparse addition, exact output shape (table headers + rows).
6. `POSTGRES_TRANSITION.md` runbook section — exact heading, exact SQL block, exact doctor-refusal quote.

## Cross-cutting

- Migration & rollout: which Track lands first? Can they run in any order? (Hint: Track B's `daemon doctor --explain` reads from the registry that Track A modifies. Track A doesn't depend on Track B. Implementers can work in parallel; integration order: Track A then Track B.)
- Migration version: 0006 is sequential; confirm no conflict with main's migration ordering.
- Version bump path: this fix-up ships as v1.50.0 per pyproject.toml convention.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
