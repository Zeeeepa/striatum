# Codex Design Prompt

Produce `docs/dogfood/031/design/codex/DESIGN.md`.

Design an implementation plan for the RFC 0028 V1 acceptance-criteria slice: a local `striatumd` daemon that manages multiple registered target repositories, with a read-only global dashboard, daemon-backed read-only `status`/`doctor`/dashboard CLI calls, MCP resources across repositories, capability-gated mutation defaults, resident recovery sweeps, and supervised-process ownership.

Your plan must cover:

- the domain model and vocabulary additions (daemon, repository tenant, operator tenant, client capability, daemon registry, global dashboard);
- daemon process lifecycle (start, stop, reload, crash recovery, version skew between CLI and daemon);
- registry storage choice from RFC 0028 options A/B/C/D with explicit recommendation and rationale, including how existing `.striatum/state.sqlite3` repositories are registered without data rewrite;
- daemon SQLite or alternative schema additions and migration behavior;
- client transports (Unix socket default, loopback HTTP, MCP stdio/socket, optional event stream) and how each maps to capability checks;
- CLI client mode: which verbs become daemon clients first, which stay direct, and how the CLI auto-detects daemon availability;
- MCP surface: which resources and tools land in V1, which are read-only by default, and how capability tokens grant mutation;
- supervisor migration from current `supervisor.py` ownership to daemon-resident supervision, including PTY vs pipe and packet delivery;
- resident recovery scheduler replacing per-run `recovery watch` while preserving D036 safety policy;
- audit log shape (client id, repository id, command, authorization result, timestamp) and where it lives without becoming a transcript;
- compatibility risks for existing examples, dogfood workflows, tests, and direct CLI mode;
- test plan covering daemon restart with pre-existing registry, multi-repo dashboard, read-only MCP, mutation refusal without capability, recovery sweep across multiple runs, and supervised process re-attach after daemon restart;
- staged plan that lands the V1 slice without overclaiming sealed provenance, lane attestation, or apply authority that RFC 0026 and RFC 0027 are still resolving.

Be explicit about what the daemon does not prove: model-token authorship, independent human decision provenance, adversarial local-root resistance, or any guarantee that depends on RFC 0027 sealed-mode authority not yet implemented. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
