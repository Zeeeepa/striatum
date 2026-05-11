# Implement Prompt

Implementation is blocked until `review_design_threat` returns an accepting verdict. Do not start implementation from RFC 0033 alone.

After the gate opens, implement only the accepted scope in `docs/dogfood/033/DESIGN_SYNTHESIS.md` and the resolved threat-model review findings. Stay inside the workflow write scope.

Expected behavior:

- introduce a daemon-owned Postgres connection layer in `src/striatum/` (concrete module name decided in the synthesis);
- add forward-only Postgres schema migrations alongside the existing repo-local SQLite migrations, with the daemon refusing to start on schema mismatch (exit code 9 parity);
- add audit row trigger / role-enforced append-only logic; map V1 audit segment manifests onto the new substrate; verify chain via `daemon doctor`;
- implement `striatum daemon migrate --from sqlite --to pg [--dry-run] [--keep-sqlite-readonly]` with byte-equivalent audit chain validation and explicit V1 read refusal after cutover;
- add the daemon-doctor onboarding hints (platform-specific Postgres install instructions when the URL is missing or unreachable);
- add the per-test Postgres harness in `tests/conftest.py` or equivalent; teardown leaves no zombie connections;
- add integration tests for: forward migration apply, V1 → V2 cutover byte-equivalent audit chain, capability check under serializable isolation, supervisor heartbeat concurrency, audit append concurrency;
- update `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`, the RFC 0033 status block, `docs/rfcs/README.md`, `README.md`, and `CHANGELOG.md` only as required by the accepted plan;
- describe daemon guarantees honestly per the RFC 0031 threat model: AI guardrail, not cryptographic proof, not defense against a malicious operator.

Do NOT:

- ship bundled / Dockerized Postgres (deferred);
- design or implement the Python → Go substrate port (D084; future RFC);
- modify repo-local `.striatum/state.sqlite3` semantics;
- add devils_advocate or security review jobs to this dogfood's workflow (those are post-implementation per operator decision);
- claim sealed-mode apply (RFC 0031) or RPC server (RFC 0030) features that depend on later RFCs.

Run `make install`, `make lint`, `make typecheck`, `make test`, `make smoke` after all changes are in place. Address every failure honestly; do not skip tests to make the bar look green.

Produce `docs/dogfood/033/BUILD_HANDOFF.md` summarizing changes, tests added/passing, deferred scope, follow-up RFC dependencies (RFC 0030 will key off the substrate version recorded here), and any human-decision items that the threat-model review did not pre-resolve.

Do not call striatum CLI; the operator publishes on your behalf.
