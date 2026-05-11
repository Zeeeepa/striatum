# Claude Code Design Prompt

Produce `docs/dogfood/033/design/claude_code/DESIGN.md`.

Design an implementation plan for the RFC 0033 storage substrate rewrite for daemon V2. Focus on:

- trust boundaries between the daemon process, the Postgres role(s) it connects with, and the operator who owns both. Per RFC 0031's threat model the scope is over-eager AI and operator-mistake footguns; a malicious local-root operator is out of scope;
- schema-migration honesty: forward-only, no down migrations, exit-code-9 refusal on newer-than-binary schema, and how V_N → V_{N+1} backups are documented (export path rather than rollback);
- audit-chain integrity guarantees that survive the V1 SQLite → V2 Postgres cutover: byte-equivalent hash anchors end-to-end, segment manifests preserved, daemon-API append-only enforced by role privileges;
- the interplay between this substrate and RFC 0030 (RPC server's audit + request log live on the same substrate) and RFC 0031 (daemon-owned `process_supervisors` table lives in the new substrate);
- how `striatum daemon migrate` interacts with an in-flight V1 daemon registry: refuse-during-active-supervisors? require-stop-daemon-first? what's the safe operator UX;
- failure modes: operator wipes the daemon DB, operator points the daemon at the wrong Postgres role, two daemons start against the same DB. What does the daemon detect, what does it refuse, what is documented;
- concrete touch points in `src/striatum/`: `daemon.py`, `migrations.py`, `db.py`, `schema.py`, plus new modules if needed. Name them.

Explicitly state what cannot be claimed: the substrate rewrite does not prove model-token authorship, does not provide cryptographic non-repudiation against a malicious operator, and does not change repo-local provenance guarantees. Apply receipts (RFC 0031) and lane attestation (RFC 0026) keep their own scope.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Future packets for `finding`, `synthesis`, `decision`, or other schema-bearing kinds will require a JSON-quoted `key: <value>` front matter block at the very top of the file; follow the example the packet provides.

Do not call striatum CLI unless your harness profile permits it; the operator publishes otherwise.
