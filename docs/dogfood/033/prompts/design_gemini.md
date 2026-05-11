# Gemini Design Prompt

Produce `docs/dogfood/033/design/gemini/DESIGN.md`.

Design an implementation plan for the RFC 0033 storage substrate rewrite for daemon V2. Emphasize operational reality across macOS, Linux, and Windows-via-WSL. The substrate decision is "system Postgres required" (RFC 0033 §2); bundled / Dockerized distribution is deferred to a follow-up RFC.

Your plan must cover:

- per-platform operator install paths for Postgres (Homebrew on macOS, apt/yum/pacman on Linux, WSL + apt on Windows) and how `striatum daemon doctor` surfaces platform-specific hints when the install is missing or the version is wrong;
- minimum supported Postgres major version (recommendation in RFC 0033: PG 14+; reviewers may push). Cross-platform availability of that version on each supported package manager;
- daemon connection model: `STRIATUM_DAEMON_DB_URL` env var, `--postgres-url` flag, `~/.config/striatum/daemon.toml` precedence and merging; how secrets land (URL contains password? PGPASSFILE? unix-socket no-password connections);
- role and database creation UX: does the operator run `CREATE ROLE` and `CREATE DATABASE` manually, or does `daemon doctor --bootstrap-role` emit the SQL? Pick one and justify;
- connection pool shape inside the daemon (pgxpool-equivalent in Python via psycopg pool); maximum connections, idle timeout, refresh on Postgres restart;
- crash and restart scenarios: daemon restarts cleanly, Postgres restarts under daemon, both restart simultaneously, Postgres goes away mid-transaction;
- the operational lifecycle that's NOT in this RFC: PG backup, PG upgrade, PG monitoring. Document them as operator-owned with pointers to platform-standard tooling;
- adversarial test cases: malformed `STRIATUM_DAEMON_DB_URL`, role with insufficient privileges, schema in unexpected state, two daemons against the same DB simultaneously, PG version drift mid-run, network drop to a localhost-bound Postgres;
- staged delivery: which parts of the substrate land first (registry + audit), which parts depend on RFC 0030 (request log), which parts depend on RFC 0031 (supervision tables);
- documentation for `docs/HOW_TO_HUMAN.md`: the new "first time using daemon" walkthrough including PG install commands.

State which parts of the design require platform-specific work and which are cross-platform. Bundled / Dockerized distribution is deferred; do not design it.

## Byline and front matter discipline (hard constraints)

These are HARD constraints. Do not improvise variations.

**Byline format:**

- The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.
- The byline must be a plain Markdown line with NO bold (no `**`), NO italics (no `*` or `_`), NO heading prefix (no `#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly (not `Author:`, not `**Author:**`, not `# Author`).
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (this is what failed in dogfood-031 and dogfood-033)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

**Front matter (when required):**

This `handoff` artifact does NOT require front matter. Skip this section for design.

For OTHER artifact kinds you may produce in future packets (`finding`, `synthesis`, `decision`, `findings_ledger`, `support_ledger`, `action_item_ledger`, `harness_improvement_proposal`), the publisher requires a YAML-style front matter block at the very top of the file. Each line is `key: <JSON-value>` (strings must be JSON-quoted, lists are JSON arrays, bools are `true`/`false`). Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-NNNN"]
---
```

The byline appears AFTER the front matter block and a blank line, not inside it.

Do not call striatum CLI; the operator publishes on your behalf.
