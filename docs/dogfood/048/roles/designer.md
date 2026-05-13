# Designer Role (Dogfood 048)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0043 V1 (Postgres-as-sole-substrate + daemon-required
runtime). Synthesis picks one path across two implementer tracks
(A schema, B CLI). Cite existing code that your design changes — do not
propose green-field shapes.

Required citations (read these before designing):

- `docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md` —
  §1 (substrate boundary), §3 (daemon-required CLI), §4 (migration
  command), §5 (method-registry expansion), §7 (test infrastructure).
- `docs/rfcs/0030-daemon-rpc-server-and-version-skew-protocol.md` — RPC
  envelope, method registry pattern.
- `docs/rfcs/0033-storage-substrate-rewrite-for-daemon-v2.md` — daemon
  Postgres schema, roles (read-write, append-only, migration), audit
  chain anchoring.
- `docs/DECISION_LOG.md` — D094 supersedes D006/D007/D036 and the SQLite
  half of D009.
- `src/striatum/db.py`, `src/striatum/schema.py`, `src/striatum/migrations.py` —
  V1 SQLite surface being retired.
- `src/striatum/daemon_pg/` (config, connection, migrations, audit,
  cutover) — daemon-side Postgres surface to extend.
- `src/striatum/daemon_rpc/` — RPC method registry to expand.
- `src/striatum/cli/parser.py`, `src/striatum/cli/dispatch.py`,
  `src/striatum/cli/mutations.py` — the CLI surface that retires
  `--no-daemon` and that gains exit codes 11 + 12.

Address both tracks (Track A schema + migrate-repo-local; Track B CLI
surface + RPC registry). Cover concretely: exact file paths, function
names, capability mapping per RPC method, test paths. Cite the patterns
in RFC 0030/0033 you mirror.

**D094 framing is non-negotiable**: this is the substrate flip. The
SQLite half of D006/D007/D036/D009 is superseded. `--no-daemon` retires
immediately. There is no soft-retirement period. Exit codes 11 + 12 are
new and reserved.

Out of scope: bundled Postgres distribution, multi-tenancy enforcement,
hosted-mode auth, the RFC 0039 Go-core revision itself, rewriting
historical dogfood scaffolds. README / TODO / CHANGELOG / SPEC / HOW_TO
updates are operator-only after the dogfood lands.
