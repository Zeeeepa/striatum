# Codex Design Prompt

Produce `docs/dogfood/033/design/codex/DESIGN.md`.

Design an implementation plan for the RFC 0033 storage substrate rewrite for daemon V2.

Your plan must cover:

- the concrete Postgres schema layout for daemon-owned state (registry, capability tokens, audit rows, audit segment manifests, scheduler cursors, daemon metadata) and how it differs from the V1 SQLite shape;
- the forward-only migration runner: where migrations live in `src/striatum/`, how the daemon refuses to start when the on-disk schema is newer than the binary (exit-code-9 parity with repo-local SQLite), and how schema version is recorded in every audit row;
- the audit-chain Postgres mapping: row triggers, `previous_hash`/`row_hash` calculation, role-enforced append-only insert privileges, and how `daemon doctor` verifies segment manifests against retained rows;
- the V1 SQLite registry → V2 Postgres cutover: `striatum daemon migrate --from sqlite --to pg [--dry-run]` UX, byte-equivalent audit chain validation, V1 read refusal post-cutover, and the `--keep-sqlite-readonly` tombstone option;
- daemon doctor: minimum Postgres major-version check, role privilege check, schema version surfacing, and platform-specific install hints in error messages;
- test harness shape: per-test Postgres connection isolation (schema-per-test or DB-per-test), teardown that leaves no zombie connections, and how integration tests exercise the V1 SQLite → V2 PG cutover end-to-end;
- concurrency primitives in the daemon paths that key off the substrate (supervisor heartbeats, audit append, capability lookup) and how they coexist under serializable isolation without deadlock;
- documentation deltas in `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, and `docs/HOW_TO_HUMAN.md`;
- the operator-onboarding story for someone who has never installed Postgres: platform-specific install hints, `STRIATUM_DAEMON_DB_URL` examples, role-creation snippets.

Explicitly mark as deferred (do NOT design them):

- bundled / Dockerized distribution (separate follow-up RFC);
- Python → Go substrate port (D084; named, not designed);
- repo-local `.striatum/state.sqlite3` changes (out of RFC 0033 scope).

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.
- Correct: `author: designer-codex-gpt-5.5-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Future packets for `finding`, `synthesis`, `decision`, or other schema-bearing kinds will require a JSON-quoted `key: <value>` front matter block at the very top of the file; follow the example the packet provides.

Do not call striatum CLI unless your harness profile already permits it; the operator publishes on your behalf otherwise.
