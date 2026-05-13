---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0043", "v1.5", "build"]
---

author: reviewer-unknown-model-001

# Threat-Model Review

## Verdict

needs_revision

This was a document-only review using the supplied packet references plus
the prompt-named dogfood-050 build handoff. I did not inspect production
source or tests. Under that scope, the crash-recovery and parser surfaces
are described as mitigated, but the CLI escape path remains explicitly
documented as an operator path, which fails the central RFC 0043 threat
boundary.

## Trust Boundaries And Attack Surfaces

RFC 0043 makes the daemon the only workflow-state writer. Its CLI boundary
requires every state-touching verb to route through the daemon RPC envelope,
with no direct repo-local path and no SQLite file creation, read, or fallback
when the daemon is unreachable
[docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:176].

The migration boundary is the transition from `.striatum/state.sqlite3` to
daemon-owned Postgres. RFC 0043 requires the full migration to run in one
serializable Postgres transaction, preserve source ordering, recompute and
verify byte-equivalent audit-chain anchors, write a migration checkpoint,
then finalize the SQLite source as a read-only tombstone or explicit delete
[docs/rfcs/0043-postgres-as-sole-substrate-and-daemon-required-runtime.md:233].

The post-commit recovery boundary is the kill -9 window between the Postgres
commit and SQLite finalization. Dogfood-048 identified that window as a real
split-brain risk because a crash could leave Postgres migrated while the
source SQLite remained writable on disk
[docs/dogfood/048/BUILD_HANDOFF.md:163].

The V1.5 handoff describes a sentinel-based mitigation: write
`.striatum/state.sqlite3.migrated` atomically after the Postgres commit and
before tombstone/delete, then resume finalization from already-migrated
early-return paths after verifying the source SHA against the checkpoint
[docs/dogfood/050/build/HANDOFF.md:93].

## Finding

**High: `STRIATUM_DAEMON_REQUIRED=0` remains a documented operator escape
path to the SQLite fallback.**

The V1.5 prompt allows the environment variable only if it is an explicit
opt-out gated to test-only contexts; it also requires that no silent SQLite
fallback remains anywhere in the CLI surface
[docs/dogfood/050/prompts/review_build.md:31]. The handoff satisfies the
default flip locally: `resolve_requirement` now enforces by default unless
the command is optional or `STRIATUM_DAEMON_REQUIRED == "0"`
[docs/dogfood/050/build/HANDOFF.md:48].

The problem is that the same handoff later documents `STRIATUM_DAEMON_REQUIRED=0`
as an operator migration path: upgrading operators may set it "to keep the
SQLite-backed fallback while they migrate their environments"
[docs/dogfood/050/build/HANDOFF.md:257]. That is not test-only gating. It is
a supported runtime bypass around the daemon-required boundary.

This fails the threat objective because a production user with an unmigrated
or partially finalized repo can still choose an environment variable that
routes non-lifecycle CLI verbs to the legacy SQLite implementations. The
handoff says those legacy paths remain present behind the top-level
enforcement gate in `mutations.py`, `introspect.py`, `recovery.py`,
`worktree.py`, `run_summary.py`, and `evidence.py`
[docs/dogfood/050/build/HANDOFF.md:59]. With the env opt-out enabled, that
gate intentionally returns `None`, so the bypass is usable by design.

Required revision: remove the production/operator opt-out, or gate
`STRIATUM_DAEMON_REQUIRED=0` so it is impossible outside the test harness
and explicitly impossible for normal CLI invocations. The upgrade story
should tell operators to start the daemon and run `striatum daemon
migrate-repo-local`; it should not preserve SQLite-backed fallback as a
supported migration mode.

## Non-Blocking Notes

The crash-recovery design is directionally correct in the handoff. The
sentinel is written after the Postgres commit and before finalization; resume
verifies the source SHA against the checkpoint and refuses with exit code 8
on mismatch rather than deleting suspicious data
[docs/dogfood/050/build/HANDOFF.md:99]. The handoff also names a regression
test that fails on V1 and passes on V1.5 by simulating the post-checkpoint
tombstone crash
[docs/dogfood/050/build/HANDOFF.md:214].

The exit-12 test is also shaped correctly in the handoff: it calls
`dispatch_mod.main(["--repo", str(tmp_path), "status"])`, asserts exit code
12, and verifies `repo_not_migrated` plus the `migrate-repo-local`
remediation text
[docs/dogfood/050/build/HANDOFF.md:72].

I could not verify source lines, grep results, help output, or test results
under the document-only access policy. The handoff itself says `make lint`,
`make typecheck`, `make test`, and the `migrate-repo-local --help` smoke were
not executed inside the supervised invocation
[docs/dogfood/050/build/HANDOFF.md:188].
