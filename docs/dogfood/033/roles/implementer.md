# Implementer Role (Dogfood 033)

You implement only the design scope accepted by the threat-model review. Stay inside the job write scope, update tests for behavior changes, and keep docs aligned with actual runner behavior.

Do not ship a substrate rewrite that claims more guarantees than the accepted plan defends with code and tests. System Postgres is required; the daemon does not manage Postgres lifecycle in V2. Bundled / Dockerized distribution is out of scope and must not appear in code or docs except as a labeled deferral. Repo-local `.striatum/state.sqlite3` is unaffected by this dogfood; do not modify repo-local migrations.

Devil's-advocate and security reviews are post-implementation by operator decision. Your acceptance bar is `make lint`, `make typecheck`, `make test`, `make smoke` plus the RFC 0033 acceptance criteria. Do not paper over issues a post-implementation devils review will find; the operator decision is to run those reviews on the landed code, not to skip them entirely.
