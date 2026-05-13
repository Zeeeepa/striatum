# Track C Build Handoff: RFC 0042 Draft

author: implementer-codex-gpt-5.5-002

## Summary

Drafted `docs/rfcs/0042-repo-local-state-to-postgres.md` from the
Track C synthesis. The RFC proposes moving authoritative live workflow
state from per-repository `.striatum/state.sqlite3` files into daemon
Postgres keyed by `repository_id`, making the daemon mandatory for all
state-touching reads and writes, and preserving `.striatum/` as
operational scratch only.

The draft includes the required schema migration shape, including all
eighteen repo-local application tables plus non-migrated `schema_meta`,
composite-key rules, `striatum daemon migrate-repo-local --from sqlite
--to pg --repo <path>` verb, daemon-unavailable refusal behavior, RFC
0039 scope revision, migration ordering and rollback, audit-chain
preservation, acceptance criteria, implementation plan,
D006/D007/D028 supersession statement, open questions, and domain
modeling.

## Files Written

- `docs/rfcs/0042-repo-local-state-to-postgres.md`
- `docs/dogfood/042/track_c/build/HANDOFF.md`

## Notes For Review

This job intentionally did not update `docs/rfcs/README.md`,
`docs/TODO.md`, `CHANGELOG.md`, or `docs/DECISION_LOG.md`; the work
packet assigns those changes to `consolidate_phase_1`.

The RFC references D093 as requested. The current `docs/DECISION_LOG.md`
row for D093 appears to describe RFC 0040 operator-side harness work,
while the Track C synthesis says D093 is the umbrella supersession
decision for RFC 0042. I left the RFC phrasing aligned with the work
packet and synthesis, and did not alter the decision log.
