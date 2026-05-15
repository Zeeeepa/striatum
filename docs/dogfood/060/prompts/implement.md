# Implement Prompt: RFC 0048 Phase C read-surface PG handlers (codex, single track)

Produce `docs/dogfood/060/build/HANDOFF.md`. Front matter:

```
---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/060/DESIGN_SYNTHESIS.md", "docs/dogfood/060/review/design/REVIEW.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `implementer-unknown-model-<NN>`.

## Scope

Port the read-surface methods listed in the synthesis to PG handlers under `src/striatum/daemon_pg/handlers/reads/`. Default list (synthesis may adjust): `status`, `dashboard`, `list.runs`, `list.sessions`, `list.jobs`, `list.artifacts`, `list.workflows`, `run.summary`, `why`, `doctor`, `evidence.export`, `corpus.export`.

For each handler:

- File at the synthesis-locked path (default `src/striatum/daemon_pg/handlers/reads/<method>.py`).
- Decorator: `@register_pg_handler("<method.name>", read_only=True)` from `striatum.daemon_pg.handlers.registry`.
- Signature: `def handle(ctx: RepoHandlerContext, params: dict[str, Any]) -> dict[str, Any]`.
- Queries scoped by `ctx.repository_id` against the striatumd.\* tables locked by synthesis.
- Returns the exact top-level JSON shape locked by synthesis (parity with the legacy function).

## Registration

Wire the new handlers into the existing Phase A registry:

- `src/striatum/daemon_pg/handlers/reads/__init__.py` — imports every method file (`from . import status, dashboard, list_, run_summary, why, doctor, evidence_export, corpus_export` etc.) so the `@register_pg_handler` decorators run on package import.
- `src/striatum/daemon_pg/handlers/__init__.py` — add `from . import reads` so `daemon_rpc.server.py`'s `import striatum.daemon_pg.handlers` line picks up the read handlers too.

## Tests

For each method, one test file at `tests/daemon_pg/handlers/reads/test_<method>.py`:

- Parity assertion per synthesis strategy:
  - **If per-key diff**: seed both PG and SQLite from the same `Seed` fixture; call PG handler; call legacy function against SQLite; assert per-key equality with a diff helper.
  - **If shape-only smoke**: seed PG; call handler; assert returned dict has the locked top-level keys with the expected types.
- Repository scoping: seed two repos in PG (A and B); call handler for repo A; assert no repo-B rows leak into the response.
- Error modes: handler raises `RpcError("repo_not_registered", ...)` on missing `repository_id`; handler returns an empty-but-shaped response on no-rows (not Python exceptions); handler raises `RpcError("schema_invalid", ...)` on malformed `run_id` / `session_id` / `job_id`.
- Capability auth (smoke): monkeypatch `authorize()` to deny; assert handler is never called (this is router responsibility, but the test documents the expectation).

## Sub-agents (use them aggressively)

Cluster the work for parallelism inside this implement job:

- **core-reads sub-agent**: `status`, `why`, `doctor`, `dashboard`.
- **reporting-reads sub-agent**: `list.runs`, `list.sessions`, `list.jobs`, `list.artifacts`, `list.workflows`.
- **summary-reads sub-agent**: `run.summary`, `evidence.export`, `corpus.export`.

Each sub-agent ports its cluster + writes its tests. The implementer integrates and writes the HANDOFF.

## Forbidden writes

- `src/striatum/daemon_pg/handlers/workflow_loop/` (Phase A Track A).
- `src/striatum/daemon_pg/handlers/recovery_evidence/` (Phase A Track B).
- `src/striatum/daemon_pg/sql/` (schema migrations).
- `src/striatum/daemon_rpc/` (router + transport — already in place from V1.5).
- `src/striatum/daemon.py` (accept loop + bootstrap — already in place from V1.5).
- `src/striatum/cli/daemon_rpc_route.py` (CLI Phase C hook — already in place from V1.51).
- `src/striatum/cli/dispatch.py` (the dispatch hook is wired).

## HANDOFF.md content

Per method: handler path + function + test path + parity confirmation (per-key equal OR shape-asserted) + queries + response shape + behavior delta (preferably none).

Top-level summary table cross-referencing the synthesis method list. Note any synthesis methods deferred (with reason).

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
