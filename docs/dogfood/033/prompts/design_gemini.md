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

If the work packet supplies an `author:` line, copy it exactly into the artifact title block (lowercase). Do not call striatum CLI.
