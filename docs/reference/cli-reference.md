# CLI Reference

> This page is a copy-paste reference. It may lag the parser;
> `striatum --help` (and `striatum <verb> --help`) is
> authoritative.

## Core lifecycle

```text
striatum init [--with-skills <profile>] [--with-ddd-layout]
              [--ddd-layout-force] [--ddd-layout-dry-run]
              [--with-striatum-layout]
              [--striatum-layout-workflow <slug>]
              [--striatum-layout-dry-run]
striatum adopt [--profile <profile>] [--postgres-url <url>]
               [--dry-run] [--no-skills] [--no-plugins]
               [--no-ddd-layout] [--no-register]
striatum workflow validate
striatum workflow generate
striatum workflow templates list
striatum workflow templates show
striatum run prepare
striatum branch confirm
striatum run start
striatum run summary
striatum archive create
striatum archive verify
striatum operator current-brief
```

`workflow generate` (below) is the way to scaffold a starter workflow tree;
`run prepare` requires an explicit `--workflow <path>` and creates no runtime
default. The generated tree uses a single `local` process lane as a valid
placeholder; edit lanes and job `lane_id` bindings for real agent runs.

`workflow templates list [--kind shape|lane_set|role_pack|adversary_pack]`
and `workflow templates show <template_id>` read the bundled local
workflow-template catalog (embedded under `go/pkg/workflowtemplates`).

`workflow generate --shape <shape> [--lane-set <set>] [--workflow-id <id>]
[--scaffold-root <path>] [--artifact-root <path>] [--option key=value ...]`
compiles a concrete workflow tree from that catalog. It previews the planned
repo-relative files by default and writes them only with `--write`; `--json`
emits the structured envelope. The lane set defaults to the `local` fixture
lane (which needs no real lane command), so `workflow generate --shape
conversation --option topic="…"` scaffolds a valid starter out of the box; edit
in real lanes (e.g. `--lane-set author_reviewer` then supply lane commands in
the generated `workflow.json`) before a real run. `--option phases=…`
(`multi_phase`) and `--role-pack`/`--adversary-pack` options
(`implementation_panel`) are accepted as the shape requires.

The Python-era `workflow init`, `workflow lint`, `workflow plan`,
`workflow graph`, `workflow upgrade`, and `workflow templates render-md`
verbs are not part of the current Go CLI (RFC 0078 ported `validate`,
`generate`, and `templates {list,show}`); workflow-authoring lint is enforced
at `validate`/generation time. A fuller CLI_REFERENCE audit against the Go
command surface is tracked separately.

Same-model-pairing lint is enforced by `workflow validate` (refuse unless
`--allow-same-model-pairing`); operational accepted-risk overrides are recorded
through the daemon `workflow accept-risk` / `workflow accepted-risks` commands.

`striatum init` creates `.striatum/` in the target repo. The
optional flags scaffold extra material:

- `--with-skills <profile>` (RFC 0015) — write the agent skill
  bundle for `claude_code` | `codex` | `gemini` | `generic` |
  `all`. Default profile is `claude_code`.
- `--with-ddd-layout` (RFC 0021) — scaffold the seven canonical
  reader-facing DDD documents (`docs/SPEC.md`, `docs/PRD.md`,
  `docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/DDD.md`, `docs/rfcs/README.md`,
  `docs/rfcs/0001-template.md`). Existing files are preserved.
- `--ddd-layout-force` (RFC 0021 V1.5) — overwrite existing
  regular-file targets with the template body. Records
  `prior_sha256` for audit. Non-regular-file targets
  (directories, broken symlinks) still error and are not
  touched.
- `--ddd-layout-dry-run` (RFC 0021 V1.5) — preview without
  writing. Per-file statuses use the `would_*` vocabulary.
- `--with-striatum-layout` (RFC 0056 Phase B) — create the
  recommended consumer-repo directories `striatum/workflows/` and
  `striatum/<workflow-slug>/`. It writes no workflow files and no
  `.gitignore` policy.
- `--striatum-layout-workflow <slug>` — select the artifact-root
  directory slug for `--with-striatum-layout`; default is
  `code-change`.
- `--striatum-layout-dry-run` — preview the Striatum directory
  scaffold without creating directories.

`striatum adopt` is the day-zero guided flow. It initializes
`.striatum/`, installs the selected skill/plugin profile, scaffolds the
DDD docs, registers the repo into daemon PostgreSQL when a Postgres URL is
configured, and returns a suggested starter workflow
path. Use `--dry-run` to preview, or `--no-register` when you only want
the filesystem setup.

`operator current-brief [--operator-docs-root <path>] [--json]`
(RFC 0058 V1.5) is a local read-only helper for the operator progress
surface. It reads `docs/operator/BRIEF.md` by default, refuses a missing
brief, symlink, non-regular file, invalid `operator_brief` front matter,
or `status` other than `current`, and prints `path`, `brief_id`,
`supersedes`, `scope_links`, `context_budget_lines`,
`retrieval_priority`, and `cold_start_paths`. This command does not
route through daemon RPC and is exempt from daemon-required enforcement
because Markdown operator provenance is the authority for this surface.
`--operator-docs-root` points at the directory containing `BRIEF.md`,
not at the file itself.

## Agent / session work loop

```text
striatum register-session
striatum claim-next
striatum ack
striatum heartbeat
striatum release
striatum send
striatum block
striatum publish-artifact
striatum complete
striatum verdict
striatum submit-review
striatum decision record
```

`publish-artifact` validates lease ownership, write scope, path
safety, artifact kind, front matter, and byline. Model-bylined
artifacts require lane evidence: a path-specific supervised
`artifact_observed` event when the wrapper reports one, or the legacy
clean `process_executions` fallback. Operators can explicitly bypass
missing lane evidence with `--allow-no-process-execution
--override-rationale <text>`; the rationale is stored on the artifact
row and in the provenance event.

## Worktree (opt-in per lane via `worktree_isolation: per_job`)

```text
striatum worktree create
striatum worktree release
striatum worktree list
```

## Supervisor (RFC 0009)

```text
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list
```

## Dashboard

```text
striatum dashboard
striatum dashboard --all
```

`dashboard --all` (RFC 0028 V1) groups registered repositories and
reports daemon/Postgres-backed per-repository runs, blockers, claimable jobs,
stale leases, and degraded repositories. It uses daemon-owned repository
registration state and requires a daemon `read` capability token
(bootstrapped by `daemon start`) even when `--daemon` is not passed.

## Daemon and multi-repo registry (RFC 0028 V1)

```text
striatum daemon start [--core go]
striatum daemon status
striatum daemon stop
striatum daemon sweep
striatum daemon service install [--manager auto|systemd|launchd] [--dry-run]
striatum daemon service start [--manager auto|systemd|launchd] [--dry-run]
striatum daemon service status [--manager auto|systemd|launchd]
striatum daemon doctor [--postgres-url <url>] [--apply-migrations]
                       [--as-owner <owner-url>]
                       [--provision-rw-role] [--repair-grants]
                       [--explain] [--authority] [--repo <path>] [--json]
striatum daemon migrate-db [--admin-url <dsn>] [--json]
striatum daemon migrate --from sqlite --to pg [retired compatibility refusal]
striatum daemon migrate-repo-local --from sqlite --to pg
                         [--repo <path>] [retired compatibility refusal]
striatumd [daemon-start options]
striatum repo add <path> [--init] [--no-migrate compatibility flag]
striatum repo list
striatum repo remove <id>
striatum cross-repo list
striatum cross-repo describe <cross_repo_run_id>
striatum cross-repo why <cross_repo_run_id>
striatum cross-repo cancel <cross_repo_run_id> [--reason <text>]
```

`striatum daemon start` / `striatumd` runs the supported foreground daemon
process. It launches the Go daemon; `--core go` is a deprecated no-op
compatibility flag, `--core python` is no longer accepted, and
`striatumd --foreground` is accepted only as a legacy spelling for
`striatumd`. Per D094 / RFC 0043 the daemon is a hard prerequisite for every
Striatum verb; CLI verbs without a reachable daemon refuse with exit code 11
(`daemon_unreachable`) and do not fall back to direct mode.

The first `daemon start` bootstraps a single admin token when
daemon-owned Postgres has no clients and writes a `0600`
runtime `client-token` file. Token secrets are never read from environment
variables, never logged to audit, and never stored in the registry.
Authorization uses the closed daemon method capability vocabulary:
`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.

`repo add` canonicalizes the repository root, refuses
symlink/path-traversal ambiguity, derives a realpath/inode-based
repository identity, and refuses active path re-occupation by a
different identity. Pass `--init` when no `.striatum/` directory
exists; it creates operational scratch only and does not create
`.striatum/retired-local-state`. If a pre-D094 repo-local SQLite source
exists, registration refuses and tells the operator to archive/remove
the legacy SQLite file before registering.

`repo remove` is idempotent, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses
`repository_id`; re-adding allocates a fresh id.

`daemon sweep` is admin-gated and runs the sweep loop manually
across registered active runs; the normal recovery sweep also
runs from the foreground daemon process.

`daemon service install|start|status` renders and controls a user
service for the local daemon. `--manager auto` chooses systemd user
units on Linux and launchd agents on macOS; explicit `systemd` and
`launchd` choices are available for testing or non-standard hosts.

RFC 0033 V2 accepts system PostgreSQL as the daemon-owned
storage substrate for daemon-global state. Configure it with
`STRIATUM_DAEMON_DB_URL`, daemon config, or an explicit
`--postgres-url` client surface. The daemon owns schema
migrations and roles, but it does not install, start, stop, or
upgrade PostgreSQL. Bundled, embedded, and Dockerized Postgres
distributions are deferred.

`daemon doctor` reports daemon DB connectivity, substrate version,
schema version, audit-chain status, segment-manifest verification,
repository registration status, and retired SQLite evidence. It runs
even when the daemon process is down (it reads configuration directly)
and emits the remediation list operators need to bring the daemon online.
`--apply-migrations` brings the daemon-owned schema forward
in-place; without it, doctor reports the required version and
exits so operators can review before applying.
`--as-owner <owner-url>` (GH #22) is the supported owner-role migration
path: when set together with `--apply-migrations` (and/or
`--provision-rw-role` / `--repair-grants`), doctor opens a second
connection against the owner URL and runs migration application,
role provisioning, and grant repair through it, while the
`unsafe_privileges` runtime guardrail still evaluates the runtime role
resolved from `--postgres-url` / `STRIATUM_DAEMON_DB_URL`. Local Linux
installs typically use a peer-auth socket URL such as
`postgresql:///striatum_daemon`. If the owner URL is unreachable, doctor
returns `status: "as_owner_unreachable"` with the redacted URL and the
scrubbed error.
`--provision-rw-role` creates the local `striatumd_rw` runtime role
when the current Postgres connection can create roles. `--repair-grants`
applies the runtime grants and append-only revokes; when privileges are
insufficient, doctor returns pasteable SQL for an admin session.
`--authority` adds a cutover report that names PostgreSQL live-state
authority, legacy SQLite registry status, method fallback counts, and
remaining migration/test-only SQLite exceptions.
`--repo <path>` adds the verify-only
`striatum.repo_cutover_report.v1` for that target repository to the doctor
output without opening SQLite; with `--authority`, the authority report also
summarizes whether that repository cutover is healthy.

`daemon migrate-db [--admin-url <dsn>] [--json]` (RFC 0079 §5) applies pending
daemon PostgreSQL schema migrations using an owner/admin DSN, so DDL the runtime
role (`striatumd_rw`) cannot perform — e.g. a migration that adds a foreign key
to an owner-held table — is applied before the daemon serves. The admin DSN is
resolved from `--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL`, then the normal
daemon DSN (flag/env/`daemon.toml`) as a fallback for additive migrations the
runtime role can apply itself. This is distinct from the retired SQLite-era
`daemon migrate` below.

`daemon migrate --from sqlite --to pg` and
`daemon migrate-repo-local --from sqlite --to pg --repo <path>` are retired
compatibility spellings (the SQLite→PostgreSQL import, not schema migration).
They remain parseable so old automation receives a
clear error, but they refuse with exit code 12 before importing or opening
SQLite migration code. `daemon doctor --repo <path> --authority --json` is the
supported cutover-evidence diagnostic and does not open SQLite as a database.
CLI verbs against an unregistered repo refuse with exit code 12
(`repo_not_migrated`) and point operators to archive/remove legacy SQLite
files, then register with `adopt` or `repo add --init`.

RFC 0030/0031 add the daemon V2 RPC and supervision/apply foundation on
top of RFC 0033. The wire envelope is versioned JSON; `daemon.hello`
negotiates envelope/framing, `daemon.describe` publishes the method
registry and `methods_etag`, and incompatible clients refuse with exit
code 10. RFC 0048 completed the production handler-port work in
v1.49.0-v1.55.0: mapped production verbs are daemon/Postgres-backed and
fail closed without daemon reachability, repository registration, and
capability authorization. Legacy SQLite paths remain only for migration
sources, golden fixtures, and explicitly gated subprocess compatibility
tests under `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`.

## Daemon-required runtime

Production commands route through the daemon authority boundary. Some
read verbs still accept `--daemon` (or `STRIATUM_DAEMON=1`) as an
explicit compatibility spelling, but it is no longer the switch that
turns daemon mode on:

```text
striatum --daemon status [--run-id <id>]
striatum --daemon doctor
striatum --daemon why <job-id>
striatum --daemon dashboard --all
striatum doctor --first-run
```

The V1 `--no-daemon` flag is retired (D094 / RFC 0043); parsing it
returns the standard argparse "unrecognized arguments" error and exit
code 2. Production mutation and read verbs do not fall back to direct
repo-local mode.

`doctor --first-run` is a bootstrap smoke check, not a normal
repo-state doctor run. It returns a
`striatum.first_run_diagnostic.v1` JSON report that verifies daemon socket
reachability, Go daemon binary provenance from `striatumd --describe`,
Postgres doctor status, runtime token presence, repository registration,
MCP tool visibility, one sample daemon read route, and the daemon authority
report.

Daemon RPC method capabilities use the closed vocabulary `read`,
`write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.
`supervise.*` and `apply.*` are daemon RPC routes; sealed apply fails
closed unless a daemon signing key and `apply` capability are present.
The Go daemon rotates the local Ed25519 fallback signing key through the
admin RPC method `daemon.key.rotate`; there is not yet a stable
user-facing `striatum keys` CLI.

RFC 0032 adds cross-repo workflow schema and daemon MCP mutation
capability gating on the PostgreSQL daemon substrate. Cross-repo
workflow files declare `repositories`, `primary_repository`, and
per-job `repository` aliases. The daemon DB records canonical
`cross_repo_run_id` rows under participating repository scopes.
`cross-repo list|describe|why` inspect those daemon records according to
capability scope. `cross-repo cancel` is the `cross_repo.cancel` recovery
route: it cancels non-terminal participant runs through the PG-native
participant runner, skips terminal participants and preparing participants
that never created a local run, and returns `blocked` with diagnostics when a
participant cannot be canceled. Daemon MCP `tools/list` is filtered by each
token's effective capabilities and scope, and `tools/call` re-checks
authorization and audits denials.

RFC 0036 adds no new CLI verb. Regenerate agent-facing MCP guidance with
`striatum skills install` or `striatum plugin install`; chat workflow
generation uses the existing local service and RFC 0034 generator paths.

## Skills (RFC 0015)

```text
striatum skills install
```

## Service (RFC 0012 / 0013)

```text
striatum serve
```

### Web routes (RFC 0013 / 0022 / 0024 / 0038)

`striatum serve --web` exposes the legacy server-rendered operator UI on the
same localhost-only origin as the JSON API. RFC 0038 V1 mounts React
"frontend islands" into specific page slots; the rest of every page remains
server-rendered. RFC 0078 has begun the Go web-service cutover, but full route
parity and daemon startup wiring remain deletion blockers. There are no new
CLI verbs; the routes below are reachable in any browser pointed at the bound
URL.

| Route | Surface |
| --- | --- |
| `/` | Run list (RFC 0013/0022/0037). |
| `/run/<id>` | Run detail with state-coloured dependency graph. |
| `/run/<id>/job/<id>` | Job detail. |
| `/run/<id>/artifact/<id>` | Artifact viewer with inline Markdown. |
| `/workflows/` | Workflow file browser (RFC 0024 V1). |
| `/workflows/<path>` | Workflow detail with graph thumbnail and the promoted Edit button (RFC 0038 V1). |
| `/workflows/edit/<path>` | Drag-drop graph editor island (RFC 0038 V1) over the existing `POST /workflows/edit/<path>` endpoint with `If-Match` semantics. |
| `/workflows/new` | Chooser-wizard island that calls `POST /workflows/generate/preview` then `POST /workflows/generate` with a `<dialog>`-driven operator confirmation (RFC 0038 V1; requires `--allow-mutations`). |
| `/view/` | Tree-browser island over `GET /v1/repo/tree?path=<rel>` (RFC 0038 V1). |
| `/view/<path>` | Single-file viewer; Markdown renders server-side, other text files mount the Shiki code-viewer island (RFC 0038 V1). |
| `/chat` | Chat surface (RFC 0023 / RFC 0036 / RFC 0040). |
| `/doctor` | Grouped doctor problems with terminal-run filter (RFC 0037). |

## List (read-only enumeration)

```text
striatum list runs
striatum list sessions
striatum list jobs
striatum list artifacts
striatum list workflows
```

`list runs` includes the workflow identity triple for each run:
`workflow_id`, `workflow_version`, and `workflow_snapshot_id`. The web
run list uses the same snapshot identity to display the workflow name
and link back to the workflow detail when the source path is known.

## Inspection and recovery

```text
striatum status
striatum why
striatum doctor
striatum git snapshot
striatum git commit-apply
striatum evidence export
striatum run graph
striatum recovery auto
striatum recovery auto-publish
striatum recovery auto-finalize
striatum recovery watch
striatum recovery stale-leases
striatum recovery requeue-stale
striatum recovery cancel-job
striatum recovery process-reconcile
striatum recovery resume
striatum checkpoint resolve
striatum override-verdict
```

`run graph --run-id <id> [--format mermaid|json|dot|ascii]`
renders the workflow graph for an existing run with each node
colored by current job state. Mermaid output appends
`classDef`/`class` lines; JSON adds `current_state`, `attempt`,
and a `latest_verdict` block on review nodes; `ascii` reuses the
dashboard's graph panel renderer (RFC 0016).

`git snapshot --json [--ancestry-limit N] [--no-ancestry]` emits the
daemon read-only `git.snapshot` projection for the registered target
repository: local branch, HEAD metadata, dirty counts, changed paths,
and bounded ancestry. It does not fetch, push, commit, read remote URLs,
or include diff hunks or commit bodies.

`git commit-apply <commit-request-path> --confirm --confirm-request-id <id>
--json` emits daemon method `git.commit_apply`. It creates only a local
commit from a `striatum.commit_request.v1` artifact whose
`confirmation_status` is already `operator_confirmed` or `human_confirmed`.
It refuses base-HEAD, branch, or dirty-path mismatches, disables repository
Git hooks for the commit invocation, and never pushes or calls hosted
providers.

`recovery auto` emits the daemon `recovery.sweep` method. The sweep
runs workflow-opt-in `recovery.auto_finalize` before lazy lease expiry,
then the existing stale-lease, process-reconcile, and review-only requeue
recovery pieces where policy allows. Timed-out human checkpoints execute
the configured `recovery_policy.escalation_hook` in live sweeps; dry-runs
report the hook kind without side effects, and hook failures are reported
inside `escalations[]`. `recovery auto-publish` emits the explicit
`recovery.auto_publish_stale_artifacts` method; the deprecated `recovery.auto`
alias is not emitted by the current CLI. `recovery watch` is a CLI-local
foreground scheduler that repeatedly calls daemon `recovery.sweep`; it is not
a registered daemon RPC method.

## Corpus export (RFC 0044 V1 / RFC 0057 contract)

```text
striatum corpus export --since <ref> --out <dir>
striatum corpus verify --bundle <dir>
```

`corpus export` emits a redacted JSONL bundle of Striatum's durable
provenance (RFCs, decision-log rows, operator reports, run summaries,
audit-chain entries, changelog entries, ubiquitous-language terms,
harness-friction patterns, recent commits) plus a verifying
`manifest.json` with explicit `state_authority` metadata. Re-running over
unchanged inputs produces byte-identical JSONL files and stable per-file
SHA-256s; only `generated_at` varies, and it is excluded from the bundle
digest.
`corpus verify` is a local read-only checker for an existing bundle; it
validates the manifest, per-file hashes and byte counts, JSONL row shape,
duplicate row ids, row/file `sub_kind` consistency, row counts, and the
implied V1 corpus contract version.

The bundle is operator-triggered local provenance, never streamed to any
external service. Optional consumers (Engram is the first reference under
RFC 0044) may ingest the bundle for retrieval, but Striatum does not call
them at runtime and runs identically when no consumer is configured. The
V2 contract decisions (multi-corpus identity, redaction-tier metadata,
incremental watermarks, optional context-injection policy) are scoped by
[RFC 0057](rfcs/0057-corpus-contract-v2.md).

## Run archive

```text
striatum archive create --run-id <id> --out <dir>
striatum archive verify --bundle <dir> [--manifest-only] [--repo-root <path>]
striatum archive inspect --bundle <dir> [--repo-root <path>]
```

`archive create` is a daemon/Postgres-backed read command that writes a
local archive directory for one run. The V2 archive contains the run row,
workflow snapshot, run-scoped rows, artifact metadata, event metadata, and a
self-verifying `manifest.json`; it does not copy artifact contents,
transcripts, or `.striatum/` scratch. `archive verify` is local and
read-only against an existing archive bundle, and it runs offline semantic
replay by default. `--manifest-only` is the explicit fast path that skips
semantic replay; `--repo-root` also verifies artifact content hashes against
files in a local repository checkout. `archive inspect` is a read-only local
projection over the same verifier.

## Adapter

```text
striatum adapter run
```

`adapter run` is retired outside the explicit legacy test-fixture
compatibility environment. Use daemon-supervised process lanes instead.

## Session lifecycle

```text
striatum session close
```

## Stable exit codes

- `0`: success, including `claim-next` with `no_work`.
- `1`: generic / unhandled runtime error.
- `2`: CLI usage error (argparse).
- `3`: missing run, session, job, message, blocker, artifact,
  verdict, or session target.
- `4`: invalid state transition.
- `5`: lease expiry or ownership mismatch.
- `6`: artifact or write-scope violation.
- `7`: branch confirmation required before work can be claimed.
- `8`: workflow config rejected (also raised by `branch confirm`
  when a requested git operation cannot be performed).
- `9`: state schema is newer than this striatum install supports
  (daemon PostgreSQL in production; legacy SQLite only in fixture paths).
- `10`: daemon RPC transport, handshake, or version-skew refusal.
- `11`: `daemon_unreachable`. The CLI could not reach the daemon
  socket; stderr names the socket path and remediation. No SQLite
  fallback is attempted.
- `12`: `repo_not_migrated`. The target repository is not registered for
  daemon/PostgreSQL state or still has a legacy `.striatum/retired-local-state`;
  stderr and the `--json` hint tell the operator to archive/remove legacy
  SQLite files and register with `adopt` or `repo add --init`.

## See also

- [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) — the operator's playbook
  with examples per verb.
- [HOW_TO_AGENT.md](HOW_TO_AGENT.md) — the coding-agent
  companion to the RFC 0015 skill bundle.
- [SPEC.md](SPEC.md) — the implementation contract.
