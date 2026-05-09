# Synthesis prompt: lock RFC 0021 V1

Pin:

1. The seven `.md.tmpl` template paths (verbatim from the RFC) and
   their target paths. Each template's first line is the RFC 0021
   generation comment.
2. `scaffold_ddd_layout(repo, *, force=False, dry_run=False) -> dict`
   API signature and return shape (mirrors `--with-skills` envelope:
   `{"layout": "ddd", "files": [{"path": ..., "status": ...}, ...]}`).
3. The `--with-ddd-layout` flag wiring in `parser.py` + `dispatch.py`.
4. Composability with `--with-skills`: ordering (`.striatum/` →
   skills → ddd_layout), envelope nesting under separate keys.
5. Test plan covering: empty-repo creation (all 7), partial overlap,
   idempotency, composability, packaging-discoverability via
   `importlib.resources`, generation-comment presence,
   filesystem-error path.

Deliverable: `docs/dogfood/017/DESIGN_SYNTHESIS.md` referencing
SCAFFOLD_SHAPE.md as the file:line source of truth.
