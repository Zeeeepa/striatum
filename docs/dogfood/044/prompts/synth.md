# Synthesis Prompt: RFC 0040 V1.5 (F1-F6)

Produce `docs/dogfood/044/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/044/design/codex/DESIGN.md", "docs/dogfood/044/design/claude_code/DESIGN.md", "docs/dogfood/044/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration.

Reconcile the 3 designs into ONE concrete plan for RFC 0040 V1.5 F1-F6:

- **F1 dispatch wiring**: exact daemon entry function under `src/striatum/daemon_pg/` that owns `tools/call`, exact method-registry handle it routes to, exact audit-row semantics (allow-row + dispatch-result-row, or unified post-dispatch row — pick one).
- **F2/F3 atomicity + verdict-recording**: exact transaction or compensation strategy for composite tools in `src/striatum/web/chat_tools.py`. Pick a model (SAVEPOINT, single SQL transaction wrapping the composite, or explicit reverse compensations) and justify in one sentence.
- **F4 watcher invocation**: exact supervisor-lifecycle hook in `src/striatum/daemon_pg/` (or `src/striatum/supervisor.py`) where the watcher task launches, plus the join-on-shutdown path. Name the function.
- **F5 race + signal hardening**: enumerate the guarded race windows and the guards. Specify SIGTERM handling and rotated-log behavior.
- **F6 e2e tests**: exact new files under `tests/`, the smoke harness hook, the composite-rollback case fixture.
- **Backward-compatibility**: explicit assertion that existing MCP tools and daemon RPC envelope-v1 stay unchanged; list the regression test fixtures that pin this.

Choose; do not enumerate. Output is a SPECIFIC plan ready to implement against. If the three designs disagree on a point, pick one and justify in one sentence.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
