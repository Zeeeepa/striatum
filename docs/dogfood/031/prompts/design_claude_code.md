# Claude Code Design Prompt

Produce `docs/dogfood/031/design/claude_code/DESIGN.md`.

Design an implementation plan for the RFC 0028 V1 acceptance-criteria slice. Focus on the trust and authority boundaries the daemon introduces: daemon vs CLI client vs MCP client vs web client vs supervised agent process, and how each boundary is authenticated, authorized, and audited.

Your plan must cover:

- per-client capability model (read, claim, review, apply, recovery, admin) and how capabilities are minted, scoped to repositories, expired, and revoked locally without a hosted directory;
- MCP authorization defaults (read-only) and the explicit promotion path for mutation tools, replacing today's global `striatum serve --allow-mutations` flag;
- the interplay with RFC 0026 lane attestation: how the daemon attests sessions when it owns the supervised process, and how attestation downgrades when a CLI client connects without a supervised session;
- the interplay with RFC 0027 sealed patch provenance: which sealed-mode operations may move into the daemon later, and which V1 capabilities must avoid implying sealed guarantees the daemon does not yet provide;
- how the daemon refuses to parse stdout/stderr as workflow state, and how supervised agents continue to drive state only through structured commands, artifacts, verdicts, blockers, and receipts;
- how direct CLI mode (no daemon) remains a working fallback during the phased migration in RFC 0028 §8 (steps 1–6), and which step this dogfood implements;
- migration of existing `.striatum/state.sqlite3` runs and the operator UX for `striatum repo add` against a repository that already has live runs;
- concrete touch points in `src/striatum/`: `api.py`, `service.py`, `mcp.py`, `supervisor.py`, `recovery/`, `web/`, `dashboard.py`, `cli/`, `db.py`, `migrations.py`;
- error model: how the daemon reports refusals (capability denied, repository not registered, version skew, mutation disabled) to CLI/MCP/web clients;
- test plan covering capability default-deny, audit log completeness, daemon restart, multi-repo dashboard, MCP read-only resources across repositories, and supervised-agent re-attach;
- staged delivery that lands a single defensible slice, with deferred scope explicitly listed.

Be explicit about what cannot be proved by the daemon: model-token authorship, independent human decision provenance, or adversarial local-root resistance. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
