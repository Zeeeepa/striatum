# Designer Role (Dogfood 058)

Three fresh-design lanes (codex, claude, gemini) produce independent perspectives on RFC 0048 V1.5 fix-up. Synthesis picks one path; tracks split as in dogfood-057.

Required reading:

- `docs/rfcs/0048-daemon-side-substrate-migration.md` — especially the V1.5 follow-up section.
- `docs/dogfood/057/review/build/{codex,claude}/REVIEW.md` — verbatim finding lists.
- `docs/dogfood/057/build/{track_a,track_b}/HANDOFF.md` — current V1 state.
- `src/striatum/daemon.py` — `run_daemon_foreground` accept-loop gap.
- `src/striatum/daemon_rpc/server.py` — `DaemonRpcRouter._route` (route-and-fall-back today).
- `src/striatum/daemon_pg/handlers/` — 16 handlers needing chain-locking + capability checks.
- `src/striatum/daemon_pg/sql/` — schema migration sequence (latest is 0005).
- `tests/daemon_pg/handlers/recovery_evidence/conftest.py` — the advertised parity rig you'll wire up.

Output: `docs/dogfood/058/design/<lane>/DESIGN.md`.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.
