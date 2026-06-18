Current status (2026-05-17): GH #15 is historical context. RFC 0048
completed in v1.55.0, and current operator-facing docs should describe
daemon-owned PostgreSQL as the production live-state substrate with
repo-local SQLite limited to migration sources, tombstones, and fixtures.
The issue body below is preserved verbatim as the original report.

    # GH #15 -- Docs: clarify PostgreSQL transition guidance

    Source: <https://github.com/halbritt/striatum/issues/15> (filed 2026-05-14).
    Labels: none.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    ## Problem

The current docs are not sufficient to guide an operator through the PostgreSQL transition. The design and release history contain the necessary pieces, but the public/operator-facing docs still contradict the accepted D094/RFC0043 direction and the current CLI surface.

## Evidence

- `README.md` still says `.striatum/retired-local-state` is authoritative live state and reports an old status/version.
- `docs/SPEC.md` still says SQLite under `.striatum/retired-local-state` is authoritative, even though D094/RFC0043 move workflow state to daemon-owned PostgreSQL.
- `docs/GETTING_STARTED.md` omits PostgreSQL from prerequisites and the quick-start path.
- `docs/HOW_TO_HUMAN.md` still says `init` creates `.striatum/retired-local-state` and describes RFC0033 as leaving repo-local SQLite as live run state.
- `docs/CLI_REFERENCE.md` omits `striatum daemon migrate-repo-local` from the daemon command list and still says `--no-daemon` forces direct mode.
- `docs/UBIQUITOUS_LANGUAGE.md` still defines repo-local state, daemon DB, repository tenant, supervisor pointer, etc. in the pre-D094 hybrid model.
- Skill/plugin templates still tell agents that `.striatum/retired-local-state` is live state / the substrate not to touch.
- RFC0048 correctly documents that the daemon-side substrate migration is incomplete at the business-logic-handler layer, so the docs also need a status matrix instead of implying the transition is fully complete.

## Expected outcome

Create coherent PostgreSQL transition guidance that distinguishes:

1. RFC0033 daemon-global PostgreSQL substrate.
2. RFC0043 D094 PostgreSQL-as-authoritative-state direction and migration command.
3. Current shipped behavior and hardening.
4. RFC0048 remaining work where daemon RPC still delegates some single-repo business logic through SQLite-backed paths.

## Suggested scope

- Update `README.md`, `docs/SPEC.md`, `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`, and `docs/CLI_REFERENCE.md` to tell one consistent PostgreSQL-first story.
- Add a dedicated transition runbook, e.g. `docs/POSTGRES_TRANSITION.md`, covering:
  - system PostgreSQL prerequisite;
  - `STRIATUM_DAEMON_DB_URL` / daemon config / `--postgres-url`;
  - `striatum daemon doctor --postgres-url ... --apply-migrations`;
  - daemon startup expectations;
  - `striatum daemon migrate-repo-local --from sqlite --to pg --repo <path> --dry-run`;
  - full migration;
  - tombstone vs delete behavior;
  - verification after migration;
  - exit code 11 `daemon_unreachable` and exit code 12 `repo_not_migrated`;
  - rollback/inspection limits.
- Update `docs/UBIQUITOUS_LANGUAGE.md` with the post-D094 terms already listed in RFC0043.
- Update skill/plugin templates so regenerated agent guidance no longer teaches the old SQLite-authoritative model.
- Add or update doc-link/reference tests so stale SQLite-authoritative wording does not come back outside frozen historical dogfood artifacts and explicit migration fixtures.

## Definition of done

- A new operator can follow docs from install through PostgreSQL setup, daemon doctor, repo migration, and verification without reading RFC history.
- The docs clearly mark RFC0048 daemon-side substrate migration as remaining work where applicable.
- No current product doc claims `.striatum/retired-local-state` is authoritative live state except when describing historical V1 behavior, migration fixtures, or tombstone inspection.
- `striatum daemon migrate-repo-local --help` and docs agree on command shape and flags.
