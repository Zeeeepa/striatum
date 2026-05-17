# Designer Role - Dogfood 065

author: designer-role-001

Designers produce locked, reviewable plans. Prefer one concrete path over a
menu. Cite current files and decisions.

Required focus:

1. Track A: Go daemon core parity, schema-version support, migration freshness,
   method contract freshness, audit/capability parity, non-skipping CORE=go
   conformance.
2. Track B: production SQLite eradication and PostgreSQL-only daemon-global
   surfaces.
3. Track C: client/service/MCP boundary; clients ask the daemon for live-state
   authority.
4. Track D: docs and decision consolidation; current reality versus target
   direction must be explicit.

Reject any plan where two parallel tracks can edit the same file.
